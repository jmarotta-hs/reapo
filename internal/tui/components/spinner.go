package components

import (
	"fmt"
)

// SpinnerComponent renders animated spinners for processing states
type SpinnerComponent struct {
	frames  []string
	current int
	message string
}

// NewSpinnerComponent creates a new spinner with optional message
func NewSpinnerComponent(message string) *SpinnerComponent {
	return &SpinnerComponent{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		current: 0,
		message: message,
	}
}

// Tick advances the spinner to the next frame
func (s *SpinnerComponent) Tick() {
	s.current = (s.current + 1) % len(s.frames)
}

// SetMessage updates the spinner message
func (s *SpinnerComponent) SetMessage(message string) {
	s.message = message
}

// View returns just the spinner frame without message (for inline use)
func (s *SpinnerComponent) View() string {
	return s.frames[s.current]
}

// Render returns the current spinner frame with message
func (s *SpinnerComponent) Render() string {
	frame := s.frames[s.current]
	if s.message != "" {
		return fmt.Sprintf("%s %s", frame, s.message)
	}
	return frame
}

// RenderInline returns just the spinner frame without message (for inline use)
func (s *SpinnerComponent) RenderInline() string {
	return s.frames[s.current]
}