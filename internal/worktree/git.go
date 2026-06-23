// Package worktree holds fleet's git-worktree operations and the pure-data
// helpers that resolve a user's answers into a concrete worktree plan. It is
// provider- and TUI-free, so it can be unit-tested directly.
package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RepoContext describes the git repo we're operating on.
type RepoContext struct {
	Root   string // absolute path to the repo root
	Name   string // basename of the root
	Parent string // directory containing the repo (where worktrees go)
}

// Context inspects dir (or cwd when dir is ""). ok is false when dir is not
// inside a git repo.
func Context(dir string) (RepoContext, bool) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return RepoContext{}, false
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return RepoContext{}, false
	}
	return ContextFromRoot(root), true
}

// ContextFromRoot builds a RepoContext from a known repo root (e.g. one the
// directory browser already verified is a repo).
func ContextFromRoot(root string) RepoContext {
	return RepoContext{
		Root:   root,
		Name:   filepath.Base(root),
		Parent: filepath.Dir(root),
	}
}

// Git runs a git command in dir, returning trimmed combined output.
func Git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// GitOK reports whether a git command succeeds, discarding output.
func GitOK(dir string, args ...string) bool {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// SpawnInherit runs name+args in dir wired to the real terminal.
func SpawnInherit(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SpawnQuiet runs name+args in dir discarding output (used for dep installs).
func SpawnQuiet(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}
