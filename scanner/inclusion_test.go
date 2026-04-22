package scanner

import (
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
)

func TestRepoInclusionReasons_uncommitted(t *testing.T) {
	t.Parallel()
	g := "f.go"
	rs := RepoStatus{
		Status: git.Status{
			g: &git.FileStatus{Staging: 'M', Worktree: 'M'},
		},
		Porcelain: PorcelainStatus{
			Entries: []PorcelainEntry{{
				Path: g, Staging: 'M', Worktree: 'M',
			}},
		},
		Branches: BranchStatus{Branch: "main", Locations: []BranchLocation{
			{Name: "local", Exists: true, TipHash: "a"},
			{Name: "origin", Exists: true, TipHash: "a"},
		}},
	}
	lines := RepoInclusionReasons(rs)
	if len(lines) < 1 || !strings.Contains(lines[0], "Uncommitted") {
		t.Fatalf("expected uncommitted first line, got %q", lines)
	}
}

func TestRepoInclusionReasons_branchOnly(t *testing.T) {
	t.Parallel()
	rs := RepoStatus{
		Branches: BranchStatus{
			Branch: "main",
			Locations: []BranchLocation{
				{Name: "local", Exists: true, TipHash: "aaa"},
				{Name: "origin", Exists: true, TipHash: "bbb", Incoming: 0, Outgoing: 1},
			},
		},
	}
	lines := RepoInclusionReasons(rs)
	if len(lines) != 1 || !strings.Contains(lines[0], "On remote") {
		t.Fatalf("expected remote tip line, got %q", lines)
	}
}
