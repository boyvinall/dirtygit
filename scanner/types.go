package scanner

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
)

type RepoStatus struct {
	git.Status

	Porcelain PorcelainStatus
	Branches  BranchStatus
	ScanTime  time.Duration
}

type MultiGitStatus map[string]RepoStatus

type BranchStatus struct {
	Branch         string
	Detached       bool
	Locations      []BranchLocation
	NewestLocation string
	// LocalBranches lists refs/heads in name order (from git for-each-ref).
	LocalBranches []LocalBranchRef
}

// LocalBranchRef is one local branch tip (refs/heads/*).
// Locations holds local vs same-named remote refs (refs/remotes/<remote>/<name>);
// it is empty when detached or before GitBranchStatus fills it.
type LocalBranchRef struct {
	Name      string
	TipHash   string
	TipUnix   int64
	Current   bool
	Locations []BranchLocation
}

// HasTipMismatchAcrossRemotes reports whether the branch should appear in the
// branch pane: true when Locations is empty (e.g. detached), there are no
// configured remotes, any same-named remote ref is missing, or any remote tip
// differs from the local tip. False only when every remote has the ref and each
// tip matches local.
func (lb LocalBranchRef) HasTipMismatchAcrossRemotes() bool {
	if len(lb.Locations) == 0 {
		return true
	}
	var local *BranchLocation
	for i := range lb.Locations {
		if lb.Locations[i].Name == "local" {
			local = &lb.Locations[i]
			break
		}
	}
	if local == nil || !local.Exists {
		return true
	}
	hasRemote := false
	for i := range lb.Locations {
		loc := lb.Locations[i]
		if loc.Name == "local" {
			continue
		}
		hasRemote = true
		if !loc.Exists || loc.TipHash != local.TipHash {
			return true
		}
	}
	return !hasRemote
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

type BranchLocation struct {
	Name             string
	Ref              string
	Exists           bool
	TipHash          string
	TipUnix          int64
	UniqueCount      int
	NewestUniqueUnix int64
	// Incoming/Outgoing compare this ref to the local branch ref only (remote
	// rows). Incoming is commits reachable from this remote but not local (+N);
	// Outgoing is commits on local not reachable from this remote (UI: -M).
	Incoming int
	Outgoing int
	// HistoriesUnrelated means git found no merge base between local and this
	// remote tip; the UI shows "differs" instead of numeric deltas.
	HistoriesUnrelated bool
}

// HasLocalRemoteMismatch reports whether the current local branch differs from
// any tracked remote location for the same branch name.
func (b BranchStatus) HasLocalRemoteMismatch() bool {
	if b.Detached {
		return false
	}

	var local *BranchLocation
	for i := range b.Locations {
		if b.Locations[i].Name == "local" {
			local = &b.Locations[i]
			break
		}
	}
	if local == nil || !local.Exists {
		return false
	}

	hasRemote := false
	for i := range b.Locations {
		loc := b.Locations[i]
		if loc.Name == "local" {
			continue
		}
		hasRemote = true
		if !loc.Exists {
			return true
		}
		if loc.TipHash != local.TipHash {
			return true
		}
	}

	return hasRemote && local.UniqueCount > 0
}

type PorcelainEntry struct {
	Staging      git.StatusCode
	Worktree     git.StatusCode
	Path         string
	OriginalPath string
}

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

// ScanProgress reports coarse scan activity for UIs (discovery vs git status).
// ReposFound may increase while ReposChecked catches up; both match the final total when complete.
type ScanProgress struct {
	ReposFound   int
	ReposChecked int
	CurrentPath  string
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
	branchDefaultAlways   map[string]struct{}
}

// ShouldHideLocalOnlyBranch returns true when lb is local-only (see
// LocalBranchRef.IsLocalOnly) and its short branch name matches any pattern in
// branches.hidelocalonly.regex.
func (c *Config) ShouldHideLocalOnlyBranch(lb LocalBranchRef) bool {
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

// AlwaysListBranch reports whether name is listed under branches.default
// (after trim); those branches are listed in the pane whenever they exist
// locally, regardless of remote tip agreement.
func (c *Config) AlwaysListBranch(name string) bool {
	if len(c.branchDefaultAlways) == 0 {
		return false
	}
	_, ok := c.branchDefaultAlways[name]
	return ok
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
