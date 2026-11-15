package atelier

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// ConnStatus is how the connection to Atelier stands.
type ConnStatus string

const (
	StatusReady             ConnStatus = "ready"
	StatusPythonUnavailable ConnStatus = "python_unavailable"
	StatusUnsupported       ConnStatus = "unsupported"
)

// ProbeResult is the answer of Probe. Code and Diagnostic say what is missing
// when Status is not ready.
type ProbeResult struct {
	Status        ConnStatus `json:"status"`
	Python        string     `json:"python"`
	PythonVersion string     `json:"python_version"`
	Home          string     `json:"home"`
	Code          ErrorCode  `json:"code"`
	Diagnostic    string     `json:"diagnostic"`
}

// Minimum Python version Atelier's desk scripts need.
const (
	minPythonMajor = 3
	minPythonMinor = 10
)

// exitCommandNotFound is what cmd.exe returns for an unknown command (and what
// the Microsoft Store python stub returns when Python is not installed).
const exitCommandNotFound = 9009

// FindPython returns the interpreter to run. A configured path is used as is.
// Otherwise it looks for python, then python3, then (on Windows) py in PATH.
func FindPython(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	names := []string{"python", "python3"}
	if runtime.GOOS == "windows" {
		names = append(names, "py")
	}
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p, nil
		}
	}
	return "", &Error{Code: ErrPythonNotFound, Diagnostic: "not found in PATH: " + strings.Join(names, ", ")}
}

// parsePythonVersion reads "3.13.14" (extra text after the third number and
// trailing whitespace are ignored).
func parsePythonVersion(s string) (major, minor int, full string, ok bool) {
	full = strings.TrimSpace(s)
	parts := strings.Split(full, ".")
	if len(parts) < 2 {
		return 0, 0, full, false
	}
	var err1, err2 error
	major, err1 = strconv.Atoi(parts[0])
	minor, err2 = strconv.Atoi(parts[1])
	return major, minor, full, err1 == nil && err2 == nil
}

// Probe checks Python (found, 3.10 or newer) and calls desk_state.py once to
// see that Atelier answers with atelier.state/1. It reports the outcome in the
// result; the error is non-nil only when ctx ended.
func (c *CLIClient) Probe(ctx context.Context) (ProbeResult, error) {
	pr := ProbeResult{Home: c.cfg.Home}
	unavailable := func(code ErrorCode, diag string) (ProbeResult, error) {
		pr.Status, pr.Code, pr.Diagnostic = StatusPythonUnavailable, code, diag
		return pr, nil
	}
	unsupported := func(code ErrorCode, diag string) (ProbeResult, error) {
		pr.Status, pr.Code, pr.Diagnostic = StatusUnsupported, code, diag
		return pr, nil
	}
	// fromErr maps a run error: a dead context is returned, the rest is reported.
	fromErr := func(err error, report func(ErrorCode, string) (ProbeResult, error)) (ProbeResult, error) {
		var e *Error
		if errors.As(err, &e) {
			return report(e.Code, e.Diagnostic)
		}
		return pr, err
	}

	python, err := FindPython(c.cfg.Python)
	if err != nil {
		return unavailable(CodeOf(err), diagOf(err))
	}
	pr.Python = python

	res, err := c.run(ctx, python, "python version", []string{"-c", "import sys;print('%d.%d.%d'%sys.version_info[:3])"})
	if err != nil {
		return fromErr(err, unavailable)
	}
	if res.ExitCode == exitCommandNotFound {
		return unavailable(ErrPythonNotFound, firstNonEmpty(stderrText(res), "exit code 9009"))
	}
	if res.ExitCode != 0 {
		return unavailable(ErrPythonNotFound, firstNonEmpty(stderrText(res), fmt.Sprintf("exit code %d", res.ExitCode)))
	}
	major, minor, full, ok := parsePythonVersion(string(res.Stdout))
	if !ok {
		return unavailable(ErrPythonNotFound, "no Python version in the output: "+full)
	}
	pr.PythonVersion = full
	if major < minPythonMajor || (major == minPythonMajor && minor < minPythonMinor) {
		return unavailable(ErrPythonTooOld, fmt.Sprintf("Python %s (need %d.%d or newer)", full, minPythonMajor, minPythonMinor))
	}

	if err := c.checkScript("desk_state.py"); err != nil {
		return unsupported(CodeOf(err), diagOf(err))
	}
	args := []string{c.scriptPath("desk_state.py")}
	if c.cfg.Home != "" {
		args = append(args, "--home", c.cfg.Home)
	}
	res, err = c.run(ctx, python, "desk_state.py", args)
	if err != nil {
		return fromErr(err, unavailable)
	}
	switch res.ExitCode {
	case 0:
	case 1:
		// Exit 1 is unused by Atelier's contract; in practice it is an import failure.
		return unavailable(ErrPythonModule, firstNonEmpty(stderrText(res), "exit code 1"))
	default:
		e := exitError(res, false).(*Error)
		return unsupported(e.Code, e.Diagnostic)
	}
	st, err := parseState(res.Stdout)
	if err != nil {
		return unsupported(CodeOf(err), diagOf(err))
	}
	if st.Home != "" {
		pr.Home = st.Home
	}
	pr.Status = StatusReady
	return pr, nil
}

func diagOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Diagnostic
	}
	return err.Error()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
