package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"reapo/internal/logger"
)

// ToolUseInfo represents a tool use request from Claude
type ToolUseInfo struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// ToolDefinition represents a tool that can be called by agents
type ToolDefinition struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	InputSchema anthropic.ToolInputSchemaParam `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

// RunTaskInput represents the input for running a task
type RunTaskInput struct {
	Task    string `json:"task" jsonschema:"description=The task to execute"`
	Context string `json:"context" jsonschema:"description=Additional context for the task"`
}

// ToolCallback represents a callback function for tool lifecycle events
type ToolCallback func(event string, toolName, toolID, data string)

// Agent represents an AI agent that can interact with tools
type Agent struct {
	client       *anthropic.Client
	tools        []ToolDefinition
	systemPrompt string
	toolCallback ToolCallback
}

// NewAgent creates a new agent
func NewAgent(client *anthropic.Client, getUserMessage func() (string, bool), toolDefs []ToolDefinition, systemPrompt string) *Agent {
	return &Agent{
		client:       client,
		tools:        toolDefs,
		systemPrompt: systemPrompt,
	}
}

// SetToolCallback sets the callback function for tool lifecycle events
func (a *Agent) SetToolCallback(callback ToolCallback) {
	a.toolCallback = callback
}

// GenerateText runs inference and returns the text response
func (a *Agent) GenerateText(ctx context.Context, message string) (string, error) {
	conversation := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(message)),
	}

	response, err := a.RunInference(ctx, conversation)
	if err != nil {
		return "", err
	}

	// Extract text content from response
	var responseText strings.Builder
	for _, content := range response.Content {
		if content.Type == "text" {
			responseText.WriteString(content.Text)
		}
	}

	return responseText.String(), nil
}

// RunInference executes inference with Claude API
func (a *Agent) RunInference(ctx context.Context, conversation []anthropic.MessageParam) (*anthropic.Message, error) {
	anthropicTools := []anthropic.ToolUnionParam{}
	for _, tool := range a.tools {
		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: tool.InputSchema,
			},
		})
	}

	// Log all messages with their content and tool information
	messages := make([]map[string]interface{}, 0)
	for i, msg := range conversation {
		messageInfo := map[string]interface{}{
			"index": i,
			"role":  msg.Role,
		}
		
		// Extract different types of content
		var textContent []string
		var toolUses []map[string]interface{}
		var toolResults []map[string]interface{}
		
		for _, content := range msg.Content {
			if text := content.GetText(); text != nil {
				textContent = append(textContent, *text)
			}
			
			// Check for tool use blocks
			if content.OfToolUse != nil {
				toolUses = append(toolUses, map[string]interface{}{
					"id":   content.OfToolUse.ID,
					"name": content.OfToolUse.Name,
					"input": content.OfToolUse.Input,
				})
			}
			
			// Check for tool result blocks  
			if content.OfToolResult != nil {
				toolResults = append(toolResults, map[string]interface{}{
					"tool_use_id": content.OfToolResult.ToolUseID,
					"content":     content.OfToolResult.Content,
					"is_error":    content.OfToolResult.IsError,
				})
			}
		}
		
		if len(textContent) > 0 {
			messageInfo["text"] = textContent
		}
		if len(toolUses) > 0 {
			messageInfo["tool_uses"] = toolUses
		}
		if len(toolResults) > 0 {
			messageInfo["tool_results"] = toolResults
		}
		
		messages = append(messages, messageInfo)
	}
	
	logger.Chat("REQUEST", map[string]interface{}{
		"model":    "claude-4-sonnet-20250514",
		"messages": messages,
		"toolCount": len(anthropicTools),
	})

	message, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude4Sonnet20250514,
		MaxTokens: int64(1024),
		Messages:  conversation,
		Tools:     anthropicTools,
		System:    []anthropic.TextBlockParam{{Type: "text", Text: a.systemPrompt}},
	})

	// Log the chat response
	if err != nil {
		logger.Chat("ERROR", map[string]interface{}{
			"error": err.Error(),
		})
		logger.Error("API request failed: %v", err)
	} else {
		logger.Chat("RESPONSE", message)
	}

	return message, err
}

// ExecuteToolsConcurrently runs multiple tools in parallel
func (a *Agent) ExecuteToolsConcurrently(toolUses []ToolUseInfo) []anthropic.ContentBlockParamUnion {
	if len(toolUses) == 0 {
		return nil
	}

	results := make([]anthropic.ContentBlockParamUnion, len(toolUses))
	resultChan := make(chan struct {
		index  int
		result anthropic.ContentBlockParamUnion
	}, len(toolUses))

	// Kick off all tools concurrently
	for i, toolUse := range toolUses {
		go func(index int, tu ToolUseInfo) {
			result := a.ExecuteTool(tu.ID, tu.Name, tu.Input)
			resultChan <- struct {
				index  int
				result anthropic.ContentBlockParamUnion
			}{index, result}
		}(i, toolUse)
	}

	// Collect all results
	for range toolUses {
		execResult := <-resultChan
		results[execResult.index] = execResult.result
	}

	return results
}

// ExecuteTool executes a single tool
func (a *Agent) ExecuteTool(id, name string, input json.RawMessage) anthropic.ContentBlockParamUnion {
	var toolDef ToolDefinition
	var found bool
	for _, tool := range a.tools {
		if tool.Name == name {
			toolDef = tool
			found = true
			break
		}
	}
	if !found {
		if a.toolCallback != nil {
			a.toolCallback("error", name, id, "tool not found")
		}
		return anthropic.NewToolResultBlock(id, "tool not found", true)
	}

	// Notify UI that tool is starting
	if a.toolCallback != nil {
		a.toolCallback("start", name, id, string(input))
	}

	// Log tool execution to file instead of stdout to avoid TUI corruption
	logger.Tool(name, string(input))

	startTime := time.Now()
	response, err := toolDef.Function(input)
	duration := time.Since(startTime)

	// Notify UI of completion
	if a.toolCallback != nil {
		if err != nil {
			a.toolCallback("error", name, id, err.Error())
		} else {
			resultData := fmt.Sprintf(`{"output": %q, "duration": %q}`, response, duration.String())
			a.toolCallback("complete", name, id, resultData)
		}
	}

	if err != nil {
		return anthropic.NewToolResultBlock(id, err.Error(), true)
	}

	return anthropic.NewToolResultBlock(id, response, false)
}
