package ui

import (
	"strings"
	"sync"

	cspinner "github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"

	"github.com/boyvinall/dirtygit/scanner"
)

// pane identifies which UI pane currently owns focus.
type pane int

const (
	paneRepo pane = iota
	paneStatus
	paneBranches
	paneDiff
	paneLog
)

const minTermHeight = 22

// tickMsg drives periodic polling while scans are running.
type tickMsg struct{}

// diffMode selects whether Diff shows worktree or staged changes.
type diffMode int

const (
	diffModeWorktree diffMode = iota
	diffModeStaged
)

// scanResult carries the finished scan data and any scan error.
type scanResult struct {
	mgs scanner.MultiGitStatus
	err error
}

// logBuffer stores a bounded in-memory log stream for the Log pane.
type logBuffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

// Write appends incoming log bytes while keeping only the newest lines.
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

// String returns the buffered log lines joined by newlines.
func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Join(b.lines, "\n")
}

// model contains all Bubble Tea UI state for dirtygit.
type model struct {
	config *scanner.Config
	logBuf *logBuffer

	width  int
	height int

	repositories  scanner.MultiGitStatus
	repoList      []string
	repoScrollTop int // first visible repo index when the list exceeds pane height
	cursor        int
	focus         pane

	statusTable        table.Model
	statusPaths        []string
	statusFileSelected bool
	branchTable        table.Model
	diffMode           diffMode
	diffNeedsRefresh   bool
	// repoNavSettleGen increments on each repo list movement; only the matching
	// repoNavSettledMsg applies heavy pane updates so rapid key repeat debounces.
	repoNavSettleGen uint64
	// diffRequestGen increments when we're ready to load a diff; only the matching
	// runDiffForGen handler loads git so rapid scrolling does not run stale work.
	diffRequestGen uint64
	diffContent    string
	diffErr        error
	diffVP         viewport.Model
	logVP          viewport.Model

	scanning       bool
	scanResultCh   chan scanResult
	scanProgressCh chan scanner.ScanProgress

	scanProgress scanner.ScanProgress
	scanSpinner  cspinner.Model

	err error

	helpOpen    bool
	whyRepoOpen bool
	// deleteRepoConfirmOpen shows the recursive-delete confirmation for the selected repository path.
	deleteRepoConfirmOpen bool
	// deleteStatusFileConfirmOpen asks before deleting the selected status path from disk.
	deleteStatusFileConfirmOpen bool
	// deleteStatusFilePendingRel is the repo-relative path pending confirmation (git status form).
	deleteStatusFilePendingRel string
	// checkoutStatusFileConfirmOpen asks before git checkout HEAD -- path.
	checkoutStatusFileConfirmOpen bool
	// checkoutStatusFilePendingRel is the repo-relative path pending checkout confirmation.
	checkoutStatusFilePendingRel string
	// deleteConfirmYes is true when "Yes" is highlighted; default is false ("No" highlighted).
	deleteConfirmYes bool

	zoomed     bool
	zoomTarget pane // which pane is fullscreen when zoomed

	// layoutUseCustomVertical is true after the user resizes a horizontal seam;
	// layoutRepoBody, layoutStatusBody, and layoutLogBody are inner body heights.
	layoutUseCustomVertical bool
	layoutRepoBody          int
	layoutStatusBody        int
	layoutLogBody           int

	// layoutBranchesOuter is the framed Branches column width in cells (0 = automatic).
	layoutBranchesOuter int

	resizeDrag resizeSplit
}
