# File Reference Implementation Plan

## Overview

Replace the current `@` file expansion system with proactive tool execution to provide a cleaner TUI experience and eliminate redundant tool calls.

## Current Issues

1. **Messy TUI Display**: `@_docs/roadmap.md` shows full file contents in chat
2. **Redundant Tool Calls**: Claude receives expanded content but still calls `read_file`
3. **Complex Logic**: Manual file expansion with formatting inconsistencies
4. **Protocol Mismatch**: File contents come through text expansion rather than tool results

## Target Solution

Transform `@` references from text expansion into proactive tool execution:

- **User types**: `@_docs/roadmap.md What is in here?`
- **TUI shows**: `@_docs/roadmap.md What is in here?` (clean, original message)
- **Claude receives**: Original message + preemptive tool results
- **No redundant calls**: Claude already has file contents via tool protocol

## Implementation Steps

### 1. Create File Reference Parser
```go
func extractFileReferences(message string) []string {
    // Parse @filename syntax (reuse existing regex logic)
    // Return list of file paths to read
}
```

### 2. Add File/Directory Detection
```go
func (m Model) executeFileReferences(message string) []anthropic.ContentBlockParamUnion {
    references := extractFileReferences(message)
    var toolResults []anthropic.ContentBlockParamUnion
    
    for _, ref := range references {
        if isDirectory(ref) {
            // Execute list_files tool
            result := m.agent.ExecuteTool(generateID(), "list_files", marshalInput(ref))
        } else {
            // Execute read_file tool  
            result := m.agent.ExecuteTool(generateID(), "read_file", marshalInput(ref))
        }
        toolResults = append(toolResults, result)
    }
    return toolResults
}
```

### 3. Update Message Processing Flow
**Current flow:**
```
ProcessMessageSequenceMsg → expandFileReferences() → buildConversationHistory()
```

**New flow:**
```
ProcessMessageSequenceMsg → executeFileReferences() → buildConversationWithToolResults()
```

### 4. Build Enhanced Conversation
```go
func (m Model) buildConversationWithFileReferences(originalMessage string) []anthropic.MessageParam {
    // Build conversation from TUI messages (original display content)
    conversation := m.buildConversationHistory()
    
    // Execute proactive tool calls for @references
    toolResults := m.executeFileReferences(originalMessage)
    
    if len(toolResults) > 0 {
        // Add tool results as user message
        conversation = append(conversation, anthropic.NewUserMessage(toolResults...))
    }
    
    return conversation
}
```

### 5. Cleanup Legacy Code
- Remove `expandFileReferences()` function
- Remove `readFileContents()`, `readDirectoryContents()` helpers  
- Remove `readFileOrDirectoryContents()` logic
- Simplify `processAgentRequestWithID()` flow

## Expected Conversation Structure

**Before (current):**
```
User: "Contents of _docs/roadmap.md:\n```\n- Add @ for files\n- fix logging\n...\n``` What is in here?"
```

**After (new):**
```
User: "@_docs/roadmap.md What is in here?"
Tool Result (read_file): "- Add @ for files\n- fix logging\n..."
```

## Tool Mapping

| Reference Type | Tool Used | Example |
|---------------|-----------|---------|
| File | `read_file` | `@internal/agent.go` → `read_file({"path": "internal/agent.go"})` |
| Directory | `list_files` | `@internal/` → `list_files({"path": "internal/"})` |

## Benefits

1. **Clean TUI**: Shows `@file` references instead of expanded content
2. **No Redundant Calls**: Claude won't call `read_file` again (already has content)
3. **Standard Protocol**: Uses Anthropic's expected tool result format
4. **Simpler Code**: Removes complex expansion and formatting logic
5. **Consistent Behavior**: All file access goes through standard tool system
6. **Better Performance**: No duplicate file reads

## Files to Modify

1. **internal/tui/update.go**:
   - Replace `expandFileReferences()` with `extractFileReferences()`
   - Add `executeFileReferences()` method
   - Update `processAgentRequestWithID()` to use tool execution
   - Remove file content expansion logic

2. **Testing**:
   - Verify `@filename` works without redundant tool calls
   - Test `@directory/` uses `list_files` appropriately
   - Confirm TUI shows clean original messages
   - Validate Claude receives proper tool results

## Migration Strategy

1. Implement new `extractFileReferences()` function
2. Add `executeFileReferences()` alongside existing expansion
3. Test new system in parallel with old system
4. Switch `processAgentRequestWithID()` to use new system
5. Remove legacy expansion code after verification

This approach transforms `@` references from a text preprocessing feature into a first-class tool execution system that aligns with Claude's expected interaction patterns.