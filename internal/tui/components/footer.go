package components

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"reapo/internal/tui/components/vimtextarea"
)

// FooterComponent handles the rendering of the status bar footer
type FooterComponent struct {
	mode  vimtextarea.Mode
	width int
}

// NewFooterComponent creates a new footer component
func NewFooterComponent(mode vimtextarea.Mode, width int) *FooterComponent {
	return &FooterComponent{
		mode:  mode,
		width: width,
	}
}

// Render renders the complete footer with mode indicator and status bar
func (f *FooterComponent) Render() string {
	// Create mode indicator
	modeIndicator := NewModeIndicatorComponent(f.mode)
	modeIndicatorRendered := modeIndicator.Render()
	modeIndicatorWidth := modeIndicator.Width()

	// Calculate remaining width for main footer content
	remainingWidth := f.width - modeIndicatorWidth

	// Get current directory info
	pwd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" && strings.HasPrefix(pwd, homeDir) {
		pwd = "~" + pwd[len(homeDir):]
	}

	leftText := "reapo"
	rightText := "claude-sonnet-4"
	centerText := pwd

	// Calculate spacing for justify-between layout in remaining space
	totalContentWidth := len(leftText) + len(centerText) + len(rightText)
	// Account for padding (2 spaces) in the remaining width
	spacingWidth := remainingWidth - totalContentWidth - 2

	// Ensure we don't have negative spacing
	if spacingWidth < 0 {
		spacingWidth = 0
	}

	// Distribute spacing: left-center and center-right
	leftSpacing := spacingWidth / 2
	rightSpacing := spacingWidth - leftSpacing

	mainFooterText := leftText + strings.Repeat(" ", leftSpacing) + centerText + strings.Repeat(" ", rightSpacing) + rightText
	mainFooter := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("0")).
		Width(remainingWidth).
		Padding(0, 1).
		Render(mainFooterText)

	// Combine mode indicator and main footer
	return modeIndicatorRendered + mainFooter
}
