package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"reapo/internal/agent"
	"reapo/internal/tools"
)

//go:embed system_prompt.txt
var systemPromptContent string

func main() {
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

	// Create interactive agent
	getUserMessage := agent.GetUserInputFromStdin()
	agentInstance := agent.NewAgent(&client, getUserMessage, toolDefs, systemPromptContent)

	// Run the agent
	err := agentInstance.Run(context.TODO())
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}