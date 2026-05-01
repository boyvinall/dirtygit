package scanner

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestScanFindsDirtyRepos(t *testing.T) {
	root := t.TempDir()
	dirtyA := filepath.Join(root, "dirty-a")
	dirtyB := filepath.Join(root, "dirty-b")

	for _, dir := range []string{dirtyA, dirtyB} {
		gitMinimalInit(t, dir)
		gitCommitFile(t, dir, "README.md", "init\n", "init")
	}
	if err := os.WriteFile(filepath.Join(dirtyA, "untracked.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirtyB, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}
	cfg.ScanDirs.Include = []string{root}

	mgs, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if mgs.Len() != 2 {
		t.Fatalf("Scan() len = %d, want 2, keys=%v", mgs.Len(), mgs.SortedRepoPaths())
	}
	if _, ok := mgs.Get(dirtyA); !ok {
		t.Fatalf("missing dirty repo a: %v", mgs.SortedRepoPaths())
	}
	if _, ok := mgs.Get(dirtyB); !ok {
		t.Fatalf("missing dirty repo b: %v", mgs.SortedRepoPaths())
	}
}

func TestScanEmptyTree(t *testing.T) {
	root := t.TempDir()
	cfg := &Config{}
	cfg.ScanDirs.Include = []string{root}

	mgs, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if mgs.Len() != 0 {
		t.Fatalf("want no repos, got %d", mgs.Len())
	}
}

func TestScanWithProgressReportsProgress(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "r1")
	gitMinimalInit(t, repo)
	gitCommitFile(t, repo, "f.txt", "v1\n", "c1")
	if err := os.WriteFile(filepath.Join(repo, "extra.log"), []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}
	cfg.ScanDirs.Include = []string{root}

	var mu sync.Mutex
	var last ScanProgress
	var maxFound, maxChecked int
	callback := func(p ScanProgress) {
		mu.Lock()
		defer mu.Unlock()
		last = p
		if p.ReposFound > maxFound {
			maxFound = p.ReposFound
		}
		if p.ReposChecked > maxChecked {
			maxChecked = p.ReposChecked
		}
	}

	mgs, err := ScanWithProgress(cfg, callback)
	if err != nil {
		t.Fatalf("ScanWithProgress: %v", err)
	}
	if mgs.Len() != 1 {
		t.Fatalf("want 1 dirty repo, got %d", mgs.Len())
	}
	mu.Lock()
	defer mu.Unlock()
	if maxFound < 1 {
		t.Fatalf("expected ReposFound >= 1 in some callback, got maxFound=%d", maxFound)
	}
	if maxChecked < 1 {
		t.Fatalf("expected ReposChecked >= 1, got maxChecked=%d", maxChecked)
	}
	if last.ReposFound < 1 || last.ReposChecked < 1 {
		t.Fatalf("last progress should reflect completed work: %+v", last)
	}
	if last.CurrentPath != repo {
		t.Fatalf("last progress should keep CurrentPath until next repo: got %q want %q", last.CurrentPath, repo)
	}
}
