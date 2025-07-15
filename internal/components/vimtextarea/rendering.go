package vimtextarea

import (
	"strings"
	"github.com/charmbracelet/lipgloss"
)

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