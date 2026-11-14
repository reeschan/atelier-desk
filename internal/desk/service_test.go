package desk

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/reeschan/atelier-desk/internal/appconfig"
	"github.com/reeschan/atelier-desk/internal/atelier"
	"github.com/reeschan/atelier-desk/internal/homestore"
)

const waitLimit = 5 * time.Second

// scriptedProbe is the Probe of one version of a fake Atelier. A Probe stops
// until the test sends its answer, so the test decides when a check ends.
type scriptedProbe struct {
	entered  chan atelier.Config
	answers  chan atelier.ProbeResult
	returned chan struct{}
}

func newScriptedProbe() *scriptedProbe {
	return &scriptedProbe{
		entered:  make(chan atelier.Config, 16),
		answers:  make(chan atelier.ProbeResult, 16),
		returned: make(chan struct{}, 16),
	}
}

func (p *scriptedProbe) run(cfg atelier.Config) (atelier.ProbeResult, error) {
	p.entered <- cfg
	r := <-p.answers
	p.returned <- struct{}{}
	return r, nil
}

// waitEntered waits until a Probe has started and returns the Config of its client.
func (p *scriptedProbe) waitEntered(t *testing.T) atelier.Config {
	t.Helper()
	select {
	case cfg := <-p.entered:
		return cfg
	case <-time.After(waitLimit):
		t.Fatal("the Probe did not start")
		return atelier.Config{}
	}
}

func (p *scriptedProbe) waitReturned(t *testing.T) {
	t.Helper()
	select {
	case <-p.returned:
	case <-time.After(waitLimit):
		t.Fatal("the Probe did not return")
	}
}

// fakeEnv is what the fake clients share.
type fakeEnv struct {
	mu         sync.Mutex
	probes     map[string]*scriptedProbe // by version name
	validate   func(ctx context.Context, req atelier.Request) (atelier.Validation, error)
	cfgs       []atelier.Config // every Config NewClient was given
	validated  int
	unexpected []string
}

type fakeClient struct {
	env *fakeEnv
	cfg atelier.Config
}

func (c *fakeClient) Probe(context.Context) (atelier.ProbeResult, error) {
	v := filepath.Base(c.cfg.VersionDir)
	c.env.mu.Lock()
	p := c.env.probes[v]
	if p == nil {
		c.env.unexpected = append(c.env.unexpected, "Probe of version "+v)
	}
	c.env.mu.Unlock()
	if p == nil {
		return atelier.ProbeResult{}, errors.New("unexpected Probe")
	}
	return p.run(c.cfg)
}

func (c *fakeClient) Components(context.Context) ([]atelier.Component, error) { return nil, nil }
func (c *fakeClient) Workflows(context.Context) ([]atelier.Workflow, error)   { return nil, nil }
func (c *fakeClient) State(context.Context) (atelier.State, error)            { return atelier.State{}, nil }

func (c *fakeClient) Validate(ctx context.Context, req atelier.Request) (atelier.Validation, error) {
	c.env.mu.Lock()
	c.env.validated++
	fn := c.env.validate
	c.env.mu.Unlock()
	if fn != nil {
		return fn(ctx, req)
	}
	return atelier.Validation{Valid: false, Problems: []string{"stub"}}, nil
}

func (c *fakeClient) Plan(context.Context, atelier.Request) (atelier.PlanResult, error) {
	return atelier.PlanResult{}, nil
}

// recorder keeps what Emit was given.
type recorder struct {
	mu     sync.Mutex
	events []Connection // connection:changed payloads
	other  []string     // names of the other events
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == EventConnectionChanged {
		if c, ok := data.(Connection); ok {
			r.events = append(r.events, c)
			return
		}
	}
	r.other = append(r.other, name)
}

func (r *recorder) snapshot() []Connection {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Connection(nil), r.events...)
}

func (r *recorder) states() []ConnState {
	var out []ConnState
	for _, c := range r.snapshot() {
		out = append(out, c.State)
	}
	return out
}

// waitEvent waits for a connection:changed whose payload satisfies pred and
// returns it.
func (r *recorder) waitEvent(t *testing.T, what string, pred func(Connection) bool) Connection {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for {
		for _, c := range r.snapshot() {
			if pred(c) {
				return c
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no connection:changed for %s; states so far: %v", what, r.states())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (r *recorder) waitState(t *testing.T, st ConnState, version string) Connection {
	t.Helper()
	return r.waitEvent(t, string(st)+" "+version, func(c Connection) bool {
		return c.State == st && c.Version == version
	})
}

// fixture is a Service on a temporary user home with fake versions.
type fixture struct {
	t          *testing.T
	root       string
	userHome   string
	home       string // the Atelier home the fake Probe reports
	cfgPath    string
	atelierDir string
	env        *fakeEnv
	rec        *recorder
	svc        *Service
}

// newFixture makes the Atelier skill directory with the given versions (the
// first one is ACTIVE). Nothing outside t.TempDir() is touched.
func newFixture(t *testing.T, versions ...string) *fixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("PBI_RUNS_ARCHIVE", filepath.Join(root, "archive"))
	f := &fixture{
		t:          t,
		root:       root,
		userHome:   filepath.Join(root, "user"),
		home:       filepath.Join(root, "atelier-home"),
		cfgPath:    filepath.Join(root, "cfg", "config.json"),
		atelierDir: filepath.Join(root, "user", ".claude", "skills", "atelier"),
		env:        &fakeEnv{probes: map[string]*scriptedProbe{}},
		rec:        &recorder{},
	}
	if err := os.MkdirAll(f.home, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, v := range versions {
		if err := os.MkdirAll(filepath.Join(f.atelierDir, "versions", v), 0o755); err != nil {
			t.Fatal(err)
		}
		f.env.probes[v] = newScriptedProbe()
	}
	if err := os.WriteFile(filepath.Join(f.atelierDir, "ACTIVE"), []byte(versions[0]+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.svc = NewServiceWith(Deps{
		CfgPath:  f.cfgPath,
		UserHome: f.userHome,
		NewClient: func(cfg atelier.Config) atelier.Client {
			f.env.mu.Lock()
			f.env.cfgs = append(f.env.cfgs, cfg)
			f.env.mu.Unlock()
			return &fakeClient{env: f.env, cfg: cfg}
		},
		Emit:     f.rec.emit,
		Debounce: 10 * time.Millisecond,
	})
	return f
}

// start runs Start and arranges Stop. The test must settle every check
// (answer every Probe it started) before it ends.
func (f *fixture) start() {
	f.t.Helper()
	Start(f.svc, context.Background())
	f.t.Cleanup(func() { Stop(f.svc, context.Background()) })
	f.t.Cleanup(func() {
		f.env.mu.Lock()
		defer f.env.mu.Unlock()
		if len(f.env.unexpected) > 0 {
			f.t.Errorf("unexpected calls to the fake client: %v", f.env.unexpected)
		}
	})
}

func (f *fixture) versionDir(v string) string {
	return filepath.Join(f.atelierDir, "versions", v)
}

func (f *fixture) ready(v string) atelier.ProbeResult {
	return atelier.ProbeResult{Status: atelier.StatusReady, Python: "py-" + v, PythonVersion: "3.12.0", Home: f.home}
}

func (f *fixture) clientConfigs() []atelier.Config {
	f.env.mu.Lock()
	defer f.env.mu.Unlock()
	return append([]atelier.Config(nil), f.env.cfgs...)
}

// startReady starts the Service on the first version and settles it Ready.
func (f *fixture) startReady(v string) {
	f.t.Helper()
	f.start()
	f.env.probes[v].waitEntered(f.t)
	f.env.probes[v].answers <- f.ready(v)
	f.rec.waitState(f.t, Ready, v)
}

func deskErrCode(t *testing.T, err error) string {
	t.Helper()
	var de *Error
	if !errors.As(err, &de) {
		t.Fatalf("error %v is not a *desk.Error", err)
	}
	return de.Code
}

func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A Probe of the first switch that ends after the second switch's Probe must
// not change the version (or the answer) the screen sees.
func TestSwitchVersionTwiceKeepsLatest(t *testing.T) {
	f := newFixture(t, "v1", "v2")
	f.start()
	p1, p2 := f.env.probes["v1"], f.env.probes["v2"]
	p1.waitEntered(t)

	c, err := f.svc.SwitchVersion("v2")
	if err != nil {
		t.Fatalf("SwitchVersion: %v", err)
	}
	if c.State != Checking || c.Version != "v2" {
		t.Fatalf("after the switch: state=%s version=%s, want checking v2", c.State, c.Version)
	}
	p2.waitEntered(t)

	// The second check ends first, then the first (old) check ends.
	p2.answers <- f.ready("v2")
	f.rec.waitState(t, Ready, "v2")
	p1.answers <- f.ready("v1")
	p1.waitReturned(t)

	// A late result is dropped: look for a while that nothing changes.
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		got := f.svc.Connection()
		if got.State != Ready || got.Version != "v2" || got.Python != "py-v2" {
			t.Fatalf("Connection = %+v, want ready v2 with py-v2", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, e := range f.rec.snapshot() {
		if e.Version == "v1" && e.State == Ready {
			t.Errorf("connection:changed announced the dropped v1 result: %+v", e)
		}
	}
	cfg, err := appconfig.Load(f.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LastVersion != "v2" {
		t.Errorf("LastVersion = %q, want v2", cfg.LastVersion)
	}
}

// While a save runs, the version and the Python path cannot be changed.
func TestBusySavingRejectsSwitchAndPython(t *testing.T) {
	f := newFixture(t, "v1", "v2")
	validating := make(chan struct{}, 1)
	release := make(chan struct{})
	f.env.validate = func(context.Context, atelier.Request) (atelier.Validation, error) {
		validating <- struct{}{}
		<-release
		return atelier.Validation{Valid: false, Problems: []string{"held"}}, nil
	}
	f.startReady("v1")

	type saved struct {
		res homestore.SaveResult
		err error
	}
	done := make(chan saved, 1)
	go func() {
		res, err := f.svc.SaveSetting(SaveSettingRequest{Kind: "routing", Input: "{}"})
		done <- saved{res, err}
	}()
	select {
	case <-validating:
	case <-time.After(waitLimit):
		close(release)
		t.Fatal("the save did not reach Validate")
	}
	released := false
	releaseOnce := func() {
		if !released {
			released = true
			close(release)
		}
	}
	defer releaseOnce()

	if !f.svc.Connection().Saving {
		t.Error("Connection().Saving is false while a save runs")
	}
	c, err := f.svc.SwitchVersion("v2")
	if got := deskErrCode(t, err); got != ErrSaving {
		t.Errorf("SwitchVersion code = %q, want %q", got, ErrSaving)
	}
	if c.Version != "v1" {
		t.Errorf("SwitchVersion returned version %q, want v1", c.Version)
	}
	c, err = f.svc.SetPythonPath(filepath.Join(f.root, "other-python"))
	if got := deskErrCode(t, err); got != ErrSaving {
		t.Errorf("SetPythonPath code = %q, want %q", got, ErrSaving)
	}
	if c.Version != "v1" {
		t.Errorf("SetPythonPath returned version %q, want v1", c.Version)
	}
	if got := f.svc.Connection(); got.Version != "v1" || got.State != Ready {
		t.Errorf("Connection = %+v, want ready v1", got)
	}
	cfg, err := appconfig.Load(f.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PythonPath != "" {
		t.Errorf("the refused Python path was saved: %q", cfg.PythonPath)
	}

	releaseOnce()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("SaveSetting: %v", r.err)
		}
		if r.res.Outcome != homestore.Rejected {
			t.Errorf("Outcome = %q, want %q", r.res.Outcome, homestore.Rejected)
		}
	case <-time.After(waitLimit):
		t.Fatal("SaveSetting did not return")
	}
	if f.svc.Connection().Saving {
		t.Error("Connection().Saving stays true after the save")
	}
}

// A save is accepted only when the connection is ready; nothing is written to
// the home otherwise (the "%% never" setting is not written).
func TestSaveRefusedWhenNotReady(t *testing.T) {
	cases := []struct {
		name   string
		answer *atelier.ProbeResult // nil: the check is still running
		want   ConnState
	}{
		{"python_unavailable", &atelier.ProbeResult{Status: atelier.StatusPythonUnavailable, Code: atelier.ErrPythonNotFound, Diagnostic: "no python"}, PythonUnavailable},
		{"unsupported", &atelier.ProbeResult{Status: atelier.StatusUnsupported, Code: atelier.ErrUnsupported, Diagnostic: "too old"}, Unsupported},
		{"checking", nil, Checking},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, "v1")
			if err := os.MkdirAll(filepath.Join(f.home, "routing"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(f.home, "routing", "keep.txt"), []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}
			f.start()
			p := f.env.probes["v1"]
			p.waitEntered(t)
			if tc.answer != nil {
				p.answers <- *tc.answer
				f.rec.waitState(t, tc.want, "v1")
			}
			before := snapshotDir(t, f.home)

			_, err := f.svc.SaveSetting(SaveSettingRequest{Kind: "routing", Input: `{"never":true}`})
			if err == nil {
				t.Fatal("SaveSetting succeeded, want not_ready")
			}
			if got := deskErrCode(t, err); got != ErrNotReady {
				t.Errorf("code = %q, want %q", got, ErrNotReady)
			}
			if after := snapshotDir(t, f.home); !mapsEqual(before, after) {
				t.Errorf("the home changed: before %v, after %v", before, after)
			}
			f.env.mu.Lock()
			validated := f.env.validated
			f.env.mu.Unlock()
			if validated != 0 {
				t.Errorf("Validate was called %d times", validated)
			}
			if got := f.svc.Connection(); got.State != tc.want || got.Saving {
				t.Errorf("Connection = %+v, want state %s and not saving", got, tc.want)
			}

			if tc.answer == nil {
				// Settle the check so that nothing runs after the test.
				p.answers <- f.ready("v1")
				f.rec.waitState(t, Ready, "v1")
			}
		})
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// After a switch, the version the screen shows is the version directory the
// client was built for.
func TestSwitchVersionMatchesClientVersionDir(t *testing.T) {
	f := newFixture(t, "v1", "v2")
	f.startReady("v1")
	if got := f.svc.Connection().Version; got != "v1" {
		t.Fatalf("started on %q, want v1", got)
	}

	for _, v := range []string{"v2", "v1"} {
		c, err := f.svc.SwitchVersion(v)
		if err != nil {
			t.Fatalf("SwitchVersion(%s): %v", v, err)
		}
		if c.Version != v {
			t.Errorf("SwitchVersion(%s) returned version %q", v, c.Version)
		}
		cfg := f.env.probes[v].waitEntered(t)
		if cfg.VersionDir != f.versionDir(v) {
			t.Errorf("the Probe client of %s got VersionDir %q, want %q", v, cfg.VersionDir, f.versionDir(v))
		}
		seen := countReady(f.rec.snapshot(), v)
		f.env.probes[v].answers <- f.ready(v)
		f.rec.waitEvent(t, "a new ready "+v+" after the switch", func(Connection) bool {
			return countReady(f.rec.snapshot(), v) > seen
		})
		got := f.svc.Connection()
		if got.Version != v || got.State != Ready {
			t.Fatalf("Connection = %+v, want ready %s", got, v)
		}
		// The client that serves the version (built with the home) is for the same directory.
		cfgs := f.clientConfigs()
		last := cfgs[len(cfgs)-1]
		if last.VersionDir != f.versionDir(v) || filepath.Base(last.VersionDir) != got.Version {
			t.Errorf("last client VersionDir = %q, Connection().Version = %q", last.VersionDir, got.Version)
		}
		if last.Home != f.home {
			t.Errorf("last client Home = %q, want %q", last.Home, f.home)
		}
	}
}

func countReady(events []Connection, v string) int {
	n := 0
	for _, e := range events {
		if e.State == Ready && e.Version == v {
			n++
		}
	}
	return n
}

// The connection moves checking -> ready / python_unavailable / unsupported,
// and from the last two back to checking by SetPythonPath / SwitchVersion;
// every move is announced with connection:changed.
func TestConnectionStateTransitions(t *testing.T) {
	joinStates := func(ss []ConnState) string {
		out := make([]string, len(ss))
		for i, s := range ss {
			out[i] = string(s)
		}
		return strings.Join(out, ",")
	}

	for _, tc := range []struct {
		name   string
		answer atelier.ProbeResult
		want   ConnState
	}{
		{"checking to ready", atelier.ProbeResult{Status: atelier.StatusReady, Python: "py", PythonVersion: "3.12.0"}, Ready},
		{"checking to python_unavailable", atelier.ProbeResult{Status: atelier.StatusPythonUnavailable, Code: atelier.ErrPythonNotFound, Diagnostic: "no python"}, PythonUnavailable},
		{"checking to unsupported", atelier.ProbeResult{Status: atelier.StatusUnsupported, Code: atelier.ErrUnsupported, Diagnostic: "too old"}, Unsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, "v1")
			f.start()
			if got := f.svc.Connection(); got.State != Checking || got.Version != "v1" {
				t.Fatalf("after Start: %+v, want checking v1", got)
			}
			ans := tc.answer
			if ans.Status == atelier.StatusReady {
				ans.Home = f.home
			}
			f.env.probes["v1"].waitEntered(t)
			f.env.probes["v1"].answers <- ans
			got := f.rec.waitState(t, tc.want, "v1")
			if got.Code != string(ans.Code) || got.Diagnostic != ans.Diagnostic || got.Python != ans.Python {
				t.Errorf("event = %+v, want code %q diagnostic %q python %q", got, ans.Code, ans.Diagnostic, ans.Python)
			}
			if want := "checking," + string(tc.want); joinStates(f.rec.states()) != want {
				t.Errorf("events = %s, want %s", joinStates(f.rec.states()), want)
			}
			if c := f.svc.Connection(); c.State != tc.want {
				t.Errorf("Connection().State = %s, want %s", c.State, tc.want)
			}
		})
	}

	t.Run("python_unavailable to checking by SetPythonPath", func(t *testing.T) {
		f := newFixture(t, "v1")
		f.start()
		p := f.env.probes["v1"]
		p.waitEntered(t)
		p.answers <- atelier.ProbeResult{Status: atelier.StatusPythonUnavailable, Code: atelier.ErrPythonNotFound, Diagnostic: "no python"}
		f.rec.waitState(t, PythonUnavailable, "v1")

		py := filepath.Join(f.root, "my-python")
		c, err := f.svc.SetPythonPath(py)
		if err != nil {
			t.Fatalf("SetPythonPath: %v", err)
		}
		if c.State != Checking || c.Version != "v1" {
			t.Errorf("SetPythonPath returned %+v, want checking v1", c)
		}
		if cfg := p.waitEntered(t); cfg.Python != py {
			t.Errorf("the new check used Python %q, want %q", cfg.Python, py)
		}
		saved, err := appconfig.Load(f.cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		if saved.PythonPath != py {
			t.Errorf("saved PythonPath = %q, want %q", saved.PythonPath, py)
		}
		p.answers <- f.ready("v1")
		f.rec.waitState(t, Ready, "v1")
		if want := "checking,python_unavailable,checking,ready"; joinStates(f.rec.states()) != want {
			t.Errorf("events = %s, want %s", joinStates(f.rec.states()), want)
		}
	})

	t.Run("unsupported to checking by SwitchVersion", func(t *testing.T) {
		f := newFixture(t, "v1", "v2")
		f.start()
		f.env.probes["v1"].waitEntered(t)
		f.env.probes["v1"].answers <- atelier.ProbeResult{Status: atelier.StatusUnsupported, Code: atelier.ErrUnsupported, Diagnostic: "too old"}
		f.rec.waitState(t, Unsupported, "v1")

		c, err := f.svc.SwitchVersion("v2")
		if err != nil {
			t.Fatalf("SwitchVersion: %v", err)
		}
		if c.State != Checking || c.Version != "v2" {
			t.Errorf("SwitchVersion returned %+v, want checking v2", c)
		}
		f.env.probes["v2"].waitEntered(t)
		f.env.probes["v2"].answers <- f.ready("v2")
		f.rec.waitState(t, Ready, "v2")
		if want := "checking,unsupported,checking,ready"; joinStates(f.rec.states()) != want {
			t.Errorf("events = %s, want %s", joinStates(f.rec.states()), want)
		}
		evs := f.rec.snapshot()
		if got := evs[len(evs)-1]; got.Version != "v2" || got.Code != "" {
			t.Errorf("last event = %+v, want v2 without the old error code", got)
		}
	})
}

// A repository added while the first check is still running is read by Runs
// during the check and after it (the end of the check must not restore the
// repositories it saw when it began; PBI-010 rework-1).
func TestAddRepoDuringProbeVisibleToRuns(t *testing.T) {
	f := newFixture(t, "v1")
	repo := filepath.Join(f.root, "repo")
	runsFile := filepath.Join(repo, "docs", "pbi", "runs.jsonl")
	if err := os.MkdirAll(filepath.Dir(runsFile), 0o755); err != nil {
		t.Fatal(err)
	}
	// An unreadable line makes the reader name this file in a warning.
	if err := os.WriteFile(runsFile, []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsRepo := func(when string) {
		t.Helper()
		v, err := f.svc.Runs()
		if err != nil {
			t.Fatalf("Runs %s: %v", when, err)
		}
		for _, w := range v.Warnings {
			if strings.HasSuffix(filepath.ToSlash(w.Path), "docs/pbi/runs.jsonl") {
				return
			}
		}
		t.Errorf("Runs %s does not read the added repository; warnings: %+v", when, v.Warnings)
	}

	f.start()
	p := f.env.probes["v1"]
	p.waitEntered(t)
	repos, err := f.svc.AddRepo(repo)
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("AddRepo returned %v", repos)
	}
	readsRepo("during the check")

	p.answers <- f.ready("v1")
	f.rec.waitState(t, Ready, "v1")
	readsRepo("after the check")
	if got := f.svc.Repos(); !slices.Equal(got, repos) {
		t.Errorf("Repos() = %v, want %v", got, repos)
	}
}
