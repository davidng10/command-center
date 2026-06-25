package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// logo renders the block-letter "fleet" art with a depth gradient (bright top → dim bottom).
func logo() [3]string {
	return [3]string{
		lipgloss.NewStyle().Foreground(cLogoTop).Render("█▀▀ █   █▀▀ █▀▀ ▀█▀"),
		lipgloss.NewStyle().Foreground(cLogoMid).Render("█▀  █   █▀  █▀   █"),
		lipgloss.NewStyle().Foreground(cLogoBot).Render("▀   ▀▀▀ ▀▀▀ ▀▀▀  ▀"),
	}
}

// wizHeader renders the "<Label>   <title> · step N of M" line shared by the
// wizard and onboarding.
func wizHeader(label, title string, step, total int) string {
	return stAccentB.Render(label) + "  " +
		stDim.Render(fmt.Sprintf("%s · step %d of %d", title, step, total))
}

// gitChip renders the small "git" marker next to a repo in the directory list.
func gitChip() string {
	return lipgloss.NewStyle().Foreground(cAccent).Background(lipgloss.Color("#1a1726")).
		Padding(0, 1).Render("git")
}

// optionLine renders one selectable row: an accent caret when selected, the
// label (rendered in fg, or accent+bold when selected), an optional git chip, and
// a right-aligned dim tag. Selection is shown by caret + color rather than a
// background fill — backgrounds don't survive the resets inside pre-styled
// segments, so this stays correct across terminals.
func optionLine(inner int, text, chip, tag string, selected bool, fg lipgloss.Color) string {
	car := "  "
	style := lipgloss.NewStyle().Foreground(fg)
	if selected {
		car = stCaret.Render("▸") + " "
		style = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	}
	left := car + style.Render(text)
	if chip != "" {
		left += " " + gitChip()
	}
	if tag == "" {
		return left
	}
	gap := inner - lipgloss.Width(left) - lipgloss.Width(tag) - 1
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + stDimmer.Render(tag)
}

// optionList windows a set of rendered option lines around the cursor so a long
// list never overflows, joining them into one block.
func optionList(lines []string, cursor, maxRows int) string {
	if maxRows < 3 {
		maxRows = 3
	}
	shown := windowAround(lines, cursor, maxRows)
	return strings.Join(shown, "\n")
}

// filterBar renders the type-to-search affordance under a list.
func filterBar(inner int, filter, placeholder string) string {
	divider := stDimmer.Render(strings.Repeat("─", min(inner, 48)))
	var txt string
	if filter == "" {
		txt = stDimmer.Render(placeholder)
	} else {
		txt = stInk.Render(filter)
	}
	return divider + "\n" + stDimmer.Render("⌕") + " " + txt + stCaret.Render("▏")
}

// kv renders a left-aligned key/value summary (the review + onboarding done
// screens). Keys are dim; values default to ink, or accent when accent[i] is set.
func kv(rows [][2]string, accentKeys map[string]bool) string {
	var b strings.Builder
	for _, r := range rows {
		key := lipgloss.NewStyle().Foreground(cDim).Width(9).Render(r[0])
		val := r[1]
		if accentKeys[r[0]] {
			val = stAccent.Render(val)
		} else {
			val = stInk.Render(val)
		}
		b.WriteString(key + " " + val + "\n")
	}
	return b.String()
}
