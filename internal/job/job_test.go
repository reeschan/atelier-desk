package job

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestHelperProcess is not a real test: it is the child process the tests
// start (the test binary itself). It does nothing unless GO_WANT_HELPER_PROCESS=1.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	switch os.Getenv("GO_HELPER_MODE") {
	case "sleep":
		time.Sleep(60 * time.Second)
	case "line-sleep":
		fmt.Println("hello")
		time.Sleep(60 * time.Second)
	case "line-exit":
		fmt.Println("bye")
	case "exit3":
		os.Exit(3)
	case "grandchild":
		// Longer than the test Timeout (500ms), shorter than waitDelay (2s),
		// so it is gone before the test binary exits.
		time.Sleep(1 * time.Second)
	case "spawn-exit":
		// Leave a grandchild holding stdout, then exit 0 at once.
		self, _ := os.Executable()
		c := exec.Command(self, "-test.run=^TestHelperProcess$")
		c.Env = append(os.Environ(), "GO_HELPER_MODE=grandchild")
		c.Stdout = os.Stdout
		_ = c.Start()
		fmt.Println("done")
	}
	os.Exit(0)
}

func helperSpec(t *testing.T, mode string) Spec {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Spec{
		Name: mode,
		Path: self,
		Args: []string{"-test.run=^TestHelperProcess$"},
		Env:  append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "GO_HELPER_MODE="+mode),
	}
}

// waitResult waits for the job with a generous deadline.
func waitResult(t *testing.T, j *Job) Result {
	t.Helper()
	ch := make(chan Result, 1)
	go func() { ch <- j.Wait() }()
	select {
	case r := <-ch:
		return r
	case <-time.After(30 * time.Second):
		t.Fatal("job did not finish in time")
		return Result{}
	}
}

// lineCollector gathers onLine calls and lets a test wait for the first one.
type lineCollector struct {
	mu    sync.Mutex
	lines []string
	first chan struct{}
	once  sync.Once
}

func newLineCollector() *lineCollector { return &lineCollector{first: make(chan struct{})} }

func (c *lineCollector) onLine(id string, s Stream, line string) {
	c.mu.Lock()
	c.lines = append(c.lines, line)
	c.mu.Unlock()
	c.once.Do(func() { close(c.first) })
}

func (c *lineCollector) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.lines...)
}

func TestCancelSetsCanceled(t *testing.T) {
	r := NewRunner(5)
	c := newLineCollector()
	j, err := r.Start(context.Background(), helperSpec(t, "line-sleep"), c.onLine)
	if err != nil {
		t.Fatal(err)
	}
	// Once the child has printed, it is certainly alive and sleeping.
	select {
	case <-c.first:
	case <-time.After(30 * time.Second):
		t.Fatal("child never printed")
	}
	j.Cancel()
	j.Cancel() // safe to call again
	res := waitResult(t, j)
	if res.State != Canceled {
		t.Fatalf("State = %q, want %q", res.State, Canceled)
	}
	if res.ID != j.ID() {
		t.Errorf("Result.ID = %q, want %q", res.ID, j.ID())
	}
}

func TestTimeoutSetsTimedOut(t *testing.T) {
	r := NewRunner(5)
	spec := helperSpec(t, "sleep")
	spec.Timeout = 100 * time.Millisecond
	start := time.Now()
	res := r.Run(context.Background(), spec)
	if res.State != TimedOut {
		t.Fatalf("State = %q, want %q", res.State, TimedOut)
	}
	if d := time.Since(start); d > 30*time.Second {
		t.Errorf("took %s, the sleeping child was not stopped", d)
	}
}

func TestOnLineStreamsBeforeExit(t *testing.T) {
	r := NewRunner(5)
	c := newLineCollector()
	j, err := r.Start(context.Background(), helperSpec(t, "line-sleep"), c.onLine)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-c.first:
	case <-time.After(30 * time.Second):
		t.Fatal("onLine did not receive the line while the child was still running")
	}
	// The child is still sleeping, so the job has not ended.
	waited := make(chan Result, 1)
	go func() { waited <- j.Wait() }()
	select {
	case <-waited:
		t.Fatal("job ended before the child exited")
	case <-time.After(200 * time.Millisecond):
	}
	if got := c.snapshot(); len(got) == 0 || got[0] != "hello" {
		t.Errorf("lines = %q, want first line %q", got, "hello")
	}
	j.Cancel()
	var res Result
	select {
	case res = <-waited:
	case <-time.After(30 * time.Second):
		t.Fatal("job did not finish after Cancel")
	}
	if !strings.Contains(string(res.Stdout), "hello") {
		t.Errorf("Stdout = %q, want it to contain hello", res.Stdout)
	}
}

func TestHistoryNewestFirstAndLimit(t *testing.T) {
	r := NewRunner(2)
	var ids []string
	for i := 0; i < 3; i++ {
		spec := helperSpec(t, "line-exit")
		spec.Name = fmt.Sprintf("run-%d", i)
		res := r.Run(context.Background(), spec)
		if res.State != Succeeded {
			t.Fatalf("run %d: State = %q, want %q", i, res.State, Succeeded)
		}
		ids = append(ids, res.ID)
	}
	h := r.History()
	if len(h) != 2 {
		t.Fatalf("len(History) = %d, want 2", len(h))
	}
	if h[0].ID != ids[2] || h[1].ID != ids[1] {
		t.Errorf("History IDs = [%s %s], want newest first [%s %s]", h[0].ID, h[1].ID, ids[2], ids[1])
	}

	for _, limit := range []int{0, -1} {
		r0 := NewRunner(limit)
		r0.Run(context.Background(), helperSpec(t, "line-exit"))
		if got := r0.History(); len(got) != 0 {
			t.Errorf("NewRunner(%d): len(History) = %d, want 0", limit, len(got))
		}
	}
}

func TestStartFailureReturnsFailedJob(t *testing.T) {
	r := NewRunner(5)
	j, err := r.Start(context.Background(), Spec{Name: "nope", Path: "/definitely/not/a/real/executable"}, nil)
	if err == nil {
		t.Fatal("Start error = nil, want an error")
	}
	if j == nil {
		t.Fatal("Start returned a nil Job")
	}
	res := waitResult(t, j)
	if res.State != Failed {
		t.Errorf("State = %q, want %q", res.State, Failed)
	}
	if res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", res.ExitCode)
	}
	if res.Err == "" {
		t.Error("Err is empty, want the start error")
	}
	if h := r.History(); len(h) != 1 || h[0].State != Failed {
		t.Errorf("History = %+v, want the failed job recorded", h)
	}
}

// A child that exited 0 before the deadline must be Succeeded even when a
// descendant keeps the output pipe open past the Timeout.
// (fails now: job.go:262 takes the state from ctx.Err() after the drain.)
func TestExitedChildIsNotTimedOut(t *testing.T) {
	r := NewRunner(1)
	spec := helperSpec(t, "spawn-exit")
	spec.Timeout = 500 * time.Millisecond
	res := r.Run(context.Background(), spec)
	if res.State != Succeeded {
		t.Fatalf("State = %q (exit %d), want %q: the child exited 0 before the deadline", res.State, res.ExitCode, Succeeded)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
}
