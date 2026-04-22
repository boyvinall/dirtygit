package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// helpPanel renders keyboard shortcut documentation.
func (m *model) helpPanel() string {
	lines := []string{
		"Tab          Focus next pane (Repositories → Status → Branches → Diff → Log); when zoomed, cycle pane",
		"Shift+Tab    Focus previous pane; when zoomed, cycle pane backward",
		"Enter        Zoom focused pane to fullscreen; Enter again to restore layout",
		"Esc          Exit zoom (when zoomed), or clear Status file selection",
		"↑ / ↓        Move repo selection or scroll Status / Diff / Log",
		"← / →        In Diff pane, switch Worktree/Staged diff mode",
		"a  r         In Status pane with a file row selected: git add / git reset (unstage) that path",
		"s            Scan / rescan repositories",
		"e            Open selected repository in VS Code (code)",
		"q  Ctrl+C    Quit",
		"?  h         Show this help",
		"",
		"Esc, ?, or h closes this window.",
	}
	content := strings.Join(lines, "\n")
	boxW := min(m.width-4, 72)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Width(boxW).
		Padding(1, 2).
		Render("Keyboard shortcuts\n\n" + content)
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

	titleText := " " + m.sectionTitle(p, title) + " "
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

// repoListView renders the repository list with current selection styling.
func (m *model) repoListView(innerH int) string {
	selFocused := lipgloss.NewStyle().Background(lipgloss.Color("42")).Foreground(lipgloss.Color("0"))
	selBlurred := lipgloss.NewStyle().Background(lipgloss.Color("248")).Foreground(lipgloss.Color("0"))
	if len(m.repoList) == 0 && !m.scanning {
		return "(no dirty or diverged repositories)"
	}
	var b strings.Builder
	for i, path := range m.repoList {
		if i > 0 {
			b.WriteString("\n")
		}
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

// renderHelpOverlay centers and draws the help panel.
func (m *model) renderHelpOverlay() string {
	help := m.helpPanel()
	h := m.height
	if h < 1 {
		h = minTermHeight
	}
	return lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center, help,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
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
		return m.framedBlock(paneDiff, m.width, m.height, "Diff ("+m.diffModeLabel()+")", m.diffVP.View())
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
	diffBlock := m.framedBlock(paneDiff, m.width, diffOuter, "Diff ("+m.diffModeLabel()+")", m.diffVP.View())
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
	m.syncViewports()

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
