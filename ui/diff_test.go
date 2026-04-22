package ui

import (
	"strings"
	"testing"
)

// TestDiffPaneBorderTitle lists Worktree and Staged and highlights the active diff mode.
func TestDiffPaneBorderTitle(t *testing.T) {
	m := newTestModel()
	m.focus = paneRepo

	m.diffMode = diffModeWorktree
	wt := m.diffPaneBorderTitle()
	if !strings.Contains(wt, "Worktree") || !strings.Contains(wt, "Staged") {
		t.Fatalf("want both mode labels, got %q", wt)
	}
	if strings.Count(wt, "38;5;51m") < 1 {
		t.Fatalf("worktree mode should use cyan 51 for active label, got %q", wt)
	}

	m.diffMode = diffModeStaged
	st := m.diffPaneBorderTitle()
	if strings.Count(st, "38;5;51m") < 1 {
		t.Fatalf("staged mode should use cyan 51 for active label, got %q", st)
	}

	m.focus = paneDiff
	m.diffMode = diffModeWorktree
	foc := m.diffPaneBorderTitle()
	if !strings.Contains(foc, "Diff\x1b[0m") || !strings.Contains(foc, "93mDiff") {
		t.Fatalf("diff pane focus should emphasize Diff, got %q", foc)
	}
}
