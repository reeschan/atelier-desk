// Package runs reads the run records Atelier writes (runs.jsonl and cases.jsonl)
// from the registered repositories and from the archive root. It only reads, and
// uses the standard library only.
package runs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/reeschan/atelier-desk/internal/paths"
)

// Values of judged.outcome. They are data of the files, so they are compared by
// these constants only.
const (
	OutcomePassed    = "合格"
	OutcomeRework    = "差し戻し"
	OutcomeEscalated = "昇格"
	OutcomeUnusable  = "使えず"
	OutcomeHeld      = "保留"
)

const (
	runsFile  = "runs.jsonl"
	casesFile = "cases.jsonl"
)

// Run is one execution: the rows that share (repo, work_key). Rows whose
// work_key is null make one Run per file.
type Run struct {
	Repo     string
	WorkKey  string
	FirstAt  time.Time
	LastAt   time.Time
	Archived bool // true only when every row came from the archive
	Source   string
	PBIs     []PBIResult
}

// PBIResult is the outcome of one PBI in a Run, built from its attempts.
type PBIResult struct {
	ID          string
	Passed      bool // the last attempt passed
	Reworks     int  // the largest rework number seen
	Escalations int  // attempts judged 昇格
	Held        bool // the last attempt is 保留
	HoldReason  string
	Model       string // model of the last attempt
	Attempts    []Attempt
}

// Attempt is one row of runs.jsonl: one attempt of one PBI.
type Attempt struct {
	Report   string
	Attempt  int
	Rework   int
	Step     string
	Runner   string
	Model    string
	Outcome  string
	Causes   []string
	Findings json.RawMessage
	Verify   json.RawMessage
	Raw      json.RawMessage
}

// ReadWarning reports a line that was skipped.
type ReadWarning struct {
	Path   string
	Line   int
	Reason string
}

// Reader reads run records of the registered repositories and the archive root.
type Reader struct {
	loc paths.Locations
}

// NewReader returns a Reader that uses loc.Repos and loc.ArchiveRoot.
func NewReader(loc paths.Locations) *Reader {
	return &Reader{loc: loc}
}

type source struct {
	path     string
	archived bool
	repoName string // used when a row has no repo
	matched  bool   // the directory itself is a registered repository
}

type row struct {
	src  source
	line int
	raw  json.RawMessage
	obj  map[string]json.RawMessage
}

func (r *row) str(key string) string {
	var s string
	if v, ok := r.obj[key]; ok {
		if json.Unmarshal(v, &s) != nil {
			return ""
		}
	}
	return s
}

func objStr(obj map[string]json.RawMessage, key string) string {
	return (&row{obj: obj}).str(key)
}

func objInt(obj map[string]json.RawMessage, key string) int {
	v, ok := obj[key]
	if !ok {
		return 0
	}
	var f float64
	if json.Unmarshal(v, &f) != nil {
		return 0
	}
	return int(f)
}

// scalarString turns a JSON string or number into text; anything else gives "".
func scalarString(v json.RawMessage) string {
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s
	}
	var f float64
	if json.Unmarshal(v, &f) == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return ""
}

func (r *Reader) repoBases() map[string]bool {
	m := map[string]bool{}
	for _, repo := range r.loc.Repos {
		if repo == "" {
			continue
		}
		m[filepath.Base(filepath.Clean(repo))] = true
	}
	return m
}

// workKeyHead is the part of a work_key before its first ':'.
func workKeyHead(wk string) string {
	if i := strings.Index(wk, ":"); i >= 0 {
		return wk[:i]
	}
	return wk
}

// sources lists the files named file to read: the registered repositories first,
// then the archive root (so that a row found in a repository wins).
func (r *Reader) sources(file string) []source {
	var out []source
	for _, repo := range r.loc.Repos {
		if repo == "" {
			continue
		}
		out = append(out, source{
			path:     filepath.Join(repo, "docs", "pbi", file),
			repoName: filepath.Base(filepath.Clean(repo)),
			matched:  true,
		})
	}
	if r.loc.ArchiveRoot == "" {
		return out
	}
	bases := r.repoBases()
	dirs, err := os.ReadDir(r.loc.ArchiveRoot)
	if err != nil {
		return out
	}
	for _, d := range dirs {
		if !isDir(filepath.Join(r.loc.ArchiveRoot, d.Name())) {
			continue
		}
		repoDir := filepath.Join(r.loc.ArchiveRoot, d.Name())
		matched := bases[d.Name()]
		add := func(p string) {
			out = append(out, source{path: p, archived: true, repoName: d.Name(), matched: matched})
		}
		if fileExists(filepath.Join(repoDir, file)) {
			add(filepath.Join(repoDir, file))
		}
		subs, err := os.ReadDir(repoDir)
		if err != nil {
			continue
		}
		for _, s := range subs {
			p := filepath.Join(repoDir, s.Name(), file)
			if s.IsDir() && fileExists(p) {
				add(p)
			}
		}
	}
	return out
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// readRows reads every source of file. Broken lines are skipped with a warning.
// Rows of an archive directory that is not a registered repository are kept only
// when the head of their work_key is a registered folder name; the problems of
// such directories are not reported.
func (r *Reader) readRows(file string) ([]row, []ReadWarning) {
	bases := r.repoBases()
	var rows []row
	var warns []ReadWarning
	for _, src := range r.sources(file) {
		f, err := os.Open(src.path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) && src.matched {
				warns = append(warns, ReadWarning{Path: src.path, Reason: err.Error()})
			}
			continue
		}
		br := bufio.NewReaderSize(f, 1<<20)
		lineNo := 0
		for {
			b, rerr := br.ReadBytes('\n')
			if len(b) > 0 {
				lineNo++
				if lineNo == 1 {
					b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
				}
				b = bytes.TrimSpace(b)
				if len(b) > 0 {
					var obj map[string]json.RawMessage
					uerr := json.Unmarshal(b, &obj)
					switch {
					case uerr != nil:
						if src.matched {
							warns = append(warns, ReadWarning{Path: src.path, Line: lineNo, Reason: "invalid JSON: " + uerr.Error()})
						}
					case obj == nil:
						if src.matched {
							warns = append(warns, ReadWarning{Path: src.path, Line: lineNo, Reason: "not a JSON object"})
						}
					default:
						rw := row{src: src, line: lineNo, raw: append(json.RawMessage(nil), b...), obj: obj}
						if src.matched || bases[workKeyHead(rw.str("work_key"))] {
							rows = append(rows, rw)
						}
					}
				}
			}
			if rerr != nil {
				if rerr != io.EOF {
					warns = append(warns, ReadWarning{Path: src.path, Line: lineNo, Reason: rerr.Error()})
				}
				break
			}
		}
		f.Close()
	}
	return rows, warns
}

type runBuild struct {
	run   Run
	pbis  map[string][]timedAttempt
	order []string
	live  bool // some row came from outside the archive
}

// Runs lists the runs, ordered by repository and then newest LastAt first.
// An attempt found both in a repository and in the archive is kept once, as the
// repository one.
func (r *Reader) Runs() ([]Run, []ReadWarning, error) {
	rows, warns := r.readRows(runsFile)
	seen := map[string]bool{}
	builds := map[string]*runBuild{}
	var keys []string

	for _, rw := range rows {
		repo := rw.str("repo")
		if repo == "" {
			repo = rw.src.repoName
		}
		wk := rw.str("work_key")
		report := rw.str("report")
		attemptNo := objInt(rw.obj, "attempt")
		dk := strings.Join([]string{repo, wk, report, strconv.Itoa(attemptNo)}, "\x00")
		if seen[dk] {
			continue
		}
		seen[dk] = true

		gk := repo + "\x00" + wk
		if wk == "" {
			gk = repo + "\x00\x01file:" + rw.src.path
		}
		b := builds[gk]
		if b == nil {
			b = &runBuild{run: Run{Repo: repo, WorkKey: wk, Source: rw.src.path}, pbis: map[string][]timedAttempt{}}
			builds[gk] = b
			keys = append(keys, gk)
		}
		if !rw.src.archived {
			b.live = true
		}

		at := parseTime(rw.str("recorded_at"))
		if !at.IsZero() {
			if b.run.FirstAt.IsZero() || at.Before(b.run.FirstAt) {
				b.run.FirstAt = at
			}
			if at.After(b.run.LastAt) {
				b.run.LastAt = at
			}
		}

		a := Attempt{
			Report:  report,
			Attempt: attemptNo,
			Rework:  objInt(rw.obj, "rework"),
			Runner:  rw.str("runner"),
			Model:   rw.str("model"),
			Raw:     rw.raw,
		}
		if v, ok := rw.obj["step"]; ok {
			a.Step = scalarString(v)
		}
		if v, ok := rw.obj["verify"]; ok {
			a.Verify = v
		}
		if j, ok := rw.obj["judged"]; ok {
			var jo map[string]json.RawMessage
			if json.Unmarshal(j, &jo) == nil {
				a.Outcome = objStr(jo, "outcome")
				a.Findings = jo["findings"]
				a.Causes = flattenCauses(jo["cause"])
			}
		}
		pbi := rw.str("pbi")
		if _, ok := b.pbis[pbi]; !ok {
			b.order = append(b.order, pbi)
		}
		b.pbis[pbi] = append(b.pbis[pbi], timedAttempt{at: at, att: a})
	}

	out := make([]Run, 0, len(keys))
	for _, gk := range keys {
		b := builds[gk]
		b.run.Archived = !b.live
		for _, id := range b.order {
			b.run.PBIs = append(b.run.PBIs, buildPBIResult(id, b.pbis[id]))
		}
		out = append(out, b.run)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		if !out[i].LastAt.Equal(out[j].LastAt) {
			return out[i].LastAt.After(out[j].LastAt)
		}
		return out[i].WorkKey < out[j].WorkKey
	})
	return out, warns, nil
}

// timedAttempt keeps the recorded_at of an attempt (zero when missing or unreadable).
type timedAttempt struct {
	at  time.Time
	att Attempt
}

// buildPBIResult orders the attempts by recorded_at (then rework, then attempt,
// then read order; rows without a readable time come first) and takes the
// result from the newest one.
func buildPBIResult(id string, timed []timedAttempt) PBIResult {
	sort.SliceStable(timed, func(i, j int) bool {
		a, b := timed[i], timed[j]
		if !a.at.Equal(b.at) {
			return a.at.Before(b.at)
		}
		if a.att.Rework != b.att.Rework {
			return a.att.Rework < b.att.Rework
		}
		return a.att.Attempt < b.att.Attempt
	})
	atts := make([]Attempt, len(timed))
	for i, t := range timed {
		atts[i] = t.att
	}
	res := PBIResult{ID: id, Attempts: atts}
	for _, a := range atts {
		if a.Rework > res.Reworks {
			res.Reworks = a.Rework
		}
		if a.Outcome == OutcomeEscalated {
			res.Escalations++
		}
	}
	if n := len(atts); n > 0 {
		last := atts[n-1]
		res.Passed = last.Outcome == OutcomePassed
		res.Model = last.Model
		if last.Outcome == OutcomeHeld {
			res.Held = true
			res.HoldReason = strings.Join(last.Causes, ",")
		}
	}
	return res
}

// flattenCauses reads judged.cause / result.attempts[].cause: null, a string or an array of strings.
func flattenCauses(v json.RawMessage) []string {
	if len(v) == 0 {
		return nil
	}
	var list []string
	if json.Unmarshal(v, &list) == nil {
		return list
	}
	var one string
	if json.Unmarshal(v, &one) == nil && one != "" {
		return []string{one}
	}
	return nil
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	return time.Time{}
}
