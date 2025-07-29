package components

import (
	"fmt"
	"strings"

	"reapo/internal/tui/completion"
)

type CompletionComponent struct {
	items    []completion.CompletionItem
	selected int
	height   int
	width    int
}

func NewCompletionComponent(items []completion.CompletionItem, selected int, width int) CompletionComponent {
	maxHeight := 8 // Maximum completion window height
	height := len(items)
	if height > maxHeight {
		height = maxHeight
	}

	return CompletionComponent{
		items:    items,
		selected: selected,
		height:   height,
		width:    width,
	}
}

func (c CompletionComponent) Render() string {
	if len(c.items) == 0 {
		return ""
	}

	var lines []string

	// Calculate visible range for scrolling
	startIdx := 0
	endIdx := len(c.items)

	if len(c.items) > c.height {
		// Scroll to keep selected item visible
		if c.selected >= c.height {
			startIdx = c.selected - c.height + 1
		}
		endIdx = startIdx + c.height
		if endIdx > len(c.items) {
			endIdx = len(c.items)
			startIdx = endIdx - c.height
		}
	}

	// Render visible items
	for i := startIdx; i < endIdx; i++ {
		item := c.items[i]

		// Format the line
		var line string
		if i == c.selected {
			// Highlight selected item
			line = fmt.Sprintf("> %s", item.Text)
		} else {
			line = fmt.Sprintf("  %s", item.Text)
		}

		// Add description if present
		if item.Description != "" {
			// Calculate available space for description
			availableSpace := c.width - len(line) - 2 // 2 for padding
			if availableSpace > 0 && len(item.Description) > 0 {
				description := item.Description
				if len(description) > availableSpace {
					description = description[:availableSpace-3] + "..."
				}
				line = fmt.Sprintf("%-*s %s", c.width-len(description)-2, line, description)
			}
		}

		// Truncate if too long
		if len(line) > c.width-2 {
			line = line[:c.width-5] + "..."
		}

		lines = append(lines, line)
	}

	// Create border
	if len(lines) > 0 {
		border := strings.Repeat("─", c.width-2)
		result := "┌" + border + "┐\n"

		for _, line := range lines {
			// Pad line to full width
			paddedLine := fmt.Sprintf("│%-*s│", c.width-2, line)
			result += paddedLine + "\n"
		}

		result += "└" + border + "┘"
		return result
	}

	return ""
}

func (c CompletionComponent) Height() int {
	if len(c.items) == 0 {
		return 0
	}
	return c.height + 2 // +2 for borders
}
