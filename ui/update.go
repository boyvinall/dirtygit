package ui

import (
	"os/exec"

	cspinner "github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// handleWindowSize updates dimensions and recomputes pane layout.
func (m *model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	if m.logVP.Height == 0 {
		inner := max(8, msg.Width-4)
		m.logVP = viewport.New(inner, 8)
	}
	if m.diffVP.Height == 0 {
		inner := max(8, msg.Width-4)
		m.diffVP = viewport.New(inner, 8)
	}
	m.syncViewports()
	return m, nil
}

// handleSpinnerTick advances the spinner and drains scan progress updates.
func (m *model) handleSpinnerTick(msg cspinner.TickMsg) (tea.Model, tea.Cmd) {
	if !m.scanning {
		return m, nil
	}
	m.drainScanProgress()
	var spinCmd tea.Cmd
	m.scanSpinner, spinCmd = m.scanSpinner.Update(msg)
	return m, spinCmd
}

// finishScan stores scan results and resets scanning state.
func (m *model) finishScan(r scanResult) {
	m.scanning = false
	m.drainScanProgress()
	m.err = r.err
	if r.err != nil {
		return
	}

	m.repositories = r.mgs
	m.repoList = sortedRepoPaths(r.mgs)
	m.statusFileSelected = false
	m.diffNeedsRefresh = true
	if m.cursor >= len(m.repoList) {
		m.cursor = max(0, len(m.repoList)-1)
	}
}

// handleScanTick polls for scan completion and schedules the next poll.
func (m *model) handleScanTick() (tea.Model, tea.Cmd) {
	if !m.scanning {
		return m, nil
	}
	m.drainScanProgress()
	select {
	case r := <-m.scanResultCh:
		m.finishScan(r)
		m.syncViewports()
		return m, nil
	default:
		return m, tickCmd()
	}
}

// handleHelpOverlayKey processes keys while the help overlay is open.
func (m *model) handleHelpOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.helpOpen = false
		return m, nil
	default:
		if helpKey(msg) {
			m.helpOpen = false
		}
		return m, nil
	}
}

// handleScanningKey handles the limited key set allowed during scans.
func (m *model) handleScanningKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	default:
		return m, nil
	}
}

// toggleZoom enters or exits fullscreen mode for the focused pane.
func (m *model) toggleZoom() {
	if m.zoomed {
		m.zoomed = false
		return
	}
	m.zoomed = true
	m.zoomTarget = m.focus
}

// cycleFocus moves focus across panes in forward or reverse order.
func (m *model) cycleFocus(forward bool) {
	const paneCount = 5
	if m.zoomed {
		if forward {
			m.zoomTarget = (m.zoomTarget + 1) % paneCount
		} else {
			m.zoomTarget = (m.zoomTarget - 1 + paneCount) % paneCount
		}
		m.focus = m.zoomTarget
		return
	}

	if forward {
		m.focus = (m.focus + 1) % paneCount
		return
	}
	m.focus = (m.focus - 1 + paneCount) % paneCount
}

// openCurrentRepo opens the selected repository in VS Code.
func (m *model) openCurrentRepo() {
	repo := m.currentRepo()
	if repo != "" {
		_ = exec.Command("code", repo).Run()
	}
}

// handleCommandKey handles global command keys and focus controls.
func (m *model) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit, true
	case "esc":
		if m.zoomed {
			m.zoomed = false
			m.syncViewports()
			return m, nil, true
		}
		if m.focus == paneStatus && m.statusFileSelected {
			m.statusFileSelected = false
			m.statusTable.Blur()
			m.diffNeedsRefresh = true
			m.syncViewports()
			return m, nil, true
		}
	case "enter":
		m.toggleZoom()
		m.syncViewports()
		return m, nil, true
	case "tab":
		m.cycleFocus(true)
		m.syncViewports()
		return m, nil, true
	case "shift+tab":
		m.cycleFocus(false)
		m.syncViewports()
		return m, nil, true
	case "s":
		if !m.scanning {
			return m, m.beginScan(), true
		}
		return m, nil, true
	case "e":
		m.openCurrentRepo()
		return m, nil, true
	default:
		if helpKey(msg) {
			m.helpOpen = true
			return m, nil, true
		}
	}
	return m, nil, false
}

// handleArrowKey applies directional key behavior for the focused pane.
func (m *model) handleArrowKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown:
		if m.focus == paneRepo {
			prev := m.cursor
			if msg.Type == tea.KeyUp && m.cursor > 0 {
				m.cursor--
			} else if msg.Type == tea.KeyDown && m.cursor < len(m.repoList)-1 {
				m.cursor++
			}
			if m.cursor != prev {
				m.statusFileSelected = false
				m.refreshStatusContent()
				m.diffNeedsRefresh = true
			}
			return m, nil, true
		}
		if m.focus == paneStatus {
			if !m.statusFileSelected && len(m.statusPaths) > 0 {
				m.statusFileSelected = true
				m.statusTable.Focus()
			}
			var cmd tea.Cmd
			m.statusTable, cmd = m.statusTable.Update(msg)
			if len(m.statusPaths) > 0 {
				m.statusFileSelected = true
				m.diffNeedsRefresh = true
				m.syncViewports()
			}
			return m, cmd, true
		}
		if m.focus == paneDiff {
			var cmd tea.Cmd
			m.diffVP, cmd = m.diffVP.Update(msg)
			return m, cmd, true
		}
		if m.focus == paneLog {
			m.logVP.SetContent(m.logBuf.String())
			var cmd tea.Cmd
			m.logVP, cmd = m.logVP.Update(msg)
			return m, cmd, true
		}
	case tea.KeyLeft, tea.KeyRight:
		if m.focus != paneDiff {
			return m, nil, false
		}
		prevMode := m.diffMode
		if msg.Type == tea.KeyLeft {
			m.diffMode = diffModeWorktree
		} else {
			m.diffMode = diffModeStaged
		}
		if m.diffMode != prevMode {
			m.diffNeedsRefresh = true
			m.syncViewports()
		}
		return m, nil, true
	}
	return m, nil, false
}

// handleKey routes keyboard input through overlay, command, and navigation handlers.
func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.helpOpen {
		return m.handleHelpOverlayKey(msg)
	}
	if m.scanning {
		return m.handleScanningKey(msg)
	}

	if mod, cmd, handled := m.handleCommandKey(msg); handled {
		return mod, cmd
	}
	if mod, cmd, handled := m.handleArrowKey(msg); handled {
		return mod, cmd
	}
	return m, nil
}

// handlePassiveInput forwards non-command input to focused interactive widgets.
func (m *model) handlePassiveInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		if m.scanning {
			return m, nil
		}
		if m.focus == paneStatus {
			if !m.statusFileSelected && len(m.statusPaths) > 0 {
				m.statusFileSelected = true
				m.statusTable.Focus()
			}
			var cmd tea.Cmd
			m.statusTable, cmd = m.statusTable.Update(msg)
			if len(m.statusPaths) > 0 {
				m.statusFileSelected = true
				m.diffNeedsRefresh = true
				m.syncViewports()
			}
			return m, cmd
		}
		if m.focus == paneDiff {
			var cmd tea.Cmd
			m.diffVP, cmd = m.diffVP.Update(msg)
			return m, cmd
		}
		if m.focus == paneLog {
			var cmd tea.Cmd
			m.logVP, cmd = m.logVP.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// Update handles Bubble Tea messages and advances application state.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case cspinner.TickMsg:
		return m.handleSpinnerTick(msg)

	case tickMsg:
		return m.handleScanTick()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m.handlePassiveInput(msg)
}
