package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/boyvinall/dirtygit/scanner"
)

func TestMouseRepoLineSelect(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"/a", "/b", "/c"}
	m.cursor = 0
	m.focus = paneRepo

	rb, _, _, _ := m.layoutBodies()
	repoOuter := panelOuter(rb)
	if repoOuter < 4 {
		t.Fatalf("repoOuter=%d too small for test", repoOuter)
	}
	// Inner line index 1 -> second repository (y = top border + 2)
	y := 2
	if y > repoOuter-2 {
		t.Fatalf("pick y inside inner body, repoOuter=%d", repoOuter)
	}
	msg := tea.MouseMsg{
		X:      2,
		Y:      y,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	ok, _ := m.handleMousePaneLineSelect(msg)
	if !ok {
		t.Fatal("expected click on second repo line to be handled")
	}
	if m.cursor != 1 {
		t.Fatalf("cursor=%d want 1", m.cursor)
	}
}

func TestMouseStatusLineSelect(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"/repo"}
	m.repositories = scanner.MultiGitStatus{
		"/repo": {
			Porcelain: scanner.PorcelainStatus{Entries: []scanner.PorcelainEntry{
				{Path: "a.go", Worktree: 'M', Staging: ' '},
				{Path: "b.go", Worktree: 'M', Staging: ' '},
				{Path: "c.go", Worktree: 'M', Staging: ' '},
			}},
		},
	}
	m.focus = paneStatus
	m.syncViewports()

	rb, _, _, _ := m.layoutBodies()
	statusTop := panelOuter(rb)
	// First data row: inner starts at statusTop+1, then header, then row 0.
	y := statusTop + 1 + statusTableHeaderLines(m.statusTable)
	msg := tea.MouseMsg{
		X:      2,
		Y:      y,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	if ok, _ := m.handleMousePaneLineSelect(msg); !ok {
		t.Fatal("expected click on first status data row to be handled")
	}
	if m.statusTable.Cursor() != 0 || !m.statusFileSelected {
		t.Fatalf("cursor=%d statusFileSelected=%v want 0,true", m.statusTable.Cursor(), m.statusFileSelected)
	}

	y2 := y + 1
	msg2 := tea.MouseMsg{X: 2, Y: y2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	if ok, _ := m.handleMousePaneLineSelect(msg2); !ok {
		t.Fatal("expected second row click handled")
	}
	if m.statusTable.Cursor() != 1 {
		t.Fatalf("cursor=%d want 1", m.statusTable.Cursor())
	}
}

func TestMouseStatusHeaderClickConsumed(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"/r"}
	m.repositories = scanner.MultiGitStatus{
		"/r": {
			Porcelain: scanner.PorcelainStatus{Entries: []scanner.PorcelainEntry{
				{Path: "x.go", Worktree: 'M', Staging: ' '},
			}},
		},
	}
	m.focus = paneStatus
	m.syncViewports()

	rb, _, _, _ := m.layoutBodies()
	statusTop := panelOuter(rb)
	y := statusTop + 1 // first inner line = table header
	prev := m.statusTable.Cursor()
	msg := tea.MouseMsg{X: 2, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	if ok, _ := m.handleMousePaneLineSelect(msg); !ok {
		t.Fatal("header click should be consumed")
	}
	if m.statusTable.Cursor() != prev {
		t.Fatal("clicking header should not move cursor")
	}
}
