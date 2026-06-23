package main

import (
	"fmt"
	"os"

	"command-center/internal/provider"
	"command-center/internal/provider/claude"
	"command-center/internal/tui"
)

// version is overridable at build time:  -ldflags "-X main.version=1.2.3"
var version = "0.1.0"

// appName is the command name shown in help.
const appName = "fleet"

func main() {
	// Register the agent providers. Today only Claude; a second Register call is
	// all it takes to add Codex (FR-20).
	provider.Register(claude.New())

	var cmd string
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "": // no args → the persistent TUI home
		exitOn(tui.Run(tui.Options{}))

	case "hook": // `fleet hook <state>` — the Claude tracker writer (reads stdin)
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: fleet hook <running|finished|inactive>")
			os.Exit(2)
		}
		// A hook must never fail the agent's turn — report to stderr, exit 0.
		if err := claude.HandleHook(os.Args[2], os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, "fleet hook:", err)
		}

	case "--new", "new": // back-compat shim: open the TUI straight into /new
		prefill := ""
		if len(os.Args) > 2 {
			prefill = os.Args[2]
		}
		exitOn(tui.Run(tui.Options{StartWizard: true, PrefillBranch: prefill}))

	case "setup": // re-run onboarding (provider + hook install)
		exitOn(tui.Run(tui.Options{ForceSetup: true}))

	case "install": // CLI equivalent of onboarding's hook install
		exitOn(installDefault())

	case "uninstall": // remove fleet's hooks
		exitOn(uninstallDefault())

	case "--version", "-v":
		fmt.Println(version)

	case "--help", "-h":
		printHelp()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

// installDefault installs the default provider's state integration.
func installDefault() error {
	p, err := defaultProvider()
	if err != nil {
		return err
	}
	if err := p.Install(); err != nil {
		return err
	}
	fmt.Printf("configured %s status tracking (scoped to fleet sessions; swept any legacy global hooks)\n", p.Name())
	return nil
}

func uninstallDefault() error {
	p, err := defaultProvider()
	if err != nil {
		return err
	}
	if err := p.Uninstall(); err != nil {
		return err
	}
	fmt.Printf("removed %s status tracking (scoped settings + any legacy global hooks)\n", p.Name())
	return nil
}

func defaultProvider() (provider.Provider, error) {
	all := provider.All()
	if len(all) == 0 {
		return nil, fmt.Errorf("no providers registered")
	}
	return all[0], nil
}

func exitOn(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf(`
  command center — %[1]s

  A persistent TUI session manager for agent worktrees.

  Usage
    $ %[1]s                 Launch the home dashboard (all sessions, live state)
    $ %[1]s --new [branch]  Jump straight into the new-session wizard
    $ %[1]s setup           Re-run first-run onboarding (provider + hooks)
    $ %[1]s install         Set up status tracking (scoped to fleet sessions)
    $ %[1]s uninstall       Remove status tracking + any legacy global hooks
    $ %[1]s --help
    $ %[1]s --version

  In the TUI, the command bar runs /commands:
    /new [branch]   start the new-session wizard
    /view <id>      open a session's worktree in your IDE
    /open <id>      open the worktree in your IDE
    /rm <id>        remove a worktree + session
    /setup          re-run onboarding
    /quit           exit (sessions keep running in their terminals)

  Per-repo config: drop a .ccrc.json at a repo root to override defaults.
  See .ccrc.example.json. Global config lives in ~/.config/fleet/.

`, appName)
}
