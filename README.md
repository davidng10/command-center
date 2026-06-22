# command center — `fleet`

Agent parallelization management tool in CLI to spin up **isolated worktrees and run a fleet of
parallel agents**

Supports:

1. [Claude Code](https://claude.com/claude-code)

```
$ fleet --new

┃ Branch name?
┃ task/SP-1234-login-fix
┃ Base branch?
┃ > main
┃   develop

┃ Will create
┃ branch  task/SP-1234-login-fix
┃ base    origin/main
┃ folder  ~/Documents/gitlab/product-catalog-task-sp-1234-login-fix
┃ Create this worktree? (Y/n)

✓ worktree at ~/Documents/product-catalog-task-sp-1234-login-fix
✓ copied: .env, .env.local
✓ dependencies installed (pnpm)

Ready. launching claude in product-catalog-task-sp-1234-login-fix …
```

It then drops you into `claude` inside the new worktree. Open a second
terminal, run `fleet --new` for another ticket, and you've got two agents
working in parallel with zero file collisions.

---

## What's a worktree

Most devs work on one repo and on one branch at a time. Worktrees lets one repo have
several folders checked out to different branches at once, all sharing the same
`.git` history. Agent A edits folder A on its branch; agent B edits folder B on
its branch. They never step on each other. `fleet --new` automates creating one
of those folders + a branch you name yourself, however your team names branches.

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

> Once you publish to GitHub Releases, teammates can skip cloning:
> `RELEASE_BASE_URL=https://github.com/<org>/command-center/releases/latest/download ./install.sh`

### Requirements at runtime

- **git** (any modern version)
- **claude** on PATH — only if you keep the default `launch: "claude"`

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
fleet --new        # interactive: branch name → base branch → create + launch
fleet --help
fleet --version
```

Run it from **inside the repo you want a worktree of**. `fleet` finds the repo
root, creates the worktree as a sibling folder, copies your gitignored env
files, installs dependencies, and launches your agent.

---

## Configuration

Defaults live in [`config.go`](config.go). Override them **per repo** by
dropping a `.ccrc.json` at that repo's root (copy
[`.ccrc.example.json`](.ccrc.example.json)):

| key            | default                       | meaning                                                       |
| -------------- | ----------------------------- | ------------------------------------------------------------- |
| `baseBranches` | `["main", "develop"]`         | branches offered to fork from                                 |
| `defaultBase`  | `main`                        | pre-selected base                                             |
| `worktreeName` | `{repo}-{branch}`             | sibling folder name. Tokens: `{repo}`, `{branch}` (slugified) |
| `copyFiles`    | `[".env", ".env.local", ...]` | gitignored files copied into the worktree                     |
| `install`      | `true`                        | run the package manager after creating the worktree           |
| `launch`       | `claude`                      | command run in the worktree when done (`""` to skip)          |
| `fetch`        | `true`                        | `git fetch` the base before forking so it's fresh             |

You name the branch yourself, however your team names branches — the only
normalization is that surrounding/internal whitespace collapses to dashes
(`login fix` → `login-fix`). The worktree folder name is the slugified branch
(`task/SP-1234-login-fix` → `product-catalog-task-sp-1234-login-fix`).
Package manager is auto-detected from the lockfile (pnpm/yarn/bun/npm).

---

## Cleanup when a ticket is done

```bash
git worktree list                              # see all worktrees
git worktree remove ../product-catalog-task-sp-1234-login-fix   # delete the folder
git branch -d task/SP-1234-login-fix                            # (optional) drop the local branch
```

---

## Project layout

```
command-center/
├── main.go            # entrypoint + arg routing + --help/--version
├── newcmd.go          # the `fleet --new` flow (huh forms + git steps)
├── config.go          # defaults + .ccrc.json loading
├── git.go             # git repo detection + exec helpers
├── pkg.go             # package-manager detection
├── util.go            # slugify / branch-sanitize / template helpers
├── *_test.go          # unit + end-to-end worktree tests
├── build.sh           # cross-compile every platform → dist/
├── install.sh         # macOS/Linux/WSL installer
├── install.ps1        # Windows installer
└── .ccrc.example.json
```

Uses [charmbracelet/huh](https://github.com/charmbracelet/huh) for the
interactive prompts. The repo/module is named `command-center`; the installed
command is `fleet`.

---

## Extending it

E.g. To add `fleet --list`:

1. Write `runList()` in a new `listcmd.go`.
2. Add a `case "--list", "list":` in `main.go`.
3. Document it in `printHelp()`.

Natural next commands: `--list`, `--rm` (remove a worktree), `--open`, and a
conventional-commit helper.
