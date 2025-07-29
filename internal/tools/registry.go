package tools

import (
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"reapo/internal/agent"
)

// Tool represents a tool that can be used by the agent
type Tool interface {
	Name() string
	Description() string
	InputSchema() anthropic.ToolInputSchemaParam
	Execute(input json.RawMessage) (string, error)
}

// ToolDefinition is an alias for agent.ToolDefinition
type ToolDefinition = agent.ToolDefinition

// Registry manages available tools
type Registry struct {
	tools map[string]ToolDefinition
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]ToolDefinition),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool ToolDefinition) {
	r.tools[tool.Name] = tool
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (ToolDefinition, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// GetAll returns all registered tools
func (r *Registry) GetAll() []ToolDefinition {
	tools := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Execute runs a tool by name with the given input
func (r *Registry) Execute(name string, input json.RawMessage) (string, error) {
	tool, exists := r.tools[name]
	if !exists {
		return "", fmt.Errorf("tool %s not found", name)
	}

	return tool.Function(input)
}
