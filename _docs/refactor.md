# Refactoring Plan

## Overview
This document outlines the refactoring plan for the reapo project to improve code organization and maintainability. The current `main.go` file is 618 lines and contains all functionality in a single file.

## Current Issues
- **Monolithic structure**: All components in one file (agents, tools, schema generation, todo system)
- **Mixed concerns**: HTTP client logic mixed with business logic
- **No separation of interfaces**: Direct coupling between components
- **Global state**: In-memory todo storage with global variables
- **Hard to test**: Tightly coupled components make unit testing difficult

## Simplified Refactoring Plan

### Package Structure
```
reapo/
├── cmd/
│   └── reapo/
│       └── main.go                 # CLI entry point (~50 lines)
├── internal/
│   ├── agent/
│   │   ├── agent.go               # Agent struct and both Run methods
│   │   └── task.go                # Task-specific system prompt
│   ├── tools/
│   │   ├── registry.go            # Tool interface and registry
│   │   ├── file.go                # File operation tools
│   │   ├── todo.go                # Todo tools with in-memory storage
│   │   └── task.go                # RunTask tool (spawns sub-agents)
│   └── schema/
│       └── generator.go           # JSON schema generation
```

### Key Simplifications

**Single Agent File**:
- Keep both interactive and task modes in `internal/agent/agent.go`
- Extract just the task system prompt to `internal/agent/task.go`

**In-Memory Todo Storage**:
- Keep global todo variables in `internal/tools/todo.go`
- No storage abstraction needed for now

**Fewer Interfaces**:
- Just `Tool` interface in `tools/registry.go`
- Keep agent as concrete struct (no interface yet)

### File Breakdown

1. **`cmd/reapo/main.go`** (~50 lines)
   - CLI setup and agent initialization
   - Tool registration
   - Error handling

2. **`internal/agent/agent.go`** (~200 lines)
   - Agent struct and constructor
   - Both `Run()` and `RunTask()` methods
   - Tool execution logic

3. **`internal/tools/registry.go`** (~50 lines)
   - Tool interface definition
   - Tool registry and lookup

4. **`internal/tools/file.go`** (~150 lines)
   - ReadFile, ListFiles, EditFile tools

5. **`internal/tools/todo.go`** (~100 lines)
   - Todo struct and global storage
   - TodoRead, TodoWrite tools

6. **`internal/tools/task.go`** (~50 lines)
   - RunTask tool that spawns TaskAgent sub-agents

7. **`internal/schema/generator.go`** (~20 lines)
   - GenerateSchema function

8. **`internal/agent/task.go`** (~20 lines)
   - Task system prompt constant

### Key Improvements

**Separation of Concerns**:
- Move agent logic to `internal/agent/` 
- Extract tools to `internal/tools/`
- Create schema generation utility in `internal/schema/`

**Interface-Based Design**:
- Define `Tool` interface in `tools/registry.go`
- Add tool registry for easy management

**Better Error Handling**:
- Replace `panic()` calls with proper error handling
- Add structured logging instead of `fmt.Printf`

**Configuration Management**:
- Extract hardcoded values (max tokens, model name) to config
- Add environment variable support

**Testing Support**:
- Make components testable through dependency injection
- Add mock implementations for external dependencies

## Implementation Order

1. Create directory structure
2. Extract schema generation (`internal/schema/generator.go`)
3. Create tool interface and registry (`internal/tools/registry.go`)
4. Split tools into separate files (`internal/tools/{file,todo,task}.go`)
5. Extract agent logic (`internal/agent/agent.go`)
6. Create minimal main file (`cmd/reapo/main.go`)
7. Update imports and test compilation
8. Add tests for individual components

## Benefits

- **Maintainability**: Easier to locate and modify specific functionality
- **Testability**: Components can be tested in isolation
- **Readability**: Smaller, focused files are easier to understand
- **Extensibility**: New tools and features can be added easily
- **Reusability**: Components can be reused in other contexts

This refactoring reduces the main file from 618 lines to ~50 lines while improving maintainability, testability, and extensibility.