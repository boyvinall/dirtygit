package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestResizeSplitAtHorizontalSeams(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"a", "b", "c"}

	repoBody, _, _, diffBody, _ := m.layoutBodies()
	repoOuter := panelOuter(repoBody)
	middleOuter := panelOuter(diffBody)

	if k, ok := m.resizeSplitAt(0, repoOuter); !ok || k != resizeRepoStatus {
		t.Fatalf("resizeSplitAt repo seam y=%d = (%v,%v), want (resizeRepoStatus,true)", repoOuter, k, ok)
	}
	y1 := repoOuter + middleOuter
	if k, ok := m.resizeSplitAt(50, y1); !ok || k != resizeMiddleLog {
		t.Fatalf("resizeSplitAt middle/log y=%d = (%v,%v), want (resizeMiddleLog,true)", y1, k, ok)
	}
}

func TestResizeSplitAtVerticalBetweenStatusAndBranches(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"a", "b", "c"}

	repoBody, statusBody, _, _, _ := m.layoutBodies()
	repoOuter := panelOuter(repoBody)
	statusOuter := panelOuter(statusBody)
	leftW, _ := m.middleRowColumnOuterWidths(m.width)
	y := repoOuter + statusOuter - 1
	if k, ok := m.resizeSplitAt(leftW/2, y); !ok || k != resizeStatusBranch {
		t.Fatalf("resizeSplitAt status/branch seam = (%v,%v), want (resizeStatusBranch,true)", k, ok)
	}
}

func TestResizeSplitAtMiddleColumnDivider(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"a", "b", "c"}

	repoBody, _, _, diffBody, _ := m.layoutBodies()
	repoOuter := panelOuter(repoBody)
	leftW, _ := m.middleRowColumnOuterWidths(m.width)
	y := repoOuter + panelOuter(diffBody)/2
	if k, ok := m.resizeSplitAt(leftW, y); !ok || k != resizeMiddleColumns {
		t.Fatalf("resizeSplitAt left/diff column = (%v,%v), want (resizeMiddleColumns,true)", k, ok)
	}
}

func TestMouseDragResizesRepoPane(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.repoList = []string{"a", "b", "c"}

	before, _, _, _, _ := m.layoutBodies()
	repoOuter := panelOuter(before)
	press := tea.MouseMsg{
		X:      0,
		Y:      repoOuter,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	next0, _ := m.Update(press)
	mm0 := next0.(*model)
	if mm0.resizeDrag != resizeRepoStatus {
		t.Fatalf("expected resizeRepoStatus drag, got %v", mm0.resizeDrag)
	}
	motion := tea.MouseMsg{
		X:      0,
		Y:      repoOuter + 3,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	}
	next, _ := mm0.Update(motion)
	mm := next.(*model)
	after, _, _, _, _ := mm.layoutBodies()
	if after <= before {
		t.Fatalf("repo body after drag = %d, before = %d, expected larger", after, before)
	}
	if !mm.layoutUseCustomVertical {
		t.Fatal("expected layoutUseCustomVertical after resize")
	}
	release := tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease}
	if _, _ = mm.Update(release); mm.resizeDrag != resizeNone {
		t.Fatalf("expected resize drag cleared, got %v", mm.resizeDrag)
	}
}
