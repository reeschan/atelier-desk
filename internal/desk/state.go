// Package desk is the one boundary the screen uses (Wails binds Service only).
// It holds the state machine of the connection between the app and Atelier,
// refuses version switches and Python changes while a save is in progress, and
// streams file changes, job output and connection changes as Wails events.
// Go never writes a sentence for the user: errors are codes plus raw details.
package desk

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/reeschan/atelier-desk/internal/atelier"
	"github.com/reeschan/atelier-desk/internal/skills"
)

// ConnState is where the connection to Atelier stands.
type ConnState string

const (
	Checking          ConnState = "checking"
	Ready             ConnState = "ready"
	PythonUnavailable ConnState = "python_unavailable"
	Unsupported       ConnState = "unsupported"
)

// Connection is what the screen shows about the connection. Version is the
// Atelier version the app asks (never another one). Saving is true while a
// save is in progress; the screen disables version switching and the Python
// setting with it.
type Connection struct {
	State         ConnState `json:"state"`
	Version       string    `json:"version"`
	Python        string    `json:"python"`
	PythonVersion string    `json:"pythonVersion"`
	Home          string    `json:"home"`
	Code          string    `json:"code"`
	Diagnostic    string    `json:"diagnostic"`
	Saving        bool      `json:"saving"`
}

// Error codes of desk itself. Codes of the lower packages (atelier.ErrorCode,
// skills.ErrorCode) are passed through as they are.
const (
	ErrSaving         = "busy_saving"
	ErrNotReady       = "not_ready"
	ErrUnknownVersion = "unknown_version"
	ErrNotFound       = "not_found"
	ErrBadInput       = "bad_input"
	ErrInternal       = "internal"
)

// Error is what a bound method rejects with. Error() is the JSON of the value,
// so the screen reads {code, detail} back from the rejected promise.
type Error struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func (e *Error) Error() string {
	b, err := json.Marshal(e)
	if err != nil {
		return `{"code":"internal","detail":""}`
	}
	return string(b)
}

func newError(code, detail string) *Error { return &Error{Code: code, Detail: detail} }

// toError turns any error into a *Error, keeping the code of the lower layer.
func toError(err error) error {
	if err == nil {
		return nil
	}
	var de *Error
	if errors.As(err, &de) {
		return de
	}
	var ae *atelier.Error
	if errors.As(err, &ae) {
		return &Error{Code: string(ae.Code), Detail: ae.Diagnostic}
	}
	if c := skills.CodeOf(err); c != "" {
		return &Error{Code: string(c), Detail: err.Error()}
	}
	return &Error{Code: ErrInternal, Detail: err.Error()}
}

// machine is the connection state machine. A check has a generation number;
// the result of a check that is no longer the latest is dropped.
type machine struct {
	mu     sync.Mutex
	gen    int
	saving int
	conn   Connection
}

func newMachine() *machine {
	return &machine{conn: Connection{State: Checking}}
}

// beginCheck moves to Checking for version. It is refused while saving.
// Every call advances the generation.
func (m *machine) beginCheck(version string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saving > 0 {
		return 0, newError(ErrSaving, "")
	}
	m.gen++
	m.conn = Connection{State: Checking, Version: version}
	return m.gen, nil
}

// finishCheck applies the result of check gen. A result of an older check is
// dropped; the return value says whether it was applied.
func (m *machine) finishCheck(gen int, r atelier.ProbeResult) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if gen != m.gen || m.conn.State != Checking {
		return false
	}
	var st ConnState
	switch r.Status {
	case atelier.StatusReady:
		st = Ready
	case atelier.StatusPythonUnavailable:
		st = PythonUnavailable
	default:
		st = Unsupported
	}
	m.conn = Connection{
		State:         st,
		Version:       m.conn.Version,
		Python:        r.Python,
		PythonVersion: r.PythonVersion,
		Home:          r.Home,
		Code:          string(r.Code),
		Diagnostic:    r.Diagnostic,
		Saving:        m.saving > 0,
	}
	return true
}

// isCurrent reports whether gen is the latest check.
func (m *machine) isCurrent(gen int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return gen == m.gen
}

// generation returns the latest check.
func (m *machine) generation() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gen
}

// beginSave starts a save. Only Ready accepts it (never while checking, with
// Python unavailable or with an unsupported version).
func (m *machine) beginSave() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn.State != Ready {
		return newError(ErrNotReady, string(m.conn.State))
	}
	m.saving++
	m.conn.Saving = true
	return nil
}

// endSave ends a save started by beginSave.
func (m *machine) endSave() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saving > 0 {
		m.saving--
	}
	m.conn.Saving = m.saving > 0
}

// current returns a copy of the connection.
func (m *machine) current() Connection {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.conn
}
