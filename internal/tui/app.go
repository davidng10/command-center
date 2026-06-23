// Package tui is fleet's persistent bubbletea shell: a home dashboard of agent
// sessions with live state, a /new wizard, and first-run onboarding. It is the
// only package that imports bubbletea; everything below it (session, provider,
// worktree, config, fsbrowse, term) is UI-free.
package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"command-center/internal/config"
	"command-center/internal/provider"
	"command-center/internal/session"
	"command-center/internal/worktree"
)

type screen int

const (
	scrOnboard screen = iota
	scrHome
	scrWizard
)

// Options configures how the TUI launches.
type Options struct {
	StartWizard   bool   // jump straight into /new (the `fleet --new` shim)
	PrefillBranch string // optional branch name to prefill the wizard's name step
	ForceSetup    bool   // open onboarding regardless of setupComplete (`fleet setup`)
}

// App is the root bubbletea model. Sub-screen state lives inline; the registry
// and tracker channel are the only shared external handles.
type App struct {
	width, height int
	scr           screen

	reg    *session.Registry
	prov   provider.Provider
	global config.Global
	acts   <-chan provider.StateUpdate

	now     time.Time
	pulseOn bool
	flash   string // transient feedback line (errors / confirmations)

	// home
	homeCursor      int
	cmdMode         bool
	cmdInput        textinput.Model
	confirmRemoveID string // non-empty ⇒ awaiting y/n to remove this session

	// wizard / onboarding (nil unless on that screen)
	wiz *wizardModel
	onb *onboardModel

	busy bool // a create flow is running
}

// Messages.
type tickMsg time.Time
type activityMsg provider.StateUpdate
type createResultMsg struct {
	sess session.Session
	warn string
	err  error
}

// Run loads state, wires the default provider's tracker, and runs the program.
func Run(opts Options) error {
	reg, err := session.Load()
	if err != nil {
		return err
	}
	// Reconcile dead sessions before first paint (§8).
	_, _ = reg.ReconcileLiveness()

	global := config.LoadGlobal()
	prov := resolveDefaultProvider(global)

	// Write fleet's scoped status hooks fresh (so the hook command always points at
	// the current binary) and sweep any hooks an older fleet left in the user's
	// global ~/.claude/settings.json — those fired, and errored, in every unrelated
	// Claude Code session. Idempotent and non-invasive, so it runs every launch.
	if prov != nil {
		_ = prov.Install()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var acts <-chan provider.StateUpdate
	if prov != nil {
		acts = prov.Tracker().Updates(ctx)
	} else {
		acts = provider.NoopTracker{}.Updates(ctx)
	}

	a := NewApp(reg, prov, global, acts, opts)
	_, err = tea.NewProgram(a, tea.WithAltScreen()).Run()
	return err
}

// resolveDefaultProvider picks the configured default, falling back to the first
// registered provider.
func resolveDefaultProvider(global config.Global) provider.Provider {
	if p, ok := provider.Get(global.DefaultProvider); ok {
		return p
	}
	if all := provider.All(); len(all) > 0 {
		return all[0]
	}
	return nil
}

// NewApp builds the initial model and chooses the opening screen.
func NewApp(reg *session.Registry, prov provider.Provider, global config.Global, acts <-chan provider.StateUpdate, opts Options) App {
	ci := textinput.New()
	ci.Prompt = ""
	// No textinput placeholder: we render the idle hint (and caret) ourselves in
	// renderCmdBar. The textinput's own placeholder path renders the first char
	// through the cursor, which drops/garbles it (the stray "T" artifact).
	ci.Placeholder = ""
	ci.TextStyle = stInk
	ci.Cursor.Style = stCaret

	a := App{
		reg: reg, prov: prov, global: global, acts: acts,
		now: time.Now(), pulseOn: true, cmdInput: ci,
	}
	switch {
	case opts.ForceSetup:
		a.scr = scrOnboard
		a.onb = newOnboard(prov)
	case opts.StartWizard:
		a.scr = scrWizard
		a.wiz = newWizard(opts.PrefillBranch, prov, global)
	case !global.SetupComplete:
		a.scr = scrOnboard
		a.onb = newOnboard(prov)
	default:
		a.scr = scrHome
	}
	return a
}

func (a App) Init() tea.Cmd {
	return tea.Batch(tickCmd(), waitActivity(a.acts))
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// waitActivity blocks on the tracker channel and turns one update into a msg,
// re-issued after handling so the stream stays subscribed.
func waitActivity(ch <-chan provider.StateUpdate) tea.Cmd {
	return func() tea.Msg {
		upd, ok := <-ch
		if !ok {
			return nil
		}
		return activityMsg(upd)
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		return a, nil

	case tickMsg:
		a.now = time.Now()
		a.pulseOn = !a.pulseOn
		_, _ = a.reg.ReconcileLiveness()
		return a, tickCmd()

	case activityMsg:
		// Map the firing's cwd back to a session and record the new state.
		_, _, _ = a.reg.ApplyActivity(m.Cwd, m.AgentSession, m.State, m.At)
		return a, waitActivity(a.acts)

	case createResultMsg:
		a.busy = false
		if m.err != nil {
			a.flash = stErr("create failed: " + m.err.Error())
			a.scr = scrHome
			a.wiz = nil
			return a, nil
		}
		_ = a.reg.Add(m.sess)
		a.homeCursor = 0
		a.flash = stOK("created " + m.sess.Branch)
		if m.warn != "" {
			a.flash = stWarn(m.warn)
		}
		a.scr = scrHome
		a.wiz = nil
		return a, nil

	case tea.KeyMsg:
		if m.Type == tea.KeyCtrlC {
			return a, tea.Quit
		}
		switch a.scr {
		case scrHome:
			return a.updateHome(m)
		case scrWizard:
			return a.updateWizard(m)
		case scrOnboard:
			return a.updateOnboard(m)
		}
	}

	// Forward non-key messages to the active text input where relevant.
	return a, nil
}

func (a App) View() string {
	if a.width == 0 {
		return "" // wait for the first WindowSizeMsg
	}
	switch a.scr {
	case scrHome:
		return a.viewHome()
	case scrWizard:
		return a.viewWizard()
	case scrOnboard:
		return a.viewOnboard()
	}
	return ""
}

// ---- frame composition -----------------------------------------------------

// frame assembles the persistent chrome. From top to bottom:
//
//	header → body → flash → cmdbar → key hints → [spacer] → context bar (if any)
//
// The command bar and key hints sit directly below the content so they stay close
// on tall screens. The context bar (e.g. remove confirmation) is pinned to the
// bottom via a spacer; it is omitted entirely when empty.
func (a App) frame(header, body, ctx string, keys [][2]string, showCmdBar bool) string {
	inner := a.innerWidth()

	upper := []string{header, body}
	if a.flash != "" {
		upper = append(upper, " "+a.flash)
	}
	if showCmdBar {
		upper = append(upper, a.renderCmdBar())
	}
	if len(keys) > 0 {
		upper = append(upper, " "+keysString(keys))
	}
	top := lipgloss.JoinVertical(lipgloss.Left, upper...)

	if ctx == "" {
		// No context bar — just pad to fill the screen.
		topLines := strings.Count(top, "\n") + 1
		spacerHeight := a.height - topLines
		if spacerHeight < 0 {
			spacerHeight = 0
		}
		return top + strings.Repeat("\n", spacerHeight)
	}

	// Context bar pinned to the bottom.
	ctxBar := a.renderContextBar(inner, ctx)
	topLines := strings.Count(top, "\n") + 1
	ctxLines := strings.Count(ctxBar, "\n") + 1
	spacerHeight := a.height - topLines - ctxLines
	if spacerHeight < 0 {
		spacerHeight = 0
	}
	return top + strings.Repeat("\n", spacerHeight) + ctxBar
}

func (a App) innerWidth() int {
	w := a.width - 2
	if w > 110 {
		w = 110
	}
	if w < 20 {
		w = 20
	}
	return w
}

// brand is the "▌ fleet" wordmark shared by every header.
func brand() string { return stAccentB.Render("▌ fleet") }

// brandHeader is the default header (wizard / onboarding): wordmark + cwd.
func (a App) brandHeader(width int) string {
	return brand() + "  " + stDim.Render(homeTilde(mustGetwd()))
}

// cmdPlaceholder is the hint shown in the command bar when idle.
const cmdPlaceholder = "Type / to see available commands"

// renderCmdBar draws the command input between a top and bottom rule (no corners,
// no side borders) spanning the full terminal width, edge to edge. A bold ❯ is
// the caret. In nav mode we render the placeholder ourselves rather than via the
// textinput, whose blurred placeholder path drops its first character (the stray
// "T" artifact).
func (a App) renderCmdBar() string {
	caret := stAccentB.Render("❯")
	var content string
	if a.cmdMode {
		content = caret + " " + a.cmdInput.View()
	} else {
		content = caret + " " + stDimmer.Render(cmdPlaceholder)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, true, false). // top + bottom only
		BorderForeground(cAccent).
		Padding(0, 1).
		Width(a.width). // span the entire terminal width, end to end
		Render(content)
}

// renderContextBar draws the bottom context bar (e.g. remove confirmation),
// separated by a top border. Only rendered when ctx is non-empty.
func (a App) renderContextBar(width int, ctx string) string {
	return lipgloss.NewStyle().Foreground(cDim).
		BorderTop(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(cLine).
		Width(width).Render(ctx)
}

func keysString(keys [][2]string) string {
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, stAccentB.Render(k[0])+" "+stDim.Render(k[1]))
	}
	return strings.Join(parts, "   ")
}

// ---- small helpers ----------------------------------------------------------

func stErr(s string) string {
	return lipgloss.NewStyle().Foreground(cInfo).Render("✗ ") + stDim.Render(s)
}
func stOK(s string) string {
	return lipgloss.NewStyle().Foreground(cDone).Render("✓ ") + stDim.Render(s)
}
func stWarn(s string) string {
	return lipgloss.NewStyle().Foreground(cRun).Render("! ") + stDim.Render(s)
}

// clampHeight pads s with blank lines (or truncates) so it occupies exactly h
// rows — keeps the status bar pinned to the bottom regardless of body size.
func clampHeight(s string, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// windowAround returns at most max items from lines, scrolled to keep cursor
// visible.
func windowAround(lines []string, cursor, max int) []string {
	v, _, _ := windowAroundInfo(lines, cursor, max)
	return v
}

// windowAroundInfo is windowAround plus whether there are hidden items above
// and below the visible slice.
func windowAroundInfo(lines []string, cursor, max int) (visible []string, scrollUp, scrollDown bool) {
	if len(lines) <= max {
		return lines, false, false
	}
	start := cursor - max/2
	if start < 0 {
		start = 0
	}
	if start+max > len(lines) {
		start = len(lines) - max
	}
	return lines[start : start+max], start > 0, start+max < len(lines)
}

func mustGetwd() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// homeTilde abbreviates the user's home dir prefix to ~.
func homeTilde(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

// openIDE launches the configured IDE on path, non-blocking. A blank IDE
// command falls back to PATH auto-detection. The returned string is empty on
// success, or an actionable message when the command isn't installed.
func openIDE(ide, path string) string {
	if strings.TrimSpace(ide) == "" {
		ide = config.DetectIDE()
	}
	name, args := worktree.SplitLaunch(ide)
	if _, err := exec.LookPath(name); err != nil {
		return "IDE command '" + name + "' not found on PATH — set \"ide\" in ~/.config/fleet/config.json (e.g. \"cursor\")"
	}
	args = append(args, path)
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return "could not open IDE (" + name + "): " + err.Error()
	}
	go func() { _ = cmd.Wait() }()
	return ""
}
