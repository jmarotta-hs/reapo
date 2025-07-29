package tui

import (
	"log"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	tea "github.com/charmbracelet/bubbletea"
	"reapo/internal/tools"
)

// RunTUI starts the TUI interface
func RunTUI(client anthropic.Client, toolDefs []tools.ToolDefinition, systemPrompt string) {
	// Set the system prompt for the TUI package
	systemPromptContent = systemPrompt

	// Create the TUI model
	m := NewModel(client, toolDefs)

	// Run the Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}
