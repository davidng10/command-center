package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"command-center/internal/session"
	"command-center/internal/worktree"
)

func (a App) viewWizard() string {
	w := a.wiz
	if w == nil {
		return ""
	}
	inner := a.innerWidth()
	maxRows := max(a.height-12, 4)

	var body, ctx string
	var keys [][2]string

	switch w.step {
	case wsName:
		body = wizHeader("New session", "Name", 1, w.total()) + "\n\n" +
			stInk.Render("Name this session / branch") + "\n" +
			stDimmer.Render("however your team names branches — e.g. task/SP-1234-login-fix") + "\n\n" +
			"  " + w.nameInput.View()
		ctx = stInkB.Render("New session") + stDim.Render(" · name the branch")
		keys = [][2]string{{"type", "branch name"}, {"enter", "next"}, {"Esc", "cancel"}}

	case wsDir:
		items := w.dirDisplay()
		lines := make([]string, len(items))
		for i, it := range items {
			lines[i] = renderDirItem(inner, it, i == w.dirCursor)
		}
		list := stDimmer.Render("no matching folders here")
		if len(lines) > 0 {
			list = optionList(lines, w.dirCursor, maxRows)
		}
		body = wizHeader("New session", "Directory", 2, w.total()) + "\n" +
			crumb(w.stack) + "\n" +
			stInk.Render("Select the directory to build the worktree in") + "\n" +
			stDimmer.Render("navigate folders · open a ") + stAccent.Render("git") + stDimmer.Render(" repo to select it as the worktree source") + "\n\n" +
			list + "\n" + filterBar(inner, w.dirFilter, "type to search this folder")
		ctx = stInkB.Render("New session") + stDim.Render(" · choose directory")
		keys = [][2]string{{"↑↓", "navigate"}, {"⏎", dirEnterLabel(items, w.dirCursor)}, {"←", "up"}, {"type", "search"}, {"Esc", "back"}}

	case wsBase:
		items := w.baseDisplay()
		lines := make([]string, len(items))
		for i, b := range items {
			lines[i] = renderBaseItem(inner, b, i == w.baseCursor)
		}
		list := stDimmer.Render("no matching branches")
		if len(lines) > 0 {
			list = optionList(lines, w.baseCursor, maxRows)
		}
		body = wizHeader("New session", "Base branch", 3, w.total()) + "\n\n" +
			stInk.Render("Select base branch") + "\n" +
			stDim.Render(fmt.Sprintf("%s · %d branches · the branch your worktree forks from", w.repo.Name, len(w.baseItems))) + "\n\n" +
			list + "\n" + filterBar(inner, w.baseFilter, "search branches")
		ctx = stInkB.Render("New session") + stDim.Render(" · choose base branch")
		keys = [][2]string{{"↑↓", "navigate"}, {"type", "search"}, {"⏎", "select"}, {"Esc", "back"}}

	case wsSetup:
		if w.customizing {
			body = wizHeader("New session", "Setup", 4, w.total()) + "\n\n" +
				stInk.Render("Custom setup command") + "\n" +
				stDimmer.Render("runs in the new worktree — blank to skip") + "\n\n" +
				"  " + w.customInput.View()
			keys = [][2]string{{"type", "command"}, {"enter", "use"}, {"Esc", "back"}}
		} else {
			items := w.setupDisplay()
			lines := make([]string, len(items))
			for i, s := range items {
				lines[i] = optionLine(inner, s.Label, "", "", i == w.setupCursor, cInk)
			}
			list := stDimmer.Render("no matches")
			if len(lines) > 0 {
				list = optionList(lines, w.setupCursor, maxRows)
			}
			body = wizHeader("New session", "Setup", 4, w.total()) + "\n\n" +
				stInk.Render("Setup command") + "\n" +
				stDim.Render("runs once in the new worktree before launch") + "\n\n" +
				list + "\n" + filterBar(inner, w.setupFilter, "filter")
			keys = [][2]string{{"↑↓", "navigate"}, {"type", "filter"}, {"⏎", "select"}, {"Esc", "back"}}
		}
		ctx = stInkB.Render("New session") + stDim.Render(" · setup command")

	case wsConfirm:
		body = a.viewConfirm(inner)
		ctx = stInkB.Render("New session") + stDim.Render(" · review")
		keys = [][2]string{{"enter", "create"}, {"Esc", "back"}}
		if a.busyLabel != "" {
			keys = [][2]string{{"…", "creating worktree"}}
		}
	}

	return a.frame(a.brandHeader(inner), body, ctx, keys, false)
}

func (a App) viewConfirm(inner int) string {
	w := a.wiz
	plan := worktree.BuildPlan(w.repo, w.cfg, w.branch)
	setupLabel := w.setupCmd
	if strings.TrimSpace(setupLabel) == "" {
		setupLabel = "(skip)"
	}
	baseLabel := w.base
	if !strings.HasPrefix(baseLabel, "origin/") {
		baseLabel = "origin/" + baseLabel
	}
	launch := "claude"
	if a.prov != nil {
		if p := a.prov.LaunchSpec(session.Session{}).Program; p != "" {
			launch = p
		}
	}

	rows := [][2]string{
		{"branch", plan.Branch},
		{"in", w.dirLabel},
		{"base", baseLabel},
		{"folder", plan.WorktreeName},
		{"setup", setupLabel},
		{"launch", launch},
	}
	table := kv(rows, map[string]bool{"branch": true})

	ask := stInk.Render("Create this worktree and launch the agent?")
	yn := stDim.Render("Enter = create · Esc = back")
	if a.busyLabel != "" {
		ask = stAccent.Render("Creating worktree and launching the agent…")
		yn = ""
	}
	return wizHeader("New session", "Review", 5, w.total()) + "\n\n" + table + "\n" + ask + "\n" + yn
}

func (w *wizardModel) total() int {
	if w.repo.Root != "" && !w.cfg.Install {
		return 4
	}
	return 5
}

func crumb(stack []string) string {
	parts := make([]string, len(stack))
	for i, p := range stack {
		name := homeTilde(p)
		if i == len(stack)-1 {
			parts[i] = stAccent.Render(name)
		} else {
			// show only the leaf for intermediate entries to keep it short
			leaf := name
			if i > 0 {
				leaf = baseName(p)
			}
			parts[i] = stDim.Render(leaf)
		}
	}
	return strings.Join(parts, stDimmer.Render(" / "))
}

func baseName(p string) string {
	p = strings.TrimRight(p, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func renderDirItem(inner int, it dirItem, selected bool) string {
	if it.IsUp {
		return optionLine(inner, "..", "", "", selected, cInfo)
	}
	if it.IsRepo {
		return optionLine(inner, it.Label, "git", it.Tag, selected, cInk)
	}
	return optionLine(inner, it.Label+"/", "", it.Tag, selected, cInfo)
}

func renderBaseItem(inner int, b baseItem, selected bool) string {
	fg := cInk
	if b.Remote {
		fg = cDim
	}
	tag := b.Tag
	if selected && tag == "" {
		tag = "●"
	}
	return optionLine(inner, b.Name, "", tag, selected, fg)
}

func dirEnterLabel(items []dirItem, cursor int) string {
	if cursor < 0 || cursor >= len(items) {
		return "open"
	}
	it := items[cursor]
	switch {
	case it.IsUp:
		return "up"
	case it.IsRepo:
		return "select repo"
	default:
		return "open"
	}
}

var _ = lipgloss.Width
