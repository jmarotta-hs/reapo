package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// MessageStatus represents the current state of a message
type MessageStatus string

const (
	MessagePending    MessageStatus = "pending"    // User message waiting to be processed
	MessageProcessing MessageStatus = "processing" // Agent is working on this message
	MessageCompleted  MessageStatus = "completed"  // Agent finished processing
	MessageError      MessageStatus = "error"      // Processing failed
)

// MessageType represents different types of messages
type MessageType string

const (
	MessageTypeText           MessageType = "text"            // Regular user/assistant text message
	MessageTypeToolInvocation MessageType = "tool_invocation" // Tool being invoked
	MessageTypeToolResult     MessageType = "tool_result"     // Tool execution result
)

// Progress represents progress information for a message
type Progress struct {
	Description string  // What the agent is currently doing
	Step        int     // Current step number
	TotalSteps  int     // Total number of steps (if known)
	Percentage  float64 // Completion percentage (0-100)
}

// ToolInfo represents information about a tool invocation
type ToolInfo struct {
	Name       string // Tool name (e.g., "read_file", "edit_file")
	Input      string // Tool input parameters (JSON string)
	Output     string // Tool output/result
	Error      string // Error message if tool failed
	Duration   string // How long the tool took to execute
	ShowOutput bool   // Whether to show the output for this tool
}

// Message represents a chat message
type Message struct {
	ID        string        // Unique identifier for message updates
	Role      string        // "user" or "assistant"
	Content   string        // Message content (can be updated)
	Type      MessageType   // Type of message (text, tool_invocation, tool_result)
	Status    MessageStatus // Current processing status
	IsError   bool          // Legacy error flag
	Timestamp time.Time     // When message was created
	UpdatedAt time.Time     // Last update time
	Progress  *Progress     // Optional progress information
	ToolInfo  *ToolInfo     // Optional tool information for tool-related messages
}

// ShouldShowToolOutput determines if a tool's output should be displayed
func ShouldShowToolOutput(toolName string) bool {
	// Tools that should show output
	outputTools := map[string]bool{
		"edit_file":   true,
		"write_file":  true,
		"todoread":    true,
		"todowrite":   true,
		"run_task":    true,
		"list_files":  true,
	}
	
	// Tools that should only show invocation (no output)
	// read_file, and others not listed above
	
	return outputTools[toolName]
}

// ChatComponent handles the rendering of chat messages
type ChatComponent struct {
	messages []Message
	height   int
	width    int
}

// NewChatComponent creates a new chat component
func NewChatComponent(messages []Message, height int, width int) *ChatComponent {
	return &ChatComponent{
		messages: messages,
		height:   height,
		width:    width,
	}
}

// Render renders the chat messages with proper styling and scrolling
func (c *ChatComponent) Render() string {
	return c.RenderWithSpinners(nil)
}

// RenderWithSpinners renders chat messages with spinner support
func (c *ChatComponent) RenderWithSpinners(spinners map[string]*SpinnerComponent) string {
	// Styles for bullet points only
	userBulletStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))      // Blue
	assistantBulletStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // Yellow
	errorBulletStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))     // Red
	processingBulletStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // Cyan
	
	// Text style matches input text (default terminal color)
	textStyle := lipgloss.NewStyle() // No color specified, uses default

	// Build chat messages
	var chatLines []string
	for i, msg := range c.messages {
		content := c.renderMessage(msg, spinners, userBulletStyle, assistantBulletStyle, errorBulletStyle, processingBulletStyle, textStyle)
		chatLines = append(chatLines, content)
		
		// Add empty line between messages (except after the last message)
		if i < len(c.messages)-1 {
			chatLines = append(chatLines, "")
		}
	}

	// Limit chat lines to fit viewport
	chatHeight := max(c.height, 1)
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

	return chat
}

// wrapText wraps text to fit within the specified width, accounting for prefix length
func wrapText(text string, width int, prefixLen int) string {
	if width <= prefixLen {
		return text // Can't wrap meaningfully
	}
	
	availableWidth := width - prefixLen
	if availableWidth <= 0 {
		return text
	}
	
	lines := strings.Split(text, "\n")
	var wrappedLines []string
	
	for _, line := range lines {
		if len(line) <= availableWidth {
			wrappedLines = append(wrappedLines, line)
			continue
		}
		
		// Wrap this line
		var currentLine strings.Builder
		var currentLen int
		
		words := strings.Fields(line)
		for i, word := range words {
			wordLen := len(word)
			spaceLen := 0
			if i > 0 {
				spaceLen = 1 // for the space
			}
			
			// Check if adding this word would exceed the width
			if currentLen+spaceLen+wordLen > availableWidth && currentLen > 0 {
				// Start a new line
				wrappedLines = append(wrappedLines, currentLine.String())
				currentLine.Reset()
				currentLen = 0
				spaceLen = 0
			}
			
			// Add space if not the first word on the line
			if currentLen > 0 {
				currentLine.WriteString(" ")
				currentLen += 1
			}
			
			// Add the word
			currentLine.WriteString(word)
			currentLen += wordLen
		}
		
		// Add any remaining content
		if currentLine.Len() > 0 {
			wrappedLines = append(wrappedLines, currentLine.String())
		}
	}
	
	return strings.Join(wrappedLines, "\n")
}

// renderMessage renders a single message with appropriate status indicators
func (c *ChatComponent) renderMessage(msg Message, spinners map[string]*SpinnerComponent, userBulletStyle, assistantBulletStyle, errorBulletStyle, processingBulletStyle, textStyle lipgloss.Style) string {
	// Handle tool-specific messages
	if msg.Type == MessageTypeToolInvocation || msg.Type == MessageTypeToolResult {
		return c.renderToolMessage(msg, spinners, textStyle)
	}
	
	// Default to text message if Type is empty (backward compatibility)
	if msg.Type == "" {
		msg.Type = MessageTypeText
	}

	var prefix string
	var bulletStyle lipgloss.Style

	// Set role-based prefix and bullet style
	if msg.Role == "user" {
		prefix = "> "
		bulletStyle = userBulletStyle
	} else {
		prefix = "⏺ "
		bulletStyle = assistantBulletStyle
	}

	// Override bullet style for errors
	if msg.IsError || msg.Status == MessageError {
		bulletStyle = errorBulletStyle
	}

	// Set bullet style based on message status
	switch msg.Status {
	case MessagePending:
		bulletStyle = processingBulletStyle
	case MessageProcessing:
		bulletStyle = processingBulletStyle
	case MessageError:
		bulletStyle = errorBulletStyle
	}

	// Build the complete message content
	var content string
	if msg.Status == MessageProcessing && spinners != nil && spinners[msg.ID] != nil {
		content = msg.Content
		if content == "" && msg.Progress != nil {
			// For processing messages with no content, show progress inline
			content = msg.Progress.Description
		}
		
		// Wrap text accounting for bullet + spinner + space
		spinnerPrefix := prefix + spinners[msg.ID].RenderInline() + " "
		wrappedContent := wrapText(content, c.width, len(spinnerPrefix))
		
		// Handle multi-line content with proper indentation
		lines := strings.Split(wrappedContent, "\n")
		if len(lines) <= 1 {
			return bulletStyle.Render(prefix) + spinners[msg.ID].RenderInline() + " " + textStyle.Render(wrappedContent)
		}
		
		// First line gets bullet + spinner
		result := bulletStyle.Render(prefix) + spinners[msg.ID].RenderInline() + " " + textStyle.Render(lines[0])
		// Subsequent lines get indentation
		indent := strings.Repeat(" ", 3) // Fixed indentation for visual alignment
		for _, line := range lines[1:] {
			result += "\n" + indent + textStyle.Render(line)
		}
		return result
	} else {
		content = msg.Content
		// Add progress information if available
		if msg.Progress != nil && msg.Status != MessageProcessing {
			content += fmt.Sprintf("\n   %s", msg.Progress.Description)
		}
		
		// Wrap text accounting for bullet
		wrappedContent := wrapText(content, c.width, len(prefix))
		
		// Handle multi-line content with proper indentation
		lines := strings.Split(wrappedContent, "\n")
		if len(lines) <= 1 {
			return bulletStyle.Render(prefix) + textStyle.Render(wrappedContent)
		}
		
		// First line gets bullet
		result := bulletStyle.Render(prefix) + textStyle.Render(lines[0])
		// Subsequent lines get indentation
		indent := strings.Repeat(" ", 3) // Fixed indentation for visual alignment
		for _, line := range lines[1:] {
			result += "\n" + indent + textStyle.Render(line)
		}
		return result
	}
}

// renderToolMessage renders tool invocation and result messages
func (c *ChatComponent) renderToolMessage(msg Message, spinners map[string]*SpinnerComponent, textStyle lipgloss.Style) string {
	// Tool-specific styling
	toolInvocationStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // Magenta
	toolSuccessStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))    // Green
	toolErrorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))      // Red
	toolProcessingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // Cyan

	var prefix string
	var bulletStyle lipgloss.Style
	var content string

	if msg.Type == MessageTypeToolInvocation {
		prefix = "🔧 "
		bulletStyle = toolInvocationStyle

		if msg.ToolInfo != nil {
			if msg.Status == MessageProcessing {
				bulletStyle = toolProcessingStyle
				content = fmt.Sprintf("Running tool: %s", msg.ToolInfo.Name)
				
				// Show input if available (truncated)
				if msg.ToolInfo.Input != "" && msg.ToolInfo.Input != "{}" {
					inputPreview := msg.ToolInfo.Input
					if len(inputPreview) > 100 {
						inputPreview = inputPreview[:100] + "..."
					}
					content += fmt.Sprintf("\n   Input: %s", inputPreview)
				}
			} else {
				content = fmt.Sprintf("Tool: %s", msg.ToolInfo.Name)
			}
		} else {
			content = "Tool invocation"
		}
	} else { // MessageTypeToolResult
		if msg.ToolInfo != nil && msg.ToolInfo.Error != "" {
			prefix = "❌ "
			bulletStyle = toolErrorStyle
			content = fmt.Sprintf("Tool %s failed: %s", msg.ToolInfo.Name, msg.ToolInfo.Error)
		} else {
			prefix = "✅ "
			bulletStyle = toolSuccessStyle
			content = fmt.Sprintf("Tool %s completed", msg.ToolInfo.Name)
			if msg.ToolInfo != nil && msg.ToolInfo.Duration != "" {
				content += fmt.Sprintf(" (%s)", msg.ToolInfo.Duration)
			}
			
			// Show truncated output if available and tool should show output
			if msg.ToolInfo != nil && msg.ToolInfo.Output != "" && msg.ToolInfo.ShowOutput {
				outputPreview := msg.ToolInfo.Output
				if len(outputPreview) > 200 {
					outputPreview = outputPreview[:200] + "..."
				}
				content += fmt.Sprintf("\n   Result: %s", outputPreview)
			}
		}
	}

	// Handle processing state with spinner
	if msg.Status == MessageProcessing && spinners != nil && spinners[msg.ID] != nil {
		// Wrap text accounting for prefix + spinner + space
		spinnerPrefix := prefix + spinners[msg.ID].RenderInline() + " "
		wrappedContent := wrapText(content, c.width, len(spinnerPrefix))
		
		// Handle multi-line content with proper indentation
		lines := strings.Split(wrappedContent, "\n")
		if len(lines) <= 1 {
			return bulletStyle.Render(prefix) + spinners[msg.ID].RenderInline() + " " + textStyle.Render(wrappedContent)
		}
		
		// First line gets prefix + spinner
		result := bulletStyle.Render(prefix) + spinners[msg.ID].RenderInline() + " " + textStyle.Render(lines[0])
		// Subsequent lines get indentation
		indent := strings.Repeat(" ", len(prefix)+2) // prefix + spinner width
		for _, line := range lines[1:] {
			result += "\n" + indent + textStyle.Render(line)
		}
		return result
	} else {
		// Regular rendering without spinner
		wrappedContent := wrapText(content, c.width, len(prefix))
		
		// Handle multi-line content with proper indentation
		lines := strings.Split(wrappedContent, "\n")
		if len(lines) <= 1 {
			return bulletStyle.Render(prefix) + textStyle.Render(wrappedContent)
		}
		
		// First line gets prefix
		result := bulletStyle.Render(prefix) + textStyle.Render(lines[0])
		// Subsequent lines get indentation
		indent := strings.Repeat(" ", len(prefix))
		for _, line := range lines[1:] {
			result += "\n" + indent + textStyle.Render(line)
		}
		return result
	}
}
