package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
		branchTable:      newBranchTable(),
		diffMode:         diffModeWorktree,
		diffNeedsRefresh: true,
	}
	m.logVP = viewport.New(20, 5)
	m.diffVP = viewport.New(20, 5)
	return m
}

// TestPaneAtTerminalCell maps screen coordinates to panes for the main layout.
func TestPaneAtTerminalCell(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"a", "b", "c"}

	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	repoOuter := panelOuter(repoBody)
	statusOuter := panelOuter(statusBody)
	diffOuter := panelOuter(diffBody)
	logOuter := panelOuter(logBody)
	statusW, _ := m.statusBranchesOuterWidths(m.width)

	tests := []struct {
		x, y int
		want pane
		ok   bool
	}{
		{0, 0, paneRepo, true},
		{99, repoOuter - 1, paneRepo, true},
		{0, repoOuter, paneStatus, true},
		{statusW - 1, repoOuter, paneStatus, true},
		{statusW, repoOuter, paneBranches, true},
		{0, repoOuter + statusOuter, paneDiff, true},
		{0, repoOuter + statusOuter + diffOuter, paneLog, true},
		{0, repoOuter + statusOuter + diffOuter + logOuter - 1, paneLog, true},
		{-1, 0, paneRepo, false},
		{0, m.height, paneRepo, false},
	}
	for _, tc := range tests {
		got, ok := m.paneAtTerminalCell(tc.x, tc.y)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("paneAtTerminalCell(%d,%d) = (%v,%v), want (%v,%v) repoOuter=%d statusOuter=%d diffOuter=%d logOuter=%d statusW=%d",
				tc.x, tc.y, got, ok, tc.want, tc.ok, repoOuter, statusOuter, diffOuter, logOuter, statusW)
		}
	}

	m.zoomed = true
	m.zoomTarget = paneDiff
	if p, ok := m.paneAtTerminalCell(5, 10); !ok || p != paneDiff {
		t.Fatalf("zoomed paneAtTerminalCell = (%v,%v), want (paneDiff,true)", p, ok)
	}
}

// TestMouseFocusClickUpdatesFocus exercises left-click pane focusing via Update.
func TestMouseFocusClickUpdatesFocus(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"a", "b", "c"}
	m.focus = paneRepo

	rb, sb, _, _ := m.layoutBodies()
	click := tea.MouseMsg{
		X:      0,
		Y:      panelOuter(rb) + panelOuter(sb)/2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}

	next, _ := m.Update(click)
	mm, ok := next.(*model)
	if !ok {
		t.Fatalf("Update should return *model, got %T", next)
	}
	if mm.focus != paneStatus {
		t.Fatalf("after click on status area, focus = %v, want paneStatus", mm.focus)
	}

	mm.err = fmt.Errorf("boom")
	if _, _ = mm.Update(click); mm.focus != paneStatus {
		t.Fatal("click should not change focus while error overlay is active")
	}
	mm.err = nil

	// Same pane: should fall through without breaking focus
	if _, _ = mm.Update(click); mm.focus != paneStatus {
		t.Fatalf("click same pane should keep focus=%v", mm.focus)
	}

	wheel := tea.MouseMsg{
		X:      click.X,
		Y:      click.Y,
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	}
	if _, _ = mm.Update(wheel); mm.focus != paneStatus {
		t.Fatal("wheel should not retarget focus")
	}
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

// TestRefreshBranchContentOneRowPerBranch verifies one table row per local branch name.
func TestRefreshBranchContentOneRowPerBranch(t *testing.T) {
	m := newTestModel()
	m.repoList = []string{"/repo"}
	m.repositories["/repo"] = scanner.RepoStatus{
		Branches: scanner.BranchStatus{
			Branch:         "aaa",
			NewestLocation: "origin",
			Locations: []scanner.BranchLocation{
				{Name: "local", Exists: true, TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, UniqueCount: 2},
				{Name: "origin", Exists: true, TipHash: "bbbbbbbbbbbbbbbb", TipUnix: 1_700_000_001, UniqueCount: 1, NewestUniqueUnix: 1_700_000_001, Incoming: 1, Outgoing: 2},
				{Name: "upstream", Exists: false},
			},
			// Names sort opposite to recency so the test proves UI order is by tip time, not name.
			LocalBranches: []scanner.LocalBranchRef{
				{Name: "aaa", TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, Current: true, Locations: []scanner.BranchLocation{
					{Name: "local", Exists: true, TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, UniqueCount: 2},
					{Name: "origin", Exists: true, TipHash: "bbbbbbbbbbbbbbbb", TipUnix: 1_700_000_001, UniqueCount: 1, NewestUniqueUnix: 1_700_000_001, Incoming: 1, Outgoing: 2},
					{Name: "upstream", Exists: false},
				}},
				{Name: "zzz", TipHash: "cccccccccccccccc", TipUnix: 1_700_000_002, Current: false, Locations: []scanner.BranchLocation{
					{Name: "local", Exists: true, TipHash: "cccccccccccccccc", TipUnix: 1_700_000_002},
					{Name: "origin", Exists: true, TipHash: "cccccccccccccccc", TipUnix: 1_700_000_002},
				}},
			},
		},
	}

	m.refreshBranchContent(60)
	cols := m.branchTable.Columns()
	if len(cols) != 4 {
		t.Fatalf("branch columns len = %d, want 4 (Branch, Commit, Tip age, Remotes)", len(cols))
	}
	rows := m.branchTable.Rows()
	if len(rows) != 1 {
		t.Fatalf("branch rows len = %d, want 1 (in-sync branch zzz omitted)", len(rows))
	}
	if rows[0][0] != "aaa" {
		t.Fatalf("branch name column = %#v, want aaa (only branch with remote tip mismatch)", rows[0][0])
	}
	wantRemote := "origin +1-2, upstream: missing"
	if rows[0][3] != wantRemote {
		t.Fatalf("branch remotes cell = %q, want %q", rows[0][3], wantRemote)
	}
}

func TestRefreshBranchContentHidesLocalOnlyMatchingRegex(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "dirtygit.yml")
	content := `
scandirs:
  include:
    - /tmp
branches:
  hidelocalonly:
    regex:
      - "^wip/"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := scanner.ParseConfigFile(cfgPath, "")
	if err != nil {
		t.Fatalf("ParseConfigFile: %v", err)
	}

	m := newTestModel()
	m.config = cfg
	m.repoList = []string{"/repo"}
	m.repositories["/repo"] = scanner.RepoStatus{
		Branches: scanner.BranchStatus{
			Branch:         "aaa",
			NewestLocation: "origin",
			Locations: []scanner.BranchLocation{
				{Name: "local", Exists: true, TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, UniqueCount: 2},
				{Name: "origin", Exists: true, TipHash: "bbbbbbbbbbbbbbbb", TipUnix: 1_700_000_001, UniqueCount: 1, NewestUniqueUnix: 1_700_000_001, Incoming: 1, Outgoing: 2},
			},
			LocalBranches: []scanner.LocalBranchRef{
				{Name: "aaa", TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, Current: true, Locations: []scanner.BranchLocation{
					{Name: "local", Exists: true, TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, UniqueCount: 2},
					{Name: "origin", Exists: true, TipHash: "bbbbbbbbbbbbbbbb", TipUnix: 1_700_000_001, UniqueCount: 1, NewestUniqueUnix: 1_700_000_001, Incoming: 1, Outgoing: 2},
				}},
				{Name: "wip/hidden", TipHash: "dddddddddddddddd", TipUnix: 1_700_000_003, Current: false, Locations: []scanner.BranchLocation{
					{Name: "local", Exists: true, TipHash: "dddddddddddddddd", TipUnix: 1_700_000_003},
					{Name: "origin", Exists: false},
				}},
				{Name: "other-local", TipHash: "eeeeeeeeeeeeeeee", TipUnix: 1_700_000_004, Current: false, Locations: []scanner.BranchLocation{
					{Name: "local", Exists: true, TipHash: "eeeeeeeeeeeeeeee", TipUnix: 1_700_000_004},
					{Name: "origin", Exists: false},
				}},
			},
		},
	}

	m.refreshBranchContent(60)
	rows := m.branchTable.Rows()
	if len(rows) != 2 {
		t.Fatalf("branch rows len = %d, want 2 (wip/hidden omitted)", len(rows))
	}
	got := map[string]bool{rows[0][0]: true, rows[1][0]: true}
	for _, name := range []string{"aaa", "other-local"} {
		if !got[name] {
			t.Fatalf("branch rows = [%s %s], want aaa and other-local", rows[0][0], rows[1][0])
		}
	}
}

func TestRefreshBranchContentAlwaysListsDefaultBranchesInSync(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "dirtygit-defaults.yml")
	content := `
scandirs:
  include:
    - /tmp
branches:
  default:
    - main
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := scanner.ParseConfigFile(cfgPath, "")
	if err != nil {
		t.Fatalf("ParseConfigFile: %v", err)
	}

	m := newTestModel()
	m.config = cfg
	m.repoList = []string{"/repo"}
	m.repositories["/repo"] = scanner.RepoStatus{
		Branches: scanner.BranchStatus{
			Branch:         "aaa",
			NewestLocation: "origin",
			Locations: []scanner.BranchLocation{
				{Name: "local", Exists: true, TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, UniqueCount: 2},
				{Name: "origin", Exists: true, TipHash: "bbbbbbbbbbbbbbbb", TipUnix: 1_700_000_001, UniqueCount: 1, NewestUniqueUnix: 1_700_000_001, Incoming: 1, Outgoing: 2},
				{Name: "upstream", Exists: false},
			},
			LocalBranches: []scanner.LocalBranchRef{
				{Name: "aaa", TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, Current: true, Locations: []scanner.BranchLocation{
					{Name: "local", Exists: true, TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, UniqueCount: 2},
					{Name: "origin", Exists: true, TipHash: "bbbbbbbbbbbbbbbb", TipUnix: 1_700_000_001, UniqueCount: 1, NewestUniqueUnix: 1_700_000_001, Incoming: 1, Outgoing: 2},
					{Name: "upstream", Exists: false},
				}},
				{Name: "main", TipHash: "cccccccccccccccc", TipUnix: 1_700_000_002, Current: false, Locations: []scanner.BranchLocation{
					{Name: "local", Exists: true, TipHash: "cccccccccccccccc", TipUnix: 1_700_000_002},
					{Name: "origin", Exists: true, TipHash: "cccccccccccccccc", TipUnix: 1_700_000_002},
				}},
				{Name: "zzz", TipHash: "dddddddddddddddd", TipUnix: 1_700_000_003, Current: false, Locations: []scanner.BranchLocation{
					{Name: "local", Exists: true, TipHash: "dddddddddddddddd", TipUnix: 1_700_000_003},
					{Name: "origin", Exists: true, TipHash: "dddddddddddddddd", TipUnix: 1_700_000_003},
				}},
			},
		},
	}

	m.refreshBranchContent(60)
	rows := m.branchTable.Rows()
	if len(rows) != 2 {
		t.Fatalf("branch rows len = %d, want 2 (zzz in sync omitted, main listed as default)", len(rows))
	}
	got := map[string]bool{rows[0][0]: true, rows[1][0]: true}
	for _, name := range []string{"aaa", "main"} {
		if !got[name] {
			t.Fatalf("branch rows = [%s %s], want aaa and main", rows[0][0], rows[1][0])
		}
	}
}

func TestRefreshBranchContentDefaultBranchOverridesLocalOnlyHide(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "dirtygit-default-hide.yml")
	content := `
scandirs:
  include:
    - /tmp
branches:
  hidelocalonly:
    regex:
      - "^main$"
  default:
    - main
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := scanner.ParseConfigFile(cfgPath, "")
	if err != nil {
		t.Fatalf("ParseConfigFile: %v", err)
	}

	m := newTestModel()
	m.config = cfg
	m.repoList = []string{"/repo"}
	m.repositories["/repo"] = scanner.RepoStatus{
		Branches: scanner.BranchStatus{
			Branch:         "aaa",
			NewestLocation: "origin",
			Locations: []scanner.BranchLocation{
				{Name: "local", Exists: true, TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, UniqueCount: 2},
				{Name: "origin", Exists: true, TipHash: "bbbbbbbbbbbbbbbb", TipUnix: 1_700_000_001, UniqueCount: 1, NewestUniqueUnix: 1_700_000_001, Incoming: 1, Outgoing: 2},
			},
			LocalBranches: []scanner.LocalBranchRef{
				{Name: "aaa", TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, Current: true, Locations: []scanner.BranchLocation{
					{Name: "local", Exists: true, TipHash: "aaaaaaaaaaaaaaaa", TipUnix: 1_700_000_000, UniqueCount: 2},
					{Name: "origin", Exists: true, TipHash: "bbbbbbbbbbbbbbbb", TipUnix: 1_700_000_001, UniqueCount: 1, NewestUniqueUnix: 1_700_000_001, Incoming: 1, Outgoing: 2},
				}},
				{Name: "main", TipHash: "cccccccccccccccc", TipUnix: 1_700_000_002, Current: false, Locations: []scanner.BranchLocation{
					{Name: "local", Exists: true, TipHash: "cccccccccccccccc", TipUnix: 1_700_000_002},
					{Name: "origin", Exists: false},
				}},
			},
		},
	}

	m.refreshBranchContent(60)
	rows := m.branchTable.Rows()
	if len(rows) != 2 {
		t.Fatalf("branch rows len = %d, want 2 (main shown despite hide regex)", len(rows))
	}
	got := map[string]bool{rows[0][0]: true, rows[1][0]: true}
	for _, name := range []string{"aaa", "main"} {
		if !got[name] {
			t.Fatalf("branch rows = [%s %s], want aaa and main", rows[0][0], rows[1][0])
		}
	}
}
