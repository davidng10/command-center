package worktree

import (
	"strings"
)

// WorktreeEntry is one worktree from `git worktree list --porcelain`.
type WorktreeEntry struct {
	Path   string
	Branch string
	IsBare bool
}

// ListWorktrees returns all worktrees for a repo by parsing `git worktree list --porcelain`.
func ListWorktrees(repoRoot string) ([]WorktreeEntry, error) {
	out, err := Git(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreeList(out), nil
}

func parseWorktreeList(output string) []WorktreeEntry {
	var entries []WorktreeEntry
	var current WorktreeEntry

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				entries = append(entries, current)
				current = WorktreeEntry{}
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "bare":
			current.IsBare = true
		}
	}

	if current.Path != "" {
		entries = append(entries, current)
	}

	return entries
}
