package main

import (
	"fmt"
	"os"
)

// version is overridable at build time:  -ldflags "-X main.version=1.2.3"
var version = "0.1.0"

// appName is the command name shown in help. Change here if you rename the
// tool (also update CMD_NAME in install.sh / install.ps1).
const appName = "fleet"

func main() {
	var cmd string
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "--new", "new":
		if err := runNew(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "--version", "-v":
		fmt.Println(version)
	case "", "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf(`
  command center — %[1]s

  Usage
    $ %[1]s --new        Create an isolated git worktree + branch, then launch an agent
    $ %[1]s --help
    $ %[1]s --version

  %[1]s --new walks you through:
    1. Branch name   (e.g. task/SP-1234-login-fix)
    2. Base branch   (main / develop)

  ...then creates that branch in a sibling worktree folder, copies your
  gitignored env files, installs dependencies, and drops you into claude.

  Per-repo config: drop a .ccrc.json at a repo root to override defaults
  (baseBranches, worktreeName, copyFiles, install, launch, ...).
  See .ccrc.example.json.

`, appName)
}
