package vimtextarea

import "reapo/internal/tui/completion"

type Mode int

const (
	Normal Mode = iota
	Insert
	Visual
)

type Position struct {
	Row int
	Col int
}

type Selection struct {
	Start Position
	End   Position
}

type CommandState struct {
	operator       string // "d", "y", "c", etc.
	count          int    // count before operator
	motionCount    int    // count before motion
	awaitingMotion bool
}

type UndoState struct {
	content []string
	cursor  Position
}

type UndoHistory struct {
	states  []UndoState
	index   int
	maxSize int
}

type Model struct {
	mode        Mode
	content     []string
	cursor      Position
	selection   *Selection
	clipboard   string
	width       int
	height      int
	placeholder string
	focused     bool

	// Viewport/scrolling
	scrollOffset int // Line number at top of viewport

	// Navigation state
	lastFindChar rune
	lastFindDir  int // 1 for forward, -1 for backward
	lastFindType int // 1 for 'f', 2 for 't'

	// Search state
	searchPattern string
	searchResults []Position
	searchIndex   int

	// Command state
	commandState CommandState
	inputCount   int // Current number being typed

	// Replace state
	awaitingReplaceChar bool
	replaceCount        int

	// Insert session tracking
	inInsertSession bool // Track if we're currently in an insert session

	// Undo system
	undoHistory UndoHistory

	// Completion system
	completionState    completion.CompletionState
	completionEngine   *completion.CompletionEngine
	completionStartPos Position // Track where completion started for insertion
}
