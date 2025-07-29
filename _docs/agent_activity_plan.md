# Agent Activity Display Plan

## Overview
Enhance the chat interface to show real-time agent activity with progress indicators, status updates, and the ability to update messages in place as the agent processes requests.

## Current State
- Messages are static `components.Message` objects with Role, Content, and IsError fields
- Chat displays completed messages only
- No progress indication during agent processing
- Agent processing is synchronous with no intermediate feedback

## Proposed Architecture

### 1. Enhanced Message Types

```go
type MessageStatus string

const (
    MessagePending    MessageStatus = "pending"    // User message waiting to be processed
    MessageProcessing MessageStatus = "processing" // Agent is working on this message
    MessageCompleted  MessageStatus = "completed"  // Agent finished processing
    MessageError      MessageStatus = "error"      // Processing failed
)

type Message struct {
    ID          string        // Unique identifier for message updates
    Role        string        // "user" or "assistant"
    Content     string        // Message content (can be updated)
    Status      MessageStatus // Current processing status
    IsError     bool          // Legacy error flag
    Timestamp   time.Time     // When message was created
    UpdatedAt   time.Time     // Last update time
    Progress    *Progress     // Optional progress information
}

type Progress struct {
    Description string  // What the agent is currently doing
    Step        int     // Current step number
    TotalSteps  int     // Total number of steps (if known)
    Percentage  float64 // Completion percentage (0-100)
}
```

### 2. Message Update System

#### Update Messages via Commands
```go
type MessageUpdateMsg struct {
    MessageID string
    Content   string
    Status    MessageStatus
    Progress  *Progress
}

// In update.go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case MessageUpdateMsg:
        // Find and update message by ID
        for i, message := range m.messages {
            if message.ID == msg.MessageID {
                m.messages[i].Content = msg.Content
                m.messages[i].Status = msg.Status
                m.messages[i].Progress = msg.Progress
                m.messages[i].UpdatedAt = time.Now()
                break
            }
        }
        return m, nil
    }
}
```

### 3. Spinner/Loading Components

#### Spinner Component
```go
// internal/tui/components/spinner.go
type SpinnerComponent struct {
    frames    []string
    current   int
    message   string
}

func NewSpinnerComponent(message string) SpinnerComponent {
    return SpinnerComponent{
        frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
        current: 0,
        message: message,
    }
}

func (s SpinnerComponent) Render() string {
    frame := s.frames[s.current%len(s.frames)]
    return fmt.Sprintf("%s %s", frame, s.message)
}
```

#### Progress Bar Component
```go
// internal/tui/components/progress.go
type ProgressComponent struct {
    percentage float64
    width      int
    description string
}

func (p ProgressComponent) Render() string {
    filled := int(float64(p.width) * p.percentage / 100)
    bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)
    return fmt.Sprintf("%s [%s] %.1f%%", p.description, bar, p.percentage)
}
```

### 4. Enhanced Chat Message Component

```go
// internal/tui/components/message.go
type MessageComponent struct {
    message Message
    width   int
}

func (mc MessageComponent) Render() string {
    var content strings.Builder
    
    // Role prefix
    if mc.message.Role == "user" {
        content.WriteString("👤 You: ")
    } else {
        content.WriteString("🤖 Agent: ")
    }
    
    // Status indicator
    switch mc.message.Status {
    case MessagePending:
        content.WriteString("⏳ ")
    case MessageProcessing:
        spinner := NewSpinnerComponent("")
        content.WriteString(spinner.Render() + " ")
    case MessageCompleted:
        content.WriteString("✅ ")
    case MessageError:
        content.WriteString("❌ ")
    }
    
    // Main content
    content.WriteString(mc.message.Content)
    
    // Progress information if available
    if mc.message.Progress != nil {
        content.WriteString("\n")
        progress := ProgressComponent{
            percentage:  mc.message.Progress.Percentage,
            width:       40,
            description: mc.message.Progress.Description,
        }
        content.WriteString(progress.Render())
    }
    
    return content.String()
}
```

### 5. Agent Processing Pipeline Updates

#### Modified processMessage Flow
```go
// In update.go
func (m Model) processMessage(message string) tea.Cmd {
    return tea.Batch(
        // Add user message immediately
        func() tea.Msg {
            userMsg := components.Message{
                ID:        generateMessageID(),
                Role:      "user",
                Content:   message,
                Status:    MessageCompleted,
                Timestamp: time.Now(),
            }
            return AddMessageMsg{Message: userMsg}
        },
        
        // Start agent processing
        func() tea.Msg {
            agentMsg := components.Message{
                ID:        generateMessageID(),
                Role:      "assistant",
                Content:   "",
                Status:    MessageProcessing,
                Timestamp: time.Now(),
                Progress: &Progress{
                    Description: "Processing your request...",
                    Percentage:  0,
                },
            }
            return AddMessageMsg{Message: agentMsg}
        },
        
        // Start actual agent work
        m.processAgentRequest(message),
    )
}
```

#### Streaming Agent Responses
```go
func (m Model) processAgentRequest(message string) tea.Cmd {
    return func() tea.Msg {
        // Get the agent message ID (last message added)
        messageID := m.messages[len(m.messages)-1].ID
        
        // Send periodic updates
        updateProgress := func(description string, percentage float64) {
            // Send update command
            return MessageUpdateMsg{
                MessageID: messageID,
                Status:    MessageProcessing,
                Progress: &Progress{
                    Description: description,
                    Percentage:  percentage,
                },
            }
        }
        
        // Simulate agent work with progress updates
        // Phase 1: Expanding file references
        updateProgress("Expanding file references...", 10)
        expandedMessage := m.expandFileReferences(message)
        
        // Phase 2: Building conversation
        updateProgress("Building conversation history...", 30)
        conversation := m.buildConversationHistory()
        
        // Phase 3: Sending to Claude
        updateProgress("Sending request to Claude...", 50)
        response, err := m.agent.RunInference(ctx, conversation)
        
        // Phase 4: Processing response
        updateProgress("Processing response...", 80)
        
        if err != nil {
            return MessageUpdateMsg{
                MessageID: messageID,
                Content:   fmt.Sprintf("Error: %s", err.Error()),
                Status:    MessageError,
                Progress:  nil,
            }
        }
        
        // Phase 5: Complete
        return MessageUpdateMsg{
            MessageID: messageID,
            Content:   extractResponseText(response),
            Status:    MessageCompleted,
            Progress:  nil,
        }
    }
}
```

### 6. Animation System

#### Spinner Animation
```go
// Add to Model
type Model struct {
    // ... existing fields
    animationTicker *time.Ticker
}

// Animation command
func (m Model) startAnimation() tea.Cmd {
    return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
        return AnimationTickMsg{}
    })
}

// Handle animation in Update
case AnimationTickMsg:
    // Update spinner frames for all processing messages
    hasProcessing := false
    for i := range m.messages {
        if m.messages[i].Status == MessageProcessing {
            hasProcessing = true
            // Update spinner state
        }
    }
    
    if hasProcessing {
        return m, m.startAnimation() // Continue animation
    }
    return m, nil // Stop animation when no processing messages
```

### 7. Integration Points

#### Update Model Structure
```go
// In model.go
type Model struct {
    // ... existing fields
    messageCounter int // For generating unique message IDs
}

func (m Model) generateMessageID() string {
    m.messageCounter++
    return fmt.Sprintf("msg_%d_%d", time.Now().Unix(), m.messageCounter)
}
```

#### Modified Message Types
```go
type AddMessageMsg struct {
    Message components.Message
}

type AnimationTickMsg struct{}
```

## Implementation Steps

1. **Phase 1: Core Infrastructure**
   - Update Message struct with Status, ID, Progress fields
   - Create spinner and progress bar components
   - Implement message update system

2. **Phase 2: Agent Integration** 
   - Modify processMessage to add immediate user message
   - Add agent processing message with spinner
   - Implement progress updates in agent pipeline

3. **Phase 3: Animation System**
   - Add animation ticker for spinners
   - Update chat component to render dynamic states
   - Handle animation lifecycle

4. **Phase 4: Polish & Testing**
   - Test message updates and progress tracking
   - Optimize animation performance
   - Add error handling and edge cases

## Benefits

- **Real-time Feedback**: Users see what the agent is doing
- **Progress Indication**: Visual feedback on long-running operations  
- **Better UX**: Eliminates "dead time" waiting for responses
- **Debugging**: Clear visibility into agent processing steps
- **Extensible**: Framework supports future enhancements like streaming responses

## Future Enhancements

- Streaming response chunks as they arrive
- Tool execution progress for multi-step operations
- Cancellable requests with progress tracking
- Message editing and re-processing
- Export/save conversation with timestamps