package homestore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/reeschan/atelier-desk/internal/atelier"
)

// Plan write operations in the CLI contract. Nothing else is applied.
const (
	opReplace = "replace"
	opDelete  = "delete"
)

// Store saves personal settings into one Atelier home. It asks Atelier to
// validate and plan, checks the fingerprints of what the plan read before the
// save and right before every replace, keeps the previous version, and writes
// each file through a temporary file and a rename. It takes no lock: a change
// right after the last check, and the all-or-nothing of several files, are not
// guaranteed.
type Store struct {
	client     atelier.Client
	home       string
	atelierDir string
	backupRoot string
	mu         sync.Mutex
}

// NewStore returns a Store writing into home. atelierDir is the Atelier
// version directory (the base of root "atelier" reads). backupRoot is where
// backups go (<app config dir>/backups).
func NewStore(c atelier.Client, home string, atelierDir string, backupRoot string) *Store {
	return &Store{client: c, home: home, atelierDir: atelierDir, backupRoot: backupRoot}
}

// Baseline is the fingerprint of every file in the home at one moment. Files
// maps the "/"-separated home-relative path to the sha256 in hex; a file that
// does not exist has no key.
type Baseline struct {
	Taken time.Time
	Files map[string]string
}

// Snapshot fingerprints every file in the home, leaving out temporary files
// (".<name>.<random>.tmp"). A home that does not exist gives an empty baseline.
func (s *Store) Snapshot() (Baseline, error) {
	b := Baseline{Taken: time.Now(), Files: map[string]string{}}
	err := filepath.WalkDir(s.home, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				if p == s.home {
					return fs.SkipAll
				}
				return nil
			}
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() || isTempFile(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(s.home, p)
		if err != nil {
			return err
		}
		sum, err := FileSHA256(p)
		if err != nil {
			return err
		}
		if sum != missingSHA {
			b.Files[filepath.ToSlash(rel)] = sum
		}
		return nil
	})
	if err != nil {
		return Baseline{}, err
	}
	return b, nil
}

// SaveRequest is one save. Base is the baseline taken when the setting was
// opened; to overwrite an external change, send the Current of the previous
// SaveResult as Base.
type SaveRequest struct {
	Kind  string
	Name  string
	Input json.RawMessage
	Base  Baseline
}

// Outcome says how a save ended.
type Outcome string

const (
	Saved          Outcome = "saved"
	Rejected       Outcome = "rejected"
	ExternalChange Outcome = "external_change"
	PartialFailure Outcome = "partial_failure"
)

// SaveResult is what Save did. Problems are Atelier's own texts (Rejected).
// Changed are the paths found changed outside. Written are the paths replaced
// or deleted, Failed the write that did not happen (PartialFailure). Current is
// the fingerprint right now (ExternalChange, PartialFailure); Base (Saved) is
// the request's Base with this save's own writes applied, to send as the next
// Base. It never takes in a change made outside.
type SaveResult struct {
	Outcome      Outcome
	Problems     []string
	Changed      []string
	Written      []string
	Failed       string
	BackupDir    string
	RecoveryFile string
	Current      Baseline
	Base         Baseline
}

// Save validates, plans and applies one save. See SaveResult for the
// outcomes. An error means nothing was written (or, with a PartialFailure
// result, that the rest was not).
func (s *Store) Save(ctx context.Context, req SaveRequest) (SaveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		return SaveResult{}, errors.New("homestore: no Atelier client")
	}
	areq := atelier.Request{Kind: req.Kind, Name: req.Name, Input: req.Input}

	// (1) validate
	v, err := s.client.Validate(ctx, areq)
	if err != nil {
		return SaveResult{}, err
	}
	if !v.Valid {
		return rejected(v.Problems), nil
	}

	// (2) plan
	pr, err := s.client.Plan(ctx, areq)
	if err != nil {
		return SaveResult{}, err
	}
	if !pr.Validation.Valid {
		return rejected(pr.Validation.Problems), nil
	}
	if pr.Plan == nil {
		return SaveResult{}, errors.New("homestore: Atelier returned no save plan")
	}
	plan := pr.Plan
	if err := checkWrites(plan.Writes); err != nil {
		return SaveResult{}, err
	}

	// (3) check before the save: now == plan, and home reads == Base.
	changed, err := VerifyReads(s.home, s.atelierDir, plan.Reads, nil)
	if err != nil {
		return SaveResult{}, err
	}
	changed = uniqueSorted(append(changed, baseMismatch(plan.Reads, req.Base)...))
	if len(changed) > 0 {
		return s.externalChange(changed, SaveResult{})
	}

	// (6) nothing to write
	if len(plan.Writes) == 0 {
		return saved(SaveResult{}, req.Base, nil), nil
	}

	// (4) backup
	dir, err := s.backup(req.Kind, plan.Writes, req.Name, plan.Reads)
	if err != nil {
		var ce *changedError
		if errors.As(err, &ce) {
			return s.externalChange(ce.paths, SaveResult{})
		}
		return SaveResult{}, err
	}
	res := SaveResult{BackupDir: dir, RecoveryFile: filepath.Join(dir, recoveryName)}
	rec, err := readRecovery(dir)
	if err != nil {
		return res, err
	}

	// (5) each write: temp file -> check every read -> rename / remove.
	var applied []atelier.PlanWrite
	for i, w := range plan.Writes {
		changed, err := s.applyOne(w, plan.Reads, applied)
		if err != nil || len(changed) > 0 {
			_ = setRecoveryState(dir, rec, i, StateFailed)
			if len(applied) == 0 {
				if err != nil {
					return res, err
				}
				return s.externalChange(changed, res)
			}
			res.Outcome = PartialFailure
			res.Failed = w.Path
			res.Changed = changed
			if cur, serr := s.Snapshot(); serr == nil {
				res.Current = cur
			}
			return res, nil
		}
		applied = append(applied, w)
		res.Written = append(res.Written, w.Path)
		// The home write itself succeeded; a stale record is the lesser harm.
		_ = setRecoveryState(dir, rec, i, StateDone)
	}

	// (7) saved: Base = what the caller saw + this save's own writes.
	return saved(res, req.Base, applied), nil
}

// applyOne writes one plan write. It returns the changed paths when the last
// check right before the rename found an external change (nothing replaced).
func (s *Store) applyOne(w atelier.PlanWrite, reads []atelier.PlanRead, applied []atelier.PlanWrite) ([]string, error) {
	target, err := joinRel(s.home, w.Path)
	if err != nil {
		return nil, err
	}
	tmp := ""
	if w.Op == opReplace {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		tmp, err = writeTemp(target, []byte(w.ContentUTF8))
		if err != nil {
			return nil, err
		}
	}
	// Last check right before the replace (the gap after it is accepted).
	changed, err := VerifyReads(s.home, s.atelierDir, reads, applied)
	if err != nil || len(changed) > 0 {
		if tmp != "" {
			os.Remove(tmp)
		}
		return changed, err
	}
	if w.Op == opDelete {
		if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		return nil, nil
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return nil, err
	}
	return nil, nil
}

func rejected(problems []string) SaveResult {
	if problems == nil {
		problems = []string{}
	}
	return SaveResult{Outcome: Rejected, Problems: problems}
}

func (s *Store) externalChange(changed []string, res SaveResult) (SaveResult, error) {
	cur, err := s.Snapshot()
	if err != nil {
		return SaveResult{}, err
	}
	res.Outcome = ExternalChange
	res.Changed = changed
	res.Current = cur
	return res, nil
}

// saved builds the post-save Base from what the caller already saw (req.Base)
// plus this save's own writes: a replaced path gets the sha256 of the bytes
// written, a deleted path leaves Files. Paths changed outside meanwhile keep
// the caller's fingerprint, so the next save still finds them changed.
func saved(res SaveResult, base Baseline, applied []atelier.PlanWrite) SaveResult {
	files := make(map[string]string, len(base.Files)+len(applied))
	for p, sum := range base.Files {
		files[p] = sum
	}
	for _, w := range applied {
		switch w.Op {
		case opReplace:
			files[w.Path] = bytesSHA256([]byte(w.ContentUTF8))
		case opDelete:
			delete(files, w.Path)
		}
	}
	res.Outcome = Saved
	res.Base = Baseline{Taken: time.Now(), Files: files}
	return res
}

// checkWrites refuses an op other than replace/delete and a path that is not
// local to the home ("..", absolute). It runs before anything is written.
func checkWrites(writes []atelier.PlanWrite) error {
	for _, w := range writes {
		if w.Op != opReplace && w.Op != opDelete {
			return fmt.Errorf("homestore: write op %q for %q is not supported", w.Op, w.Path)
		}
		if w.Path == "" || strings.Contains(w.Path, `\`) || !filepath.IsLocal(filepath.FromSlash(w.Path)) {
			return fmt.Errorf("homestore: write path %q is not inside the home", w.Path)
		}
	}
	return nil
}

// baseMismatch returns the home reads whose plan fingerprint differs from
// the baseline the caller opened (or chose to overwrite) with.
func baseMismatch(reads []atelier.PlanRead, base Baseline) []string {
	var out []string
	for _, r := range reads {
		if r.Root != rootHome {
			continue
		}
		switch r.Type {
		case readFile:
			got, ok := base.Files[r.Path]
			if !ok {
				got = missingSHA
			}
			if got != r.SHA256 {
				out = append(out, r.Path)
			}
		case readDir:
			want := map[string]string{}
			for _, e := range r.Entries {
				want[e.Name] = e.SHA256
			}
			if !sameEntries(baseEntries(base, r.Path), want) {
				out = append(out, r.Path)
			}
		default:
			out = append(out, r.Path)
		}
	}
	return out
}

// baseEntries builds a directory read's entries from the baseline: the
// "*.json" files directly under dir.
func baseEntries(base Baseline, dir string) map[string]string {
	out := map[string]string{}
	for p, sum := range base.Files {
		parent, name := splitSlash(p)
		if parent == dir && isJSONName(name) {
			out[name] = sum
		}
	}
	return out
}

// writeTemp writes data to ".<name>.<random>.tmp" next to target.
func writeTemp(target string, data []byte) (string, error) {
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", err
	}
	tmp := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+"."+hex.EncodeToString(rnd[:])+".tmp")
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return tmp, nil
}

// replaceFile writes data to path through a temporary file and a rename.
func replaceFile(path string, data []byte) error {
	tmp, err := writeTemp(path, data)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
