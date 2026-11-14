// Package paths gathers every location Atelier Desk reads from into one value,
// Locations, which is built once at start-up and handed to the other modules.
// It imports no other package of this module.
package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	// ErrVersionNotFound means the directory of the requested Atelier version does not exist.
	ErrVersionNotFound = errors.New("paths: atelier version not found")
	// ErrNoActive means the ACTIVE file is missing or empty.
	ErrNoActive = errors.New("paths: atelier ACTIVE is missing or empty")
)

// versionName matches v<number>(.<number>)*.
var versionName = regexp.MustCompile(`^v[0-9]+(\.[0-9]+)*$`)

// Locations is the set of places Desk reads from.
type Locations struct {
	UserHome             string
	SkillsDir            string
	PluginsCacheDir      string
	InstalledPluginsFile string // <UserHome>/.claude/plugins/installed_plugins.json
	AtelierSkillDir      string
	Version              string
	VersionDir           string
	Home                 string
	ArchiveRoot          string
	Repos                []string
}

// Options are the inputs of Resolve.
type Options struct {
	UserHome string // empty: os.UserHomeDir()
	Version  string // empty: the ACTIVE version
	Repos    []string
}

// Resolve builds Locations. It only reads the file system.
func Resolve(opts Options) (Locations, error) {
	userHome := opts.UserHome
	if userHome == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return Locations{}, fmt.Errorf("paths: resolve user home: %w", err)
		}
		userHome = h
	}
	claude := filepath.Join(userHome, ".claude")
	skills := filepath.Join(claude, "skills")
	atelier := filepath.Join(skills, "atelier")

	archive := os.Getenv("PBI_RUNS_ARCHIVE")
	if archive == "" {
		archive = filepath.Join(claude, "pbi-runs")
	}

	l := Locations{
		UserHome:             userHome,
		SkillsDir:            skills,
		PluginsCacheDir:      filepath.Join(claude, "plugins", "cache"),
		InstalledPluginsFile: filepath.Join(claude, "plugins", "installed_plugins.json"),
		AtelierSkillDir:      atelier,
		ArchiveRoot:          archive,
		Repos:                append([]string(nil), opts.Repos...),
	}

	version := opts.Version
	if version == "" {
		v, err := ActiveVersion(atelier)
		if err != nil {
			return Locations{}, err
		}
		version = v
	}
	return l.WithVersion(version)
}

// ActiveVersion reads the one line of <atelierSkillDir>/ACTIVE and trims it.
// A missing or empty file gives ErrNoActive.
func ActiveVersion(atelierSkillDir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(atelierSkillDir, "ACTIVE"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: %s", ErrNoActive, filepath.Join(atelierSkillDir, "ACTIVE"))
		}
		return "", err
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return "", fmt.Errorf("%w: %s", ErrNoActive, filepath.Join(atelierSkillDir, "ACTIVE"))
	}
	return v, nil
}

// ListVersions lists the directories under <atelierSkillDir>/versions whose name
// is v<number>(.<number>)*, in numeric order (v7.10 comes after v7.9).
// A missing versions directory gives an empty list.
func ListVersions(atelierSkillDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(atelierSkillDir, "versions"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			continue
		}
		if !versionName.MatchString(e.Name()) {
			continue
		}
		// resolve symlinks: only a directory counts
		if fi, err := os.Stat(filepath.Join(atelierSkillDir, "versions", e.Name())); err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Slice(out, func(i, j int) bool { return compareVersions(out[i], out[j]) < 0 })
	return out, nil
}

// compareVersions compares two names that match versionName, number by number.
func compareVersions(a, b string) int {
	pa := strings.Split(strings.TrimPrefix(a, "v"), ".")
	pb := strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = atoi(pa[i])
		}
		if i < len(pb) {
			y = atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// WithVersion returns a copy pointing at version v. It fails with ErrVersionNotFound
// when the version directory does not exist.
func (l Locations) WithVersion(v string) (Locations, error) {
	if !versionName.MatchString(v) {
		return Locations{}, fmt.Errorf("%w: %q", ErrVersionNotFound, v)
	}
	dir := filepath.Join(l.AtelierSkillDir, "versions", v)
	fi, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Locations{}, fmt.Errorf("%w: %s", ErrVersionNotFound, dir)
		}
		return Locations{}, err
	}
	if !fi.IsDir() {
		return Locations{}, fmt.Errorf("%w: %s is not a directory", ErrVersionNotFound, dir)
	}
	l.Version = v
	l.VersionDir = dir
	return l, nil
}

// WithHome returns a copy with Home set (the home Atelier's desk_state reported).
func (l Locations) WithHome(home string) Locations {
	l.Home = home
	return l
}

// WithRepos returns a copy with the registered repository roots replaced.
func (l Locations) WithRepos(repos []string) Locations {
	l.Repos = append([]string(nil), repos...)
	return l
}
