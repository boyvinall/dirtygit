package ui

import (
	"fmt"
	"sort"
	"time"

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

// statusBranchesOuterWidths splits the row width between Status and Branches panes.
func (m *model) statusBranchesOuterWidths(total int) (statusOuter, branchesOuter int) {
	if total < 20 {
		return max(10, total-10), min(10, total)
	}
	branchesOuter = max(24, total/3)
	if branchesOuter > total-12 {
		branchesOuter = total - 12
	}
	statusOuter = total - branchesOuter
	return statusOuter, branchesOuter
}

// statusBranchesInnerWidths returns table content widths inside each pane's own border.
func (m *model) statusBranchesInnerWidths() (statusInnerW, branchInnerW int) {
	statusOuterW, branchesOuterW := m.statusBranchesOuterWidths(m.width)
	statusInnerW = max(8, statusOuterW-2)
	branchInnerW = max(8, branchesOuterW-2)
	return statusInnerW, branchInnerW
}

// syncViewports applies layout dimensions and refreshes pane content.
func (m *model) syncViewports() {
	repoBody, statusBody, diffBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && diffBody == 0 && logBody == 0 {
		return
	}

	statusInnerW := m.innerWidth()
	branchInnerW := statusInnerW
	if !m.zoomed {
		statusInnerW, branchInnerW = m.statusBranchesInnerWidths()
	}

	m.statusTable.SetWidth(statusInnerW)
	m.statusTable.SetHeight(statusBody)
	m.statusTable.SetColumns(statusColumns(statusInnerW))
	if m.focus == paneStatus && m.statusFileSelected {
		m.statusTable.Focus()
	} else {
		m.statusTable.Blur()
	}
	m.logVP.Width = m.innerWidth()
	m.logVP.Height = logBody
	m.diffVP.Width = m.innerWidth()
	m.diffVP.Height = diffBody
	m.refreshStatusContent()
	m.refreshBranchContent(branchInnerW)
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

// newBranchTable builds the branch pane table with default styling.
func newBranchTable() table.Model {
	t := table.New(
		table.WithColumns(nil),
		table.WithRows(nil),
		table.WithFocused(false),
		table.WithHeight(6),
	)
	return t
}

// statusColumns computes table column widths for the given pane width.
// Bubbles table Header and Cell styles use Padding(0, 1), so each column adds
// two cells to the rendered row; totals must stay within totalWidth.
func statusColumns(totalWidth int) []table.Column {
	stagedWidth := 10
	worktreeWidth := 10
	const cols = 3
	horizontalPad := 2 * cols
	pathWidth := max(1, totalWidth-stagedWidth-worktreeWidth-horizontalPad)
	return []table.Column{
		{Title: "Staged", Width: stagedWidth},
		{Title: "Worktree", Width: worktreeWidth},
		{Title: "Path", Width: pathWidth},
	}
}

// branchColumns computes branch table columns based on local + remote names.
// Account for Padding(0, 1) on every header and cell (see statusColumns).
func branchColumns(totalWidth int, locations []string) []table.Column {
	if len(locations) == 0 {
		return []table.Column{{Title: "Info", Width: max(1, totalWidth-2)}}
	}
	metricWidth := 11
	n := len(locations)
	numCols := n + 1
	horizontalPad := 2 * numCols
	colWidth := max(1, (totalWidth-metricWidth-horizontalPad)/n)
	cols := make([]table.Column, 0, len(locations)+1)
	cols = append(cols, table.Column{Title: "Metric", Width: metricWidth})
	for _, location := range locations {
		cols = append(cols, table.Column{Title: location, Width: colWidth})
	}
	return cols
}

// refreshBranchContent rebuilds branch divergence rows for the selected repository.
func (m *model) refreshBranchContent(totalWidth int) {
	repo := m.currentRepo()
	st, ok := m.repositories[repo]
	if !ok {
		m.branchTable.SetColumns(branchColumns(totalWidth, nil))
		m.branchTable.SetRows([]table.Row{{"(select repository)"}})
		m.branchTable.SetHeight(3)
		return
	}
	branch := st.Branches
	if branch.Detached {
		m.branchTable.SetColumns(branchColumns(totalWidth, []string{"local"}))
		m.branchTable.SetRows([]table.Row{{"Branch", branch.Branch}})
		m.branchTable.SetHeight(3)
		return
	}

	locations := make([]string, 0, len(branch.Locations))
	for _, loc := range branch.Locations {
		locations = append(locations, loc.Name)
	}
	cols := branchColumns(totalWidth, locations)
	rows := make([]table.Row, 0, 5)
	nameRow := table.Row{"Name"}
	for range branch.Locations {
		nameRow = append(nameRow, branch.Branch)
	}
	commitRow := table.Row{"Commit"}
	onlyHereRow := table.Row{"Only here"}
	recencyRow := table.Row{"Recency"}
	tipRow := table.Row{"Tip age"}
	for _, loc := range branch.Locations {
		if !loc.Exists {
			commitRow = append(commitRow, "(missing)")
			onlyHereRow = append(onlyHereRow, "(missing)")
			recencyRow = append(recencyRow, "(missing)")
			tipRow = append(tipRow, "(missing)")
			continue
		}
		commitRow = append(commitRow, shortHash(loc.TipHash))
		if loc.UniqueCount > 0 {
			onlyHereRow = append(onlyHereRow, fmt.Sprintf("yes (%d)", loc.UniqueCount))
		} else {
			onlyHereRow = append(onlyHereRow, "-")
		}
		switch {
		case loc.UniqueCount == 0:
			recencyRow = append(recencyRow, "-")
		case branch.NewestLocation == loc.Name:
			recencyRow = append(recencyRow, "newest")
		default:
			recencyRow = append(recencyRow, "older")
		}
		tipRow = append(tipRow, relativeTime(loc.TipUnix))
	}
	rows = append(rows, nameRow, commitRow, onlyHereRow, recencyRow, tipRow)
	m.branchTable.SetColumns(cols)
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

// borderFor picks a heavier border for the active pane (used by framedBlock).
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
