package vimtextarea

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"reapo/internal/logger"
)

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
}

func New() Model {
	initialContent := []string{""}
	m := Model{
		mode:         Insert,
		content:      initialContent,
		cursor:       Position{0, 0},
		focused:      true,
		width:        80,
		height:       1,
		scrollOffset: 0,
		undoHistory: UndoHistory{
			states:  make([]UndoState, 0, 100),
			index:   -1,
			maxSize: 100,
		},
	}

	// Save initial state
	m = m.saveUndoState()

	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	switch m.mode {
	case Normal:
		return m.handleNormalMode(key)
	case Insert:
		return m.handleInsertMode(key, msg)
	case Visual:
		return m.handleVisualMode(key)
	}

	return m, nil
}

func (m Model) handleNormalMode(key string) (Model, tea.Cmd) {
	// If we're awaiting a replacement character, handle it
	if m.awaitingReplaceChar {
		if len(key) == 1 {
			m.replaceCharacter(key, m.replaceCount)
			m.awaitingReplaceChar = false
			m.replaceCount = 0
		}
		return m, nil
	}

	// Handle counts
	if len(key) == 1 && key >= "1" && key <= "9" && m.inputCount == 0 && !m.commandState.awaitingMotion {
		m.inputCount = int(key[0] - '0')
		return m, nil
	} else if len(key) == 1 && key >= "0" && key <= "9" && (m.inputCount > 0 || m.commandState.awaitingMotion) {
		if m.commandState.awaitingMotion {
			m.commandState.motionCount = m.commandState.motionCount*10 + int(key[0]-'0')
		} else {
			m.inputCount = m.inputCount*10 + int(key[0]-'0')
		}
		return m, nil
	}

	// If we're awaiting a motion, handle motion commands
	if m.commandState.awaitingMotion {
		return m.handleMotionCommand(key)
	}

	count := m.inputCount
	if count == 0 {
		count = 1
	}

	switch key {
	// Basic movement
	case "h", "left":
		m.cursor = m.moveLeft(count)
		m = m.adjustScroll()
	case "j", "down":
		m.cursor = m.moveDown(count)
		m = m.adjustScroll()
	case "k", "up":
		m.cursor = m.moveUp(count)
		m = m.adjustScroll()
	case "l", "right":
		m.cursor = m.moveRight(count)
		m = m.adjustScroll()

	// Word movement
	case "w":
		m.cursor = m.moveWordForward(count)
		m = m.adjustScroll()
	case "W":
		m.cursor = m.moveWORDForward(count)
		m = m.adjustScroll()
	case "b":
		m.cursor = m.moveWordBackward(count)
		m = m.adjustScroll()
	case "B":
		m.cursor = m.moveWORDBackward(count)
		m = m.adjustScroll()
	case "e":
		m.cursor = m.moveWordEnd(count)
		m = m.adjustScroll()
	case "E":
		m.cursor = m.moveWORDEnd(count)
		m = m.adjustScroll()

	// Line movement
	case "0":
		m.cursor.Col = 0
		m = m.adjustScroll()
	case "^":
		m.cursor = m.moveToFirstNonWhitespace()
		m = m.adjustScroll()
	case "_":
		m.cursor = m.moveToFirstNonWhitespace()
		m = m.adjustScroll()
	case "$":
		m.cursor = m.moveToEndOfLine()
		m = m.adjustScroll()

	// Document navigation
	case "g":
		return m.handleGCommand()
	case "G":
		if count == 1 {
			// No count specified, go to last line
			m.cursor = m.moveToLastLine()
		} else {
			// Count specified, go to that line number
			m.cursor = m.moveToLine(count - 1)
		}
		m = m.adjustScroll()

	// Find/till
	case "f", "F", "t", "T":
		return m.handleFindCommand(key)
	case ";":
		m.cursor = m.repeatFind(1)
	case ",":
		m.cursor = m.repeatFind(-1)

	// Insert mode entry
	case "i":
		m = m.startInsertSession()
		m.mode = Insert
	case "I":
		m = m.startInsertSession()
		m.cursor = m.moveToFirstNonWhitespace()
		m = m.adjustScroll()
		m.mode = Insert
	case "a":
		m = m.startInsertSession()
		m.cursor = m.moveRight(1)
		m = m.adjustScroll()
		m.mode = Insert
	case "A":
		m = m.startInsertSession()
		m.cursor = m.moveToEndOfLine()
		m = m.adjustScroll()
		m.mode = Insert
	case "o":
		m = m.startInsertSession()
		m = m.insertNewLineBelow()
		m.mode = Insert
	case "O":
		m = m.startInsertSession()
		m = m.insertNewLineAbove()
		m.mode = Insert

	// Visual mode
	case "v":
		m.mode = Visual
		m.selection = &Selection{Start: m.cursor, End: m.cursor}

	// Text operations
	case "x":
		m = m.deleteChar(count)
	case "r":
		m.awaitingReplaceChar = true
		m.replaceCount = count
		return m, nil
	case "d":
		m.commandState.operator = "d"
		m.commandState.count = count
		m.commandState.awaitingMotion = true
		m.commandState.motionCount = 0
		return m, nil
	case "D":
		m = m.deleteToEndOfLine()
	case "C":
		m = m.changeToEndOfLine()
	case "Y":
		m = m.yankLine()
	case "y":
		m.commandState.operator = "y"
		m.commandState.count = count
		m.commandState.awaitingMotion = true
		m.commandState.motionCount = 0
		return m, nil
	case "c":
		m.commandState.operator = "c"
		m.commandState.count = count
		m.commandState.awaitingMotion = true
		m.commandState.motionCount = 0
		return m, nil
	case "p":
		m = m.pasteAfter()
	case "P":
		m = m.pasteBefore()

	// Undo/Redo
	case "u":
		m = m.undo()
	case "ctrl+r":
		m = m.redo()
	}

	m.inputCount = 0
	m.cursor = m.validateCursor(m.cursor)
	return m, nil
}

func (m Model) handleInsertMode(key string, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch key {
	case "esc":
		m = m.endInsertSession()
		m.mode = Normal
		m.cursor = m.moveLeft(1)
		m = m.adjustScroll()
	case "enter":
		m = m.insertNewLine()
	case "backspace":
		m = m.backspace()
	case "tab":
		m = m.insertText("\t")
	default:
		if len(msg.Runes) > 0 {
			m = m.insertText(string(msg.Runes))
		}
	}

	m.cursor = m.validateCursor(m.cursor)
	return m, nil
}

func (m Model) handleVisualMode(key string) (Model, tea.Cmd) {
	switch key {
	case "esc":
		m.mode = Normal
		m.selection = nil

	// Movement in visual mode updates selection
	case "h", "left":
		m.cursor = m.moveLeft(1)
		m = m.adjustScroll()
		m.selection.End = m.cursor
	case "j", "down":
		m.cursor = m.moveDown(1)
		m = m.adjustScroll()
		m.selection.End = m.cursor
	case "k", "up":
		m.cursor = m.moveUp(1)
		m = m.adjustScroll()
		m.selection.End = m.cursor
	case "l", "right":
		m.cursor = m.moveRight(1)
		m = m.adjustScroll()
		m.selection.End = m.cursor

	// Text operations
	case "d", "x":
		m = m.deleteSelection()
		m.mode = Normal
		m.selection = nil
	case "y":
		m = m.yankSelection()
		m.mode = Normal
		m.selection = nil
	}

	m.cursor = m.validateCursor(m.cursor)
	return m, nil
}

func (m Model) handleMotionCommand(key string) (Model, tea.Cmd) {
	motionCount := m.commandState.motionCount
	if motionCount == 0 {
		motionCount = 1
	}

	operatorCount := m.commandState.count
	totalCount := operatorCount * motionCount

	var startPos, endPos Position
	startPos = m.cursor

	switch key {
	// Line operations (dd, yy, cc)
	case m.commandState.operator:
		return m.executeLineOperation(totalCount)

	// Basic motions
	case "h", "left":
		endPos = m.moveLeft(motionCount)
	case "l", "right":
		endPos = m.moveRight(motionCount)
	case "j", "down":
		endPos = m.moveDown(motionCount)
	case "k", "up":
		endPos = m.moveUp(motionCount)

	// Word motions
	case "w":
		endPos = m.moveWordForward(motionCount)
	case "W":
		endPos = m.moveWORDForward(motionCount)
	case "b":
		endPos = m.moveWordBackward(motionCount)
	case "B":
		endPos = m.moveWORDBackward(motionCount)
	case "e":
		endPos = m.moveWordEnd(motionCount)
	case "E":
		endPos = m.moveWORDEnd(motionCount)

	// Line motions
	case "0":
		endPos = Position{Row: m.cursor.Row, Col: 0}
	case "^":
		endPos = m.moveToFirstNonWhitespace()
	case "_":
		endPos = m.moveToFirstNonWhitespace()
	case "$":
		endPos = m.moveToEndOfLine()

	// Document motions
	case "G":
		if motionCount == 1 {
			endPos = m.moveToLastLine()
		} else {
			endPos = m.moveToLine(motionCount - 1)
		}

	default:
		// Invalid motion, cancel command
		m.commandState = CommandState{}
		return m, nil
	}

	// Execute the operation with the motion
	m = m.executeOperation(startPos, endPos)
	logger.Debug("Undo history size: %d", len(m.undoHistory.states))
	m.commandState = CommandState{}
	logger.Debug("Undo history size: %d", len(m.undoHistory.states))
	m.cursor = m.validateCursor(m.cursor)
	logger.Debug("Undo history size: %d", len(m.undoHistory.states))
	return m, nil
}

func (m Model) executeLineOperation(count int) (Model, tea.Cmd) {
	switch m.commandState.operator {
	case "d":
		m = m.deleteLines(count)
	case "y":
		m = m.yankLines(count)
	case "c":
		m = m.changeLines(count)
	}

	m.commandState = CommandState{}
	m.cursor = m.validateCursor(m.cursor)
	return m, nil
}

func (m Model) executeOperation(startPos, endPos Position) Model {
	// Ensure proper order
	if startPos.Row > endPos.Row || (startPos.Row == endPos.Row && startPos.Col > endPos.Col) {
		startPos, endPos = endPos, startPos
	}

	// For word-end motions in delete/change operations, include the character at endPos
	if m.commandState.operator == "d" || m.commandState.operator == "c" {
		// Check if this was an 'e' motion by seeing if endPos is at a word boundary
		if endPos.Row < len(m.content) && endPos.Col < len(m.content[endPos.Row]) {
			line := m.content[endPos.Row]
			// If we're at the last character of a word, include it in the deletion
			if endPos.Col < len(line) && !m.isWordBoundary(line[endPos.Col], false) &&
				(endPos.Col+1 >= len(line) || m.isWordBoundary(line[endPos.Col+1], false)) {
				endPos.Col++
			}
		}
	}

	switch m.commandState.operator {
	case "d":
		m = m.deleteRange(startPos, endPos)
	case "y":
		m = m.yankRange(startPos, endPos)
	case "c":
		m = m.changeRange(startPos, endPos)
	}
	return m
}

func (m Model) handleGCommand() (Model, tea.Cmd) {
	// This would need a state machine for multi-key commands
	// For now, just handle 'gg'
	m.cursor = Position{0, 0}
	m.adjustScroll()
	return m, nil
}

func (m Model) handleFindCommand(cmd string) (Model, tea.Cmd) {
	// This would need to wait for the next character
	// For now, simplified implementation
	return m, nil
}

// Movement methods
func (m Model) moveLeft(count int) Position {
	pos := m.cursor
	pos.Col = max(0, pos.Col-count)
	return pos
}

func (m Model) moveRight(count int) Position {
	pos := m.cursor
	if pos.Row < len(m.content) {
		lineLen := len(m.content[pos.Row])
		pos.Col = min(lineLen, pos.Col+count)
	}
	return pos
}

func (m Model) moveUp(count int) Position {
	pos := m.cursor
	pos.Row = max(0, pos.Row-count)
	return m.validateCursor(pos)
}

func (m Model) moveDown(count int) Position {
	pos := m.cursor
	pos.Row = min(len(m.content)-1, pos.Row+count)
	return m.validateCursor(pos)
}

func (m Model) moveToFirstNonWhitespace() Position {
	pos := m.cursor
	if pos.Row < len(m.content) {
		line := m.content[pos.Row]
		for i, r := range line {
			if !unicode.IsSpace(r) {
				pos.Col = i
				break
			}
		}
	}
	return pos
}

func (m Model) moveToEndOfLine() Position {
	pos := m.cursor
	if pos.Row < len(m.content) {
		pos.Col = max(0, len(m.content[pos.Row]))
	}
	return pos
}

func (m Model) moveToLine(line int) Position {
	pos := Position{Row: min(max(0, line), len(m.content)-1), Col: 0}
	return m.validateCursor(pos)
}

func (m Model) moveToLastLine() Position {
	lastRow := len(m.content) - 1
	if lastRow < 0 {
		lastRow = 0
	}
	pos := Position{Row: lastRow, Col: 0}
	return m.validateCursor(pos)
}

func (m Model) moveWordForward(count int) Position {
	pos := m.cursor
	for i := 0; i < count; i++ {
		pos = m.nextWord(pos, false)
	}
	return pos
}

func (m Model) moveWORDForward(count int) Position {
	pos := m.cursor
	for i := 0; i < count; i++ {
		pos = m.nextWord(pos, true)
	}
	return pos
}

func (m Model) moveWordBackward(count int) Position {
	pos := m.cursor
	for i := 0; i < count; i++ {
		pos = m.prevWord(pos, false)
	}
	return pos
}

func (m Model) moveWORDBackward(count int) Position {
	pos := m.cursor
	for i := 0; i < count; i++ {
		pos = m.prevWord(pos, true)
	}
	return pos
}

func (m Model) moveWordEnd(count int) Position {
	pos := m.cursor
	for i := 0; i < count; i++ {
		pos = m.nextWordEnd(pos, false)
	}
	return pos
}

func (m Model) moveWORDEnd(count int) Position {
	pos := m.cursor
	for i := 0; i < count; i++ {
		pos = m.nextWordEnd(pos, true)
	}
	return pos
}

func (m Model) nextWord(pos Position, isWORD bool) Position {
	if pos.Row >= len(m.content) {
		return pos
	}

	line := m.content[pos.Row]
	col := pos.Col

	// Skip current word
	for col < len(line) && !m.isWordBoundary(line[col], isWORD) {
		col++
	}

	// Skip whitespace
	for col < len(line) && unicode.IsSpace(rune(line[col])) {
		col++
	}

	if col >= len(line) && pos.Row < len(m.content)-1 {
		return Position{Row: pos.Row + 1, Col: 0}
	}

	return Position{Row: pos.Row, Col: col}
}

func (m Model) prevWord(pos Position, isWORD bool) Position {
	if pos.Row < 0 {
		return pos
	}

	line := m.content[pos.Row]
	col := pos.Col

	if col > 0 {
		col--
	}

	// Skip whitespace
	for col > 0 && unicode.IsSpace(rune(line[col])) {
		col--
	}

	// Skip to beginning of word
	for col > 0 && !m.isWordBoundary(line[col-1], isWORD) {
		col--
	}

	return Position{Row: pos.Row, Col: col}
}

func (m Model) nextWordEnd(pos Position, isWORD bool) Position {
	if pos.Row >= len(m.content) {
		return pos
	}

	line := m.content[pos.Row]
	col := pos.Col

	// If we're at the end of the line, go to next line
	if col >= len(line) {
		if pos.Row < len(m.content)-1 {
			nextLine := m.content[pos.Row+1]
			// Find first non-whitespace character
			for i := 0; i < len(nextLine); i++ {
				if !unicode.IsSpace(rune(nextLine[i])) {
					// Find the end of this word
					for j := i; j < len(nextLine) && !m.isWordBoundary(nextLine[j], isWORD); j++ {
						i = j
					}
					return Position{Row: pos.Row + 1, Col: i}
				}
			}
		}
		return pos
	}

	// Check if we're currently at the end of a word
	atWordEnd := col < len(line) && !m.isWordBoundary(line[col], isWORD) &&
		(col+1 >= len(line) || m.isWordBoundary(line[col+1], isWORD))

	// If we're at the end of a word, move forward to find the next word
	if atWordEnd {
		col++ // Move past current character
	}

	// Skip any whitespace or punctuation
	for col < len(line) && (unicode.IsSpace(rune(line[col])) || m.isWordBoundary(line[col], isWORD)) {
		col++
	}

	// If we reached end of line, try next line
	if col >= len(line) {
		if pos.Row < len(m.content)-1 {
			return m.nextWordEnd(Position{Row: pos.Row + 1, Col: 0}, isWORD)
		}
		return pos
	}

	// Now we should be at the beginning of a word - find its end
	if col < len(line) && !m.isWordBoundary(line[col], isWORD) {
		// Find the end of this word
		for col < len(line) && !m.isWordBoundary(line[col], isWORD) {
			col++
		}
		// Move back one position to be ON the last character, not after it
		if col > 0 {
			col--
		}
		return Position{Row: pos.Row, Col: col}
	}

	return pos
}

func (m Model) isWordBoundary(char byte, isWORD bool) bool {
	r := rune(char)
	if isWORD {
		return unicode.IsSpace(r)
	}
	return unicode.IsSpace(r) || unicode.IsPunct(r)
}

func (m Model) repeatFind(dir int) Position {
	// TODO: Implement find repeat
	return m.cursor
}

// Text editing methods
func (m Model) insertText(text string) Model {
	if m.cursor.Row >= len(m.content) {
		return m
	}

	line := m.content[m.cursor.Row]
	before := line[:m.cursor.Col]
	after := line[m.cursor.Col:]

	m.content[m.cursor.Row] = before + text + after
	m.cursor.Col += len(text)
	return m
}

func (m Model) insertNewLine() Model {
	if m.cursor.Row >= len(m.content) {
		return m
	}

	line := m.content[m.cursor.Row]
	before := line[:m.cursor.Col]
	after := line[m.cursor.Col:]

	m.content[m.cursor.Row] = before

	newContent := make([]string, len(m.content)+1)
	copy(newContent[:m.cursor.Row+1], m.content[:m.cursor.Row+1])
	newContent[m.cursor.Row+1] = after
	copy(newContent[m.cursor.Row+2:], m.content[m.cursor.Row+1:])

	m.content = newContent
	m.cursor.Row++
	m.cursor.Col = 0
	m.adjustScroll()
	return m
}

func (m Model) insertNewLineBelow() Model {
	line := ""
	if m.cursor.Row < len(m.content) {
		line = ""
	}

	newContent := make([]string, len(m.content)+1)
	copy(newContent[:m.cursor.Row+1], m.content[:m.cursor.Row+1])
	newContent[m.cursor.Row+1] = line
	copy(newContent[m.cursor.Row+2:], m.content[m.cursor.Row+1:])

	m.content = newContent
	m.cursor.Row++
	m.cursor.Col = 0
	m.adjustScroll()
	return m
}

func (m Model) insertNewLineAbove() Model {
	newContent := make([]string, len(m.content)+1)
	copy(newContent[:m.cursor.Row], m.content[:m.cursor.Row])
	newContent[m.cursor.Row] = ""
	copy(newContent[m.cursor.Row+1:], m.content[m.cursor.Row:])

	m.content = newContent
	m.cursor.Col = 0
	m.adjustScroll()
	return m
}

// adjustScroll ensures the cursor is visible in the viewport
func (m Model) adjustScroll() Model {
	// Don't scroll if we have fewer lines than the viewport height
	if len(m.content) <= m.height {
		m.scrollOffset = 0
		return m
	}

	// If cursor is above viewport, scroll up
	if m.cursor.Row < m.scrollOffset {
		m.scrollOffset = m.cursor.Row
	}

	// If cursor is below viewport, scroll down
	if m.cursor.Row >= m.scrollOffset+m.height {
		m.scrollOffset = m.cursor.Row - m.height + 1
	}

	// Ensure scrollOffset doesn't go negative
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}

	// Don't scroll beyond the content
	maxOffset := len(m.content) - m.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	return m
}

func (m Model) backspace() Model {
	if m.cursor.Col > 0 {
		line := m.content[m.cursor.Row]
		before := line[:m.cursor.Col-1]
		after := line[m.cursor.Col:]
		m.content[m.cursor.Row] = before + after
		m.cursor.Col--
	} else if m.cursor.Row > 0 {
		// Join with previous line
		prevLine := m.content[m.cursor.Row-1]
		currentLine := m.content[m.cursor.Row]

		m.content[m.cursor.Row-1] = prevLine + currentLine

		// Remove current line
		newContent := make([]string, len(m.content)-1)
		copy(newContent[:m.cursor.Row], m.content[:m.cursor.Row])
		copy(newContent[m.cursor.Row:], m.content[m.cursor.Row+1:])
		m.content = newContent

		m.cursor.Row--
		m.cursor.Col = len(prevLine)
	}
	m = m.adjustScroll()
	return m
}

func (m Model) deleteChar(count int) Model {
	for i := 0; i < count; i++ {
		if m.cursor.Row < len(m.content) {
			line := m.content[m.cursor.Row]
			if m.cursor.Col < len(line) {
				before := line[:m.cursor.Col]
				after := line[m.cursor.Col+1:]
				m.content[m.cursor.Row] = before + after
			}
		}
	}

	if count > 0 {
		m = m.saveUndoState()
	}
	return m
}

func (m Model) replaceCharacter(replacement string, count int) Model {
	if m.cursor.Row >= len(m.content) {
		return m
	}

	line := m.content[m.cursor.Row]
	for i := 0; i < count && m.cursor.Col+i < len(line); i++ {
		if m.cursor.Col+i < len(line) {
			before := line[:m.cursor.Col+i]
			after := line[m.cursor.Col+i+1:]
			line = before + replacement + after
		}
	}

	m.content[m.cursor.Row] = line
	m = m.saveUndoState()
	return m
}

func (m Model) deleteLines(count int) Model {
	startRow := m.cursor.Row
	endRow := min(startRow+count-1, len(m.content)-1)

	// Yank the lines to clipboard
	var deletedLines []string
	for i := startRow; i <= endRow; i++ {
		if i < len(m.content) {
			deletedLines = append(deletedLines, m.content[i])
		}
	}
	m.clipboard = strings.Join(deletedLines, "\n")

	// Handle edge case: deleting all lines
	if len(m.content) <= count {
		m.content = []string{""}
		m.cursor = Position{0, 0}
		m = m.saveUndoState()
		return m
	}

	// Remove the lines
	newContent := make([]string, len(m.content)-len(deletedLines))
	copy(newContent[:startRow], m.content[:startRow])
	copy(newContent[startRow:], m.content[endRow+1:])
	m.content = newContent

	// Adjust cursor position
	if m.cursor.Row >= len(m.content) {
		m.cursor.Row = len(m.content) - 1
	}
	m.cursor.Col = 0
	m = m.adjustScroll()

	m = m.saveUndoState()
	return m
}

func (m Model) yankLines(count int) Model {
	startRow := m.cursor.Row
	endRow := min(startRow+count-1, len(m.content)-1)

	var yankedLines []string
	for i := startRow; i <= endRow; i++ {
		if i < len(m.content) {
			yankedLines = append(yankedLines, m.content[i])
		}
	}
	m.clipboard = strings.Join(yankedLines, "\n")
	return m
}

func (m Model) yankLine() Model {
	if m.cursor.Row < len(m.content) {
		m.clipboard = m.content[m.cursor.Row]
	}
	return m
}

func (m Model) deleteToEndOfLine() Model {
	if m.cursor.Row < len(m.content) {
		line := m.content[m.cursor.Row]
		if m.cursor.Col < len(line) {
			// Yank the deleted text
			m.clipboard = line[m.cursor.Col:]
			// Delete from cursor to end of line
			m.content[m.cursor.Row] = line[:m.cursor.Col]
		}
	}

	m = m.saveUndoState()
	return m
}

func (m Model) changeToEndOfLine() Model {
	m = m.deleteToEndOfLine()
	m = m.startInsertSession()
	m.mode = Insert
	return m
}

func (m Model) changeLines(count int) Model {
	m = m.deleteLines(count)
	m = m.startInsertSession()
	m.mode = Insert
	return m
}

func (m Model) deleteRange(startPos, endPos Position) Model {
	logger.Debug("deleteRange called")
	if startPos.Row == endPos.Row {
		// Single line deletion
		line := m.content[startPos.Row]
		before := line[:startPos.Col]
		after := line[endPos.Col:]
		m.clipboard = line[startPos.Col:endPos.Col]
		m.content[startPos.Row] = before + after
		m.cursor = startPos
	} else {
		// Multi-line deletion
		var deletedText []string

		// First line (partial)
		firstLine := m.content[startPos.Row]
		deletedText = append(deletedText, firstLine[startPos.Col:])

		// Middle lines (complete)
		for row := startPos.Row + 1; row < endPos.Row; row++ {
			deletedText = append(deletedText, m.content[row])
		}

		// Last line (partial)
		if endPos.Row < len(m.content) {
			lastLine := m.content[endPos.Row]
			deletedText = append(deletedText, lastLine[:endPos.Col])
		}

		m.clipboard = strings.Join(deletedText, "\n")

		// Merge remaining parts
		before := m.content[startPos.Row][:startPos.Col]
		after := ""
		if endPos.Row < len(m.content) {
			after = m.content[endPos.Row][endPos.Col:]
		}

		// Create new content
		newContent := make([]string, len(m.content)-(endPos.Row-startPos.Row))
		copy(newContent[:startPos.Row], m.content[:startPos.Row])
		newContent[startPos.Row] = before + after
		copy(newContent[startPos.Row+1:], m.content[endPos.Row+1:])

		m.content = newContent
		m.cursor = startPos
	}

	m = m.saveUndoState()
	logger.Debug("undoHistory size: %d", len(m.undoHistory.states))
	return m
}

func (m Model) yankRange(startPos, endPos Position) Model {
	if startPos.Row == endPos.Row {
		// Single line yank
		line := m.content[startPos.Row]
		m.clipboard = line[startPos.Col:endPos.Col]
	} else {
		// Multi-line yank
		var yankedText []string

		// First line (partial)
		firstLine := m.content[startPos.Row]
		yankedText = append(yankedText, firstLine[startPos.Col:])

		// Middle lines (complete)
		for row := startPos.Row + 1; row < endPos.Row; row++ {
			yankedText = append(yankedText, m.content[row])
		}

		// Last line (partial)
		if endPos.Row < len(m.content) {
			lastLine := m.content[endPos.Row]
			yankedText = append(yankedText, lastLine[:endPos.Col])
		}

		m.clipboard = strings.Join(yankedText, "\n")
	}
	return m
}

func (m Model) changeRange(startPos, endPos Position) Model {
	m = m.deleteRange(startPos, endPos)
	m = m.startInsertSession()
	m.mode = Insert
	return m
}

func (m Model) deleteSelection() Model {
	if m.selection == nil {
		return m
	}

	start, end := m.normalizeSelection(*m.selection)

	if start.Row == end.Row {
		// Single line selection
		line := m.content[start.Row]
		before := line[:start.Col]
		after := line[end.Col:]
		m.content[start.Row] = before + after
		m.cursor = start
	} else {
		// Multi-line selection
		var deletedText []string
		for row := start.Row; row <= end.Row; row++ {
			if row == start.Row {
				deletedText = append(deletedText, m.content[row][start.Col:])
			} else if row == end.Row {
				deletedText = append(deletedText, m.content[row][:end.Col])
			} else {
				deletedText = append(deletedText, m.content[row])
			}
		}
		m.clipboard = strings.Join(deletedText, "\n")

		// Delete lines
		before := m.content[start.Row][:start.Col]
		after := m.content[end.Row][end.Col:]

		newContent := make([]string, len(m.content)-(end.Row-start.Row))
		copy(newContent[:start.Row], m.content[:start.Row])
		newContent[start.Row] = before + after
		copy(newContent[start.Row+1:], m.content[end.Row+1:])

		m.content = newContent
		m.cursor = start
	}

	m = m.saveUndoState()
	return m
}

func (m Model) yankSelection() Model {
	if m.selection == nil {
		return m
	}

	start, end := m.normalizeSelection(*m.selection)

	if start.Row == end.Row {
		line := m.content[start.Row]
		m.clipboard = line[start.Col:end.Col]
	} else {
		var yankedText []string
		for row := start.Row; row <= end.Row; row++ {
			if row == start.Row {
				yankedText = append(yankedText, m.content[row][start.Col:])
			} else if row == end.Row {
				yankedText = append(yankedText, m.content[row][:end.Col])
			} else {
				yankedText = append(yankedText, m.content[row])
			}
		}
		m.clipboard = strings.Join(yankedText, "\n")
	}
	return m
}

func (m Model) pasteAfter() Model {
	if m.clipboard == "" {
		return m
	}

	if strings.Contains(m.clipboard, "\n") {
		// Paste as new line(s)
		lines := strings.Split(m.clipboard, "\n")

		newContent := make([]string, len(m.content)+len(lines))
		copy(newContent[:m.cursor.Row+1], m.content[:m.cursor.Row+1])

		for i, line := range lines {
			newContent[m.cursor.Row+1+i] = line
		}

		copy(newContent[m.cursor.Row+1+len(lines):], m.content[m.cursor.Row+1:])
		m.content = newContent
		m.cursor.Row++
		m.cursor.Col = 0
	} else {
		// Paste as text
		m.cursor = m.moveRight(1)
		m = m.insertText(m.clipboard)
	}

	m = m.saveUndoState()
	return m
}

func (m Model) pasteBefore() Model {
	if m.clipboard == "" {
		return m
	}

	if strings.Contains(m.clipboard, "\n") {
		// Paste as new line(s) before current
		lines := strings.Split(m.clipboard, "\n")

		newContent := make([]string, len(m.content)+len(lines))
		copy(newContent[:m.cursor.Row], m.content[:m.cursor.Row])

		for i, line := range lines {
			newContent[m.cursor.Row+i] = line
		}

		copy(newContent[m.cursor.Row+len(lines):], m.content[m.cursor.Row:])
		m.content = newContent
		m.cursor.Col = 0
	} else {
		// Paste as text
		m = m.insertText(m.clipboard)
	}

	m = m.saveUndoState()
	return m
}

func (m Model) normalizeSelection(sel Selection) (Position, Position) {
	start, end := sel.Start, sel.End

	if start.Row > end.Row || (start.Row == end.Row && start.Col > end.Col) {
		start, end = end, start
	}

	return start, end
}

func (m Model) validateCursor(pos Position) Position {
	if len(m.content) == 0 {
		return Position{0, 0}
	}

	pos.Row = min(max(0, pos.Row), len(m.content)-1)

	if pos.Row < len(m.content) {
		maxCol := len(m.content[pos.Row])
		if m.mode == Normal && maxCol > 0 {
			maxCol-- // Normal mode cursor can't go past last character
		}
		pos.Col = min(max(0, pos.Col), maxCol)
	}

	return pos
}

func (m Model) View() string {
	if len(m.content) == 0 {
		return ""
	}

	var lines []string

	// Check if we should show placeholder (empty content or first line is empty)
	showPlaceholder := len(m.content) == 1 && m.content[0] == "" && m.placeholder != ""

	// Calculate the visible range based on scroll offset
	visibleStart := m.scrollOffset

	for i := 0; i < m.height; i++ {
		actualRow := visibleStart + i
		var styledLine string

		if actualRow < len(m.content) {
			line := m.content[actualRow]

			// Show placeholder on first line if content is empty
			if actualRow == 0 && showPlaceholder {
				styledLine = m.renderPlaceholderWithCursor()
			} else {
				if m.mode == Visual && m.selection != nil {
					styledLine = m.renderLineWithSelection(actualRow, line)
				} else {
					styledLine = m.renderLine(actualRow, line)
				}
			}
		} else {
			styledLine = ""
		}

		// Add prefix: ">" for first line, "  " padding for subsequent lines
		if actualRow == 0 {
			styledLine = "> " + styledLine
		} else {
			styledLine = "  " + styledLine
		}

		lines = append(lines, styledLine)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderLine(row int, line string) string {
	if row != m.cursor.Row {
		return line
	}

	// Render cursor
	if m.cursor.Col >= len(line) {
		// Cursor at end of line
		cursorStyle := m.getCursorStyle()
		return line + cursorStyle.Render(" ")
	}

	before := line[:m.cursor.Col]
	char := string(line[m.cursor.Col])
	after := line[m.cursor.Col+1:]

	cursorStyle := m.getCursorStyle()
	return before + cursorStyle.Render(char) + after
}

func (m Model) renderLineWithSelection(row int, line string) string {
	if m.selection == nil {
		return m.renderLine(row, line)
	}

	start, end := m.normalizeSelection(*m.selection)

	if row < start.Row || row > end.Row {
		return m.renderLine(row, line)
	}

	selectionStyle := lipgloss.NewStyle().Background(lipgloss.Color("240"))

	var startCol, endCol int

	if row == start.Row {
		startCol = start.Col
	} else {
		startCol = 0
	}

	if row == end.Row {
		endCol = end.Col
	} else {
		endCol = len(line)
	}

	before := line[:startCol]
	selected := line[startCol:endCol]
	after := line[endCol:]

	result := before + selectionStyle.Render(selected) + after

	// Add cursor if on this line
	if row == m.cursor.Row {
		// TODO: Render cursor within selection
	}

	return result
}

func (m Model) getCursorStyle() lipgloss.Style {
	// Use the same cursor color (gray) for all modes
	return lipgloss.NewStyle().Background(lipgloss.Color("7")).Foreground(lipgloss.Color("0"))
}

func (m Model) renderPlaceholderWithCursor() string {
	if m.placeholder == "" {
		return ""
	}

	placeholderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Dim gray
	cursorStyle := m.getCursorStyle()

	// Since placeholder is only shown when content is empty, cursor is always at 0,0
	// Show cursor on first character
	if len(m.placeholder) > 0 {
		firstChar := string(m.placeholder[0])
		rest := m.placeholder[1:]

		return cursorStyle.Render(firstChar) + placeholderStyle.Render(rest)
	}

	return placeholderStyle.Render(m.placeholder)
}

// Public API methods to match textarea interface
func (m Model) Value() string {
	return strings.Join(m.content, "\n")
}

func (m *Model) SetValue(value string) {
	m.content = strings.Split(value, "\n")
	m.cursor = Position{0, 0}
}

func (m *Model) SetWidth(width int) {
	m.width = width
}

func (m *Model) SetHeight(height int) {
	m.height = height
	m.adjustScroll()
}

func (m Model) Height() int {
	return m.height
}

func (m *Model) SetPlaceholder(placeholder string) {
	m.placeholder = placeholder
}

func (m *Model) Focus() {
	m.focused = true
}

func (m *Model) Blur() {
	m.focused = false
}

func (m Model) Focused() bool {
	return m.focused
}

func (m Model) Mode() Mode {
	return m.mode
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Insert session methods
func (m Model) startInsertSession() Model {
	if !m.inInsertSession {
		// Don't save state here - we save when exiting insert mode
		m.inInsertSession = true
	}
	return m
}

func (m Model) endInsertSession() Model {
	if m.inInsertSession {
		m = m.saveUndoState() // Save the final state after insert session
		m.inInsertSession = false
	}
	return m
}

// Undo system methods
func (m Model) saveUndoState() Model {
	logger.Debug("=== SAVE UNDO STATE ===")
	logger.Debug("Before save - Index: %d, Total states: %d", m.undoHistory.index, len(m.undoHistory.states))
	logger.Debug("Current content: %v, Cursor: %v", m.content, m.cursor)

	// Check if there's anything to compare against
	if m.undoHistory.index >= 0 && m.undoHistory.index < len(m.undoHistory.states) {
		lastState := m.undoHistory.states[m.undoHistory.index]

		if m.contentEquals(lastState.content) {
			logger.Debug("Duplicate state detected, not saving")
			return m // No changes, don't save duplicate state
		}
	}

	// Deep copy content
	contentCopy := make([]string, len(m.content))
	copy(contentCopy, m.content)

	state := UndoState{
		content: contentCopy,
		cursor:  m.cursor,
	}

	// Remove any states after current index (when we're not at the end)
	if m.undoHistory.index < len(m.undoHistory.states)-1 {
		logger.Debug("Truncating redo history - before: %d states, after: %d states", len(m.undoHistory.states), m.undoHistory.index+1)
		m.undoHistory.states = m.undoHistory.states[:m.undoHistory.index+1]
	}

	// Add new state
	m.undoHistory.states = append(m.undoHistory.states, state)
	m.undoHistory.index++

	// Limit history size
	if len(m.undoHistory.states) > m.undoHistory.maxSize {
		m.undoHistory.states = m.undoHistory.states[1:]
		m.undoHistory.index--
	}

	logger.Debug("After save - Index: %d, Total states: %d", m.undoHistory.index, len(m.undoHistory.states))
	logger.Debug("=== SAVE COMPLETE ===")
	return m
}

// contentEquals compares two content slices for equality
func (m *Model) contentEquals(other []string) bool {
	if len(m.content) != len(other) {
		return false
	}

	for i := range m.content {
		if m.content[i] != other[i] {
			return false
		}
	}

	return true
}

func (m Model) undo() Model {
	logger.Debug("=== UNDO CALLED ===")
	logger.Debug("Current index: %d, Total states: %d", m.undoHistory.index, len(m.undoHistory.states))

	// Log all states
	for i, state := range m.undoHistory.states {
		marker := "  "
		if i == m.undoHistory.index {
			marker = "->"
		}
		logger.Debug("%s [%d] Content: %v, Cursor: %v", marker, i, state.content, state.cursor)
	}

	if m.undoHistory.index <= 0 {
		logger.Debug("Nothing to undo")
		return m // Nothing to undo
	}

	// Log current state before undo
	logger.Debug("Before undo - Content: %v, Cursor: %v", m.content, m.cursor)

	m.undoHistory.index--
	state := m.undoHistory.states[m.undoHistory.index]

	// Restore state
	m.content = make([]string, len(state.content))
	copy(m.content, state.content)
	m.cursor = state.cursor
	m = m.adjustScroll()

	// Log restored state
	logger.Debug("After undo - Now at index: %d, Content: %v, Cursor: %v", m.undoHistory.index, m.content, m.cursor)
	logger.Debug("=== UNDO COMPLETE ===")
	return m
}

func (m Model) redo() Model {
	if m.undoHistory.index >= len(m.undoHistory.states)-1 {
		return m // Nothing to redo
	}

	m.undoHistory.index++
	state := m.undoHistory.states[m.undoHistory.index]

	// Restore state
	m.content = make([]string, len(state.content))
	copy(m.content, state.content)
	m.cursor = state.cursor
	m = m.adjustScroll()
	return m
}
