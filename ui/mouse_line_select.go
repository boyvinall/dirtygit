package ui

import (
	"reflect"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// bubblesTableSlice reads the table's current [start,end) row window used for
// rendering. Field names match github.com/charmbracelet/bubbles/table v1.0.0.
func bubblesTableSlice(t table.Model) (start, end int) {
	v := reflect.ValueOf(t)
	sf := v.FieldByName("start")
	ef := v.FieldByName("end")
	if !sf.IsValid() || !ef.IsValid() {
		return 0, 0
	}
	return int(sf.Int()), int(ef.Int())
}

// bubblesTableViewportYOffset reads the embedded viewport's vertical scroll.
func bubblesTableViewportYOffset(t table.Model) int {
	vp := reflect.ValueOf(t).FieldByName("viewport")
	if !vp.IsValid() {
		return 0
	}
	y := vp.FieldByName("YOffset")
	if !y.IsValid() {
		return 0
	}
	return int(y.Int())
}

func statusTableHeaderLines(t table.Model) int {
	s := strings.SplitN(t.View(), "\n", 2)
	if len(s) < 1 {
		return 0
	}
	return lipgloss.Height(s[0])
}

// repoPaneOuterHeight returns the framed repo pane height in terminal rows.
func (m *model) repoPaneOuterHeight() (outerH int, ok bool) {
	if m.height < layoutMinTermHeight || m.width < layoutMinTermWidth {
		return 0, false
	}
	lay := m.layoutBodies()
	if lay.isZero() {
		return 0, false
	}
	if m.zoomed {
		if m.zoomTarget != paneRepo {
			return 0, false
		}
		return m.height, true
	}
	return panelOuter(lay.repo), true
}

// statusPaneFrame returns the top Y of the status pane, its outer height, and
// outer width (for hit-testing X). The status column always starts at x=0.
func (m *model) statusPaneFrame() (topY, outerH, outerW int, ok bool) {
	if m.height < layoutMinTermHeight || m.width < layoutMinTermWidth {
		return 0, 0, 0, false
	}
	lay := m.layoutBodies()
	if lay.isZero() {
		return 0, 0, 0, false
	}
	if m.zoomed {
		if m.zoomTarget != paneStatus {
			return 0, 0, 0, false
		}
		return 0, m.height, m.width, true
	}
	repoOuter := panelOuter(lay.repo)
	statusOuter := panelOuter(lay.status)
	leftW, _ := m.middleRowColumnOuterWidths(m.width)
	return repoOuter, statusOuter, leftW, true
}

// handleMousePaneLineSelect handles left-click row selection when the repo or
// status pane is already focused. Returns (true, cmd) when the click is
// handled; cmd may schedule deferred work (e.g. git diff for a new repo).
func (m *model) handleMousePaneLineSelect(msg tea.MouseMsg) (bool, tea.Cmd) {
	if !m.interactiveAppReady() {
		return false, nil
	}
	if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionPress {
		return false, nil
	}

	switch m.focus {
	case paneRepo:
		outerH, ok := m.repoPaneOuterHeight()
		if !ok || msg.X < 0 || msg.X >= m.width {
			return false, nil
		}
		const repoTopY = 0
		relY := msg.Y - repoTopY
		bodyH := outerH - 2
		if relY < 1 || relY > bodyH {
			return false, nil
		}
		lineIdx := relY - 1
		global := m.repoScrollTop + lineIdx
		if len(m.repoList) == 0 || global < 0 || global >= len(m.repoList) {
			return false, nil
		}
		if global == m.cursor {
			return true, nil
		}
		// Drop any pending keyboard debounce; mouse selection should feel immediate.
		m.repoNavSettleGen++
		m.cursor = global
		m.statusFileSelected = false
		m.diffNeedsRefresh = true
		m.applyViewportAndPanes(false)
		return true, m.scheduleRunDiff()

	case paneStatus:
		topY, outerH, outerW, ok := m.statusPaneFrame()
		if !ok || msg.X < 0 || msg.X >= outerW {
			return false, nil
		}
		relY := msg.Y - topY
		bodyH := outerH - 2
		if relY < 1 || relY > bodyH {
			return false, nil
		}
		lineInBody := relY - 1
		headerH := statusTableHeaderLines(m.statusTable)
		r := lineInBody - headerH
		if r < 0 {
			return true, nil
		}
		rows := m.statusTable.Rows()
		if len(rows) == 0 {
			return true, nil
		}
		start, end := bubblesTableSlice(m.statusTable)
		yoff := bubblesTableViewportYOffset(m.statusTable)
		vpH := m.statusTable.Height()
		contentLines := end - start
		if contentLines <= 0 {
			return true, nil
		}
		vis := vpH
		if yoff+vis > contentLines {
			vis = contentLines - yoff
		}
		if vis < 0 {
			vis = 0
		}
		if r >= vis {
			return true, nil
		}
		global := start + yoff + r
		if global < 0 {
			global = 0
		}
		if global >= len(rows) {
			global = len(rows) - 1
		}
		m.statusTable.SetCursor(global)
		if len(m.statusPaths) > 0 {
			m.statusFileSelected = true
			m.statusTable.Focus()
		}
		m.applyStatusTableFocusAndStyles()
		m.diffNeedsRefresh = true
		m.syncViewports()
		return true, nil

	default:
		return false, nil
	}
}
