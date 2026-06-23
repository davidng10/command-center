package tui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"command-center/internal/config"
	"command-center/internal/fsbrowse"
	"command-center/internal/provider"
	"command-center/internal/session"
	"command-center/internal/term"
	"command-center/internal/worktree"
)

type wizStep int

const (
	wsName wizStep = iota
	wsDir
	wsBase
	wsSetup
	wsConfirm
)

// customSentinel marks the "type my own command" choice in the setup picker. The
// NUL byte can never collide with a real command.
const customSentinel = "\x00custom"

type dirItem struct {
	Label  string
	Path   string
	IsUp   bool
	IsRepo bool
	Pinned bool
	Tag    string
}

type baseItem struct {
	Name   string // display name, e.g. "main" or "origin/main"
	Base   string // start-point base, origin/ stripped ("main")
	Remote bool
	Tag    string
}

type setupItem struct {
	Label string
	Value string
}

type wizardModel struct {
	prov   provider.Provider
	global config.Global

	step wizStep

	nameInput textinput.Model

	// directory browser
	startDir  string
	stack     []string
	dirCursor int
	dirFilter string

	// chosen repo
	repo     worktree.RepoContext
	cfg      config.Config
	dirLabel string

	// base branch
	baseItems  []baseItem
	baseCursor int
	baseFilter string

	// setup
	setupItems  []setupItem
	setupCursor int
	setupFilter string
	customInput textinput.Model
	customizing bool

	// resolved answers
	branch   string
	base     string
	setupCmd string
}

func newWizard(prefill string, prov provider.Provider, global config.Global) *wizardModel {
	ni := textinput.New()
	ni.Prompt = stAccent.Render("› ")
	ni.Placeholder = "task/SP-1234-login-fix"
	ni.PlaceholderStyle = stDimmer
	ni.TextStyle = stInk
	ni.Cursor.Style = stCaret

	ci := textinput.New()
	ci.Prompt = stAccent.Render("› ")
	ci.Placeholder = "runs in the new worktree — blank to skip"
	ci.PlaceholderStyle = stDimmer
	ci.TextStyle = stInk
	ci.Cursor.Style = stCaret

	start := homeDir()
	w := &wizardModel{
		prov: prov, global: global,
		nameInput: ni, customInput: ci,
		startDir: start, stack: []string{start},
	}
	if strings.TrimSpace(prefill) != "" {
		w.branch = prefill
		w.nameInput.SetValue(prefill)
		w.step = wsDir
	} else {
		w.nameInput.Focus()
	}
	return w
}

// ---- update -----------------------------------------------------------------

func (a App) updateWizard(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	w := a.wiz
	if w == nil {
		a.scr = scrHome
		return a, nil
	}

	// While a create is running, ignore input so Enter can't double-submit.
	if a.busyLabel != "" {
		return a, nil
	}

	// Esc backs out a step (or cancels the in-progress custom-command field).
	if m.Type == tea.KeyEsc {
		return a.wizBack(), nil
	}

	switch w.step {
	case wsName:
		return a.updateWizName(m)
	case wsDir:
		return a.updateWizDir(m)
	case wsBase:
		return a.updateWizBase(m)
	case wsSetup:
		return a.updateWizSetup(m)
	case wsConfirm:
		if m.Type == tea.KeyEnter {
			return a.startCreate()
		}
	}
	return a, nil
}

func (a App) wizBack() App {
	w := a.wiz
	if w.customizing {
		w.customizing = false
		return a
	}
	switch w.step {
	case wsName:
		a.scr = scrHome
		a.wiz = nil
	case wsDir:
		w.step = wsName
		w.nameInput.Focus()
	case wsBase:
		w.step = wsDir
	case wsSetup:
		w.step = wsBase
	case wsConfirm:
		if w.cfg.Install {
			w.step = wsSetup
		} else {
			w.step = wsBase
		}
	}
	return a
}

func (a App) updateWizName(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	w := a.wiz
	if m.Type == tea.KeyEnter {
		if strings.TrimSpace(w.nameInput.Value()) == "" {
			return a, nil
		}
		w.branch = w.nameInput.Value()
		w.nameInput.Blur()
		w.step = wsDir
		w.dirCursor = 0
		return a, nil
	}
	var cmd tea.Cmd
	w.nameInput, cmd = w.nameInput.Update(m)
	return a, cmd
}

func (a App) updateWizDir(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	w := a.wiz
	items := w.dirDisplay()
	switch {
	case m.String() == "up":
		if w.dirCursor > 0 {
			w.dirCursor--
		}
	case m.String() == "down":
		if w.dirCursor < len(items)-1 {
			w.dirCursor++
		}
	case m.Type == tea.KeyLeft:
		w.popDir()
	case m.Type == tea.KeyEnter:
		return a.wizActivateDir(items)
	case m.Type == tea.KeyBackspace:
		if w.dirFilter != "" {
			w.dirFilter = trimLastRune(w.dirFilter)
			w.dirCursor = 0
		}
	case m.Type == tea.KeyRunes:
		w.dirFilter += string(m.Runes)
		w.dirCursor = 0
	case m.Type == tea.KeySpace:
		w.dirFilter += " "
		w.dirCursor = 0
	}
	return a, nil
}

func (a App) wizActivateDir(items []dirItem) (tea.Model, tea.Cmd) {
	w := a.wiz
	if w.dirCursor < 0 || w.dirCursor >= len(items) {
		return a, nil
	}
	it := items[w.dirCursor]
	switch {
	case it.IsUp:
		w.popDir()
	case it.IsRepo:
		a.wizSelectRepo(it.Path)
	default:
		w.stack = append(w.stack, it.Path)
		w.dirCursor = 0
		w.dirFilter = ""
	}
	return a, nil
}

func (w *wizardModel) popDir() {
	if len(w.stack) > 1 {
		w.stack = w.stack[:len(w.stack)-1]
		w.dirCursor = 0
		w.dirFilter = ""
	}
}

func (a App) wizSelectRepo(path string) {
	w := a.wiz
	w.repo = worktree.ContextFromRoot(path)
	w.cfg = config.Load(path)
	w.dirLabel = homeTilde(path)
	w.baseItems = buildBaseItems(w.repo, w.cfg)
	w.baseCursor = 0
	w.baseFilter = ""
	w.step = wsBase
}

func (a App) updateWizBase(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	w := a.wiz
	items := w.baseDisplay()
	switch {
	case m.String() == "up":
		if w.baseCursor > 0 {
			w.baseCursor--
		}
	case m.String() == "down":
		if w.baseCursor < len(items)-1 {
			w.baseCursor++
		}
	case m.Type == tea.KeyEnter:
		if w.baseCursor < len(items) {
			w.base = items[w.baseCursor].Base
			a.enterSetupOrConfirm()
		}
	case m.Type == tea.KeyBackspace:
		if w.baseFilter != "" {
			w.baseFilter = trimLastRune(w.baseFilter)
			w.baseCursor = 0
		}
	case m.Type == tea.KeyRunes:
		w.baseFilter += string(m.Runes)
		w.baseCursor = 0
	case m.Type == tea.KeySpace:
		w.baseFilter += " "
		w.baseCursor = 0
	}
	return a, nil
}

// enterSetupOrConfirm advances from base: into the setup picker when the repo
// offers setup, else straight to review (§4.4 / FR-12).
func (a App) enterSetupOrConfirm() {
	w := a.wiz
	if !w.cfg.Install {
		w.setupCmd = ""
		w.step = wsConfirm
		return
	}
	w.setupItems = buildSetupItems(w.cfg, w.repo)
	w.setupCursor = 0
	w.setupFilter = ""
	w.step = wsSetup
}

func (a App) updateWizSetup(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	w := a.wiz
	if w.customizing {
		if m.Type == tea.KeyEnter {
			w.setupCmd = strings.TrimSpace(w.customInput.Value())
			w.customizing = false
			w.step = wsConfirm
			return a, nil
		}
		var cmd tea.Cmd
		w.customInput, cmd = w.customInput.Update(m)
		return a, cmd
	}

	items := w.setupDisplay()
	switch {
	case m.String() == "up":
		if w.setupCursor > 0 {
			w.setupCursor--
		}
	case m.String() == "down":
		if w.setupCursor < len(items)-1 {
			w.setupCursor++
		}
	case m.Type == tea.KeyEnter:
		if w.setupCursor < len(items) {
			it := items[w.setupCursor]
			if it.Value == customSentinel {
				w.customizing = true
				w.customInput.SetValue("")
				w.customInput.Focus()
				return a, nil
			}
			w.setupCmd = it.Value
			w.step = wsConfirm
		}
	case m.Type == tea.KeyBackspace:
		if w.setupFilter != "" {
			w.setupFilter = trimLastRune(w.setupFilter)
			w.setupCursor = 0
		}
	case m.Type == tea.KeyRunes:
		w.setupFilter += string(m.Runes)
		w.setupCursor = 0
	case m.Type == tea.KeySpace:
		w.setupFilter += " "
		w.setupCursor = 0
	}
	return a, nil
}

// ---- display lists ----------------------------------------------------------

func (w *wizardModel) dirDisplay() []dirItem {
	cur := w.stack[len(w.stack)-1]
	f := strings.ToLower(w.dirFilter)
	var items []dirItem
	if f == "" {
		if len(w.stack) == 1 {
			if p := firstRecentRepo(); p != "" {
				items = append(items, dirItem{
					Label: filepath.Base(p), Path: p, IsRepo: true, Pinned: true,
					Tag: homeTilde(filepath.Dir(p)) + " · last used",
				})
			}
		} else {
			items = append(items, dirItem{Label: "..", IsUp: true})
		}
	}
	entries, _ := fsbrowse.List(cur)
	for _, e := range entries {
		if f != "" && !strings.Contains(strings.ToLower(e.Name), f) {
			continue
		}
		items = append(items, dirItem{Label: e.Name, Path: e.Path, IsRepo: e.IsRepo})
	}
	return items
}

func (w *wizardModel) baseDisplay() []baseItem {
	f := strings.ToLower(w.baseFilter)
	if f == "" {
		return w.baseItems
	}
	var out []baseItem
	for _, b := range w.baseItems {
		if strings.Contains(strings.ToLower(b.Name), f) {
			out = append(out, b)
		}
	}
	return out
}

func (w *wizardModel) setupDisplay() []setupItem {
	f := strings.ToLower(w.setupFilter)
	if f == "" {
		return w.setupItems
	}
	var out []setupItem
	for _, s := range w.setupItems {
		if strings.Contains(strings.ToLower(s.Label), f) {
			out = append(out, s)
		}
	}
	return out
}

func buildBaseItems(repo worktree.RepoContext, cfg config.Config) []baseItem {
	branches, _ := fsbrowse.Branches(repo.Root)
	remembered := cfg.DefaultBase
	if b, ok := config.CachedBase(repo.Root); ok {
		remembered = b
	}
	items := make([]baseItem, 0, len(branches))
	for _, b := range branches {
		base := strings.TrimPrefix(b.Name, "origin/")
		items = append(items, baseItem{Name: b.Name, Base: base, Remote: b.Remote})
	}
	// Pin the remembered/default base to the front, tagged "last used".
	for i := range items {
		if items[i].Base == remembered && !items[i].Remote {
			items[i].Tag = "last used"
			pinned := items[i]
			items = append(items[:i], items[i+1:]...)
			items = append([]baseItem{pinned}, items...)
			break
		}
	}
	if len(items) == 0 {
		// Fall back to the configured default so the user is never stuck.
		items = []baseItem{{Name: remembered, Base: remembered, Tag: "default"}}
	}
	return items
}

func buildSetupItems(cfg config.Config, repo worktree.RepoContext) []setupItem {
	def, src := worktree.ResolveSetupDefault(cfg, repo.Root)
	var items []setupItem
	seen := map[string]bool{}
	add := func(label, val string) {
		if seen[val] {
			return
		}
		seen[val] = true
		items = append(items, setupItem{Label: label, Value: val})
	}
	if def != "" {
		add(fmt.Sprintf("%s  (%s)", def, src), def)
	}
	for _, c := range worktree.CommonSetups {
		add(c, c)
	}
	add("Custom…", customSentinel)
	add("Skip (no setup)", "")
	return items
}

// ---- create flow ------------------------------------------------------------

func (a App) startCreate() (tea.Model, tea.Cmd) {
	w := a.wiz
	a.busyLabel = "Creating worktree…"
	repo, cfg := w.repo, w.cfg
	branchRaw, base, setupCmd := w.branch, w.base, w.setupCmd
	prov, global := a.prov, a.global
	return a, func() tea.Msg {
		return doCreate(repo, cfg, branchRaw, base, setupCmd, prov, global)
	}
}

// doCreate runs the worktree creation + launch off the UI goroutine and returns
// a createResultMsg. It never mutates the registry (the Update handler does that
// on the UI goroutine) to keep registry access single-threaded.
func doCreate(repo worktree.RepoContext, cfg config.Config, branchRaw, base, setupCmd string, prov provider.Provider, global config.Global) createResultMsg {
	p := worktree.BuildPlan(repo, cfg, branchRaw)
	if _, err := os.Stat(p.WorktreePath); err == nil {
		return createResultMsg{err: fmt.Errorf("folder already exists: %s", p.WorktreePath)}
	}

	var warns []string
	startPoint, fetchErr := worktree.ResolveStartPoint(repo, cfg, base)
	if fetchErr != nil {
		warns = append(warns, fmt.Sprintf("fetch failed (%s may be stale)", startPoint))
	}
	if err := worktree.AddWorktree(repo, p, startPoint); err != nil {
		return createResultMsg{err: err}
	}
	worktree.CopyConfiguredFiles(repo, cfg, p)

	if cfg.Install && strings.TrimSpace(setupCmd) != "" {
		name, args := worktree.SplitLaunch(setupCmd)
		if err := worktree.SpawnQuiet(p.WorktreePath, name, args...); err != nil {
			warns = append(warns, fmt.Sprintf("setup %q failed — run it manually", setupCmd))
		}
	}

	// Remember choices for next time (skip setup memory when .ccrc.json pins it).
	if cfg.Setup == "" {
		_ = config.RememberSetup(repo.Root, setupCmd)
	}
	_ = config.RememberBase(repo.Root, base)
	_ = config.PushRecentDir(repo.Root)
	if prov != nil {
		_ = config.RememberProvider(prov.Name())
	}

	now := time.Now()
	sess := session.Session{
		ID:           newID(),
		Provider:     providerName(prov),
		Branch:       p.Branch,
		RepoDir:      repo.Root,
		Base:         base,
		WorktreePath: p.WorktreePath,
		Setup:        setupCmd,
		State:        session.StateRunning,
		LastActivity: now,
		CreatedAt:    now,
	}

	if prov != nil {
		launch := prov.LaunchSpec(sess)
		if launch.Program != "" {
			pid, err := term.Spawn(launch.Dir, launch.Program, launch.Args, global.Terminal)
			if err != nil {
				warns = append(warns, "agent terminal not launched: "+err.Error())
			} else {
				sess.PID = pid
			}
		}
	}

	return createResultMsg{sess: sess, warn: strings.Join(warns, " · ")}
}

func providerName(p provider.Provider) string {
	if p == nil {
		return ""
	}
	return p.Name()
}

// newID returns a short random hex id (e.g. "f3a1").
func newID() string {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:4]
	}
	return hex.EncodeToString(b[:])
}

// ---- helpers ----------------------------------------------------------------

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return mustGetwd()
}

// firstRecentRepo returns the most-recent directory that is still a git repo, for
// pinning at the top of the directory browser.
func firstRecentRepo() string {
	for _, d := range config.LoadPrefs().RecentDirs {
		if fsbrowse.IsRepo(d) {
			return d
		}
	}
	return ""
}

func trimLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}
