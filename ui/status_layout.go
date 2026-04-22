package ui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5"

	"github.com/boyvinall/dirtygit/scanner"
)

// layoutBodies returns inner content heights: repo list, status table, diff viewport, log viewport.
// Each framed panel's total row count is panelOuter(body) (see framedBlock).
func (m *model) layoutBodies() (repoBody, statusBody, diffBody, logBody int) {
	if m.height < minTermHeight || m.width < 20 {
		return 0, 0, 0, 0
	}
	if m.zoomed {
		body := max(
			// panelOuter(body) == m.height
			m.height-2, 3)
		switch m.zoomTarget {
		case paneRepo:
			return body, 0, 0, 0
		case paneStatus:
			return 0, body, 0, 0
		case paneDiff:
			return 0, 0, body, 0
		case paneLog:
			return 0, 0, 0, body
		}
	}
	effH := m.height
	logBody = 4
	n := len(m.repoList)
	if n == 0 {
		n = 1
	}
	repoBody = min(n+2, effH/3)
	repoBody = max(3, repoBody)

	available := effH - 8 - repoBody - logBody
	for available < 6 && logBody > 3 {
		logBody--
		available = effH - 8 - repoBody - logBody
	}
	for available < 6 && repoBody > 3 {
		repoBody--
		available = effH - 8 - repoBody - logBody
	}
	if available < 6 {
		return 0, 0, 0, 0
	}

	// Give Status and Diff a 1:3 height split.
	// Any remainder rows go to Diff to keep it as large as possible.
	statusBody = available / 4
	diffBody = available - statusBody
	if statusBody < 3 || diffBody < 3 || logBody < 3 || repoBody < 3 {
		return 0, 0, 0, 0
	}
	return repoBody, statusBody, diffBody, logBody
}

// panelOuter converts an inner body height into full framed panel height.
func panelOuter(body int) int {
	return body + 2 // top border (with title) + body + bottom border
}

// innerWidth returns content width available inside pane borders.
func (m *model) innerWidth() int {
	w := m.width - 4
	if w < 8 {
		w = max(8, m.width-2)
	}
	return w
}

// syncViewports applies layout dimensions and refreshes pane content.
func (m *model) syncViewports() {
	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && diffBody == 0 && logBody == 0 {
		return
	}
	innerW := m.innerWidth()
	m.statusTable.SetWidth(innerW)
	m.statusTable.SetHeight(statusBody)
	m.statusTable.SetColumns(statusColumns(innerW))
	if m.focus == paneStatus && m.statusFileSelected {
		m.statusTable.Focus()
	} else {
		m.statusTable.Blur()
	}
	m.logVP.Width = innerW
	m.logVP.Height = logBody
	m.diffVP.Width = innerW
	m.diffVP.Height = diffBody
	m.refreshStatusContent()
	m.refreshDiffContent()
	m.diffVP.SetContent(m.diffContent)
	m.logVP.SetContent(m.logBuf.String())
}

// sortedRepoPaths returns repository paths in stable alphabetical order.
func sortedRepoPaths(mgs scanner.MultiGitStatus) []string {
	paths := make([]string, 0, len(mgs))
	for r := range mgs {
		paths = append(paths, r)
	}
	sort.Strings(paths)
	return paths
}

// newStatusTable builds the status pane table with default styling.
func newStatusTable() table.Model {
	t := table.New(
		table.WithColumns(statusColumns(48)),
		table.WithRows(nil),
		table.WithFocused(false),
		table.WithHeight(6),
	)
	styles := table.DefaultStyles()
	styles.Selected = styles.Selected.Bold(true)
	t.SetStyles(styles)
	return t
}

// statusColumns computes table column widths for the given pane width.
func statusColumns(totalWidth int) []table.Column {
	stagedWidth := 10
	worktreeWidth := 10
	pathWidth := max(8, totalWidth-stagedWidth-worktreeWidth-4)
	return []table.Column{
		{Title: "Staged", Width: stagedWidth},
		{Title: "Worktree", Width: worktreeWidth},
		{Title: "Path", Width: pathWidth},
	}
}

// refreshStatusContent rebuilds status rows for the selected repository.
func (m *model) refreshStatusContent() {
	repo := m.currentRepo()
	st, ok := m.repositories[repo]
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
				statusCodeLabel(entry.Staging),
				statusCodeLabel(entry.Worktree),
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
				statusCodeLabel(status.Staging),
				statusCodeLabel(status.Worktree),
				path,
			})
			paths = append(paths, path)
		}
	}
	m.statusPaths = paths
	if len(paths) == 0 {
		m.statusFileSelected = false
		m.statusTable.Blur()
	}
	m.statusTable.SetRows(rows)
	if len(rows) > 0 && m.statusTable.Cursor() >= len(rows) {
		m.statusTable.SetCursor(len(rows) - 1)
	}
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

// borderFor picks a heavier border for the active pane.
func borderFor(p, active pane) lipgloss.Border {
	if p == active {
		return lipgloss.ThickBorder()
	}
	return lipgloss.NormalBorder()
}

// sectionTitle highlights pane titles when the pane is focused.
func (m *model) sectionTitle(p pane, label string) string {
	if m.focus == p {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render(label)
	}
	return label
}
