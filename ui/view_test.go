package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/boyvinall/dirtygit/scanner"
)

func init() {
	// Stable ANSI sequences for border color assertions (integration-style tests).
	lipgloss.SetColorProfile(termenv.ANSI256)
}

// TestMiddleRowColumnWidthsSumToTermWidth ensures middle row columns fill the width.
func TestMiddleRowColumnWidthsSumToTermWidth(t *testing.T) {
	m := newTestModel()
	m.width = 100
	lo, ro := m.middleRowColumnOuterWidths(m.width)
	if lo+ro != m.width {
		t.Fatalf("leftOuter(%d) + rightOuter(%d) = %d, want terminal width %d",
			lo, ro, lo+ro, m.width)
	}
	if lo != 30 || ro != 70 {
		t.Fatalf("default horizontal split = (%d,%d), want (30,70)", lo, ro)
	}
}

func TestMiddleRowColumnOuterWidthsOddWidth(t *testing.T) {
	m := newTestModel()
	m.width = 81
	lo, ro := m.middleRowColumnOuterWidths(m.width)
	if lo+ro != m.width {
		t.Fatalf("leftOuter(%d)+rightOuter(%d) != width %d", lo, ro, m.width)
	}
	if lo != 25 || ro != 56 {
		t.Fatalf("leftOuter, rightOuter = (%d,%d), want (25,56)", lo, ro)
	}
}

// TestStatusTableViewFitsInnerWidth guards against table padding widening rows past the pane.
func TestStatusTableViewFitsInnerWidth(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"/repo"}
	m.syncViewports()
	si, _ := m.middleRowColumnInnerWidths()
	for _, line := range strings.Split(m.statusTable.View(), "\n") {
		if line == "" {
			continue
		}
		if w := lipgloss.Width(line); w > si {
			t.Fatalf("status table line width %d > inner width %d", w, si)
		}
	}
}

func TestBranchTableViewFitsInnerWidth(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"/repo"}
	m.repositories.Set("/repo", scanner.RepoStatus{
		Branches: scanner.BranchStatus{
			Branch:         "main",
			NewestLocation: "origin",
			Locations: []scanner.BranchLocation{
				{Name: "local", Exists: true, TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, UniqueCount: 2},
				{Name: "origin", Exists: true, TipHash: "bbbbbbbbbbbbbbbb", TipUnix: 1_700_000_001, UniqueCount: 1, NewestUniqueUnix: 1_700_000_001, Incoming: 1, Outgoing: 2},
				{Name: "upstream", Exists: false},
			},
		},
	})
	m.syncViewports()
	_, bi := m.middleRowColumnInnerWidths()
	for _, line := range strings.Split(m.branchTable.View(), "\n") {
		if line == "" {
			continue
		}
		if w := lipgloss.Width(line); w > bi {
			t.Fatalf("branch table line width %d > inner width %d", w, bi)
		}
	}
}

// blankMiddleRow renders the middle band with empty bodies for predictable ANSI.
func blankMiddleRow(m *model, statusBody, branchBody, diffBody int) string {
	m.syncViewports()
	li, ri := m.middleRowColumnInnerWidths()
	return m.framedMiddleRow(statusBody, branchBody, diffBody,
		strings.Repeat(" ", li), strings.Repeat(" ", li), strings.Repeat(" ", ri))
}

// ansi214Count counts 256-color foreground sequences for lipgloss color 214 (focus accent).
func ansi214Count(s string) int {
	return strings.Count(s, "38;5;214m")
}

// TestFramedMiddleRowBorderAccentIsolation checks that only the focused pane's border uses
// the 214 accent when Status or Branches is focused (repo focus uses none).
func TestFramedMiddleRowBorderAccentIsolation(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 28
	m.repoList = []string{"/r"}

	sb, bb, db := 4, 4, 10
	m.focus = paneRepo
	repo := blankMiddleRow(m, sb, bb, db)
	m.focus = paneStatus
	st := blankMiddleRow(m, sb, bb, db)
	m.focus = paneBranches
	br := blankMiddleRow(m, sb, bb, db)

	if c := ansi214Count(repo); c != 0 {
		t.Fatalf("repo focus: expected no 214 border accents, got %d in %q", c, repo)
	}
	if ansi214Count(st) <= ansi214Count(repo) {
		t.Fatalf("status focus should use more 214 than repo focus")
	}
	if ansi214Count(br) <= ansi214Count(repo) {
		t.Fatalf("branches focus should use more 214 than repo focus")
	}
}
