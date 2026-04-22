package ui

import (
	"log"
	"time"

	cspinner "github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/boyvinall/dirtygit/scanner"
)

func newScanSpinner() cspinner.Model {
	return cspinner.New(
		cspinner.WithSpinner(cspinner.MiniDot),
		cspinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("159"))),
	)
}

// tickCmd schedules the next scan progress poll.
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Run starts the terminal UI for scanning and navigating dirty repositories.
func Run(config *scanner.Config) error {
	prevLog := log.Writer()
	defer log.SetOutput(prevLog)

	m := &model{
		config:           config,
		logBuf:           &logBuffer{max: 500},
		focus:            paneRepo,
		scanResultCh:     make(chan scanResult, 1),
		diffMode:         diffModeWorktree,
		diffNeedsRefresh: true,
		scanSpinner:      newScanSpinner(),
	}
	m.statusTable = newStatusTable()
	m.branchTable = newBranchTable()
	log.SetOutput(m.logBuf)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// Init starts the initial repository scan when the app launches.
func (m *model) Init() tea.Cmd {
	return m.beginScan()
}

// beginScan kicks off an asynchronous repository scan.
func (m *model) beginScan() tea.Cmd {
	if m.scanning {
		return nil
	}
	m.err = nil
	m.whyRepoOpen = false
	m.deleteRepoConfirmOpen = false
	m.scanning = true
	m.scanProgress = scanner.ScanProgress{}
	m.scanProgressCh = make(chan scanner.ScanProgress, 256)
	progCh := m.scanProgressCh
	m.scanSpinner = newScanSpinner()
	go func() {
		mgs, err := scanner.ScanWithProgress(m.config, func(p scanner.ScanProgress) {
			select {
			case progCh <- p:
			default:
			}
		})
		m.scanResultCh <- scanResult{mgs: mgs, err: err}
	}()
	return tea.Batch(tickCmd(), func() tea.Msg {
		return m.scanSpinner.Tick()
	})
}

// drainScanProgress consumes queued progress updates and keeps the newest one.
func (m *model) drainScanProgress() {
	if m.scanProgressCh == nil {
		return
	}
	for {
		select {
		case p := <-m.scanProgressCh:
			m.scanProgress = p
		default:
			return
		}
	}
}
