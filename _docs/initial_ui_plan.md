# Initial UI Plan for Reapo

## Overview

This document outlines the plan for implementing Bubble Tea to enhance the terminal UI for the Reapo AI agent CLI tool.

## Command Structure

The application will have two distinct modes:

### `reapo` - TUI Interactive Mode
- Rich terminal interface with Bubble Tea
- Conversation history, scrolling, styling
- Tool execution progress indicators
- Keyboard shortcuts and help

### `reapo run` - Non-Interactive Mode  
- Takes input from args or stdin
- Runs single task, outputs result, exits
- Perfect for scripting and CI/CD

## Usage Examples

```bash
# Interactive TUI
reapo

# Non-interactive from args
reapo run "help me fix this bug"

# Non-interactive from stdin
echo "analyze this code" | reapo run
reapo run < prompt.txt
```

## Dependencies

```go
// Only for TUI mode
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles  
github.com/charmbracelet/lipgloss
```

## TUI Interface Design

### Main Layout Components
- **Chat Area**: Full-height scrollable conversation history
- **Input Line**: Bottom input with prompt indicator
- **Status Line**: Shows tool execution status when active, or token count when idle
- **Footer**: Tool name (lower left), PWD (center), current model (lower right)

### Message Display
- User messages: `> <message>` format
- Claude responses: `• <response>` format  
- Tool calls: `• <name>(<params>)` format (different color)
- Clean, minimal styling with basic colors

## Implementation Tasks

### High Priority
1. Add Bubble Tea ecosystem dependencies to go.mod
2. Add CLI command parsing with 'reapo run' subcommand
3. Implement non-interactive mode for 'reapo run' with stdin/args
4. Create core TUI model and initial Bubble Tea program structure
5. Implement chat interface with proper message display and styling
6. Integrate with existing agent system and tool execution

### Medium Priority
7. Add input handling with enhanced text input (multiline support)
8. Implement tool execution progress indicators and status display
9. Add conversation history navigation and scrolling
10. Test the complete TUI implementation and fix any issues

### Low Priority
11. Add keyboard shortcuts and help system
12. Implement theming and visual polish with LipGloss
13. Add mouse support with BubbleZone for better interaction

## Architecture

- `main.go` handles command parsing
- TUI mode uses existing agent with Bubble Tea frontend
- Non-interactive mode uses existing agent with simple stdio
- Clean separation of concerns, no complexity around detection or flags

## Benefits

- **Simplicity**: Two clear modes with distinct purposes
- **Flexibility**: Rich interactive experience or scriptable automation
- **Maintainability**: Clean separation between UI and core logic
- **User Experience**: Appropriate interface for each use case