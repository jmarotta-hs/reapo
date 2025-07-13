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
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"reapo/internal/agent"
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
	textarea textarea.Model
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
	return textarea.Blink
}

// Update handles messages and updates the model
func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.width = msg.Width
		m.viewport.height = msg.Height
		m.textarea.SetWidth(msg.Width - 4) // Account for border padding
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		// Handle key events before passing to textarea
		switch {
		case msg.String() == "ctrl+c":
			return m, tea.Quit
		case msg.String() == "ctrl+s":
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
	lines := strings.Count(m.textarea.Value(), "\n") + 2

	maxHeight := min(max((m.viewport.height)/2, 1), 12) // Between 3-8 lines
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
	userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))                // Blue
	assistantStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))           // Yellow
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))               // Red

	// Calculate heights: total - textarea height - border (2 lines) - footer line - spacing
	textareaHeight := m.textarea.Height()
	chatHeight := m.viewport.height - textareaHeight - 4
	if chatHeight < 1 {
		chatHeight = 1
	}

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

	// Calculate spacing for justify-between layout
	totalContentWidth := len(leftText) + len(centerText) + len(rightText)
	availableWidth := m.viewport.width - 2 // Account for padding
	spacingWidth := availableWidth - totalContentWidth

	// Distribute spacing: left-center and center-right
	leftSpacing := spacingWidth / 2
	rightSpacing := spacingWidth - leftSpacing

	footerText := leftText + strings.Repeat(" ", leftSpacing) + centerText + strings.Repeat(" ", rightSpacing) + rightText
	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("0")).
		Width(m.viewport.width).
		Padding(0, 1).
		Render(footerText)

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
	// Initialize textarea
	ta := textarea.New()
	ta.Placeholder = "Type a message... (Ctrl+S to send)"
	ta.ShowLineNumbers = false
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
