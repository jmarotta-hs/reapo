# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Run
```bash
# Build the application
go build -o reapo src/main.go

# Run directly from source
go run src/main.go

# Install dependencies
go mod download

# Tidy dependencies
go mod tidy
```

### Development
```bash
# Format code
go fmt ./...

# Static analysis
go vet ./...

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...
```

## Architecture

This is a Go-based AI agent CLI tool that integrates with Anthropic's Claude API. The architecture consists of:

**Current Structure** (single-file implementation):
- `src/main.go` - Main application with all components (~1000+ lines)
- Embedded system prompts in `src/system_prompt.txt` and `src/system_prompt_tasks.txt`
- Basic tool system with file operations, todo management, and task execution

**Core Components**:
- **Agent System**: Interactive and task-specific agents using Claude API
- **Tool Registry**: Built-in tools for file operations, todo management, and task running
- **Concurrent Tool Execution**: Tools run in parallel for improved performance
- **JSON Schema Generation**: Dynamic schema creation for tool parameters

**Key Tools Available**:
- `read_file` - Read file contents
- `list_files` - Directory listings with recursive walk
- `edit_file` - String replacement-based file editing
- `todoread`/`todowrite` - In-memory todo management
- `run_task` - Spawn sub-agents for complex tasks

**Data Flow**:
1. User input → Agent conversation loop
2. Claude API call with tool definitions
3. Concurrent tool execution for multiple tool calls
4. Results fed back to conversation context
5. Continues until no more tool calls needed

## Refactoring Plans

The project has documented plans to reorganize from the current monolithic structure into:
- `cmd/` - CLI entry points
- `internal/agent/` - Agent logic and conversation management  
- `internal/tools/` - Tool implementations and registry
- `internal/models/` - Data structures
- `pkg/schema/` - Reusable schema generation

## Dependencies

- `github.com/anthropics/anthropic-sdk-go` - Claude API client
- `github.com/invopop/jsonschema` - JSON schema generation
- `github.com/mattn/go-sqlite3` - SQLite database (planned for persistence)

## Development Notes

- All code currently resides in `src/main.go` - refer to this file for implementations
- System prompts are embedded via `//go:embed` directives
- Uses Claude Sonnet 4 model specifically
- Tool execution is designed to be concurrent and stateless
- No external configuration files currently used
- In-memory todo system (no persistence yet)