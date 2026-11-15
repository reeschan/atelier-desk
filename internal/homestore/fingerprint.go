package homestore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/reeschan/atelier-desk/internal/atelier"
)

// missingSHA is the fingerprint of a file that does not exist (Atelier's "missing").
const missingSHA = "missing"

const (
	rootHome    = "home"
	rootAtelier = "atelier"
	readFile    = "file"
	readDir     = "directory"
)

// FileSHA256 returns the lowercase hex sha256 of the raw bytes of the file at
// path, or "missing" when there is no file there.
func FileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return missingSHA, nil
		}
		return "", err
	}
	return bytesSHA256(b), nil
}

func bytesSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// isTempFile reports whether the base name is a temporary file name
// (".<name>.<random>.tmp") written by Studio or by this package.
func isTempFile(name string) bool {
	return len(name) > len(".tmp") && strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".tmp")
}

// isJSONName reports whether a directory entry name counts as "*.json".
func isJSONName(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".json")
}

// resolveRoot returns the directory a plan read's root refers to.
func resolveRoot(home, atelierDir, root string) (string, error) {
	switch root {
	case rootHome:
		return home, nil
	case rootAtelier:
		return atelierDir, nil
	}
	return "", fmt.Errorf("homestore: unknown read root %q", root)
}

// joinRel joins a "/"-separated relative path to base, refusing ".." and
// absolute paths.
func joinRel(base, rel string) (string, error) {
	p := filepath.FromSlash(rel)
	if rel == "" {
		return base, nil
	}
	if !filepath.IsLocal(p) {
		return "", fmt.Errorf("homestore: path %q is not local", rel)
	}
	return filepath.Join(base, p), nil
}

// dirEntries returns the "*.json" regular files directly under dir as name ->
// sha256, and whether the directory is missing.
func dirEntries(dir string) (map[string]string, bool, error) {
	list, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, true, nil
		}
		return nil, false, err
	}
	out := map[string]string{}
	for _, e := range list {
		if !isJSONName(e.Name()) {
			continue
		}
		if !e.Type().IsRegular() {
			// Atelier refuses such entries; a different type is a change.
			out[e.Name()] = "not-a-file"
			continue
		}
		sum, err := FileSHA256(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, false, err
		}
		if sum != missingSHA {
			out[e.Name()] = sum
		}
	}
	return out, false, nil
}

// expectedFile returns the fingerprint a home file should have: the plan's
// value, or the effect of the last applied write to the same path.
func expectedFile(r atelier.PlanRead, applied []atelier.PlanWrite) string {
	want := r.SHA256
	if r.Root != rootHome {
		return want
	}
	for _, w := range applied {
		if w.Path != r.Path {
			continue
		}
		if w.Op == opDelete {
			want = missingSHA
		} else {
			want = bytesSHA256([]byte(w.ContentUTF8))
		}
	}
	return want
}

// expectedDir returns the entries (name -> sha256) a directory dependency
// should have after the applied writes.
func expectedDir(r atelier.PlanRead, applied []atelier.PlanWrite) map[string]string {
	want := map[string]string{}
	for _, e := range r.Entries {
		want[e.Name] = e.SHA256
	}
	if r.Root != rootHome {
		return want
	}
	for _, w := range applied {
		dir, name := splitSlash(w.Path)
		if dir != r.Path || !isJSONName(name) {
			continue
		}
		if w.Op == opDelete {
			delete(want, name)
		} else {
			want[name] = bytesSHA256([]byte(w.ContentUTF8))
		}
	}
	return want
}

// splitSlash splits a "/"-separated path into its parent ("" for the root) and
// base name.
func splitSlash(p string) (string, string) {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i], p[i+1:]
	}
	return "", p
}

func sameEntries(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// VerifyReads recomputes every read of a plan and returns the paths whose
// fingerprint differs from what is expected. The expectation is the plan's
// value with the effect of applied (the writes already replaced or deleted by
// this save, in order) laid on top. A "directory" read compares its entries.
func VerifyReads(home, atelierDir string, reads []atelier.PlanRead, applied []atelier.PlanWrite) ([]string, error) {
	var changed []string
	for _, r := range reads {
		base, err := resolveRoot(home, atelierDir, r.Root)
		if err != nil {
			return nil, err
		}
		p, err := joinRel(base, r.Path)
		if err != nil {
			return nil, err
		}
		switch r.Type {
		case readFile:
			got, err := FileSHA256(p)
			if err != nil {
				return nil, err
			}
			if got != expectedFile(r, applied) {
				changed = append(changed, r.Path)
			}
		case readDir:
			// Only the entries are compared: a missing directory and an empty
			// one plan the same, and the replace itself may create the parent.
			got, _, err := dirEntries(p)
			if err != nil {
				return nil, err
			}
			if !sameEntries(got, expectedDir(r, applied)) {
				changed = append(changed, r.Path)
			}
		default:
			return nil, fmt.Errorf("homestore: unknown read type %q for %q", r.Type, r.Path)
		}
	}
	return uniqueSorted(changed), nil
}

func uniqueSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
