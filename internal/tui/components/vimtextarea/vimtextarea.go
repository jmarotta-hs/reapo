package vimtextarea

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"reapo/internal/tui/completion"
)

// SlashCommandMsg represents a slash command to be executed
type SlashCommandMsg struct {
	Command string
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
	// Handle completion navigation first
	if m.completionState.Active {
		switch key {
		case "esc":
			m.completionState.Reset()
			return m, nil
		case "ctrl+n", "down":
			m.completionState.SelectNext()
			return m, nil
		case "ctrl+p", "up":
			m.completionState.SelectPrev()
			return m, nil
		case "ctrl+y", "enter":
			// Insert selected completion
			if selected := m.completionState.GetSelectedItem(); selected != nil {
				// Check if it's a slash command
				if strings.HasPrefix(selected.Text, "/") {
					// Clear textarea and execute command
					m.content = []string{""}
					m.cursor = Position{0, 0}
					m.completionState.Reset()

					// Return command to execute
					return m, func() tea.Msg {
						return SlashCommandMsg{Command: selected.Text}
					}
				} else {
					// Regular completion - insert as before
					m = m.insertCompletion(selected.Text)
					m.completionState.Reset()
				}
			}
			return m, nil
		case "backspace":
			m = m.backspace()
			// Check if we should trigger or update completion
			m = m.checkAndTriggerCompletion()
			return m, nil
		case "left", "right":
			// Let the text area handle cursor movement, then check completion
			// Fall through to normal handling
		default:
			// For regular character input during completion
			if len(msg.Runes) > 0 {
				m = m.insertText(string(msg.Runes))
				m = m.updateCompletion()
				return m, nil
			}
		}
	}

	// Normal insert mode handling
	switch key {
	case "esc":
		m.completionState.Reset()
		m = m.endInsertSession()
		m.mode = Normal
		m.cursor = m.moveLeft(1)
		m = m.adjustScroll()
	case "enter":
		m = m.insertNewLine()
	case "backspace":
		m = m.backspace()
		// Check if we should trigger completion after backspace
		m = m.checkAndTriggerCompletion()
	case "tab":
		m = m.insertText("\t")
	case "left":
		m.cursor = m.moveLeft(1)
		m = m.adjustScroll()
		m = m.checkAndTriggerCompletion()
	case "right":
		m.cursor = m.moveRight(1)
		m = m.adjustScroll()
		m = m.checkAndTriggerCompletion()
	case "up":
		m.cursor = m.moveUp(1)
		m = m.adjustScroll()
		m.completionState.Reset() // Reset completion when moving lines
	case "down":
		m.cursor = m.moveDown(1)
		m = m.adjustScroll()
		m.completionState.Reset() // Reset completion when moving lines
	default:
		if len(msg.Runes) > 0 {
			char := msg.Runes[0]

			// Check for completion triggers
			if m.shouldTriggerCompletion(char) {
				m = m.insertText(string(msg.Runes))
				m = m.triggerCompletion(char)
			} else {
				m = m.insertText(string(msg.Runes))
			}
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

func (m Model) CompletionState() completion.CompletionState {
	return m.completionState
}

func (m *Model) SetCompletionEngine(engine *completion.CompletionEngine) {
	m.completionEngine = engine
}

func (m Model) CompletionEngine() *completion.CompletionEngine {
	return m.completionEngine
}

func (m Model) shouldTriggerCompletion(char rune) bool {
	if m.cursor.Row >= len(m.content) {
		return false
	}

	line := m.content[m.cursor.Row]

	// For slash commands, must be at start of line
	if char == '/' {
		return m.cursor.Col == 0
	}

	// For @ completions, check if not escaped
	if char == '@' {
		if m.cursor.Col == 0 {
			return true
		}

		if m.cursor.Col > 0 && m.cursor.Col <= len(line) {
			prevChar := line[m.cursor.Col-1]
			// Don't trigger if escaped with \ or inside quotes
			return prevChar != '\\' && prevChar != '"'
		}
	}

	return false
}

func (m Model) extractCompletionQuery(trigger rune) (string, Position) {
	if m.cursor.Row >= len(m.content) {
		return "", m.cursor
	}

	line := m.content[m.cursor.Row]
	startPos := m.cursor

	// Find the start of the completion (trigger character)
	for i := m.cursor.Col; i >= 0; i-- {
		if i < len(line) && rune(line[i]) == trigger {
			startPos = Position{Row: m.cursor.Row, Col: i}
			break
		}
	}

	// Extract query from trigger position to current position
	if startPos.Col < len(line) && m.cursor.Col <= len(line) {
		query := line[startPos.Col:m.cursor.Col]
		return query, startPos
	}

	return "", startPos
}

func (m Model) triggerCompletion(trigger rune) Model {
	query, startPos := m.extractCompletionQuery(trigger)

	m.completionState.Active = true
	m.completionState.Trigger = trigger
	m.completionState.Query = query
	m.completionState.Selected = 0

	// Store start position for completion insertion (using a field we'll add)
	m.completionStartPos = startPos

	if trigger == '/' {
		m.completionState.Type = completion.SlashCommand
		// Remove leading '/' from query for slash commands
		if strings.HasPrefix(query, "/") {
			m.completionState.Query = query[1:]
		}
	} else {
		m.completionState.Type = completion.FileFolder
		// Remove leading '@' from query for file completions
		if strings.HasPrefix(query, "@") {
			m.completionState.Query = query[1:]
		}
	}

	m = m.updateCompletionItems()
	return m
}

func (m Model) updateCompletion() Model {
	if !m.completionState.Active {
		return m
	}

	// Extract current query
	query, startPos := m.extractCompletionQuery(m.completionState.Trigger)
	m.completionStartPos = startPos

	// If query becomes empty (user deleted trigger), deactivate completion
	if !strings.Contains(query, string(m.completionState.Trigger)) {
		m.completionState.Reset()
		return m
	}

	// Update query, removing trigger prefix
	if m.completionState.Trigger == '/' && strings.HasPrefix(query, "/") {
		m.completionState.Query = query[1:]
	} else if m.completionState.Trigger == '@' && strings.HasPrefix(query, "@") {
		m.completionState.Query = query[1:]
	} else {
		m.completionState.Query = query
	}

	m = m.updateCompletionItems()
	return m
}

func (m Model) updateCompletionItems() Model {
	if m.completionEngine == nil {
		return m
	}

	items := m.completionEngine.GetCompletions(m.completionState.Trigger, m.completionState.Query)
	m.completionState.Items = items

	// Reset selection if items changed
	if len(items) > 0 && m.completionState.Selected >= len(items) {
		m.completionState.Selected = 0
	}

	// Deactivate if no items
	if len(items) == 0 {
		m.completionState.Reset()
	}

	return m
}

func (m Model) insertCompletion(text string) Model {
	if !m.completionState.Active {
		return m
	}

	// Replace from start position to current cursor with completion text
	startPos := m.completionStartPos
	endPos := m.cursor

	if startPos.Row < len(m.content) && endPos.Row < len(m.content) {
		line := m.content[startPos.Row]

		// Build new line with completion
		var newLine string
		if startPos.Col > 0 {
			newLine = line[:startPos.Col]
		}

		// Handle different completion types
		if m.completionState.Trigger == '@' {
			// For @ completions, just insert @filename for user editing
			newLine += "@" + text
		} else {
			// For slash commands, just insert the text
			newLine += text
		}

		if endPos.Col < len(line) {
			newLine += line[endPos.Col:]
		}

		m.content[startPos.Row] = newLine

		// Update cursor position
		var insertedLength int
		if m.completionState.Trigger == '@' {
			insertedLength = len(text) + 1 // +1 for @
		} else {
			insertedLength = len(text)
		}
		newCursorCol := startPos.Col + insertedLength
		m.cursor = Position{Row: startPos.Row, Col: newCursorCol}
	}

	return m
}

func (m Model) checkAndTriggerCompletion() Model {
	// If completion is already active, just update it
	if m.completionState.Active {
		return m.updateCompletion()
	}

	// Check if cursor is positioned after a trigger character
	if m.cursor.Row >= len(m.content) || m.cursor.Col == 0 {
		return m
	}

	line := m.content[m.cursor.Row]
	if m.cursor.Col > len(line) {
		return m
	}

	// Look backwards from cursor to find potential trigger
	for i := m.cursor.Col - 1; i >= 0; i-- {
		char := rune(line[i])

		// Check for @ trigger
		if char == '@' {
			// Check if it's not escaped
			if i == 0 || line[i-1] != '\\' {
				// Found valid @ trigger, check if we have a partial query
				query := line[i:m.cursor.Col]
				if len(query) > 0 { // Only trigger if there's at least the @ character
					m = m.triggerCompletion('@')
					return m
				}
			}
			break // Stop searching after finding @
		}

		// Check for / trigger at start of line
		if char == '/' && i == 0 {
			query := line[i:m.cursor.Col]
			if len(query) > 0 { // Only trigger if there's at least the / character
				m = m.triggerCompletion('/')
				return m
			}
			break
		}

		// Stop searching if we hit whitespace or certain punctuation
		if char == ' ' || char == '\t' || char == '\n' || char == ',' || char == ';' || char == '.' || char == '!' || char == '?' {
			break
		}
	}

	return m
}
