package tui

import (
	"reapo/internal/tui/components"
)

// View renders the TUI
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Get completion state
	completionState := m.textarea.CompletionState()

	// Create completion component if active
	var completionComponent components.CompletionComponent
	var completionHeight int
	if completionState.Active {
		completionComponent = components.NewCompletionComponent(
			completionState.Items,
			completionState.Selected,
			m.viewport.width,
		)
		completionHeight = completionComponent.Height()
	}

	// Calculate processing indicator height (if active)
	processingHeight := 0
	if m.processing {
		processingHeight = 2 // 1 line for content + 1 for spacing
	}

	// Calculate heights: total - textarea height - completion height - processing height - border (2 lines) - footer line - spacing
	textareaHeight := m.textarea.Height()
	chatHeight := m.viewport.height - textareaHeight - completionHeight - processingHeight - 4

	// Create and render components
	chatComponent := components.NewChatComponent(m.messages, chatHeight, m.viewport.width)
	chat := chatComponent.RenderWithSpinners(m.spinners)

	// Render processing indicator if active
	var processingIndicator string
	if m.processing && m.processingSpinner != nil {
		processingIndicator = "\n  " + m.processingSpinner.View() + " " + m.processingText + "\n"
	}

	// Render completion above input if active
	var completion string
	if completionState.Active {
		completion = completionComponent.Render() + "\n"
	}

	inputComponent := components.NewInputComponent(m.textarea, m.viewport.width)
	input := inputComponent.Render()

	footerComponent := components.NewFooterComponent(m.textarea.Mode(), m.viewport.width)
	footer := footerComponent.Render()

	// Render help modal if visible (overlay on top)
	if m.helpModal.IsVisible() {
		return m.helpModal.View()
	}
	
	// Render status modal if visible (overlay on top)
	if m.statusModal.IsVisible() {
		return m.statusModal.View()
	}
	
	// Render auth modal if active (overlay on top)
	if m.authModal.Active() {
		return m.authModal.View()
	}

	return chat + processingIndicator + completion + input + "\n\n\n" + footer
}
