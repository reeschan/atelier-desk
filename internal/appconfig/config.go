// Package appconfig reads and writes the app's own settings
// (%APPDATA%/atelier-desk/config.json), which are separate from Atelier's home.
package appconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config is the content of config.json.
type Config struct {
	PythonPath  string   `json:"python_path"`
	Repos       []string `json:"repos"`
	LastVersion string   `json:"last_version"`
}

// DefaultPath is <os.UserConfigDir()>/atelier-desk/config.json (%APPDATA% on Windows).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "atelier-desk", "config.json"), nil
}

// Load reads the config. A missing file gives an empty Config and nil.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Save writes the config to a temporary file in the same directory and renames it
// over path, so a reader never sees a half-written file.
func Save(path string, c Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// normalize makes dir absolute and clean. When it cannot, the cleaned input is used.
func normalize(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return filepath.Clean(dir)
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		// filepath.Clean turns "/" into "\" on Windows, so entries stored with
		// either separator compare equal; case is ignored as well.
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return a == b
}

// AddRepo returns a copy with dir (made absolute) added. A repository already
// registered is not added again; on Windows case is ignored.
func (c Config) AddRepo(dir string) Config {
	abs := normalize(dir)
	repos := append([]string(nil), c.Repos...)
	for _, r := range repos {
		if samePath(r, abs) {
			c.Repos = repos
			return c
		}
	}
	c.Repos = append(repos, abs)
	return c
}

// RemoveRepo returns a copy without dir (made absolute; case ignored on Windows).
func (c Config) RemoveRepo(dir string) Config {
	abs := normalize(dir)
	repos := make([]string, 0, len(c.Repos))
	for _, r := range c.Repos {
		if !samePath(r, abs) {
			repos = append(repos, r)
		}
	}
	c.Repos = repos
	return c
}
