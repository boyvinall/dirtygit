package ui

import (
	"strings"
	"sync"

	cspinner "github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"

	"github.com/boyvinall/dirtygit/scanner"
)

type pane int

const (
	paneRepo pane = iota
	paneStatus
	paneDiff
	paneLog
)

const minTermHeight = 22

type tickMsg struct{}

type diffMode int

const (
	diffModeWorktree diffMode = iota
	diffModeStaged
)

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
	for line := range strings.SplitSeq(s, "\n") {
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

	statusTable        table.Model
	statusPaths        []string
	statusFileSelected bool
	diffMode           diffMode
	diffNeedsRefresh   bool
	diffContent        string
	diffErr            error
	diffVP             viewport.Model
	logVP              viewport.Model

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
