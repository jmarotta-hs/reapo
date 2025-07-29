package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"reapo/internal/agent"
	"reapo/internal/schema"
)

// Global variables to store client and system prompt for task execution
var (
	taskClient       *anthropic.Client
	taskSystemPrompt string
)

// InitializeTaskAgent sets up the global client and system prompt for task execution
func InitializeTaskAgent(client *anthropic.Client, systemPrompt string) {
	taskClient = client
	taskSystemPrompt = systemPrompt
}

// runTaskWithAvailableTools uses GenerateText to execute tasks
func runTaskWithAvailableTools(input json.RawMessage) (string, error) {
	if taskClient == nil {
		return "", fmt.Errorf("task client not initialized - call InitializeTaskAgent first")
	}

	var taskInput agent.RunTaskInput
	if err := json.Unmarshal(input, &taskInput); err != nil {
		return "", fmt.Errorf("invalid task input: %w", err)
	}

	// Create an agent with available tools for task execution
	availableTools := []agent.ToolDefinition{
		ReadFileDefinition,
		ListFilesDefinition,
		EditFileDefinition,
		TodoReadDefinition,
		TodoWriteDefinition,
	}

	taskAgent := agent.NewAgent(taskClient, nil, availableTools, taskSystemPrompt)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Format the task message
	message := fmt.Sprintf("Task: %s\n\nContext: %s", taskInput.Task, taskInput.Context)

	// Use GenerateText to execute the task
	return taskAgent.GenerateText(ctx, message)
}

// RunTask tool definition
var RunTaskDefinition = ToolDefinition{
	Name:        "run_task",
	Description: "Run a specific task or question with the TaskAgent. The TaskAgent will analyze the task, use available tools to gather information or perform actions, and provide a concise summary of the task's results.",
	InputSchema: schema.GenerateSchema[agent.RunTaskInput](),
	Function:    runTaskWithAvailableTools,
}
