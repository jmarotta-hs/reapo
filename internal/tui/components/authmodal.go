package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AuthModal is a modal dialog for authentication input
type AuthModal struct {
	active      bool
	title       string
	message     string
	url         string
	input       textinput.Model
	width       int
	height      int
	onSubmit    func(string) tea.Cmd
	onCancel    func() tea.Cmd
}

// NewAuthModal creates a new auth modal
func NewAuthModal() AuthModal {
	ti := textinput.New()
	ti.Placeholder = "Paste authorization code here..."
	ti.CharLimit = 256
	ti.Width = 50
	
	return AuthModal{
		input: ti,
	}
}

// AuthModalConfig contains configuration for showing the modal
type AuthModalConfig struct {
	Title    string
	Message  string
	URL      string
	Width    int
	Height   int
	OnSubmit func(string) tea.Cmd
	OnCancel func() tea.Cmd
}

// Show displays the modal with the given configuration
func (m *AuthModal) Show(config AuthModalConfig) tea.Cmd {
	m.active = true
	m.title = config.Title
	m.message = config.Message
	m.url = config.URL
	m.width = config.Width
	m.height = config.Height
	m.onSubmit = config.OnSubmit
	m.onCancel = config.OnCancel
	m.input.Reset()
	m.input.Focus()
	
	// Calculate initial input width (same as in View)
	modalWidth := m.width * 85 / 100
	if modalWidth < 60 {
		modalWidth = min(60, m.width-4)
	}
	m.input.Width = max(10, modalWidth-6)
	
	return textinput.Blink
}

// Hide hides the modal
func (m *AuthModal) Hide() {
	m.active = false
	m.input.Blur()
	m.input.Reset()
}

// Active returns whether the modal is currently shown
func (m AuthModal) Active() bool {
	return m.active
}

// Update handles tea messages
func (m AuthModal) Update(msg tea.Msg) (AuthModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.onSubmit != nil && m.input.Value() != "" {
				code := m.input.Value()
				m.Hide()
				return m, m.onSubmit(code)
			}
		case tea.KeyEsc, tea.KeyCtrlC:
			if m.onCancel != nil {
				m.Hide()
				return m, m.onCancel()
			}
		}
	
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Calculate modal width same as in View
		modalWidth := m.width * 85 / 100
		if modalWidth < 60 {
			modalWidth = min(60, m.width-4)
		}
		// Adjust input width to fit modal (modal padding is 2 on each side, plus some margin)
		m.input.Width = max(10, modalWidth-6)
	}
	
	// Update the text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the modal
func (m AuthModal) View() string {
	if !m.active {
		return ""
	}
	
	// Handle very small terminals
	if m.width < 20 || m.height < 10 {
		return "Terminal too small"
	}
	
	// Calculate modal width - 85% of screen width
	modalWidth := m.width * 85 / 100
	// Ensure minimum width for usability
	if modalWidth < 60 {
		modalWidth = min(60, m.width-4)
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
		Width(modalWidth - 4) // Account for padding
	
	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginBottom(1).
		Width(modalWidth - 4)
	
	urlLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Bold(true)
	
	urlStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("33")).
		MarginBottom(1).
		Width(modalWidth - 4)
	
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1).
		Align(lipgloss.Center).
		Width(modalWidth - 4)
	
	// Build content
	var content strings.Builder
	
	if m.title != "" {
		content.WriteString(titleStyle.Render(m.title))
		content.WriteString("\n")
	}
	
	if m.message != "" {
		content.WriteString(messageStyle.Render(m.message))
		content.WriteString("\n")
	}
	
	if m.url != "" {
		// Wrap URL if needed
		content.WriteString(urlLabelStyle.Render("Visit this URL:"))
		content.WriteString("\n")
		wrappedURL := wrapURL(m.url, max(10, modalWidth-4))
		content.WriteString(urlStyle.Render(wrappedURL))
		content.WriteString("\n")
	}
	
	// Adjust input width to fit modal
	m.input.Width = max(10, modalWidth-6)
	content.WriteString(m.input.View())
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("Enter to submit • Esc to cancel"))
	
	modal := modalStyle.Render(content.String())
	
	// Center the modal vertically and horizontally
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// wrapURL wraps URL text to fit within the specified width
func wrapURL(text string, width int) string {
	// Ensure width is at least 1 to prevent panic
	if width <= 0 {
		return text
	}
	
	if len(text) <= width {
		return text
	}
	
	var result strings.Builder
	for i := 0; i < len(text); i += width {
		end := i + width
		if end > len(text) {
			end = len(text)
		}
		result.WriteString(text[i:end])
		if end < len(text) {
			result.WriteString("\n")
		}
	}
	return result.String()
}