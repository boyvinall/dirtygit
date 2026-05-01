package ui

// paneLayout holds inner body heights for the main stack (one pane may be
// non-zero in zoom mode; all zero means layout could not be computed).
type paneLayout struct {
	repo, status, branch, diff, logBody int
}

func (p paneLayout) isZero() bool {
	return p.repo == 0 && p.status == 0 && p.branch == 0 && p.diff == 0 && p.logBody == 0
}

// innerVerticalBudget is total inner body lines available below the outer chrome
// (status bar, titles, etc.).
func innerVerticalBudget(termHeight int) int {
	return termHeight - layoutFrameStackOuterRows
}

// panelOuter converts an inner body height into full framed panel height.
func panelOuter(body int) int {
	return body + 2 // top border (with title) + body + bottom border
}

// diffBodyFromStackedStatusBranch returns diff inner body height so the Diff pane
// outer height matches Status and Branches stacked (each pane contributes top+bottom borders).
func diffBodyFromStackedStatusBranch(statusBody, branchBody int) int {
	return statusBody + branchBody + 2
}

// tightenRepoAndLogForMiddleSpare shrinks log then repo until the middle row has at
// least layoutMinSpareForSplit lines, or returns ok=false.
func tightenRepoAndLogForMiddleSpare(effH, repoBody, logBody int) (repo, log int, available int, ok bool) {
	repo, log = repoBody, logBody
	available = effH - layoutFrameStackOuterRows - repo - log
	for available < layoutMinSpareForSplit && log > layoutMinBodyLines {
		log--
		available = effH - layoutFrameStackOuterRows - repo - log
	}
	for available < layoutMinSpareForSplit && repo > layoutMinBodyLines {
		repo--
		available = effH - layoutFrameStackOuterRows - repo - log
	}
	if available < layoutMinSpareForSplit {
		return 0, 0, 0, false
	}
	return repo, log, available, true
}

// splitStatusBranchEvenly divides available inner lines between status and branch tables.
func splitStatusBranchEvenly(available int) (statusBody, branchBody int) {
	statusBody = available / 2
	branchBody = available - statusBody
	return statusBody, branchBody
}

// nonZoomStackBodiesValid checks minimum inner heights and that status+branch fill
// the middle vertical budget (diff height is derived from the stack).
func nonZoomStackBodiesValid(repo, status, branch, diff, logBody, available int) bool {
	if status < layoutMinBodyLines || branch < layoutMinBodyLines || diff < layoutMinBodyLines ||
		logBody < layoutMinBodyLines || repo < layoutMinBodyLines ||
		status+branch != available {
		return false
	}
	return true
}

// customVerticalLayoutOK validates user-resized vertical splits against the same
// invariants as layoutBodies.
func customVerticalLayoutOK(status, branch, diff, available int) bool {
	if available < layoutMinSpareForSplit || diff < layoutMinBodyLines ||
		status+branch != available ||
		panelOuter(status)+panelOuter(branch) != panelOuter(diff) {
		return false
	}
	return true
}

// clampDiffColumnOuterWidth maps a mouse X (left column outer width) to a valid
// Diff column outer width for the middle row.
func clampDiffColumnOuterWidth(termWidth, leftOuter int) (rightOuter int, ok bool) {
	if termWidth < layoutMinTermWidth {
		return 0, false
	}
	rightOuter = termWidth - leftOuter
	if rightOuter < layoutMinStatusBranchesColumn {
		rightOuter = layoutMinStatusBranchesColumn
	}
	if rightOuter > termWidth-layoutMinStatusBranchesColumn {
		rightOuter = termWidth - layoutMinStatusBranchesColumn
	}
	if rightOuter < layoutMinStatusBranchesColumn || rightOuter > termWidth-layoutMinStatusBranchesColumn {
		return 0, false
	}
	return rightOuter, true
}
