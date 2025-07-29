package tools

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"reapo/internal/schema"
)

// Todo represents a todo item
type Todo struct {
	ID          string     `json:"id"`
	Text        string     `json:"text"`
	Completed   bool       `json:"completed"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Global in-memory storage with thread safety
var (
	todos       []Todo
	todoCounter int
	todosMutex  sync.RWMutex
)

func generateTodoID() string {
	todosMutex.Lock()
	defer todosMutex.Unlock()
	todoCounter++
	return fmt.Sprintf("todo_%d", todoCounter)
}

// TodoRead tool definition
var TodoReadDefinition = ToolDefinition{
	Name:        "todoread",
	Description: "List all todos with their current status (completed or pending).",
	InputSchema: schema.GenerateSchema[TodoReadInput](),
	Function:    TodoRead,
}

type TodoReadInput struct {
	// No parameters needed for listing
}

func TodoRead(input json.RawMessage) (string, error) {
	todosMutex.RLock()
	defer todosMutex.RUnlock()

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

// TodoWrite tool definition
var TodoWriteDefinition = ToolDefinition{
	Name:        "todowrite",
	Description: "Create new todos or mark existing todos as completed. Use 'add' to create a new todo or 'complete' to mark a todo as done.",
	InputSchema: schema.GenerateSchema[TodoWriteInput](),
	Function:    TodoWrite,
}

type TodoWriteInput struct {
	Action string `json:"action" jsonschema_description:"Action to perform: 'add' to create a new todo, 'complete' to mark a todo as completed"`
	Text   string `json:"text,omitempty" jsonschema_description:"Todo text (required for 'add' action)"`
	ID     string `json:"id,omitempty" jsonschema_description:"Todo ID (required for 'complete' action)"`
}

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

	todosMutex.Lock()
	todos = append(todos, newTodo)
	todosMutex.Unlock()

	return fmt.Sprintf("Added todo: %s (ID: %s)", newTodo.Text, newTodo.ID), nil
}

func completeTodo(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("todo ID cannot be empty")
	}

	todosMutex.Lock()
	defer todosMutex.Unlock()

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
