# dirtygit

Do you find yourself context-switching between a bunch of different git repos?

Have you ever accidentally discovered that changes you've made locally have not
been committed or pushed to your git server?

`dirtygit` is a terminal UI that walks configured directories, finds git
repositories that need attention, and surfaces them in one place. A repo is
listed when it has **uncommitted changes** (after your ignore rules) or when the
**current branch** differs from what your **remotes** expect (missing same-named
remote branch, different tips, or unpushed / unpulled commits summarized per
remote).

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
| `scandirs`                     | `include` / `exclude` roots for the walk                                                                        |
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

| Flag             | Meaning                                            |
| ---------------- | -------------------------------------------------- |
| `--config`, `-c` | Config file path (default: `~/.dirtygit.yml`)      |
| `--debug`        | Skip the UI; print per-repo scan timings to stdout |

![demo](demo.gif)

## UI

The layout needs a terminal **height of at least 22** rows and **width of at
least 20** columns. While a scan runs, a modal shows how many repositories were
found, how many have been checked, and the path currently being processed.

Focus moves across five panes in order: **Repositories**, **Status**, **Branches**,
**Diff**, and **Log**. **Status** and **Branches** share one row (side by side); **Diff**
sits below them. The Status table lists dirty files with **Worktree** and **Staged**
columns (same left-to-right order as the Diff pane). The Diff pane runs `git
diff` with basic colorization; use **←** / **→** in Status or Diff to switch between
**Worktree** and **Staged** views. With a file row selected, **a** runs `git add` and **r**
runs `git reset` (unstage) on that path (from the Status or Diff pane), then the
current repo is refreshed.

The **Branches** pane lists local branches that need attention: tips that do not
match every configured remote, missing same-named remote refs, or branches listed
under `branches.default`. Local-only branches can be hidden when they match
`branches.hidelocalonly.regex` (unless they are defaults). The **Remotes** column
compresses each remote into a short status (`ok`, `missing`, `differs`, or
`+N` / `-M` style counts when histories are comparable).

| Key                 | Action                                                                              |
| ------------------- | ----------------------------------------------------------------------------------- |
| `Tab` / `Shift+Tab` | Next / previous pane (when zoomed, cycles which pane is fullscreen)                 |
| `Enter`             | Zoom the focused pane; `Enter` again restores the split layout                      |
| `Esc`               | Exit zoom, or clear the Status file selection                                       |
| `↑` / `↓`           | Move repo selection, or scroll Status / Diff / Log                                  |
| `←` / `→`           | In Status or Diff: Worktree vs Staged diff                                          |
| `a` / `r`           | With a status file row selected (Status or Diff): `git add` / `git reset` that path |
| `s`                 | Scan or rescan                                                                      |
| `e`                 | Open the selected repo using `edit.command` from config                             |
| `q` / `Ctrl+C`      | Quit                                                                                |
| `?` / `h`           | Toggle help (same keys plus `Esc` close the overlay)                                |

## Development

```bash
make lint
make test
```

## Future

- Clearer error logging and in-app error presentation
- Optional destructive actions (e.g. discard changes) with strong safeguards
