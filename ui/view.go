package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/boyvinall/dirtygit/scanner"
)

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
	if max < 8 || path == "" || len(path) <= max {
		return path
	}
	return "…" + path[len(path)-(max-3):]
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
	boxW := min(m.width-6, 64)
	if boxW < 20 {
		boxW = min(m.width-2, 20)
	}
	// Inner text width: border (2) + horizontal padding (4) is a safe shave from boxW.
	innerW := max(8, boxW-6)
	bar := scanProgressBar(innerW, p.ReposChecked, max(p.ReposFound, 1))

	line := fmt.Sprintf("Found %d repo(s)  ·  checked git status %d", p.ReposFound, p.ReposChecked)
	line = truncateASCII(line, max(4, innerW-4))

	spin := lipgloss.NewStyle().Width(2).MaxWidth(2).Align(lipgloss.Center).Render(m.scanSpinner.View())
	row1 := lipgloss.JoinHorizontal(lipgloss.Left, spin, " ", line)
	row1 = lipgloss.Place(innerW, 1, lipgloss.Left, lipgloss.Top, row1,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))

	pathText := shortenScanPath(p.CurrentPath, innerW)
	if pathText == "" {
		pathText = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("—")
	}
	pathRow := lipgloss.Place(innerW, 1, lipgloss.Left, lipgloss.Top, pathText,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))

	title := lipgloss.Place(innerW, 1, lipgloss.Left, lipgloss.Top, truncateASCII("Scanning repositories", innerW),
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
	footer := lipgloss.Place(innerW, 1, lipgloss.Left, lipgloss.Top, truncateASCII("Please wait...", innerW),
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))

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

	body = lipgloss.Place(innerW, scanModalInnerLines, lipgloss.Left, lipgloss.Top, body,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Width(boxW).
		Padding(1, 2).
		Render(body)
}

// helpPanel renders keyboard shortcut documentation in a frame that fills the terminal.
func (m *model) helpPanel() string {
	lines := []string{
		"Click        Focus a pane; in Repositories or Status (when focused), select a row",
		"Tab          Focus next pane (Repositories → Status → Branches → Diff → Log); when zoomed, cycle pane",
		"Shift+Tab    Focus previous pane; when zoomed, cycle pane backward",
		"Enter        Zoom focused pane to fullscreen; Enter again to restore layout",
		"Esc          Exit zoom (when zoomed), or clear Status file selection",
		"↑ / ↓        Move repo selection or scroll Status / Diff / Log",
		"← / →        In Status or Diff pane, switch Worktree/Staged diff mode",
		"a  r         With a status file row selected (Status or Diff pane): git add / git reset (unstage) that path",
		"s            Scan / rescan repositories",
		"e            Open selected repository (edit.command in config)",
		"w            With Repositories focused: why this repository is in the list",
		"D            With Repositories focused: delete that directory (asks for confirmation)",
		"q  Ctrl+C    Quit",
		"?  h         Show this help",
		"",
		"Esc, ?, or h closes this window.",
	}
	body := "Keyboard shortcuts\n\n" + strings.Join(lines, "\n")
	w, h := m.width, m.height
	if h < 1 {
		h = minTermHeight
	}
	innerW := max(1, w-2)
	innerH := max(1, h-2)
	padded := lipgloss.Place(innerW, innerH, lipgloss.Left, lipgloss.Top, body,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Render(padded)
}

// renderWhyInclusionOverlay shows why the selected repository appears in the list.
func (m *model) renderWhyInclusionOverlay() string {
	repo := m.currentRepo()
	boxW := min(m.width-6, 72)
	if boxW < 20 {
		boxW = min(m.width-2, 20)
	}
	innerW := max(8, boxW-6)
	if repo == "" {
		box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("39")).Width(boxW).Padding(1, 2).Render("No repository selected.")
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
			lipgloss.WithWhitespaceChars("░"),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
			lipgloss.WithWhitespaceBackground(lipgloss.Color("235")))
	}
	rs, ok := m.repositories[repo]
	if !ok {
		box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("39")).Width(boxW).Padding(1, 2).Render("No status data for this path.")
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
			lipgloss.WithWhitespaceChars("░"),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
			lipgloss.WithWhitespaceBackground(lipgloss.Color("235")))
	}
	lines := scanner.RepoInclusionReasons(rs)
	if len(lines) == 0 {
		lines = []string{"No inclusion reason (unexpected for a listed repository)."}
	}
	reasons := strings.Join(lines, "\n\n")
	wrapped := lipgloss.NewStyle().Width(innerW).MaxWidth(innerW).Render(reasons)
	t := lipgloss.NewStyle().Bold(true).Render("Why is this repository listed?")
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(truncateASCII(repo, innerW))
	foot := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("w or Esc to close")
	inner := strings.Join([]string{t, "", sub, "", wrapped, "", foot}, "\n")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Width(boxW).
		Padding(1, 2).
		Render(inner)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars("░"),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
		lipgloss.WithWhitespaceBackground(lipgloss.Color("235")))
}

// renderDeleteRepoConfirmOverlay asks whether to recursively delete the selected repository directory.
func (m *model) renderDeleteRepoConfirmOverlay() string {
	repo := m.currentRepo()
	boxW := min(m.width-6, 72)
	if boxW < 20 {
		boxW = min(m.width-2, 20)
	}
	innerW := max(8, boxW-6)
	if repo == "" {
		box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("39")).Width(boxW).Padding(1, 2).Render("No repository selected.")
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
			lipgloss.WithWhitespaceChars("░"),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
			lipgloss.WithWhitespaceBackground(lipgloss.Color("235")))
	}
	pathLine := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(truncateASCII(repo, innerW))
	t := lipgloss.NewStyle().Bold(true).Render("Delete this directory recursively?")
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Width(innerW).MaxWidth(innerW).Render("This will remove the folder and all of its contents. This cannot be undone.")

	hlYes := lipgloss.NewStyle().Background(lipgloss.Color("160")).Foreground(lipgloss.Color("230"))
	hlNo := lipgloss.NewStyle().Background(lipgloss.Color("160")).Foreground(lipgloss.Color("230"))
	plain := lipgloss.NewStyle()
	var yesBtn, noBtn string
	if m.deleteConfirmYes {
		yesBtn = hlYes.Render(" Yes ")
		noBtn = plain.Render(" No ")
	} else {
		yesBtn = plain.Render(" Yes ")
		noBtn = hlNo.Render(" No ")
	}
	btns := lipgloss.JoinHorizontal(lipgloss.Left, yesBtn, "  ", noBtn)
	foot := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("←/→ or y/n · Enter to confirm · Esc to cancel")
	inner := strings.Join([]string{t, "", pathLine, "", warn, "", btns, "", foot}, "\n")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Width(boxW).
		Padding(1, 2).
		Render(inner)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars("░"),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
		lipgloss.WithWhitespaceBackground(lipgloss.Color("235")))
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
	selFocused := lipgloss.NewStyle().Background(lipgloss.Color("42")).Foreground(lipgloss.Color("0"))
	selBlurred := lipgloss.NewStyle().Background(lipgloss.Color("248")).Foreground(lipgloss.Color("0"))
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
	return lipgloss.Place(m.innerWidth(), innerH, lipgloss.Left, lipgloss.Top, b.String(),
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
}

// renderHelpOverlay draws the help panel edge-to-edge in the terminal.
func (m *model) renderHelpOverlay() string {
	return m.helpPanel()
}

// renderScanOverlay centers and draws the scanning modal.
func (m *model) renderScanOverlay() string {
	popup := m.scanProgressPopup()
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup,
		lipgloss.WithWhitespaceChars("░"),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
		lipgloss.WithWhitespaceBackground(lipgloss.Color("235")))
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
		m.logVP.SetContent(m.logBuf.String())
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
	m.logVP.SetContent(m.logBuf.String())
	logBlock := m.framedBlock(paneLog, m.width, logOuter, "Log", m.logVP.View())

	return lipgloss.JoinVertical(lipgloss.Left, repoBlock, statusRow, diffBlock, logBlock)
}

// renderErrorOverlay shows an error dialog with recovery hints.
func (m *model) renderErrorOverlay() string {
	errW := min(m.width-4, 80)
	errBox := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("9")).
		Width(errW).
		Padding(1, 2).
		Render("Error\n\n" + m.err.Error() + "\n\n(s to rescan, q to quit)")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, errBox,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
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
	if m.whyRepoOpen {
		return m.renderWhyInclusionOverlay()
	}
	if m.height < minTermHeight {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Need bigger screen (min height 22).")
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
