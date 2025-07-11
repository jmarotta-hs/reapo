package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"reapo/internal/tools"
)

// Agent represents an AI agent that can interact with tools
type Agent struct {
	client         *anthropic.Client
	getUserMessage func() (string, bool)
	tools          []tools.ToolDefinition
	systemPrompt   string
}

// NewAgent creates a new interactive agent
func NewAgent(
	client *anthropic.Client,
	getUserMessage func() (string, bool),
	toolDefs []tools.ToolDefinition,
	systemPrompt string,
) *Agent {
	return &Agent{
		client:         client,
		getUserMessage: getUserMessage,
		tools:          toolDefs,
		systemPrompt:   systemPrompt,
	}
}

// NewTaskAgent creates a new task-specific agent
func NewTaskAgent(client *anthropic.Client, toolDefs []tools.ToolDefinition, systemPrompt string) *Agent {
	return &Agent{
		client:       client,
		tools:        toolDefs,
		systemPrompt: systemPrompt,
	}
}

// Run starts the interactive agent loop
func (a *Agent) Run(ctx context.Context) error {
	if a.getUserMessage == nil {
		return fmt.Errorf("getUserMessage function required for interactive mode")
	}

	conversation := []anthropic.MessageParam{}
	fmt.Println("Chat with Claude (use 'ctrl-c' to quit)")

	readUserInput := true
	for {
		if readUserInput {
			fmt.Print("\u001b[94mYou\u001b[0m: ")
			userInput, ok := a.getUserMessage()
			if !ok {
				break
			}

			userMessage := anthropic.NewUserMessage(anthropic.NewTextBlock(userInput))
			conversation = append(conversation, userMessage)
		}

		message, err := a.runInference(ctx, conversation)
		if err != nil {
			return err
		}
		conversation = append(conversation, message.ToParam())

		toolUses := []toolUseInfo{}
		for _, content := range message.Content {
			switch content.Type {
			case "text":
				fmt.Printf("\u001b[93mClaude\u001b[0m: %s\n", content.Text)
			case "tool_use":
				toolUses = append(toolUses, toolUseInfo{
					id:    content.ID,
					name:  content.Name,
					input: content.Input,
				})
			}
		}

		toolResults := a.executeToolsConcurrently(toolUses)

		if len(toolResults) == 0 {
			readUserInput = true
			continue
		}
		readUserInput = false
		conversation = append(conversation, anthropic.NewUserMessage(toolResults...))
	}

	return nil
}

// RunTask executes a single task and returns the result
func (a *Agent) RunTask(ctx context.Context, task, context string) (string, error) {
	conversation := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(fmt.Sprintf("Task: %s\n\nContext: %s", task, context))),
	}

	maxRounds := 5
	var finalResponse string

	for round := range maxRounds {
		message, err := a.runInference(ctx, conversation)
		if err != nil {
			return "", fmt.Errorf("task execution failed at round %d: %w", round+1, err)
		}

		conversation = append(conversation, message.ToParam())

		toolUses := []toolUseInfo{}
		hasToolUse := false

		for _, content := range message.Content {
			switch content.Type {
			case "text":
				finalResponse = content.Text
			case "tool_use":
				hasToolUse = true
				toolUses = append(toolUses, toolUseInfo{
					id:    content.ID,
					name:  content.Name,
					input: content.Input,
				})
			}
		}

		if !hasToolUse {
			break
		}

		toolResults := a.executeToolsConcurrently(toolUses)
		conversation = append(conversation, anthropic.NewUserMessage(toolResults...))
	}

	if finalResponse == "" {
		return "", fmt.Errorf("no final response received")
	}

	return finalResponse, nil
}

func (a *Agent) runInference(ctx context.Context, conversation []anthropic.MessageParam) (*anthropic.Message, error) {
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

	message, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude4Sonnet20250514,
		MaxTokens: int64(1024),
		Messages:  conversation,
		Tools:     anthropicTools,
		System:    []anthropic.TextBlockParam{{Type: "text", Text: a.systemPrompt}},
	})
	return message, err
}

type toolUseInfo struct {
	id    string
	name  string
	input json.RawMessage
}

type toolExecutionResult struct {
	index  int
	result anthropic.ContentBlockParamUnion
	err    error
}

func (a *Agent) executeToolsConcurrently(toolUses []toolUseInfo) []anthropic.ContentBlockParamUnion {
	if len(toolUses) == 0 {
		return nil
	}

	results := make([]anthropic.ContentBlockParamUnion, len(toolUses))
	resultChan := make(chan toolExecutionResult, len(toolUses))

	// Kick off all tools concurrently
	for i, toolUse := range toolUses {
		go func(index int, tu toolUseInfo) {
			result := a.executeTool(tu.id, tu.name, tu.input)
			resultChan <- toolExecutionResult{
				index:  index,
				result: result,
				err:    nil,
			}
		}(i, toolUse)
	}

	// Collect all results
	for i := 0; i < len(toolUses); i++ {
		execResult := <-resultChan
		results[execResult.index] = execResult.result
	}

	return results
}

func (a *Agent) executeTool(id, name string, input json.RawMessage) anthropic.ContentBlockParamUnion {
	var toolDef tools.ToolDefinition
	var found bool
	for _, tool := range a.tools {
		if tool.Name == name {
			toolDef = tool
			found = true
			break
		}
	}
	if !found {
		return anthropic.NewToolResultBlock(id, "tool not found", true)
	}

	// Simple logging - could be customized based on agent type if needed
	fmt.Printf("\u001b[92mtool\u001b[0m: %s(%s)\n", name, input)

	response, err := toolDef.Function(input)
	if err != nil {
		return anthropic.NewToolResultBlock(id, err.Error(), true)
	}

	return anthropic.NewToolResultBlock(id, response, false)
}

// GetUserInputFromStdin creates a getUserMessage function that reads from stdin
func GetUserInputFromStdin() func() (string, bool) {
	scanner := bufio.NewScanner(os.Stdin)
	return func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}
}