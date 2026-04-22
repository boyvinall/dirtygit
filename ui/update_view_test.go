package ui

import (
	"testing"

	cspinner "github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/boyvinall/dirtygit/scanner"
)

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
}

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

func TestHandleSpinnerTickWhenScanning(t *testing.T) {
	m := newTestModel()
	m.scanning = true
	m.scanSpinner = cspinner.New()

	_, cmd := m.handleSpinnerTick(cspinner.TickMsg{})
	if cmd == nil {
		t.Fatal("handleSpinnerTick should return spinner command while scanning")
	}
}
