package tui

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	tea "github.com/charmbracelet/bubbletea"
	"reapo/internal/agent"
	"reapo/internal/auth"
	"reapo/internal/logger"
	"reapo/internal/tui/components"
	"reapo/internal/tui/components/vimtextarea"
)

//go:embed summary_prompt.txt
var summaryPrompt string

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Handle auth modal updates first
	if m.authModal.Active() {
		m.authModal, cmd = m.authModal.Update(msg)
		if cmd != nil {
			return m, cmd
		}
		// If modal is still active, don't process other messages
		if m.authModal.Active() {
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.width = msg.Width
		m.viewport.height = msg.Height
		m.textarea.SetWidth(msg.Width - 6) // Account for border padding + prefix
		m.ready = true
		// Update auth modal size
		m.authModal, cmd = m.authModal.Update(msg)
		// Update status modal size
		if m.statusModal != nil {
			statusModal, _ := m.statusModal.Update(msg)
			m.statusModal = &statusModal
		}
		return m, cmd

	case tea.KeyMsg:
		// Handle status modal key events first
		if m.statusModal.IsVisible() {
			statusModal, cmd := m.statusModal.Update(msg)
			m.statusModal = &statusModal
			return m, cmd
		}
		
		// Handle key events before passing to textarea
		switch {
		case msg.String() == "ctrl+c":
			return m, tea.Quit
		case msg.String() == "esc" && m.helpModal.IsVisible():
			// Hide help modal on Esc
			m.helpModal.Hide()
			return m, nil
		case msg.String() == "enter" && m.textarea.Mode() == vimtextarea.Normal:
			// Don't send message if completion is active
			if m.textarea.CompletionState().Active {
				// Let textarea handle completion selection
				break
			}
			// Enter sends message in Normal mode
			if m.textarea.Value() != "" && !m.processing {
				userMessage := m.textarea.Value()
				m.textarea.SetValue("")
				
				m.processing = true
				return m, m.processMessage(userMessage)
			}
			return m, nil
		case msg.String() == "ctrl+s" && (m.textarea.Mode() == vimtextarea.Insert || m.textarea.Mode() == vimtextarea.Visual):
			// Ctrl+S sends message in Insert and Visual modes
			if m.textarea.Value() != "" && !m.processing {
				userMessage := m.textarea.Value()
				m.textarea.SetValue("")
				
				m.processing = true
				return m, m.processMessage(userMessage)
			}
			return m, nil
		}

	case AddMessageMsg:
		m.messages = append(m.messages, msg.Message)
		// Create spinner for processing messages
		if msg.Message.Status == components.MessageProcessing {
			m.spinners[msg.Message.ID] = components.NewSpinnerComponent("")
		}
		return m, nil

	case MessageUpdateMsg:
		// Check if this is the final agent response (no existing message to update)
		messageExists := false
		for i, message := range m.messages {
			if message.ID == msg.MessageID {
				messageExists = true
				m.messages[i].Content = msg.Content
				m.messages[i].Status = msg.Status
				m.messages[i].Progress = msg.Progress
				m.messages[i].ToolInfo = msg.ToolInfo
				m.messages[i].UpdatedAt = time.Now()
				break
			}
		}

		// If no message exists, this is the final agent response - add it
		if !messageExists && msg.Content != "" {
			agentMsg := components.Message{
				ID:        msg.MessageID,
				Role:      "assistant",
				Content:   msg.Content,
				Type:      components.MessageTypeText,
				Status:    msg.Status,
				IsError:   msg.Status == components.MessageError,
				Timestamp: time.Now(),
				UpdatedAt: time.Now(),
			}
			m.messages = append(m.messages, agentMsg)
		}

		// Clean up processing state when complete
		if msg.Status == components.MessageCompleted || msg.Status == components.MessageError {
			m.processing = false
			m.processingText = ""
			m.processingSpinner = nil
		}

		return m, nil

	case ToolInvocationMsg:
		// Add tool invocation message
		toolMsg := components.Message{
			ID:        msg.MessageID,
			Role:      "assistant",
			Content:   "",
			Type:      components.MessageTypeToolInvocation,
			Status:    components.MessageProcessing,
			Timestamp: time.Now(),
			UpdatedAt: time.Now(),
			ToolInfo: &components.ToolInfo{
				Name:  msg.ToolName,
				Input: msg.Input,
			},
		}
		m.messages = append(m.messages, toolMsg)
		m.spinners[msg.MessageID] = components.NewSpinnerComponent("")
		return m, nil

	case ToolResultMsg:
		// Add tool result message
		toolMsg := components.Message{
			ID:        msg.MessageID,
			Role:      "assistant",
			Content:   "",
			Type:      components.MessageTypeToolResult,
			Status:    components.MessageCompleted,
			Timestamp: time.Now(),
			UpdatedAt: time.Now(),
			ToolInfo: &components.ToolInfo{
				Name:       msg.ToolName,
				Output:     msg.Output,
				Error:      msg.Error,
				Duration:   msg.Duration,
				ShowOutput: components.ShouldShowToolOutput(msg.ToolName),
			},
		}
		if msg.Error != "" {
			toolMsg.Status = components.MessageError
		}
		m.messages = append(m.messages, toolMsg)
		return m, nil

	case ProcessMessageSequenceMsg:
		// Add user message with original content for TUI display
		userMsg := components.Message{
			ID:        msg.UserMessageID,
			Role:      "user",
			Content:   msg.UserMessage, // Original message for display
			Type:      components.MessageTypeText,
			Status:    components.MessageCompleted,
			Timestamp: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.messages = append(m.messages, userMsg)

		// Set processing state instead of adding a message
		m.processing = true
		m.processingText = "Processing your request..."
		m.processingSpinner = components.NewSpinnerComponent("")

		return m, tea.Batch(
			m.startAnimation(),
			m.processAgentRequestWithID(msg.UserMessage, msg.AgentMessageID),
		)

	case AnimationTickMsg:
		// Update all active spinners
		hasProcessing := false
		for _, spinner := range m.spinners {
			spinner.Tick()
			hasProcessing = true
		}

		// Also update processing spinner if active
		if m.processing && m.processingSpinner != nil {
			m.processingSpinner.Tick()
			hasProcessing = true
		}

		if hasProcessing {
			return m, m.startAnimation() // Continue animation
		}
		return m, nil

	case AgentResponseMsg:
		// Legacy support - convert to new message format
		m.messages = append(m.messages, components.Message{
			ID:        generateMessageID(),
			Role:      "assistant",
			Content:   msg.Content,
			Type:      components.MessageTypeText,
			Status:    components.MessageCompleted,
			IsError:   msg.IsError,
			Timestamp: time.Now(),
			UpdatedAt: time.Now(),
		})
		m.processing = false
		m.processingText = ""
		m.processingSpinner = nil
		m.tokenCount += len(msg.Content) // Simple token approximation
		return m, nil

	case ProcessToolsMsg:
		// Handle tool processing by returning the batch command
		return m, m.processToolUse(msg.Conversation, msg.Response, msg.AgentMessageID)

	case vimtextarea.SlashCommandMsg:
		// Handle slash commands
		switch msg.Command {
		case "/help":
			// Show help modal
			m.helpModal.Show()
			return m, nil
		case "/status":
			// Show status modal
			authStatus := auth.GetAuthStatus()
			m.statusModal.Show(authStatus, m.viewport.width, m.viewport.height)
			return m, nil
		case "/clear":
			// Clear conversation history
			m.messages = []components.Message{}
			m.tokenCount = 0
			return m, nil
		case "/editor":
			// Open external editor
			return m, m.openExternalEditor()
		case "/compact":
			// Compact conversation history
			m.processing = true
			m.processingText = "Compacting conversation..."
			m.processingSpinner = components.NewSpinnerComponent("")
			return m, tea.Batch(
				m.startAnimation(),
				m.compactConversation(),
			)
		case "/login":
			// Start login flow
			return m, m.startLoginFlow()
		case "/logout":
			// Logout immediately
			return m, m.startLogoutFlow()
		}

	case EditorFinishedMsg:
		// Handle external editor result
		if msg.Error != nil {
			// Show error message
			m.messages = append(m.messages, components.Message{
				ID:        generateMessageID(),
				Role:      "system",
				Content:   fmt.Sprintf("Error opening editor: %s", msg.Error.Error()),
				Type:      components.MessageTypeText,
				Status:    components.MessageError,
				IsError:   true,
				Timestamp: time.Now(),
				UpdatedAt: time.Now(),
			})
		}
		// Don't modify textarea - editor was just for convenience
		return m, nil

	case CompactConversationMsg:
		if msg.Error != nil {
			// Show error message
			m.messages = append(m.messages, components.Message{
				ID:        generateMessageID(),
				Role:      "system",
				Content:   fmt.Sprintf("Error compacting conversation: %s", msg.Error.Error()),
				Type:      components.MessageTypeText,
				Status:    components.MessageError,
				IsError:   true,
				Timestamp: time.Now(),
				UpdatedAt: time.Now(),
			})
			m.processing = false
			m.processingText = ""
			m.processingSpinner = nil
			return m, nil
		}

		// Clear conversation history
		m.messages = []components.Message{}

		// Add summary as first user message
		summaryMsg := components.Message{
			ID:        generateMessageID(),
			Role:      "user",
			Content:   msg.Summary,
			Type:      components.MessageTypeText,
			Status:    components.MessageCompleted,
			Timestamp: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.messages = append(m.messages, summaryMsg)

		// Add system message about compaction
		systemMsg := components.Message{
			ID:        generateMessageID(),
			Role:      "system",
			Content:   "Previous conversation was compacted.",
			Type:      components.MessageTypeText,
			Status:    components.MessageCompleted,
			Timestamp: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.messages = append(m.messages, systemMsg)

		// Reset token count
		m.tokenCount = len(msg.Summary) // Simple approximation

		// Clear processing state
		m.processing = false
		m.processingText = ""
		m.processingSpinner = nil

		return m, nil
		
	case SetProcessingMsg:
		// Update processing state
		m.processing = msg.Active
		m.processingText = msg.Text
		if msg.Active {
			m.processingSpinner = components.NewSpinnerComponent("")
			return m, m.startAnimation()
		} else {
			m.processingSpinner = nil
		}
		return m, nil
		
	case StoreVerifierAndShowModalMsg:
		// Store the verifier first
		m.authVerifier = msg.Verifier
		
		// Then show the auth modal
		var message string
		if msg.BrowserOpened {
			message = "Browser opened. Please authenticate and copy the authorization code."
		} else {
			message = "Could not open browser. Please visit the URL below and copy the authorization code."
		}
		
		cmd := m.authModal.Show(components.AuthModalConfig{
			Title:    "Claude Max Authentication",
			Message:  message,
			URL:      msg.URL,
			Width:    m.viewport.width,
			Height:   m.viewport.height,
			OnSubmit: func(code string) tea.Cmd {
				return m.handleAuthCode(code, m.authVerifier)
			},
			OnCancel: func() tea.Cmd {
				return func() tea.Msg {
					return AuthFlowCompleteMsg{
						Success: false,
						Message: "Authentication cancelled",
					}
				}
			},
		})
		return m, cmd
		
	case ShowAuthModalMsg:
		// Show the auth modal
		var message string
		if msg.BrowserOpened {
			message = "Browser opened. Please authenticate and copy the authorization code."
		} else {
			message = "Could not open browser. Please visit the URL below and copy the authorization code."
		}
		
		cmd := m.authModal.Show(components.AuthModalConfig{
			Title:    "Claude Max Authentication",
			Message:  message,
			URL:      msg.URL,
			Width:    m.viewport.width,
			Height:   m.viewport.height,
			OnSubmit: func(code string) tea.Cmd {
				return m.handleAuthCode(code, m.authVerifier)
			},
			OnCancel: func() tea.Cmd {
				return func() tea.Msg {
					return AuthFlowCompleteMsg{
						Success: false,
						Message: "Authentication cancelled",
					}
				}
			},
		})
		return m, cmd
		
	case AuthFlowCompleteMsg:
		// Handle auth flow completion
		m.authVerifier = ""
		m.authModal.Hide()
		
		if msg.Success {
			// Reinitialize client
			newClient, err := auth.NewClient()
			if err != nil {
				logger.Debug("Failed to reinitialize client: %v", err)
			} else {
				m.client = newClient
				m.agent = agent.NewAgent(&m.client, nil, m.toolDefs, systemPromptContent)
			}
			m.processingText = msg.Message
			// Clear processing after a delay
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return SetProcessingMsg{Active: false}
			})
		} else {
			// Show error
			m.processingText = msg.Message
			m.processing = false
			m.processingSpinner = nil
			return m, nil
		}
		
	
	}

	m.textarea, cmd = m.textarea.Update(msg)

	// Dynamically adjust textarea height based on content
	lines := strings.Count(m.textarea.Value(), "\n") + 1

	maxHeight := min(max((m.viewport.height)/2, 1), 12) // Between 1-12 lines
	height := min(max(lines, 1), maxHeight)

	if height != m.textarea.Height() {
		m.textarea.SetHeight(height)
	}

	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// buildConversationHistory converts TUI messages to Claude conversation format
func (m Model) buildConversationHistory() []anthropic.MessageParam {
	var conversation []anthropic.MessageParam
	for _, msg := range m.messages {
		if msg.Role == "user" && msg.Content != "" {
			conversation = append(conversation, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		} else if msg.Role == "assistant" && !msg.IsError && msg.Content != "" && msg.Status == components.MessageCompleted {
			conversation = append(conversation, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		}
		// Skip error messages, empty messages, and processing messages from conversation history
	}
	return conversation
}

// processMessage sends a message to the agent with conversation history using new message system
func (m Model) processMessage(message string) tea.Cmd {
	// Generate IDs ahead of time
	userMessageID := generateMessageID()
	agentMessageID := generateMessageID()

	return func() tea.Msg {
		return ProcessMessageSequenceMsg{
			UserMessage:    message,
			UserMessageID:  userMessageID,
			AgentMessageID: agentMessageID,
		}
	}
}

// processAgentRequestWithID handles the actual agent processing with progress updates
func (m Model) processAgentRequestWithID(originalMessage string, agentMessageID string) tea.Cmd {
	// First, return a batch command that includes file reference messages
	fileRefMessages, fileRefCmds, err := m.executeFileReferences(originalMessage)
	if err != nil {
		return func() tea.Msg {
			return MessageUpdateMsg{
				MessageID: agentMessageID,
				Content:   fmt.Sprintf("Error processing file references: %s", err.Error()),
				Status:    components.MessageError,
				Progress:  nil,
			}
		}
	}

	// If we have file reference commands, batch them with the main processing
	if len(fileRefCmds) > 0 {
		cmds := fileRefCmds
		cmds = append(cmds, m.processAgentRequestCore(originalMessage, agentMessageID, fileRefMessages))
		return tea.Batch(cmds...)
	}

	// No file references, just do the main processing
	return m.processAgentRequestCore(originalMessage, agentMessageID, fileRefMessages)
}

func (m Model) processAgentRequestCore(originalMessage string, agentMessageID string, fileRefMessages []anthropic.MessageParam) tea.Cmd {
	return func() tea.Msg {

		// Update progress helper function (available for future use)
		_ = func(description string) MessageUpdateMsg {
			return MessageUpdateMsg{
				MessageID: agentMessageID,
				Status:    components.MessageProcessing,
				Progress: &components.Progress{
					Description: description,
				},
			}
		}

		// Build conversation history from TUI messages (original display content)
		conversation := m.buildConversationHistory()

		// Add simulated tool call cycle if any @references were found
		conversation = append(conversation, fileRefMessages...)

		// Create context with timeout and cancellation
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Use the persistent agent with conversation history
		response, err := m.agent.RunInference(ctx, conversation)
		if err != nil {
			var errMsg string
			if ctx.Err() == context.DeadlineExceeded {
				errMsg = "Request timed out after 60 seconds"
			} else if ctx.Err() == context.Canceled {
				errMsg = "Request was cancelled"
			} else {
				errMsg = fmt.Sprintf("Error: %s", err.Error())
			}
			return MessageUpdateMsg{
				MessageID: agentMessageID,
				Content:   errMsg,
				Status:    components.MessageError,
				Progress:  nil,
			}
		}

		// Handle tool calls if present
		if len(response.Content) > 0 {
			// Check if response contains tool use
			for _, content := range response.Content {
				if content.Type == "tool_use" {
					// Process tools and create messages
					// We need to return a message that will trigger the batch command
					return ProcessToolsMsg{
						Conversation:   conversation,
						Response:       response,
						AgentMessageID: agentMessageID,
					}
				}
			}
		}

		// Extract text content from response
		var responseText strings.Builder
		for _, content := range response.Content {
			if content.Type == "text" {
				responseText.WriteString(content.Text)
			}
		}

		return MessageUpdateMsg{
			MessageID: agentMessageID,
			Content:   responseText.String(),
			Status:    components.MessageCompleted,
			Progress:  nil,
		}
	}
}

// processToolUse handles tool execution and continues the conversation
func (m Model) processToolUse(conversation []anthropic.MessageParam, response *anthropic.Message, agentMessageID string) tea.Cmd {
	// Extract tool information
	toolUses := extractToolUses(response)

	// Add assistant's response with tool use to conversation
	conversation = append(conversation, response.ToParam())

	// Create batch of commands
	var cmds []tea.Cmd

	// First, send all tool start messages immediately
	for _, toolUse := range toolUses {
		startMsg := components.Message{
			ID:        generateMessageID(),
			Role:      "assistant",
			Content:   fmt.Sprintf("%s(%s)", toolUse.Name, formatToolArguments(toolUse.Name, toolUse.Input)),
			Type:      components.MessageTypeText,
			Status:    components.MessageCompleted,
			Timestamp: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Create command that sends this message immediately
		cmd := func(msg components.Message) tea.Cmd {
			return func() tea.Msg {
				return AddMessageMsg{Message: msg}
			}
		}(startMsg)
		cmds = append(cmds, cmd)
	}

	// Then execute tools and update the agent message with the response
	executeCmd := m.executeToolsAndRespond(conversation, toolUses, agentMessageID)
	cmds = append(cmds, executeCmd)

	// Return batch that sends messages immediately then executes tools
	return tea.Batch(cmds...)
}

// executeToolsAndRespond executes tools concurrently and updates the agent message with the final response
func (m Model) executeToolsAndRespond(conversation []anthropic.MessageParam, toolUses []agent.ToolUseInfo, agentMessageID string) tea.Cmd {
	return func() tea.Msg {
		// Execute tools concurrently
		type toolResult struct {
			index  int
			result anthropic.ContentBlockParamUnion
		}

		resultChan := make(chan toolResult, len(toolUses))

		// Launch concurrent tool executions
		for i, toolUse := range toolUses {
			go func(index int, tu agent.ToolUseInfo) {
				result := m.agent.ExecuteTool(tu.ID, tu.Name, tu.Input)
				resultChan <- toolResult{
					index:  index,
					result: result,
				}
			}(i, toolUse)
		}

		// Collect results in order
		toolResults := make([]anthropic.ContentBlockParamUnion, len(toolUses))
		for i := 0; i < len(toolUses); i++ {
			res := <-resultChan
			toolResults[res.index] = res.result
		}

		// Add tool results to conversation
		conversation = append(conversation, anthropic.NewUserMessage(toolResults...))

		// Create context with timeout for follow-up response
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Get follow-up response after tool execution
		followUpResponse, err := m.agent.RunInference(ctx, conversation)
		if err != nil {
			var errMsg string
			if ctx.Err() == context.DeadlineExceeded {
				errMsg = "Follow-up request timed out after 60 seconds"
			} else if ctx.Err() == context.Canceled {
				errMsg = "Follow-up request was cancelled"
			} else {
				errMsg = fmt.Sprintf("Error after tool execution: %s", err.Error())
			}
			return MessageUpdateMsg{
				MessageID: agentMessageID,
				Content:   errMsg,
				Status:    components.MessageError,
				Progress:  nil,
			}
		}

		// Check if follow-up response has more tool uses
		for _, content := range followUpResponse.Content {
			if content.Type == "tool_use" {
				// Return a message that will trigger more tool processing
				return ProcessToolsMsg{
					Conversation:   conversation,
					Response:       followUpResponse,
					AgentMessageID: agentMessageID,
				}
			}
		}

		// Extract text from follow-up response
		var responseText strings.Builder
		for _, content := range followUpResponse.Content {
			if content.Type == "text" {
				responseText.WriteString(content.Text)
			}
		}

		// Update the agent message with the final response
		return MessageUpdateMsg{
			MessageID: agentMessageID,
			Content:   responseText.String(),
			Status:    components.MessageCompleted,
			Progress:  nil,
		}
	}
}

// extractToolUses extracts tool use information from a message
func extractToolUses(message *anthropic.Message) []agent.ToolUseInfo {
	var toolUses []agent.ToolUseInfo
	for _, content := range message.Content {
		if content.Type == "tool_use" {
			toolUses = append(toolUses, agent.ToolUseInfo{
				ID:    content.ID,
				Name:  content.Name,
				Input: content.Input,
			})
		}
	}
	return toolUses
}

// extractFileReferences extracts @filename references from text and returns list of file paths
func (m Model) extractFileReferences(text string) []string {
	var references []string
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		char := runes[i]

		// Check for @ that is not escaped
		if char == '@' && (i == 0 || runes[i-1] != '\\') {
			// Find the end of the filename
			start := i + 1
			end := start

			// Find word boundary or whitespace (but allow / and . in filenames)
			for end < len(runes) && !isWhitespace(runes[end]) {
				end++
			}

			if end > start {
				// Extract filename
				filename := string(runes[start:end])
				references = append(references, filename)

				// Skip past the filename
				i = end - 1
			}
		} else if char == '\\' && i+1 < len(runes) && runes[i+1] == '@' {
			// Skip escaped @: \@ becomes @
			i++ // Skip the @
		}
	}

	return references
}

// executeFileReferences executes appropriate tools for @filename references and returns simulated tool call cycle
func (m Model) executeFileReferences(text string) ([]anthropic.MessageParam, []tea.Cmd, error) {
	references := m.extractFileReferences(text)
	if len(references) == 0 {
		return nil, nil, nil
	}

	var toolUseBlocks []anthropic.ContentBlockParamUnion
	var toolResultBlocks []anthropic.ContentBlockParamUnion
	var cmds []tea.Cmd

	// Get working directory from completion engine via textarea
	workingDir := "."
	if completionEngine := m.textarea.CompletionEngine(); completionEngine != nil {
		workingDir = completionEngine.GetWorkingDir()
	}

	for _, ref := range references {
		// Build full path
		fullPath := filepath.Join(workingDir, ref)

		// Check if it's a directory or file
		info, err := os.Stat(fullPath)
		if err != nil {
			// Generate error tool result
			toolID := generateMessageID()
			errorMsg := fmt.Sprintf("Error accessing %s: %v", ref, err)

			// Create fake tool use block
			toolInput := map[string]string{"path": ref}
			toolUseBlocks = append(toolUseBlocks, anthropic.NewToolUseBlock(toolID, toolInput, "read_file"))
			toolResultBlocks = append(toolResultBlocks, anthropic.NewToolResultBlock(toolID, errorMsg, true))

			// Create command to show error message
			cmd := func(ref string, err error) tea.Cmd {
				return func() tea.Msg {
					return AddMessageMsg{
						Message: components.Message{
							ID:        generateMessageID(),
							Role:      "assistant",
							Content:   fmt.Sprintf("read_file(%s) - Error: %v", ref, err),
							Type:      components.MessageTypeText,
							Status:    components.MessageError,
							Timestamp: time.Now(),
							UpdatedAt: time.Now(),
						},
					}
				}
			}(ref, err)
			cmds = append(cmds, cmd)
			continue
		}

		toolID := generateMessageID()

		if info.IsDir() {
			// Create tool use block for list_files
			toolInput := map[string]string{"path": ref}
			toolInputJSON, _ := json.Marshal(toolInput)
			toolUseBlocks = append(toolUseBlocks, anthropic.NewToolUseBlock(toolID, toolInput, "list_files"))

			// Create command to show tool invocation message
			cmd := func(ref string) tea.Cmd {
				return func() tea.Msg {
					return AddMessageMsg{
						Message: components.Message{
							ID:        generateMessageID(),
							Role:      "assistant",
							Content:   fmt.Sprintf("list_files(%s)", ref),
							Type:      components.MessageTypeText,
							Status:    components.MessageCompleted,
							Timestamp: time.Now(),
							UpdatedAt: time.Now(),
						},
					}
				}
			}(ref)
			cmds = append(cmds, cmd)

			// Execute list_files tool and get result
			result := m.agent.ExecuteTool(toolID, "list_files", toolInputJSON)
			toolResultBlocks = append(toolResultBlocks, result)
		} else {
			// Create tool use block for read_file
			toolInput := map[string]string{"path": ref}
			toolInputJSON, _ := json.Marshal(toolInput)
			toolUseBlocks = append(toolUseBlocks, anthropic.NewToolUseBlock(toolID, toolInput, "read_file"))

			// Create command to show tool invocation message
			cmd := func(ref string) tea.Cmd {
				return func() tea.Msg {
					return AddMessageMsg{
						Message: components.Message{
							ID:        generateMessageID(),
							Role:      "assistant",
							Content:   fmt.Sprintf("read_file(%s)", ref),
							Type:      components.MessageTypeText,
							Status:    components.MessageCompleted,
							Timestamp: time.Now(),
							UpdatedAt: time.Now(),
						},
					}
				}
			}(ref)
			cmds = append(cmds, cmd)

			// Execute read_file tool and get result
			result := m.agent.ExecuteTool(toolID, "read_file", toolInputJSON)
			toolResultBlocks = append(toolResultBlocks, result)
		}
	}

	// Build the message sequence: assistant message with tool uses, then user message with tool results
	var messages []anthropic.MessageParam

	// Assistant message with tool calls
	if len(toolUseBlocks) > 0 {
		messages = append(messages, anthropic.NewAssistantMessage(toolUseBlocks...))
	}

	// User message with tool results
	if len(toolResultBlocks) > 0 {
		messages = append(messages, anthropic.NewUserMessage(toolResultBlocks...))
	}

	return messages, cmds, nil
}

// expandFileReferences expands @filename references to actual file contents
func (m Model) expandFileReferences(text string) string {
	var result strings.Builder
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		char := runes[i]

		// Check for @ that is not escaped
		if char == '@' && (i == 0 || runes[i-1] != '\\') {
			// Find the end of the filename
			start := i + 1
			end := start

			// Find word boundary or whitespace (but allow / and . in filenames)
			for end < len(runes) && !isWhitespace(runes[end]) {
				end++
			}

			if end > start {
				// Extract filename
				filename := string(runes[start:end])

				// Expand to file contents
				fileContents := m.readFileOrDirectoryContents(filename)
				result.WriteString(fileContents)

				// Skip past the filename
				i = end - 1
			} else {
				// No filename after @, just write the @
				result.WriteRune(char)
			}
		} else if char == '\\' && i+1 < len(runes) && runes[i+1] == '@' {
			// Handle escaped @: \@ becomes @
			result.WriteRune('@')
			i++ // Skip the @
		} else {
			result.WriteRune(char)
		}
	}

	return result.String()
}

func (m Model) readFileOrDirectoryContents(relativePath string) string {
	// Get working directory from completion engine via textarea
	workingDir := "."
	if completionEngine := m.textarea.CompletionEngine(); completionEngine != nil {
		workingDir = completionEngine.GetWorkingDir()
	}

	// Build full path
	fullPath := filepath.Join(workingDir, relativePath)

	// Check if it's a directory or file
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Sprintf("Error accessing %s: %v", relativePath, err)
	}

	if info.IsDir() {
		return m.readDirectoryContents(fullPath, relativePath)
	} else {
		return m.readFileContents(fullPath, relativePath)
	}
}

func (m Model) readFileContents(fullPath, relativePath string) string {
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf("Error reading file %s: %v", relativePath, err)
	}

	// Format as a code block with file path
	return fmt.Sprintf("Contents of %s:\n```\n%s\n```", relativePath, string(content))
}

func (m Model) readDirectoryContents(fullPath, relativePath string) string {
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return fmt.Sprintf("Error reading directory %s: %v", relativePath, err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Contents of directory %s:\n", relativePath))

	for _, entry := range entries {
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("- %s/ (directory)\n", entry.Name()))
		} else {
			// Get file info for size
			info, err := entry.Info()
			if err == nil {
				result.WriteString(fmt.Sprintf("- %s (%d bytes)\n", entry.Name(), info.Size()))
			} else {
				result.WriteString(fmt.Sprintf("- %s\n", entry.Name()))
			}
		}
	}

	return result.String()
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// startAnimation returns a command to tick spinner animations
func (m Model) startAnimation() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return AnimationTickMsg{}
	})
}

// formatToolArguments formats tool arguments for display
func formatToolArguments(toolName string, input json.RawMessage) string {
	switch toolName {
	case "read_file", "edit_file", "write_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(input, &args); err == nil && args.Path != "" {
			return args.Path
		}
	case "list_files":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(input, &args); err == nil && args.Path != "" {
			return args.Path
		}
		return "." // Default to current directory
	case "todoread":
		return "read"
	case "todowrite":
		return "write"
	case "run_task":
		var args struct {
			Task string `json:"task"`
		}
		if err := json.Unmarshal(input, &args); err == nil && args.Task != "" {
			// Truncate long tasks
			if len(args.Task) > 50 {
				return args.Task[:47] + "..."
			}
			return args.Task
		}
	}

	// Default: show raw input if short, otherwise indicate complex args
	if len(input) < 50 {
		return string(input)
	}
	return "..."
}

// openExternalEditor opens the user's preferred editor for quick access
func (m Model) openExternalEditor() tea.Cmd {
	return func() tea.Msg {
		// Get the editor from environment variable
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim" // Default to vim if no EDITOR is set
		}

		// Just open the editor - no files, no stdin, no stdout
		c := exec.Command(editor)

		// Use tea.ExecProcess to run the external command
		return tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				return EditorFinishedMsg{
					Content: "",
					Error:   fmt.Errorf("failed to run editor: %w", err),
				}
			}
			return EditorFinishedMsg{
				Content: "",
				Error:   nil,
			}
		})()
	}
}

// EditorFinishedMsg is sent when the external editor is closed
type EditorFinishedMsg struct {
	Content string
	Error   error
}

// compactConversation summarizes the current conversation and clears history
func (m Model) compactConversation() tea.Cmd {
	return func() tea.Msg {
		// Build conversation history for summarization
		conversation := m.buildConversationHistory()

		// If no conversation to compact, return early
		if len(conversation) == 0 {
			return CompactConversationMsg{
				Summary: "",
				Error:   fmt.Errorf("no conversation to compact"),
			}
		}

		// Build a new conversation for summarization
		summaryConversation := []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(summaryPrompt)),
		}

		// Add the entire conversation as context
		for _, msg := range conversation {
			summaryConversation = append(summaryConversation, msg)
		}

		// Add final instruction to summarize
		summaryConversation = append(summaryConversation,
			anthropic.NewUserMessage(anthropic.NewTextBlock("Please summarize this conversation.")))

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Run inference to get summary
		response, err := m.agent.RunInference(ctx, summaryConversation)
		if err != nil {
			return CompactConversationMsg{
				Summary: "",
				Error:   fmt.Errorf("failed to generate summary: %w", err),
			}
		}

		// Extract text content from response
		var summary strings.Builder
		for _, content := range response.Content {
			if content.Type == "text" {
				summary.WriteString(content.Text)
			}
		}

		return CompactConversationMsg{
			Summary: summary.String(),
			Error:   nil,
		}
	}
}

