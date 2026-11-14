package homestore

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDebounce is the window used when NewWatcher gets a non-positive debounce.
const DefaultDebounce = 300 * time.Millisecond

// IsTempName reports whether name is a temporary file name written by Studio
// (".<name>.<random>.tmp"): it starts with "." and ends with ".tmp".
// Only the base name is looked at.
func IsTempName(name string) bool {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	return len(name) > len(".tmp") && strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".tmp")
}

// Watcher watches a home directory (and its subdirectories) and reports the
// changed paths, thinned out into one call per debounce window.
type Watcher struct {
	home     string
	debounce time.Duration
	onChange func(paths []string)
	fsw      *fsnotify.Watcher

	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// NewWatcher starts watching home and every directory under it. Directories
// created later are registered too. onChange receives the home-relative,
// "/"-separated paths that changed within the window, without duplicates, in
// ascending order. Temporary files (IsTempName) are ignored. It returns an
// error when home does not exist or is not a directory.
func NewWatcher(home string, debounce time.Duration, onChange func(paths []string)) (*Watcher, error) {
	info, err := os.Stat(home)
	if err != nil {
		return nil, fmt.Errorf("homestore: watch home: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("homestore: watch home: %s is not a directory", home)
	}
	if debounce <= 0 {
		debounce = DefaultDebounce
	}
	if onChange == nil {
		onChange = func([]string) {}
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("homestore: watch home: %w", err)
	}
	w := &Watcher{
		home:     filepath.Clean(home),
		debounce: debounce,
		onChange: onChange,
		fsw:      fsw,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	if err := w.addTree(w.home, nil); err != nil {
		_ = fsw.Close()
		return nil, fmt.Errorf("homestore: watch home: %w", err)
	}
	go w.loop()
	return w, nil
}

// Close stops watching. After Close returns, onChange is not called again.
// Do not call Close from inside onChange.
func (w *Watcher) Close() error {
	w.closeOnce.Do(func() {
		close(w.stop)
		w.closeErr = w.fsw.Close()
		<-w.done
	})
	return w.closeErr
}

// addTree registers dir and all directories below it. Files found in the
// tree are passed to found (when not nil), so that files created before the
// directory was registered are not lost.
func (w *Watcher) addTree(dir string, found func(path string)) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == dir {
				return err
			}
			return nil // vanished while walking
		}
		if d.IsDir() {
			if err := w.fsw.Add(p); err != nil && p == dir {
				return err
			}
			return nil
		}
		if found != nil && !IsTempName(d.Name()) {
			found(p)
		}
		return nil
	})
}

func (w *Watcher) rel(p string) (string, bool) {
	r, err := filepath.Rel(w.home, p)
	if err != nil || r == "." || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(r), true
}

func (w *Watcher) loop() {
	defer close(w.done)
	pending := map[string]struct{}{}
	add := func(p string) {
		if r, ok := w.rel(p); ok {
			pending[r] = struct{}{}
		}
	}
	timer := time.NewTimer(w.debounce)
	if !timer.Stop() {
		<-timer.C
	}
	armed := false
	defer timer.Stop()

	flush := func() {
		armed = false
		if len(pending) == 0 {
			return
		}
		paths := make([]string, 0, len(pending))
		for p := range pending {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		pending = map[string]struct{}{}
		select {
		case <-w.stop:
			return
		default:
		}
		w.onChange(paths)
	}

	for {
		select {
		case <-w.stop:
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if IsTempName(ev.Name) {
				continue
			}
			if ev.Has(fsnotify.Create) {
				if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
					// Register the new directory; report files already in it.
					_ = w.addTree(ev.Name, add)
				} else {
					add(ev.Name)
				}
			} else {
				add(ev.Name)
			}
			if len(pending) == 0 {
				continue
			}
			if armed && !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(w.debounce)
			armed = true
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		case <-timer.C:
			flush()
		}
	}
}
