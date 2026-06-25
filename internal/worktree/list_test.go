package worktree

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseWorktreeList(t *testing.T) {
	input := `worktree /Users/alice/repos/myapp
HEAD abc123def456
branch refs/heads/main
bare

worktree /Users/alice/repos/myapp-feat-login
HEAD 789abc
branch refs/heads/feat/login

worktree /Users/alice/repos/myapp-fix-bug
HEAD deadbeef
branch refs/heads/fix/bug-123
`
	entries := parseWorktreeList(input)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Path != "/Users/alice/repos/myapp" || entries[0].Branch != "main" || !entries[0].IsBare {
		t.Fatalf("entry 0 wrong: %+v", entries[0])
	}
	if entries[1].Path != "/Users/alice/repos/myapp-feat-login" || entries[1].Branch != "feat/login" || entries[1].IsBare {
		t.Fatalf("entry 1 wrong: %+v", entries[1])
	}
	if entries[2].Branch != "fix/bug-123" {
		t.Fatalf("entry 2 branch wrong: %+v", entries[2])
	}
}

func TestListWorktreesReal(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	root := filepath.Join(tmp, "repo")
	mustMkdir(t, root)
	run(t, root, "git", "init", "-q", "-b", "main")
	run(t, root, "git", "config", "user.email", "t@t.co")
	run(t, root, "git", "config", "user.name", "t")
	mustWrite(t, filepath.Join(root, "f"), "x")
	run(t, root, "git", "add", "f")
	run(t, root, "git", "commit", "-qm", "init")

	entries, err := ListWorktrees(root)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (the main worktree), got %d", len(entries))
	}
	if entries[0].Branch != "main" {
		t.Fatalf("expected branch main, got %q", entries[0].Branch)
	}
}
