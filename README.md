# dirtygit

## What does this do?

- Walks your directory tree looking for git repositories
- Shows only the ones that are dirty, meaning at least one of:
  - Uncommitted changes in the working tree or index (after any extra ignores from your config)
  - Local branches whose tips don’t match every configured remote (or other branch-pane rules)

## Why is this useful?

You're busy.  You probably context-switched weeks ago and forgot to push something. It happens.

There are plenty of tools for managing a single git repo, but few that give you the bigger
picture. `dirtygit` tells you what’s living only on your machine so you can make sure it
reaches your git server.

## Installation

### Homebrew

On macOS you can install via [homebrew](https://brew.sh/) as follows:

```bash
brew install --cask boyvinall/tap/dirtygit
```

Upgrade with:

```bash
brew upgrade --cask boyvinall/tap/dirtygit
```

### Release

On Linux, Windows, and macOS (without homebrew), you can download a pre-built archive from the
[GitHub releases page](https://github.com/boyvinall/dirtygit/releases). Pick the archive
matching your OS and architecture, extract it, and place the `dirtygit` binary somewhere on
your `PATH`.

On Linux/macOS:

```bash
tar -xzf dirtygit_*.tar.gz
mv dirtygit /usr/local/bin/
```

On Windows, extract the `.tar.gz` and move `dirtygit.exe` to a directory in your `%PATH%`.

### Source

Alternatively, if you have a recent [Go](https://go.dev/) toolchain, you can build/install the latest release
from source into `$GOPATH/bin`:

```bash
go install github.com/boyvinall/dirtygit@latest
```

Or, from a clone of this repository:

```bash
make install
```

## Configuration

If `~/.dirtygit.yml` (or the path given with `--config` / `-c`) does not exist, the binary falls back to an
**embedded default** (the same shape as [.dirtygit.yml](.dirtygit.yml) in this repo). Copy that file to your home
directory and edit it to suit your setup; environment variables in paths are expanded.

Options include:

| Area                           | Purpose                                                                                                         |
| ------------------------------ | --------------------------------------------------------------------------------------------------------------- |
| `scandirs`                     | `include` / `exclude` roots for the walk — **configure this first**                                             |
| `gitignore`                    | Extra `fileglob` / `dirglob` ignores on top of each repo’s `.gitignore`                                         |
| `followsymlinks`               | Whether to descend symlinked directories                                                                        |
| `branches.hidelocalonly.regex` | Regexes (full string match per pattern) for **local-only** branches to omit from the branch pane                |
| `branches.default`             | Short branch names (e.g. `main`) to **always** show when they exist locally, even when every remote tip matches |
| `edit.command`                 | Program and arguments for opening a repo from the UI (`e`); see below                                           |

### Opening a repo (`edit.command`)

`edit.command` is a YAML list of argv pieces passed to `exec` (no shell). Put the
literal `{repo}` in any argument to substitute the absolute repository path. If
no argument contains `{repo}`, the path is appended as the last argument.
Environment variables in each argument are expanded. If `edit.command` is
omitted or empty, the default is `code` with the repo path (VS Code on `PATH`).

## Running

```bash
dirtygit [ <directories...> ]
```

If you pass one or more `<directories>`, they replace `scandirs.include` from
your config for that run (paths are still expanded from the environment).

| Flag             | Meaning                                       |
| ---------------- | --------------------------------------------- |
| `--config`, `-c` | Config file path (default: `~/.dirtygit.yml`) |

![demo](demo.gif)

## UI

The layout requires a terminal at least **22 rows tall** and **20 columns wide**. While a scan
runs, a modal shows how many repositories were found, how many have been checked, and the
path currently being processed.

Focus moves across five panes in order: **Repositories**, **Status**, **Branches**,
**Diff**, and **Log**. **Status** and **Branches** share one row (side by side); **Diff**
sits below them. The mouse is enabled: **click** a pane to focus it, or a row in
**Repositories** / **Status** to move the selection. **Drag** a pane border to resize
splits (unavailable when zoomed, scanning, on error, or with an overlay open). The Status table
lists dirty files with **Worktree** and **Staged** columns (same left-to-right
order as the Diff pane). The Diff pane runs `git diff` with basic colorization;
use **←** / **→** in Status or Diff to switch between **Worktree** and **Staged** views.
With a file row selected, **a** runs `git add` and **r** runs `git reset` (unstage) on
that path (from the Status or Diff pane), then the current repo is refreshed. **C**
asks for confirmation, then runs `git checkout HEAD -- <path>` to restore that path to
the last commit (discards unstaged work for tracked files).

With **Repositories** focused, **w** opens a short explanation of why the current repo
is listed. **D** asks to recursively **delete** either the whole selected repository
directory (repo list, Repositories pane) or the selected file path under the repo
(Status or Diff with a file row selected), each with confirmation.

The **Branches** pane lists local branches that need attention: tips that do not
match every configured remote, missing same-named remote refs, or branches listed
under `branches.default`. Local-only branches can be hidden when they match
`branches.hidelocalonly.regex` (unless they are defaults). The **Remotes** column
compresses each remote into a short status (`ok`, `missing`, `differs`, or
`+N` / `-M` style counts when histories are comparable).

| Key                   | Action                                                                                                                                                                               |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| *Mouse*               | Click to focus a pane; in Repositories or Status (when focused), select a row. Drag a border to resize splits (unavailable when zoomed, scanning, on error, or with an overlay open) |
| `Tab` / `Shift+Tab`   | Next / previous pane; when zoomed, cycle which pane is fullscreen                                                                                                                    |
| `Enter`               | Zoom the focused pane; `Enter` again restores the split layout                                                                                                                       |
| `Esc`                 | Exit zoom, or clear the Status file selection                                                                                                                                        |
| `↑` / `↓`             | Move repo selection, or scroll Status / Diff / Log                                                                                                                                   |
| `Shift+↑` / `Shift+↓` | Same, in steps of 10 lines                                                                                                                                                           |
| `←` / `→`             | In Status or Diff: Worktree vs Staged diff                                                                                                                                           |
| `a` / `r`             | With a status file row selected (Status or Diff): `git add` / `git reset` that path                                                                                                  |
| `C`                   | With a status file row selected (Status or Diff): confirm, then `git checkout HEAD --` that path (restore to last commit)                                                            |
| `s`                   | Scan or rescan                                                                                                                                                                       |
| `e`                   | Open the selected repo using `edit.command` from config                                                                                                                              |
| `t`                   | Open a new terminal for the selected repo (working directory set to that path); parent terminal inferred from `TERM_PROGRAM` when known                                              |
| `w`                   | With Repositories focused: why this repository is in the list                                                                                                                        |
| `D`                   | Repositories: delete that repo directory; Status or Diff with a file row: delete that path under the repo (each confirms)                                                            |
| `q` / `Ctrl+C`        | Quit                                                                                                                                                                                 |
| `?` / `h`             | Show help (`Esc`, `?`, or `h` closes the overlay; `q` / `Ctrl+C` still quit)                                                                                                         |

### Opening a terminal (`t`)

Press **`t`** to spawn a separate terminal whose initial directory is the **currently selected repository**
(absolute path). dirtygit reads **`TERM_PROGRAM`** from the environment and picks a launcher when it recognizes the
value—for example Terminal.app, iTerm, Warp, WezTerm, or kitty on macOS; WezTerm, kitty, or Ghostty on Linux; and common
fallbacks that probe `PATH` on Linux when needed. On **Windows**, **Windows Terminal** (`wt`) is used when available;
otherwise a new `cmd` window runs `cd` into the repo. With **Ghostty on macOS**, dirtygit prefers opening a **new tab**
in the front window (AppleScript), falling back to a new Ghostty window if that fails. If nothing matches, you may see a
log line about no launcher; run dirtygit from a terminal that sets `TERM_PROGRAM`, or rely on the Linux
`PATH` fallback list.

## Acknowledgements

The TUI is built with the awesome [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Development

Run the following to see all available make targets:

```plaintext
make help
```

## Future

A few possibilities listed below, none of this is promised or scheduled.

- **More tool integration** - beyond `e` (edit) and `t` (terminal), maybe a git gui.  And better configurability.
- **Machine-readable output** — JSON or similar (flag or subcommand) for scripting and CI (e.g. exit non-zero if
  anything is dirty).
- **Filter / jump in the repo list** — type-ahead or substring match on paths.
- **Copy repo path** — send the selected repository path to the OS clipboard where supported.
- **Richer “why dirty” signals** — stash count, unpushed commits, or upstream ahead/behind in the UI or in the
  “why listed” overlay.
- **Submodules and worktrees** — scan or label linked worktrees and submodules explicitly instead of treating them
  only as nested `.git` dirs.
- **Parallel scan** — configurable worker count for status/branch checks, plus clearer cancel behaviour while a
  scan is running.
- **Configurable diff** — options such as ignore whitespace or word diff, driven from config, for the Diff pane.
- **Safer delete housekeeping** — dry-run delete, or move to Trash on macOS instead of only recursive delete.
