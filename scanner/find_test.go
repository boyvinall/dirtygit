package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestWalkFindsRepos(t *testing.T) {
	root := t.TempDir()
	repoA := filepath.Join(root, "a")
	repoB := filepath.Join(root, "nested", "b")

	for _, p := range []string{
		filepath.Join(repoA, ".git"),
		filepath.Join(repoB, ".git"),
	} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", p, err)
		}
	}

	cfg := &Config{}
	cfg.ScanDirs.Include = []string{root}

	results := make(chan string, 10)
	if err := Walk(context.Background(), cfg, results, nil); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	var got []string
	for repo := range results {
		got = append(got, repo)
	}
	sort.Strings(got)

	want := []string{repoA, repoB}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("Walk() repos = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Walk() repos = %v, want %v", got, want)
		}
	}
}

func TestWalkHonorsExclude(t *testing.T) {
	root := t.TempDir()
	repoA := filepath.Join(root, "a")
	repoB := filepath.Join(root, "b")

	for _, p := range []string{
		filepath.Join(repoA, ".git"),
		filepath.Join(repoB, ".git"),
	} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", p, err)
		}
	}

	cfg := &Config{}
	cfg.ScanDirs.Include = []string{root}
	cfg.ScanDirs.Exclude = []string{repoB}

	results := make(chan string, 10)
	if err := Walk(context.Background(), cfg, results, nil); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	var got []string
	for repo := range results {
		got = append(got, repo)
	}
	if len(got) != 1 || got[0] != repoA {
		t.Fatalf("Walk() repos = %v, want [%q]", got, repoA)
	}
}
