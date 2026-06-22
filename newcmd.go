package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	dim    = lipgloss.NewStyle().Faint(true)
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	cyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// Plan is the fully-resolved result of the user's answers — pure data, so it
// can be unit-tested without any TUI.
type Plan struct {
	Branch       string
	WorktreeName string
	WorktreePath string
}

// buildPlan derives the worktree location from the user-supplied branch name.
func buildPlan(repo RepoContext, cfg Config, branchRaw string) Plan {
	branch := sanitizeBranch(branchRaw)
	wtName := applyTemplate(cfg.WorktreeName, map[string]string{
		"repo": repo.Name, "branch": slugify(branch),
	})
	return Plan{
		Branch:       branch,
		WorktreeName: wtName,
		WorktreePath: filepath.Join(repo.Parent, wtName),
	}
}

// resolveStartPoint prefers a fresh origin/<base>, falling back to the local
// branch when there's no remote. fetchErr is the (non-fatal) error from
// refreshing origin/<base>; the caller should warn on it, since the resolved
// start point may be stale when the fetch failed.
func resolveStartPoint(repo RepoContext, cfg Config, base string) (string, error) {
	startPoint := base
	var fetchErr error
	if cfg.Fetch {
		if out, err := git(repo.Root, "fetch", "origin", base); err != nil {
			fetchErr = fmt.Errorf("%w: %s", err, out)
		}
		if gitOK(repo.Root, "rev-parse", "--verify", "origin/"+base) {
			startPoint = "origin/" + base
		}
	}
	return startPoint, fetchErr
}

// addWorktree creates the worktree + branch off startPoint.
func addWorktree(repo RepoContext, p Plan, startPoint string) error {
	if out, err := git(repo.Root, "worktree", "add", p.WorktreePath, "-b", p.Branch, startPoint); err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

// copyConfiguredFiles copies the configured gitignored files (those that exist)
// from the repo root into the worktree, returning what was copied.
func copyConfiguredFiles(repo RepoContext, cfg Config, p Plan) []string {
	var copied []string
	for _, rel := range cfg.CopyFiles {
		src := filepath.Join(repo.Root, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, filepath.Join(p.WorktreePath, rel)); err == nil {
			copied = append(copied, rel)
		}
	}
	return copied
}

// runNew drives the interactive `fleet --new` flow.
func runNew() error {
	repo, ok := getRepoContext()
	if !ok {
		return errors.New("not inside a git repository — cd into your repo and try again")
	}
	cfg := loadConfig(repo.Root)

	var branchRaw string
	base := cfg.DefaultBase

	intake := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Branch name?").Placeholder("task/SP-1234-login-fix").
			Value(&branchRaw).Validate(required),
		huh.NewSelect[string]().Title("Base branch?").
			Options(huh.NewOptions(cfg.BaseBranches...)...).Value(&base),
	))
	if err := intake.Run(); err != nil {
		return formErr(err)
	}

	p := buildPlan(repo, cfg, branchRaw)
	if _, err := os.Stat(p.WorktreePath); err == nil {
		return fmt.Errorf("folder already exists: %s", p.WorktreePath)
	}

	summary := fmt.Sprintf("%s  %s\n%s  %s\n%s  %s",
		dim.Render("branch"), green.Render(p.Branch),
		dim.Render("base  "), base,
		dim.Render("folder"), p.WorktreePath,
	)
	confirm := true
	review := huh.NewForm(huh.NewGroup(
		huh.NewNote().Title("Will create").Description(summary),
		huh.NewConfirm().Title("Create this worktree?").Value(&confirm),
	))
	if err := review.Run(); err != nil {
		return formErr(err)
	}
	if !confirm {
		fmt.Println("Aborted.")
		return nil
	}

	startPoint, fetchErr := resolveStartPoint(repo, cfg, base)
	if fetchErr != nil {
		fmt.Printf("%s fetch failed, %s may be out of date: %v\n",
			yellow.Render("!"), startPoint, fetchErr)
	}
	fmt.Printf("%s %s\n", dim.Render("base:"), startPoint)

	if err := addWorktree(repo, p, startPoint); err != nil {
		return fmt.Errorf("git worktree add failed: %w", err)
	}
	fmt.Printf("%s worktree at %s\n", green.Render("✓"), cyan.Render(p.WorktreePath))

	if copied := copyConfiguredFiles(repo, cfg, p); len(copied) > 0 {
		fmt.Printf("%s copied: %s\n", green.Render("✓"), strings.Join(copied, ", "))
	}

	if cfg.Install {
		if pm, ok := detectPackageManager(p.WorktreePath); ok {
			fmt.Printf("%s installing dependencies (%s) ...\n", dim.Render("…"), pm.Name)
			if err := spawnQuiet(p.WorktreePath, pm.Name, pm.Args...); err != nil {
				fmt.Printf("%s install failed — run \"%s %s\" manually\n",
					yellow.Render("!"), pm.Name, strings.Join(pm.Args, " "))
			} else {
				fmt.Printf("%s dependencies installed (%s)\n", green.Render("✓"), pm.Name)
			}
		}
	}

	if cfg.Launch != "" {
		fmt.Printf("\n%s launching %s in %s …\n\n",
			green.Render("Ready."), cyan.Render(cfg.Launch), p.WorktreeName)
		name, args := splitLaunch(cfg.Launch)
		// Hands over the terminal. A non-zero exit from the launched program is
		// its own business (the user quitting claude is a clean exit for us),
		// but failing to *start* it — a bad command or missing binary — is a
		// real error worth surfacing instead of leaving the user with nothing.
		if err := spawnInherit(p.WorktreePath, name, args...); err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return fmt.Errorf("launch %q failed: %w", cfg.Launch, err)
			}
		}
	} else {
		fmt.Printf("\n%s  cd %s\n", green.Render("Done."), p.WorktreePath)
	}
	return nil
}

func required(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("required")
	}
	return nil
}

// formErr treats a user abort (Ctrl+C / Esc) as a clean exit.
func formErr(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return nil
	}
	return err
}

func copyFile(src, dst string) error {
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
