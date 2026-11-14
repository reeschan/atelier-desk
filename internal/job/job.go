// Package job runs child processes as "jobs": start, cancel, time out,
// receive output line by line, and keep a short in-memory history.
//
// OS differences (hiding the console window, killing a process tree) are
// confined to proc_windows.go and proc_other.go.
package job

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// waitDelay bounds how long Wait blocks on pipes held open by descendants.
const waitDelay = 2 * time.Second

// Spec describes a child process to run.
type Spec struct {
	Name    string        // label for the job (free text)
	Path    string        // executable
	Args    []string      // arguments (without the executable)
	Dir     string        // working directory ("" = inherit)
	Env     []string      // environment; nil = the parent's environment
	Stdin   []byte        // bytes fed to stdin (empty = no stdin)
	Timeout time.Duration // 0 = no limit
}

// Stream tells which output stream a line came from.
type Stream int

const (
	Stdout Stream = iota
	Stderr
)

// State is the lifecycle state of a job.
type State string

const (
	Running   State = "running"
	Succeeded State = "succeeded"
	Failed    State = "failed"
	Canceled  State = "canceled"
	TimedOut  State = "timed_out"
)

// Result is the outcome of a job. Stdout and Stderr hold every byte read.
// Err is non-empty only when the process could not be started.
type Result struct {
	ID        string
	Name      string
	State     State
	ExitCode  int
	Stdout    []byte
	Stderr    []byte
	StartedAt time.Time
	EndedAt   time.Time
	Err       string
}

// Runner starts jobs and keeps the most recent finished ones.
type Runner struct {
	mu      sync.Mutex
	limit   int
	history []Result // newest first
	seq     int
}

// NewRunner returns a Runner that keeps the latest historyLimit results in
// memory (a value of 0 or less keeps none).
func NewRunner(historyLimit int) *Runner {
	if historyLimit < 0 {
		historyLimit = 0
	}
	return &Runner{limit: historyLimit}
}

// Job is a started child process.
type Job struct {
	id     string
	cancel context.CancelFunc
	done   chan struct{}
	result Result
}

// ID returns the job id.
func (j *Job) ID() string { return j.id }

// Cancel asks the job to stop. It is safe to call any number of times.
func (j *Job) Cancel() { j.cancel() }

// Wait blocks until the job has ended and returns its Result.
func (j *Job) Wait() Result {
	<-j.done
	return cloneResult(j.result)
}

func cloneResult(r Result) Result {
	r.Stdout = append([]byte(nil), r.Stdout...)
	r.Stderr = append([]byte(nil), r.Stderr...)
	return r
}

func (r *Runner) nextID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return "job-" + strconv.Itoa(r.seq)
}

func (r *Runner) record(res Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.limit <= 0 {
		return
	}
	r.history = append([]Result{cloneResult(res)}, r.history...)
	if len(r.history) > r.limit {
		r.history = r.history[:r.limit]
	}
}

// History returns a copy of the finished jobs, newest first.
func (r *Runner) History() []Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Result, len(r.history))
	for i, h := range r.history {
		out[i] = cloneResult(h)
	}
	return out
}

// Run starts the job and waits for it. Intended for short calls.
func (r *Runner) Run(ctx context.Context, spec Spec) Result {
	j, _ := r.Start(ctx, spec, nil)
	return j.Wait()
}

// lineSink serializes onLine calls from the two output streams.
type lineSink struct {
	mu     *sync.Mutex
	id     string
	onLine func(id string, s Stream, line string)
}

func (l lineSink) emit(s Stream, line []byte) {
	if l.onLine == nil {
		return
	}
	line = bytes.TrimSuffix(line, []byte("\n"))
	line = bytes.TrimSuffix(line, []byte("\r"))
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onLine(l.id, s, string(line))
}

// syncBuffer is a bytes.Buffer safe for a writer goroutine that may outlive
// the reader of the result.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// Bytes returns a copy of everything written so far.
func (b *syncBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

// collect reads r to EOF, keeping every byte and reporting each line. Lines
// have no length limit.
func collect(rd io.Reader, s Stream, sink lineSink, buf *syncBuffer) {
	br := bufio.NewReader(rd)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			buf.Write(line)
			sink.emit(s, line)
		}
		if err != nil {
			return
		}
	}
}

// Start launches the process and returns at once. onLine (may be nil) is
// called for each line of stdout and stderr as it arrives; calls are
// serialized. The returned error is non-nil only when the process could not be
// started; in that case the returned Job is already finished (Wait gives a
// Failed Result with ExitCode -1 and Err set) and is recorded in the history.
func (r *Runner) Start(ctx context.Context, spec Spec, onLine func(id string, s Stream, line string)) (*Job, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	id := r.nextID()
	started := time.Now()

	cctx, cancel := context.WithCancel(ctx)
	tctx, tcancel := cctx, context.CancelFunc(func() {})
	if spec.Timeout > 0 {
		tctx, tcancel = context.WithTimeout(cctx, spec.Timeout)
	}
	release := func() { tcancel(); cancel() }

	j := &Job{id: id, cancel: cancel, done: make(chan struct{})}

	fail := func(err error) (*Job, error) {
		release()
		j.result = Result{
			ID: id, Name: spec.Name, State: Failed, ExitCode: -1,
			StartedAt: started, EndedAt: time.Now(), Err: err.Error(),
		}
		r.record(j.result)
		close(j.done)
		return j, err
	}

	if spec.Path == "" {
		return fail(errors.New("job: empty Path"))
	}

	cmd := exec.CommandContext(tctx, spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	if spec.Env != nil {
		cmd.Env = spec.Env
	}
	if len(spec.Stdin) > 0 {
		cmd.Stdin = bytes.NewReader(spec.Stdin)
	}
	cmd.Cancel = func() error { return kill(cmd) }
	cmd.WaitDelay = waitDelay
	prepare(cmd)

	// The child writes to OS pipes we own (not to a Writer), so cmd.Wait
	// returns as soon as the process itself exits, even when a descendant
	// still holds the pipes. The deadline and cancellation are judged at that
	// moment, not while we drain the output afterwards.
	outR, outW, err := os.Pipe()
	if err != nil {
		return fail(fmt.Errorf("job: stdout pipe: %w", err))
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		outR.Close()
		outW.Close()
		return fail(fmt.Errorf("job: stderr pipe: %w", err))
	}
	cmd.Stdout = outW
	cmd.Stderr = errW

	startErr := cmd.Start()
	// The parent's copies of the write ends must go, or the readers never see EOF.
	outW.Close()
	errW.Close()
	if startErr != nil {
		outR.Close()
		errR.Close()
		return fail(fmt.Errorf("job: start %q: %w", spec.Path, startErr))
	}

	var (
		outBuf, errBuf syncBuffer
		sinkMu         sync.Mutex
		readers        sync.WaitGroup
	)
	sink := lineSink{mu: &sinkMu, id: id, onLine: onLine}
	readers.Add(2)
	go func() { defer readers.Done(); collect(outR, Stdout, sink, &outBuf) }()
	go func() { defer readers.Done(); collect(errR, Stderr, sink, &errBuf) }()

	go func() {
		waitErr := cmd.Wait()
		// Decide timeout / cancel from the state at the process's exit.
		ctxErr := tctx.Err()

		// Drain the output. A descendant holding the pipes open is given
		// waitDelay, then the read ends are closed and what was read is kept.
		drained := make(chan struct{})
		go func() { readers.Wait(); close(drained) }()
		select {
		case <-drained:
		case <-time.After(waitDelay):
			outR.Close()
			errR.Close()
		}

		res := Result{
			ID: id, Name: spec.Name, ExitCode: -1,
			Stdout: outBuf.Bytes(), Stderr: errBuf.Bytes(),
			StartedAt: started,
		}
		switch {
		case errors.Is(ctxErr, context.DeadlineExceeded):
			res.State = TimedOut
		case ctxErr != nil:
			res.State = Canceled
		case cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0 &&
			(waitErr == nil || errors.Is(waitErr, exec.ErrWaitDelay)):
			res.State = Succeeded
			res.ExitCode = 0
		default:
			res.State = Failed
			if cmd.ProcessState != nil {
				res.ExitCode = cmd.ProcessState.ExitCode()
			}
		}
		res.EndedAt = time.Now()
		release()
		j.result = res
		r.record(res)
		close(j.done)
	}()

	return j, nil
}
