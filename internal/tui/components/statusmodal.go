package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StatusModal is a modal dialog for displaying authentication status
type StatusModal struct {
	visible    bool
	title      string
	authStatus string
	width      int
	height     int
}

// NewStatusModal creates a new status modal
func NewStatusModal() *StatusModal {
	return &StatusModal{
		title: "Authentication Status",
	}
}

// Show displays the modal with the given authentication status
func (m *StatusModal) Show(authStatus string, width, height int) {
	m.visible = true
	m.authStatus = authStatus
	m.width = width
	m.height = height
}

// Hide hides the modal
func (m *StatusModal) Hide() {
	m.visible = false
}

// IsVisible returns whether the modal is currently shown
func (m StatusModal) IsVisible() bool {
	return m.visible
}

// Update handles tea messages
func (m StatusModal) Update(msg tea.Msg) (StatusModal, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyEnter, tea.KeySpace:
			m.Hide()
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// View renders the modal
func (m StatusModal) View() string {
	if !m.visible {
		return ""
	}

	// Handle very small terminals
	if m.width < 20 || m.height < 10 {
		return "Terminal too small"
	}

	// Calculate modal width - 60% of screen width
	modalWidth := m.width * 60 / 100
	if modalWidth < 40 {
		modalWidth = min(40, m.width-4)
	}
	if modalWidth > 80 {
		modalWidth = 80
	}

	// Define styles
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 2).
		Width(modalWidth)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1).
		Align(lipgloss.Center).
		Width(modalWidth - 4)

	contentStyle := lipgloss.NewStyle().
		Width(modalWidth - 4)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1).
		Align(lipgloss.Center).
		Width(modalWidth - 4)

	// Build content
	var content strings.Builder
	
	content.WriteString(titleStyle.Render(m.title))
	content.WriteString("\n\n")
	content.WriteString(contentStyle.Render("Auth: " + m.authStatus))
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("Press Esc, Enter, or Space to close"))

	modal := modalStyle.Render(content.String())

	// Center the modal
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)
}