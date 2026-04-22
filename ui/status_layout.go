package ui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5"

	"github.com/boyvinall/dirtygit/scanner"
)

// layoutBodies returns inner content heights: repo list, status viewport, log viewport.
// Each framed panel's total row count is panelOuter(body) (see framedBlock).
func (m *model) layoutBodies() (repoBody, statusBody, logBody int) {
	if m.height < minTermHeight || m.width < 20 {
		return 0, 0, 0
	}
	if m.zoomed {
		body := max(
			// panelOuter(body) == m.height
			m.height-2, 3)
		switch m.zoomTarget {
		case paneRepo:
			return body, 0, 0
		case paneStatus:
			return 0, body, 0
		case paneLog:
			return 0, 0, body
		}
	}
	effH := m.height
	logBody = 4
	n := len(m.repoList)
	if n == 0 {
		n = 1
	}
	repoBody = min(n+2, effH/2)
	repoBody = max(3, repoBody)

	statusBody = effH - 6 - repoBody - logBody
	for statusBody < 3 && logBody > 3 {
		logBody--
		statusBody = effH - 6 - repoBody - logBody
	}
	for statusBody < 3 && repoBody > 3 {
		repoBody--
		statusBody = effH - 6 - repoBody - logBody
	}
	if statusBody < 3 || logBody < 3 || repoBody < 3 {
		return 0, 0, 0
	}
	return repoBody, statusBody, logBody
}

func panelOuter(body int) int {
	return body + 2 // top border (with title) + body + bottom border
}

func (m *model) innerWidth() int {
	w := m.width - 4
	if w < 8 {
		w = max(8, m.width-2)
	}
	return w
}

func (m *model) syncViewports() {
	repoBody, statusBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && logBody == 0 {
		return
	}
	innerW := m.innerWidth()
	m.statusTable.SetWidth(innerW)
	m.statusTable.SetHeight(statusBody)
	m.statusTable.SetColumns(statusColumns(innerW))
	if m.focus == paneStatus {
		m.statusTable.Focus()
	} else {
		m.statusTable.Blur()
	}
	m.logVP.Width = innerW
	m.logVP.Height = logBody
	m.refreshStatusContent()
	m.logVP.SetContent(m.logBuf.String())
}

func sortedRepoPaths(mgs scanner.MultiGitStatus) []string {
	paths := make([]string, 0, len(mgs))
	for r := range mgs {
		paths = append(paths, r)
	}
	sort.Strings(paths)
	return paths
}

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

func (m *model) refreshStatusContent() {
	repo := m.currentRepo()
	st, ok := m.repositories[repo]
	rows := make([]table.Row, 0)
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
		}
	}
	m.statusTable.SetRows(rows)
}

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

func (m *model) currentRepo() string {
	if m.cursor < 0 || m.cursor >= len(m.repoList) {
		return ""
	}
	return m.repoList[m.cursor]
}

func borderFor(p, active pane) lipgloss.Border {
	if p == active {
		return lipgloss.ThickBorder()
	}
	return lipgloss.NormalBorder()
}

func (m *model) sectionTitle(p pane, label string) string {
	if m.focus == p {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render(label)
	}
	return label
}
