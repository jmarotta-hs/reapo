package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HelpModal represents a help modal showing available commands
type HelpModal struct {
	visible bool
}

// NewHelpModal creates a new help modal
func NewHelpModal() *HelpModal {
	return &HelpModal{
		visible: false,
	}
}

// Show makes the help modal visible
func (h *HelpModal) Show() {
	h.visible = true
}

// Hide makes the help modal invisible
func (h *HelpModal) Hide() {
	h.visible = false
}

// IsVisible returns whether the modal is visible
func (h *HelpModal) IsVisible() bool {
	return h.visible
}

// View renders the help modal
func (h *HelpModal) View() string {
	if !h.visible {
		return ""
	}

	// Define styles
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Background(lipgloss.Color("235"))

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214")).
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("246"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	// Build content
	var content strings.Builder
	content.WriteString(titleStyle.Render("Reapo Help"))
	content.WriteString("\n\n")

	content.WriteString(keyStyle.Render("Slash Commands:"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("/help") + " - " + descStyle.Render("Show this help menu"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("/clear") + " - " + descStyle.Render("Clear conversation history"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("/editor") + " - " + descStyle.Render("Quick access to $EDITOR"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("/compact") + " - " + descStyle.Render("Summarize and compact conversation"))
	content.WriteString("\n\n")

	content.WriteString(keyStyle.Render("Vim Modes:"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("Normal") + " - " + descStyle.Render("Navigate and enter commands"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("Insert") + " - " + descStyle.Render("Type your message"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("Visual") + " - " + descStyle.Render("Select text"))
	content.WriteString("\n\n")

	content.WriteString(keyStyle.Render("Key Bindings:"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("Enter (Normal)") + " - " + descStyle.Render("Send message"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("Ctrl+S (Insert/Visual)") + " - " + descStyle.Render("Send message"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("Ctrl+C") + " - " + descStyle.Render("Exit application"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("Esc") + " - " + descStyle.Render("Return to Normal mode / Close modal"))
	content.WriteString("\n\n")

	content.WriteString(keyStyle.Render("File References:"))
	content.WriteString("\n")
	content.WriteString(commandStyle.Render("@filename") + " - " + descStyle.Render("Reference a file (with completion)"))
	content.WriteString("\n\n")

	content.WriteString(descStyle.Render("Press Esc to close this help"))

	return modalStyle.Render(content.String())
}
