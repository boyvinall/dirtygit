package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// resizeSplit identifies which pane boundary is being mouse-dragged.
type resizeSplit int

const (
	resizeNone resizeSplit = iota
	resizeRepoStatus
	resizeStatusDiff
	resizeDiffLog
	resizeStatusBranches
)

// handleMousePaneResize handles click-drag on pane borders to resize splits.
// Returns true when the message is consumed.
func (m *model) handleMousePaneResize(msg tea.MouseMsg) bool {
	if !m.mousePaneResizeReady() {
		if msg.Action == tea.MouseActionRelease {
			m.resizeDrag = resizeNone
		}
		return false
	}

	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && diffBody == 0 && logBody == 0 {
		return false
	}

	switch msg.Action {
	case tea.MouseActionRelease:
		if m.resizeDrag != resizeNone {
			m.resizeDrag = resizeNone
			m.syncViewports()
			return true
		}
		return false

	case tea.MouseActionMotion:
		if m.resizeDrag == resizeNone {
			return false
		}
		if msg.Button != tea.MouseButtonLeft {
			return false
		}
		m.applyResizeDrag(msg.X, msg.Y)
		m.syncViewports()
		return true

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			return false
		}
		kind, ok := m.resizeSplitAt(msg.X, msg.Y)
		if !ok {
			return false
		}
		m.resizeDrag = kind
		m.applyResizeDrag(msg.X, msg.Y)
		m.syncViewports()
		return true

	default:
		return false
	}
}

func (m *model) applyResizeDrag(x, y int) {
	innerTotal := m.height - layoutFrameStackOuterRows
	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && diffBody == 0 && logBody == 0 {
		return
	}

	switch m.resizeDrag {
	case resizeRepoStatus:
		prevRepoOuter := panelOuter(repoBody)
		var repoOuter int
		if y < prevRepoOuter {
			repoOuter = y + 1
		} else {
			repoOuter = y
		}
		repoOuterMax := m.height - panelOuter(logBody) - layoutRepoOuterMaxBottomMargin
		if repoOuterMax < layoutMinPanelOuter {
			return
		}
		repoOuter = max(layoutMinPanelOuter, min(repoOuter, repoOuterMax))
		repo := repoOuter - 2
		if repo < layoutMinBodyLines {
			return
		}
		available := innerTotal - repo - logBody
		if available < layoutMinSpareForSplit {
			return
		}
		st := statusBody
		di := available - st
		if di < layoutMinBodyLines {
			di = layoutMinBodyLines
			st = available - di
		}
		if st < layoutMinBodyLines {
			st = layoutMinBodyLines
			di = available - st
		}
		if di < layoutMinBodyLines || st < layoutMinBodyLines {
			return
		}
		m.layoutUseCustomVertical = true
		m.layoutRepoBody = repo
		m.layoutStatusBody = st
		m.layoutLogBody = logBody

	case resizeStatusDiff:
		repoOuter := panelOuter(repoBody)
		prevStatusOuter := panelOuter(statusBody)
		var statusOuter int
		if y < repoOuter+prevStatusOuter {
			statusOuter = y - repoOuter + 1
		} else {
			statusOuter = y - repoOuter
		}
		statusOuterMax := innerTotal - repoBody - logBody - 1
		if statusOuterMax < layoutMinPanelOuter {
			return
		}
		statusOuter = max(layoutMinPanelOuter, min(statusOuter, statusOuterMax))
		status := statusOuter - 2
		if status < layoutMinBodyLines {
			return
		}
		available := innerTotal - repoBody - logBody
		diff := available - status
		if diff < layoutMinBodyLines {
			return
		}
		m.layoutUseCustomVertical = true
		m.layoutRepoBody = repoBody
		m.layoutStatusBody = status
		m.layoutLogBody = logBody

	case resizeDiffLog:
		repoOuter := panelOuter(repoBody)
		y0 := repoOuter + panelOuter(statusBody)
		prevDiffOuter := panelOuter(diffBody)
		minLogOuter := panelOuter(layoutMinBodyLines)
		maxDiffOuter := m.height - repoOuter - panelOuter(statusBody) - minLogOuter
		if maxDiffOuter < minLogOuter {
			return
		}
		var diffOuter int
		if y <= y0+prevDiffOuter-1 {
			diffOuter = y - y0 + 1
		} else {
			diffOuter = y - y0
		}
		diffOuter = max(minLogOuter, min(diffOuter, maxDiffOuter))
		diff := diffOuter - 2
		log := innerTotal - repoBody - statusBody - diff
		if log < layoutMinBodyLines {
			return
		}
		m.layoutUseCustomVertical = true
		m.layoutRepoBody = repoBody
		m.layoutStatusBody = statusBody
		m.layoutLogBody = log

	case resizeStatusBranches:
		if m.width < layoutMinTermWidth {
			return
		}
		statusOuter := x
		branches := m.width - statusOuter
		if branches < layoutMinStatusBranchesColumn {
			branches = layoutMinStatusBranchesColumn
		}
		if branches > m.width-layoutMinStatusBranchesColumn {
			branches = m.width - layoutMinStatusBranchesColumn
		}
		if branches < layoutMinStatusBranchesColumn || branches > m.width-layoutMinStatusBranchesColumn {
			return
		}
		m.layoutBranchesOuter = branches

	default:
		return
	}
}

// resizeSplitAt returns which resize handle (if any) lies at (x, y).
func (m *model) resizeSplitAt(x, y int) (resizeSplit, bool) {
	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && diffBody == 0 && logBody == 0 {
		return resizeNone, false
	}
	repoOuter := panelOuter(repoBody)
	statusOuter := panelOuter(statusBody)
	diffOuter := panelOuter(diffBody)
	statusW, _ := m.statusBranchesOuterWidths(m.width)

	if nearInt(y, repoOuter-1) || nearInt(y, repoOuter) {
		return resizeRepoStatus, true
	}
	y1 := repoOuter + statusOuter
	if nearInt(y, y1-1) || nearInt(y, y1) {
		return resizeStatusDiff, true
	}
	y2 := repoOuter + statusOuter + diffOuter
	if nearInt(y, y2-1) || nearInt(y, y2) {
		return resizeDiffLog, true
	}
	if x >= 0 && x < m.width && y >= repoOuter && y < repoOuter+statusOuter {
		if nearInt(x, statusW-1) || nearInt(x, statusW) {
			return resizeStatusBranches, true
		}
	}
	return resizeNone, false
}

func nearInt(a, b int) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= mouseBorderHitSlop
}
