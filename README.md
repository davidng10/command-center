# command center — `fleet`

A persistent TUI for running a **fleet of parallel coding agents**, each in its
own isolated git worktree. `fleet` is a long-running program (like Claude Code
itself): a home dashboard of every agent session and its **live status**, with a
`/new` wizard to spin up more.

Supports:

1. [Claude Code](https://claude.com/claude-code) (more providers planned — the
   core is provider-agnostic)

```
▌ fleet  ~/Documents/gitlab/platform-server

● Active sessions (3)                                    1 running

▌  1 ● task/SP-12392-build-navigation-bar               Running
        main · 4m · working…
   2 ● fix/SP-12222-fix-bug-in-home-page                Finished
        main · 1m · idle 1m ago · ready to review
   3 ● chore/SP-12001-bump-deps                         Inactive
        main · 2h · ended (terminal closed)

› Type a command…  try /new
─────────────────────────────────────────────────────────────────
Viewing active sessions · task/SP-12392…   ↑↓ navigate  enter view  o open IDE  x remove  / command  Esc exit
```

Type `/new`, answer the wizard (name → directory → base branch → setup →
review), and `fleet` creates the worktree and launches the agent **in a new
terminal window**. `fleet` keeps your terminal and tracks each agent's state —
Running / Finished / Inactive — live.

---

## What's a worktree

Most devs work on one repo and one branch at a time. Worktrees let one repo have
several folders checked out to different branches at once, all sharing the same
`.git` history. Agent A edits folder A on its branch; agent B edits folder B on
its branch. They never step on each other. The `/new` wizard automates creating
one of those folders + a branch you name yourself, however your team names
branches.

---

## Install

```bash
# macOS / Linux / WSL
./install.sh
#   → installs to ~/.local/bin/fleet   (override: BIN_DIR=/usr/local/bin ./install.sh)
```

```powershell
# native Windows (PowerShell). WSL users: use ./install.sh instead.
.\install.ps1
```

If `~/.local/bin` isn't on your PATH, the script tells you the line to add.

On first launch `fleet` runs a one-time **onboarding**: just pick your provider.
Status tracking is automatic and needs no setup — `fleet` injects its status
hooks **only into the sessions it launches** (via `claude --settings`), so your
global `~/.claude/settings.json` is never modified and unrelated Claude Code
sessions never see them. (Upgrading from an older `fleet`? It removes the global
hooks a previous version installed, automatically, on next launch.)

### Requirements at runtime

- **git** (any modern version)
- **claude** on PATH — only if you keep the default `launch: "claude"`
- a terminal `fleet` can spawn agents into (per-OS default; override with
  `terminal` in `~/.config/fleet/config.json`)

---

## Build from source (for maintainers)

Requires **Go 1.22+**.

```bash
go build -o fleet .       # build for your machine
./build.sh                # cross-compile ALL platforms into dist/
go test ./...             # run the test suite
```

`build.sh` produces binaries for darwin/linux/windows × amd64/arm64. To cut a
release, run `./build.sh v1.2.3` and upload `dist/*` to a GitHub Release.

---

## Usage

```bash
fleet              # launch the home dashboard (all sessions, live state)
fleet --new [b]    # jump straight into the new-session wizard (optional branch prefill)
fleet setup        # re-run first-run onboarding (provider)
fleet install      # (re)write the scoped status hooks + sweep any legacy global hooks
fleet uninstall    # remove the scoped status hooks + any legacy global hooks
fleet --help
fleet --version
```

In the TUI the command bar (press `/`) runs `/commands`:

| Command         | Hotkey      | Action                                       |
| --------------- | ----------- | -------------------------------------------- |
| `/new [branch]` | `/` then type | start the new-session wizard               |
| `/view <id>`    | `enter`     | open the session's worktree in your IDE      |
| `/open <id>`    | `o`         | open the worktree in your IDE                |
| `/rm <id>`      | `x`         | remove the worktree + session (with confirm) |
| `/setup`        | —           | re-run onboarding                            |
| `/quit`         | `Esc`       | exit (sessions keep running in their terminals) |

`id` defaults to the selected row when omitted.

---

## Configuration

Two layers, neither of which ever lands in your repo:

**Per-repo** — drop a `.ccrc.json` at a repo's root (copy
[`.ccrc.example.json`](.ccrc.example.json)):

| key            | default                       | meaning                                                       |
| -------------- | ----------------------------- | ------------------------------------------------------------- |
| `baseBranches` | `["main", "develop"]`         | branches offered to fork from                                 |
| `defaultBase`  | `main`                        | pre-selected base                                             |
| `worktreeName` | `{repo}-{branch}`             | sibling folder name. Tokens: `{repo}`, `{branch}` (slugified) |
| `copyFiles`    | `[".env", ".env.local", ...]` | gitignored files copied into the worktree                     |
| `install`      | `true`                        | whether to offer a setup step in the wizard                   |
| `setup`        | `""`                          | explicit setup command; overrides auto-detection when set     |
| `launch`       | `claude`                      | (legacy) the agent command; providers now define their launch |
| `fetch`        | `true`                        | `git fetch` the base before forking so it's fresh             |

**Global** — `~/.config/fleet/` (honoring `XDG_CONFIG_HOME`), all owned by fleet:

```
~/.config/fleet/
├── config.json              # setupComplete, defaultProvider, ide, terminal
├── sessions.json            # the session registry (persisted, survives restarts)
├── prefs.json               # "last used" cache: recent dirs, per-repo base & setup
├── claude/settings.json     # scoped status hooks, injected via `claude --settings`
└── state/<session>.json     # transient per-session activity, written by `fleet hook`
```

`config.json` keys: `ide` (e.g. `"code"`, used by `/open` and `/view`) and
`terminal` (the command fleet spawns agents into; `""` = a per-OS default —
macOS `osascript`, Linux `$TERMINAL`/`x-terminal-emulator`, Windows `wt`). A
custom `terminal` template may use `{dir}` and `{cmd}` placeholders.

**Setup step.** A fresh worktree shares `.git` but not gitignored, per-folder
state like `node_modules`, so JS projects need dependencies installed before the
worktree is usable. The wizard's setup picker pre-selects in order:

1. an explicit `setup` in `.ccrc.json` (always wins), then
2. **your last choice for this repo** (from `prefs.json`), then
3. auto-detection from the lockfile (`pnpm install`, `npm ci`, … — JS only).

Your choice (including *Skip*) is remembered per repo. A legacy
`~/.config/fleet/setups.json` from older versions is migrated into `prefs.json`
automatically on first run.

---

## How live status works

fleet writes 3 status hooks into its own `~/.config/fleet/claude/settings.json`
(each invoking the fleet binary itself — no bash/jq, cross-platform) and launches
the agent with `claude --settings ~/.config/fleet/claude/settings.json`. The hooks
therefore fire **only for the sessions fleet launches** — never for unrelated
Claude Code sessions, and never by modifying your global `~/.claude/settings.json`:

| Claude hook event  | `fleet hook …`  | state          |
| ------------------ | --------------- | -------------- |
| `UserPromptSubmit` | `running`       | **Running**    |
| `Stop`             | `finished`      | **Finished**   |
| `SessionEnd`       | `inactive`      | **Inactive**   |

`fleet hook <state>` reads the hook payload on stdin, maps `cwd` → worktree →
session (resolving symlinks, so a hook's canonical cwd still matches), and
atomically writes `~/.config/fleet/state/<session_id>.json`. The TUI watches that
directory and updates the matching row. A hard kill (closing the terminal) fires
no `SessionEnd` hook; fleet falls back to process liveness where a PID is
available.

The hook command is rewritten on every launch with the current binary path, so a
rebuilt or relocated `fleet` can never leave a dead command behind. Earlier
versions installed these hooks globally into `~/.claude/settings.json`; fleet now
sweeps those out automatically on launch (or via `fleet uninstall`).

---

## Project layout

```
command-center/
├── main.go                     # arg routing: TUI · hook · install/uninstall · setup · --new shim
├── internal/
│   ├── tui/                     # bubbletea models: app shell, home, wizard, onboarding
│   ├── session/                 # Session, canonical State, persisted Registry, liveness
│   ├── provider/                # Provider + StateTracker seam; claude/ adapter (hooks, tracker)
│   ├── worktree/                # git worktree ops + branch/plan/util/pkg helpers
│   ├── fsbrowse/                # directory browser + repo detection + branch listing
│   ├── term/                    # spawn an agent in a new terminal (configurable + per-OS)
│   └── config/                  # .ccrc.json + global config.json + prefs.json
├── design/                      # DESIGN.md spec + mockup.html UI reference
├── build.sh · install.sh · install.ps1 · .ccrc.example.json
```

Built on [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) +
[bubbles](https://github.com/charmbracelet/bubbles) +
[lipgloss](https://github.com/charmbracelet/lipgloss). The repo/module is named
`command-center`; the installed command is `fleet`.

---

## Extending it: adding a provider

The core never names Claude. A new agent backend (e.g. Codex) implements the
`provider.Provider` seam — `LaunchSpec`, `Install`/`Uninstall`, `Tracker` — and
calls `provider.Register(...)` in `main.go`. No changes to the shell, registry,
or state model. A provider with no activity integration still gets
Active/Inactive for free via process liveness. See
[`design/DESIGN.md`](design/DESIGN.md) §7 and §14.
