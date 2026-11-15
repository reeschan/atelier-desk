package paths

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// copyHome copies testdata/home into a temp dir and returns it.
func copyHome(t *testing.T) string {
	t.Helper()
	src := filepath.Join("testdata", "home")
	dst := t.TempDir()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestResolveSelectsActiveVersion(t *testing.T) {
	home := copyHome(t)
	l, err := Resolve(Options{UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	if l.Version != "v7.10" {
		t.Errorf("Version = %q, want v7.10", l.Version)
	}
	wantDir := filepath.Join(home, ".claude", "skills", "atelier", "versions", "v7.10")
	if l.VersionDir != wantDir {
		t.Errorf("VersionDir = %q, want %q", l.VersionDir, wantDir)
	}
	vs, err := ListVersions(l.AtelierSkillDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"v7.9", "v7.10"}; !reflect.DeepEqual(vs, want) {
		t.Errorf("ListVersions = %v, want %v", vs, want)
	}
}

func TestWithVersionUnknownIsError(t *testing.T) {
	home := copyHome(t)
	l, err := Resolve(Options{UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.WithVersion("v9.9"); !errors.Is(err, ErrVersionNotFound) {
		t.Errorf("WithVersion(v9.9) err = %v, want ErrVersionNotFound", err)
	}
	got, err := l.WithVersion("v7.9")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "v7.9" {
		t.Errorf("Version = %q, want v7.9", got.Version)
	}
}

func TestResolveErrors(t *testing.T) {
	home := copyHome(t)
	if _, err := Resolve(Options{UserHome: home, Version: "v1.0"}); !errors.Is(err, ErrVersionNotFound) {
		t.Errorf("missing version dir: err = %v, want ErrVersionNotFound", err)
	}
	active := filepath.Join(home, ".claude", "skills", "atelier", "ACTIVE")
	if err := os.WriteFile(active, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(Options{UserHome: home}); !errors.Is(err, ErrNoActive) {
		t.Errorf("empty ACTIVE: err = %v, want ErrNoActive", err)
	}
}
