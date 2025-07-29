package components

import (
	"github.com/charmbracelet/lipgloss"
	"reapo/internal/tui/components/vimtextarea"
)

// ModeIndicatorComponent handles the rendering of the vim mode indicator
type ModeIndicatorComponent struct {
	mode vimtextarea.Mode
}

// NewModeIndicatorComponent creates a new mode indicator component
func NewModeIndicatorComponent(mode vimtextarea.Mode) *ModeIndicatorComponent {
	return &ModeIndicatorComponent{
		mode: mode,
	}
}

// Render renders the vim mode indicator with colored background
func (m *ModeIndicatorComponent) Render() string {
	var modeText string
	var modeColor string

	switch m.mode {
	case vimtextarea.Normal:
		modeText = " NORMAL "
		modeColor = "4" // Blue background for normal mode
	case vimtextarea.Insert:
		modeText = " INSERT "
		modeColor = "2" // Green background for insert mode
	case vimtextarea.Visual:
		modeText = " VISUAL "
		modeColor = "5" // Magenta background for visual mode
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")). // Black text
		Background(lipgloss.Color(modeColor)).
		Render(modeText)
}

// Width returns the width of the mode indicator
func (m *ModeIndicatorComponent) Width() int {
	switch m.mode {
	case vimtextarea.Normal:
		return len(" NORMAL ")
	case vimtextarea.Insert:
		return len(" INSERT ")
	case vimtextarea.Visual:
		return len(" VISUAL ")
	default:
		return 0
	}
}
