package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"command-center/internal/config"
	"command-center/internal/provider"
)

type onbStep int

const (
	obProvider onbStep = iota
	obHooks
	obDone
)

type onboardModel struct {
	prov        provider.Provider
	step        onbStep
	hooksCursor int // 0 = install, 1 = skip
	installed   bool
	installErr  string
}

func newOnboard(prov provider.Provider) *onboardModel {
	return &onboardModel{prov: prov}
}

func (a App) updateOnboard(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	o := a.onb
	if o == nil {
		a.scr = scrHome
		return a, nil
	}
	switch o.step {
	case obProvider:
		switch m.Type {
		case tea.KeyEsc:
			a.scr = scrHome // bail without completing; first-run re-offers next launch
			a.onb = nil
		case tea.KeyEnter:
			o.step = obHooks
		}
	case obHooks:
		switch {
		case m.Type == tea.KeyEsc:
			o.step = obProvider
		case m.String() == "up":
			o.hooksCursor = 0
		case m.String() == "down":
			o.hooksCursor = 1
		case m.Type == tea.KeyEnter:
			if o.hooksCursor == 0 && o.prov != nil {
				if err := o.prov.Install(); err != nil {
					o.installErr = err.Error()
				} else {
					o.installed = true
				}
			}
			o.step = obDone
		}
	case obDone:
		switch m.Type {
		case tea.KeyEsc:
			o.step = obHooks
		case tea.KeyEnter:
			a.global.SetupComplete = true
			if o.prov != nil {
				a.global.DefaultProvider = o.prov.Name()
			}
			_ = config.SaveGlobal(a.global)
			a.scr = scrHome
			a.onb = nil
		}
	}
	return a, nil
}

func (a App) viewOnboard() string {
	o := a.onb
	if o == nil {
		return ""
	}
	inner := a.innerWidth()
	var body, ctx string
	var keys [][2]string

	switch o.step {
	case obProvider:
		opts := optionList([]string{
			optionLine(inner, "Claude Code", "", "available", true, cInk),
			optionLine(inner, "Codex", "", "coming soon", false, cDimmer),
		}, 0, 8)
		body = wizHeader("Welcome to fleet", "Provider", 1, 3) + "\n\n" +
			stInk.Render("Choose your agent provider") + "\n" +
			stDimmer.Render("fleet launches this agent in each worktree and tracks its status · this becomes your default") + "\n\n" +
			opts
		ctx = stInkB.Render("First-run setup") + stDim.Render(" · choose provider")
		keys = [][2]string{{"↑↓", "navigate"}, {"⏎", "next"}, {"Esc", "skip setup"}}

	case obHooks:
		opts := optionList([]string{
			optionLine(inner, "Install hooks", "", "recommended", o.hooksCursor == 0, cInk),
			optionLine(inner, "Skip for now", "", "Active / Inactive only", o.hooksCursor == 1, cInk),
		}, o.hooksCursor, 8)
		body = wizHeader("Welcome to fleet", "Status hooks", 2, 3) + "\n\n" +
			stInk.Render("Set up status tracking") + "\n" +
			stDimmer.Render("fleet adds 4 hooks to ~/.claude/settings.json so it can show Running / Finished / Needs input.") + "\n" +
			stDimmer.Render("Merged with your existing hooks — nothing overwritten. Remove anytime with `fleet uninstall`.") + "\n\n" +
			preflightBlock(o.prov) + "\n" +
			opts
		ctx = stInkB.Render("First-run setup") + stDim.Render(" · status hooks")
		keys = [][2]string{{"↑↓", "navigate"}, {"⏎", "select"}, {"Esc", "back"}}

	case obDone:
		hooksLine := stDimmer.Render("skipped") + stDim.Render(" · Active / Inactive only · run /setup later")
		if o.installed {
			hooksLine = lipgloss.NewStyle().Foreground(cDone).Render("✓ installed") + stDim.Render(" · 4 events → fleet hook")
		} else if o.installErr != "" {
			hooksLine = stWarn("install failed: " + o.installErr)
		}
		rows := [][2]string{{"provider", providerLabel(o.prov)}}
		table := kv(rows, map[string]bool{"provider": true})
		body = wizHeader("Welcome to fleet", "Done", 3, 3) + "\n\n" +
			stInk.Render("You're all set") + "\n\n" +
			table +
			stDim.Render("hooks    ") + hooksLine + "\n\n" +
			stInk.Render("Type ") + stAccent.Render("/new") + stInk.Render(" to create your first session.") + "\n" +
			stDim.Render("Enter = go to home")
		ctx = stInkB.Render("First-run setup") + stDim.Render(" · complete")
		keys = [][2]string{{"⏎", "go to home"}}
	}

	return a.frame(a.brandHeader(inner), body, ctx, keys, false)
}

func preflightBlock(prov provider.Provider) string {
	pf, ok := prov.(provider.Preflighter)
	if !ok {
		return ""
	}
	var lines []string
	for _, it := range pf.Preflight() {
		mark := lipgloss.NewStyle().Foreground(cDone).Render("✓ ")
		if !it.OK {
			mark = lipgloss.NewStyle().Foreground(cRun).Render("! ")
		}
		lines = append(lines, mark+stDim.Render(it.Label))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func providerLabel(p provider.Provider) string {
	if p == nil {
		return "claude"
	}
	switch p.Name() {
	case "claude":
		return "Claude Code"
	default:
		return p.Name()
	}
}
