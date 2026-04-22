package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/go-git/go-git/v5"

	"github.com/boyvinall/dirtygit/scanner"
)

// newTestModel builds a model with minimal defaults for UI unit tests.
func newTestModel() *model {
	m := &model{
		logBuf:           &logBuffer{max: 50},
		repositories:     make(scanner.MultiGitStatus),
		focus:            paneRepo,
		statusTable:      newStatusTable(),
		diffMode:         diffModeWorktree,
		diffNeedsRefresh: true,
	}
	m.logVP = viewport.New(20, 5)
	m.diffVP = viewport.New(20, 5)
	return m
}

// TestLayoutBodies verifies valid pane body sizes on normal terminals.
func TestLayoutBodies(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"a", "b", "c"}

	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	if repoBody < 3 || statusBody < 3 || diffBody < 3 || logBody < 3 {
		t.Fatalf("layoutBodies() = (%d, %d, %d, %d), expected all >= 3", repoBody, statusBody, diffBody, logBody)
	}
	if diffBody < statusBody*3 {
		t.Fatalf("layoutBodies() expected diff to be at least 3x status, got status=%d diff=%d", statusBody, diffBody)
	}
}

// TestLayoutBodiesReturnsZerosOnSmallScreen ensures tiny terminals short-circuit layout.
func TestLayoutBodiesReturnsZerosOnSmallScreen(t *testing.T) {
	m := newTestModel()
	m.width = 10
	m.height = 10

	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	if repoBody != 0 || statusBody != 0 || diffBody != 0 || logBody != 0 {
		t.Fatalf("layoutBodies() = (%d, %d, %d, %d), want (0,0,0,0)", repoBody, statusBody, diffBody, logBody)
	}
}

// TestLayoutBodiesZoomedPaneOnly ensures zoom mode only sizes one pane.
func TestLayoutBodiesZoomedPaneOnly(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.zoomed = true
	m.zoomTarget = paneStatus

	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	if repoBody != 0 || diffBody != 0 || logBody != 0 || statusBody == 0 {
		t.Fatalf("layoutBodies() = (%d, %d, %d, %d), want repo=0 status>0 diff=0 log=0", repoBody, statusBody, diffBody, logBody)
	}
}

// TestSortedRepoPaths checks repository path ordering is alphabetical.
func TestSortedRepoPaths(t *testing.T) {
	got := sortedRepoPaths(scanner.MultiGitStatus{
		"/z": {},
		"/a": {},
		"/m": {},
	})
	want := []string{"/a", "/m", "/z"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortedRepoPaths() = %v, want %v", got, want)
		}
	}
}

// TestRefreshStatusContentUsesPorcelainAndSorts validates porcelain-first rendering.
func TestRefreshStatusContentUsesPorcelainAndSorts(t *testing.T) {
	m := newTestModel()
	m.repoList = []string{"/repo"}
	m.repositories["/repo"] = scanner.RepoStatus{
		Porcelain: scanner.PorcelainStatus{
			Entries: []scanner.PorcelainEntry{
				{Staging: 'R', Worktree: ' ', OriginalPath: "z-old.go", Path: "a-new.go"},
				{Staging: 'M', Worktree: ' ', Path: "b.go"},
			},
		},
	}

	m.refreshStatusContent()
	rows := m.statusTable.Rows()
	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2", len(rows))
	}
	if rows[0][2] != "z-old.go -> a-new.go" {
		t.Fatalf("first path = %q, want rename path", rows[0][2])
	}
	if rows[1][2] != "b.go" {
		t.Fatalf("second path = %q, want b.go", rows[1][2])
	}
}

// TestRefreshStatusContentFallsBackToGitStatus validates status-map fallback behavior.
func TestRefreshStatusContentFallsBackToGitStatus(t *testing.T) {
	m := newTestModel()
	m.repoList = []string{"/repo"}
	m.repositories["/repo"] = scanner.RepoStatus{
		Status: git.Status{
			"z.go": &git.FileStatus{Staging: 'M', Worktree: ' '},
			"a.go": &git.FileStatus{Staging: ' ', Worktree: 'D'},
		},
	}

	m.refreshStatusContent()
	rows := m.statusTable.Rows()
	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2", len(rows))
	}
	if rows[0][2] != "a.go" || rows[1][2] != "z.go" {
		t.Fatalf("rows order = [%q, %q], want [a.go, z.go]", rows[0][2], rows[1][2])
	}
}

// TestStatusCodeLabel verifies human labels for git status codes.
func TestStatusCodeLabel(t *testing.T) {
	cases := map[git.StatusCode]string{
		'M': "modified",
		'A': "added",
		'D': "deleted",
		'R': "renamed",
		'C': "copied",
		'U': "unmerged",
		'?': "untracked",
		'!': "ignored",
		' ': "-",
		'X': "X",
	}
	for code, want := range cases {
		got := statusCodeLabel(code)
		if got != want {
			t.Fatalf("statusCodeLabel(%q) = %q, want %q", code, got, want)
		}
	}
}

// TestCurrentRepoBounds ensures cursor bounds return expected repository paths.
func TestCurrentRepoBounds(t *testing.T) {
	m := newTestModel()
	m.repoList = []string{"/a", "/b"}

	m.cursor = 1
	if got := m.currentRepo(); got != "/b" {
		t.Fatalf("currentRepo() = %q, want /b", got)
	}
	m.cursor = -1
	if got := m.currentRepo(); got != "" {
		t.Fatalf("currentRepo() = %q, want empty for negative cursor", got)
	}
	m.cursor = 2
	if got := m.currentRepo(); got != "" {
		t.Fatalf("currentRepo() = %q, want empty for out-of-range cursor", got)
	}
}

// TestSyncViewportsSetsDimensions checks positive dimensions after layout sync.
func TestSyncViewportsSetsDimensions(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.focus = paneStatus
	m.repoList = []string{"r1", "r2"}
	m.statusTable = table.New()

	m.syncViewports()

	if m.statusTable.Width() <= 0 || m.statusTable.Height() <= 0 {
		t.Fatalf("syncViewports() produced non-positive table dimensions: %dx%d", m.statusTable.Width(), m.statusTable.Height())
	}
	if m.logVP.Width <= 0 || m.logVP.Height <= 0 {
		t.Fatalf("syncViewports() produced non-positive viewport dimensions: %dx%d", m.logVP.Width, m.logVP.Height)
	}
	if m.diffVP.Width <= 0 || m.diffVP.Height <= 0 {
		t.Fatalf("syncViewports() produced non-positive diff viewport dimensions: %dx%d", m.diffVP.Width, m.diffVP.Height)
	}
}
