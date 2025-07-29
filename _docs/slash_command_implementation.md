# Slash Command Implementation Plan

This document outlines the implementation plan for adding slash commands to the vimtextarea component in the reapo TUI.

## Overview

We want to implement three core slash commands:
- `/help` - Show all available slash commands
- `/clear` - Clear the conversation context
- `/editor` - Open the external editor ($EDITOR like nvim)

## 1. Slash Command Detection & Parsing

**Location**: Add to `internal/tui/components/vimtextarea/types.go`
- Add `SlashCommand` type and state tracking
- Extend `Model` to track slash command mode

**Location**: Modify `internal/tui/components/vimtextarea/vimtextarea.go`
- Detect slash commands when Enter is pressed in Normal mode
- Parse command and arguments from textarea content
- Pattern: `/command [args]`

## 2. Command Integration Points

**TUI Integration**: Modify `internal/tui/update.go:33-45`
- Intercept Enter in Normal mode before processing message
- Check if content starts with `/` and handle as slash command
- Return appropriate `tea.Cmd` for each command type

**New Message Types**: Add to `internal/tui/model.go`
```go
type SlashCommandMsg struct {
    Command string
    Args    []string
}

type ClearContextMsg struct{}
type OpenEditorMsg struct{}
type ShowHelpMsg struct{}
```

## 3. Command Implementations

### /help Command
- **Implementation**: Add help text as static content in TUI
- **Display**: Show available commands, descriptions, and usage
- **Content**: 
  - `/help` - Show this help
  - `/clear` - Clear conversation context
  - `/editor` - Open external editor ($EDITOR)

### /clear Command
- **Implementation**: Reset `m.messages` slice in TUI model
- **Action**: Clear conversation history but keep current textarea content
- **Feedback**: Show confirmation message

### /editor Command
- **Implementation**: 
  1. Save current textarea content to temp file
  2. Launch `$EDITOR` (fallback to `vi`/`nano`) via `exec.Command`
  3. Suspend TUI with `tea.Suspend`
  4. Read modified content back when editor exits
  5. Resume TUI and update textarea
- **Error Handling**: Check if `$EDITOR` exists, handle editor failures
- **Integration**: Use `tea.ExecProcess` for proper TUI suspension/restoration

## 4. Technical Architecture

**Command Router**: `internal/tui/commands.go` (new file)
```go
type SlashCommandHandler interface {
    Execute(args []string, model *Model) (tea.Model, tea.Cmd)
}

func HandleSlashCommand(command string, args []string, model *Model) (tea.Model, tea.Cmd)
```

**State Management**: 
- Add `slashCommandMode bool` to vimtextarea Model
- Track whether we're processing a slash command
- Prevent normal message sending during slash command execution

## 5. Integration Flow

1. **Detection**: User types `/command` and presses Enter in Normal mode
2. **Interception**: `update.go:33` catches Enter before normal message processing  
3. **Routing**: Parse command and route to appropriate handler
4. **Execution**: Command executes and returns appropriate `tea.Cmd`
5. **Response**: TUI updates based on command result
6. **Cleanup**: Clear textarea and return to normal input mode

## 6. File Structure Changes

```
internal/tui/
├── commands/           # New directory
│   ├── handler.go     # Command router and interface
│   ├── help.go        # /help implementation  
│   ├── clear.go       # /clear implementation
│   └── editor.go      # /editor implementation
├── update.go          # Modified: slash command detection
└── model.go           # Modified: new message types
```

## Implementation Notes

- The `/editor` command is the most complex, requiring proper process management and TUI suspension
- `/help` and `/clear` are straightforward implementations that enhance user experience
- The architecture is designed to be extensible for adding more slash commands in the future
- Integration with the existing vim-style textarea should be seamless