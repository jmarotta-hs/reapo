package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"reapo/internal/logger"
	"reapo/internal/schema"
)

// RunTask tool definition
var RunTaskDefinition = ToolDefinition{
	Name:        "run_task",
	Description: "Run a specific task or question with the TaskAgent. The TaskAgent will analyze the task, use available tools to gather information or perform actions, and provide a concise summary of the task's results.",
	InputSchema: schema.GenerateSchema[RunTaskInput](),
	Function:    RunTask,
}

type RunTaskInput struct {
	Task    string `json:"task" jsonschema_description:"The specific task or question to be executed by the TaskAgent"`
	Context string `json:"context,omitempty" jsonschema_description:"Optional additional context or information relevant to the task"`
}

func RunTask(input json.RawMessage) (string, error) {
	runTaskInput := RunTaskInput{}
	err := json.Unmarshal(input, &runTaskInput)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if runTaskInput.Task == "" {
		return "", fmt.Errorf("task cannot be empty")
	}

	taskSystemPrompt := `Launch a new agent that has access to the following tools: ReadFile, WriteFile, ListFiles, TodoRead, TodoWrite. When you are searching for a keyword or file and are not confident that you will find the right match in the first few tries, use the Agent tool to perform the search for you.

When to use the Agent tool:
- If you are searching for a keyword like "config" or "logger", or for questions like "which file does X?", the Agent tool is strongly recommended

When NOT to use the Agent tool:
- If you want to read a specific file path, use the Read or Glob tool instead of the Agent tool, to find the match more quickly
- If you are searching for a specific class definition like "class Foo", use the Glob tool instead, to find the match more quickly
- If you are searching for code within a specific file or set of 2-3 files, use the Read tool instead of the Agent tool, to find the match more quickly

Usage notes:
1. For maximum efficiency, whenever you need to perform multiple independent operations, invoke all relevant tools simultaneously rather than sequentially
2. When the agent is done, it will return a single message back to you. The result returned by the agent is not visible to the user. To show the user the result, you should send a text message back to the user with a concise summary of the result.
3. Each agent invocation is stateless. You will not be able to send additional messages to the agent, nor will the agent be able to communicate with you outside of its final report. Therefore, your prompt should contain a highly detailed task description for the agent to perform autonomously and you should specify exactly what information the agent should return back to you in its final and only message to you.
4. The agent's outputs should generally be trusted
5. Clearly tell the agent whether you expect it to write code or just to do research (search, file reads, web fetches, etc.), since it is not aware of the user's intent`

	client := anthropic.NewClient()
	availableTools := []ToolDefinition{ReadFileDefinition, ListFilesDefinition, EditFileDefinition, TodoReadDefinition, TodoWriteDefinition}

	// Create a temporary agent for task execution
	taskAgent := &TaskAgent{
		client:       &client,
		tools:        availableTools,
		systemPrompt: taskSystemPrompt,
	}

	result, err := taskAgent.RunTask(context.Background(), runTaskInput.Task, runTaskInput.Context)
	if err != nil {
		return "", fmt.Errorf("task execution error: %w", err)
	}

	if result == "" {
		return "TaskAgent completed the task but returned no output", nil
	}

	return fmt.Sprintf("TaskAgent Summary:\n%s", result), nil
}

// TaskAgent is a simplified agent for task execution
type TaskAgent struct {
	client       *anthropic.Client
	tools        []ToolDefinition
	systemPrompt string
}

// RunTask executes a task with the TaskAgent
func (ta *TaskAgent) RunTask(ctx context.Context, task, context string) (string, error) {
	conversation := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(fmt.Sprintf("Task: %s\n\nContext: %s", task, context))),
	}

	maxRounds := 5
	var finalResponse string

	for round := range maxRounds {
		message, err := ta.runInference(ctx, conversation)
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

		toolResults := ta.executeToolsConcurrently(toolUses)
		conversation = append(conversation, anthropic.NewUserMessage(toolResults...))
	}

	if finalResponse == "" {
		return "", fmt.Errorf("no final response received")
	}

	return finalResponse, nil
}

func (ta *TaskAgent) runInference(ctx context.Context, conversation []anthropic.MessageParam) (*anthropic.Message, error) {
	anthropicTools := []anthropic.ToolUnionParam{}
	for _, tool := range ta.tools {
		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: tool.InputSchema,
			},
		})
	}

	message, err := ta.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude4Sonnet20250514,
		MaxTokens: int64(1024),
		Messages:  conversation,
		Tools:     anthropicTools,
		System:    []anthropic.TextBlockParam{{Type: "text", Text: ta.systemPrompt}},
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

func (ta *TaskAgent) executeToolsConcurrently(toolUses []toolUseInfo) []anthropic.ContentBlockParamUnion {
	if len(toolUses) == 0 {
		return nil
	}

	results := make([]anthropic.ContentBlockParamUnion, len(toolUses))
	resultChan := make(chan toolExecutionResult, len(toolUses))

	// Kick off all tools concurrently
	for i, toolUse := range toolUses {
		go func(index int, tu toolUseInfo) {
			result := ta.executeTool(tu.id, tu.name, tu.input)
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

func (ta *TaskAgent) executeTool(id, name string, input json.RawMessage) anthropic.ContentBlockParamUnion {
	var toolDef ToolDefinition
	var found bool
	for _, tool := range ta.tools {
		if tool.Name == name {
			toolDef = tool
			found = true
			break
		}
	}
	if !found {
		return anthropic.NewToolResultBlock(id, "tool not found", true)
	}

	// Log tool execution to file instead of stdout to avoid TUI corruption
	logger.Tool(name, string(input))

	response, err := toolDef.Function(input)
	if err != nil {
		return anthropic.NewToolResultBlock(id, err.Error(), true)
	}

	return anthropic.NewToolResultBlock(id, response, false)
}
