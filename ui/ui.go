package ui

import (
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	cspinner "github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/boyvinall/dirtygit/scanner"
)

type pane int

const (
	paneRepo pane = iota
	paneStatus
	paneLog
)

const minTermHeight = 22

type tickMsg struct{}

type scanResult struct {
	mgs scanner.MultiGitStatus
	err error
}

type logBuffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := strings.TrimSuffix(string(p), "\n")
	if s == "" {
		return len(p), nil
	}
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			b.lines = append(b.lines, line)
		}
	}
	if len(b.lines) > b.max {
		b.lines = b.lines[len(b.lines)-b.max:]
	}
	return len(p), nil
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Join(b.lines, "\n")
}

type model struct {
	config *scanner.Config
	logBuf *logBuffer

	width  int
	height int

	repositories scanner.MultiGitStatus
	repoList     []string
	cursor       int
	focus        pane

	statusVP viewport.Model
	logVP    viewport.Model

	scanning       bool
	scanResultCh   chan scanResult
	scanProgressCh chan scanner.ScanProgress

	scanProgress scanner.ScanProgress
	scanSpinner  cspinner.Model

	err error

	helpOpen bool

	zoomed     bool
	zoomTarget pane // which pane is fullscreen when zoomed
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func Run(config *scanner.Config) error {
	prevLog := log.Writer()
	defer log.SetOutput(prevLog)

	m := &model{
		config:       config,
		logBuf:       &logBuffer{max: 500},
		focus:        paneRepo,
		scanResultCh: make(chan scanResult, 1),
		scanSpinner: cspinner.New(
			cspinner.WithSpinner(cspinner.MiniDot),
			cspinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("159"))),
		),
	}
	log.SetOutput(m.logBuf)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func (m *model) Init() tea.Cmd {
	return m.beginScan()
}

func (m *model) beginScan() tea.Cmd {
	if m.scanning {
		return nil
	}
	m.err = nil
	m.scanning = true
	m.scanProgress = scanner.ScanProgress{}
	m.scanProgressCh = make(chan scanner.ScanProgress, 256)
	progCh := m.scanProgressCh
	m.scanSpinner = cspinner.New(
		cspinner.WithSpinner(cspinner.MiniDot),
		cspinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("159"))),
	)
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

func scanProgressBar(width, checked, found int) string {
	if width < 1 {
		return ""
	}
	d := found
	if d < 1 {
		d = 1
	}
	filled := checked * width / d
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func shortenScanPath(path string, max int) string {
	if max < 8 || path == "" || len(path) <= max {
		return path
	}
	return "…" + path[len(path)-(max-3):]
}

// scanModalInnerLines is the fixed content row count inside the scan popup (excluding border/padding).
const scanModalInnerLines = 9

func truncateASCII(s string, max int) string {
	if max < 2 || len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// layoutBodies returns inner content heights: repo list, status viewport, log viewport.
// Each framed panel's total row count is panelOuter(body) (see framedBlock).
func (m *model) layoutBodies() (repoBody, statusBody, logBody int) {
	if m.height < minTermHeight || m.width < 20 {
		return 0, 0, 0
	}
	if m.zoomed {
		body := m.height - 3 // panelOuter(body) == m.height
		if body < 3 {
			body = 3
		}
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

	statusBody = effH - 9 - repoBody - logBody
	for statusBody < 3 && logBody > 3 {
		logBody--
		statusBody = effH - 9 - repoBody - logBody
	}
	for statusBody < 3 && repoBody > 3 {
		repoBody--
		statusBody = effH - 9 - repoBody - logBody
	}
	if statusBody < 3 || logBody < 3 || repoBody < 3 {
		return 0, 0, 0
	}
	return repoBody, statusBody, logBody
}

func panelOuter(body int) int {
	return body + 3 // top border + title + body ... + bottom border → body + 1 title + 2 border = body+3
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
	m.statusVP.Width = innerW
	m.statusVP.Height = statusBody
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

func (m *model) refreshStatusContent() {
	repo := m.currentRepo()
	st, ok := m.repositories[repo]
	var b strings.Builder
	if ok && len(st.Status) > 0 {
		b.WriteString(" SW\n")
		b.WriteString("-----\n")
		paths := make([]string, 0, len(st.Status))
		for path := range st.Status {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			status := st.Status[path]
			b.WriteString(fmt.Sprintf(" %c%c  %s\n", status.Staging, status.Worktree, path))
		}
	}
	m.statusVP.SetContent(strings.TrimSuffix(b.String(), "\n"))
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

func helpKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "?", "h", "shift+/":
		return true
	default:
		return false
	}
}

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

func (m *model) helpPanel() string {
	lines := []string{
		"Tab          Focus next pane (Repositories → Status → Log); when zoomed, cycle pane",
		"Shift+Tab    Focus previous pane; when zoomed, cycle pane backward",
		"Enter        Zoom focused pane to fullscreen; Enter again to restore layout",
		"Esc          Exit zoom (when zoomed)",
		"↑ / ↓        Move repo selection or scroll Status / Log",
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

func (m *model) framedBlock(p pane, outerH int, title string, body string) string {
	fg := lipgloss.Color("240")
	if m.focus == p {
		fg = lipgloss.Color("214")
	}
	inner := lipgloss.JoinVertical(lipgloss.Left, m.sectionTitle(p, title), body)
	// lipgloss applies Height to the block *before* drawing borders; borders add two rows.
	// outerH is the full panel height (title + body + borders), so use outerH-2 here.
	return lipgloss.NewStyle().
		Border(borderFor(p, m.focus)).
		BorderForeground(fg).
		Width(m.width - 2).
		Height(outerH - 2).
		MaxHeight(outerH).
		Render(inner)
}

func (m *model) repoListView(innerH int) string {
	selFocused := lipgloss.NewStyle().Background(lipgloss.Color("42")).Foreground(lipgloss.Color("0"))
	selBlurred := lipgloss.NewStyle().Background(lipgloss.Color("248")).Foreground(lipgloss.Color("0"))
	if len(m.repoList) == 0 && !m.scanning {
		return "(no dirty repositories)"
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

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.statusVP.Height == 0 {
			inner := max(8, msg.Width-4)
			m.statusVP = viewport.New(inner, 8)
			m.logVP = viewport.New(inner, 8)
		}
		m.syncViewports()
		return m, nil

	case cspinner.TickMsg:
		if !m.scanning {
			return m, nil
		}
		m.drainScanProgress()
		var spinCmd tea.Cmd
		m.scanSpinner, spinCmd = m.scanSpinner.Update(msg)
		return m, spinCmd

	case tickMsg:
		if !m.scanning {
			return m, nil
		}
		m.drainScanProgress()
		select {
		case r := <-m.scanResultCh:
			m.scanning = false
			m.drainScanProgress()
			m.err = r.err
			if r.err == nil {
				m.repositories = r.mgs
				m.repoList = sortedRepoPaths(r.mgs)
				if m.cursor >= len(m.repoList) {
					m.cursor = max(0, len(m.repoList)-1)
				}
			}
			m.syncViewports()
			return m, nil
		default:
			return m, tickCmd()
		}

	case tea.KeyMsg:
		if m.helpOpen {
			switch msg.String() {
			case "esc":
				m.helpOpen = false
				return m, nil
			default:
				if helpKey(msg) {
					m.helpOpen = false
					return m, nil
				}
				return m, nil
			}
		}
		if m.scanning {
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			default:
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.zoomed {
				m.zoomed = false
				m.syncViewports()
				return m, nil
			}
		case "enter":
			if m.zoomed {
				m.zoomed = false
			} else {
				m.zoomed = true
				m.zoomTarget = m.focus
			}
			m.syncViewports()
			return m, nil
		case "tab":
			if m.zoomed {
				m.zoomTarget = (m.zoomTarget + 1) % 3
				m.focus = m.zoomTarget
			} else {
				m.focus = (m.focus + 1) % 3
			}
			m.syncViewports()
			return m, nil
		case "shift+tab":
			if m.zoomed {
				m.zoomTarget = (m.zoomTarget - 1 + 3) % 3
				m.focus = m.zoomTarget
			} else {
				m.focus = (m.focus - 1 + 3) % 3
			}
			m.syncViewports()
			return m, nil
		case "s":
			if !m.scanning {
				return m, m.beginScan()
			}
			return m, nil
		case "e":
			repo := m.currentRepo()
			if repo != "" {
				_ = exec.Command("code", repo).Run()
			}
			return m, nil
		default:
			if helpKey(msg) {
				m.helpOpen = true
				return m, nil
			}
		}

		switch msg.Type {
		case tea.KeyUp, tea.KeyDown:
			if m.focus == paneRepo {
				if msg.Type == tea.KeyUp && m.cursor > 0 {
					m.cursor--
				} else if msg.Type == tea.KeyDown && m.cursor < len(m.repoList)-1 {
					m.cursor++
				}
				m.refreshStatusContent()
				return m, nil
			}
			if m.focus == paneStatus {
				var cmd tea.Cmd
				m.statusVP, cmd = m.statusVP.Update(msg)
				return m, cmd
			}
			if m.focus == paneLog {
				m.logVP.SetContent(m.logBuf.String())
				var cmd tea.Cmd
				m.logVP, cmd = m.logVP.Update(msg)
				return m, cmd
			}
		}
	}

	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		if m.scanning {
			return m, nil
		}
		if m.focus == paneStatus {
			var cmd tea.Cmd
			m.statusVP, cmd = m.statusVP.Update(msg)
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

func (m *model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.helpOpen {
		help := m.helpPanel()
		h := m.height
		if h < 1 {
			h = minTermHeight
		}
		return lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center, help,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
	}
	if m.height < minTermHeight {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Need bigger screen (min height 22).")
	}
	if m.scanning {
		popup := m.scanProgressPopup()
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup,
			lipgloss.WithWhitespaceChars("░"),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("236")),
			lipgloss.WithWhitespaceBackground(lipgloss.Color("235")))
	}

	repoBody, statusBody, logBody := m.layoutBodies()
	if repoBody == 0 && statusBody == 0 && logBody == 0 {
		return ""
	}
	m.syncViewports()

	if m.zoomed {
		switch m.zoomTarget {
		case paneRepo:
			return m.framedBlock(paneRepo, m.height, "Repositories", m.repoListView(repoBody))
		case paneStatus:
			return m.framedBlock(paneStatus, m.height, "Status", m.statusVP.View())
		case paneLog:
			m.logVP.SetContent(m.logBuf.String())
			return m.framedBlock(paneLog, m.height, "Log", m.logVP.View())
		}
	}

	repoOuter := panelOuter(repoBody)
	statusOuter := panelOuter(statusBody)
	logOuter := panelOuter(logBody)

	repoBlock := m.framedBlock(paneRepo, repoOuter, "Repositories", m.repoListView(repoBody))
	statusBlock := m.framedBlock(paneStatus, statusOuter, "Status", m.statusVP.View())
	m.logVP.SetContent(m.logBuf.String())
	logBlock := m.framedBlock(paneLog, logOuter, "Log", m.logVP.View())

	stack := lipgloss.JoinVertical(lipgloss.Left, repoBlock, statusBlock, logBlock)

	if m.err != nil {
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

	return stack
}
