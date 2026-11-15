// Package atelier is the one place that talks to Atelier. It runs the desk_*.py
// scripts that ship with an Atelier version as jobs, checks the "schema" of
// what comes back, and turns it into Go types. Errors carry an ErrorCode and
// Atelier's own diagnostic; this package never writes a sentence for the user.
package atelier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reeschan/atelier-desk/internal/job"
)

// Client is the seam between the app and Atelier. Tests replace it with a fake.
type Client interface {
	Probe(ctx context.Context) (ProbeResult, error)
	Components(ctx context.Context) ([]Component, error)
	Workflows(ctx context.Context) ([]Workflow, error)
	State(ctx context.Context) (State, error)
	Validate(ctx context.Context, req Request) (Validation, error)
	Plan(ctx context.Context, req Request) (PlanResult, error)
}

// Request names what Validate or Plan checks. Kind is passed to Atelier as is
// (Atelier rejects an unknown kind). An empty Name omits --name. Input is the
// JSON object body (empty means {}).
type Request struct {
	Kind  string          `json:"kind"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// Config tells where Atelier and Python are.
type Config struct {
	Python     string        // interpreter; empty = search PATH
	VersionDir string        // the Atelier version directory holding desk_*.py
	Home       string        // absolute Atelier home; empty only for State and Probe
	Timeout    time.Duration // per call; 0 = 30 seconds
}

const defaultTimeout = 30 * time.Second

// ErrorCode names why a call failed. The screen turns it into a sentence.
type ErrorCode string

const (
	ErrPythonNotFound ErrorCode = "python_not_found"
	ErrPythonTooOld   ErrorCode = "python_too_old"
	ErrPythonModule   ErrorCode = "python_module_missing"
	ErrUnsupported    ErrorCode = "unsupported"
	ErrBadArgs        ErrorCode = "bad_args"
	ErrInputMissing   ErrorCode = "input_missing"
	ErrBroken         ErrorCode = "atelier_broken"
	ErrMalformed      ErrorCode = "malformed_output"
	ErrTimeout        ErrorCode = "timeout"
	ErrHomeRequired   ErrorCode = "home_required"
)

// Error is a failed call to Atelier. Diagnostic is Atelier's (or the OS's) own
// text, untranslated.
type Error struct {
	Code       ErrorCode
	ExitCode   int
	Diagnostic string
}

func (e *Error) Error() string {
	if e.Diagnostic == "" {
		return "atelier: " + string(e.Code)
	}
	return "atelier: " + string(e.Code) + ": " + e.Diagnostic
}

// CodeOf returns the ErrorCode inside err, or "" when there is none.
func CodeOf(err error) ErrorCode {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// CLIClient runs the desk_*.py scripts through a job.Runner.
type CLIClient struct {
	runner *job.Runner
	cfg    Config
}

var _ Client = (*CLIClient)(nil)

// NewCLIClient returns a Client backed by the scripts in cfg.VersionDir.
func NewCLIClient(r *job.Runner, cfg Config) *CLIClient {
	if r == nil {
		r = job.NewRunner(0)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	return &CLIClient{runner: r, cfg: cfg}
}

// script paths: <VersionDir>/<name>.
func (c *CLIClient) scriptPath(name string) string {
	return filepath.Join(c.cfg.VersionDir, name)
}

// checkScript returns ErrUnsupported when the script is not in VersionDir.
func (c *CLIClient) checkScript(name string) error {
	p := c.scriptPath(name)
	if c.cfg.VersionDir == "" {
		return &Error{Code: ErrUnsupported, Diagnostic: "no Atelier version directory is set"}
	}
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		return &Error{Code: ErrUnsupported, Diagnostic: "missing " + p}
	}
	return nil
}

// run starts `<python> args...` and waits. It returns an *Error for a process
// that did not start (python_not_found), a timeout, and a cancelled context.
// Otherwise it returns the finished Result whatever its exit code.
func (c *CLIClient) run(ctx context.Context, python, label string, args []string) (job.Result, error) {
	res := c.runner.Run(ctx, job.Spec{
		Name:    label,
		Path:    python,
		Args:    args,
		Timeout: c.cfg.Timeout,
	})
	if res.Err != "" {
		return res, &Error{Code: ErrPythonNotFound, ExitCode: res.ExitCode, Diagnostic: res.Err}
	}
	switch res.State {
	case job.TimedOut:
		return res, &Error{Code: ErrTimeout, ExitCode: res.ExitCode, Diagnostic: label}
	case job.Canceled:
		if err := ctx.Err(); err != nil {
			return res, err
		}
		return res, context.Canceled
	}
	return res, nil
}

func stderrText(res job.Result) string {
	return strings.TrimSpace(strings.ToValidUTF8(string(res.Stderr), "�"))
}

// exitError maps a non-zero exit code to an *Error. allowOne says exit 1 is a
// normal answer (validate and plan).
func exitError(res job.Result, allowOne bool) error {
	switch res.ExitCode {
	case 0:
		return nil
	case 1:
		if allowOne {
			return nil
		}
	case 2:
		return &Error{Code: ErrBadArgs, ExitCode: 2, Diagnostic: stderrText(res)}
	case 3:
		return &Error{Code: ErrInputMissing, ExitCode: 3, Diagnostic: stderrText(res)}
	}
	d := stderrText(res)
	if d == "" {
		d = fmt.Sprintf("exit code %d", res.ExitCode)
	}
	return &Error{Code: ErrBroken, ExitCode: res.ExitCode, Diagnostic: d}
}

// call runs one desk script and returns its stdout and exit code. homeRequired
// says Config.Home must be set; --home is passed whenever it is set. allowOne
// accepts exit 1 as an answer (validate and plan).
func (c *CLIClient) call(ctx context.Context, script string, head []string, homeRequired, allowOne bool, tail []string) ([]byte, int, error) {
	if homeRequired && c.cfg.Home == "" {
		return nil, 0, &Error{Code: ErrHomeRequired}
	}
	if err := c.checkScript(script); err != nil {
		return nil, 0, err
	}
	python, err := FindPython(c.cfg.Python)
	if err != nil {
		return nil, 0, err
	}
	args := []string{c.scriptPath(script)}
	args = append(args, head...)
	if c.cfg.Home != "" {
		args = append(args, "--home", c.cfg.Home)
	}
	args = append(args, tail...)
	res, err := c.run(ctx, python, script, args)
	if err != nil {
		return nil, res.ExitCode, err
	}
	if err := exitError(res, allowOne); err != nil {
		return nil, res.ExitCode, err
	}
	return res.Stdout, res.ExitCode, nil
}

func decode(raw []byte, v any) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return &Error{Code: ErrMalformed, Diagnostic: err.Error()}
	}
	return nil
}

// Components returns every component (bundled and home).
func (c *CLIClient) Components(ctx context.Context) ([]Component, error) {
	out, _, err := c.call(ctx, "desk_catalog.py", []string{"components"}, true, false, nil)
	if err != nil {
		return nil, err
	}
	if err := checkSchema(out, SchemaComponents); err != nil {
		return nil, err
	}
	var w struct {
		Items []Component `json:"items"`
	}
	if err := decode(out, &w); err != nil {
		return nil, err
	}
	if w.Items == nil {
		w.Items = []Component{}
	}
	return w.Items, nil
}

// Workflows returns every workflow. One broken definition makes Atelier exit 4
// for the whole list; that comes back as ErrBroken with the diagnostic.
func (c *CLIClient) Workflows(ctx context.Context) ([]Workflow, error) {
	out, _, err := c.call(ctx, "desk_catalog.py", []string{"workflows"}, true, false, nil)
	if err != nil {
		return nil, err
	}
	if err := checkSchema(out, SchemaWorkflows); err != nil {
		return nil, err
	}
	var w struct {
		Items []wireWorkflow `json:"items"`
	}
	if err := decode(out, &w); err != nil {
		return nil, err
	}
	res := make([]Workflow, 0, len(w.Items))
	for _, it := range w.Items {
		wf, err := buildWorkflow(it)
		if err != nil {
			return nil, &Error{Code: ErrMalformed, Diagnostic: "workflow " + it.Name + ": " + err.Error()}
		}
		res = append(res, wf)
	}
	return res, nil
}

// State returns Atelier's state. Without Config.Home, --home is left out and
// Atelier resolves the home itself.
func (c *CLIClient) State(ctx context.Context) (State, error) {
	out, _, err := c.call(ctx, "desk_state.py", nil, false, false, nil)
	if err != nil {
		return State{}, err
	}
	return parseState(out)
}

func parseState(out []byte) (State, error) {
	if err := checkSchema(out, SchemaState); err != nil {
		return State{}, err
	}
	var st State
	if err := decode(out, &st); err != nil {
		return State{}, err
	}
	st.Raw = append(json.RawMessage(nil), bytes.TrimSpace(out)...)
	return st, nil
}

// withInput writes the request body to a BOM-free temp file, runs fn with its
// path as `--input`, and removes the file afterwards.
func withInput(body json.RawMessage, fn func(tail []string) error) error {
	body = bytes.TrimPrefix(bytes.TrimSpace(body), []byte("\xef\xbb\xbf"))
	if len(body) == 0 {
		body = []byte("{}")
	}
	f, err := os.CreateTemp("", "atelier-desk-*.json")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err := f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return fn([]string{"--input", path})
}

func (c *CLIClient) kindCall(ctx context.Context, script string, req Request) ([]byte, int, error) {
	var (
		out  []byte
		code int
	)
	err := withInput(req.Input, func(tail []string) error {
		if req.Name != "" {
			tail = append(tail, "--name", req.Name)
		}
		var err error
		out, code, err = c.call(ctx, script, []string{req.Kind}, true, true, tail)
		return err
	})
	return out, code, err
}

func parseValidation(out []byte) (Validation, error) {
	if err := checkSchema(out, SchemaValidation); err != nil {
		return Validation{}, err
	}
	var v Validation
	if err := decode(out, &v); err != nil {
		return Validation{}, err
	}
	return v, nil
}

// Validate asks Atelier whether the body would be accepted. Exit 1 ("not
// valid") is an answer, not an error.
func (c *CLIClient) Validate(ctx context.Context, req Request) (Validation, error) {
	out, _, err := c.kindCall(ctx, "desk_validate.py", req)
	if err != nil {
		return Validation{}, err
	}
	return parseValidation(out)
}

// Plan asks Atelier what saving the body would write. Exit 1 comes back as an
// invalid PlanResult with no plan and no error. The plan is not applied here.
func (c *CLIClient) Plan(ctx context.Context, req Request) (PlanResult, error) {
	out, code, err := c.kindCall(ctx, "desk_plan.py", req)
	if err != nil {
		return PlanResult{}, err
	}
	if code == 1 {
		v, err := parseValidation(out)
		if err != nil {
			return PlanResult{}, err
		}
		v.Valid = false
		return PlanResult{Validation: v}, nil
	}
	if err := checkSchema(out, SchemaSavePlan); err != nil {
		return PlanResult{}, err
	}
	var p SavePlan
	if err := decode(out, &p); err != nil {
		return PlanResult{}, err
	}
	for _, w := range p.Writes {
		if w.Op != opReplace && w.Op != opDelete {
			return PlanResult{}, &Error{Code: ErrUnsupported, Diagnostic: fmt.Sprintf("write op %q for %q", w.Op, w.Path)}
		}
	}
	return PlanResult{Validation: Validation{Kind: p.Kind, Valid: true, Problems: []string{}}, Plan: &p}, nil
}
