package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCreateWorktreeEndToEnd builds a throwaway git repo and exercises the real
// git side of the flow: resolveStartPoint -> addWorktree -> copyConfiguredFiles.
func TestCreateWorktreeEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	root := filepath.Join(tmp, "demo-repo")
	mustMkdir(t, root)

	// init a repo on `main` with a gitignored .env and one commit
	run(t, root, "git", "init", "-q", "-b", "main")
	run(t, root, "git", "config", "user.email", "t@t.co")
	run(t, root, "git", "config", "user.name", "t")
	mustWrite(t, filepath.Join(root, ".gitignore"), "node_modules/\n.env\n")
	mustWrite(t, filepath.Join(root, ".env"), "SECRET=shh\n")
	mustWrite(t, filepath.Join(root, "package.json"), `{"name":"demo"}`)
	run(t, root, "git", "add", ".gitignore", "package.json")
	run(t, root, "git", "commit", "-qm", "init")

	repo := RepoContext{Root: root, Name: "demo-repo", Parent: tmp}
	cfg := defaultConfig()
	cfg.Fetch = false // no remote in the sandbox

	p := buildPlan(repo, cfg, "task/SP-1234-login fix")
	if p.Branch != "task/SP-1234-login-fix" {
		t.Fatalf("unexpected branch %q", p.Branch)
	}

	startPoint, fetchErr := resolveStartPoint(repo, cfg, "main")
	if fetchErr != nil {
		t.Fatalf("unexpected fetch error: %v", fetchErr)
	}
	if startPoint != "main" {
		t.Fatalf("startPoint = %q, want main", startPoint)
	}

	if err := addWorktree(repo, p, startPoint); err != nil {
		t.Fatalf("addWorktree: %v", err)
	}

	// worktree folder exists and is checked out on the right branch
	if _, err := os.Stat(p.WorktreePath); err != nil {
		t.Fatalf("worktree path missing: %v", err)
	}
	branch, err := git(p.WorktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	if branch != p.Branch {
		t.Fatalf("checked out %q, want %q", branch, p.Branch)
	}

	// the gitignored .env was carried over
	copied := copyConfiguredFiles(repo, cfg, p)
	if len(copied) == 0 {
		t.Fatalf("expected .env to be copied")
	}
	got, err := os.ReadFile(filepath.Join(p.WorktreePath, ".env"))
	if err != nil || string(got) != "SECRET=shh\n" {
		t.Fatalf("copied .env wrong: %q err=%v", string(got), err)
	}

	// cleanup
	run(t, root, "git", "worktree", "remove", p.WorktreePath)
}

// TestResolveStartPointFetchFailure verifies that a failed fetch is surfaced
// (not swallowed) and that we still fall back to the local base branch.
func TestResolveStartPointFetchFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	root := filepath.Join(tmp, "no-remote")
	mustMkdir(t, root)
	run(t, root, "git", "init", "-q", "-b", "main")
	run(t, root, "git", "config", "user.email", "t@t.co")
	run(t, root, "git", "config", "user.name", "t")
	mustWrite(t, filepath.Join(root, "f"), "x")
	run(t, root, "git", "add", "f")
	run(t, root, "git", "commit", "-qm", "init")

	repo := RepoContext{Root: root, Name: "no-remote", Parent: tmp}
	cfg := defaultConfig() // Fetch is true by default, but there is no 'origin'

	startPoint, fetchErr := resolveStartPoint(repo, cfg, "main")
	if fetchErr == nil {
		t.Fatal("expected a fetch error when there is no origin remote")
	}
	if startPoint != "main" {
		t.Fatalf("startPoint = %q, want fallback to local 'main'", startPoint)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
