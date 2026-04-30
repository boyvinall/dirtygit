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
	if !strings.Contains(foc, "Diff\x1b[0m") || strings.Count(foc, "38;5;227m") < 1 {
		t.Fatalf("diff pane focus should emphasize Diff, got %q", foc)
	}
}

func TestDiffModeLabel(t *testing.T) {
	m := newTestModel()
	m.diffMode = diffModeWorktree
	if got := m.diffModeLabel(); got != "Worktree" {
		t.Fatalf("diffModeLabel worktree = %q", got)
	}
	m.diffMode = diffModeStaged
	if got := m.diffModeLabel(); got != "Staged" {
		t.Fatalf("diffModeLabel staged = %q", got)
	}
}

func TestStyleDiffContentEmpty(t *testing.T) {
	if got := styleDiffContent(""); got != "" {
		t.Fatalf("empty input = %q, want empty", got)
	}
}

func TestStyleDiffContentPreservesLinesWithANSI(t *testing.T) {
	raw := strings.Join([]string{
		"diff --git a/x b/x",
		"index 111..222 100644",
		"--- a/x",
		"+++ b/x",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")
	out := styleDiffContent(raw)
	for _, sub := range []string{"diff --git", "old", "new", "@@"} {
		if !strings.Contains(out, sub) {
			t.Fatalf("output should contain %q, got %q", sub, out)
		}
	}
	if !strings.Contains(out, "\x1b[") {
		t.Fatal("expected ANSI sequences from lipgloss styles")
	}
}

func TestStyleDiffContentMetaLines(t *testing.T) {
	raw := "new file mode 100644\nsimilarity index 50%\nrename from a\nrename to b\n"
	out := styleDiffContent(raw)
	if !strings.Contains(out, "new file mode") {
		t.Fatalf("expected meta line preserved/styled: %q", out)
	}
}
