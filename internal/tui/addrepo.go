package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"command-center/internal/config"
	"command-center/internal/fsbrowse"
)

// addRepoModel holds state for the /add directory browser.
type addRepoModel struct {
	stack  []string
	cursor int
	filter string
}

func newAddRepoModel() *addRepoModel {
	start := homeDir()
	return &addRepoModel{
		stack: []string{start},
	}
}

func (m *addRepoModel) dirDisplay() []dirItem {
	cur := m.stack[len(m.stack)-1]
	f := strings.ToLower(m.filter)
	var items []dirItem
	if f == "" && len(m.stack) > 1 {
		items = append(items, dirItem{Label: "..", IsUp: true})
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

func (m *addRepoModel) popDir() {
	if len(m.stack) > 1 {
		m.stack = m.stack[:len(m.stack)-1]
		m.cursor = 0
		m.filter = ""
	}
}

func (a App) updateAddRepo(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	ar := a.add
	if ar == nil {
		a.scr = scrHome
		return a, nil
	}

	if m.Type == tea.KeyEsc {
		a.scr = scrHome
		a.add = nil
		return a, nil
	}

	items := ar.dirDisplay()

	switch {
	case m.String() == "up":
		if ar.cursor > 0 {
			ar.cursor--
		}
	case m.String() == "down":
		if ar.cursor < len(items)-1 {
			ar.cursor++
		}
	case m.Type == tea.KeyLeft:
		ar.popDir()
	case m.Type == tea.KeyEnter:
		if ar.cursor >= 0 && ar.cursor < len(items) {
			it := items[ar.cursor]
			switch {
			case it.IsUp:
				ar.popDir()
			case it.IsRepo:
				_ = config.AddRepo(it.Path)
				a.refreshRepos()
				a.flash = stOK("added " + it.Label)
				a.scr = scrHome
				a.add = nil
			default:
				ar.stack = append(ar.stack, it.Path)
				ar.cursor = 0
				ar.filter = ""
			}
		}
	case m.Type == tea.KeyBackspace:
		if ar.filter != "" {
			ar.filter = trimLastRune(ar.filter)
			ar.cursor = 0
		}
	case m.Type == tea.KeyRunes:
		ar.filter += string(m.Runes)
		ar.cursor = 0
	case m.Type == tea.KeySpace:
		ar.filter += " "
		ar.cursor = 0
	}
	return a, nil
}

func (a App) viewAddRepo() string {
	ar := a.add
	if ar == nil {
		return ""
	}
	inner := a.innerWidth()
	maxRows := max(a.height-12, 4)

	items := ar.dirDisplay()
	lines := make([]string, len(items))
	for i, it := range items {
		lines[i] = renderDirItem(inner, it, i == ar.cursor)
	}
	list := stDimmer.Render("no matching folders here")
	if len(lines) > 0 {
		list = optionList(lines, ar.cursor, maxRows)
	}

	header := stDim.Render("repos") + stDimmer.Render(" › ") + stInkB.Render("add repo")

	body := wizHeader("Add repo", "Select", 1, 1) + "\n" +
		crumb(ar.stack) + "\n" +
		stInk.Render("Select a git repository to add") + "\n" +
		stDimmer.Render("navigate folders · select a ") + stAccent.Render("git") + stDimmer.Render(" repo to add it") + "\n\n" +
		list + "\n" + filterBar(inner, ar.filter, "type to search this folder")

	ctx := stInkB.Render("Add repo") + stDim.Render(" · choose a git repository")
	keys := [][2]string{{"↑↓", "navigate"}, {"⏎", addEnterLabel(items, ar.cursor)}, {"←", "up"}, {"type", "search"}, {"Esc", "cancel"}}
	return a.frame(header, body, ctx, keys, false)
}

func addEnterLabel(items []dirItem, cursor int) string {
	if cursor < 0 || cursor >= len(items) {
		return "open"
	}
	it := items[cursor]
	switch {
	case it.IsUp:
		return "up"
	case it.IsRepo:
		return "add repo"
	default:
		return "open"
	}
}
