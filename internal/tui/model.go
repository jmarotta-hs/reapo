package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"reapo/internal/agent"
	"reapo/internal/tools"
	"reapo/internal/tui/completion"
	"reapo/internal/tui/components"
	"reapo/internal/tui/components/vimtextarea"
)

// Model represents the Bubble Tea model for the TUI
type Model struct {
	messages []components.Message
	textarea vimtextarea.Model
	viewport struct {
		width  int
		height int
	}
	agent             *agent.Agent
	client            anthropic.Client
	toolDefs          []tools.ToolDefinition
	ready             bool
	processing        bool
	processingText    string // Text to show during processing
	processingSpinner *components.SpinnerComponent
	tokenCount        int
	spinners          map[string]*components.SpinnerComponent // Track spinners by message ID
	helpModal         *components.HelpModal                   // Help modal
}

// AgentResponseMsg represents a message from the agent
type AgentResponseMsg struct {
	Content string
	IsError bool
}

// AddMessageMsg represents adding a new message to the chat
type AddMessageMsg struct {
	Message components.Message
}

// MessageUpdateMsg represents updating an existing message
type MessageUpdateMsg struct {
	MessageID string
	Content   string
	Status    components.MessageStatus
	Progress  *components.Progress
	ToolInfo  *components.ToolInfo
}

// ToolInvocationMsg represents a tool being invoked
type ToolInvocationMsg struct {
	ToolName  string
	ToolID    string
	Input     string
	MessageID string
}

// ToolResultMsg represents a tool execution result
type ToolResultMsg struct {
	ToolName  string
	ToolID    string
	Output    string
	Error     string
	Duration  string
	MessageID string
}

// AnimationTickMsg represents a tick for spinner animations
type AnimationTickMsg struct{}

// ProcessMessageSequenceMsg represents the start of message processing sequence
type ProcessMessageSequenceMsg struct {
	UserMessage    string
	UserMessageID  string
	AgentMessageID string
}

// AgentStatusMsg represents agent thinking/status updates
type AgentStatusMsg struct {
	Message   string
	Timestamp time.Time
}

// ProcessToolsMsg triggers processing of tool uses
type ProcessToolsMsg struct {
	Conversation   []anthropic.MessageParam
	Response       *anthropic.Message
	AgentMessageID string
}

// SlashCommandMsg represents a slash command to be executed
type SlashCommandMsg struct {
	Command string
}

// ShowHelpModalMsg triggers showing the help modal
type ShowHelpModalMsg struct{}

// ClearConversationMsg triggers clearing the conversation
type ClearConversationMsg struct{}

// OpenExternalEditorMsg triggers opening the external editor
type OpenExternalEditorMsg struct{}

// CompactConversationMsg triggers conversation compaction with the summary result
type CompactConversationMsg struct {
	Summary string
	Error   error
}

// systemPromptContent will be set by the runner
var systemPromptContent string

// NewModel creates a new TUI model
func NewModel(client anthropic.Client, toolDefs []tools.ToolDefinition) Model {
	// Initialize vim textarea
	ta := vimtextarea.New()
	ta.SetPlaceholder("Type a message... (Enter in Normal, Ctrl+S in Insert/Visual)")
	ta.Focus()
	ta.SetHeight(1)

	// Get working directory for completion engine
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = "." // fallback to current directory
	}

	// Initialize completion engine
	completionEngine := completion.NewCompletionEngine(workingDir)
	ta.SetCompletionEngine(completionEngine)

	chatAgent := agent.NewAgent(&client, nil, toolDefs, systemPromptContent)

	model := Model{
		messages:  []components.Message{},
		textarea:  ta,
		agent:     chatAgent,
		client:    client,
		toolDefs:  toolDefs,
		spinners:  make(map[string]*components.SpinnerComponent),
		helpModal: components.NewHelpModal(),
	}

	return model
}

// Init initializes the TUI model
func (m Model) Init() tea.Cmd {
	return m.textarea.Init()
}

// generateMessageID creates a unique UUIDv7-based message ID
func generateMessageID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// Fallback to a timestamp-based ID if UUID generation fails
		return fmt.Sprintf("msg_fallback_%d", time.Now().UnixNano())
	}
	return id.String()
}
