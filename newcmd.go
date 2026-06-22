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

// customCmd is the picker sentinel that means "let me type my own command". It
// uses a NUL byte so it can never collide with a real shell command.
const customCmd = "\x00custom"

// setupOptions builds the setup-command picker: the pre-selected default (if
// any) first, then the common catalog, then Custom… and Skip. def is shown with
// its source (detected/saved/configured) so the user knows where it came from.
func setupOptions(def, source string) []huh.Option[string] {
	var opts []huh.Option[string]
	seen := map[string]bool{}
	add := func(label, val string) {
		if seen[val] {
			return
		}
		seen[val] = true
		opts = append(opts, huh.NewOption(label, val))
	}
	if def != "" {
		add(fmt.Sprintf("%s  (%s)", def, source), def)
	}
	for _, c := range commonSetups {
		add(c, c)
	}
	add("Custom…", customCmd)
	add("Skip (no setup)", "")
	return opts
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

	// Pick the setup command to pre-select: explicit config > remembered choice
	// > auto-detection. The picker lets the user keep it, choose another, type a
	// custom one, or skip — so a wrong default is harmless.
	var setupChoice, setupSource string
	if cfg.Install {
		setupChoice, setupSource = resolveSetupDefault(cfg, repo.Root)
	}

	intakeFields := []huh.Field{
		huh.NewInput().Title("Branch name?").Placeholder("task/SP-1234-login-fix").
			Value(&branchRaw).Validate(required),
		huh.NewSelect[string]().Title("Base branch?").
			Options(huh.NewOptions(cfg.BaseBranches...)...).Value(&base),
	}
	if cfg.Install {
		intakeFields = append(intakeFields, huh.NewSelect[string]().Title("Setup command?").
			Options(setupOptions(setupChoice, setupSource)...).Value(&setupChoice))
	}
	if err := huh.NewForm(huh.NewGroup(intakeFields...)).Run(); err != nil {
		return formErr(err)
	}

	// Resolve the picker selection into the command to run ("" = skip).
	setupCmd := setupChoice
	if cfg.Install && setupChoice == customCmd {
		setupCmd = "" // start the custom field blank rather than with the sentinel
		custom := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Custom setup command").
				Description("runs in the new worktree — blank to skip").Value(&setupCmd),
		))
		if err := custom.Run(); err != nil {
			return formErr(err)
		}
	}

	p := buildPlan(repo, cfg, branchRaw)
	if _, err := os.Stat(p.WorktreePath); err == nil {
		return fmt.Errorf("folder already exists: %s", p.WorktreePath)
	}

	lines := []string{
		fmt.Sprintf("%s  %s", dim.Render("branch"), green.Render(p.Branch)),
		fmt.Sprintf("%s  %s", dim.Render("base  "), base),
		fmt.Sprintf("%s  %s", dim.Render("folder"), p.WorktreePath),
	}
	if cfg.Install {
		setupLabel := setupCmd
		if strings.TrimSpace(setupLabel) == "" {
			setupLabel = "(skip)"
		}
		lines = append(lines, fmt.Sprintf("%s  %s", dim.Render("setup "), setupLabel))
	}

	confirm := true
	review := huh.NewForm(huh.NewGroup(
		huh.NewNote().Title("Will create").Description(strings.Join(lines, "\n")),
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
		if strings.TrimSpace(setupCmd) != "" {
			fmt.Printf("%s running setup (%s) ...\n", dim.Render("…"), setupCmd)
			name, args := splitLaunch(setupCmd)
			if err := spawnQuiet(p.WorktreePath, name, args...); err != nil {
				fmt.Printf("%s setup failed — run \"%s\" manually\n", yellow.Render("!"), setupCmd)
			} else {
				fmt.Printf("%s setup complete (%s)\n", green.Render("✓"), setupCmd)
			}
		}
		// Remember the choice for next time, unless .ccrc.json already pins it
		// (in which case the config file is the source of truth, not the cache).
		if cfg.Setup == "" {
			if err := rememberSetup(repo.Root, setupCmd); err != nil {
				fmt.Printf("%s could not remember setup choice: %v\n", yellow.Render("!"), err)
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
