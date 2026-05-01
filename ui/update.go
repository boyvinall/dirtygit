package ui

import (
	"log"
	"os"
	"time"

	cspinner "github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// repoNavSettleDebounce is the quiet period after the last Up/Down on the repo
// list before status/branches/diff are refreshed. Keeps list scrolling smooth.
const repoNavSettleDebounce = 100 * time.Millisecond

// repoNavSettledMsg is sent after the debounce; see repoNavSettleDebounce.
type repoNavSettledMsg struct{ gen uint64 }

// runDiffForGen is sent after a repo selection change so the diff loads on a
// follow-up message. That lets the first frame re-render the repo list without
// blocking on git.
type runDiffForGen struct{ gen uint64 }

// handleWindowSize updates dimensions and recomputes pane layout.
func (m *model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	if m.logVP.Height == 0 {
		inner := max(layoutMinInnerContentWidth, msg.Width-4)
		m.logVP = viewport.New(inner, layoutMinInnerContentWidth)
	}
	if m.diffVP.Height == 0 {
		inner := max(layoutMinInnerContentWidth, msg.Width-4)
		m.diffVP = viewport.New(inner, layoutMinInnerContentWidth)
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
	m.repoList = r.mgs.SortedRepoPaths()
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
	case "ctrl+c", "q":
		return m, tea.Quit
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

// handleWhyRepoOverlayKey processes keys while the "why listed" modal is open.
func (m *model) handleWhyRepoOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "w":
		m.whyRepoOpen = false
		return m, nil
	default:
		return m, nil
	}
}

// handleDeleteRepoConfirmKey processes keys while the delete-directory confirmation is open.
func (m *model) handleDeleteRepoConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.deleteRepoConfirmOpen = false
		return m, nil
	case "n":
		m.deleteConfirmYes = false
		return m, nil
	case "y":
		m.deleteConfirmYes = true
		return m, nil
	case "left", "h":
		m.deleteConfirmYes = false
		return m, nil
	case "right", "l":
		m.deleteConfirmYes = true
		return m, nil
	case "enter":
		m.deleteRepoConfirmOpen = false
		if !m.deleteConfirmYes {
			return m, nil
		}
		m.deleteSelectedRepoFromDisk()
		return m, nil
	default:
		return m, nil
	}
}

// handleCheckoutStatusFileConfirmKey processes keys while the checkout-from-HEAD confirmation is open.
func (m *model) handleCheckoutStatusFileConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.checkoutStatusFileConfirmOpen = false
		m.checkoutStatusFilePendingRel = ""
		return m, nil
	case "n":
		m.deleteConfirmYes = false
		return m, nil
	case "y":
		m.deleteConfirmYes = true
		return m, nil
	case "left", "h":
		m.deleteConfirmYes = false
		return m, nil
	case "right", "l":
		m.deleteConfirmYes = true
		return m, nil
	case "enter":
		m.checkoutStatusFileConfirmOpen = false
		pending := m.checkoutStatusFilePendingRel
		m.checkoutStatusFilePendingRel = ""
		if !m.deleteConfirmYes {
			return m, nil
		}
		repo := m.currentRepo()
		if err := gitCheckoutHeadPath(repo, pending); err != nil {
			log.Printf("git: %v", err)
		} else {
			m.refreshRepoStatusAfterGit()
		}
		m.diffNeedsRefresh = true
		m.syncViewports()
		return m, nil
	default:
		return m, nil
	}
}

// handleDeleteStatusFileConfirmKey processes keys while the delete-status-file confirmation is open.
func (m *model) handleDeleteStatusFileConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.deleteStatusFileConfirmOpen = false
		m.deleteStatusFilePendingRel = ""
		return m, nil
	case "n":
		m.deleteConfirmYes = false
		return m, nil
	case "y":
		m.deleteConfirmYes = true
		return m, nil
	case "left", "h":
		m.deleteConfirmYes = false
		return m, nil
	case "right", "l":
		m.deleteConfirmYes = true
		return m, nil
	case "enter":
		m.deleteStatusFileConfirmOpen = false
		pending := m.deleteStatusFilePendingRel
		m.deleteStatusFilePendingRel = ""
		if !m.deleteConfirmYes {
			return m, nil
		}
		m.deletePendingStatusFileFromDisk(pending)
		return m, nil
	default:
		return m, nil
	}
}

// deletePendingStatusFileFromDisk removes the given repo-relative path after confirmation.
func (m *model) deletePendingStatusFileFromDisk(gitRelPath string) {
	repo := m.currentRepo()
	if repo == "" || gitRelPath == "" {
		return
	}
	if err := removeStatusPathOnDisk(repo, gitRelPath); err != nil {
		log.Printf("remove status path: %v", err)
		return
	}
	m.refreshRepoStatusAfterGit()
	m.diffNeedsRefresh = true
	m.syncViewports()
}

// deleteSelectedRepoFromDisk runs os.RemoveAll on the selected repository path and
// updates list state. Logs failures; on success the entry is removed from the UI.
func (m *model) deleteSelectedRepoFromDisk() {
	repo := m.currentRepo()
	if repo == "" {
		return
	}
	if err := os.RemoveAll(repo); err != nil {
		log.Printf("remove %q: %v", repo, err)
		return
	}
	m.repositories.Delete(repo)
	m.repoList = m.repositories.SortedRepoPaths()
	if m.cursor >= len(m.repoList) {
		m.cursor = max(0, len(m.repoList)-1)
	}
	m.statusFileSelected = false
	m.diffNeedsRefresh = true
	m.syncViewports()
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
			m.diffNeedsRefresh = true
			// syncViewports can no-op before the first WindowSizeMsg; still update the table widget.
			m.applyStatusTableFocusAndStyles()
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
	case "t":
		m.openTerminalInCurrentRepo()
		return m, nil, true
	case "w":
		if m.repoPaneReady() {
			m.whyRepoOpen = true
			return m, nil, true
		}
		return m, nil, false
	case "D":
		if path, ok := m.selectedStatusPathForOps(); ok {
			m.deleteStatusFileConfirmOpen = true
			m.deleteStatusFilePendingRel = path
			m.deleteConfirmYes = false
			return m, nil, true
		}
		if m.repoPaneReady() {
			m.deleteRepoConfirmOpen = true
			m.deleteConfirmYes = false
			return m, nil, true
		}
		return m, nil, false
	case "a", "r":
		if path, ok := m.selectedStatusPathForOps(); ok {
			repo := m.currentRepo()
			var err error
			if msg.String() == "a" {
				err = gitAdd(repo, path)
			} else {
				err = gitResetPath(repo, path)
			}
			if err != nil {
				log.Printf("git: %v", err)
			} else {
				m.refreshRepoStatusAfterGit()
			}
			m.diffNeedsRefresh = true
			m.syncViewports()
			return m, nil, true
		}
	case "C":
		if path, ok := m.selectedStatusPathForOps(); ok {
			m.checkoutStatusFileConfirmOpen = true
			m.checkoutStatusFilePendingRel = path
			m.deleteConfirmYes = false
			return m, nil, true
		}
	default:
		if helpKey(msg) {
			m.helpOpen = true
			return m, nil, true
		}
	}
	return m, nil, false
}

// listKeyScrollPage is the step size for Shift+↑/↓ in scrollable lists and viewports.
const listKeyScrollPage = 10

// handleArrowKey applies directional key behavior for the focused pane.
func (m *model) handleArrowKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown, tea.KeyShiftUp, tea.KeyShiftDown:
		step := 1
		if msg.Type == tea.KeyShiftUp || msg.Type == tea.KeyShiftDown {
			step = listKeyScrollPage
		}
		up := msg.Type == tea.KeyUp || msg.Type == tea.KeyShiftUp
		down := msg.Type == tea.KeyDown || msg.Type == tea.KeyShiftDown

		if m.focus == paneRepo {
			prev := m.cursor
			if up && m.cursor > 0 {
				m.cursor = max(0, m.cursor-step)
			} else if down && len(m.repoList) > 0 && m.cursor < len(m.repoList)-1 {
				m.cursor = min(len(m.repoList)-1, m.cursor+step)
			}
			if m.cursor != prev {
				m.statusFileSelected = false
				m.diffNeedsRefresh = true
				m.syncRepoListScrollOnly()
				m.repoNavSettleGen++
				g := m.repoNavSettleGen
				return m, tea.Tick(repoNavSettleDebounce, func(t time.Time) tea.Msg {
					return repoNavSettledMsg{gen: g}
				}), true
			}
			return m, nil, true
		}
		if m.focus == paneStatus {
			if !m.statusFileSelected && len(m.statusPaths) > 0 {
				m.statusFileSelected = true
				m.statusTable.Focus()
			}
			var cmd tea.Cmd
			if step == listKeyScrollPage {
				if up {
					m.statusTable.MoveUp(listKeyScrollPage)
				} else {
					m.statusTable.MoveDown(listKeyScrollPage)
				}
			} else {
				m.statusTable, cmd = m.statusTable.Update(msg)
			}
			if len(m.statusPaths) > 0 {
				m.statusFileSelected = true
				m.diffNeedsRefresh = true
				m.syncViewports()
			}
			return m, cmd, true
		}
		if m.focus == paneDiff {
			if step == listKeyScrollPage {
				if up {
					m.diffVP.ScrollUp(listKeyScrollPage)
				} else {
					m.diffVP.ScrollDown(listKeyScrollPage)
				}
				return m, nil, true
			}
			var cmd tea.Cmd
			m.diffVP, cmd = m.diffVP.Update(msg)
			return m, cmd, true
		}
		if m.focus == paneLog {
			m.setLogVPContent()
			if step == listKeyScrollPage {
				if up {
					m.logVP.ScrollUp(listKeyScrollPage)
				} else {
					m.logVP.ScrollDown(listKeyScrollPage)
				}
				return m, nil, true
			}
			var cmd tea.Cmd
			m.logVP, cmd = m.logVP.Update(msg)
			return m, cmd, true
		}
	case tea.KeyLeft, tea.KeyRight:
		if m.focus != paneDiff && m.focus != paneStatus {
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

func (m *model) scheduleRunDiff() tea.Cmd {
	m.diffRequestGen++
	g := m.diffRequestGen
	return tea.Tick(0, func(t time.Time) tea.Msg {
		return runDiffForGen{gen: g}
	})
}

// handleRepoNavSettled applies status/branches/diff after keyboard repo list
// navigation has been quiet for repoNavSettleDebounce. Stale generations are
// dropped when the user is still moving.
func (m *model) handleRepoNavSettled(msg repoNavSettledMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.repoNavSettleGen {
		return m, nil
	}
	m.applyViewportAndPanes(false)
	return m, m.scheduleRunDiff()
}

// handleRunDiffForGen runs git diff after the list/panes have been laid out. If
// a newer request superseded this one, reschedules a tick for the latest gen.
func (m *model) handleRunDiffForGen(msg runDiffForGen) (tea.Model, tea.Cmd) {
	if msg.gen != m.diffRequestGen {
		g := m.diffRequestGen
		return m, tea.Tick(0, func(t time.Time) tea.Msg {
			return runDiffForGen{gen: g}
		})
	}
	if !m.diffNeedsRefresh {
		return m, nil
	}
	m.syncViewports()
	return m, nil
}

// handleKey routes keyboard input through overlay, command, and navigation handlers.
func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.helpOpen {
		return m.handleHelpOverlayKey(msg)
	}
	if m.deleteRepoConfirmOpen {
		return m.handleDeleteRepoConfirmKey(msg)
	}
	if m.deleteStatusFileConfirmOpen {
		return m.handleDeleteStatusFileConfirmKey(msg)
	}
	if m.checkoutStatusFileConfirmOpen {
		return m.handleCheckoutStatusFileConfirmKey(msg)
	}
	if m.whyRepoOpen {
		return m.handleWhyRepoOverlayKey(msg)
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

// handleMouseFocusClick moves keyboard focus to the pane under the cursor on
// left-button press. Returns true when the event is consumed (focus changed);
// otherwise the caller should forward the mouse message as usual.
func (m *model) handleMouseFocusClick(msg tea.MouseMsg) bool {
	if !m.mouseFocusClickReady() {
		return false
	}
	if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionPress {
		return false
	}
	p, ok := m.paneAtTerminalCell(msg.X, msg.Y)
	if !ok {
		return false
	}
	if p == m.focus {
		return false
	}
	m.focus = p
	m.syncViewports()
	return true
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
	case repoNavSettledMsg:
		return m.handleRepoNavSettled(msg)
	case runDiffForGen:
		return m.handleRunDiffForGen(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case cspinner.TickMsg:
		return m.handleSpinnerTick(msg)

	case tickMsg:
		return m.handleScanTick()

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		if m.handleMousePaneResize(msg) {
			return m, nil
		}
		if m.handleMouseFocusClick(msg) {
			return m, nil
		}
		if ok, cmd := m.handleMousePaneLineSelect(msg); ok {
			return m, cmd
		}
		return m.handlePassiveInput(msg)
	}

	return m.handlePassiveInput(msg)
}
