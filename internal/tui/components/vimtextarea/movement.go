package vimtextarea

import "unicode"

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
