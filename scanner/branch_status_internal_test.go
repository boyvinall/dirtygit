package scanner

import (
	"os/exec"
	"strings"
	"testing"
)

func TestHaveMergeBaseRelatedBranches(t *testing.T) {
	dir := t.TempDir()
	gitMinimalInit(t, dir)
	gitCommitFile(t, dir, "f.txt", "a\n", "m0")
	execGit(t, dir, "checkout", "-b", "side")
	gitCommitFile(t, dir, "f.txt", "a\nb\n", "m1")
	execGit(t, dir, "checkout", "main")

	mainHash := strings.TrimSpace(execGitOutput(t, dir, "rev-parse", "main"))
	sideHash := strings.TrimSpace(execGitOutput(t, dir, "rev-parse", "side"))
	ok, err := haveMergeBase(dir, mainHash, sideHash)
	if err != nil {
		t.Fatalf("haveMergeBase: %v", err)
	}
	if !ok {
		t.Fatal("expected merge base for related branch tips")
	}
}

func TestHaveMergeBaseUnrelatedHistories(t *testing.T) {
	dir := t.TempDir()
	gitMinimalInit(t, dir)
	gitCommitFile(t, dir, "a.txt", "1\n", "on-main")
	execGit(t, dir, "checkout", "--orphan", "orph")
	gitCommitFile(t, dir, "b.txt", "2\n", "on-orph")

	mainHash := strings.TrimSpace(execGitOutput(t, dir, "rev-parse", "main"))
	orphHash := strings.TrimSpace(execGitOutput(t, dir, "rev-parse", "orph"))

	ok, err := haveMergeBase(dir, mainHash, orphHash)
	if err != nil {
		t.Fatalf("haveMergeBase: %v", err)
	}
	if ok {
		t.Fatal("expected no merge base between main and unrelated orphan branch")
	}
}

func TestUniqueCommitCountAheadOnBranch(t *testing.T) {
	dir := t.TempDir()
	gitMinimalInit(t, dir)
	gitCommitFile(t, dir, "f.txt", "v0\n", "base")
	execGit(t, dir, "checkout", "-b", "ahead")
	gitCommitFile(t, dir, "f.txt", "v0\nv1\n", "ahead-only")
	execGit(t, dir, "checkout", "main")

	n, err := uniqueCommitCount(dir, "refs/heads/ahead", []string{"refs/heads/main"})
	if err != nil {
		t.Fatalf("uniqueCommitCount: %v", err)
	}
	if n != 1 {
		t.Fatalf("unique commit count = %d, want 1", n)
	}
}

func execGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func execGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
