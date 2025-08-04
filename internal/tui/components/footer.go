package components

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"reapo/internal/tui/components/vimtextarea"
)

// FooterComponent handles the rendering of the status bar footer
type FooterComponent struct {
	mode             vimtextarea.Mode
	width            int
	contextTokens    int
	maxContextTokens int
	modelName        string
}

// NewFooterComponent creates a new footer component
func NewFooterComponent(mode vimtextarea.Mode, width int) *FooterComponent {
	return &FooterComponent{
		mode:             mode,
		width:            width,
		contextTokens:    0,
		maxContextTokens: 200000,
		modelName:        "claude-sonnet-4",
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

	// Format context usage
	percentage := float64(f.contextTokens) / float64(f.maxContextTokens) * 100
	contextText := fmt.Sprintf("%s/%s (%.1f%%)", 
		formatTokenCount(f.contextTokens), 
		formatTokenCount(f.maxContextTokens),
		percentage)
	
	// Choose color based on usage percentage
	var contextColor string
	if percentage < 50 {
		contextColor = "2" // Green
	} else if percentage < 80 {
		contextColor = "3" // Yellow
	} else {
		contextColor = "1" // Red
	}
	
	leftText := "reapo"
	rightText := f.modelName

	// Build the sections with proper spacing
	// Layout: reapo | pwd | context | model
	sections := []string{leftText, pwd, contextText, rightText}
	
	// Calculate spacing between sections
	totalContentWidth := 0
	for _, section := range sections {
		totalContentWidth += len(section)
	}
	
	// Account for separators (3 spaces between each section) and padding
	separatorCount := len(sections) - 1
	totalSeparatorWidth := separatorCount * 3
	availableWidth := remainingWidth - totalContentWidth - totalSeparatorWidth - 2
	
	// Distribute extra space evenly
	extraSpacePerGap := availableWidth / separatorCount
	if extraSpacePerGap < 0 {
		extraSpacePerGap = 0
	}
	
	// Create the colored context text
	contextStyled := lipgloss.NewStyle().
		Foreground(lipgloss.Color(contextColor)).
		Background(lipgloss.Color("236")).
		Render(contextText)
	
	// Build footer parts
	separator := strings.Repeat(" ", 3+extraSpacePerGap)
	
	// Style each part with consistent background
	styledLeft := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("236")).
		Render(leftText)
		
	styledPwd := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("236")).
		Render(pwd)
		
	styledRight := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("236")).
		Render(rightText)
		
	styledSeparator := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Render(separator)
	
	// Compose the footer
	composedFooter := styledLeft + styledSeparator + styledPwd + styledSeparator + contextStyled + styledSeparator + styledRight
	
	// Ensure the footer fills the entire width with padding
	paddingNeeded := remainingWidth - lipgloss.Width(composedFooter) - 2 // -2 for left/right padding
	if paddingNeeded > 0 {
		composedFooter += lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Render(strings.Repeat(" ", paddingNeeded))
	}
	
	mainFooter := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Width(remainingWidth).
		Padding(0, 1).
		Render(composedFooter)

	// Combine mode indicator and main footer
	return modeIndicatorRendered + mainFooter
}

// UpdateContextInfo updates the context token information
func (f *FooterComponent) UpdateContextInfo(contextTokens, maxContextTokens int, modelName string) {
	f.contextTokens = contextTokens
	f.maxContextTokens = maxContextTokens
	f.modelName = modelName
}

// formatTokenCount formats token count with k suffix for thousands
func formatTokenCount(tokens int) string {
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}
