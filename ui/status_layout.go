package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5"

	"github.com/boyvinall/dirtygit/scanner"
)

// autoLayoutBodies returns the default vertical split without user resize prefs.
// Middle row: Status above Branches on the left, Diff on the right (same outer height as the stack).
// diffBody == statusBody+branchBody+2 so the Diff viewport fills the band; it is not added to the vertical body sum.
func (m *model) autoLayoutBodies() paneLayout {
	if m.height < layoutMinTermHeight || m.width < layoutMinTermWidth {
		return paneLayout{}
	}
	if m.zoomed {
		body := max(
			// panelOuter(body) == m.height
			m.height-2, layoutMinBodyLines)
		switch m.zoomTarget {
		case paneRepo:
			return paneLayout{repo: body}
		case paneStatus:
			return paneLayout{status: body}
		case paneBranches:
			return paneLayout{branch: body}
		case paneDiff:
			return paneLayout{diff: body}
		case paneLog:
			return paneLayout{logBody: body}
		default:
			return paneLayout{}
		}
	}
	effH := m.height
	logBody := layoutDefaultLogBodyLines
	n := len(m.repoList)
	if n == 0 {
		n = 1
	}
	repoBody := min(n+2, effH/3)
	repoBody = max(layoutMinBodyLines, repoBody)

	repoBody, logBody, available, ok := tightenRepoAndLogForMiddleSpare(effH, repoBody, logBody)
	if !ok {
		return paneLayout{}
	}
	statusBody, branchBody := splitStatusBranchEvenly(available)
	diffBody := diffBodyFromStackedStatusBranch(statusBody, branchBody)
	if !nonZoomStackBodiesValid(repoBody, statusBody, branchBody, diffBody, logBody, available) {
		return paneLayout{}
	}
	return paneLayout{repo: repoBody, status: statusBody, branch: branchBody, diff: diffBody, logBody: logBody}
}

func (m *model) clearCustomVerticalLayout() {
	m.layoutUseCustomVertical = false
	m.layoutRepoBody, m.layoutStatusBody, m.layoutBranchBody, m.layoutLogBody = 0, 0, 0, 0
}

// layoutBodies returns inner content heights: repo list, status table, branch table, diff viewport, log viewport.
// Each framed panel's total row count is panelOuter(body) (see framedBlock).
func (m *model) layoutBodies() paneLayout {
	auto := m.autoLayoutBodies()
	if auto.isZero() {
		return paneLayout{}
	}
	if m.zoomed || !m.layoutUseCustomVertical {
		return auto
	}
	innerTotal := innerVerticalBudget(m.height)
	repoBody := m.layoutRepoBody
	statusBody := m.layoutStatusBody
	branchBody := m.layoutBranchBody
	logBody := m.layoutLogBody
	if repoBody < layoutMinBodyLines || statusBody < layoutMinBodyLines || branchBody < layoutMinBodyLines || logBody < layoutMinBodyLines {
		m.clearCustomVerticalLayout()
		return auto
	}
	available := innerTotal - repoBody - logBody
	diffBody := diffBodyFromStackedStatusBranch(statusBody, branchBody)
	if !customVerticalLayoutOK(statusBody, branchBody, diffBody, available) {
		m.clearCustomVerticalLayout()
		return auto
	}
	return paneLayout{repo: repoBody, status: statusBody, branch: branchBody, diff: diffBody, logBody: logBody}
}

// paneAtTerminalCell maps a 0-based terminal cell (from Bubble Tea mouse events)
// to the pane that contains it. It mirrors renderMainStack / renderZoomedPane geometry.
func (m *model) paneAtTerminalCell(x, y int) (pane, bool) {
	if m.width <= 0 || m.height < layoutMinTermHeight {
		return paneRepo, false
	}
	if x < 0 || y < 0 || x >= m.width || y >= m.height {
		return paneRepo, false
	}
	lay := m.layoutBodies()
	if lay.isZero() {
		return paneRepo, false
	}
	if m.zoomed {
		return m.zoomTarget, true
	}
	repoOuter := panelOuter(lay.repo)
	statusOuter := panelOuter(lay.status)
	middleOuter := panelOuter(lay.diff)
	logOuter := panelOuter(lay.logBody)
	leftOuter, _ := m.middleRowColumnOuterWidths(m.width)

	if y < repoOuter {
		return paneRepo, true
	}
	y -= repoOuter
	if y < middleOuter {
		if x < leftOuter {
			if y < statusOuter {
				return paneStatus, true
			}
			return paneBranches, true
		}
		return paneDiff, true
	}
	y -= middleOuter
	if y < logOuter {
		return paneLog, true
	}
	return paneRepo, false
}

// innerWidth returns content width available inside pane borders.
func (m *model) innerWidth() int {
	w := m.width - 4
	if w < layoutMinInnerContentWidth {
		w = max(layoutMinInnerContentWidth, m.width-2)
	}
	return w
}

// middleRowColumnOuterWidths splits the middle row: left (Status+Branches stack) vs right (Diff).
func (m *model) middleRowColumnOuterWidths(total int) (leftOuter, rightOuter int) {
	if m.layoutBranchesOuter > 0 {
		right := m.layoutBranchesOuter
		right = max(layoutMinStatusBranchesColumn, min(right, total-layoutMinStatusBranchesColumn))
		return total - right, right
	}
	rawRight := total * layoutDefaultMiddleDiffColumnPct / 100
	rightOuter = max(layoutMinStatusBranchesColumn, min(rawRight, total-layoutMinStatusBranchesColumn))
	leftOuter = total - rightOuter
	return leftOuter, rightOuter
}

// middleRowColumnInnerWidths returns content widths inside the left stack panes and the Diff pane.
func (m *model) middleRowColumnInnerWidths() (leftInnerW, rightInnerW int) {
	leftOuterW, rightOuterW := m.middleRowColumnOuterWidths(m.width)
	leftInnerW = max(layoutMinInnerContentWidth, leftOuterW-2)
	rightInnerW = max(layoutMinInnerContentWidth, rightOuterW-2)
	return leftInnerW, rightInnerW
}

// syncViewports applies layout dimensions and refreshes all pane content,
// including running git when diffNeedsRefresh (expensive).
func (m *model) syncViewports() {
	m.applyViewportAndPanes(true)
}

// setLogVPContent refreshes the log viewport from the buffer. If the view was
// already at the bottom, it stays pinned so new log lines stay visible; if the
// user has scrolled up, the scroll position is preserved.
func (m *model) setLogVPContent() {
	wasAtBottom := m.logVP.AtBottom()
	m.logVP.SetContent(m.logBuf.String())
	if wasAtBottom {
		m.logVP.GotoBottom()
	}
}

// applyViewportAndPanes resizes panes and refreshes status, branches, log, and
// repo scroll. If syncDiff is true, it runs refreshDiffContent (git diff).
// If false, it defers the diff: when diffNeedsRefresh, the diff pane shows
// a loading line until a follow-up runDiffForGen is handled. Used when
// the repository selection changes so scrolling stays responsive.
func (m *model) applyViewportAndPanes(syncDiff bool) {
	lay := m.layoutBodies()
	if lay.isZero() {
		return
	}

	statusInnerW := m.innerWidth()
	branchInnerW := statusInnerW
	diffInnerW := statusInnerW
	if !m.zoomed {
		statusInnerW, diffInnerW = m.middleRowColumnInnerWidths()
		branchInnerW = statusInnerW
	}

	m.statusTable.SetWidth(statusInnerW)
	m.statusTable.SetHeight(lay.status)
	m.statusTable.SetColumns(statusColumns(statusInnerW))
	m.logVP.Width = m.innerWidth()
	m.logVP.Height = lay.logBody
	m.diffVP.Width = diffInnerW
	m.diffVP.Height = lay.diff
	m.refreshStatusContent()
	m.refreshBranchContent(branchInnerW)
	m.branchTable.SetHeight(lay.branch)
	if syncDiff {
		m.refreshDiffContent()
	} else if m.diffNeedsRefresh {
		m.diffContent = "(Loading diff…)"
	}
	m.diffVP.SetContent(m.diffContent)
	m.setLogVPContent()
	m.clampRepoScroll(lay.repo)
}

// newStatusTable builds the status pane table with default styling.
func newStatusTable() table.Model {
	t := table.New(
		table.WithColumns(statusColumns(48)),
		table.WithRows(nil),
		table.WithFocused(false),
		table.WithHeight(layoutDefaultTableViewRows),
	)
	t.SetStyles(statusTableStyles(false))
	return t
}

// statusTableStyles returns table styles for the status pane. The bubbles table
// always applies Selected to the cursor row; it does not dim that style when
// Blur() is called, so we swap Selected to match repo-list behavior (green when
// this pane owns selection, grey when not).
func statusTableStyles(selectionFocused bool) table.Styles {
	s := table.DefaultStyles()
	s.Selected = tableSelectedRow(selectionFocused)
	return s
}

// applyStatusTableFocusAndStyles syncs table focus and cursor-row styling with
// whether the status pane is actively selecting a file.
func (m *model) applyStatusTableFocusAndStyles() {
	selectionFocused := m.focus == paneStatus && m.statusFileSelected
	if selectionFocused {
		m.statusTable.Focus()
	} else {
		m.statusTable.Blur()
	}
	m.statusTable.SetStyles(statusTableStyles(selectionFocused))
}

// newBranchTable builds the branch pane table with default styling.
func newBranchTable() table.Model {
	t := table.New(
		table.WithColumns(branchRowColumns(48)),
		table.WithRows(nil),
		table.WithFocused(false),
		table.WithHeight(layoutDefaultTableViewRows),
	)
	return t
}

// statusColumns computes table column widths for the given pane width.
// Bubbles table Header and Cell styles use Padding(0, 1), so each column adds
// two cells to the rendered row; totals must stay within totalWidth.
func statusColumns(totalWidth int) []table.Column {
	stagedWidth := layoutStatusWorktreeStagedNarrowCol
	worktreeWidth := layoutStatusWorktreeStagedNarrowCol
	const cols = 3
	horizontalPad := 2 * cols
	pathWidth := max(1, totalWidth-stagedWidth-worktreeWidth-horizontalPad)
	return []table.Column{
		{Title: "Worktree", Width: worktreeWidth},
		{Title: "Staged", Width: stagedWidth},
		{Title: "Path", Width: pathWidth},
	}
}

// branchRowColumns sizes the branch pane: one row per local branch name.
// Account for Padding(0, 1) on every header and cell (see statusColumns).
func branchRowColumns(totalWidth int) []table.Column {
	const cols = 4
	horizontalPad := 2 * cols
	commitW := 10
	tipW := 8
	rest := totalWidth - horizontalPad - commitW - tipW
	if rest < 2 {
		commitW = max(4, min(8, totalWidth/4))
		tipW = max(4, min(6, totalWidth/5))
		rest = totalWidth - horizontalPad - commitW - tipW
	}
	rest = max(1, rest)
	branchW := max(1, rest/2)
	remotesW := max(1, rest-branchW)
	return []table.Column{
		{Title: "", Width: branchW},
		{Title: "Commit", Width: commitW},
		{Title: "Tip age", Width: tipW},
		{Title: "Remotes", Width: remotesW},
	}
}

// branchRemoteSummary compresses local vs remote tips for the checked-out branch
// when BranchStatus.Locations is populated (e.g. no local heads listed).
func branchRemoteSummary(b scanner.BranchStatus) string {
	if b.Detached || len(b.Locations) == 0 {
		return "-"
	}
	return branchRemoteSummaryFromLocations(b.Locations)
}

func branchRemoteSummaryFromLocations(locations []scanner.BranchLocation) string {
	if len(locations) == 0 {
		return "-"
	}
	var local *scanner.BranchLocation
	for i := range locations {
		if locations[i].Name == "local" {
			local = &locations[i]
			break
		}
	}
	if local == nil || !local.Exists {
		return "-"
	}
	parts := make([]string, 0, len(locations))
	for _, loc := range locations {
		if loc.Name == "local" {
			continue
		}
		if !loc.Exists {
			parts = append(parts, loc.Name+": missing")
			continue
		}
		if loc.TipHash != local.TipHash {
			if loc.HistoriesUnrelated {
				parts = append(parts, loc.Name+": differs")
				continue
			}
			switch {
			case loc.Incoming > 0 && loc.Outgoing > 0:
				parts = append(parts, fmt.Sprintf("%s +%d-%d", loc.Name, loc.Incoming, loc.Outgoing))
			case loc.Incoming > 0:
				parts = append(parts, fmt.Sprintf("%s +%d", loc.Name, loc.Incoming))
			case loc.Outgoing > 0:
				parts = append(parts, fmt.Sprintf("%s -%d", loc.Name, loc.Outgoing))
			default:
				parts = append(parts, loc.Name+": differs")
			}
			continue
		}
		parts = append(parts, loc.Name+": ok")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// sortLocalBranchesByTipNewestFirst orders branches for the table: latest tip commit first.
// Tie-breaker is name so order is stable when tips share a timestamp.
func sortLocalBranchesByTipNewestFirst(branches []scanner.LocalBranchRef) {
	sort.SliceStable(branches, func(i, j int) bool {
		ui, uj := branches[i].TipUnix, branches[j].TipUnix
		if ui != uj {
			return ui > uj
		}
		return branches[i].Name < branches[j].Name
	})
}

// refreshBranchContent rebuilds the branch pane: one table row per local branch name.
func (m *model) refreshBranchContent(totalWidth int) {
	cols := branchRowColumns(totalWidth)
	m.branchTable.SetColumns(cols)

	repo := m.currentRepo()
	st, ok := m.repositories.Get(repo)
	if !ok {
		m.branchTable.SetRows([]table.Row{{"(select repository)", "-", "-", "-"}})
		m.branchTable.SetHeight(layoutMinBodyLines)
		return
	}
	branch := st.Branches

	if branch.Detached {
		locals := append([]scanner.LocalBranchRef(nil), branch.LocalBranches...)
		sortLocalBranchesByTipNewestFirst(locals)
		rows := make([]table.Row, 0, 1+len(locals))
		rows = append(rows, table.Row{"(detached HEAD)", shortHash(branch.Branch), "-", "-"})
		for _, lb := range locals {
			always := m.config != nil && m.config.AlwaysListBranch(lb.Name)
			if m.config != nil && m.config.ShouldHideLocalOnlyBranch(lb) && !always {
				continue
			}
			rows = append(rows, table.Row{
				lb.Name,
				shortHash(lb.TipHash),
				relativeTime(lb.TipUnix),
				"-",
			})
		}
		m.branchTable.SetRows(rows)
		m.branchTable.SetHeight(max(4, len(rows)+1))
		return
	}

	if len(branch.LocalBranches) == 0 {
		remote := branchRemoteSummary(branch)
		tip := "-"
		when := "-"
		for _, loc := range branch.Locations {
			if loc.Name == "local" && loc.Exists {
				tip = shortHash(loc.TipHash)
				when = relativeTime(loc.TipUnix)
				break
			}
		}
		m.branchTable.SetRows([]table.Row{{branch.Branch, tip, when, remote}})
		m.branchTable.SetHeight(4)
		return
	}

	locals := append([]scanner.LocalBranchRef(nil), branch.LocalBranches...)
	sortLocalBranchesByTipNewestFirst(locals)
	rows := make([]table.Row, 0, len(locals))
	for _, lb := range locals {
		always := m.config != nil && m.config.AlwaysListBranch(lb.Name)
		if m.config != nil && m.config.ShouldHideLocalOnlyBranch(lb) && !always {
			continue
		}
		if !lb.HasTipMismatchAcrossRemotes() && !always {
			continue
		}
		remote := branchRemoteSummaryFromLocations(lb.Locations)
		rows = append(rows, table.Row{
			lb.Name,
			shortHash(lb.TipHash),
			relativeTime(lb.TipUnix),
			remote,
		})
	}
	if len(rows) == 0 {
		rows = []table.Row{{"(in sync with remotes)", "-", "-", "-"}}
	}
	m.branchTable.SetRows(rows)
	m.branchTable.SetHeight(max(4, len(rows)+1))
}

func shortHash(hash string) string {
	if len(hash) <= 8 {
		return hash
	}
	return hash[:8]
}

func relativeTime(unix int64) string {
	if unix <= 0 {
		return "-"
	}
	d := time.Since(time.Unix(unix, 0))
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// refreshStatusContent rebuilds status rows for the selected repository.
func (m *model) refreshStatusContent() {
	repo := m.currentRepo()
	st, ok := m.repositories.Get(repo)
	rows := make([]table.Row, 0)
	paths := make([]string, 0)
	if ok && len(st.Porcelain.Entries) > 0 {
		entries := append([]scanner.PorcelainEntry(nil), st.Porcelain.Entries...)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Path < entries[j].Path
		})
		for _, entry := range entries {
			path := entry.Path
			if entry.OriginalPath != "" {
				path = fmt.Sprintf("%s -> %s", entry.OriginalPath, entry.Path)
			}
			rows = append(rows, table.Row{
				statusCodeLabel(entry.Worktree),
				statusCodeLabel(entry.Staging),
				path,
			})
			paths = append(paths, entry.Path)
		}
	} else if ok && len(st.Status) > 0 {
		// Fallback for statuses that do not include parsed porcelain data.
		paths := make([]string, 0, len(st.Status))
		for path := range st.Status {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			status := st.Status[path]
			rows = append(rows, table.Row{
				statusCodeLabel(status.Worktree),
				statusCodeLabel(status.Staging),
				path,
			})
			paths = append(paths, path)
		}
	}
	m.statusPaths = paths
	if len(paths) == 0 {
		m.statusFileSelected = false
	}
	m.statusTable.SetRows(rows)
	if len(rows) > 0 && m.statusTable.Cursor() >= len(rows) {
		m.statusTable.SetCursor(len(rows) - 1)
	}
	m.applyStatusTableFocusAndStyles()
}

// selectedStatusPath returns the currently highlighted file path.
func (m *model) selectedStatusPath() string {
	if !m.statusFileSelected {
		return ""
	}
	i := m.statusTable.Cursor()
	if i < 0 || i >= len(m.statusPaths) {
		return ""
	}
	return m.statusPaths[i]
}

// statusFileOpsFocused is true when shortcuts that operate on the selected
// status row apply (Status or Diff pane with an active file selection).
func (m *model) statusFileOpsFocused() bool {
	return m.statusFileSelected && (m.focus == paneStatus || m.focus == paneDiff)
}

// selectedStatusPathForOps returns the repo-relative path when
// statusFileOpsFocused holds and a row is selected; otherwise ("", false).
func (m *model) selectedStatusPathForOps() (path string, ok bool) {
	if !m.statusFileOpsFocused() {
		return "", false
	}
	path = m.selectedStatusPath()
	if path == "" {
		return "", false
	}
	return path, true
}

// statusCodeLabel maps git status codes to human-friendly labels.
func statusCodeLabel(code git.StatusCode) string {
	switch code {
	case 'M':
		return "modified"
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'U':
		return "unmerged"
	case '?':
		return "untracked"
	case '!':
		return "ignored"
	case ' ':
		return "-"
	default:
		return string(code)
	}
}

// currentRepo returns the repository currently selected in the list.
func (m *model) currentRepo() string {
	if m.cursor < 0 || m.cursor >= len(m.repoList) {
		return ""
	}
	return m.repoList[m.cursor]
}

// repoPaneReady is true when the Repo pane is focused and the list is usable.
func (m *model) repoPaneReady() bool {
	return m.focus == paneRepo && m.err == nil && len(m.repoList) > 0
}

// interactiveAppReady is true when the main TUI (not a modal) is on screen and idle.
func (m *model) interactiveAppReady() bool {
	return !m.helpOpen && !m.deleteRepoConfirmOpen && !m.deleteStatusFileConfirmOpen &&
		!m.checkoutStatusFileConfirmOpen && !m.whyRepoOpen && !m.scanning && m.err == nil
}

// mouseFocusClickReady is true when left-click to change pane focus is allowed.
func (m *model) mouseFocusClickReady() bool {
	return m.interactiveAppReady() && m.height >= layoutMinTermHeight
}

// mousePaneResizeReady is true when split border drag-to-resize may run.
func (m *model) mousePaneResizeReady() bool {
	return m.interactiveAppReady() && !m.zoomed && m.height >= layoutMinTermHeight && m.width >= layoutMinTermWidth
}

// borderFor picks a heavier border for the active pane (used by framedBlock).
func borderFor(p, active pane) lipgloss.Border {
	if p == active {
		return lipgloss.ThickBorder()
	}
	return lipgloss.NormalBorder()
}

// sectionTitle highlights pane titles when the pane is focused.
func (m *model) sectionTitle(p pane, label string) string {
	return focusedSectionTitle(m.focus == p, label)
}
