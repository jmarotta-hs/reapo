package components

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

// StatuslineMessageType represents the type of statusline message
type StatuslineMessageType int

const (
	StatuslineInfo StatuslineMessageType = iota
	StatuslineWarning
	StatuslineError
)

// StatuslineMessage represents a message to display in the statusline
type StatuslineMessage struct {
	Type     StatuslineMessageType
	Text     string
	Duration time.Duration
	ShowTime time.Time
}

// StatuslineComponent handles the rendering of the statusline
type StatuslineComponent struct {
	message *StatuslineMessage
	width   int
}

// NewStatuslineComponent creates a new statusline component
func NewStatuslineComponent(width int) *StatuslineComponent {
	return &StatuslineComponent{
		width: width,
	}
}

// SetMessage sets the current message to display
func (s *StatuslineComponent) SetMessage(msg *StatuslineMessage) {
	s.message = msg
}

// ClearMessage clears the current message
func (s *StatuslineComponent) ClearMessage() {
	s.message = nil
}

// HasExpired checks if the current message has expired
func (s *StatuslineComponent) HasExpired() bool {
	if s.message == nil || s.message.Duration == 0 {
		return false
	}
	return time.Since(s.message.ShowTime) > s.message.Duration
}

// Render renders the statusline
func (s *StatuslineComponent) Render() string {
	// If no message or expired, return empty line with background
	if s.message == nil {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("0")).
			Width(s.width).
			Render(" ")
	}

	// Choose color based on message type
	var fg lipgloss.Color
	switch s.message.Type {
	case StatuslineWarning:
		fg = lipgloss.Color("226") // Yellow
	case StatuslineError:
		fg = lipgloss.Color("196") // Red
	default:
		fg = lipgloss.Color("252") // Light gray for info
	}

	// Render the message
	return lipgloss.NewStyle().
		Foreground(fg).
		Background(lipgloss.Color("0")).
		Width(s.width).
		Padding(0, 1).
		Render(s.message.Text)
}

// Width returns the width of the statusline
func (s *StatuslineComponent) Width() int {
	return s.width
}

// SetWidth updates the width of the statusline
func (s *StatuslineComponent) SetWidth(width int) {
	s.width = width
}