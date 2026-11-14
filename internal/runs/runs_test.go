package runs

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/reeschan/atelier-desk/internal/paths"
)

// copyTestdata copies testdata/ into a temporary directory so that the tests
// never read or write anything else.
func copyTestdata(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir("testdata", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("testdata", p)
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
		t.Fatalf("copy testdata: %v", err)
	}
	return dst
}

func newTestReader(t *testing.T, repoNames ...string) *Reader {
	t.Helper()
	root := copyTestdata(t)
	var repos []string
	for _, n := range repoNames {
		repos = append(repos, filepath.Join(root, "repos", n))
	}
	return NewReader(paths.Locations{Repos: repos, ArchiveRoot: filepath.Join(root, "archive")})
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return at
}

func findRun(t *testing.T, rs []Run, repo, workKey string) Run {
	t.Helper()
	for _, r := range rs {
		if r.Repo == repo && r.WorkKey == workKey {
			return r
		}
	}
	t.Fatalf("no run for repo %q work_key %q in %d runs", repo, workKey, len(rs))
	return Run{}
}

func findPBI(t *testing.T, r Run, id string) PBIResult {
	t.Helper()
	for _, p := range r.PBIs {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("run %q/%q has no PBI %q", r.Repo, r.WorkKey, id)
	return PBIResult{}
}

func TestRunsOrderedAndDeduplicated(t *testing.T) {
	rd := newTestReader(t, "alpha", "atelier-desk")
	rs, _, err := rd.Runs()
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}

	type key struct{ repo, wk string }
	want := []key{
		{"alpha", "alpha:main:W-001"},
		{"alpha", ""},
		{"alpha", "alpha:main:W-002"},
		{"atelier-desk", "W-003"},
		{"atelier-desk", "atelier-desk"},
		{"atelier-desk", ""},
		{"atelier-desk-wt1", "atelier-desk:feat/x:W-004"},
		{"atelier-desk-wt1", "atelier-desk:feat/y:W-005"},
	}
	var got []key
	for _, r := range rs {
		got = append(got, key{r.Repo, r.WorkKey})
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runs order = %v, want %v (by repository, then newest LastAt first)", got, want)
	}

	// The same attempt in a repository and in the archive is one attempt, and the
	// run that has a repository row is not archived.
	w4 := findRun(t, rs, "atelier-desk-wt1", "atelier-desk:feat/x:W-004")
	if w4.Archived {
		t.Errorf("W-004 has a repository row, Archived = true, want false")
	}
	if n := len(findPBI(t, w4, "PBI-001").Attempts); n != 1 {
		t.Errorf("W-004 PBI-001 attempts = %d, want 1 (repo and archive copy are one attempt)", n)
	}
	if n := len(findPBI(t, w4, "PBI-002").Attempts); n != 1 {
		t.Errorf("W-004 PBI-002 attempts = %d, want 1 (archive only)", n)
	}
	w1 := findRun(t, rs, "alpha", "alpha:main:W-001")
	if w1.Archived {
		t.Errorf("alpha W-001 has repository rows, Archived = true, want false")
	}
	if n := len(findPBI(t, w1, "PBI-002").Attempts); n != 2 {
		t.Errorf("alpha W-001 PBI-002 attempts = %d, want 2 (one copy is in both places)", n)
	}

	// A run whose rows all come from the archive is marked archived.
	for _, k := range []key{{"alpha", "alpha:main:W-002"}, {"atelier-desk-wt1", "atelier-desk:feat/y:W-005"}} {
		if r := findRun(t, rs, k.repo, k.wk); !r.Archived {
			t.Errorf("run %v is only in the archive, Archived = false, want true", k)
		}
	}

	// FirstAt / LastAt come from recorded_at.
	if !w1.FirstAt.Equal(mustTime(t, "2026-11-11T09:00:00+09:00")) {
		t.Errorf("alpha W-001 FirstAt = %v, want 2026-11-11T09:00:00+09:00", w1.FirstAt)
	}
	if !w1.LastAt.Equal(mustTime(t, "2026-11-11T12:00:00+09:00")) {
		t.Errorf("alpha W-001 LastAt = %v, want 2026-11-11T12:00:00+09:00", w1.LastAt)
	}
	if w1.Source == "" {
		t.Errorf("alpha W-001 Source is empty, want the path of the file read")
	}
}

func TestPBIResultsFromRealRows(t *testing.T) {
	rd := newTestReader(t, "alpha", "atelier-desk")
	rs, _, err := rd.Runs()
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	w1 := findRun(t, rs, "alpha", "alpha:main:W-001")

	// PBI-001: the repository file only kept the newest row (a passing rework);
	// the archive adds the older rejected attempt. The newest attempt by
	// recorded_at decides Passed, not the order in which files were read.
	p1 := findPBI(t, w1, "PBI-001")
	if len(p1.Attempts) != 2 {
		t.Fatalf("PBI-001 attempts = %d, want 2", len(p1.Attempts))
	}
	if p1.Attempts[0].Report != "PBI-001" || p1.Attempts[0].Outcome != OutcomeRework {
		t.Errorf("PBI-001 first attempt = %q/%q, want PBI-001/%s (oldest first)", p1.Attempts[0].Report, p1.Attempts[0].Outcome, OutcomeRework)
	}
	if last := p1.Attempts[1]; last.Report != "PBI-001.rework-1" || last.Outcome != OutcomePassed {
		t.Errorf("PBI-001 last attempt = %q/%q, want PBI-001.rework-1/%s", last.Report, last.Outcome, OutcomePassed)
	}
	if !p1.Passed {
		t.Errorf("PBI-001 Passed = false, want true (the newest attempt by recorded_at passed)")
	}
	if p1.Reworks != 1 {
		t.Errorf("PBI-001 Reworks = %d, want 1", p1.Reworks)
	}
	if p1.Model != "model-new" {
		t.Errorf("PBI-001 Model = %q, want model-new (model of the last attempt)", p1.Model)
	}
	if !reflect.DeepEqual(p1.Attempts[0].Causes, []string{"spec"}) {
		t.Errorf("PBI-001 first attempt Causes = %v, want [spec]", p1.Attempts[0].Causes)
	}
	var findings map[string]int
	if err := json.Unmarshal(p1.Attempts[0].Findings, &findings); err != nil || findings["中"] != 1 {
		t.Errorf("PBI-001 first attempt Findings = %s (err %v), want 中=1", p1.Attempts[0].Findings, err)
	}
	if p1.Attempts[0].Step != "1" || p1.Attempts[0].Runner != "agent" || len(p1.Attempts[0].Raw) == 0 {
		t.Errorf("PBI-001 first attempt Step/Runner/Raw = %q/%q/%d bytes, want 1/agent/non-empty", p1.Attempts[0].Step, p1.Attempts[0].Runner, len(p1.Attempts[0].Raw))
	}

	// PBI-002: escalated once, then passed by the next attempt.
	p2 := findPBI(t, w1, "PBI-002")
	if !p2.Passed || p2.Escalations != 1 || p2.Held {
		t.Errorf("PBI-002 Passed/Escalations/Held = %v/%d/%v, want true/1/false", p2.Passed, p2.Escalations, p2.Held)
	}
	if p2.Model != "model-b" {
		t.Errorf("PBI-002 Model = %q, want model-b", p2.Model)
	}
	if !reflect.DeepEqual(p2.Attempts[0].Causes, []string{"model", "env"}) {
		t.Errorf("PBI-002 first attempt Causes = %v, want [model env]", p2.Attempts[0].Causes)
	}
	if p2.Attempts[1].Causes != nil {
		t.Errorf("PBI-002 second attempt Causes = %v, want none (cause is null)", p2.Attempts[1].Causes)
	}

	// PBI-003: held, with its causes as the reason.
	p3 := findPBI(t, w1, "PBI-003")
	if p3.Passed || !p3.Held {
		t.Errorf("PBI-003 Passed/Held = %v/%v, want false/true", p3.Passed, p3.Held)
	}
	if p3.HoldReason != "spec,env" {
		t.Errorf("PBI-003 HoldReason = %q, want spec,env", p3.HoldReason)
	}
}

func TestRepoRowForWorktreeNameAndNullWorkKey(t *testing.T) {
	// Only the main clone is registered. The rows of the worktree (repo
	// "atelier-desk-wt1") are found in the archive through the head of work_key.
	rd := newTestReader(t, "atelier-desk")
	rs, warns, err := rd.Runs()
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("warnings = %v, want none (null / bare work_key and unregistered archive folders are not errors)", warns)
	}
	repos := map[string]bool{}
	for _, r := range rs {
		repos[r.Repo] = true
	}
	if !reflect.DeepEqual(repos, map[string]bool{"atelier-desk": true, "atelier-desk-wt1": true}) {
		t.Errorf("repos = %v, want atelier-desk and atelier-desk-wt1 only (no alpha, no stranger)", repos)
	}

	// The worktree runs appear, from the repository row and from the archive.
	w4 := findRun(t, rs, "atelier-desk-wt1", "atelier-desk:feat/x:W-004")
	if len(w4.PBIs) != 2 {
		t.Errorf("W-004 PBIs = %d, want 2", len(w4.PBIs))
	}
	w5 := findRun(t, rs, "atelier-desk-wt1", "atelier-desk:feat/y:W-005")
	if !w5.Archived {
		t.Errorf("W-005 Archived = false, want true")
	}

	// work_key null, "atelier-desk" and "W-003" all make a run.
	nullRun := findRun(t, rs, "atelier-desk", "")
	if !findPBI(t, nullRun, "PBI-004").Passed {
		t.Errorf("the run with a null work_key lost PBI-004 Passed")
	}
	if !findPBI(t, findRun(t, rs, "atelier-desk", "atelier-desk"), "PBI-001").Passed {
		t.Errorf("the run with work_key atelier-desk lost PBI-001 Passed")
	}
	if !findPBI(t, findRun(t, rs, "atelier-desk", "W-003"), "PBI-001").Passed {
		t.Errorf("the run with work_key W-003 lost PBI-001 Passed")
	}

	// With the other repository only, the worktree rows are not picked up.
	rs2, _, err := newTestReader(t, "alpha").Runs()
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	for _, r := range rs2 {
		if r.Repo != "alpha" {
			t.Errorf("run of repo %q found when only alpha is registered", r.Repo)
		}
	}
}

func TestSameWorkKeyAttemptsMerge(t *testing.T) {
	rd := newTestReader(t, "alpha", "atelier-desk")
	rs, _, err := rd.Runs()
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	var matching []Run
	for _, r := range rs {
		if r.Repo == "alpha" && r.WorkKey == "alpha:main:W-001" {
			matching = append(matching, r)
		}
	}
	if len(matching) != 1 {
		t.Fatalf("runs for alpha:main:W-001 = %d, want 1", len(matching))
	}
	var ids []string
	for _, p := range matching[0].PBIs {
		ids = append(ids, p.ID)
	}
	if !reflect.DeepEqual(ids, []string{"PBI-001", "PBI-002", "PBI-003"}) {
		t.Errorf("PBIs of the merged run = %v, want [PBI-001 PBI-002 PBI-003]", ids)
	}

	// cause: an array, null and a plain string all give Causes.
	p2 := findPBI(t, matching[0], "PBI-002")
	if !reflect.DeepEqual(p2.Attempts[0].Causes, []string{"model", "env"}) {
		t.Errorf("array cause = %v, want [model env]", p2.Attempts[0].Causes)
	}
	if len(p2.Attempts[1].Causes) != 0 {
		t.Errorf("null cause = %v, want none", p2.Attempts[1].Causes)
	}
	nullKey := findRun(t, rs, "alpha", "")
	p9 := findPBI(t, nullKey, "PBI-009")
	if !reflect.DeepEqual(p9.Attempts[0].Causes, []string{"env"}) {
		t.Errorf("string cause = %v, want [env]", p9.Attempts[0].Causes)
	}
	if p9.Passed {
		t.Errorf("PBI-009 was judged %s, Passed = true, want false", OutcomeUnusable)
	}
}

func TestBadRowsBecomeWarnings(t *testing.T) {
	rd := newTestReader(t, "alpha", "atelier-desk")

	rs, warns, err := rd.Runs()
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	// Broken lines of alpha runs.jsonl: a truncated object (line 6) and `null` (line 7).
	// The broken line in the archive folder of an unregistered repository is not reported.
	if len(warns) != 2 {
		t.Fatalf("Runs warnings = %v, want 2", warns)
	}
	for _, w := range warns {
		if filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(w.Path)))) != "alpha" || filepath.Base(w.Path) != "runs.jsonl" {
			t.Errorf("warning path = %q, want alpha's docs/pbi/runs.jsonl", w.Path)
		}
		if strings.Contains(w.Path, "stranger") {
			t.Errorf("warning for an unregistered archive folder: %v", w)
		}
		switch w.Line {
		case 6, 7:
		default:
			t.Errorf("warning line = %d, want 6 or 7", w.Line)
		}
		if w.Reason == "" {
			t.Errorf("warning for line %d has no reason", w.Line)
		}
	}
	// The other rows are still read.
	if len(rs) != 8 {
		t.Errorf("runs = %d, want 8 (good rows are read around the broken ones)", len(rs))
	}
	if !findPBI(t, findRun(t, rs, "alpha", "alpha:main:W-001"), "PBI-003").Held {
		t.Errorf("a row before the broken lines was lost")
	}

	// cases.jsonl: line 4 has another schema, line 5 is truncated.
	cs, cwarns, err := rd.Cases()
	if err != nil {
		t.Fatalf("Cases: %v", err)
	}
	lines := map[int]string{}
	for _, w := range cwarns {
		lines[w.Line] = w.Reason
	}
	if len(cwarns) != 2 {
		t.Fatalf("Cases warnings = %v, want 2", cwarns)
	}
	if !strings.Contains(lines[4], "atelier.case/2") {
		t.Errorf("line 4 warning = %q, want the unsupported schema atelier.case/2", lines[4])
	}
	if lines[5] == "" {
		t.Errorf("line 5 (truncated JSON) gave no warning, got %v", cwarns)
	}
	for _, c := range cs {
		if c.PBI == "PBI-005" {
			t.Errorf("a case with another schema was read: %+v", c)
		}
	}
	if len(cs) != 4 {
		t.Errorf("cases = %d, want 4 (good rows are read around the bad ones)", len(cs))
	}
}

func TestFilterCasesByCause(t *testing.T) {
	rd := newTestReader(t, "alpha")
	cs, _, err := rd.Cases()
	if err != nil {
		t.Fatalf("Cases: %v", err)
	}
	pbis := func(list []Case) []string {
		var out []string
		for _, c := range list {
			out = append(out, c.PBI)
		}
		return out
	}

	// Cases are ordered by (repo, work_key, pbi); the archive copy of PBI-003 is dropped.
	if got := pbis(cs); !reflect.DeepEqual(got, []string{"PBI-001", "PBI-002", "PBI-003", "PBI-004"}) {
		t.Fatalf("cases = %v, want [PBI-001 PBI-002 PBI-003 PBI-004]", got)
	}
	byPBI := map[string]Case{}
	for _, c := range cs {
		byPBI[c.PBI] = c
	}
	if c := byPBI["PBI-003"]; c.Archived || c.Summary != "Render list" {
		t.Errorf("PBI-003 Archived/Summary = %v/%q, want false/Render list (the repository copy wins)", c.Archived, c.Summary)
	}
	if c := byPBI["PBI-004"]; !c.Archived {
		t.Errorf("PBI-004 is only in the archive, Archived = false, want true")
	}
	if !reflect.DeepEqual(byPBI["PBI-003"].Causes, []string{"spec", "env"}) {
		t.Errorf("PBI-003 Causes = %v, want [spec env] (no duplicates)", byPBI["PBI-003"].Causes)
	}
	if len(byPBI["PBI-001"].Causes) != 0 {
		t.Errorf("PBI-001 Causes = %v, want none", byPBI["PBI-001"].Causes)
	}

	cases := []struct {
		name   string
		causes []string
		want   []string
	}{
		{"env", []string{"env"}, []string{"PBI-002", "PBI-003", "PBI-004"}},
		{"spec", []string{"spec"}, []string{"PBI-003"}},
		{"either of two", []string{"model", "spec"}, []string{"PBI-002", "PBI-003"}},
		{"unknown", []string{"nothing"}, nil},
		{"empty keeps all", nil, []string{"PBI-001", "PBI-002", "PBI-003", "PBI-004"}},
		{"empty slice keeps all", []string{}, []string{"PBI-001", "PBI-002", "PBI-003", "PBI-004"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pbis(FilterCases(cs, tc.causes))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("FilterCases(%v) = %v, want %v", tc.causes, got, tc.want)
			}
		})
	}
}
