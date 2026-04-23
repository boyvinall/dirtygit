package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cspinner "github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/boyvinall/dirtygit/scanner"
)

// TestHelpAndUtilityFunctions validates shared key and formatting helpers.
func TestHelpAndUtilityFunctions(t *testing.T) {
	if !helpKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}) {
		t.Fatal("helpKey(?) should be true")
	}
	if !helpKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}) {
		t.Fatal("helpKey(h) should be true")
	}
	if helpKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}) {
		t.Fatal("helpKey(x) should be false")
	}

	if got := scanProgressBar(10, 2, 4); got != "█████░░░░░" {
		t.Fatalf("scanProgressBar() = %q, want █████░░░░░", got)
	}
	if got := scanProgressBar(0, 1, 1); got != "" {
		t.Fatalf("scanProgressBar() = %q, want empty", got)
	}

	if got := shortenScanPath("/tmp/short", 20); got != "/tmp/short" {
		t.Fatalf("shortenScanPath short = %q", got)
	}
	if got := shortenScanPath("/a/very/long/path/that/keeps/going", 10); got == "/a/very/long/path/that/keeps/going" {
		t.Fatal("shortenScanPath should shorten long paths")
	}

	if got := truncateASCII("abcdef", 4); got != "abc…" {
		t.Fatalf("truncateASCII() = %q, want abc…", got)
	}
}

// TestHandleHelpOverlayKey ensures overlay keys close the help modal.
func TestHandleHelpOverlayKey(t *testing.T) {
	m := newTestModel()
	m.helpOpen = true

	_, _ = m.handleHelpOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.helpOpen {
		t.Fatal("help overlay should close on escape")
	}

	m.helpOpen = true
	_, _ = m.handleHelpOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.helpOpen {
		t.Fatal("help overlay should close on help key")
	}

	m.helpOpen = true
	_, cmd := m.handleHelpOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("help overlay should return quit command on q")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("help overlay q should return QuitMsg, got %T", cmd())
	}
}

// TestHandleCommandKeyFocusAndZoom checks tab focus and zoom toggling behavior.
func TestHandleCommandKeyFocusAndZoom(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.focus = paneRepo

	_, _, handled := m.handleCommandKey(tea.KeyMsg{Type: tea.KeyTab})
	if !handled || m.focus != paneStatus {
		t.Fatalf("tab should move focus to status, got focus=%v handled=%v", m.focus, handled)
	}

	_, _, handled = m.handleCommandKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	if !handled || m.focus != paneRepo {
		t.Fatalf("shift+tab should move focus back to repo, got focus=%v handled=%v", m.focus, handled)
	}

	_, _, handled = m.handleCommandKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || !m.zoomed || m.zoomTarget != paneRepo {
		t.Fatalf("enter should enable zoom on focused pane, got zoomed=%v zoomTarget=%v", m.zoomed, m.zoomTarget)
	}

	_, _, handled = m.handleCommandKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !handled || m.zoomed {
		t.Fatalf("esc should exit zoom, got zoomed=%v handled=%v", m.zoomed, handled)
	}
}

// TestHandleArrowKeyRepoNavigation verifies repo cursor movement with arrows.
func TestHandleArrowKeyRepoNavigation(t *testing.T) {
	m := newTestModel()
	m.focus = paneRepo
	m.repoList = []string{"/a", "/b", "/c"}
	m.cursor = 1

	_, _, handled := m.handleArrowKey(tea.KeyMsg{Type: tea.KeyUp})
	if !handled || m.cursor != 0 {
		t.Fatalf("up should move cursor to 0, got cursor=%d handled=%v", m.cursor, handled)
	}

	_, _, handled = m.handleArrowKey(tea.KeyMsg{Type: tea.KeyDown})
	if !handled || m.cursor != 1 {
		t.Fatalf("down should move cursor to 1, got cursor=%d handled=%v", m.cursor, handled)
	}
}

// TestHandleArrowKeyRepoShiftStep verifies Shift+↓/↑ moves the repo cursor by 10 (clamped).
func TestHandleArrowKeyRepoShiftStep(t *testing.T) {
	m := newTestModel()
	m.focus = paneRepo
	m.repoList = make([]string, 25)
	for i := range m.repoList {
		m.repoList[i] = "/r"
	}
	m.cursor = 0

	_, _, handled := m.handleArrowKey(tea.KeyMsg{Type: tea.KeyShiftDown})
	if !handled || m.cursor != 10 {
		t.Fatalf("shift+down from 0 should go to 10, got cursor=%d handled=%v", m.cursor, handled)
	}

	_, _, handled = m.handleArrowKey(tea.KeyMsg{Type: tea.KeyShiftUp})
	if !handled || m.cursor != 0 {
		t.Fatalf("shift+up from 10 should go to 0, got cursor=%d handled=%v", m.cursor, handled)
	}

	m.cursor = 20
	_, _, handled = m.handleArrowKey(tea.KeyMsg{Type: tea.KeyShiftDown})
	if !handled || m.cursor != 24 {
		t.Fatalf("shift+down from 20 should clamp to last index 24, got cursor=%d handled=%v", m.cursor, handled)
	}
}

// TestRepoListScrollFollowsCursor ensures moving past the last visible row scrolls the repo pane.
func TestRepoListScrollFollowsCursor(t *testing.T) {
	m := newTestModel()
	m.focus = paneRepo
	m.repoList = []string{"/0", "/1", "/2", "/3", "/4", "/5", "/6", "/7", "/8", "/9"}
	m.cursor = 0
	m.repoScrollTop = 0
	const innerH = 3
	for i := 0; i < 5; i++ {
		_, _, handled := m.handleArrowKey(tea.KeyMsg{Type: tea.KeyDown})
		if !handled {
			t.Fatalf("down step %d not handled", i)
		}
		m.clampRepoScroll(innerH)
	}
	if m.cursor != 5 {
		t.Fatalf("cursor=%d want 5", m.cursor)
	}
	if m.repoScrollTop != 3 {
		t.Fatalf("repoScrollTop=%d want 3 (viewport shows indices 3..5)", m.repoScrollTop)
	}
	for i := 0; i < 3; i++ {
		_, _, handled := m.handleArrowKey(tea.KeyMsg{Type: tea.KeyUp})
		if !handled {
			t.Fatalf("up step %d not handled", i)
		}
		m.clampRepoScroll(innerH)
	}
	if m.cursor != 2 {
		t.Fatalf("cursor=%d want 2", m.cursor)
	}
	if m.cursor < m.repoScrollTop || m.cursor >= m.repoScrollTop+innerH {
		t.Fatalf("cursor=%d not in visible window [%d,%d)", m.cursor, m.repoScrollTop, m.repoScrollTop+innerH)
	}
}

// TestHandleArrowKeyDiffModeToggle verifies staged/worktree diff switching.
func TestHandleArrowKeyDiffModeToggle(t *testing.T) {
	m := newTestModel()
	m.focus = paneDiff
	m.diffMode = diffModeWorktree

	_, _, handled := m.handleArrowKey(tea.KeyMsg{Type: tea.KeyRight})
	if !handled || m.diffMode != diffModeStaged {
		t.Fatalf("right should switch diff mode to staged, got mode=%v handled=%v", m.diffMode, handled)
	}

	_, _, handled = m.handleArrowKey(tea.KeyMsg{Type: tea.KeyLeft})
	if !handled || m.diffMode != diffModeWorktree {
		t.Fatalf("left should switch diff mode to worktree, got mode=%v handled=%v", m.diffMode, handled)
	}
}

// TestHandleArrowKeyStatusPaneDiffModeToggle verifies Status pane uses the same ←/→ diff mode as Diff.
func TestHandleArrowKeyStatusPaneDiffModeToggle(t *testing.T) {
	m := newTestModel()
	m.focus = paneStatus
	m.diffMode = diffModeWorktree

	_, _, handled := m.handleArrowKey(tea.KeyMsg{Type: tea.KeyRight})
	if !handled || m.diffMode != diffModeStaged {
		t.Fatalf("right from status should switch diff mode to staged, got mode=%v handled=%v", m.diffMode, handled)
	}

	_, _, handled = m.handleArrowKey(tea.KeyMsg{Type: tea.KeyLeft})
	if !handled || m.diffMode != diffModeWorktree {
		t.Fatalf("left from status should switch diff mode to worktree, got mode=%v handled=%v", m.diffMode, handled)
	}
}

// TestHandleCommandKeyEscClearsStatusSelection ensures Esc clears status focus state.
func TestHandleCommandKeyEscClearsStatusSelection(t *testing.T) {
	m := newTestModel()
	m.focus = paneStatus
	m.statusFileSelected = true
	m.statusTable.Focus()

	_, _, handled := m.handleCommandKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !handled {
		t.Fatal("esc should be handled in status pane when file is selected")
	}
	if m.statusFileSelected {
		t.Fatal("esc should clear status file selection")
	}
	if m.statusTable.Focused() {
		t.Fatal("esc should clear status table highlight by blurring the table")
	}
}

// TestHandleCommandKeyStatusGitShortcuts ensures a/r are command keys only when a status row is selected.
func TestHandleCommandKeyStatusGitShortcuts(t *testing.T) {
	m := newTestModel()
	m.focus = paneStatus

	_, _, handled := m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if handled {
		t.Fatal("a should not be handled when no status file row is selected")
	}

	m.statusFileSelected = true
	m.statusPaths = []string{"file.go"}
	m.statusTable.SetRows([]table.Row{{"modified", "-", "file.go"}})
	m.statusTable.SetCursor(0)

	_, _, handled = m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !handled {
		t.Fatal("a should be handled when a status file is selected")
	}

	_, _, handled = m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !handled {
		t.Fatal("r should be handled when a status file is selected")
	}

	m.focus = paneDiff
	_, _, handled = m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !handled {
		t.Fatal("a should be handled in Diff pane when a status file is selected")
	}

	_, _, handled = m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	if !handled {
		t.Fatal("C should be handled in Diff pane when a status file is selected")
	}
	if !m.checkoutStatusFileConfirmOpen || m.checkoutStatusFilePendingRel != "file.go" {
		t.Fatalf("C should open checkout confirmation, open=%v pending=%q", m.checkoutStatusFileConfirmOpen, m.checkoutStatusFilePendingRel)
	}
}

// TestHandleCommandKeyDiffGitShortcutsIgnoredWithoutSelection ensures a is not swallowed in Diff without a file row.
func TestHandleCommandKeyDiffGitShortcutsIgnoredWithoutSelection(t *testing.T) {
	m := newTestModel()
	m.focus = paneDiff
	m.statusFileSelected = false

	_, _, handled := m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if handled {
		t.Fatal("a should not be handled in Diff pane when no status file is selected")
	}
	_, _, handled = m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	if handled {
		t.Fatal("C should not be handled in Diff pane when no status file is selected")
	}
}

// TestHandleScanTickFinishesScan ensures completed scan updates model state.
func TestHandleScanTickFinishesScan(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.scanning = true
	m.scanResultCh = make(chan scanResult, 1)
	m.scanProgressCh = make(chan scanner.ScanProgress, 2)
	m.scanProgressCh <- scanner.ScanProgress{ReposFound: 1, ReposChecked: 0}
	m.scanProgressCh <- scanner.ScanProgress{ReposFound: 2, ReposChecked: 1}
	m.scanResultCh <- scanResult{
		mgs: scanner.MultiGitStatus{
			"/repo": {},
		},
	}

	_, cmd := m.handleScanTick()
	if cmd != nil {
		t.Fatal("handleScanTick should return nil cmd when scan result is ready")
	}
	if m.scanning {
		t.Fatal("scanning should be false after finish")
	}
	if len(m.repoList) != 1 || m.repoList[0] != "/repo" {
		t.Fatalf("unexpected repo list: %v", m.repoList)
	}
	if m.scanProgress.ReposFound != 2 || m.scanProgress.ReposChecked != 1 {
		t.Fatalf("scan progress = %+v, want latest update", m.scanProgress)
	}
}

// TestHandleSpinnerTickWhenScanning ensures spinner keeps ticking during scans.
func TestHandleSpinnerTickWhenScanning(t *testing.T) {
	m := newTestModel()
	m.scanning = true
	m.scanSpinner = cspinner.New()

	_, cmd := m.handleSpinnerTick(cspinner.TickMsg{})
	if cmd == nil {
		t.Fatal("handleSpinnerTick should return spinner command while scanning")
	}
}

// TestWhyInclusionWKey checks that w opens the inclusion modal from the repository pane and Esc closes it.
func TestWhyInclusionWKey(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.focus = paneRepo
	m.repoList = []string{"/r"}
	m.cursor = 0
	m.repositories["/r"] = scanner.RepoStatus{}

	_, _, handled := m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	if !handled {
		t.Fatal("w should be handled in repo pane")
	}
	if !m.whyRepoOpen {
		t.Fatal("w should open why-inclusion modal")
	}

	s := m.renderWhyInclusionOverlay()
	if !strings.Contains(s, "Why is this repository") {
		t.Fatalf("modal should show title, got: %q", s)
	}

	mod, _ := m.handleWhyRepoOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	if mod.(*model).whyRepoOpen {
		t.Fatal("esc should close why-inclusion modal")
	}

	m.focus = paneStatus
	_, _, handled = m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	if handled {
		t.Fatal("w with status pane focused should not be handled as command (pass through to navigation)")
	}
}

// TestDeleteRepoDKey checks D opens the delete confirm modal, default highlights No, and
// successful confirmation removes the path from disk and the repo list.
func TestDeleteRepoDKey(t *testing.T) {
	tmp := t.TempDir()
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.focus = paneRepo
	m.repoList = []string{tmp}
	m.repositories[tmp] = scanner.RepoStatus{}
	m.cursor = 0

	_, _, handled := m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if !handled {
		t.Fatal("D should be handled in repo pane")
	}
	if !m.deleteRepoConfirmOpen {
		t.Fatal("D should open delete confirmation")
	}
	if m.deleteConfirmYes {
		t.Fatal("default selection should be No, not Yes")
	}
	overlay := m.renderDeleteRepoConfirmOverlay()
	if !strings.Contains(overlay, "Delete this directory") {
		t.Fatalf("expected delete title in overlay, got: %q", overlay)
	}

	// Enter with No: modal closes, directory still exists
	mod, _ := m.handleDeleteRepoConfirmKey(tea.KeyMsg{Type: tea.KeyEnter})
	if mod.(*model).deleteRepoConfirmOpen {
		t.Fatal("enter (No) should close the modal")
	}
	if _, err := os.Stat(tmp); err != nil {
		t.Fatalf("directory should still exist: %v", err)
	}

	// Re-open, confirm Yes removes path and prunes the list
	m.deleteRepoConfirmOpen = true
	m.deleteConfirmYes = true
	_, _ = m.handleDeleteRepoConfirmKey(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.repoList) != 0 {
		t.Fatalf("repo list should be empty, got %v", m.repoList)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatalf("directory should be removed, stat err = %v", err)
	}

	m2 := newTestModel()
	m2.width = 100
	m2.height = 30
	m2.focus = paneStatus
	_, _, handled2 := m2.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if handled2 {
		t.Fatal("d with status pane focused should not be handled as delete command")
	}
}

// TestDeleteStatusFileDKey checks D with a status row selected opens file-delete confirmation.
func TestDeleteStatusFileDKey(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"/repo"}
	m.cursor = 0
	m.focus = paneStatus
	m.statusFileSelected = true
	m.statusPaths = []string{"foo/bar.txt"}
	m.statusTable.SetRows([]table.Row{{"-", "?", "foo/bar.txt"}})
	m.statusTable.SetCursor(0)

	_, _, handled := m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if !handled {
		t.Fatal("D should be handled with status file selected")
	}
	if !m.deleteStatusFileConfirmOpen {
		t.Fatal("D should open status file delete confirmation")
	}
	if m.deleteStatusFilePendingRel != "foo/bar.txt" {
		t.Fatalf("pending rel = %q", m.deleteStatusFilePendingRel)
	}
	overlay := m.renderDeleteStatusFileConfirmOverlay()
	if !strings.Contains(overlay, "Delete this file or directory") {
		t.Fatalf("overlay title missing: %q", overlay)
	}
	if !strings.Contains(overlay, "foo/bar.txt") {
		t.Fatalf("overlay should show relative path: %q", overlay)
	}

	mod, _ := m.handleDeleteStatusFileConfirmKey(tea.KeyMsg{Type: tea.KeyEsc})
	if mod.(*model).deleteStatusFileConfirmOpen {
		t.Fatal("esc should close modal")
	}
}

// TestDeleteStatusFileConfirmRemovesPath verifies Yes + Enter deletes under repo base.
func TestDeleteStatusFileConfirmRemovesPath(t *testing.T) {
	repo := t.TempDir()
	subDir := filepath.Join(repo, "delme")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(subDir, "x.txt")
	if err := os.WriteFile(target, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{repo}
	m.cursor = 0
	m.deleteStatusFileConfirmOpen = true
	m.deleteStatusFilePendingRel = "delme/x.txt"
	m.deleteConfirmYes = true

	m.handleDeleteStatusFileConfirmKey(tea.KeyMsg{Type: tea.KeyEnter})
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("file should be removed, stat err=%v", err)
	}
}

// TestCheckoutStatusFileCKeyOpensConfirm verifies C opens the checkout-from-HEAD dialog.
func TestCheckoutStatusFileCKeyOpensConfirm(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"/repo"}
	m.cursor = 0
	m.focus = paneStatus
	m.statusFileSelected = true
	m.statusPaths = []string{"tracked.txt"}
	m.statusTable.SetRows([]table.Row{{"modified", "-", "tracked.txt"}})
	m.statusTable.SetCursor(0)

	_, _, handled := m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	if !handled {
		t.Fatal("C should be handled with status file selected")
	}
	if !m.checkoutStatusFileConfirmOpen || m.checkoutStatusFilePendingRel != "tracked.txt" {
		t.Fatalf("checkout confirm: open=%v pending=%q", m.checkoutStatusFileConfirmOpen, m.checkoutStatusFilePendingRel)
	}
	overlay := m.renderCheckoutStatusFileConfirmOverlay()
	if !strings.Contains(overlay, "Restore this path from the last commit") {
		t.Fatalf("overlay title missing: %q", overlay)
	}
	if !strings.Contains(overlay, "tracked.txt") {
		t.Fatalf("overlay should show relative path: %q", overlay)
	}

	mod, _ := m.handleCheckoutStatusFileConfirmKey(tea.KeyMsg{Type: tea.KeyEsc})
	if mod.(*model).checkoutStatusFileConfirmOpen {
		t.Fatal("esc should close checkout modal")
	}
}

// TestCheckoutStatusFileConfirmYesRestores runs git checkout HEAD after confirmation.
func TestCheckoutStatusFileConfirmYesRestores(t *testing.T) {
	repo := t.TempDir()
	runGit := func(arg ...string) {
		t.Helper()
		cmd := exec.Command("git", arg...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arg, err, out)
		}
	}
	runGit("init", "-b", "main")
	runGit("config", "user.email", "u@x")
	runGit("config", "user.name", "u")
	tracked := filepath.Join(repo, "hello.txt")
	if err := os.WriteFile(tracked, []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "hello.txt")
	runGit("commit", "-m", "init")
	if err := os.WriteFile(tracked, []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{repo}
	m.cursor = 0
	m.checkoutStatusFileConfirmOpen = true
	m.checkoutStatusFilePendingRel = "hello.txt"
	m.deleteConfirmYes = true

	m.handleCheckoutStatusFileConfirmKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.checkoutStatusFileConfirmOpen {
		t.Fatal("modal should close after confirm")
	}
	b, err := os.ReadFile(tracked)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "committed\n" {
		t.Fatalf("file content = %q, want committed version", string(b))
	}
}
