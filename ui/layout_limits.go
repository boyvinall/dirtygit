package ui

// Layout limits shared across status layout, mouse handling, and window sizing.
const (
	// layoutFrameStackOuterRows is vertical space outside the four main body areas
	// (status bar, titles, log chrome, etc.): effective height = m.height minus this.
	layoutFrameStackOuterRows = 8

	// layoutRepoOuterMaxBottomMargin is subtracted in mouse resize to cap repo height above log.
	layoutRepoOuterMaxBottomMargin = 10

	layoutMinPanelOuter    = 5
	layoutMinBodyLines     = 3
	layoutMinSpareForSplit = 6

	// layoutMinTermWidth and layoutMinTermHeight are minimum terminal columns and rows
	// for the main split TUI (mouse, resize, and pane layout assume at least this size).
	layoutMinTermWidth  = 20
	layoutMinTermHeight = 22

	// layoutMinStatusBranchesColumn is a minimum for either status or branches column
	// when splits are computed or user-resized.
	layoutMinStatusBranchesColumn = 10

	// layoutMinInnerContentWidth is a floor for content inside borders or viewports
	// (also used as a placeholder size before syncViewports runs).
	layoutMinInnerContentWidth = 8

	// mouseBorderHitSlop is how many cells a cursor may be off a border and still
	// count as "on" the resize handle.
	mouseBorderHitSlop = 1
)

// Defaults for first layout before / outside autoLayoutBodies.
const (
	layoutDefaultLogBodyLines           = 4
	layoutDefaultTableViewRows          = 6
	layoutStatusWorktreeStagedNarrowCol = 10
)

// Modal and overlay box geometry (borders + horizontal padding around inner text).
const (
	layoutModalSideGutter  = 6
	layoutModalBoxMinWidth = layoutMinTermWidth
)

// layoutPathTruncationEllipsis is rune count reserved when prefixing a path with "…".
const layoutPathTruncationEllipsis = 3

// Centered modal max widths (box outer width before placeCentered*).
const (
	layoutScanProgressModalMaxBox  = 64
	layoutWhyAndConfirmModalMaxBox = 72
)

// Error overlay: horizontal margin and max text box width.
const (
	layoutErrorOverlayHPad     = 4
	layoutErrorOverlayMaxWidth = 80
)
