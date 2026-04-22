package scanner

import (
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
type LocalBranchRef struct {
	Name    string
	TipHash string
	TipUnix int64
	Current bool
}

type BranchLocation struct {
	Name             string
	Ref              string
	Exists           bool
	TipHash          string
	TipUnix          int64
	UniqueCount      int
	NewestUniqueUnix int64
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
}
