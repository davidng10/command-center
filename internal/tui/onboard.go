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
	obDone
)

type onboardModel struct {
	prov provider.Provider
	step onbStep
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
			o.step = obDone
		}
	case obDone:
		switch m.Type {
		case tea.KeyEsc:
			o.step = obProvider
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
		body = wizHeader("Welcome to fleet", "Provider", 1, 2) + "\n\n" +
			stInk.Render("Choose your agent provider") + "\n" +
			stDimmer.Render("fleet launches this agent in each worktree and tracks its status · this becomes your default") + "\n\n" +
			opts
		ctx = stInkB.Render("First-run setup") + stDim.Render(" · choose provider")
		keys = [][2]string{{"↑↓", "navigate"}, {"⏎", "next"}, {"Esc", "skip setup"}}

	case obDone:
		rows := [][2]string{{"provider", providerLabel(o.prov)}}
		table := kv(rows, map[string]bool{"provider": true})
		body = wizHeader("Welcome to fleet", "Done", 2, 2) + "\n\n" +
			stInk.Render("You're all set") + "\n" +
			stDimmer.Render("fleet tracks each session's status automatically — Running / Finished / Inactive.") + "\n" +
			stDimmer.Render("Status hooks are scoped to the sessions fleet launches (via `claude --settings`); your global Claude config is left untouched.") + "\n\n" +
			table +
			preflightBlock(o.prov) + "\n\n" +
			stInk.Render("Type ") + stAccent.Render("/new") + stInk.Render(" to create your first session.") + "\n" +
			stDim.Render("Enter = go to home")
		ctx = stInkB.Render("First-run setup") + stDim.Render(" · complete")
		keys = [][2]string{{"⏎", "go to home"}, {"Esc", "back"}}
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
