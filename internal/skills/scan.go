// Package skills reads the skills Claude Code can see: the user's own under
// ~/.claude/skills and the ones inside installed plugins. It reads SKILL.md and
// its frontmatter directly (this is not an Atelier rule, so Atelier is not asked).
// It never writes.
package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/reeschan/atelier-desk/internal/paths"
)

// maxReadSize is the largest auxiliary file ReadFile returns.
const maxReadSize = 1 << 20

// Origin tells where a skill comes from.
type Origin string

const (
	OriginUser   Origin = "user"
	OriginPlugin Origin = "plugin"
)

// Skill is one entry of the skill list.
//
// Key is "user:<name>" or "plugin:<plugin>:<name>", with name taken from the
// frontmatter (the directory name when it has none). The catalog (PBI-007) uses
// Key to tie a component to a skill.
//
// RelPath is the path of SkillFile with "/" separators: from the plugin root
// for a plugin skill, from the skills directory for a user skill.
// For a versioned skill (an ACTIVE file), SkillFile is versions/<ACTIVE>/SKILL.md
// and Dir is the skill's own directory.
type Skill struct {
	Key                    string
	Name                   string
	Description            string
	Origin                 Origin
	Plugin                 string
	PluginVersion          string
	RelPath                string
	Dir                    string
	SkillFile              string
	DisableModelInvocation bool
	Versioned              bool
}

// ScanWarning is a SKILL.md that was skipped.
type ScanWarning struct {
	Path   string
	Reason string
}

// AuxFile is a file of a skill other than its SKILL.md. RelPath is relative to
// the directory of SkillFile, with "/" separators.
type AuxFile struct {
	RelPath string
	Size    int64
}

// SkillDetail is one skill in full.
type SkillDetail struct {
	Skill       Skill
	Frontmatter map[string]any
	Body        string
	Files       []AuxFile
	Versions    *VersionInfo // nil when the skill is not versioned
}

// ErrorCode names why Detail or ReadFile failed.
type ErrorCode string

const (
	ErrNotFound    ErrorCode = "not_found"
	ErrInvalidPath ErrorCode = "invalid_path"
	ErrTooLarge    ErrorCode = "too_large"
	ErrBinary      ErrorCode = "binary"
)

// Error is a failed Detail or ReadFile.
type Error struct {
	Code    ErrorCode
	Message string
}

func (e *Error) Error() string { return "skills: " + string(e.Code) + ": " + e.Message }

// CodeOf returns the ErrorCode inside err, or "" when there is none.
func CodeOf(err error) ErrorCode {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// Scanner reads skills from the places in a paths.Locations.
type Scanner struct {
	loc paths.Locations
}

// NewScanner makes a Scanner. It reads only SkillsDir and PluginsCacheDir of loc.
func NewScanner(loc paths.Locations) *Scanner { return &Scanner{loc: loc} }

var skippedDirs = map[string]bool{"node_modules": true, ".git": true}

// Scan lists every skill, ordered by Key. A SKILL.md that cannot be read or
// parsed is left out and reported as a ScanWarning. A missing skills or plugin
// directory is not an error.
func (s *Scanner) Scan() ([]Skill, []ScanWarning, error) {
	var skills []Skill
	var warns []ScanWarning
	if err := s.scanUser(&skills, &warns); err != nil {
		return nil, nil, err
	}
	if err := s.scanPlugins(&skills, &warns); err != nil {
		return nil, nil, err
	}
	sort.SliceStable(skills, func(i, j int) bool { return skills[i].Key < skills[j].Key })
	// a duplicate Key cannot be told apart later: keep the first, warn about the rest
	out := skills[:0]
	seen := map[string]bool{}
	for _, k := range skills {
		if seen[k.Key] {
			warns = append(warns, ScanWarning{Path: k.SkillFile, Reason: "duplicate skill key " + k.Key})
			continue
		}
		seen[k.Key] = true
		out = append(out, k)
	}
	sort.SliceStable(warns, func(i, j int) bool { return warns[i].Path < warns[j].Path })
	return out, warns, nil
}

func (s *Scanner) scanUser(skills *[]Skill, warns *[]ScanWarning) error {
	entries, err := os.ReadDir(s.loc.SkillsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("skills: read %s: %w", s.loc.SkillsDir, err)
	}
	for _, e := range entries {
		dir := filepath.Join(s.loc.SkillsDir, e.Name())
		if skippedDirs[e.Name()] || !isDir(dir) {
			continue
		}
		file := filepath.Join(dir, "SKILL.md")
		versioned := false
		if active, err := paths.ActiveVersion(dir); err == nil {
			if strings.ContainsAny(active, "/\\") || active == ".." || active == "." {
				*warns = append(*warns, ScanWarning{Path: filepath.Join(dir, "ACTIVE"), Reason: "ACTIVE is not a version name"})
				continue
			}
			versioned = true
			file = filepath.Join(dir, "versions", active, "SKILL.md")
		} else if errors.Is(err, paths.ErrNoActive) && fileExists(filepath.Join(dir, "ACTIVE")) {
			*warns = append(*warns, ScanWarning{Path: filepath.Join(dir, "ACTIVE"), Reason: "ACTIVE is empty"})
			continue
		} else if !errors.Is(err, paths.ErrNoActive) {
			*warns = append(*warns, ScanWarning{Path: filepath.Join(dir, "ACTIVE"), Reason: err.Error()})
			continue
		}
		if !fileExists(file) {
			if versioned {
				*warns = append(*warns, ScanWarning{Path: file, Reason: "the ACTIVE version has no SKILL.md"})
			}
			continue // a stray directory: not a skill
		}
		var fm map[string]any
		if versioned {
			fm, _, err = readVersionedSkill(dir, file)
		} else {
			fm, _, err = readSkillFile(file)
		}
		if err != nil {
			*warns = append(*warns, ScanWarning{Path: file, Reason: err.Error()})
			continue
		}
		name := stringOf(fm["name"])
		if name == "" {
			name = e.Name()
		}
		rel, _ := filepath.Rel(s.loc.SkillsDir, file)
		*skills = append(*skills, Skill{
			Key:                    "user:" + name,
			Name:                   name,
			Description:            stringOf(fm["description"]),
			Origin:                 OriginUser,
			RelPath:                filepath.ToSlash(rel),
			Dir:                    dir,
			SkillFile:              file,
			DisableModelInvocation: boolOf(fm["disable-model-invocation"]),
			Versioned:              versioned,
		})
	}
	return nil
}

func (s *Scanner) scanPlugins(skills *[]Skill, warns *[]ScanWarning) error {
	mkts, err := os.ReadDir(s.loc.PluginsCacheDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("skills: read %s: %w", s.loc.PluginsCacheDir, err)
	}
	for _, m := range mkts {
		mdir := filepath.Join(s.loc.PluginsCacheDir, m.Name())
		if strings.HasPrefix(m.Name(), "temp_git_") || !isDir(mdir) {
			continue
		}
		plugins, err := os.ReadDir(mdir)
		if err != nil {
			*warns = append(*warns, ScanWarning{Path: mdir, Reason: err.Error()})
			continue
		}
		for _, p := range plugins {
			pdir := filepath.Join(mdir, p.Name())
			if !isDir(pdir) {
				continue
			}
			root, version, ok := newestVersionDir(pdir)
			if !ok {
				continue
			}
			s.scanPluginRoot(root, version, pluginName(root, p.Name()), skills, warns)
		}
	}
	return nil
}

// newestVersionDir picks the version directory of a plugin with the newest mtime.
func newestVersionDir(pdir string) (dir, version string, ok bool) {
	entries, err := os.ReadDir(pdir)
	if err != nil {
		return "", "", false
	}
	var best os.FileInfo
	for _, e := range entries {
		fi, err := os.Stat(filepath.Join(pdir, e.Name()))
		if err != nil || !fi.IsDir() {
			continue
		}
		if best == nil || fi.ModTime().After(best.ModTime()) ||
			(fi.ModTime().Equal(best.ModTime()) && fi.Name() > best.Name()) {
			best = fi
		}
	}
	if best == nil {
		return "", "", false
	}
	return filepath.Join(pdir, best.Name()), best.Name(), true
}

// pluginName is the "name" of .claude-plugin/plugin.json, or fallback.
func pluginName(root, fallback string) string {
	b, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		return fallback
	}
	var v struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(b, &v) != nil || strings.TrimSpace(v.Name) == "" {
		return fallback
	}
	return strings.TrimSpace(v.Name)
}

func (s *Scanner) scanPluginRoot(root, version, plugin string, skills *[]Skill, warns *[]ScanWarning) {
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				*warns = append(*warns, ScanWarning{Path: p, Reason: err.Error()})
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if p != root && (skippedDirs[d.Name()] || d.Name() == "versions") {
			return fs.SkipDir
		}
		file := filepath.Join(p, "SKILL.md")
		if !fileExists(file) {
			return nil
		}
		fm, _, err := readSkillFile(file)
		if err != nil {
			*warns = append(*warns, ScanWarning{Path: file, Reason: err.Error()})
			return fs.SkipDir
		}
		name := stringOf(fm["name"])
		if name == "" {
			name = filepath.Base(p)
		}
		rel, _ := filepath.Rel(root, file)
		*skills = append(*skills, Skill{
			Key:                    "plugin:" + plugin + ":" + name,
			Name:                   name,
			Description:            stringOf(fm["description"]),
			Origin:                 OriginPlugin,
			Plugin:                 plugin,
			PluginVersion:          version,
			RelPath:                filepath.ToSlash(rel),
			Dir:                    p,
			SkillFile:              file,
			DisableModelInvocation: boolOf(fm["disable-model-invocation"]),
		})
		return fs.SkipDir
	})
}

// Detail returns one skill with its frontmatter, body, auxiliary files and,
// for a versioned skill, its version information.
func (s *Scanner) Detail(key string) (SkillDetail, error) {
	sk, err := s.find(key)
	if err != nil {
		return SkillDetail{}, err
	}
	fm, body, err := s.readSkill(sk)
	if err != nil {
		return SkillDetail{}, err
	}
	d := SkillDetail{Skill: sk, Frontmatter: fm, Body: body}
	if d.Files, err = listAuxFiles(filepath.Dir(sk.SkillFile), sk.SkillFile); err != nil {
		return SkillDetail{}, err
	}
	if sk.Versioned {
		if d.Versions, err = ReadVersions(sk.Dir); err != nil {
			return SkillDetail{}, err
		}
	}
	return d, nil
}

// ReadFile returns the text of an auxiliary file. rel is relative to the
// directory of the skill's SKILL.md (versions/<ACTIVE>/ for a versioned skill)
// and must stay inside it. Files over 1 MiB (too_large) and non-text files
// (binary) are errors.
func (s *Scanner) ReadFile(key, rel string) (string, error) {
	sk, err := s.find(key)
	if err != nil {
		return "", err
	}
	base := filepath.Dir(sk.SkillFile)
	slash := strings.ReplaceAll(rel, "\\", "/")
	clean := path.Clean(slash)
	if rel == "" || strings.HasPrefix(slash, "/") || clean == "." || clean == ".." ||
		strings.HasPrefix(clean, "../") || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" ||
		strings.ContainsRune(rel, 0) {
		return "", &Error{ErrInvalidPath, fmt.Sprintf("%q is outside the skill directory", rel)}
	}
	full := filepath.Join(base, filepath.FromSlash(clean))
	// a symbolic link must not lead out of the skill directory
	realBase, err1 := filepath.EvalSymlinks(base)
	realFull, err2 := filepath.EvalSymlinks(full)
	if err2 != nil {
		if errors.Is(err2, fs.ErrNotExist) {
			return "", &Error{ErrNotFound, rel + " does not exist"}
		}
		return "", err2
	}
	if err1 == nil {
		r, err := filepath.Rel(realBase, realFull)
		if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			return "", &Error{ErrInvalidPath, fmt.Sprintf("%q is outside the skill directory", rel)}
		}
	}
	fi, err := os.Stat(realFull)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", &Error{ErrNotFound, rel + " is a directory"}
	}
	if fi.Size() > maxReadSize {
		return "", &Error{ErrTooLarge, fmt.Sprintf("%s is %d bytes (limit %d)", rel, fi.Size(), maxReadSize)}
	}
	b, err := os.ReadFile(realFull)
	if err != nil {
		return "", err
	}
	if len(b) > maxReadSize {
		return "", &Error{ErrTooLarge, rel + " is over the size limit"}
	}
	if !utf8.Valid(b) || containsNUL(b) {
		return "", &Error{ErrBinary, rel + " is not a text file"}
	}
	return string(b), nil
}

func (s *Scanner) find(key string) (Skill, error) {
	all, _, err := s.Scan()
	if err != nil {
		return Skill{}, err
	}
	for _, k := range all {
		if k.Key == key {
			return k, nil
		}
	}
	return Skill{}, &Error{ErrNotFound, "no skill " + key}
}

// listAuxFiles lists every file under base except the SKILL.md at skillFile.
func listAuxFiles(base, skillFile string) ([]AuxFile, error) {
	var out []AuxFile
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable entry is left out
		}
		if d.IsDir() {
			if p != base && skippedDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if p == skillFile {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(base, p)
		if err != nil {
			return nil
		}
		out = append(out, AuxFile{RelPath: filepath.ToSlash(rel), Size: fi.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out, nil
}

// readSkillFile reads a SKILL.md. A frontmatter that is not valid YAML is read
// line by line as a fallback (see parseFrontmatterLenient) so one loose
// description does not hide a skill.
func readSkillFile(file string) (map[string]any, string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, "", err
	}
	fm, body, err := ParseFrontmatter(b)
	if err != nil {
		if m, bd, ok := parseFrontmatterLenient(b); ok {
			return m, bd, nil
		}
		return nil, "", err
	}
	return fm, body, nil
}

// readVersionedSkill reads versions/<ACTIVE>/SKILL.md. When that file lacks
// name or description, the relay SKILL.md of the skill directory fills the
// missing keys (some versioned skills keep their frontmatter only there).
func readVersionedSkill(dir, file string) (map[string]any, string, error) {
	fm, body, err := readSkillFile(file)
	if err != nil {
		return nil, "", err
	}
	if stringOf(fm["name"]) != "" && stringOf(fm["description"]) != "" {
		return fm, body, nil
	}
	if relay, _, err := readSkillFile(filepath.Join(dir, "SKILL.md")); err == nil {
		for k, v := range relay {
			if _, has := fm[k]; !has {
				fm[k] = v
			}
		}
	}
	return fm, body, nil
}

func (s *Scanner) readSkill(sk Skill) (map[string]any, string, error) {
	if sk.Versioned {
		return readVersionedSkill(sk.Dir, sk.SkillFile)
	}
	return readSkillFile(sk.SkillFile)
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func containsNUL(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

func stringOf(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func boolOf(v any) bool {
	b, _ := v.(bool)
	return b
}
