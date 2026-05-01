package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// resizeSplit identifies which pane boundary is being mouse-dragged.
type resizeSplit int

const (
	resizeNone resizeSplit = iota
	resizeRepoStatus
	resizeStatusBranch
	resizeMiddleLog
	resizeMiddleColumns
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

	repoBody, statusBody, branchBody, diffBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && branchBody == 0 && diffBody == 0 && logBody == 0 {
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

func splitSumSBPreservingRatio(statusBody, branchBody, sumSB int) (st, br int) {
	pair := statusBody + branchBody
	if pair <= 0 || sumSB <= 0 {
		st = sumSB / 2
		br = sumSB - st
		return st, br
	}
	st = statusBody * sumSB / pair
	if st < layoutMinBodyLines {
		st = layoutMinBodyLines
	}
	if st > sumSB-layoutMinBodyLines {
		st = sumSB - layoutMinBodyLines
	}
	br = sumSB - st
	return st, br
}

func (m *model) applyResizeDrag(x, y int) {
	innerTotal := m.height - layoutFrameStackOuterRows
	repoBody, statusBody, branchBody, diffBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && branchBody == 0 && diffBody == 0 && logBody == 0 {
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
		logAdj := logBody
		available := innerTotal - repo - logAdj
		if available < layoutMinSpareForSplit {
			return
		}
		sumSB := available
		if sumSB < 2*layoutMinBodyLines {
			return
		}
		st, br := splitSumSBPreservingRatio(statusBody, branchBody, sumSB)
		di := diffBodyFromStackedStatusBranch(st, br)
		if st < layoutMinBodyLines || br < layoutMinBodyLines || di < layoutMinBodyLines {
			return
		}
		m.layoutUseCustomVertical = true
		m.layoutRepoBody = repo
		m.layoutStatusBody = st
		m.layoutBranchBody = br
		m.layoutLogBody = logAdj

	case resizeStatusBranch:
		sumSB := statusBody + branchBody
		repoOuter := panelOuter(repoBody)
		middleOuter := panelOuter(diffBody)
		prevStatusOuter := panelOuter(statusBody)
		var statusOuter int
		if y < repoOuter+prevStatusOuter {
			statusOuter = y - repoOuter + 1
		} else {
			statusOuter = y - repoOuter
		}
		minBranchOuter := panelOuter(layoutMinBodyLines)
		statusOuterMax := repoOuter + middleOuter - minBranchOuter
		if statusOuterMax < layoutMinPanelOuter {
			return
		}
		statusOuter = max(layoutMinPanelOuter, min(statusOuter, statusOuterMax))
		st := statusOuter - 2
		br := sumSB - st
		if br < layoutMinBodyLines || st < layoutMinBodyLines {
			return
		}
		m.layoutUseCustomVertical = true
		m.layoutRepoBody = repoBody
		m.layoutStatusBody = st
		m.layoutBranchBody = br
		m.layoutLogBody = logBody

	case resizeMiddleLog:
		repoOuter := panelOuter(repoBody)
		y0 := repoOuter
		prevMiddleOuter := panelOuter(diffBody)
		minLogOuter := panelOuter(layoutMinBodyLines)
		minMiddleOuter := panelOuter(diffBodyFromStackedStatusBranch(layoutMinBodyLines, layoutMinBodyLines))
		maxMiddleOuter := m.height - repoOuter - minLogOuter
		if maxMiddleOuter < minMiddleOuter {
			return
		}
		var middleOuter int
		if y <= y0+prevMiddleOuter-1 {
			middleOuter = y - y0 + 1
		} else {
			middleOuter = y - y0
		}
		middleOuter = max(minMiddleOuter, min(middleOuter, maxMiddleOuter))
		diff := middleOuter - 2
		sumSB := diff - 2
		if sumSB < 2*layoutMinBodyLines {
			return
		}
		st, br := splitSumSBPreservingRatio(statusBody, branchBody, sumSB)
		if diffBodyFromStackedStatusBranch(st, br) < layoutMinBodyLines {
			return
		}
		log := innerTotal - repoBody - st - br
		if log < layoutMinBodyLines {
			return
		}
		m.layoutUseCustomVertical = true
		m.layoutRepoBody = repoBody
		m.layoutStatusBody = st
		m.layoutBranchBody = br
		m.layoutLogBody = log

	case resizeMiddleColumns:
		if m.width < layoutMinTermWidth {
			return
		}
		leftOuter := x
		rightOuter := m.width - leftOuter
		if rightOuter < layoutMinStatusBranchesColumn {
			rightOuter = layoutMinStatusBranchesColumn
		}
		if rightOuter > m.width-layoutMinStatusBranchesColumn {
			rightOuter = m.width - layoutMinStatusBranchesColumn
		}
		if rightOuter < layoutMinStatusBranchesColumn || rightOuter > m.width-layoutMinStatusBranchesColumn {
			return
		}
		m.layoutBranchesOuter = rightOuter

	default:
		return
	}
}

// resizeSplitAt returns which resize handle (if any) lies at (x, y).
func (m *model) resizeSplitAt(x, y int) (resizeSplit, bool) {
	repoBody, statusBody, branchBody, diffBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && branchBody == 0 && diffBody == 0 && logBody == 0 {
		return resizeNone, false
	}
	repoOuter := panelOuter(repoBody)
	statusOuter := panelOuter(statusBody)
	middleOuter := panelOuter(diffBody)
	leftOuter, _ := m.middleRowColumnOuterWidths(m.width)

	if nearInt(y, repoOuter-1) || nearInt(y, repoOuter) {
		return resizeRepoStatus, true
	}
	yMidLog := repoOuter + middleOuter
	if nearInt(y, yMidLog-1) || nearInt(y, yMidLog) {
		return resizeMiddleLog, true
	}
	if x >= 0 && x < leftOuter && y >= repoOuter && y < repoOuter+middleOuter {
		yStatusBranch := repoOuter + statusOuter
		if nearInt(y, yStatusBranch-1) || nearInt(y, yStatusBranch) {
			return resizeStatusBranch, true
		}
	}
	if y >= repoOuter && y < repoOuter+middleOuter && x >= 0 {
		if nearInt(x, leftOuter-1) || nearInt(x, leftOuter) {
			return resizeMiddleColumns, true
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
