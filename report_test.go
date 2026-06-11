package main

import (
	"testing"

	"github.com/go-git/go-git/v5"

	"github.com/boyvinall/dirtygit/scanner"
)

func TestBuildReportEmpty(t *testing.T) {
	mgs := scanner.NewMultiGitStatus()
	r := buildReport(mgs)
	if len(r.Repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(r.Repos))
	}
}

func TestBuildReportFileEntries(t *testing.T) {
	mgs := scanner.NewMultiGitStatus()
	mgs.AddResult("/repo/a", scanner.RepoStatus{
		Branch: "main",
		Porcelain: scanner.PorcelainStatus{
			Entries: []scanner.PorcelainEntry{
				{Staging: 'M', Worktree: ' ', Path: "foo.go"},
				{Staging: ' ', Worktree: 'D', Path: "bar.go"},
			},
		},
	})

	r := buildReport(mgs)
	if len(r.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(r.Repos))
	}
	repo := r.Repos[0]
	if repo.Path != "/repo/a" {
		t.Errorf("path = %q, want /repo/a", repo.Path)
	}
	if repo.IsClean {
		t.Error("expected IsClean=false for dirty repo")
	}
	if repo.CurrentBranch != "main" {
		t.Errorf("CurrentBranch = %q, want main", repo.CurrentBranch)
	}
	if len(repo.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(repo.Files))
	}
	if repo.Files[0].Staging != string(git.StatusCode('M')) || repo.Files[0].Path != "foo.go" {
		t.Errorf("unexpected file[0]: %+v", repo.Files[0])
	}
	if repo.Files[1].Worktree != string(git.StatusCode('D')) || repo.Files[1].Path != "bar.go" {
		t.Errorf("unexpected file[1]: %+v", repo.Files[1])
	}
}

func TestBuildReportBranchShownInTUI(t *testing.T) {
	local := scanner.BranchLocation{Name: "local", Exists: true, TipHash: "abc"}
	remote := scanner.BranchLocation{Name: "origin", Exists: true, TipHash: "abc"}

	branches := []scanner.LocalBranchRef{
		{Name: "main", Current: true, Locations: []scanner.BranchLocation{local, remote}},
		{Name: "wip", Current: false, Locations: []scanner.BranchLocation{
			{Name: "local", Exists: true, TipHash: "def"},
			{Name: "origin", Exists: false},
		}},
	}
	// FilteredBranches only contains "main" (wip is hidden by config)
	filtered := []scanner.LocalBranchRef{branches[0]}

	mgs := scanner.NewMultiGitStatus()
	mgs.AddResult("/repo/b", scanner.RepoStatus{
		Branch:           "main",
		Branches:         branches,
		FilteredBranches: filtered,
	})

	r := buildReport(mgs)
	if len(r.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(r.Repos))
	}
	if len(r.Repos[0].Branches) != 2 {
		t.Fatalf("expected 2 branch entries, got %d", len(r.Repos[0].Branches))
	}

	byName := make(map[string]reportBranchEntry)
	for _, b := range r.Repos[0].Branches {
		byName[b.Name] = b
	}

	if !byName["main"].ShownInTUI {
		t.Error("main should be ShownInTUI=true (in FilteredBranches)")
	}
	if byName["wip"].ShownInTUI {
		t.Error("wip should be ShownInTUI=false (not in FilteredBranches)")
	}
}

func TestBuildReportIsLocalOnly(t *testing.T) {
	localOnly := scanner.LocalBranchRef{
		Name: "local-feature",
		Locations: []scanner.BranchLocation{
			{Name: "local", Exists: true, TipHash: "111"},
			{Name: "origin", Exists: false},
		},
	}
	withRemote := scanner.LocalBranchRef{
		Name: "main",
		Locations: []scanner.BranchLocation{
			{Name: "local", Exists: true, TipHash: "222"},
			{Name: "origin", Exists: true, TipHash: "222"},
		},
	}

	mgs := scanner.NewMultiGitStatus()
	mgs.AddResult("/repo/c", scanner.RepoStatus{
		Branch:           "main",
		Branches:         []scanner.LocalBranchRef{localOnly, withRemote},
		FilteredBranches: []scanner.LocalBranchRef{localOnly, withRemote},
	})

	r := buildReport(mgs)
	byName := make(map[string]reportBranchEntry)
	for _, b := range r.Repos[0].Branches {
		byName[b.Name] = b
	}

	if !byName["local-feature"].IsLocalOnly {
		t.Error("local-feature should be IsLocalOnly=true")
	}
	if byName["main"].IsLocalOnly {
		t.Error("main should be IsLocalOnly=false")
	}
}

func TestBuildReportSortedPaths(t *testing.T) {
	mgs := scanner.NewMultiGitStatus()
	mgs.AddResult("/z/repo", scanner.RepoStatus{Branch: "main"})
	mgs.AddResult("/a/repo", scanner.RepoStatus{Branch: "main"})
	mgs.AddResult("/m/repo", scanner.RepoStatus{Branch: "main"})

	r := buildReport(mgs)
	if len(r.Repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(r.Repos))
	}
	if r.Repos[0].Path != "/a/repo" || r.Repos[1].Path != "/m/repo" || r.Repos[2].Path != "/z/repo" {
		t.Errorf("repos not sorted: %v", []string{r.Repos[0].Path, r.Repos[1].Path, r.Repos[2].Path})
	}
}

func TestBuildReportDetachedHead(t *testing.T) {
	mgs := scanner.NewMultiGitStatus()
	mgs.AddResult("/repo/d", scanner.RepoStatus{
		Branch:   "abc1234",
		Detached: true,
	})

	r := buildReport(mgs)
	if len(r.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(r.Repos))
	}
	if !r.Repos[0].Detached {
		t.Error("expected Detached=true")
	}
	if r.Repos[0].CurrentBranch != "abc1234" {
		t.Errorf("CurrentBranch = %q, want abc1234", r.Repos[0].CurrentBranch)
	}
}
