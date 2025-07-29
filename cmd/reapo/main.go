package main

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"reapo/internal/agent"
	"reapo/internal/logger"
	"reapo/internal/tools"
	"reapo/internal/tui"
)

//go:embed system_prompt.txt
var systemPromptContent string

func main() {
	// Initialize logger
	if err := logger.Init(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()
	logger.Debug("Starting reapo...")

	client := anthropic.NewClient()

	// Initialize task agent with client and system prompt
	tools.InitializeTaskAgent(&client, systemPromptContent)

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
	agentInstance := agent.NewAgent(&client, nil, toolDefs, systemPromptContent)

	// Run the non-interactive session with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	response, err := agentInstance.GenerateText(ctx, input)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Request timed out after 60 seconds\n")
		} else if ctx.Err() == context.Canceled {
			log.Printf("Request was cancelled\n")
		} else {
			log.Printf("Error: %s\n", err.Error())
		}
		os.Exit(1)
	}
	fmt.Print(response)
}

func runTUI(client anthropic.Client, toolDefs []tools.ToolDefinition) {
	tui.RunTUI(client, toolDefs, systemPromptContent)
}
