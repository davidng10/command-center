package worktree

import (
	"fmt"
	"os"
	"path/filepath"

	"command-center/internal/config"
)

// Plan is the fully-resolved result of the user's answers — pure data, so it
// can be unit-tested without any TUI.
type Plan struct {
	Branch       string
	WorktreeName string
	WorktreePath string
}

// BuildPlan derives the worktree location from the user-supplied branch name.
func BuildPlan(repo RepoContext, cfg config.Config, branchRaw string) Plan {
	branch := SanitizeBranch(branchRaw)
	wtName := ApplyTemplate(cfg.WorktreeName, map[string]string{
		"repo": repo.Name, "branch": Slugify(branch),
	})
	return Plan{
		Branch:       branch,
		WorktreeName: wtName,
		WorktreePath: filepath.Join(repo.Parent, wtName),
	}
}

// ResolveStartPoint prefers a fresh origin/<base>, falling back to the local
// branch when there's no remote. fetchErr is the (non-fatal) error from
// refreshing origin/<base>; the caller should warn on it, since the resolved
// start point may be stale when the fetch failed.
func ResolveStartPoint(repo RepoContext, cfg config.Config, base string) (string, error) {
	startPoint := base
	var fetchErr error
	if cfg.Fetch {
		if out, err := Git(repo.Root, "fetch", "origin", base); err != nil {
			fetchErr = fmt.Errorf("%w: %s", err, out)
		}
		if GitOK(repo.Root, "rev-parse", "--verify", "origin/"+base) {
			startPoint = "origin/" + base
		}
	}
	return startPoint, fetchErr
}

// AddWorktree creates the worktree + branch off startPoint.
func AddWorktree(repo RepoContext, p Plan, startPoint string) error {
	if out, err := Git(repo.Root, "worktree", "add", p.WorktreePath, "-b", p.Branch, startPoint); err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

// Remove deletes a worktree fleet created. It first tries a clean
// `git worktree remove`, then retries with --force (a worktree with
// uncommitted changes or a running process refuses a plain remove). The repo's
// own worktree bookkeeping is pruned either way.
func Remove(repo RepoContext, worktreePath string) error {
	if _, err := Git(repo.Root, "worktree", "remove", worktreePath); err == nil {
		return nil
	}
	if out, err := Git(repo.Root, "worktree", "remove", "--force", worktreePath); err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

// CopyConfiguredFiles copies the configured gitignored files (those that exist)
// from the repo root into the worktree, returning what was copied.
func CopyConfiguredFiles(repo RepoContext, cfg config.Config, p Plan) []string {
	var copied []string
	for _, rel := range cfg.CopyFiles {
		src := filepath.Join(repo.Root, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := CopyFile(src, filepath.Join(p.WorktreePath, rel)); err == nil {
			copied = append(copied, rel)
		}
	}
	return copied
}
