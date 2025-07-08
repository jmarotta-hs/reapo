package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/invopop/jsonschema"
)

//go:embed system_prompt.txt
var systemPromptContent string

type ToolDefinition struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	InputSchema anthropic.ToolInputSchemaParam `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

type Agent struct {
	client         *anthropic.Client
	getUserMessage func() (string, bool)
	tools          []ToolDefinition
	systemPrompt   string
}

func NewAgent(
	client *anthropic.Client,
	getUserMessage func() (string, bool),
	tools []ToolDefinition,
	systemPrompt string,
) *Agent {
	return &Agent{
		client:         client,
		getUserMessage: getUserMessage,
		tools:          tools,
		systemPrompt:   systemPrompt,
	}
}

func NewTaskAgent(client *anthropic.Client, tools []ToolDefinition, systemPrompt string) *Agent {
	return &Agent{
		client:       client,
		tools:        tools,
		systemPrompt: systemPrompt,
	}
}

func main() {
	client := anthropic.NewClient()

	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	tools := []ToolDefinition{ReadFileDefinition, ListFilesDefinition, EditFileDefinition, TodoReadDefinition, TodoWriteDefinition, RunTaskDefinition}
	agent := NewAgent(&client, getUserMessage, tools, string(systemPromptContent))
	err := agent.Run(context.TODO())
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
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

func (a *Agent) executeTool(id, name string, input json.RawMessage) anthropic.ContentBlockParamUnion {
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

type toolExecutionResult struct {
	index  int
	result anthropic.ContentBlockParamUnion
	err    error
}

type toolUseInfo struct {
	id    string
	name  string
	input json.RawMessage
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

/* Tools */

func GenerateSchema[T any]() anthropic.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T

	schema := reflector.Reflect(v)

	return anthropic.ToolInputSchemaParam{
		Properties: schema.Properties,
	}
}

var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a given relative file path. Use this when you want to see what's inside a file. Do not use this with directory names.",
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative path of a file in the working directory."`
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

func ReadFile(input json.RawMessage) (string, error) {
	readFileInput := ReadFileInput{}
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		panic(err)
	}

	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List files and directories at a given path. If no path is provided, lists files in the current directory.",
	InputSchema: ListFilesInputSchema,
	Function:    ListFiles,
}

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

func ListFiles(input json.RawMessage) (string, error) {
	listFilesInput := ListFilesInput{}
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		panic(err)
	}

	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		if relPath != "." {
			if info.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	result, err := json.Marshal(files)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

var EditFileDefinition = ToolDefinition{
	Name: "edit_file",
	Description: `Make edits to a text file.

Replaces 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other.

If the file specified with path doesn't exist, it will be created.
`,
	InputSchema: EditFileInputSchema,
	Function:    EditFile,
}

type EditFileInput struct {
	Path   string `json:"path" jsonschema_description:"The path to the file"`
	OldStr string `json:"old_str" jsonschema_description:"Text to search for - must match exactly and must only have one match exactly"`
	NewStr string `json:"new_str" jsonschema_description:"Text to replace old_str with"`
}

var EditFileInputSchema = GenerateSchema[EditFileInput]()

func EditFile(input json.RawMessage) (string, error) {
	editFileInput := EditFileInput{}
	err := json.Unmarshal(input, &editFileInput)
	if err != nil {
		return "", err
	}

	if editFileInput.Path == "" || editFileInput.OldStr == editFileInput.NewStr {
		return "", fmt.Errorf("invalid input parameters")
	}

	content, err := os.ReadFile(editFileInput.Path)
	if err != nil {
		if os.IsNotExist(err) && editFileInput.OldStr == "" {
			return createNewFile(editFileInput.Path, editFileInput.NewStr)
		}
		return "", err
	}

	oldContent := string(content)
	newContent := strings.Replace(oldContent, editFileInput.OldStr, editFileInput.NewStr, -1)

	if oldContent == newContent && editFileInput.OldStr != "" {
		return "", fmt.Errorf("old_str not found in file")
	}

	err = os.WriteFile(editFileInput.Path, []byte(newContent), 0644)
	if err != nil {
		return "", err
	}

	return "OK", nil
}

func createNewFile(filePath, content string) (string, error) {
	dir := path.Dir(filePath)
	if dir != "." {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
	}

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}

	return fmt.Sprintf("Successfully created file %s", filePath), nil
}

/* TODOs for executing TaskAgent */

type Todo struct {
	ID          string     `json:"id"`
	Text        string     `json:"text"`
	Completed   bool       `json:"completed"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Global in-memory storage
var todos []Todo
var todoCounter int

func generateTodoID() string {
	todoCounter++
	return fmt.Sprintf("todo_%d", todoCounter)
}

var TodoReadDefinition = ToolDefinition{
	Name:        "todoread",
	Description: "List all todos with their current status (completed or pending).",
	InputSchema: TodoReadInputSchema,
	Function:    TodoRead,
}

type TodoReadInput struct {
	// No parameters needed for listing
}

var TodoReadInputSchema = GenerateSchema[TodoReadInput]()

func TodoRead(input json.RawMessage) (string, error) {
	if len(todos) == 0 {
		return "No todos found", nil
	}

	result := "Todos:\n"
	for _, todo := range todos {
		status := "[ ]"
		if todo.Completed {
			status = "[x]"
		}
		result += fmt.Sprintf("%s %s (ID: %s)\n", status, todo.Text, todo.ID)
	}

	return result, nil
}

/* Todo Write Tool */

var TodoWriteDefinition = ToolDefinition{
	Name:        "todowrite",
	Description: "Create new todos or mark existing todos as completed. Use 'add' to create a new todo or 'complete' to mark a todo as done.",
	InputSchema: TodoWriteInputSchema,
	Function:    TodoWrite,
}

type TodoWriteInput struct {
	Action string `json:"action" jsonschema_description:"Action to perform: 'add' to create a new todo, 'complete' to mark a todo as completed"`
	Text   string `json:"text,omitempty" jsonschema_description:"Todo text (required for 'add' action)"`
	ID     string `json:"id,omitempty" jsonschema_description:"Todo ID (required for 'complete' action)"`
}

var TodoWriteInputSchema = GenerateSchema[TodoWriteInput]()

func TodoWrite(input json.RawMessage) (string, error) {
	var todoInput TodoWriteInput
	if err := json.Unmarshal(input, &todoInput); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	switch todoInput.Action {
	case "add":
		return addTodo(todoInput.Text)
	case "complete":
		return completeTodo(todoInput.ID)
	default:
		return "", fmt.Errorf("invalid action: %s. Use 'add' or 'complete'", todoInput.Action)
	}
}

func addTodo(text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("todo text cannot be empty")
	}

	newTodo := Todo{
		ID:        generateTodoID(),
		Text:      text,
		Completed: false,
		CreatedAt: time.Now(),
	}

	todos = append(todos, newTodo)
	return fmt.Sprintf("Added todo: %s (ID: %s)", newTodo.Text, newTodo.ID), nil
}

func completeTodo(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("todo ID cannot be empty")
	}

	for i, todo := range todos {
		if todo.ID == id {
			if todo.Completed {
				return fmt.Sprintf("Todo '%s' is already completed", todo.Text), nil
			}

			now := time.Now()
			todos[i].Completed = true
			todos[i].CompletedAt = &now

			return fmt.Sprintf("Completed todo: %s", todo.Text), nil
		}
	}

	return "", fmt.Errorf("todo with ID %s not found", id)
}

/* TaskAgent */

var RunTaskDefinition = ToolDefinition{
	Name:        "run_task",
	Description: "Run a specific task or question with the TaskAgent. The TaskAgent will analyze the task, use available tools to gather information or perform actions, and provide a concise summary of the task's results.",
	InputSchema: RunTaskInputSchema,
	Function:    RunTask,
}

type RunTaskInput struct {
	Task    string `json:"task" jsonschema_description:"The specific task or question to be executed by the TaskAgent"`
	Context string `json:"context,omitempty" jsonschema_description:"Optional additional context or information relevant to the task"`
}

var RunTaskInputSchema = GenerateSchema[RunTaskInput]()

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
	taskAgent := NewTaskAgent(&client, availableTools, taskSystemPrompt)

	result, err := taskAgent.RunTask(context.Background(), runTaskInput.Task, runTaskInput.Context)
	if err != nil {
		return "", fmt.Errorf("task execution error: %w", err)
	}

	if result == "" {
		return "TaskAgent completed the task but returned no output", nil
	}

	return fmt.Sprintf("TaskAgent Summary:\n%s", result), nil
}
