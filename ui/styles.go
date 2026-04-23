package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// --- Shared palette (ANSI 256) ---

var (
	styleDim       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleBold      = lipgloss.NewStyle().Bold(true)
	styleErr       = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleAccentLbl = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	styleDiffMode  = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	stylePlain     = lipgloss.NewStyle()
	styleScanSpin  = lipgloss.NewStyle().Foreground(lipgloss.Color("159"))

	styleSelRowFocused = lipgloss.NewStyle().Background(lipgloss.Color("42")).Foreground(lipgloss.Color("0"))
	styleSelRowBlurred = lipgloss.NewStyle().Background(lipgloss.Color("248")).Foreground(lipgloss.Color("0"))

	styleDeleteChoiceHL = lipgloss.NewStyle().Background(lipgloss.Color("160")).Foreground(lipgloss.Color("230"))
)

// roundedModal returns the standard info / overlay box (cyan border).
func roundedModal(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Width(width).
		Padding(1, 2)
}

// errorDoubleBox is the thick red-bordered error dialog shell.
func errorDoubleBox(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("9")).
		Width(width).
		Padding(1, 2)
}

// placeSpace pads or clips content to a cell using space fill and default fg.
func placeSpace(width, height int, ha, va lipgloss.Position, content string) string {
	return lipgloss.Place(width, height, ha, va, content,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
}

// warnBlock is a full-width red warning paragraph inside a modal.
func warnBlock(innerW int) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Width(innerW).MaxWidth(innerW)
}

// tableSelectedRow matches repo-list greens/greys but keeps the status table bold.
func tableSelectedRow(selectionFocused bool) lipgloss.Style {
	if selectionFocused {
		return styleSelRowFocused.Copy().Bold(true)
	}
	return styleSelRowBlurred.Copy().Bold(true)
}

// deleteConfirmFooter is the dim hint line under Yes/No in delete overlays.
func deleteConfirmFooter() string {
	return styleDim.Render("←/→ or y/n · Enter to confirm · Esc to cancel")
}

// deleteConfirmButtons renders the Yes / No row with destructive highlight on the active choice.
func deleteConfirmButtons(yesSelected bool) string {
	var yesBtn, noBtn string
	if yesSelected {
		yesBtn = styleDeleteChoiceHL.Render(" Yes ")
		noBtn = stylePlain.Render(" No ")
	} else {
		yesBtn = stylePlain.Render(" Yes ")
		noBtn = styleDeleteChoiceHL.Render(" No ")
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, yesBtn, "  ", noBtn)
}

// focusedSectionTitle highlights a pane title when that pane has focus.
func focusedSectionTitle(focused bool, label string) string {
	if focused {
		return styleAccentLbl.Render(label)
	}
	return label
}

// diff line styles for git output (see styleDiffContent).
var (
	diffStyleAdded   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	diffStyleDeleted = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	diffStyleHunk    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	diffStyleHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	diffStyleFile    = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	diffStyleMeta    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

// diffPaneTopBorderLabel builds the Diff pane top-border title segment.
func diffPaneTopBorderLabel(diffPaneFocused bool, worktreeMode bool) string {
	diffLbl := "Diff"
	if diffPaneFocused {
		diffLbl = styleAccentLbl.Render("Diff")
	}
	sep := styleDim.Render(" · ")
	worktree := styleDim.Render("Worktree")
	staged := styleDim.Render("Staged")
	if worktreeMode {
		worktree = styleDiffMode.Render("Worktree")
	} else {
		staged = styleDiffMode.Render("Staged")
	}
	return strings.Join([]string{diffLbl, sep, worktree, sep, staged}, "")
}
