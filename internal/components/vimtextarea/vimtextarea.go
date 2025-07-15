package vimtextarea

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

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
	m.commandState = CommandState{}
	m.cursor = m.validateCursor(m.cursor)
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