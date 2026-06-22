package main

import (
	"regexp"
	"strings"
)

var (
	reNonAlnum   = regexp.MustCompile(`[^a-z0-9]+`)
	reEdgeDashes = regexp.MustCompile(`^-+|-+$`)
	reSPPrefix   = regexp.MustCompile(`(?i)^sp[-\s]?`)
	reNonAlnumI  = regexp.MustCompile(`(?i)[^a-z0-9]`)
	reToken      = regexp.MustCompile(`\{(\w+)\}`)
)

// slugify: "Login Fix!" -> "login-fix"
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reNonAlnum.ReplaceAllString(s, "-")
	s = reEdgeDashes.ReplaceAllString(s, "")
	return s
}

// normalizeTicket strips a leading "SP-"/"SP" and any non-alphanumerics so both
// "SP-1234" and "1234" normalize to "1234". The prefix is re-added via the
// branch pattern.
func normalizeTicket(s string) string {
	s = strings.TrimSpace(s)
	s = reSPPrefix.ReplaceAllString(s, "")
	s = reNonAlnumI.ReplaceAllString(s, "")
	return s
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
