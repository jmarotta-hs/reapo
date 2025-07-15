package vimtextarea

import (
	"strings"
	"reapo/internal/logger"
)

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