package main

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"reapo/internal/agent"
	"reapo/internal/components/vimtextarea"
	"reapo/internal/logger"
	"reapo/internal/tools"
)

//go:embed system_prompt.txt
var systemPromptContent string

// Message represents a chat message
type Message struct {
	Role    string // "user" or "assistant"
	Content string
	IsError bool
}

// TUIModel represents the Bubble Tea model for the TUI
type TUIModel struct {
	messages []Message
	textarea vimtextarea.Model
	viewport struct {
		width  int
		height int
	}
	agent      *agent.Agent
	client     anthropic.Client
	toolDefs   []tools.ToolDefinition
	ready      bool
	processing bool
	tokenCount int
}

// Init initializes the TUI model
func (m TUIModel) Init() tea.Cmd {
	return m.textarea.Init()
}

// Update handles messages and updates the model
func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.width = msg.Width
		m.viewport.height = msg.Height
		m.textarea.SetWidth(msg.Width - 6) // Account for border padding + prefix
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		// Handle key events before passing to textarea
		switch {
		case msg.String() == "ctrl+c":
			return m, tea.Quit
		case msg.String() == "enter" && m.textarea.Mode() == vimtextarea.Normal:
			// Enter sends message in Normal mode
			if m.textarea.Value() != "" && !m.processing {
				userMessage := m.textarea.Value()
				m.messages = append(m.messages, Message{
					Role:    "user",
					Content: userMessage,
				})
				m.textarea.SetValue("")
				m.processing = true
				return m, m.processMessage(userMessage)
			}
			return m, nil
		case msg.String() == "ctrl+s" && (m.textarea.Mode() == vimtextarea.Insert || m.textarea.Mode() == vimtextarea.Visual):
			// Ctrl+S sends message in Insert and Visual modes
			if m.textarea.Value() != "" && !m.processing {
				userMessage := m.textarea.Value()
				m.messages = append(m.messages, Message{
					Role:    "user",
					Content: userMessage,
				})
				m.textarea.SetValue("")
				m.processing = true
				return m, m.processMessage(userMessage)
			}
			return m, nil
		}

	case AgentResponseMsg:
		m.messages = append(m.messages, Message{
			Role:    "assistant",
			Content: msg.Content,
			IsError: msg.IsError,
		})
		m.processing = false
		m.tokenCount += len(msg.Content) // Simple token approximation
		return m, nil
	}

	m.textarea, cmd = m.textarea.Update(msg)

	// Dynamically adjust textarea height based on content
	lines := strings.Count(m.textarea.Value(), "\n") + 1

	maxHeight := min(max((m.viewport.height)/2, 1), 12) // Between 1-12 lines
	height := min(max(lines, 1), maxHeight)

	if height != m.textarea.Height() {
		m.textarea.SetHeight(height)
	}

	return m, cmd
}

// View renders the TUI
func (m TUIModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Styles using terminal colors that adapt to user's theme
	userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))      // Blue
	assistantStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // Yellow
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))     // Red

	// Calculate heights: total - textarea height - border (2 lines) - footer line - spacing
	textareaHeight := m.textarea.Height()
	chatHeight := m.viewport.height - textareaHeight - 4
	chatHeight = max(chatHeight, 1)

	// Build chat messages
	var chatLines []string
	for _, msg := range m.messages {
		prefix := ""
		style := assistantStyle
		if msg.Role == "user" {
			prefix = "> "
			style = userStyle
		} else if msg.Role == "assistant" {
			prefix = "• "
		}

		if msg.IsError {
			style = errorStyle
		}

		chatLines = append(chatLines, style.Render(prefix+msg.Content))
	}

	// Limit chat lines to fit viewport
	if len(chatLines) > chatHeight {
		chatLines = chatLines[len(chatLines)-chatHeight:]
	}

	// Pad chat area to fill screen
	chat := strings.Join(chatLines, "\n")
	chatLineCount := len(strings.Split(chat, "\n"))
	if chat == "" {
		chatLineCount = 0
	}

	// Add padding lines to push input and footer to bottom
	paddingLines := chatHeight - chatLineCount
	if paddingLines > 0 {
		chat += strings.Repeat("\n", paddingLines)
	}

	// Input area
	input := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(m.viewport.width-2).
		Padding(0, 1).
		Render(m.textarea.View())

	// Footer
	pwd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" && strings.HasPrefix(pwd, homeDir) {
		pwd = "~" + pwd[len(homeDir):]
	}
	leftText := "reapo"
	rightText := "claude-sonnet-4"
	centerText := pwd

	// Create vim mode indicator with different background
	var modeText string
	var modeColor string
	switch m.textarea.Mode() {
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

	modeIndicator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")). // Black text
		Background(lipgloss.Color(modeColor)).
		Render(modeText)

	// Calculate remaining width for main footer content
	modeIndicatorWidth := len(modeText)
	remainingWidth := m.viewport.width - modeIndicatorWidth

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
	footer := modeIndicator + mainFooter

	return chat + input + "\n\n\n" + footer
}

// AgentResponseMsg represents a message from the agent
type AgentResponseMsg struct {
	Content string
	IsError bool
}

// processMessage sends a message to the agent
func (m TUIModel) processMessage(message string) tea.Cmd {
	return func() tea.Msg {
		// Create task agent for this message
		agent := agent.NewTaskAgent(&m.client, m.toolDefs, systemPromptContent)

		// Run the single message
		response, err := agent.RunSingleMessage(context.TODO(), message)
		if err != nil {
			return AgentResponseMsg{
				Content: fmt.Sprintf("Error: %s", err.Error()),
				IsError: true,
			}
		}

		return AgentResponseMsg{
			Content: response,
			IsError: false,
		}
	}
}

func main() {
	// Initialize logger
	if err := logger.Init(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()
	logger.Debug("Starting reapo...")

	client := anthropic.NewClient()

	// Register all available tools
	toolDefs := []tools.ToolDefinition{
		tools.ReadFileDefinition,
		tools.ListFilesDefinition,
		tools.EditFileDefinition,
		tools.TodoReadDefinition,
		tools.TodoWriteDefinition,
		tools.RunTaskDefinition,
	}

	// Parse command line arguments
	args := os.Args[1:]

	if len(args) > 0 && args[0] == "run" {
		// Non-interactive mode: reapo run
		runNonInteractive(client, toolDefs, args[1:])
	} else {
		// Interactive TUI mode: reapo
		runTUI(client, toolDefs)
	}
}

func runNonInteractive(client anthropic.Client, toolDefs []tools.ToolDefinition, args []string) {
	var input string

	if len(args) > 0 {
		// Input from command line arguments
		input = strings.Join(args, " ")
	} else {
		// Input from stdin
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading stdin: %s\n", err.Error())
			os.Exit(1)
		}
		input = strings.Join(lines, "\n")
	}

	if input == "" {
		log.Println("Error: No input provided")
		os.Exit(1)
	}

	// Create agent for non-interactive mode
	agentInstance := agent.NewTaskAgent(&client, toolDefs, systemPromptContent)

	// Run the non-interactive session
	err := agentInstance.RunNonInteractive(context.TODO(), input)
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}

func runTUI(client anthropic.Client, toolDefs []tools.ToolDefinition) {
	// Initialize vim textarea
	ta := vimtextarea.New()
	ta.SetPlaceholder("Type a message... (Enter in Normal, Ctrl+S in Insert/Visual)")
	ta.Focus()
	ta.SetHeight(1)

	// Create the TUI model
	m := TUIModel{
		messages: []Message{},
		textarea: ta,
		client:   client,
		toolDefs: toolDefs,
	}

	// Run the Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}
