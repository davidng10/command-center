package main

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

// getRepoContext inspects the cwd. ok is false when not inside a git repo.
func getRepoContext() (RepoContext, bool) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return RepoContext{}, false
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return RepoContext{}, false
	}
	return RepoContext{
		Root:   root,
		Name:   filepath.Base(root),
		Parent: filepath.Dir(root),
	}, true
}

// git runs a git command in dir, returning trimmed combined output.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// gitOK reports whether a git command succeeds, discarding output.
func gitOK(dir string, args ...string) bool {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// spawnInherit runs name+args in dir wired to the real terminal. Used to hand
// the session over to `claude` (and could run any interactive launch command).
func spawnInherit(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// spawnQuiet runs name+args in dir discarding output (used for dep installs).
func spawnQuiet(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}
