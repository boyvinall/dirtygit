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

// TestStatusBranchesRowUsesFullWidth ensures outer pane widths sum to the terminal width.
func TestStatusBranchesRowUsesFullWidth(t *testing.T) {
	m := newTestModel()
	m.width = 100
	so, bo := m.statusBranchesOuterWidths(m.width)
	if so+bo != m.width {
		t.Fatalf("statusOuter(%d) + branchesOuter(%d) = %d, want terminal width %d",
			so, bo, so+bo, m.width)
	}
	if so != 50 || bo != 50 {
		t.Fatalf("default horizontal split = (%d,%d), want (50,50)", so, bo)
	}
}

func TestStatusBranchesOuterWidthsOddWidthPutsRemainderOnBranches(t *testing.T) {
	m := newTestModel()
	m.width = 81
	so, bo := m.statusBranchesOuterWidths(m.width)
	if so != 40 || bo != 41 {
		t.Fatalf("statusOuter, branchesOuter = (%d,%d), want (40,41)", so, bo)
	}
}

// TestStatusTableViewFitsInnerWidth guards against table padding widening rows past the pane.
func TestStatusTableViewFitsInnerWidth(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 30
	m.repoList = []string{"/repo"}
	m.syncViewports()
	si, _ := m.statusBranchesInnerWidths()
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
	_, bi := m.statusBranchesInnerWidths()
	for _, line := range strings.Split(m.branchTable.View(), "\n") {
		if line == "" {
			continue
		}
		if w := lipgloss.Width(line); w > bi {
			t.Fatalf("branch table line width %d > inner width %d", w, bi)
		}
	}
}

// blankStatusBranchesRow renders the Status/Branches row with empty table bodies for predictable ANSI.
func blankStatusBranchesRow(m *model, outerH int) string {
	m.syncViewports()
	si, bi := m.statusBranchesInnerWidths()
	return m.framedStatusBranchesRow(outerH, strings.Repeat(" ", si), strings.Repeat(" ", bi))
}

// ansi214Count counts 256-color foreground sequences for lipgloss color 214 (focus accent).
func ansi214Count(s string) int {
	return strings.Count(s, "38;5;214m")
}

// TestFramedStatusBranchesBorderAccentIsolation checks that only the focused pane's border uses
// the 214 accent when Status or Branches is focused (repo focus uses none).
func TestFramedStatusBranchesBorderAccentIsolation(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 28
	m.repoList = []string{"/r"}

	m.focus = paneRepo
	repo := blankStatusBranchesRow(m, 6)
	m.focus = paneStatus
	st := blankStatusBranchesRow(m, 6)
	m.focus = paneBranches
	br := blankStatusBranchesRow(m, 6)

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
