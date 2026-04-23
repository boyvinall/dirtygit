package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusPathUnderRepo(t *testing.T) {
	repo := t.TempDir()
	sub := filepath.Join(repo, "pkg", "file.go")
	if err := os.MkdirAll(filepath.Dir(sub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sub, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := statusPathUnderRepo(repo, "pkg/file.go")
	if err != nil {
		t.Fatalf("statusPathUnderRepo: %v", err)
	}
	if got != sub {
		t.Fatalf("got %q want %q", got, sub)
	}

	if _, err := statusPathUnderRepo(repo, "../outside"); err == nil {
		t.Fatal("expected error for path escaping repo")
	}
	if _, err := statusPathUnderRepo("", "a"); err == nil {
		t.Fatal("expected error for empty repo")
	}
}
