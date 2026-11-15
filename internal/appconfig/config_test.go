package appconfig

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestAddRepoCaseInsensitiveOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("case-insensitive paths are a Windows behavior")
	}
	dir := t.TempDir()
	c := Config{}.AddRepo(dir)
	c = c.AddRepo(strings.ToUpper(dir))
	c = c.AddRepo(strings.ToLower(dir))
	if len(c.Repos) != 1 {
		t.Errorf("Repos = %v, want 1 entry", c.Repos)
	}
}

func TestAddRepoSlashVariantDeduplicated(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("backslash is a separator only on Windows")
	}
	dir := t.TempDir()
	// A repo stored in config.json with forward slashes, added again with backslashes.
	c := Config{Repos: []string{filepath.ToSlash(dir)}}
	c = c.AddRepo(filepath.FromSlash(dir))
	if len(c.Repos) != 1 {
		t.Errorf("Repos = %v, want 1 entry (slash variants are the same repo)", c.Repos)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nested", "config.json")
	repoA := filepath.Join(tmp, "a")
	repoB := filepath.Join(tmp, "b")
	c := Config{PythonPath: "/usr/bin/python3", LastVersion: "v7.10"}
	c = c.AddRepo(repoA).AddRepo(repoB).AddRepo(repoA)
	if len(c.Repos) != 2 {
		t.Fatalf("AddRepo duplicate: Repos = %v, want 2", c.Repos)
	}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, c) {
		t.Errorf("Load = %+v, want %+v", got, c)
	}
	got = got.RemoveRepo(repoA)
	if want := []string{repoB}; !reflect.DeepEqual(got.Repos, want) {
		t.Errorf("after RemoveRepo Repos = %v, want %v", got.Repos, want)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "absent", "config.json"))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, Config{}) {
		t.Errorf("Load = %+v, want empty Config", got)
	}
}
