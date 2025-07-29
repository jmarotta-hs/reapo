package vimtextarea

import "reapo/internal/logger"

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
