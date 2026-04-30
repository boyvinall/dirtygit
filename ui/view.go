package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/boyvinall/dirtygit/scanner"
)

// placeCenteredDimModal centers content on the full terminal with a dim tiled backdrop.
func (m *model) placeCenteredDimModal(content string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content,
		lipgloss.WithWhitespaceChars("░"),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
		lipgloss.WithWhitespaceBackground(lipgloss.Color("235")))
}

// placeCenteredPlain centers content on the full terminal with a simple space fill (e.g. errors).
func (m *model) placeCenteredPlain(content string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
}

// helpKey reports whether a key toggles the help overlay.
func helpKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "?", "h", "shift+/":
		return true
	default:
		return false
	}
}

// scanProgressBar renders a fixed-width bar for scan completion progress.
func scanProgressBar(width, checked, found int) string {
	if width < 1 {
		return ""
	}
	d := max(found, 1)
	filled := min(checked*width/d, width)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// shortenScanPath truncates long scan paths from the left.
func shortenScanPath(path string, max int) string {
	if max < layoutMinInnerContentWidth || path == "" || len(path) <= max {
		return path
	}
	return "…" + path[len(path)-(max-layoutPathTruncationEllipsis):]
}

// scanModalInnerLines is the fixed content row count inside the scan popup (excluding border/padding).
const scanModalInnerLines = 9

// truncateASCII truncates a string and appends an ellipsis.
func truncateASCII(s string, max int) string {
	if max < 2 || len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// scanProgressPopup renders the centered modal shown while scanning.
func (m *model) scanProgressPopup() string {
	p := m.scanProgress
	boxW := min(m.width-layoutModalSideGutter, layoutScanProgressModalMaxBox)
	if boxW < layoutModalBoxMinWidth {
		boxW = min(m.width-2, layoutModalBoxMinWidth)
	}
	// Inner text width: border (2) + horizontal padding (4) is a safe shave from boxW.
	innerW := max(layoutMinInnerContentWidth, boxW-layoutModalSideGutter)
	bar := scanProgressBar(innerW, p.ReposChecked, max(p.ReposFound, 1))

	line := fmt.Sprintf("Found %d repo(s)  ·  checked git status %d", p.ReposFound, p.ReposChecked)
	line = truncateASCII(line, max(4, innerW-4))

	spin := lipgloss.NewStyle().Width(2).MaxWidth(2).Align(lipgloss.Center).Render(m.scanSpinner.View())
	row1 := lipgloss.JoinHorizontal(lipgloss.Left, spin, " ", line)
	row1 = placeSpace(innerW, 1, row1)

	pathText := shortenScanPath(p.CurrentPath, innerW)
	if pathText == "" {
		pathText = styleDim.Render("—")
	}
	pathRow := placeSpace(innerW, 1, pathText)

	title := placeSpace(innerW, 1, truncateASCII("Scanning repositories", innerW))
	footer := placeSpace(innerW, 1, truncateASCII("Please wait...", innerW))

	body := strings.Join([]string{
		title,
		"",
		row1,
		"",
		bar,
		"",
		pathRow,
		"",
		footer,
	}, "\n")

	body = placeSpace(innerW, scanModalInnerLines, body)

	return roundedModal(boxW).Render(body)
}

// helpPanel renders keyboard shortcut documentation in a frame that fills the terminal.
func (m *model) helpPanel() string {
	lines := []string{
		"Click         Focus a pane; in Repositories or Status (when focused), select a row",
		"Drag (border) Resize adjacent panes (unavailable when zoomed, scanning, on error, or with an overlay open)",
		"Tab           Next pane: Repositories → Status → Branches → Diff → Log; when zoomed, cycle which pane is fullscreen",
		"Shift+Tab     Previous pane; when zoomed, cycle backward",
		"Enter         Zoom focused pane; Enter again restores the split layout",
		"Esc           Exit zoom, or clear Status file selection; also closes this help",
		"↑ / ↓         Move repo selection or scroll Status / Diff / Log",
		"Shift+↑/↓     Same, in steps of 10 lines",
		"← / →         In Status or Diff: switch Worktree vs Staged diff",
		"a  r          With a file row selected (Status or Diff): git add / git reset (unstage) that path",
		"C             With a file row selected (Status or Diff): restore file to last commit (confirms git checkout HEAD -- path)",
		"s             Scan / rescan",
		"e             Open selected repository (edit.command in config)",
		"t             Open a new terminal in the selected repo (from TERM_PROGRAM when known)",
		"w             Repositories focused: why this repository is in the list",
		"D             Repo list: delete the repository directory (confirm)",
		"              Status or Diff with a file row: delete that path under the repo (confirm)",
		"q  Ctrl+C     Quit (works from this overlay too)",
		"?  h          Toggle this help (also closes with ? or h)",
		"",
		"From help: Esc, ?, or h closes · q / Ctrl+C quits the app.",
	}
	body := strings.Join(lines, "\n")
	w, h := m.width, m.height
	if h < 1 {
		h = layoutMinTermHeight
	}
	innerW := max(1, w-2)
	innerH := max(1, h-2)
	padded := placeSpace(innerW, innerH, body)
	// Match other panes: title sits in the top border (see framedBlock).
	// Use paneRepo so Diff's framedBlock title override (worktree/staged) does not replace this title.
	return m.framedBlock(paneRepo, w, h, "Keyboard shortcuts", padded)
}

// renderWhyInclusionOverlay shows why the selected repository appears in the list.
func (m *model) renderWhyInclusionOverlay() string {
	repo := m.currentRepo()
	boxW := min(m.width-layoutModalSideGutter, layoutWhyAndConfirmModalMaxBox)
	if boxW < layoutModalBoxMinWidth {
		boxW = min(m.width-2, layoutModalBoxMinWidth)
	}
	innerW := max(layoutMinInnerContentWidth, boxW-layoutModalSideGutter)
	if repo == "" {
		return m.placeCenteredDimModal(roundedModal(boxW).Render("No repository selected."))
	}
	rs, ok := m.repositories[repo]
	if !ok {
		return m.placeCenteredDimModal(roundedModal(boxW).Render("No status data for this path."))
	}
	lines := scanner.RepoInclusionReasons(rs)
	if len(lines) == 0 {
		lines = []string{"No inclusion reason (unexpected for a listed repository)."}
	}
	reasons := strings.Join(lines, "\n\n")
	wrapped := lipgloss.NewStyle().Width(innerW).MaxWidth(innerW).Render(reasons)
	t := styleBold.Render("Why is this repository listed?")
	sub := styleDim.Render(truncateASCII(repo, innerW))
	foot := styleDim.Render("w or Esc to close")
	inner := strings.Join([]string{t, "", sub, "", wrapped, "", foot}, "\n")
	return m.placeCenteredDimModal(roundedModal(boxW).Render(inner))
}

// renderDeleteRepoConfirmOverlay asks whether to recursively delete the selected repository directory.
func (m *model) renderDeleteRepoConfirmOverlay() string {
	repo := m.currentRepo()
	boxW := min(m.width-layoutModalSideGutter, layoutWhyAndConfirmModalMaxBox)
	if boxW < layoutModalBoxMinWidth {
		boxW = min(m.width-2, layoutModalBoxMinWidth)
	}
	innerW := max(layoutMinInnerContentWidth, boxW-layoutModalSideGutter)
	if repo == "" {
		return m.placeCenteredDimModal(roundedModal(boxW).Render("No repository selected."))
	}
	pathLine := styleDim.Render(truncateASCII(repo, innerW))
	t := styleBold.Render("Delete this directory recursively?")
	warn := warnBlock(innerW).Render("This will remove the folder and all of its contents. This cannot be undone.")
	btns := deleteConfirmButtons(m.deleteConfirmYes)
	inner := strings.Join([]string{t, "", pathLine, "", warn, "", btns, "", deleteConfirmFooter()}, "\n")
	return m.placeCenteredDimModal(roundedModal(boxW).Render(inner))
}

// renderDeleteStatusFileConfirmOverlay asks before deleting the selected status path from disk.
func (m *model) renderDeleteStatusFileConfirmOverlay() string {
	repo := m.currentRepo()
	boxW := min(m.width-layoutModalSideGutter, layoutWhyAndConfirmModalMaxBox)
	if boxW < layoutModalBoxMinWidth {
		boxW = min(m.width-2, layoutModalBoxMinWidth)
	}
	innerW := max(layoutMinInnerContentWidth, boxW-layoutModalSideGutter)
	if repo == "" || m.deleteStatusFilePendingRel == "" {
		return m.placeCenteredDimModal(roundedModal(boxW).Render("Nothing to delete."))
	}
	absPath, err := statusPathUnderRepo(repo, m.deleteStatusFilePendingRel)
	repoLine := styleDim.Render(truncateASCII(repo, innerW))
	relLine := styleDim.Render(truncateASCII(m.deleteStatusFilePendingRel, innerW))
	t := styleBold.Render("Delete this file or directory from disk?")
	warn := warnBlock(innerW).Render("This removes the path from your working tree (not only from git's index). This cannot be undone.")
	if err != nil {
		warn = warnBlock(innerW).Render(fmt.Sprintf("Invalid path: %v", err))
	}
	btns := deleteConfirmButtons(m.deleteConfirmYes)
	parts := []string{t, "", "Repository", repoLine, "", "Path (in repo)", relLine}
	if err == nil {
		parts = append(parts, "", "Full path", styleDim.Render(truncateASCII(absPath, innerW)))
	}
	parts = append(parts, "", warn, "", btns, "", deleteConfirmFooter())
	inner := strings.Join(parts, "\n")
	return m.placeCenteredDimModal(roundedModal(boxW).Render(inner))
}

// renderCheckoutStatusFileConfirmOverlay asks before discarding local changes with git checkout HEAD -- path.
func (m *model) renderCheckoutStatusFileConfirmOverlay() string {
	repo := m.currentRepo()
	boxW := min(m.width-layoutModalSideGutter, layoutWhyAndConfirmModalMaxBox)
	if boxW < layoutModalBoxMinWidth {
		boxW = min(m.width-2, layoutModalBoxMinWidth)
	}
	innerW := max(layoutMinInnerContentWidth, boxW-layoutModalSideGutter)
	if repo == "" || m.checkoutStatusFilePendingRel == "" {
		return m.placeCenteredDimModal(roundedModal(boxW).Render("Nothing to restore."))
	}
	repoLine := styleDim.Render(truncateASCII(repo, innerW))
	relLine := styleDim.Render(truncateASCII(m.checkoutStatusFilePendingRel, innerW))
	t := styleBold.Render("Restore this path from the last commit?")
	warn := warnBlock(innerW).Render("This runs git checkout HEAD -- on the path. Uncommitted changes (staged and unstaged) for this file will be discarded.")
	btns := deleteConfirmButtons(m.deleteConfirmYes)
	inner := strings.Join([]string{
		t, "",
		"Repository", repoLine, "",
		"Path (in repo)", relLine, "",
		warn, "",
		btns, "",
		deleteConfirmFooter(),
	}, "\n")
	return m.placeCenteredDimModal(roundedModal(boxW).Render(inner))
}

// framedBlock wraps pane body content in a titled border block.
func (m *model) framedBlock(p pane, outerW, outerH int, title string, body string) string {
	fg := lipgloss.Color("240")
	if m.focus == p {
		fg = lipgloss.Color("214")
	}
	borderStyle := lipgloss.NewStyle().Foreground(fg)
	border := borderFor(p, m.focus)
	innerW := max(1, outerW-2)
	innerH := outerH - 2
	inner := lipgloss.NewStyle().
		Width(innerW).
		Height(innerH).
		MaxHeight(innerH).
		Render(body)

	titleRendered := m.sectionTitle(p, title)
	if p == paneDiff {
		titleRendered = m.diffPaneBorderTitle()
	}
	titleText := " " + titleRendered + " "
	fillW := max(0, innerW-lipgloss.Width(titleText))
	top := borderStyle.Render(border.TopLeft) +
		titleText +
		borderStyle.Render(strings.Repeat(border.Top, fillW)+border.TopRight)
	bottom := borderStyle.Render(border.BottomLeft + strings.Repeat(border.Bottom, innerW) + border.BottomRight)

	lines := strings.Split(inner, "\n")
	framed := make([]string, 0, len(lines)+2)
	framed = append(framed, top)
	for _, line := range lines {
		framed = append(framed, borderStyle.Render(border.Left)+line+borderStyle.Render(border.Right))
	}
	framed = append(framed, bottom)
	return strings.Join(framed, "\n")
}

// framedStatusBranchesRow draws Status and Branches as two framed panes side by side.
func (m *model) framedStatusBranchesRow(outerH int, statusBody, branchesBody string) string {
	statusOuterW, branchesOuterW := m.statusBranchesOuterWidths(m.width)
	statusBlock := m.framedBlock(paneStatus, statusOuterW, outerH, "Status", statusBody)
	branchesBlock := m.framedBlock(paneBranches, branchesOuterW, outerH, "Branches", branchesBody)
	return lipgloss.JoinHorizontal(lipgloss.Top, statusBlock, branchesBlock)
}

// clampRepoScroll keeps repoScrollTop in range and ensures the cursor row is visible.
func (m *model) clampRepoScroll(innerH int) {
	n := len(m.repoList)
	if innerH < 1 || n == 0 {
		m.repoScrollTop = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if n <= innerH {
		m.repoScrollTop = 0
		return
	}
	maxTop := n - innerH
	if m.repoScrollTop > maxTop {
		m.repoScrollTop = maxTop
	}
	if m.repoScrollTop < 0 {
		m.repoScrollTop = 0
	}
	if m.cursor < m.repoScrollTop {
		m.repoScrollTop = m.cursor
	}
	if m.cursor >= m.repoScrollTop+innerH {
		m.repoScrollTop = m.cursor - innerH + 1
	}
	if m.repoScrollTop > maxTop {
		m.repoScrollTop = maxTop
	}
}

// syncRepoListScrollOnly updates repoScrollTop for the current cursor; it is
// cheap and used while keyboard navigation debounces the rest of the UI.
func (m *model) syncRepoListScrollOnly() {
	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && diffBody == 0 && logBody == 0 {
		return
	}
	m.clampRepoScroll(repoBody)
}

// repoListView renders the repository list with current selection styling.
func (m *model) repoListView(innerH int) string {
	selFocused := styleSelRowFocused
	selBlurred := styleSelRowBlurred
	if len(m.repoList) == 0 && !m.scanning {
		return "(no dirty or diverged repositories)"
	}
	n := len(m.repoList)
	start := m.repoScrollTop
	if start < 0 {
		start = 0
	}
	if start > n {
		start = n
	}
	end := start + innerH
	if end > n {
		end = n
	}
	var b strings.Builder
	for i := start; i < end; i++ {
		if i > start {
			b.WriteString("\n")
		}
		path := m.repoList[i]
		if i == m.cursor {
			if m.focus == paneRepo {
				b.WriteString(selFocused.Render(path))
			} else {
				b.WriteString(selBlurred.Render(path))
			}
		} else {
			b.WriteString(path)
		}
	}
	return placeSpace(m.innerWidth(), innerH, b.String())
}

// renderHelpOverlay draws the help panel edge-to-edge in the terminal.
func (m *model) renderHelpOverlay() string {
	return m.helpPanel()
}

// renderScanOverlay centers and draws the scanning modal.
func (m *model) renderScanOverlay() string {
	popup := m.scanProgressPopup()
	return m.placeCenteredDimModal(popup)
}

// renderZoomedPane draws only the active pane in fullscreen mode.
func (m *model) renderZoomedPane(repoBody int) string {
	switch m.zoomTarget {
	case paneRepo:
		return m.framedBlock(paneRepo, m.width, m.height, "Repositories", m.repoListView(repoBody))
	case paneStatus:
		return m.framedBlock(paneStatus, m.width, m.height, "Status", m.statusTable.View())
	case paneBranches:
		return m.framedBlock(paneBranches, m.width, m.height, "Branches", m.branchTable.View())
	case paneDiff:
		return m.framedBlock(paneDiff, m.width, m.height, "Diff", m.diffVP.View())
	case paneLog:
		m.setLogVPContent()
		return m.framedBlock(paneLog, m.width, m.height, "Log", m.logVP.View())
	default:
		return ""
	}
}

// renderMainStack composes the standard four-pane vertical layout.
func (m *model) renderMainStack(repoBody, statusBody, diffBody, logBody int) string {
	repoOuter := panelOuter(repoBody)
	statusOuter := panelOuter(statusBody)
	diffOuter := panelOuter(diffBody)
	logOuter := panelOuter(logBody)

	repoBlock := m.framedBlock(paneRepo, m.width, repoOuter, "Repositories", m.repoListView(repoBody))
	statusRow := m.framedStatusBranchesRow(statusOuter, m.statusTable.View(), m.branchTable.View())
	diffBlock := m.framedBlock(paneDiff, m.width, diffOuter, "Diff", m.diffVP.View())
	m.setLogVPContent()
	logBlock := m.framedBlock(paneLog, m.width, logOuter, "Log", m.logVP.View())

	return lipgloss.JoinVertical(lipgloss.Left, repoBlock, statusRow, diffBlock, logBlock)
}

// renderErrorOverlay shows an error dialog with recovery hints.
func (m *model) renderErrorOverlay() string {
	errW := min(m.width-layoutErrorOverlayHPad, layoutErrorOverlayMaxWidth)
	errBox := errorDoubleBox(errW).Render("Error\n\n" + m.err.Error() + "\n\n(s to rescan, q to quit)")
	return m.placeCenteredPlain(errBox)
}

// View renders the full terminal UI for the current model state.
func (m *model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.helpOpen {
		return m.renderHelpOverlay()
	}
	if m.deleteRepoConfirmOpen {
		return m.renderDeleteRepoConfirmOverlay()
	}
	if m.deleteStatusFileConfirmOpen {
		return m.renderDeleteStatusFileConfirmOverlay()
	}
	if m.checkoutStatusFileConfirmOpen {
		return m.renderCheckoutStatusFileConfirmOverlay()
	}
	if m.whyRepoOpen {
		return m.renderWhyInclusionOverlay()
	}
	if m.height < layoutMinTermHeight {
		return styleErr.Render(fmt.Sprintf("Need bigger screen (min height %d).", layoutMinTermHeight))
	}
	if m.scanning {
		return m.renderScanOverlay()
	}

	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && diffBody == 0 && logBody == 0 {
		return ""
	}

	stack := ""
	if m.zoomed {
		stack = m.renderZoomedPane(repoBody)
	} else {
		stack = m.renderMainStack(repoBody, statusBody, diffBody, logBody)
	}
	if m.err != nil {
		return m.renderErrorOverlay()
	}
	return stack
}
