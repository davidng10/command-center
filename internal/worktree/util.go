package worktree

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reNonAlnum   = regexp.MustCompile(`[^a-z0-9]+`)
	reEdgeDashes = regexp.MustCompile(`^-+|-+$`)
	reSpace      = regexp.MustCompile(`\s+`)
	reToken      = regexp.MustCompile(`\{(\w+)\}`)
)

// Slugify: "Login Fix!" -> "login-fix". Used to turn a branch name (which may
// contain slashes and mixed case) into a filesystem-safe worktree folder name.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reNonAlnum.ReplaceAllString(s, "-")
	s = reEdgeDashes.ReplaceAllString(s, "")
	return s
}

// SanitizeBranch trims the input and collapses internal whitespace to single
// dashes ("login fix" -> "login-fix"). The branch is otherwise used verbatim,
// so the user keeps full control over naming — slashes, case, and any ticket
// convention their team uses.
func SanitizeBranch(s string) string {
	return reSpace.ReplaceAllString(strings.TrimSpace(s), "-")
}

// SplitLaunch splits a launch command into the program name and its arguments
// on whitespace, so a config like "code --wait" runs `code` with `--wait`
// rather than looking for a binary literally named "code --wait". Returns an
// empty name for a blank string. (Shell quoting is not interpreted.)
func SplitLaunch(cmd string) (string, []string) {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

// ApplyTemplate fills "{token}" placeholders from vars; unknown tokens are left
// untouched.
func ApplyTemplate(tpl string, vars map[string]string) string {
	return reToken.ReplaceAllStringFunc(tpl, func(m string) string {
		key := m[1 : len(m)-1] // strip the surrounding { }
		if v, ok := vars[key]; ok {
			return v
		}
		return m
	})
}

// CopyFile copies src to dst, creating dst's parent directory.
func CopyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
