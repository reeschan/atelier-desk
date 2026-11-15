package homestore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/reeschan/atelier-desk/internal/atelier"
)

// Recovery write states.
const (
	StatePending = "pending"
	StateDone    = "done"
	StateFailed  = "failed"
)

// recoveryName is the file name of the recovery record inside a backup directory.
const recoveryName = "recovery.json"

// RecoveryRecord is recovery.json in a backup directory. It says what a save
// meant to write and how far it got, so a half-done save can be put back by
// hand from the backup copies.
type RecoveryRecord struct {
	Kind      string          `json:"kind"`
	Name      string          `json:"name"`
	Home      string          `json:"home"`
	StartedAt string          `json:"started_at"`
	Writes    []RecoveryWrite `json:"writes"`
}

// RecoveryWrite is one write of a RecoveryRecord. Backup is the copy of the
// previous file, relative to the backup directory ("" when it did not exist).
type RecoveryWrite struct {
	Path    string `json:"path"`
	Op      string `json:"op"`
	Existed bool   `json:"existed"`
	Backup  string `json:"backup"`
	State   string `json:"state"`
}

// changedError says the bytes copied to the backup are not what the plan read.
type changedError struct{ paths []string }

func (e *changedError) Error() string {
	return "homestore: changed since the plan: " + strings.Join(e.paths, ", ")
}

// safeKind keeps a kind usable as part of a directory name.
func safeKind(kind string) string {
	var b strings.Builder
	for _, r := range kind {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "save"
	}
	return b.String()
}

// makeBackupDir creates <backupRoot>/<YYYYMMDDTHHMMSS.mmm>-<kind>[-<n>].
func (s *Store) makeBackupDir(kind string, now time.Time) (string, error) {
	if err := os.MkdirAll(s.backupRoot, 0o755); err != nil {
		return "", err
	}
	stem := now.Format("20060102T150405.000") + "-" + safeKind(kind)
	for n := 1; n < 1000; n++ {
		name := stem
		if n > 1 {
			name += "-" + strconv.Itoa(n)
		}
		dir := filepath.Join(s.backupRoot, name)
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			return dir, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("homestore: no free backup directory name for %q", stem)
}

// backup copies every existing file the writes replace or delete into a new
// backup directory under backupRoot and writes recovery.json there with every
// write pending. The sha256 of each copied file is checked against the plan's
// read of the same home path; a difference comes back as *changedError (and
// the backup directory is removed).
func (s *Store) backup(kind string, writes []atelier.PlanWrite, name string, reads []atelier.PlanRead) (dir string, err error) {
	now := time.Now()
	dir, err = s.makeBackupDir(kind, now)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
			dir = ""
		}
	}()
	planned := map[string]string{}
	for _, r := range reads {
		if r.Root == rootHome && r.Type == readFile {
			planned[r.Path] = r.SHA256
		}
	}
	rec := RecoveryRecord{Kind: kind, Name: name, Home: s.home, StartedAt: now.Format(time.RFC3339Nano)}
	var changed []string
	for _, w := range writes {
		src, err := joinRel(s.home, w.Path)
		if err != nil {
			return "", err
		}
		rw := RecoveryWrite{Path: w.Path, Op: w.Op, State: StatePending}
		got := missingSHA
		b, rerr := os.ReadFile(src)
		switch {
		case rerr == nil:
			got = bytesSHA256(b)
			rw.Existed = true
			rw.Backup = "files/" + w.Path
			dst := filepath.Join(dir, "files", filepath.FromSlash(w.Path))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(dst, b, 0o644); err != nil {
				return "", err
			}
		case errors.Is(rerr, fs.ErrNotExist):
		default:
			return "", rerr
		}
		if want, ok := planned[w.Path]; ok && want != got {
			changed = append(changed, w.Path)
		}
		rec.Writes = append(rec.Writes, rw)
	}
	if len(changed) > 0 {
		return "", &changedError{paths: uniqueSorted(changed)}
	}
	if err := writeRecovery(dir, &rec); err != nil {
		return "", err
	}
	return dir, nil
}

// writeRecovery writes rec to <dir>/recovery.json through a temporary file.
func writeRecovery(dir string, rec *RecoveryRecord) error {
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return replaceFile(filepath.Join(dir, recoveryName), append(b, '\n'))
}

// setRecoveryState records the state of write i and rewrites recovery.json.
func setRecoveryState(dir string, rec *RecoveryRecord, i int, state string) error {
	if i < 0 || i >= len(rec.Writes) {
		return nil
	}
	rec.Writes[i].State = state
	return writeRecovery(dir, rec)
}

// readRecovery loads <dir>/recovery.json.
func readRecovery(dir string) (*RecoveryRecord, error) {
	b, err := os.ReadFile(filepath.Join(dir, recoveryName))
	if err != nil {
		return nil, err
	}
	var rec RecoveryRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}
