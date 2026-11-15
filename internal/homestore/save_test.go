package homestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/reeschan/atelier-desk/internal/atelier"
)

// ---- helpers (test-only) ----

func tSHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// copyTree copies the directory src into dst (created when needed).
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

// readTree returns every file under root as "/"-relative path -> content.
func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("read tree %s: %v", root, err)
	}
	return out
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// fileRead is the plan read of one home file as it is right now.
func fileRead(root, home, rel string) atelier.PlanRead {
	r := atelier.PlanRead{Root: root, Type: "file", Path: rel, SHA256: "missing", Missing: true}
	if b, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(rel))); err == nil {
		r.SHA256 = tSHA(b)
		r.Missing = false
	}
	return r
}

// dirRead is the plan read of the "*.json" files directly under a home directory.
func dirRead(home, rel string) atelier.PlanRead {
	r := atelier.PlanRead{Root: "home", Type: "directory", Path: rel, Entries: []atelier.PlanEntry{}}
	list, err := os.ReadDir(filepath.Join(home, filepath.FromSlash(rel)))
	if err != nil {
		r.Missing = true
		return r
	}
	for _, e := range list {
		if filepath.Ext(e.Name()) != ".json" || e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(rel), e.Name()))
		if err != nil {
			continue
		}
		r.Entries = append(r.Entries, atelier.PlanEntry{Name: e.Name(), Type: "file", SHA256: tSHA(b)})
	}
	return r
}

// fakeClient is an Atelier client that answers Validate/Plan from fields. The
// plan's reads are computed from the home when Plan is called.
type fakeClient struct {
	validation     atelier.Validation
	planValidation *atelier.Validation // when set, Validate passes and Plan fails with it
	reads          func() []atelier.PlanRead
	writes         []atelier.PlanWrite
}

func (c *fakeClient) Probe(context.Context) (atelier.ProbeResult, error) {
	return atelier.ProbeResult{}, nil
}
func (c *fakeClient) Components(context.Context) ([]atelier.Component, error) { return nil, nil }
func (c *fakeClient) Workflows(context.Context) ([]atelier.Workflow, error)   { return nil, nil }
func (c *fakeClient) State(context.Context) (atelier.State, error)            { return atelier.State{}, nil }

func (c *fakeClient) Validate(_ context.Context, _ atelier.Request) (atelier.Validation, error) {
	return c.validation, nil
}

func (c *fakeClient) Plan(_ context.Context, req atelier.Request) (atelier.PlanResult, error) {
	if c.planValidation != nil {
		return atelier.PlanResult{Validation: *c.planValidation}, nil
	}
	if !c.validation.Valid {
		return atelier.PlanResult{Validation: c.validation}, nil
	}
	var reads []atelier.PlanRead
	if c.reads != nil {
		reads = c.reads()
	}
	return atelier.PlanResult{
		Validation: c.validation,
		Plan: &atelier.SavePlan{
			Kind:   req.Kind,
			Reads:  reads,
			Writes: c.writes,
		},
	}, nil
}

// env is one temporary home (a copy of testdata/home), a backup root and a Store.
type env struct {
	home    string
	backups string
	client  *fakeClient
	store   *Store
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	e := &env{home: filepath.Join(root, "home"), backups: filepath.Join(root, "backups")}
	copyTree(t, filepath.Join("testdata", "home"), e.home)
	e.client = &fakeClient{validation: atelier.Validation{Valid: true}}
	e.client.reads = func() []atelier.PlanRead { return []atelier.PlanRead{fileRead("home", e.home, "routing.json")} }
	e.store = NewStore(e.client, e.home, filepath.Join(root, "atelier"), e.backups)
	return e
}

func (e *env) writeRouting(content string) {
	e.client.writes = []atelier.PlanWrite{{Op: "replace", Path: "routing.json", ContentUTF8: content}}
}

func (e *env) save(t *testing.T, base Baseline) (SaveResult, error) {
	t.Helper()
	return e.store.Save(context.Background(), SaveRequest{
		Kind: "routing", Name: "", Input: json.RawMessage(`{}`), Base: base,
	})
}

func (e *env) snapshot(t *testing.T) Baseline {
	t.Helper()
	b, err := e.store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return b
}

// externalWrite changes a home file from outside the Store.
func (e *env) externalWrite(t *testing.T, rel, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(e.home, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
		t.Fatalf("external write %s: %v", rel, err)
	}
}

func (e *env) path(rel string) string { return filepath.Join(e.home, filepath.FromSlash(rel)) }

func wantSaved(t *testing.T, res SaveResult, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if res.Outcome != Saved {
		t.Fatalf("Outcome = %q (changed %v, problems %v), want %q", res.Outcome, res.Changed, res.Problems, Saved)
	}
}

func wantExternalChange(t *testing.T, res SaveResult, err error, changed ...string) {
	t.Helper()
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if res.Outcome != ExternalChange {
		t.Fatalf("Outcome = %q, want %q", res.Outcome, ExternalChange)
	}
	if !reflect.DeepEqual(res.Changed, changed) {
		t.Fatalf("Changed = %v, want %v", res.Changed, changed)
	}
}

const (
	newRouting      = "{\n  \"version\": 2,\n  \"rules\": []\n}\n"
	externalRouting = "{\"version\": 1, \"rules\": [{\"match\": \"*\", \"agent\": \"gemini\"}]}\n"
	externalAgain   = "{\"version\": 1, \"rules\": [{\"match\": \"*\", \"agent\": \"opencode\"}]}\n"
)

// ---- tests ----

func TestRejectedLeavesHomeUntouched(t *testing.T) {
	cases := map[string]func(e *env){
		"validate rejects": func(e *env) {
			e.client.validation = atelier.Validation{Valid: false, Problems: []string{"never: forbidden content"}}
		},
		"plan rejects": func(e *env) {
			e.client.planValidation = &atelier.Validation{Valid: false, Problems: []string{"never: forbidden content"}}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			setup(e)
			e.writeRouting(newRouting)
			before := readTree(t, e.home)
			base := e.snapshot(t)

			res, err := e.save(t, base)
			if err != nil {
				t.Fatalf("Save error: %v", err)
			}
			if res.Outcome != Rejected {
				t.Fatalf("Outcome = %q, want %q", res.Outcome, Rejected)
			}
			if !reflect.DeepEqual(res.Problems, []string{"never: forbidden content"}) {
				t.Fatalf("Problems = %v, want Atelier's own text", res.Problems)
			}
			if after := readTree(t, e.home); !reflect.DeepEqual(before, after) {
				t.Fatalf("home changed by a rejected save:\nbefore %v\nafter  %v", before, after)
			}
			if exists(e.backups) {
				t.Fatalf("a rejected save made a backup directory")
			}
		})
	}
}

func TestExternalChangeBlocksSave(t *testing.T) {
	e := newEnv(t)
	e.writeRouting(newRouting)
	base := e.snapshot(t) // the setting was opened here

	e.externalWrite(t, "routing.json", externalRouting)
	before := readTree(t, e.home)

	res, err := e.save(t, base)
	wantExternalChange(t, res, err, "routing.json")
	if got, want := res.Current.Files["routing.json"], tSHA([]byte(externalRouting)); got != want {
		t.Fatalf("Current routing.json = %q, want fingerprint of the external file %q", got, want)
	}
	if after := readTree(t, e.home); !reflect.DeepEqual(before, after) {
		t.Fatalf("a blocked save changed the home:\nbefore %v\nafter  %v", before, after)
	}
	if exists(e.backups) {
		t.Fatalf("a blocked save made a backup directory")
	}
}

func TestOverwriteWithCurrentBacksUp(t *testing.T) {
	e := newEnv(t)
	e.writeRouting(newRouting)
	base := e.snapshot(t)
	e.externalWrite(t, "routing.json", externalRouting)

	blocked, err := e.save(t, base)
	wantExternalChange(t, blocked, err, "routing.json")

	// The user chooses to overwrite: the Current of the answer becomes the Base.
	res, err := e.save(t, blocked.Current)
	wantSaved(t, res, err)
	if got := readFileString(t, e.path("routing.json")); got != newRouting {
		t.Fatalf("routing.json = %q, want the saved content", got)
	}
	if res.BackupDir == "" {
		t.Fatalf("BackupDir is empty")
	}
	if got := readFileString(t, filepath.Join(res.BackupDir, "files", "routing.json")); got != externalRouting {
		t.Fatalf("backup of routing.json = %q, want the file before the save %q", got, externalRouting)
	}
	if !reflect.DeepEqual(res.Written, []string{"routing.json"}) {
		t.Fatalf("Written = %v, want [routing.json]", res.Written)
	}
}

func TestSecondExternalChangeStillBlocks(t *testing.T) {
	e := newEnv(t)
	e.writeRouting(newRouting)
	base := e.snapshot(t)
	e.externalWrite(t, "routing.json", externalRouting)

	first, err := e.save(t, base)
	wantExternalChange(t, first, err, "routing.json")

	// Another change before the user confirms the overwrite.
	e.externalWrite(t, "routing.json", externalAgain)
	second, err := e.save(t, first.Current)
	wantExternalChange(t, second, err, "routing.json")
	if got := readFileString(t, e.path("routing.json")); got != externalAgain {
		t.Fatalf("routing.json = %q, want it left as changed outside", got)
	}
	if got, want := second.Current.Files["routing.json"], tSHA([]byte(externalAgain)); got != want {
		t.Fatalf("Current routing.json = %q, want %q", got, want)
	}

	// With the newest Current the overwrite goes through.
	third, err := e.save(t, second.Current)
	wantSaved(t, third, err)
	if got := readFileString(t, e.path("routing.json")); got != newRouting {
		t.Fatalf("routing.json = %q, want the saved content", got)
	}
}

func TestSavedBaseAllowsSecondSave(t *testing.T) {
	e := newEnv(t)
	e.writeRouting(newRouting)
	first, err := e.save(t, e.snapshot(t))
	wantSaved(t, first, err)
	if got, want := first.Base.Files["routing.json"], tSHA([]byte(newRouting)); got != want {
		t.Fatalf("Base routing.json = %q, want fingerprint of the saved file %q", got, want)
	}

	const second = "{\"version\": 3, \"rules\": []}\n"
	e.writeRouting(second)
	res, err := e.save(t, first.Base)
	wantSaved(t, res, err)
	if got := readFileString(t, e.path("routing.json")); got != second {
		t.Fatalf("routing.json = %q, want the second saved content", got)
	}
}

func TestTwoWritesOwnReplacementNotExternal(t *testing.T) {
	e := newEnv(t)
	e.client.reads = func() []atelier.PlanRead {
		return []atelier.PlanRead{
			fileRead("home", e.home, "history.jsonl"),
			fileRead("home", e.home, "overrides.json"),
		}
	}
	const history = "{\"event\": \"overrides.saved\"}\n"
	const overrides = "{\"version\": 2, \"overrides\": {}}\n"
	e.client.writes = []atelier.PlanWrite{
		{Op: "replace", Path: "history.jsonl", ContentUTF8: history},
		{Op: "replace", Path: "overrides.json", ContentUTF8: overrides},
	}

	res, err := e.save(t, e.snapshot(t))
	wantSaved(t, res, err)
	if !reflect.DeepEqual(res.Written, []string{"history.jsonl", "overrides.json"}) {
		t.Fatalf("Written = %v, want both files in order", res.Written)
	}
	if got := readFileString(t, e.path("history.jsonl")); got != history {
		t.Fatalf("history.jsonl = %q, want %q", got, history)
	}
	if got := readFileString(t, e.path("overrides.json")); got != overrides {
		t.Fatalf("overrides.json = %q, want %q", got, overrides)
	}
}

func TestUnsafeWritesRejected(t *testing.T) {
	cases := []struct {
		name string
		op   string
		path string
		skip bool
	}{
		{name: "parent directory", op: "replace", path: "../x.json"},
		{name: "drive letter path", op: "replace", path: "C:/x.json", skip: runtime.GOOS != "windows"},
		{name: "append op", op: "append", path: "routing.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.skip {
				t.Skip("a drive letter path is only absolute on Windows")
			}
			e := newEnv(t)
			e.client.writes = []atelier.PlanWrite{{Op: c.op, Path: c.path, ContentUTF8: "{}\n"}}
			base := e.snapshot(t)
			before := readTree(t, e.home)
			outside := filepath.Join(filepath.Dir(e.home), "x.json")

			res, err := e.save(t, base)
			if err == nil {
				t.Fatalf("Save succeeded (outcome %q), want an error", res.Outcome)
			}
			if res.Outcome == Saved {
				t.Fatalf("Outcome = %q with an error", res.Outcome)
			}
			if after := readTree(t, e.home); !reflect.DeepEqual(before, after) {
				t.Fatalf("home changed by an unsafe save:\nbefore %v\nafter  %v", before, after)
			}
			if exists(outside) {
				t.Fatalf("a file was written outside the home: %s", outside)
			}
			if exists(e.backups) {
				t.Fatalf("an unsafe save made a backup directory")
			}
		})
	}
}

func TestTemplateSaveWithoutTemplatesDirOrHome(t *testing.T) {
	const content = "{\"name\": \"t1\", \"body\": \"hello\"}\n"
	setup := func(c *fakeClient, home string) {
		c.reads = func() []atelier.PlanRead {
			return []atelier.PlanRead{dirRead(home, "templates"), fileRead("home", home, "templates/t1.json")}
		}
		c.writes = []atelier.PlanWrite{{Op: "replace", Path: "templates/t1.json", ContentUTF8: content}}
	}
	req := func(base Baseline) SaveRequest {
		return SaveRequest{Kind: "template", Name: "t1", Input: json.RawMessage(`{}`), Base: base}
	}

	t.Run("home without templates dir", func(t *testing.T) {
		e := newEnv(t)
		setup(e.client, e.home)
		if exists(e.path("templates")) {
			t.Fatalf("testdata home must not have a templates directory")
		}
		res, err := e.store.Save(context.Background(), req(e.snapshot(t)))
		wantSaved(t, res, err)
		if got := readFileString(t, e.path("templates/t1.json")); got != content {
			t.Fatalf("templates/t1.json = %q, want %q", got, content)
		}
	})

	t.Run("home does not exist", func(t *testing.T) {
		root := t.TempDir()
		home := filepath.Join(root, "no-such-home")
		c := &fakeClient{validation: atelier.Validation{Valid: true}}
		setup(c, home)
		store := NewStore(c, home, filepath.Join(root, "atelier"), filepath.Join(root, "backups"))
		base, err := store.Snapshot()
		if err != nil {
			t.Fatalf("snapshot of a missing home: %v", err)
		}
		if len(base.Files) != 0 {
			t.Fatalf("baseline of a missing home = %v, want empty", base.Files)
		}
		res, err := store.Save(context.Background(), req(base))
		wantSaved(t, res, err)
		if got := readFileString(t, filepath.Join(home, "templates", "t1.json")); got != content {
			t.Fatalf("templates/t1.json = %q, want %q", got, content)
		}
	})
}

func TestPartialFailureWritesRecovery(t *testing.T) {
	e := newEnv(t)
	// Atelier's own directory is the home here, so that the first replace
	// changes a file the plan read under root "atelier" (which the save itself
	// does not account for). The second write's check then finds a change.
	e.store = NewStore(e.client, e.home, e.home, e.backups)
	e.client.reads = func() []atelier.PlanRead {
		return []atelier.PlanRead{fileRead("atelier", e.home, "routing.json")}
	}
	e.client.writes = []atelier.PlanWrite{
		{Op: "replace", Path: "routing.json", ContentUTF8: newRouting},
		{Op: "replace", Path: "overrides.json", ContentUTF8: "{\"version\": 2}\n"},
	}
	originalOverrides := readFileString(t, e.path("overrides.json"))
	originalRouting := readFileString(t, e.path("routing.json"))

	res, err := e.save(t, e.snapshot(t))
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if res.Outcome != PartialFailure {
		t.Fatalf("Outcome = %q, want %q", res.Outcome, PartialFailure)
	}
	if !reflect.DeepEqual(res.Written, []string{"routing.json"}) {
		t.Fatalf("Written = %v, want [routing.json]", res.Written)
	}
	if res.Failed != "overrides.json" {
		t.Fatalf("Failed = %q, want overrides.json", res.Failed)
	}
	if got := readFileString(t, e.path("routing.json")); got != newRouting {
		t.Fatalf("routing.json = %q, want the first write applied", got)
	}
	if got := readFileString(t, e.path("overrides.json")); got != originalOverrides {
		t.Fatalf("overrides.json = %q, want it untouched", got)
	}
	if res.Current.Files["routing.json"] != tSHA([]byte(newRouting)) {
		t.Fatalf("Current routing.json = %q, want the fingerprint after the first write", res.Current.Files["routing.json"])
	}
	if res.RecoveryFile == "" {
		t.Fatalf("RecoveryFile is empty")
	}
	var rec RecoveryRecord
	if err := json.Unmarshal([]byte(readFileString(t, res.RecoveryFile)), &rec); err != nil {
		t.Fatalf("recovery.json is not JSON: %v", err)
	}
	if len(rec.Writes) != 2 {
		t.Fatalf("recovery writes = %+v, want 2", rec.Writes)
	}
	if rec.Writes[0].Path != "routing.json" || rec.Writes[0].State != StateDone {
		t.Fatalf("recovery write 0 = %+v, want routing.json done", rec.Writes[0])
	}
	if rec.Writes[1].Path != "overrides.json" || rec.Writes[1].State != StateFailed {
		t.Fatalf("recovery write 1 = %+v, want overrides.json failed", rec.Writes[1])
	}
	// The previous version of the file that was replaced is kept for recovery.
	if got := readFileString(t, filepath.Join(res.BackupDir, "files", "routing.json")); got != originalRouting {
		t.Fatalf("backup of routing.json = %q, want the file before the save", got)
	}
}

func TestEmptyWritesIsSavedNoop(t *testing.T) {
	e := newEnv(t)
	e.client.writes = nil
	before := readTree(t, e.home)
	base := e.snapshot(t)

	res, err := e.save(t, base)
	wantSaved(t, res, err)
	if len(res.Written) != 0 {
		t.Fatalf("Written = %v, want none", res.Written)
	}
	if after := readTree(t, e.home); !reflect.DeepEqual(before, after) {
		t.Fatalf("an empty save changed the home:\nbefore %v\nafter  %v", before, after)
	}
	if exists(e.backups) {
		t.Fatalf("an empty save made a backup directory")
	}
	var files, baseFiles []string
	for p := range res.Base.Files {
		files = append(files, p)
	}
	for p := range base.Files {
		baseFiles = append(baseFiles, p)
	}
	sort.Strings(files)
	sort.Strings(baseFiles)
	if !reflect.DeepEqual(files, baseFiles) {
		t.Fatalf("Base files = %v, want %v", files, baseFiles)
	}
}
