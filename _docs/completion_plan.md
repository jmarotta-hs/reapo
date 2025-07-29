# Completion System Implementation Plan

## Overview

Adding intelligent completion for slash commands and file/folder references in the vimtextarea. The system will provide fzf-style fuzzy filtering with real-time suggestions displayed below the input area.

## 1. Completion System Architecture

**New Components**:
- `internal/tui/completion/` - Completion engine and filtering
- `internal/tui/components/completion.go` - UI component for completion display
- Extensions to vimtextarea for completion state management

**Core Features**:
- Fuzzy matching with fzf-style scoring
- Real-time filtering as user types
- Keyboard navigation (Tab/Arrow keys/Enter)
- Context-aware completion (slash commands vs file paths)

## 2. Completion Types

### Slash Command Completion
**Trigger**: When line starts with `/` (not preceded by `\` or `"`)
**Source**: Static list of available commands
```go
var slashCommands = []CompletionItem{
    {Text: "/help", Description: "Show all available commands"},
    {Text: "/clear", Description: "Clear conversation context"}, 
    {Text: "/editor", Description: "Open external editor ($EDITOR)"},
}
```

### File/Folder Completion  
**Trigger**: When `@` is typed anywhere in line (not preceded by `\` or `"`)
**Source**: Dynamic filesystem traversal from TUI working directory
**Behavior**: 
- `@` → show files/folders in current directory
- `@path/` → show contents of specified directory  
- `@file.txt` → complete to full path

**Escape Handling**:
- `\@` → literal `@` character, no completion
- `"@` → literal `@` character within quotes, no completion
- `\/` → literal `/` character, no completion
- `"/` → literal `/` character within quotes, no completion

## 3. TUI Layout Changes

**Current Layout** (`view.go:27`):
```
chat + input + "\n\n\n" + footer
```

**New Layout**:
```
chat + completions + input + footer
```

**Space Allocation**:
- Chat: `viewport.height - textarea.height - completion.height - 4`
- Completion: Dynamic height (0-8 lines based on matches)
- Input: Current textarea height
- Footer: 1 line

## 4. VimTextarea Integration

**New State** (`types.go`):
```go
type CompletionState struct {
    active      bool
    trigger     rune // '/' or '@'  
    query       string
    items       []CompletionItem
    selected    int
    startPos    Position // Where completion started
}

// Add to Model struct
completionState CompletionState
```

**Key Handling** (`vimtextarea.go`):
- Detect `/` and `@` triggers in Insert mode (check for escape characters)
- Update completion query on character input
- Handle Tab/Arrow navigation in completion mode
- Escape cancels completion

**Trigger Detection Logic**:
```go
func shouldTriggerCompletion(content string, pos Position, char rune) bool {
    if pos.Col == 0 {
        return char == '/' // Slash at start of line
    }
    
    prevChar := content[pos.Row][pos.Col-1]
    if prevChar == '\\' || prevChar == '"' {
        return false // Escaped character
    }
    
    return char == '/' || char == '@'
}
```

## 5. Completion Engine

**Location**: `internal/tui/completion/engine.go`

```go
type CompletionItem struct {
    Text        string
    Description string
    Score       int
}

type CompletionEngine struct {
    workingDir string
    commands   []CompletionItem
}

func (c *CompletionEngine) GetCompletions(trigger rune, query string) []CompletionItem
func (c *CompletionEngine) fuzzyMatch(query string, items []CompletionItem) []CompletionItem
```

**Fuzzy Matching**:
- Character-by-character matching
- Bonus for consecutive matches
- Path-aware scoring for file completions

## 6. File System Integration

**File Walking** (`completion/filesystem.go`):
```go
func (c *CompletionEngine) getFileCompletions(query string) []CompletionItem {
    // Parse query for directory context
    // Walk filesystem from working directory
    // Return sorted results with type indicators
}
```

**Features**:
- Recursive directory traversal (with depth limit)
- File type indicators (📁 folder, 📄 file)
- Hidden file filtering (configurable)
- Git-aware filtering (ignore .git contents)

## 7. UI Component Implementation

**Location**: `internal/tui/components/completion.go`

```go
type CompletionComponent struct {
    items    []CompletionItem
    selected int
    height   int
    width    int
}

func (c CompletionComponent) Render() string {
    // Render completion list with selection highlighting
    // Show item descriptions
    // Handle scrolling for long lists
}
```

**Visual Design**:
```
┌─ Completions ────────────────────────┐
│ > /help          Show all commands   │
│   /clear         Clear context       │ 
│   /editor        Open external editor│
└──────────────────────────────────────┘
```

## 8. State Management & Integration

**Model Changes** (`model.go`):
```go
type Model struct {
    // ... existing fields
    completionEngine *completion.CompletionEngine
}
```

**Update Logic** (`update.go`):
- New message types: `CompletionUpdateMsg`, `CompletionSelectMsg`
- Handle completion navigation in key event processing
- Update completion state before passing to textarea

## 9. Key Event Flow

**Completion Trigger**:
1. User types `/` or `@` in Insert mode
2. VimTextarea detects trigger and activates completion
3. Initial completion list generated and displayed

**Real-time Filtering**:
1. User continues typing after trigger
2. Query updated in completion state
3. Completion list re-filtered with fuzzy matching
4. UI updated with new results

**Selection & Insertion**:
1. User navigates with Tab/Arrows and presses Enter
2. Selected completion replaces query text
3. Cursor positioned after inserted text
4. Completion state cleared

## 10. Technical Implementation Notes

**Performance Considerations**:
- Debounce filesystem queries for `@` completions
- Limit completion results (max 50 items)
- Cache directory listings briefly

**Error Handling**:
- Graceful fallback if filesystem access fails
- Invalid path handling for `@` completions
- Empty completion list states

**Integration Points**:
- `update.go:33-45` - Intercept completion navigation before message sending
- `view.go:27` - Modify layout calculation for completion display
- `vimtextarea/vimtextarea.go:244` - Extend Insert mode key handling

## 11. File Structure

```
internal/tui/
├── completion/
│   ├── engine.go          # Core completion logic
│   ├── filesystem.go      # File/folder completion
│   └── fuzzy.go           # Fuzzy matching algorithm
├── components/
│   └── completion.go      # Completion UI component
├── update.go              # Modified: completion message handling
├── view.go                # Modified: layout with completion area
└── model.go               # Modified: completion engine integration
```