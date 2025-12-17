# Front-End Agent

The Front-End Agent is "The Consciousness" - a lightweight, low-latency conversational interface optimized for real-time user interaction.

## Design Philosophy

The FEA follows the principle of **minimal cognitive load**:
- All heavy reasoning is delegated to the Memory Kernel
- The FEA focuses only on conversation management
- Transcript streaming is asynchronous (fire-and-forget)
- MK consultation has aggressive timeouts

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     FRONT-END AGENT                              │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    HTTP/WS Server                            ││
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐││
│  │  │ REST Handler│ │ WS Handler  │ │ Static Files            │││
│  │  │  /api/chat  │ │  /ws/chat   │ │  /static/*              │││
│  │  └──────┬──────┘ └──────┬──────┘ └─────────────────────────┘││
│  └─────────┼───────────────┼────────────────────────────────────┘│
│            └───────┬───────┘                                     │
│                    ▼                                             │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Agent Core                                ││
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐││
│  │  │Conversation │ │  MK Client  │ │     AI Client           │││
│  │  │  Manager    │ │             │ │                         │││
│  │  └─────────────┘ └──────┬──────┘ └───────────┬─────────────┘││
│  └─────────────────────────┼─────────────────────┼──────────────┘│
└────────────────────────────┼─────────────────────┼───────────────┘
                             │                     │
                    ┌────────┴────────┐    ┌───────┴───────┐
                    │ Memory Kernel   │    │  AI Services  │
                    │ :9000           │    │  :8000        │
                    └─────────────────┘    └───────────────┘
```

## Chat Flow

```
User Message ──► FEA receives
                    │
                    ├──► [Parallel, 2s timeout] Consult Memory Kernel
                    │         │
                    │         ▼
                    │    Get context brief, insights, alerts
                    │         │
                    ▼         ▼
            AI Services ◄──── Context
                    │
                    ▼
            Generate Response
                    │
                    ▼
            Return to User
                    │
                    └──► [Async] Stream transcript to NATS
```

## Conversation Management

### Conversation State

```go
type Conversation struct {
    ID        string
    UserID    string
    StartedAt time.Time
    Turns     []Turn
}

type Turn struct {
    Timestamp time.Time
    UserQuery string
    Response  string
    Latency   time.Duration
}
```

### Session Handling

```go
// Get or create conversation
func (a *Agent) getOrCreateConversation(userID, convID string) *Conversation {
    a.convMu.Lock()
    defer a.convMu.Unlock()
    
    if conv, ok := a.conversations[convID]; ok {
        return conv
    }
    
    conv := &Conversation{
        ID:        convID,
        UserID:    userID,
        StartedAt: time.Now(),
    }
    a.conversations[convID] = conv
    return conv
}
```

## Memory Kernel Integration

### Consultation with Timeout

```go
func (a *Agent) Chat(ctx context.Context, userID, convID, message string) (string, error) {
    // Prepare consultation request
    consultReq := &graph.ConsultationRequest{
        UserID:          userID,
        Query:           message,
        MaxResults:      5,
        IncludeInsights: true,
    }
    
    // Non-blocking MK consultation with 2s timeout
    var mkResponse *graph.ConsultationResponse
    mkDone := make(chan struct{})
    
    go func() {
        mkResponse, _ = a.mkClient.Consult(ctx, consultReq)
        close(mkDone)
    }()
    
    select {
    case <-mkDone:
        // Got response
    case <-time.After(2 * time.Second):
        // Timeout - proceed without context
    }
    
    // Generate response with available context
    return a.generateResponse(ctx, message, mkResponse)
}
```

### Async Transcript Streaming

```go
// Fire-and-forget transcript streaming
func (a *Agent) streamTranscript(userID, convID, query, response string) {
    event := &graph.TranscriptEvent{
        UserID:         userID,
        ConversationID: convID,
        Timestamp:      time.Now(),
        UserQuery:      query,
        AIResponse:     response,
    }
    
    // Publish to NATS - don't wait for confirmation
    kernel.PublishTranscript(a.js, event)
}
```

## HTTP API

### POST /api/chat

Chat with the agent.

**Request:**
```json
{
    "user_id": "user123",
    "conversation_id": "conv456",
    "message": "What does Alex like?"
}
```

**Response:**
```json
{
    "conversation_id": "conv456",
    "response": "Based on what I remember, Alex really loves Thai food. By the way, since you have a peanut allergy, you might want to be careful if Alex picks up Thai for dinner.",
    "latency_ms": 234
}
```

### GET /api/stats

Get agent statistics.

**Response:**
```json
{
    "active_conversations": 5,
    "total_turns": 42,
    "average_latency_ms": 180
}
```

### GET /health

Health check.

**Response:**
```json
{
    "status": "healthy"
}
```

## WebSocket API

### Connection

```
ws://localhost:3000/ws/chat?user_id=user123
```

### Message Types

**Chat Message (Client → Server):**
```json
{
    "type": "chat",
    "payload": {
        "message": "Hello!"
    }
}
```

**Response (Server → Client):**
```json
{
    "type": "response",
    "payload": {
        "response": "Hello! How can I help you today?"
    }
}
```

**Ping/Pong:**
```json
{"type": "ping"}
{"type": "pong"}
```

## Configuration

```go
type Config struct {
    NATSAddress      string        // "nats://localhost:4222"
    MemoryKernelURL  string        // "http://localhost:9000"
    AIServicesURL    string        // "http://localhost:8000"
    ResponseTimeout  time.Duration // 10 * time.Second
}
```

## Performance Considerations

### Latency Budget

| Component | Target | Max |
|-----------|--------|-----|
| MK Consultation | 100ms | 2000ms (timeout) |
| AI Generation | 500ms | 5000ms |
| Total Response | 600ms | 7000ms |

### Optimizations

1. **Parallel MK Consultation**: Consultation starts immediately, doesn't block AI generation setup
2. **Aggressive Timeouts**: 2s MK timeout prevents blocking on slow graph queries
3. **Async Transcript Streaming**: User gets response before transcript is stored
4. **Conversation Caching**: Active conversations kept in memory

## Error Handling

```go
func (a *Agent) Chat(ctx context.Context, ...) (string, error) {
    // MK failure: Continue without context
    if mkErr != nil {
        a.logger.Warn("MK consultation failed, proceeding without context")
    }
    
    // AI failure: Return error to user
    response, err := a.aiClient.GenerateResponse(ctx, ...)
    if err != nil {
        return "", fmt.Errorf("failed to generate response: %w", err)
    }
    
    return response, nil
}
```
