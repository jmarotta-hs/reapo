# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Run
```bash
# Build the application
go build -o reapo cmd/reapo/main.go

# Run directly from source
go run cmd/reapo/main.go

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

This is a Go-based AI agent CLI tool that integrates with Anthropic's Claude API. The project has been successfully refactored from a monolithic design into a clean modular architecture.

**Current Structure**:
```
reapo/
├── cmd/reapo/
│   ├── main.go              # CLI entry point and initialization
│   └── system_prompt.txt    # Main system prompt (embedded)
├── internal/
│   ├── agent/               # Agent logic and conversation management
│   │   └── agent.go         # Core agent functionality
│   ├── tools/               # Tool implementations and registry
│   │   ├── registry.go      # Tool interface and management
│   │   ├── file.go          # File operation tools
│   │   ├── todo.go          # In-memory todo management
│   │   └── task.go          # Sub-agent task spawning
│   ├── schema/              
│   │   └── generator.go     # JSON schema generation utilities
│   ├── logger/
│   │   └── logger.go        # Structured logging system
│   └── tui/                 # Terminal UI components
│       ├── model.go         # Bubble Tea model
│       ├── components/      # UI components (chat, input, vim textarea)
│       └── completion/      # Completion engine with filesystem support
```

**Core Components**:
- **Agent System**: Interactive and task-specific agents using Claude Sonnet 4 API
- **Tool Registry**: Modular tool system with concurrent execution support
- **TUI Interface**: Bubble Tea-based terminal interface with Vim-style text editing
- **Completion System**: Fuzzy completion for commands and filesystem navigation
- **Schema Generation**: Dynamic JSON schema creation for tool parameters

**Available Tools**:
- `read_file` - Read file contents with optional line ranges
- `list_files` - Directory listings with recursive traversal
- `edit_file` - String replacement-based file editing
- `todoread`/`todowrite` - In-memory todo list management
- `run_task` - Spawn sub-agents for complex tasks with dedicated context

**TUI Features**:
- Vim-style text editing with modal support
- Real-time chat interface with syntax highlighting
- Fuzzy completion for commands and file paths
- Progress indicators and request status tracking
- Conversation history management

## Dependencies

- `github.com/anthropics/anthropic-sdk-go` - Claude API client
- `github.com/invopop/jsonschema` - JSON schema generation
- `github.com/charmbracelet/bubbletea` - Terminal UI framework
- `github.com/charmbracelet/bubbles` - UI components
- `github.com/mattn/go-sqlite3` - SQLite database support

## Development Notes

- System prompts are embedded via `//go:embed` directives in `cmd/reapo/main.go`
- Uses Claude Sonnet 4 model specifically
- Tool execution is designed to be concurrent and stateless
- Logging is handled through `internal/logger` with structured output to `logs/`
- In-memory todo system with no persistence currently
- TUI supports both interactive mode and non-interactive `run` command