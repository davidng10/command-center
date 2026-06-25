package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"command-center/internal/session"
	"command-center/internal/term"
	"command-center/internal/worktree"
)

// worktreeItem is one row in the repo detail view — either a fleet session, an
// externally-created worktree, or the bare repo.
type worktreeItem struct {
	Path       string
	Branch     string
	IsBare     bool
	HasSession bool
	Session    session.Session
}

func (w worktreeItem) isExternal() bool {
	return !w.IsBare && !w.HasSession
}

// refreshDetailItems rebuilds the worktree list for the selected repo by
// merging `git worktree list` output with fleet's session registry.
func (a *App) refreshDetailItems() {
	if a.selectedRepo == "" {
		a.detailItems = nil
		return
	}
	gitWts, _ := worktree.ListWorktrees(a.selectedRepo)

	sessionByPath := map[string]session.Session{}
	for _, s := range a.reg.All() {
		if s.RepoDir == a.selectedRepo {
			sessionByPath[filepath.Clean(s.WorktreePath)] = s
		}
	}

	var items []worktreeItem
	for _, wt := range gitWts {
		item := worktreeItem{
			Path:   wt.Path,
			Branch: wt.Branch,
			IsBare: wt.IsBare,
		}
		cp := filepath.Clean(wt.Path)
		if s, ok := sessionByPath[cp]; ok {
			item.HasSession = true
			item.Session = s
			delete(sessionByPath, cp)
		}
		items = append(items, item)
	}

	// Fleet sessions whose worktree no longer appears in git list.
	for _, s := range sessionByPath {
		items = append(items, worktreeItem{
			Path:       s.WorktreePath,
			Branch:     s.Branch,
			HasSession: true,
			Session:    s,
		})
	}

	a.detailItems = items
}

func (a *App) enterRepo(repoPath string) {
	a.selectedRepo = repoPath
	a.refreshDetailItems()
	a.detailCursor = 0
	a.scr = scrRepoDetail
	a.flash = ""
	a.exitCmdMode()
	a.exitSearchMode()
}

func (a App) filteredDetailItems() []worktreeItem {
	if a.searchFilter == "" {
		return a.detailItems
	}
	q := strings.ToLower(a.searchFilter)
	var out []worktreeItem
	for _, it := range a.detailItems {
		if strings.Contains(strings.ToLower(it.Branch), q) {
			out = append(out, it)
		}
	}
	return out
}

func (a *App) clampDetailCursor(n int) {
	if a.detailCursor >= n {
		a.detailCursor = n - 1
	}
	if a.detailCursor < 0 {
		a.detailCursor = 0
	}
}

func (a App) selectedDetailItem(items []worktreeItem) (worktreeItem, bool) {
	if a.detailCursor < 0 || a.detailCursor >= len(items) {
		return worktreeItem{}, false
	}
	return items[a.detailCursor], true
}

// ---- update -----------------------------------------------------------------

func (a App) updateRepoDetail(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.busyLabel != "" {
		return a, nil
	}

	items := a.filteredDetailItems()
	a.clampDetailCursor(len(items))

	// 1) Confirming a removal.
	if a.confirmRemoveID != "" {
		switch m.String() {
		case "y", "Y":
			return a.removeSession(a.confirmRemoveID)
		default:
			a.confirmRemoveID = ""
			a.flash = ""
			return a, nil
		}
	}

	// 2) Command-bar mode.
	if a.cmdMode {
		switch m.Type {
		case tea.KeyEnter:
			cmd := strings.TrimSpace(a.cmdInput.Value())
			a.exitCmdMode()
			return a.runCommand(cmd)
		case tea.KeyEsc:
			a.exitCmdMode()
			return a, nil
		}
		var cmd tea.Cmd
		a.cmdInput, cmd = a.cmdInput.Update(m)
		if a.cmdInput.Value() == "" {
			a.exitCmdMode()
		}
		return a, cmd
	}

	// 2b) Search mode.
	if a.searchMode {
		switch m.Type {
		case tea.KeyEnter, tea.KeyEsc:
			a.exitSearchMode()
			return a, nil
		case tea.KeyUp:
			if a.detailCursor > 0 {
				a.detailCursor--
			}
			return a, nil
		case tea.KeyDown:
			if a.detailCursor < len(items)-1 {
				a.detailCursor++
			}
			return a, nil
		}
		var cmd tea.Cmd
		a.cmdInput, cmd = a.cmdInput.Update(m)
		a.searchFilter = a.cmdInput.Value()
		if a.searchFilter == "" {
			a.exitSearchMode()
		} else {
			a.detailCursor = 0
		}
		return a, cmd
	}

	// 3) Navigation mode.
	switch m.String() {
	case "up", "k":
		if a.detailCursor > 0 {
			a.detailCursor--
		}
	case "down", "j":
		if a.detailCursor < len(items)-1 {
			a.detailCursor++
		}
	case "/":
		a.enterCmdMode()
	case "enter":
		if it, ok := a.selectedDetailItem(items); ok {
			if it.HasSession && it.Session.State == session.StateInactive {
				return a.relaunchSession(it.Session)
			}
			path := it.Path
			if it.HasSession {
				path = it.Session.WorktreePath
			}
			if path != "" {
				if e := openIDE(a.global.IDE, path); e != "" {
					a.flash = stWarn(e)
				} else {
					a.flash = stOK("opened " + it.Branch + " in IDE")
				}
			}
		}
	case "t":
		if it, ok := a.selectedDetailItem(items); ok {
			if it.HasSession {
				return a.openTerminal(it.Session)
			}
			if it.Path != "" {
				a.busyLabel = "Opening terminal…"
				branch, dir, terminal := it.Branch, it.Path, a.global.Terminal
				return a, func() tea.Msg {
					if err := term.SpawnShell(dir, terminal); err != nil {
						return openTermResultMsg{branch: branch, err: err.Error()}
					}
					return openTermResultMsg{branch: branch}
				}
			}
		}
	case "o":
		if it, ok := a.selectedDetailItem(items); ok {
			path := it.Path
			if it.HasSession {
				path = it.Session.WorktreePath
			}
			if path != "" {
				if e := openIDE(a.global.IDE, path); e != "" {
					a.flash = stWarn(e)
				} else {
					a.flash = stOK("opened " + it.Branch + " in IDE")
				}
			}
		}
	case "x":
		if it, ok := a.selectedDetailItem(items); ok && it.HasSession {
			a.confirmRemoveID = it.Session.ID
			a.flash = ""
		}
	case "esc":
		a.scr = scrHome
		a.selectedRepo = ""
		a.detailItems = nil
		a.detailCursor = 0
		a.flash = ""
		a.exitSearchMode()
	default:
		if isSearchKey(m) {
			a.enterSearchMode(m.String())
			return a, nil
		}
	}
	return a, nil
}

// ---- view -------------------------------------------------------------------

func (a App) viewRepoDetail() string {
	inner := a.innerWidth()
	items := a.filteredDetailItems()
	a.clampDetailCursor(len(items))
	repoName := filepath.Base(a.selectedRepo)

	header := a.repoDetailHeader(inner, repoName, items)

	if len(a.detailItems) == 0 {
		body := a.detailEmptyState(inner)
		keys := [][2]string{{"/new", "new worktree"}, {"Esc", "back"}}
		return a.frame(header, body, "", keys, true)
	}

	if len(items) == 0 && a.searchFilter != "" {
		body := "\n" + "      " + stDimmer.Render("no worktrees matching \""+a.searchFilter+"\"")
		keys := [][2]string{{"Esc", "clear search"}}
		return a.frame(header, body, "", keys, true)
	}

	rows := make([]string, len(items))
	for i, it := range items {
		rows[i] = a.renderDetailRow(it, i == a.detailCursor, inner)
	}
	maxRows := (a.height - 11) / 2
	if maxRows < 1 {
		maxRows = 1
	}
	visible, scrollUp, scrollDown := windowAroundInfo(rows, a.detailCursor, maxRows)

	var parts []string
	if scrollUp {
		parts = append(parts, "      "+stDimmer.Render(fmt.Sprintf("↑ %d more", a.detailCursor-maxRows/2)))
	}
	parts = append(parts, visible...)
	if scrollDown {
		below := len(rows) - (a.detailCursor + maxRows/2 + 1)
		if below < 1 {
			below = len(rows) - len(visible)
		}
		parts = append(parts, "      "+stDimmer.Render(fmt.Sprintf("↓ %d more", below)))
	}

	body := "\n" + strings.Join(parts, "\n") + "\n"

	ctx := ""
	keys := [][2]string{{"↑↓", "navigate"}, {"t", "terminal"}, {"o", "open IDE"}, {"x", "remove"}, {"/", "command"}, {"Esc", "back"}}
	if it, ok := a.selectedDetailItem(items); ok && it.HasSession && it.Session.State == session.StateInactive {
		keys = [][2]string{{"↑↓", "navigate"}, {"enter", "restart agent"}, {"t", "terminal"}, {"o", "open IDE"}, {"x", "remove"}, {"/", "command"}, {"Esc", "back"}}
	}
	if a.searchMode {
		keys = [][2]string{{"↑↓", "navigate"}, {"Esc", "clear search"}}
	}
	if a.cmdMode {
		keys = [][2]string{{"enter", "run"}, {"Esc", "cancel"}}
	}
	if a.confirmRemoveID != "" {
		keys = [][2]string{{"y", "remove"}, {"n", "cancel"}}
		if s, ok := a.reg.Get(a.confirmRemoveID); ok {
			ctx = stInkB.Render("Remove this session?") + stDim.Render(" · "+s.Branch+" (worktree will be deleted)")
		}
	}
	return a.frame(header, body, ctx, keys, true)
}

func (a App) repoDetailHeader(inner int, repoName string, items []worktreeItem) string {
	left := stDim.Render("repos") + stDimmer.Render(" › ") + stInkB.Render(repoName)

	running := 0
	for _, it := range a.detailItems {
		if it.HasSession && it.Session.State == session.StateRunning {
			running++
		}
	}

	var right string
	if running > 0 {
		right = stDim.Render(fmt.Sprintf("%d running", running))
	}

	if right == "" {
		return left
	}
	gap := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (a App) detailEmptyState(inner int) string {
	block := lipgloss.JoinVertical(lipgloss.Center,
		stDimmer.Render("⚇"),
		"",
		stDim.Render("No worktrees"),
		stDimmer.Render("Type ")+stAccent.Render("/new")+stDimmer.Render(" to create your first worktree."),
	)
	return lipgloss.NewStyle().Width(inner).Align(lipgloss.Center).
		Padding(2, 0).Render(block)
}

// renderDetailRow draws one worktree as a two-line block.
func (a App) renderDetailRow(it worktreeItem, selected bool, inner int) string {
	dim := !selected
	faint := func(st lipgloss.Style) lipgloss.Style {
		if dim {
			return st.Faint(true)
		}
		return st
	}

	nameStyle := lipgloss.NewStyle().Foreground(cInk)
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	}

	caret := "  "
	if selected {
		caret = stAccent.Render("❯") + " "
	}

	if it.IsBare {
		branch := it.Branch
		if branch == "" {
			branch = "main"
		}
		name := faint(nameStyle).Render(branch)
		tag := faint(lipgloss.NewStyle().Foreground(cDimmer)).Render("bare")
		bdg := faint(lipgloss.NewStyle().Foreground(cInact)).Render("Main")
		left := caret + name + " " + tag
		gap := inner - lipgloss.Width(left) - lipgloss.Width(bdg) - 1
		if gap < 1 {
			gap = 1
		}
		line1 := left + strings.Repeat(" ", gap) + bdg
		sub := faint(lipgloss.NewStyle().Foreground(cDim)).Render(homeTilde(it.Path))
		line2 := "      " + sub
		return line1 + "\n" + line2
	}

	if it.HasSession {
		s := it.Session
		col := stateColor(s.State)
		bulletColor := col
		if s.State == session.StateRunning && !a.pulseOn {
			bulletColor = cDimmer
		}
		bull := faint(lipgloss.NewStyle().Foreground(bulletColor)).Render("●")
		bdg := faint(lipgloss.NewStyle().Foreground(col)).Render(s.State.Label())

		nameMax := inner - lipgloss.Width(bdg) - 10
		if nameMax < 8 {
			nameMax = 8
		}
		name := faint(nameStyle).Render(truncate(s.Branch, nameMax))
		left := caret + bull + " " + name
		gap := inner - lipgloss.Width(left) - lipgloss.Width(bdg) - 1
		if gap < 1 {
			gap = 1
		}
		line1 := left + strings.Repeat(" ", gap) + bdg

		sub := faint(lipgloss.NewStyle().Foreground(cDim)).
			Render(fmt.Sprintf("%s · %s · %s", s.Base, s.Age(a.now), activityPhrase(s, a.now)))
		line2 := "      " + sub
		return line1 + "\n" + line2
	}

	// External worktree (not created by fleet)
	name := faint(nameStyle).Render(truncate(it.Branch, inner-30))
	tag := faint(lipgloss.NewStyle().Foreground(cDimmer)).Render("external")
	bdg := faint(lipgloss.NewStyle().Foreground(cInact)).Render("No session")
	left := caret + name + " " + tag
	gap := inner - lipgloss.Width(left) - lipgloss.Width(bdg) - 1
	if gap < 1 {
		gap = 1
	}
	line1 := left + strings.Repeat(" ", gap) + bdg
	sub := faint(lipgloss.NewStyle().Foreground(cDim)).Render("no fleet session")
	line2 := "      " + sub
	return line1 + "\n" + line2
}

