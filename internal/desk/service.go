package desk

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/reeschan/atelier-desk/internal/appconfig"
	"github.com/reeschan/atelier-desk/internal/atelier"
	"github.com/reeschan/atelier-desk/internal/catalog"
	"github.com/reeschan/atelier-desk/internal/homestore"
	"github.com/reeschan/atelier-desk/internal/job"
	"github.com/reeschan/atelier-desk/internal/paths"
	"github.com/reeschan/atelier-desk/internal/runs"
	"github.com/reeschan/atelier-desk/internal/skills"
)

// Event names sent to the screen.
const (
	EventHomeChanged       = "home:changed"
	EventJobOutput         = "job:output"
	EventConnectionChanged = "connection:changed"
)

// HomeChanged is the payload of home:changed: home-relative, "/"-separated paths.
type HomeChanged struct {
	Paths []string `json:"paths"`
}

// JobOutput is the payload of job:output (v1 defines it; no operation emits it yet).
type JobOutput struct {
	ID     string `json:"id"`
	Stream string `json:"stream"`
	Line   string `json:"line"`
}

// SkillView is one skill with the components that call it.
type SkillView struct {
	Detail skills.SkillDetail `json:"detail"`
	UsedBy []catalog.Usage    `json:"usedBy"`
}

// HomeView is the home's settings and the baseline taken just before them.
type HomeView struct {
	State atelier.State      `json:"state"`
	Base  homestore.Baseline `json:"base"`
}

// SaveSettingRequest is one save from the screen. Input is a JSON object as a
// string (empty means {}). To overwrite an external change, the screen sends
// the Current of the previous SaveResult as Base.
type SaveSettingRequest struct {
	Kind  string             `json:"kind"`
	Name  string             `json:"name"`
	Input string             `json:"input"`
	Base  homestore.Baseline `json:"base"`
}

// RunsView is the runs with the lines that were skipped.
type RunsView struct {
	Runs     []runs.Run         `json:"runs"`
	Warnings []runs.ReadWarning `json:"warnings"`
}

// CasesView is the cases with the lines that were skipped.
type CasesView struct {
	Cases    []runs.Case        `json:"cases"`
	Warnings []runs.ReadWarning `json:"warnings"`
}

// Deps are what Service is built from. Tests pass a fake client and Emit.
type Deps struct {
	CfgPath   string
	UserHome  string
	NewClient func(atelier.Config) atelier.Client
	Emit      func(name string, data any)
	Debounce  time.Duration
}

// ownTTL is how long a path the app wrote itself is left out of home:changed.
const ownTTL = 2 * time.Second

// Service is the set of methods bound to the screen.
type Service struct {
	d Deps
	m *machine

	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	checkCancel context.CancelFunc
	cfg         appconfig.Config
	cfgErr      error           // set when the app config exists but could not be read; nothing is saved then
	loc         paths.Locations // Version empty when no version could be resolved
	client      atelier.Client  // nil unless the connection is Ready
	cat         *catalog.Catalog
	catErr      error
	store       *homestore.Store
	watcher     *homestore.Watcher
	own         map[string]ownMark
}

// ownMark is a path the app wrote: the fingerprint of what it wrote ("missing"
// for a delete) and until when an event with that content is left out.
type ownMark struct {
	sha   string
	until time.Time
}

// NewServiceWith returns a Service built from d.
func NewServiceWith(d Deps) *Service {
	if d.NewClient == nil {
		r := job.NewRunner(0)
		d.NewClient = func(c atelier.Config) atelier.Client { return atelier.NewCLIClient(r, c) }
	}
	if d.Debounce <= 0 {
		d.Debounce = homestore.DefaultDebounce
	}
	return &Service{d: d, m: newMachine(), own: map[string]ownMark{}}
}

// NewService returns the production Service: Atelier through desk_*.py and
// events through the Wails runtime.
func NewService(cfgPath string) *Service {
	s := NewServiceWith(Deps{CfgPath: cfgPath})
	s.d.Emit = func(name string, data any) {
		if ctx := s.context(); ctx != nil {
			wruntime.EventsEmit(ctx, name, data)
		}
	}
	return s
}

func (s *Service) context() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctx
}

func (s *Service) callCtx() context.Context {
	if ctx := s.context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

func (s *Service) emit(name string, data any) {
	if s.d.Emit != nil {
		s.d.Emit(name, data)
	}
}

func (s *Service) emitConnection() { s.emit(EventConnectionChanged, s.m.current()) }

// emitJobLine sends one line of a job as job:output.
func (s *Service) emitJobLine(id string, st job.Stream, line string) {
	name := "stdout"
	if st == job.Stderr {
		name = "stderr"
	}
	s.emit(EventJobOutput, JobOutput{ID: id, Stream: name, Line: line})
}

// Start reads the app config, resolves the version (the last one seen, or
// ACTIVE when that is gone) and starts checking it. The check runs in the
// background and ends with connection:changed. It is a function, not a method,
// so that the screen cannot call it through the binding.
//
// When the app config exists but cannot be read, it is left as it is: for the
// rest of this run nothing is saved into it.
func Start(s *Service, ctx context.Context) {
	cctx, cancel := context.WithCancel(ctx)
	cfg, lerr := appconfig.Load(s.d.CfgPath)
	if lerr != nil {
		cfg = appconfig.Config{}
	}

	opts := paths.Options{UserHome: s.d.UserHome, Version: cfg.LastVersion, Repos: cfg.Repos}
	loc, err := paths.Resolve(opts)
	if err != nil && cfg.LastVersion != "" && errors.Is(err, paths.ErrVersionNotFound) {
		opts.Version = ""
		loc, err = paths.Resolve(opts)
	}
	if err != nil {
		loc = baseLocations(s.d.UserHome, cfg.Repos)
	}

	s.mu.Lock()
	s.ctx, s.cancel = cctx, cancel
	s.cfg = cfg
	s.cfgErr = lerr
	s.loc = loc
	_, cerr := s.connectLocked(errDetail(err))
	s.mu.Unlock()
	if cerr == nil {
		s.emitConnection()
	}
}

// Stop stops a running check and the watcher. Like Start, it is not bound.
func Stop(s *Service, ctx context.Context) {
	s.mu.Lock()
	if s.checkCancel != nil {
		s.checkCancel()
		s.checkCancel = nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	w := s.watcher
	s.watcher = nil
	s.client, s.cat, s.catErr, s.store = nil, nil, nil, nil
	s.mu.Unlock()
	if w != nil {
		w.Close()
	}
}

// saveCfgLocked saves next as the app config. It refuses (internal) while the
// app config of this run could not be read, so a broken file is not replaced.
// s.mu must be held.
func (s *Service) saveCfgLocked(next appconfig.Config) error {
	if s.cfgErr != nil {
		return newError(ErrInternal, "app config unreadable: "+s.cfgErr.Error())
	}
	if err := appconfig.Save(s.d.CfgPath, next); err != nil {
		return toError(err)
	}
	s.cfg = next
	return nil
}

func errDetail(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// baseLocations is Locations without a version, built the way paths.Resolve
// does, for when no version can be resolved (skills and runs still work).
func baseLocations(userHome string, repos []string) paths.Locations {
	if userHome == "" {
		if h, err := os.UserHomeDir(); err == nil {
			userHome = h
		}
	}
	claude := filepath.Join(userHome, ".claude")
	skillsDir := filepath.Join(claude, "skills")
	archive := os.Getenv("PBI_RUNS_ARCHIVE")
	if archive == "" {
		archive = filepath.Join(claude, "pbi-runs")
	}
	return paths.Locations{
		UserHome:             userHome,
		SkillsDir:            skillsDir,
		PluginsCacheDir:      filepath.Join(claude, "plugins", "cache"),
		InstalledPluginsFile: filepath.Join(claude, "plugins", "installed_plugins.json"),
		AtelierSkillDir:      filepath.Join(skillsDir, "atelier"),
		ArchiveRoot:          archive,
		Repos:                append([]string(nil), repos...),
	}
}

// dropLocked forgets the client and everything built from it. The watcher is
// closed in the background: Close waits for onChange, which takes s.mu, and a
// late onChange of an old check is dropped by its generation.
func (s *Service) dropLocked() {
	if w := s.watcher; w != nil {
		s.watcher = nil
		go w.Close()
	}
	s.client, s.cat, s.catErr, s.store = nil, nil, nil, nil
}

// connectLocked starts a check of s.loc. The old client, catalog, store and
// watcher are dropped first, so nothing answers for another version. An
// unresolved version (resolveErr set, or Version empty) ends the check at
// once as unsupported. s.mu must be held.
func (s *Service) connectLocked(resolveErr string) (Connection, error) {
	gen, err := s.m.beginCheck(s.loc.Version)
	if err != nil {
		return s.m.current(), err
	}
	if s.checkCancel != nil {
		s.checkCancel()
		s.checkCancel = nil
	}
	s.dropLocked()

	if s.loc.Version == "" {
		s.m.finishCheck(gen, atelier.ProbeResult{
			Status: atelier.StatusUnsupported, Code: atelier.ErrorCode(ErrUnknownVersion), Diagnostic: resolveErr,
		})
		return s.m.current(), nil
	}

	parent := s.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	s.checkCancel = cancel
	go s.check(ctx, gen, s.loc, s.cfg.PythonPath)
	return s.m.current(), nil
}

// check probes loc, builds the client, catalog, store and watcher for it and
// applies them only when gen is still the latest check.
func (s *Service) check(ctx context.Context, gen int, loc paths.Locations, python string) {
	probe := s.d.NewClient(atelier.Config{Python: python, VersionDir: loc.VersionDir})
	r, err := probe.Probe(ctx)
	if err != nil {
		return
	}
	if !s.m.isCurrent(gen) {
		return
	}

	var (
		client  atelier.Client
		cat     *catalog.Catalog
		catErr  error
		store   *homestore.Store
		watcher *homestore.Watcher
		// retryWatch is set when the home could not be watched yet (it
		// usually does not exist); the watcher is then made later.
		retryWatch bool
	)
	if r.Status == atelier.StatusReady {
		if r.Python != "" {
			python = r.Python
		}
		loc = loc.WithHome(r.Home)
		client = s.d.NewClient(atelier.Config{Python: python, VersionDir: loc.VersionDir, Home: loc.Home})
		cat, catErr = s.buildCatalog(ctx, client, loc)
		var ae *atelier.Error
		if errors.As(catErr, &ae) && ae.Code == atelier.ErrUnsupported {
			// Atelier answers in a shape this app does not know: the
			// version is unsupported, not ready.
			r.Status, r.Code, r.Diagnostic = atelier.StatusUnsupported, atelier.ErrUnsupported, ae.Diagnostic
			client, cat, catErr = nil, nil, nil
		} else {
			store = homestore.NewStore(client, loc.Home, loc.VersionDir, s.backupRoot())
			if loc.Home != "" {
				w, werr := homestore.NewWatcher(loc.Home, s.d.Debounce, func(p []string) { s.onHomeChange(gen, p) })
				if werr == nil {
					watcher = w
				} else {
					retryWatch = true
				}
			}
		}
	}

	s.mu.Lock()
	if !s.m.finishCheck(gen, r) {
		s.mu.Unlock()
		if watcher != nil {
			watcher.Close()
		}
		return
	}
	s.client, s.cat, s.catErr, s.store, s.watcher = client, cat, catErr, store, watcher
	if r.Status == atelier.StatusReady {
		// loc was copied when the check began; only its version and home
		// fields are taken. The repositories are always the current ones.
		s.loc = loc.WithRepos(s.cfg.Repos)
		if s.cfg.LastVersion != loc.Version && s.cfgErr == nil {
			next := s.cfg
			next.LastVersion = loc.Version
			_ = s.saveCfgLocked(next)
		}
	}
	s.mu.Unlock()
	s.emitConnection()
	if retryWatch {
		go s.retryWatch(gen, loc.Home)
	}
}

// watchRetry is how often a home that could not be watched is tried again.
const watchRetry = time.Second

// retryWatch tries to watch home every watchRetry until it works, gen is no
// longer the latest check, or the service is stopped.
func (s *Service) retryWatch(gen int, home string) {
	ctx := s.callCtx()
	t := time.NewTicker(watchRetry)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.tryWatch(gen, home) {
				return
			}
		}
	}
}

// tryWatch makes the watcher of home when check gen has none. It reports false
// only when the watcher could not be made and should be tried again. When it
// is made, the catalog is rebuilt and the files now in home are sent once as
// home:changed (nothing is sent when there are none).
func (s *Service) tryWatch(gen int, home string) bool {
	if !s.watchWanted(gen) {
		return true
	}
	w, err := homestore.NewWatcher(home, s.d.Debounce, func(p []string) { s.onHomeChange(gen, p) })
	if err != nil {
		return false
	}
	s.mu.Lock()
	if !s.watchWantedLocked(gen) {
		s.mu.Unlock()
		w.Close()
		return true
	}
	s.watcher = w
	client, loc := s.client, s.loc
	s.mu.Unlock()
	if client != nil {
		s.rebuildCatalog(gen, client, loc)
	}
	if files := homeFiles(home); len(files) > 0 {
		s.emit(EventHomeChanged, HomeChanged{Paths: files})
	}
	return true
}

func (s *Service) watchWanted(gen int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.watchWantedLocked(gen)
}

// watchWantedLocked reports whether check gen still wants a watcher: it is the
// latest check, has none, and the service is not stopped. s.mu must be held.
func (s *Service) watchWantedLocked(gen int) bool {
	if !s.m.isCurrent(gen) || s.watcher != nil {
		return false
	}
	return s.ctx == nil || s.ctx.Err() == nil
}

// homeFiles lists the files under home, home-relative and "/"-separated, in
// order, leaving out temporary files.
func homeFiles(home string) []string {
	var out []string
	_ = filepath.WalkDir(home, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || homestore.IsTempName(p) {
			return nil
		}
		if rel, rerr := filepath.Rel(home, p); rerr == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func (s *Service) backupRoot() string {
	return filepath.Join(filepath.Dir(s.d.CfgPath), "backups")
}

func (s *Service) buildCatalog(ctx context.Context, c atelier.Client, loc paths.Locations) (*catalog.Catalog, error) {
	sk, _, err := skills.NewScanner(loc).Scan()
	if err != nil {
		sk = nil
	}
	return catalog.Build(ctx, c, sk)
}

// onHomeChange drops the paths the app wrote itself (while the file still has
// the content the app wrote), rebuilds the catalog and sends home:changed.
func (s *Service) onHomeChange(gen int, changed []string) {
	now := time.Now()
	s.mu.Lock()
	if !s.m.isCurrent(gen) {
		s.mu.Unlock()
		return
	}
	expect := map[string]string{}
	for _, p := range changed {
		if m, ok := s.own[p]; ok && now.Before(m.until) {
			expect[p] = m.sha
		}
	}
	for p, m := range s.own {
		if !now.Before(m.until) {
			delete(s.own, p)
		}
	}
	client, loc := s.client, s.loc
	home := s.loc.Home
	s.mu.Unlock()

	out := make([]string, 0, len(changed))
	for _, p := range changed {
		if want, ok := expect[p]; ok {
			if got, err := homestore.FileSHA256(filepath.Join(home, filepath.FromSlash(p))); err == nil && got == want {
				continue
			}
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return
	}
	if client != nil {
		s.rebuildCatalog(gen, client, loc)
	}
	s.emit(EventHomeChanged, HomeChanged{Paths: out})
}

// rebuildCatalog replaces the catalog when client is still the current one.
func (s *Service) rebuildCatalog(gen int, client atelier.Client, loc paths.Locations) {
	cat, err := s.buildCatalog(s.callCtx(), client, loc)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m.isCurrent(gen) && s.client == client {
		s.cat, s.catErr = cat, err
	}
}

// --- connection and version ---

// Connection returns the current connection.
func (s *Service) Connection() Connection { return s.m.current() }

// Versions lists the installed Atelier versions.
func (s *Service) Versions() ([]string, error) {
	s.mu.Lock()
	dir := s.loc.AtelierSkillDir
	s.mu.Unlock()
	vs, err := paths.ListVersions(dir)
	if err != nil {
		return nil, toError(err)
	}
	if vs == nil {
		vs = []string{}
	}
	return vs, nil
}

// SwitchVersion starts checking version v. It is refused while saving, and
// accepted from any other state (an unsupported version can be left).
func (s *Service) SwitchVersion(v string) (Connection, error) {
	s.mu.Lock()
	if s.m.current().Saving {
		s.mu.Unlock()
		return s.m.current(), newError(ErrSaving, "")
	}
	loc, err := s.loc.WithVersion(v)
	if err != nil {
		s.mu.Unlock()
		if errors.Is(err, paths.ErrVersionNotFound) {
			return s.m.current(), newError(ErrUnknownVersion, v)
		}
		return s.m.current(), toError(err)
	}
	prev := s.loc
	s.loc = loc
	c, err := s.connectLocked("")
	if err != nil {
		s.loc = prev
	}
	s.mu.Unlock()
	if err != nil {
		return c, err
	}
	s.emitConnection()
	return c, nil
}

// SetPythonPath stores the interpreter path (empty: search PATH) and checks
// again. It is refused while saving.
func (s *Service) SetPythonPath(p string) (Connection, error) {
	s.mu.Lock()
	if s.m.current().Saving {
		s.mu.Unlock()
		return s.m.current(), newError(ErrSaving, "")
	}
	next := s.cfg
	next.PythonPath = p
	if err := s.saveCfgLocked(next); err != nil {
		s.mu.Unlock()
		return s.m.current(), err
	}
	c, err := s.connectLocked("")
	s.mu.Unlock()
	if err != nil {
		return c, err
	}
	s.emitConnection()
	return c, nil
}

// Recheck checks the current version again. While saving it changes nothing.
func (s *Service) Recheck() Connection {
	s.mu.Lock()
	c, err := s.connectLocked("")
	s.mu.Unlock()
	if err == nil {
		s.emitConnection()
	}
	return c
}

// --- workflows and components ---

func (s *Service) catalog() (*catalog.Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cat == nil {
		if s.catErr != nil {
			return nil, toError(s.catErr)
		}
		return nil, newError(ErrNotReady, string(s.m.current().State))
	}
	return s.cat, nil
}

// Workflows lists the workflows of the current version.
func (s *Service) Workflows() ([]catalog.WorkflowView, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, err
	}
	out := c.Workflows()
	if out == nil {
		out = []catalog.WorkflowView{}
	}
	return out, nil
}

// Workflow returns the workflow named name.
func (s *Service) Workflow(name string) (catalog.WorkflowView, error) {
	c, err := s.catalog()
	if err != nil {
		return catalog.WorkflowView{}, err
	}
	w, ok := c.Workflow(name)
	if !ok {
		return catalog.WorkflowView{}, newError(ErrNotFound, name)
	}
	return w, nil
}

// Components lists the components of the current version.
func (s *Service) Components() ([]catalog.ComponentView, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, err
	}
	out := c.Components()
	if out == nil {
		out = []catalog.ComponentView{}
	}
	return out, nil
}

// Component returns the component id.
func (s *Service) Component(id string) (catalog.ComponentView, error) {
	c, err := s.catalog()
	if err != nil {
		return catalog.ComponentView{}, err
	}
	v, ok := c.Component(id)
	if !ok {
		return catalog.ComponentView{}, newError(ErrNotFound, id)
	}
	return v, nil
}

// --- skills (no call to Atelier) ---

func (s *Service) scanner() *skills.Scanner {
	s.mu.Lock()
	defer s.mu.Unlock()
	return skills.NewScanner(s.loc)
}

// Skills lists the user and plugin skills.
func (s *Service) Skills() ([]skills.Skill, error) {
	sk, _, err := s.scanner().Scan()
	if err != nil {
		return nil, toError(err)
	}
	if sk == nil {
		sk = []skills.Skill{}
	}
	return sk, nil
}

// Skill returns one skill and the components that call it (empty when the
// catalog is not available).
func (s *Service) Skill(key string) (SkillView, error) {
	d, err := s.scanner().Detail(key)
	if err != nil {
		return SkillView{}, toError(err)
	}
	used := []catalog.Usage{}
	s.mu.Lock()
	if s.cat != nil {
		if u := s.cat.UsedBy(key); u != nil {
			used = u
		}
	}
	s.mu.Unlock()
	return SkillView{Detail: d, UsedBy: used}, nil
}

// SkillFile returns the text of one file of a skill.
func (s *Service) SkillFile(key, rel string) (string, error) {
	t, err := s.scanner().ReadFile(key, rel)
	if err != nil {
		return "", toError(err)
	}
	return t, nil
}

// --- settings ---

func (s *Service) ready() (atelier.Client, *homestore.Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m.current().State != Ready || s.client == nil || s.store == nil {
		return nil, nil, newError(ErrNotReady, string(s.m.current().State))
	}
	return s.client, s.store, nil
}

// HomeState returns the home's settings with the baseline taken just before
// them (a change after State was read is not in Base).
func (s *Service) HomeState() (HomeView, error) {
	client, store, err := s.ready()
	if err != nil {
		return HomeView{}, err
	}
	base, err := store.Snapshot()
	if err != nil {
		return HomeView{}, toError(err)
	}
	st, err := client.State(s.callCtx())
	if err != nil {
		return HomeView{}, toError(err)
	}
	return HomeView{State: st, Base: base}, nil
}

// SaveSetting saves one setting into the home. It is refused unless the
// connection is Ready; while it runs, version switches and Python changes are
// refused.
func (s *Service) SaveSetting(req SaveSettingRequest) (homestore.SaveResult, error) {
	s.mu.Lock()
	if err := s.m.beginSave(); err != nil {
		s.mu.Unlock()
		return homestore.SaveResult{}, err
	}
	gen := s.m.generation()
	client, store, loc := s.client, s.store, s.loc
	s.mu.Unlock()
	s.emitConnection()
	defer func() {
		s.m.endSave()
		s.emitConnection()
	}()

	if client == nil || store == nil {
		return homestore.SaveResult{}, newError(ErrNotReady, "")
	}
	input := req.Input
	if input == "" {
		input = "{}"
	}
	if !json.Valid([]byte(input)) {
		return homestore.SaveResult{}, newError(ErrBadInput, "input")
	}
	res, err := store.Save(s.callCtx(), homestore.SaveRequest{
		Kind:  req.Kind,
		Name:  req.Name,
		Input: json.RawMessage(input),
		Base:  req.Base,
	})
	s.markOwn(res)
	if len(res.Written) > 0 && loc.Home != "" {
		s.tryWatch(gen, loc.Home)
	}
	if err != nil {
		return res, toError(err)
	}
	if len(res.Written) > 0 {
		s.rebuildCatalog(gen, client, loc)
	}
	return res, nil
}

// markOwn leaves the paths the app just wrote out of home:changed for a while,
// as long as they still hold what the app wrote (the post-save Base). Only for
// a Saved save: a PartialFailure has no fingerprint of the bytes the app wrote,
// so nothing is marked and the change goes through as an outside one. A path
// with no fingerprint was deleted.
func (s *Service) markOwn(res homestore.SaveResult) {
	if res.Outcome != homestore.Saved || len(res.Written) == 0 {
		return
	}
	files := res.Base.Files
	until := time.Now().Add(ownTTL + 2*s.d.Debounce)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range res.Written {
		p := path.Clean(filepath.ToSlash(w))
		sha, ok := files[p]
		if !ok {
			sha = "missing"
		}
		s.own[p] = ownMark{sha: sha, until: until}
	}
}

// --- runs (no call to Atelier) ---

// Repos lists the registered repositories.
func (s *Service) Repos() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.cfg.Repos...)
}

func (s *Service) setRepos(next appconfig.Config) ([]string, error) {
	if err := s.saveCfgLocked(next); err != nil {
		return nil, err
	}
	s.loc = s.loc.WithRepos(next.Repos)
	return append([]string{}, next.Repos...), nil
}

// AddRepo registers a repository and saves the app config.
func (s *Service) AddRepo(dir string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setRepos(s.cfg.AddRepo(dir))
}

// RemoveRepo unregisters a repository and saves the app config.
func (s *Service) RemoveRepo(dir string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setRepos(s.cfg.RemoveRepo(dir))
}

func (s *Service) reader() *runs.Reader {
	s.mu.Lock()
	defer s.mu.Unlock()
	return runs.NewReader(s.loc.WithRepos(s.cfg.Repos))
}

// Runs reads the run records with the lines that were skipped.
func (s *Service) Runs() (RunsView, error) {
	rs, ws, err := s.reader().Runs()
	if err != nil {
		return RunsView{}, toError(err)
	}
	if rs == nil {
		rs = []runs.Run{}
	}
	if ws == nil {
		ws = []runs.ReadWarning{}
	}
	return RunsView{Runs: rs, Warnings: ws}, nil
}

// Cases reads the cases with the lines that were skipped.
func (s *Service) Cases() (CasesView, error) {
	cs, ws, err := s.reader().Cases()
	if err != nil {
		return CasesView{}, toError(err)
	}
	if cs == nil {
		cs = []runs.Case{}
	}
	if ws == nil {
		ws = []runs.ReadWarning{}
	}
	return CasesView{Cases: cs, Warnings: ws}, nil
}
