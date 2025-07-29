package components

import (
	"github.com/charmbracelet/lipgloss"
	"reapo/internal/tui/components/vimtextarea"
)

// InputComponent handles the rendering of the input area
type InputComponent struct {
	textarea vimtextarea.Model
	width    int
}

// NewInputComponent creates a new input component
func NewInputComponent(textarea vimtextarea.Model, width int) *InputComponent {
	return &InputComponent{
		textarea: textarea,
		width:    width,
	}
}

// Render renders the input area with border and styling
func (i *InputComponent) Render() string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(i.width-2).
		Padding(0, 1).
		Render(i.textarea.View())
}
