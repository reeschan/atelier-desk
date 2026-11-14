package homestore

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const watchWait = 5 * time.Second

// collector receives the onChange calls of a Watcher.
type collector struct{ ch chan []string }

func newCollector() *collector { return &collector{ch: make(chan []string, 64)} }

func (c *collector) onChange(paths []string) {
	c.ch <- append([]string(nil), paths...)
}

func (c *collector) wantCall(t *testing.T) []string {
	t.Helper()
	select {
	case p := <-c.ch:
		return p
	case <-time.After(watchWait):
		t.Fatalf("onChange was not called within %v", watchWait)
		return nil
	}
}

func (c *collector) wantNoCall(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case p := <-c.ch:
		t.Fatalf("onChange was called with %v, want no call", p)
	case <-time.After(d):
	}
}

func startWatcher(t *testing.T, home string, debounce time.Duration, c *collector) *Watcher {
	t.Helper()
	w, err := NewWatcher(home, debounce, c.onChange)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	return w
}

func TestWatchReportsWrittenFile(t *testing.T) {
	e := newEnv(t)
	c := newCollector()
	startWatcher(t, e.home, 100*time.Millisecond, c)

	if err := os.WriteFile(e.path("routing.json"), []byte(newRouting), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := c.wantCall(t); !reflect.DeepEqual(got, []string{"routing.json"}) {
		t.Fatalf("onChange paths = %v, want [routing.json]", got)
	}
	c.wantNoCall(t, 500*time.Millisecond) // exactly one call
}

func TestWatchIgnoresTempFiles(t *testing.T) {
	e := newEnv(t)
	c := newCollector()
	startWatcher(t, e.home, 100*time.Millisecond, c)

	tmp := e.path(".routing.json.abc123.tmp")
	if err := os.WriteFile(tmp, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tmp); err != nil {
		t.Fatal(err)
	}
	c.wantNoCall(t, 700*time.Millisecond)

	// Control: the watcher is alive, and the temp file was not remembered.
	if err := os.WriteFile(e.path("overrides.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := c.wantCall(t); !reflect.DeepEqual(got, []string{"overrides.json"}) {
		t.Fatalf("onChange paths = %v, want [overrides.json]", got)
	}
}

func TestWatchDebouncesAndStopsAfterClose(t *testing.T) {
	e := newEnv(t)
	c := newCollector()
	const debounce = 400 * time.Millisecond
	w := startWatcher(t, e.home, debounce, c)

	// Five changes close together (well inside one window) are one call.
	for i := 0; i < 5; i++ {
		content := []byte{'{', '"', 'n', '"', ':', byte('0' + i), '}', '\n'}
		if err := os.WriteFile(filepath.Join(e.home, "routing.json"), content, 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := c.wantCall(t); !reflect.DeepEqual(got, []string{"routing.json"}) {
		t.Fatalf("onChange paths = %v, want [routing.json]", got)
	}
	c.wantNoCall(t, 2*debounce) // the five changes did not split into more calls

	// After Close, changes are not reported.
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := os.WriteFile(e.path("routing.json"), []byte(newRouting), 0o644); err != nil {
		t.Fatal(err)
	}
	c.wantNoCall(t, 2*debounce)
}
