package atelier

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The fake Python is this test binary. TestMain turns the process into the fake
// before the testing flags are parsed (the Python-style arguments `-c ...` and
// `<script> ...` would be rejected as unknown flags); TestHelperProcess is the
// same entry for a run through `-test.run`. The fake is steered by environment
// variables, which the child inherits:
//
//	GO_WANT_HELPER_PROCESS=1  act as the fake Python
//	FAKE_PY_VERSION           stdout of `python -c <version program>`
//	FAKE_PY_VERSION_EXIT      its exit code
//	FAKE_STDOUT, FAKE_STDERR  stdout / stderr of a desk script
//	FAKE_EXIT                 its exit code
//	FAKE_INPUT_DUMP           file the fake writes the --input path and bytes to
const helperEnv = "GO_WANT_HELPER_PROCESS"

func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == "1" {
		os.Exit(fakePython(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		return
	}
	os.Exit(fakePython(nil))
}

func fakePython(args []string) int {
	if len(args) > 0 && args[0] == "-c" {
		os.Stdout.WriteString(os.Getenv("FAKE_PY_VERSION"))
		return atoiOr(os.Getenv("FAKE_PY_VERSION_EXIT"), 0)
	}
	for i, a := range args {
		if a == "--input" && i+1 < len(args) {
			dump := os.Getenv("FAKE_INPUT_DUMP")
			body, err := os.ReadFile(args[i+1])
			if dump != "" {
				out := append([]byte(args[i+1]+"\n"), body...)
				if err != nil {
					out = []byte(args[i+1] + "\nREAD ERROR: " + err.Error())
				}
				os.WriteFile(dump, out, 0o600)
			}
		}
	}
	os.Stdout.WriteString(os.Getenv("FAKE_STDOUT"))
	os.Stderr.WriteString(os.Getenv("FAKE_STDERR"))
	return atoiOr(os.Getenv("FAKE_EXIT"), 0)
}

func atoiOr(s string, def int) int {
	n := 0
	if s == "" {
		return def
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	return n
}

const (
	componentsV1 = `{"schema":"atelier.components/1","items":[{"id":"c1","source":"bundled","data":{"id":"c1"}}]}`
	stateV1      = `{"schema":"atelier.state/1","home":"/fake/home","runners":{},"routing":{"source":"default","table":{"name":"t","tiers":{}}}}`
	invalidV1    = `{"schema":"atelier.validation/1","kind":"workflow","valid":false,"problems":["no stages","bad name"]}`
	validV1      = `{"schema":"atelier.validation/1","kind":"workflow","valid":true,"problems":[]}`
)

// fakeVersionDir returns a version directory holding empty desk scripts.
func fakeVersionDir(t *testing.T, scripts ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, s := range scripts {
		if err := os.WriteFile(filepath.Join(dir, s), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

var allScripts = []string{"desk_catalog.py", "desk_state.py", "desk_validate.py", "desk_plan.py"}

// fakeClient builds a CLIClient whose Python is this test binary.
func fakeClient(t *testing.T, versionDir string) *CLIClient {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(helperEnv, "1")
	return NewCLIClient(nil, Config{
		Python:     exe,
		VersionDir: versionDir,
		Home:       t.TempDir(),
		Timeout:    time.Minute,
	})
}

// fakeScript sets what the next desk script prints and exits with.
func fakeScript(t *testing.T, stdout, stderr string, exit string) {
	t.Helper()
	t.Setenv("FAKE_STDOUT", stdout)
	t.Setenv("FAKE_STDERR", stderr)
	t.Setenv("FAKE_EXIT", exit)
}

func TestComponentsUnsupportedSchema(t *testing.T) {
	c := fakeClient(t, fakeVersionDir(t, allScripts...))

	fakeScript(t, componentsV1, "", "0")
	items, err := c.Components(context.Background())
	if err != nil {
		t.Fatalf("Components with atelier.components/1: %v", err)
	}
	if len(items) != 1 || items[0].ID != "c1" {
		t.Fatalf("items = %+v, want the one component c1", items)
	}

	fakeScript(t, `{"schema":"atelier.components/2","items":[]}`, "", "0")
	_, err = c.Components(context.Background())
	if got := CodeOf(err); got != ErrUnsupported {
		t.Fatalf("code = %q (err %v), want %q", got, err, ErrUnsupported)
	}
}

func TestMalformedAndExitCodes(t *testing.T) {
	c := fakeClient(t, fakeVersionDir(t, allScripts...))
	cases := []struct {
		name   string
		stdout string
		stderr string
		exit   string
		code   ErrorCode
		exitNo int
		diag   string
	}{
		{"not JSON", "this is not json", "", "0", ErrMalformed, 0, "*"},
		{"bad args", "", "usage: unknown option\n", "2", ErrBadArgs, 2, "usage: unknown option"},
		{"input missing", "", "no such input file\n", "3", ErrInputMissing, 3, "no such input file"},
		{"broken", "", "Traceback: boom\n", "4", ErrBroken, 4, "Traceback: boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeScript(t, tc.stdout, tc.stderr, tc.exit)
			_, err := c.Components(context.Background())
			if err == nil {
				t.Fatal("want an error")
			}
			if got := CodeOf(err); got != tc.code {
				t.Fatalf("code = %q (err %v), want %q", got, err, tc.code)
			}
			e := err.(*Error)
			if e.ExitCode != tc.exitNo {
				t.Errorf("ExitCode = %d, want %d", e.ExitCode, tc.exitNo)
			}
			// "*": any non-empty text (the JSON parser's own message).
			if tc.diag == "*" {
				if e.Diagnostic == "" {
					t.Error("Diagnostic is empty, want the parser's message")
				}
			} else if e.Diagnostic != tc.diag {
				t.Errorf("Diagnostic = %q, want %q", e.Diagnostic, tc.diag)
			}
		})
	}
}

func TestValidateExit1IsNotAnError(t *testing.T) {
	c := fakeClient(t, fakeVersionDir(t, allScripts...))
	req := Request{Kind: "workflow", Name: "w", Input: []byte(`{"stages":{}}`)}

	fakeScript(t, invalidV1, "", "1")
	v, err := c.Validate(context.Background(), req)
	if err != nil {
		t.Fatalf("Validate with exit 1: %v", err)
	}
	if v.Valid {
		t.Error("Valid = true, want false")
	}
	if len(v.Problems) != 2 || v.Problems[0] != "no stages" || v.Problems[1] != "bad name" {
		t.Errorf("Problems = %q, want [no stages, bad name]", v.Problems)
	}

	fakeScript(t, validV1, "", "0")
	v, err = c.Validate(context.Background(), req)
	if err != nil || !v.Valid {
		t.Fatalf("Validate with exit 0 = %+v, %v; want valid", v, err)
	}
}

func TestProbePythonMissingOrTooOld(t *testing.T) {
	ctx := context.Background()
	dir := fakeVersionDir(t, allScripts...)

	t.Run("configured Python does not exist", func(t *testing.T) {
		c := NewCLIClient(nil, Config{Python: filepath.Join(t.TempDir(), "no-such-python"), VersionDir: dir})
		pr, err := c.Probe(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if pr.Status != StatusPythonUnavailable || pr.Code != ErrPythonNotFound {
			t.Fatalf("Probe = %q/%q, want python_unavailable/python_not_found", pr.Status, pr.Code)
		}
	})

	t.Run("no Python in PATH", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		c := NewCLIClient(nil, Config{VersionDir: dir})
		pr, err := c.Probe(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if pr.Status != StatusPythonUnavailable || pr.Code != ErrPythonNotFound {
			t.Fatalf("Probe = %q/%q, want python_unavailable/python_not_found", pr.Status, pr.Code)
		}
	})

	t.Run("Python 3.9 is too old", func(t *testing.T) {
		c := fakeClient(t, dir)
		t.Setenv("FAKE_PY_VERSION", "3.9.18\n")
		pr, err := c.Probe(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if pr.Status != StatusPythonUnavailable || pr.Code != ErrPythonTooOld {
			t.Fatalf("Probe = %q/%q, want python_unavailable/python_too_old", pr.Status, pr.Code)
		}
		if pr.PythonVersion != "3.9.18" {
			t.Errorf("PythonVersion = %q, want 3.9.18", pr.PythonVersion)
		}
	})

	t.Run("Python 3.10 is accepted", func(t *testing.T) {
		c := fakeClient(t, dir)
		t.Setenv("FAKE_PY_VERSION", "3.10.0\n")
		fakeScript(t, stateV1, "", "0")
		pr, err := c.Probe(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if pr.Status != StatusReady {
			t.Fatalf("Probe = %q/%q (%s), want ready", pr.Status, pr.Code, pr.Diagnostic)
		}
	})
}

func TestProbeNoDeskStateIsUnsupported(t *testing.T) {
	c := fakeClient(t, fakeVersionDir(t, "desk_catalog.py"))
	t.Setenv("FAKE_PY_VERSION", "3.12.1\n")
	pr, err := c.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if pr.Status != StatusUnsupported || pr.Code != ErrUnsupported {
		t.Fatalf("Probe = %q/%q, want unsupported/unsupported", pr.Status, pr.Code)
	}
}

func TestPlanExit1ReturnsInvalidValidation(t *testing.T) {
	c := fakeClient(t, fakeVersionDir(t, allScripts...))
	fakeScript(t, invalidV1, "", "1")
	res, err := c.Plan(context.Background(), Request{Kind: "workflow", Name: "w", Input: []byte(`{}`)})
	if err != nil {
		t.Fatalf("Plan with exit 1: %v (code %q)", err, CodeOf(err))
	}
	if res.Validation.Valid {
		t.Error("Validation.Valid = true, want false")
	}
	if len(res.Validation.Problems) != 2 {
		t.Errorf("Problems = %q, want 2 problems", res.Validation.Problems)
	}
	if res.Plan != nil {
		t.Errorf("Plan = %+v, want nil", res.Plan)
	}
}

func TestProbeExit9009IsPythonNotFound(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("exit code 9009 exists on Windows only (other systems keep the low 8 bits)")
	}
	c := fakeClient(t, fakeVersionDir(t, allScripts...))
	t.Setenv("FAKE_PY_VERSION", "")
	t.Setenv("FAKE_PY_VERSION_EXIT", "9009")
	pr, err := c.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if pr.Status != StatusPythonUnavailable || pr.Code != ErrPythonNotFound {
		t.Fatalf("Probe = %q/%q, want python_unavailable/python_not_found", pr.Status, pr.Code)
	}
}

func TestPlanAppendOpIsUnsupported(t *testing.T) {
	c := fakeClient(t, fakeVersionDir(t, allScripts...))
	req := Request{Kind: "workflow", Name: "w", Input: []byte(`{}`)}
	plan := func(op string) string {
		return `{"schema":"atelier.save-plan/1","kind":"workflow","evaluated_at":"2026-01-01T00:00:00Z","reads":[],` +
			`"writes":[{"op":"` + op + `","path":"/fake/home/workflows/w.json","content_utf8":"{}"}]}`
	}

	for _, op := range []string{"replace", "delete"} {
		fakeScript(t, plan(op), "", "0")
		res, err := c.Plan(context.Background(), req)
		if err != nil {
			t.Fatalf("op %q: %v", op, err)
		}
		if res.Plan == nil || len(res.Plan.Writes) != 1 || res.Plan.Writes[0].Op != op || !res.Validation.Valid {
			t.Fatalf("op %q: result = %+v", op, res)
		}
	}

	fakeScript(t, plan("append"), "", "0")
	_, err := c.Plan(context.Background(), req)
	if got := CodeOf(err); got != ErrUnsupported {
		t.Fatalf("op append: code = %q (err %v), want %q", got, err, ErrUnsupported)
	}
}

func TestInputTempFileNoBOMAndRemoved(t *testing.T) {
	c := fakeClient(t, fakeVersionDir(t, allScripts...))
	bom := "\xef\xbb\xbf"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"BOM is dropped", bom + `{"a":1}`, `{"a":1}`},
		{"plain body is kept", `{"b":[1,2]}`, `{"b":[1,2]}`},
		{"empty input is an empty object", "", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dump := filepath.Join(t.TempDir(), "input.dump")
			t.Setenv("FAKE_INPUT_DUMP", dump)
			fakeScript(t, validV1, "", "0")
			if _, err := c.Validate(context.Background(), Request{Kind: "workflow", Name: "w", Input: []byte(tc.in)}); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(dump)
			if err != nil {
				t.Fatalf("the fake Python did not get an --input file: %v", err)
			}
			path, body, ok := bytes.Cut(raw, []byte("\n"))
			if !ok || strings.HasPrefix(string(body), "READ ERROR") {
				t.Fatalf("the fake Python could not read --input: %q", raw)
			}
			if bytes.HasPrefix(body, []byte(bom)) {
				t.Errorf("the --input file starts with a BOM: %q", body)
			}
			if string(body) != tc.want {
				t.Errorf("the --input file = %q, want %q", body, tc.want)
			}
			if _, err := os.Stat(string(path)); !os.IsNotExist(err) {
				t.Errorf("the --input file %s still exists after the call (stat err: %v)", path, err)
			}
		})
	}
}
