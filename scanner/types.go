package scanner

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
)

// RepoStatus aggregates one repository's working tree and branch metadata:
// parsed git status --porcelain (Porcelain), HEAD and local-branch layout with
// remote tips (Branches), and an embedded [git.Status] rebuilt from Porcelain
// for code that expects go-git's map form.
type RepoStatus struct {
	git.Status

	Porcelain        PorcelainStatus
	Branches         BranchStatus
	FilteredBranches []LocalBranchRef
}

// BranchStatus is HEAD identity plus an ordered list of local branch refs and
// per-branch local vs same-named remote comparison rows.
type BranchStatus struct {
	// Branch is the checked-out branch short name, or when Detached the short
	// HEAD object name (see git rev-parse --short HEAD).
	Branch string
	// Detached is true when HEAD is not on a branch (detached HEAD).
	Detached bool
	// LocalBranches lists refs/heads in name order (from git for-each-ref).
	LocalBranches []LocalBranchRef
}

// LocalBranchRef is one local branch tip (refs/heads/*).
// Locations holds local vs same-named remote refs (refs/remotes/<remote>/<name>);
// it is empty when detached or before GitBranchStatus fills it.
type LocalBranchRef struct {
	// Name is the short branch name (the refs/heads/* ref without the prefix).
	Name string
	// TipHash is the full object name of the local ref tip; TipUnix is the tip
	// commit's committer date in Unix seconds (from branch listing / git show).
	TipHash string
	TipUnix int64
	// Current is true when this row is the checked-out branch.
	Current bool
	// Locations compares this local branch to same-named refs on each configured
	// remote; see [BranchLocation]. Empty when detached or before branch scan fills it.
	Locations []BranchLocation
}

func (lbr *LocalBranchRef) GetDisplayName() string {
	if lbr.Current {
		return "*" + lbr.Name
	}
	return lbr.Name
}

// BranchLocation is one side of a local branch compared to same-named remotes:
// either the local ref (Name "local") or a configured remote's
// refs/remotes/<Name>/<branch>. Populated by branch status scanning; stored in
// [LocalBranchRef.Locations]. The UI and helpers use it for tip hashes,
// ahead/behind counts (Incoming/Outgoing vs local), and mismatch detection.
type BranchLocation struct {
	// Either "local" or the name of a configured remote.
	Name string

	// Exists is true when this location's ref (refs/heads/<branch> for "local",
	// refs/remotes/<remote>/<branch> otherwise) exists and resolves to a commit;
	// false when the ref is missing.
	Exists bool

	// TipHash is the full hex object name of this ref's tip commit when Exists;
	// empty when not Exists.
	TipHash string
	// TipUnix is the tip commit's committer date in Unix seconds (from git show);
	// zero when not Exists.
	TipUnix int64
	// UniqueCount is commits reachable from this ref but not from any other
	// Exists location for the same branch (local plus each remote in the scan).
	// Zero when not Exists.
	UniqueCount int
	// Incoming/Outgoing compare this ref to the local branch ref only (remote
	// rows). Incoming is commits reachable from this remote but not local (+N);
	// Outgoing is commits on local not reachable from this remote (UI: -M).
	Incoming int
	Outgoing int
	// HistoriesUnrelated means git found no merge base between local and this
	// remote tip; the UI shows "differs" instead of numeric deltas.
	HistoriesUnrelated bool
}

// IsLocalOnly reports whether no configured remote has a same-named branch ref
// (refs/remotes/<remote>/<name> missing for every remote). Repositories with no
// remotes still populate only the local slot, which counts as local-only here.
// Empty Locations (e.g. some detached listings) yields false so branches are
// not classified without remote comparison data.
func (lb LocalBranchRef) IsLocalOnly() bool {
	if len(lb.Locations) == 0 {
		return false
	}
	for _, loc := range lb.Locations {
		if loc.Name == "local" {
			continue
		}
		if loc.Exists {
			return false
		}
	}
	return true
}

// CurrentBranchLocations returns local vs same-named remote rows for the
// checked-out branch (the [LocalBranchRef] with Current: true). Returns nil when
// detached or when no current row exists.
func (b *BranchStatus) CurrentBranchLocations() []BranchLocation {
	if b == nil || b.Detached {
		return nil
	}
	for i := range b.LocalBranches {
		if b.LocalBranches[i].Current {
			return b.LocalBranches[i].Locations
		}
	}
	return nil
}

func (b *BranchStatus) HasUnpushedChanges(c *Config) bool {
	for _, lb := range b.LocalBranches {
		if c.ShouldHideLocalOnlyBranch(lb) {
			continue
		}
		if lb.HasUnpushedChanges() {
			return true
		}
	}
	return false
}

// HasUnpushedChanges reports whether the local branch has any commits not on any of the remotes,
func (lb *LocalBranchRef) HasUnpushedChanges() bool {
	locs := lb.Locations
	if len(locs) == 0 {
		return false
	}

	var local *BranchLocation
	for i := range locs {
		if locs[i].Name == "local" {
			local = &locs[i]
			break
		}
	}
	if local == nil || !local.Exists {
		return false
	}

	hasRemote := false
	for i := range locs {
		loc := locs[i]
		if loc.Name == "local" {
			continue
		}
		hasRemote = true
		if !loc.Exists {
			return true
		}
		if loc.TipHash != local.TipHash {
			if !loc.HistoriesUnrelated && loc.Incoming > 0 && loc.Outgoing == 0 {
				continue
			}
			return true
		}
	}

	if hasRemote && local.UniqueCount > 0 {
		return true
	}
	return false
}

// Filter filters out local-only branches that
// [Config.ShouldHideLocalOnlyBranch] matches.
// The checked-out branch is never removed so HEAD remote comparison stays available.
func (b *BranchStatus) Filter(c *Config) []LocalBranchRef {
	out := make([]LocalBranchRef, 0)
	for _, lb := range b.LocalBranches {
		if lb.Current {
			out = append(out, lb)
			continue
		}
		if !lb.HasUnpushedChanges() {
			continue
		}
		if c.ShouldHideLocalOnlyBranch(lb) {
			continue
		}
		out = append(out, lb)
	}
	return out
}

// LocalRemoteMismatchReasons returns a short line explaining why
// [BranchStatus.HasLocalRemoteMismatch] is true, or nil when that predicate is false.
func (b *BranchStatus) LocalRemoteMismatchReasons() []string {
	if b.Detached {
		return nil
	}
	locs := b.CurrentBranchLocations()
	if len(locs) == 0 {
		return nil
	}
	var local *BranchLocation
	for i := range locs {
		if locs[i].Name == "local" {
			local = &locs[i]
			break
		}
	}
	if local == nil || !local.Exists {
		return nil
	}
	branchName := b.Branch
	if branchName == "" {
		branchName = "current branch"
	}
	hasRemote := false
	for i := range locs {
		loc := locs[i]
		if loc.Name == "local" {
			continue
		}
		hasRemote = true
		if !loc.Exists {
			return []string{fmt.Sprintf("On remote %q, there is no same-named branch to compare with your local %q (refs/remotes/…/… missing).", loc.Name, branchName)}
		}
		if loc.TipHash != local.TipHash {
			if !loc.HistoriesUnrelated && loc.Incoming > 0 && loc.Outgoing == 0 {
				continue
			}
			if loc.HistoriesUnrelated {
				return []string{fmt.Sprintf("On remote %q, %q has unrelated history to your local tip (no merge base).", loc.Name, branchName)}
			}
			if loc.Incoming > 0 && loc.Outgoing > 0 {
				return []string{fmt.Sprintf("On remote %q, %q diverged from the local tip (incoming +%d, outgoing %d).", loc.Name, branchName, loc.Incoming, loc.Outgoing)}
			}
			if loc.Outgoing > 0 {
				return []string{fmt.Sprintf("On remote %q, your local %q is ahead: %d commit(s) not on that remote (tips differ or unpushed).", loc.Name, branchName, loc.Outgoing)}
			}
			if loc.Incoming > 0 {
				return []string{fmt.Sprintf("On remote %q, the same-named ref tip differs from your local %q (and it is not the \"only behind the remote\" case).", loc.Name, branchName)}
			}
			return []string{fmt.Sprintf("On remote %q, the same-named ref tip differs from your local branch %q (not the \"only behind the remote\" case).", loc.Name, branchName)}
		}
	}
	if hasRemote && local.UniqueCount > 0 {
		return []string{fmt.Sprintf("Branch %q: %d commit(s) on local are not on any of the other refs this scan compared (e.g. same-named remotes) — see the Branches pane for detail.", branchName, local.UniqueCount)}
	}
	return nil
}

// PorcelainEntry is one parsed line of git status --porcelain (short format).
type PorcelainEntry struct {
	// Staging and Worktree are the two status columns (index vs working tree).
	Staging  git.StatusCode
	Worktree git.StatusCode
	// Path is the file path, or the new path for a rename.
	Path string
	// OriginalPath is the old path for a rename; empty when not a rename.
	OriginalPath string
}

// PorcelainStatus is the full porcelain parse for one repo (entry order matches git output).
type PorcelainStatus struct {
	Entries []PorcelainEntry
}

func (p PorcelainStatus) ToGitStatus() git.Status {
	st := make(git.Status, len(p.Entries))
	for _, entry := range p.Entries {
		st[entry.Path] = &git.FileStatus{
			Staging:  entry.Staging,
			Worktree: entry.Worktree,
		}
	}
	return st
}

// ScanProgress reports coarse scan activity for UIs (discovery vs per-repo work).
// ReposFound may increase while ReposChecked catches up; both match the final total when complete.
type ScanProgress struct {
	// ReposFound is how many git repositories have been discovered so far.
	ReposFound int
	// ReposChecked is how many of those have finished StatusForRepo (porcelain,
	// branch metadata, and config filtering), not git status alone.
	ReposChecked int
	// CurrentPath is the path currently being processed (for status display).
	CurrentPath string
}

type Config struct {
	ScanDirs struct {
		Include []string `yaml:"include"`
		Exclude []string `yaml:"exclude"`
	} `yaml:"scandirs"`
	GitIgnore struct {
		FileGlob []string `yaml:"fileglob"`
		DirGlob  []string `yaml:"dirglob"`
	} `yaml:"gitignore"`
	FollowSymlinks bool `yaml:"followsymlinks"`
	Branches       struct {
		HideLocalOnly struct {
			Regex []string `yaml:"regex"`
		} `yaml:"hidelocalonly"`
		// Default lists branch short names always shown in the branch pane when
		// present as a local ref, even when tips match every remote.
		Default []string `yaml:"default"`
	} `yaml:"branches"`
	// Edit holds argv for opening a repository from the UI (key "e").
	Edit struct {
		// Command is the program and arguments passed to exec (no shell).
		// Use the literal substring "{repo}" in any element to substitute the
		// absolute repository path. If no element contains "{repo}", the path
		// is appended as the final argument. Empty means ["code", <repo>].
		Command []string `yaml:"command"`
	} `yaml:"edit"`
	localOnlyHideCompiled []*regexp.Regexp
}

// ShouldHideLocalOnlyBranch returns true when lb is local-only (see
// LocalBranchRef.IsLocalOnly) and its short branch name matches any pattern in
// branches.hidelocalonly.regex.
func (c *Config) ShouldHideLocalOnlyBranch(lb LocalBranchRef) bool {
	if c == nil {
		return false
	}
	if len(c.localOnlyHideCompiled) == 0 {
		return false
	}
	if !lb.IsLocalOnly() {
		return false
	}
	for _, re := range c.localOnlyHideCompiled {
		if re.MatchString(lb.Name) {
			return true
		}
	}
	return false
}

const editRepoPlaceholder = "{repo}"

// EditArgv returns the argv for opening absRepo in an external editor or IDE.
// See Config.Edit.Command for placeholder and default behavior.
func (c *Config) EditArgv(absRepo string) ([]string, error) {
	raw := c.Edit.Command
	if len(raw) == 0 {
		return []string{"code", absRepo}, nil
	}
	out := make([]string, 0, len(raw)+1)
	hasPlaceholder := false
	for _, a := range raw {
		a = os.ExpandEnv(a)
		if strings.Contains(a, editRepoPlaceholder) {
			hasPlaceholder = true
			out = append(out, strings.ReplaceAll(a, editRepoPlaceholder, absRepo))
			continue
		}
		out = append(out, a)
	}
	if !hasPlaceholder {
		out = append(out, absRepo)
	}
	if len(out) == 0 || strings.TrimSpace(out[0]) == "" {
		return nil, fmt.Errorf("edit.command: program name is empty")
	}
	return out, nil
}
