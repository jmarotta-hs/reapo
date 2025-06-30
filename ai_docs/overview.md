# Reapo Agent CLI Overview

## Vision

Reapo is a command-line interface for interacting with AI agents. It provides a short-lived CLI process that leverages persistent storage through SQLite to maintain context and history across sessions. The architecture supports multiple LLM providers through a clean abstraction layer and enables parallel tool execution for efficient task processing.

## Architecture

### Core Components

```
reapo/
├── src/
│   ├── main.go              # CLI entry point and command handling
│   ├── agent/               # Agent orchestration logic
│   │   ├── agent.go         # Agent interface and core logic
│   │   ├── conversation.go  # Conversation management
│   │   └── storage.go       # SQLite persistence layer
│   ├── adapters/            # LLM provider implementations
│   │   ├── adapter.go       # Provider interface definition
│   │   ├── claude/          # Anthropic Claude implementation
│   │   │   ├── client.go    # Claude API client
│   │   │   ├── sonnet.go    # Claude Sonnet 4 configuration
│   │   │   └── opus.go      # Claude Opus 4 configuration
│   │   └── registry.go      # Provider registry and factory
│   └── tools/               # Tool implementations
│       ├── tool.go          # Tool interface and base types
│       ├── registry.go      # Tool registry and parallel execution
│       ├── bash.go          # Shell command execution
│       ├── edit_file.go     # Edit file content
│       ├── read_file.go     # Read single file
│       ├── write_file.go    # Write single file
│       ├── read_multiple_files.go   # Read multiple files
│       ├── write_multiple_files.go  # Write multiple files
│       ├── glob.go          # Glob pattern matching
│       ├── find_file.go     # Fast file finding (fd-like)
│       ├── ripgrep.go       # Content search with regex
│       ├── ls.go            # Directory listing
│       ├── webfetch.go      # HTTP content fetching
│       ├── websearch.go     # Anthropic web search
│       └── task.go          # Create a subagent
├── database/
│   └── schema.sql           # SQLite schema definitions
└── config/
    └── default.json         # Default configuration
```

### Key Design Principles

1. **Short-lived CLI Process**: Each invocation is stateless at the process level
2. **Persistent Context**: SQLite database maintains conversation history and agent state
3. **Parallel Tool Execution**: Tools can be executed concurrently for improved performance
4. **Provider Abstraction**: Clean interface allows swapping between LLM providers
5. **Modular Tools**: Easy to add new tools by implementing the Tool interface

## Database Schema

```sql
-- Sessions table
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    metadata JSON
);

-- Messages table
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content TEXT NOT NULL,
    tool_calls JSON,
    tool_results JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id)
);
```

## Provider Abstraction

```go
// Provider interface for LLM adapters
type Provider interface {
    // Complete a conversation with tool support
    Complete(ctx context.Context, messages []Message, tools []Tool) (*Response, error)

    // Stream conversation completion
    StreamComplete(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamChunk, error)

    // Get provider metadata
    GetMetadata() ProviderMetadata
}

// Example usage
provider := adapters.NewClaudeProvider(adapters.ClaudeConfig{
    Model: "claude-3-5-sonnet-20241022",
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
})
```

## Tool System

```go
// Tool interface for all tools
type Tool interface {
    // Get tool metadata including parameters schema
    GetMetadata() ToolMetadata

    // Execute the tool with given parameters
    Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)

    // Validate parameters before execution
    Validate(params map[string]interface{}) error
}

// Parallel tool execution
results := tools.ExecuteParallel(ctx, []ToolCall{
    {Name: "read_file", Params: map[string]interface{}{"path": "config.yaml"}},
    {Name: "list_files", Params: map[string]interface{}{"directory": "./src"}},
})
```

## CLI Structure

```bash
reapo/
├── chat [message]                # Start/continue conversation
│   ├── --session <name>          # Use specific session
│   ├── --resume-last             # Resume most recent
│   ├── --new [title]             # Force new session
│   ├── --pick                    # Interactive selection
|   ├── --permission <mode>       # Permission mode (default, read, allowAll)
│   ├── --allow-tools <list>      # Allow specific tools (comma-separated)
│   ├── --deny-tools <list>       # Deny specific tools (comma-separated)
│   ├── --no-tools                # Disable all tool execution
│   ├── --confirm-tools           # Prompt before each tool execution
│   └── --unsafe                  # Allow all tools without confirmation
├── sessions/                     # Session management
│   ├── list                      # List sessions with metadata
│   │   └── --limit <n>           # Limit number of results
│   ├── show <session>            # View session content
│   │   └── --format <type>       # Output format (markdown, json)
│   ├── search <query>            # Cross-session search
│   ├── export <session>          # Export session
│   │   └── --format <type>       # Export format (json, markdown)
│   ├── delete <session>          # Delete session
│   │   └── --force               # Skip confirmation prompt
│   └── stats                     # Usage statistics
└── tools/                        # Tool management
    ├── list                      # Available tools
    │   └── --enabled-only        # Show only enabled tools
    └── show <tool>               # Show tool details and schema
```

## CLI Usage

```bash
# Start a new conversation
reapo chat "Help me understand this codebase"

# Continue the last conversation
reapo chat -r "What about the database schema?"
reapo chat --resume "What about the database schema?"

# Continue a previous conversation (specify session ID)
reapo chat -c <session-id> "What about the test files?"
reapo chat --continue <session-id> "What about the test files?"

# Use a specific model
reapo chat -m claude-opus "Analyze this architecture"
reapo chat --model claude-opus "Analyze this architecture"

# List available tools
reapo tools list

# Show conversation history
reapo session list
reapo sessions show --recent=10    # Show recent messages across sessions
reapo sessions show <session>      # Show specific session

# Export conversation
reapo export --session <session-id> --format json > conversation.json
```

## Configuration

```json
// ~/.config/reapo/config.json
{
  "database": {
    "path": "~/.reapo/reapo.db"
  },
  "providers": {
    "default": "claude-sonnet",
  },
  "permissions": {
    "mode": "default" | "read" | "allowAll"
  },
  "tools": {
    "enabled": [
      "file_read",
      "file_list",
      "file_edit"
    ],
  },
  "logging": {
    "level": "info",
    "format": "json"
  }
}
```

## Implementation Roadmap

### Phase 1: Core Infrastructure
- [ ] SQLite database setup and migrations
- [ ] Provider abstraction with Claude implementation
- [ ] Basic tool system with file operations
- [ ] CLI command structure

### Phase 2: Enhanced Features
- [ ] Parallel tool execution
- [ ] Conversation branching
- [ ] Tool result caching
- [ ] Export/import functionality

### Phase 3: Extended Capabilities
- [ ] Additional providers (OpenAI, Gemini)
- [ ] Web interaction tools

## Development Guidelines

1. **Tools** should be stateless and focused on a single responsibility
2. **Providers** should handle rate limiting and retries internally
3. **Database operations** should use transactions for consistency
4. **CLI commands** should provide both interactive and scriptable interfaces
5. **Error handling** should be comprehensive with helpful error messages

## Security Considerations

1. API keys stored in environment variables, never in database
2. Tool execution sandboxing for file system operations
3. Input validation for all user-provided data
4. Audit logging for sensitive operations
