package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitMinimalInit creates a new repository with identity set. Uses core.hooksPath
// pointing at the OS null device when supported so template hooks are not installed
// (helps restricted CI/sandbox environments).
func gitMinimalInit(t *testing.T, repoRoot string) {
	t.Helper()
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	hooks := os.DevNull
	cmd := exec.Command("git", "-c", "init.defaultBranch=main", "-c", "core.hooksPath="+hooks, "init")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd2 := exec.Command("git", "-c", "init.defaultBranch=main", "init")
		cmd2.Dir = repoRoot
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			t.Fatalf("git init: %v: %s; fallback: %v: %s", err, out, err2, out2)
		}
	}
	for _, argv := range [][]string{
		{"git", "config", "user.email", "t@example.com"},
		{"git", "config", "user.name", "test"},
	} {
		c := exec.Command(argv[0], argv[1:]...)
		c.Dir = repoRoot
		if o, e := c.CombinedOutput(); e != nil {
			t.Fatalf("%v %v: %s", argv, e, o)
		}
	}
}

func gitCommitFile(t *testing.T, repoRoot, name, content, msg string) {
	t.Helper()
	p := filepath.Join(repoRoot, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	cmd := exec.Command("git", "add", "--", name)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", msg)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
}
