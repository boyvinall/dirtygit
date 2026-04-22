# dirtygit

## What does this do?

- Scans a whole bunch of directories looking for git repos
- Shows you only the ones that seem to be somehow dirty, i.e. one of:
  - It has uncommitted file
  - It has local branches that don't match remote branches

## Why is this useful?

You're busy.  You probably context-swapped a while back and forgot to commit/push a thing.

There's a whole bunch of tools that are very good at managing a single git repo, but I've not
found many that look at the bigger picture.  `dirtygit` helps you to know whether you care
about the things that are only on your local system, so that you can ensure they get pushed
to your git server.

## Source-mode installation

```bash
go install github.com/boyvinall/dirtygit@latest
```

From a clone of this repository:

```bash
go install .
```

## Configuration

Copy [.dirtygit.yml](.dirtygit.yml) to `~/.dirtygit.yml` and edit to your needs.
Environment variables in paths are expanded. Options include:

| Area                           | Purpose                                                                                                         |
| ------------------------------ | --------------------------------------------------------------------------------------------------------------- |
| `scandirs`                     | `include` / `exclude` roots for the walk – YOU SHOULD CONFIGURE AT LEAST THIS                                   |
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

The layout needs a terminal **height of at least 22** rows and **width of at
least 20** columns. While a scan runs, a modal shows how many repositories were
found, how many have been checked, and the path currently being processed.

Focus moves across five panes in order: **Repositories**, **Status**, **Branches**,
**Diff**, and **Log**. **Status** and **Branches** share one row (side by side); **Diff**
sits below them. The mouse is enabled: **click** a pane to focus it, or a row in
**Repositories** / **Status** to move the selection. **Drag** a pane border to resize
splits (unavailable when zoomed, scanning, on error, or with an overlay open). The Status table
lists dirty files with **Worktree** and **Staged** columns (same left-to-right
order as the Diff pane). The Diff pane runs `git diff` with basic colorization;
use **←** / **→** in Status or Diff to switch between **Worktree** and **Staged** views.
With a file row selected, **a** runs `git add` and **r** runs `git reset` (unstage) on
that path (from the Status or Diff pane), then the current repo is refreshed.

With **Repositories** focused, **w** opens a short explanation of why the current repo
is listed, and **D** asks to recursively **delete** that directory on disk (with
confirmation). Both are for housekeeping local clones you no longer need.

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
| `s`                   | Scan or rescan                                                                                                                                                                       |
| `e`                   | Open the selected repo using `edit.command` from config                                                                                                                              |
| `w`                   | With Repositories focused: why this repository is in the list                                                                                                                        |
| `D`                   | With Repositories focused: delete that directory (asks for confirmation)                                                                                                             |
| `q` / `Ctrl+C`        | Quit                                                                                                                                                                                 |
| `?` / `h`             | Show help (`Esc`, `?`, or `h` closes the overlay; `q` / `Ctrl+C` still quit)                                                                                                         |

## Development

```bash
make lint
make test
```

## Future

Ideas that fit the “many repos at a glance” goal; none of this is promised or scheduled.

- **Machine-readable output** — JSON or similar (flag or subcommand) for scripting and CI (e.g. exit non-zero if anything is dirty).
- **Filter / jump in the repo list** — type-ahead or substring match on paths.
- **Copy repo path** — send the selected repository path to the OS clipboard where supported.
- **Richer “why dirty” signals** — stash count, unpushed commits, or upstream ahead/behind in the UI or in the “why listed” overlay.
- **Submodules and worktrees** — scan or label linked worktrees and submodules explicitly instead of treating them only as nested `.git` dirs.
- **Parallel scan** — configurable worker count for status/branch checks, plus clearer cancel behaviour while a scan is running.
- **Configurable diff** — options such as ignore whitespace or word diff, driven from config, for the Diff pane.
- **Safer delete housekeeping** — dry-run delete, or move to Trash on macOS instead of only recursive delete.
