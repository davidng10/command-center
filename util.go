package main

import (
	"regexp"
	"strings"
)

var (
	reNonAlnum   = regexp.MustCompile(`[^a-z0-9]+`)
	reEdgeDashes = regexp.MustCompile(`^-+|-+$`)
	reSpace      = regexp.MustCompile(`\s+`)
	reToken      = regexp.MustCompile(`\{(\w+)\}`)
)

// slugify: "Login Fix!" -> "login-fix". Used to turn a branch name (which may
// contain slashes and mixed case) into a filesystem-safe worktree folder name.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reNonAlnum.ReplaceAllString(s, "-")
	s = reEdgeDashes.ReplaceAllString(s, "")
	return s
}

// sanitizeBranch trims the input and collapses internal whitespace to single
// dashes ("login fix" -> "login-fix"). The branch is otherwise used verbatim,
// so the user keeps full control over naming — slashes, case, and any ticket
// convention their team uses.
func sanitizeBranch(s string) string {
	return reSpace.ReplaceAllString(strings.TrimSpace(s), "-")
}

// splitLaunch splits a launch command into the program name and its arguments
// on whitespace, so a config like "code --wait" runs `code` with `--wait`
// rather than looking for a binary literally named "code --wait". Returns an
// empty name for a blank string. (Shell quoting is not interpreted; a launch
// command needing quoted, space-containing args isn't supported.)
func splitLaunch(cmd string) (string, []string) {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

// applyTemplate fills "{token}" placeholders from vars; unknown tokens are left
// untouched.
func applyTemplate(tpl string, vars map[string]string) string {
	return reToken.ReplaceAllStringFunc(tpl, func(m string) string {
		key := m[1 : len(m)-1] // strip the surrounding { }
		if v, ok := vars[key]; ok {
			return v
		}
		return m
	})
}
