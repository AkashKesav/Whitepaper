<<<<<<< HEAD
# Memory Kernel

The Memory Kernel is the "subconscious" of the system - a persistent, asynchronous agent that builds and maintains the Knowledge Graph through continuous reflection.

## Overview

The Memory Kernel operates independently of user interactions, running 24/7 to:
- Ingest conversation transcripts
- Extract and store entities
- Reflect on stored knowledge
- Answer consultation queries

## The Three-Phase Loop

```
┌─────────────────────────────────────────────────────────────────┐
│                    MEMORY KERNEL                                 │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │   PHASE 1    │───►│   PHASE 2    │───►│   PHASE 3    │      │
│  │  Ingestion   │    │  Reflection  │    │ Consultation │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
│         ▲                   │                    │              │
│         │                   │                    │              │
│         └───────────────────┴────────────────────┘              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Phase 1: Ingestion

The ingestion pipeline receives conversation transcripts and writes them to the Knowledge Graph.

### Data Flow

```
NATS JetStream ──► Ingestion Pipeline ──► AI Extraction ──► DGraph
     │                                         │
     │                                         ▼
     │                                 Entity Extraction
     │                                         │
     └─────────────────────────────────────────┘
                    Async, Non-blocking
```

### Transcript Event Structure

```go
type TranscriptEvent struct {
    ID                string
    UserID            string
    ConversationID    string
    Timestamp         time.Time
    UserQuery         string
    AIResponse        string
    ExtractedEntities []ExtractedEntity
    Sentiment         string
    Topics            []string
}
```

### Processing Steps

1. **Receive**: Subscribe to `transcripts.*` NATS subject
2. **Extract**: Call AI Services `/extract` endpoint
3. **Upsert Nodes**: Create or update entity nodes
4. **Boost Activation**: Increment access count on existing nodes
5. **Create Edges**: Establish relationships
6. **Cache Context**: Store recent context in Redis

### Code Example

```go
func (p *IngestionPipeline) Ingest(ctx context.Context, event *TranscriptEvent) error {
    // Extract entities via AI
    entities, err := p.extractEntities(ctx, event)
    
    // Process each entity
    for _, entity := range entities {
        // Find or create node
        existingNode, _ := p.graphClient.FindNodeByName(ctx, entity.Name, entity.Type)
        
        if existingNode != nil {
            // Boost existing node
            p.graphClient.IncrementAccessCount(ctx, existingNode.UID, config)
        } else {
            // Create new node
            p.graphClient.CreateNode(ctx, &Node{
                Name:       entity.Name,
                Type:       entity.Type,
                Activation: 0.5,
            })
        }
        
        // Create relationships
        for _, relation := range entity.Relations {
            p.processRelation(ctx, nodeUID, relation)
        }
    }
    
    // Cache for fast access
    p.cacheRecentContext(ctx, event)
    return nil
}
```

## Phase 2: Reflection

The reflection engine runs asynchronously on a configurable interval (default: 5 minutes).

See [Reflection Engine](./reflection-engine.md) for detailed documentation.

### Modules

| Module | Purpose | Frequency |
|--------|---------|-----------|
| Synthesis | Discover emergent insights | Every cycle |
| Anticipation | Detect behavioral patterns | Every cycle |
| Curation | Resolve contradictions | Every cycle |
| Prioritization | Update activation scores | Every cycle |
| Decay | Apply activation decay | Hourly |

## Phase 3: Consultation

The consultation handler answers queries from the Front-End Agent with synthesized, pre-computed insights.

### Request/Response

```go
// Request from FEA
type ConsultationRequest struct {
    UserID          string
    Query           string
    Context         string
    MaxResults      int
    IncludeInsights bool
    TopicFilters    []string
}

// Response to FEA
type ConsultationResponse struct {
    RequestID        string
    SynthesizedBrief string    // Pre-computed answer
    RelevantFacts    []Node    // Supporting facts
    Insights         []Insight // Discovered insights
    Patterns         []Pattern // Active patterns
    ProactiveAlerts  []string  // Warnings/suggestions
    Confidence       float64
}
```

### Processing Steps

1. **Cache Check**: Look for cached response in Redis
2. **Graph Search**: Find relevant facts by text similarity
3. **High Activation**: Include core knowledge nodes
4. **Get Insights**: Retrieve relevant synthesized insights
5. **Check Patterns**: Find matching behavioral patterns
6. **Synthesize Brief**: Call AI to create coherent narrative
7. **Cache Response**: Store for 5 minutes
8. **Boost Accessed**: Update activation on retrieved nodes

### RAG vs AAG Comparison

| Aspect | RAG | AAG (Memory Kernel) |
|--------|-----|---------------------|
| Returns | Raw text chunks | Synthesized brief |
| Reasoning | At query time | Pre-computed |
| Contradictions | Returns both | Already resolved |
| Insights | None | Proactive alerts |
| Latency | High | Low |

## Configuration

```go
type Config struct {
    // DGraph
    DGraphAddress string  // "localhost:9080"
    
    // NATS
    NATSAddress string    // "nats://localhost:4222"
    
    // Redis
    RedisAddress string   // "localhost:6379"
    
    // AI Services
    AIServicesURL string  // "http://localhost:8000"
    
    // Reflection
    ReflectionInterval time.Duration  // 5 * time.Minute
    ActivationDecayRate float64       // 0.05
    MinReflectionBatch int            // 10
    MaxReflectionBatch int            // 100
    
    // Ingestion
    IngestionBatchSize int            // 50
    IngestionFlushInterval time.Duration  // 10 * time.Second
}
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/consult` | Consultation query |
| GET | `/api/stats` | Kernel statistics |
| POST | `/api/reflect` | Trigger manual reflection |
| GET | `/health` | Health check |

## Lifecycle

### Startup

```go
func (k *Kernel) Start() error {
    // 1. Connect to DGraph
    k.graphClient = graph.NewClient(ctx, graphCfg)
    
    // 2. Connect to NATS
    k.natsConn = nats.Connect(natsURL)
    k.jetStream = k.natsConn.JetStream()
    
    // 3. Connect to Redis
    k.redisClient = redis.NewClient(redisOpts)
    
    // 4. Initialize reflection engine
    k.reflectionEngine = reflection.NewEngine(reflectionCfg)
    
    // 5. Start background loops
    go k.runIngestionLoop()
    go k.runReflectionLoop()
    go k.runDecayLoop()
    
    return nil
}
```

### Shutdown

```go
func (k *Kernel) Stop() error {
    // Signal goroutines to stop
    k.cancel()
    
    // Wait for completion
    k.wg.Wait()
    
    // Close connections
    k.natsConn.Close()
    k.redisClient.Close()
    k.graphClient.Close()
    
    return nil
}
```
=======
# Memory Kernel

The Memory Kernel is the "subconscious" of the system - a persistent, asynchronous processing engine that manages knowledge ingestion, reflection, and consultation.

## Three-Phase Loop

### Phase 1: Ingestion

**Location**: `internal/kernel/ingestion.go`

Receives transcript events from the Front-End Agent and writes them to the Knowledge Graph.

```go
type IngestionPipeline struct {
    graphClient   *graph.Client
    jetStream     nats.JetStreamContext
    redisClient   *redis.Client
    aiServicesURL string
    batchSize     int
    flushInterval time.Duration
}
```

**Process Flow**:
1. Receive transcript from NATS JetStream
2. Call AI Services `/extract` endpoint to identify entities
3. Batch entity creation for efficiency
4. Write nodes to DGraph with activation scores
5. Create edges representing relationships
6. Cache recent context in Redis

**Key Functions**:
- `Process()` - Processes raw NATS messages
- `Ingest()` - Ingests a single transcript event
- `extractEntities()` - Calls AI for entity extraction
- `processBatchedEntities()` - Batched graph writes

### Phase 2: Reflection

**Location**: `internal/reflection/engine.go`

Runs periodically to process and synthesize stored information.

```go
type Engine struct {
    synthesis      *SynthesisModule
    anticipation   *AnticipationModule
    curation       *CurationModule
    prioritization *PrioritizationModule
}
```

**Modules run in parallel**:
1. **Curation** - Runs first to clean contradictions
2. **Prioritization** - Updates activation scores
3. **Synthesis** - Discovers new insights
4. **Anticipation** - Detects behavioral patterns

### Phase 3: Consultation

**Location**: `internal/kernel/consultation.go`

Handles queries from the Front-End Agent and returns synthesized responses.

```go
type ConsultationHandler struct {
    graphClient   *graph.Client
    queryBuilder  *graph.QueryBuilder
    redisClient   *redis.Client
    hotCache      *memory.HotCache
    aiServicesURL string
}
```

**Process Flow**:
1. Check Redis cache for existing response
2. Search Hot Cache for recent context
3. Query DGraph for relevant facts and insights
4. Call AI Services `/synthesize` for coherent brief
5. Update accessed node activations
6. Cache response in Redis

## Configuration

```go
type Config struct {
    DGraphAddress          string
    NATSAddress            string
    RedisAddress           string
    RedisPassword          string
    RedisDB                int
    AIServicesURL          string
    ReflectionInterval     time.Duration  // Default: 5 minutes
    ActivationDecayRate    float64        // Default: 0.1
    MinReflectionBatch     int            // Default: 5
    MaxReflectionBatch     int            // Default: 50
    IngestionBatchSize     int            // Default: 10
    IngestionFlushInterval time.Duration  // Default: 5 seconds
}
```

## Kernel API

### Core Methods

```go
// Start initializes and starts all kernel components
func (k *Kernel) Start() error

// Stop gracefully shuts down the kernel
func (k *Kernel) Stop() error

// Consult handles consultation requests
func (k *Kernel) Consult(ctx context.Context, req *ConsultationRequest) (*ConsultationResponse, error)

// IngestTranscript manually ingests a transcript
func (k *Kernel) IngestTranscript(ctx context.Context, event *TranscriptEvent) error

// TriggerReflection manually triggers a reflection cycle
func (k *Kernel) TriggerReflection(ctx context.Context) error

// GetStats returns kernel statistics
func (k *Kernel) GetStats(ctx context.Context) (map[string]interface{}, error)
```

### Group Management

```go
// CreateGroup creates a new group
func (k *Kernel) CreateGroup(ctx context.Context, name, description, ownerID string) (string, error)

// ListUserGroups returns groups the user is a member of
func (k *Kernel) ListUserGroups(ctx context.Context, userID string) ([]Group, error)

// AddGroupMember adds a user to a group
func (k *Kernel) AddGroupMember(ctx context.Context, groupID, username string) error

// ShareToGroup shares a conversation with a group
func (k *Kernel) ShareToGroup(ctx context.Context, conversationID, groupID string) error
```

### Hot Cache Access

```go
// HotCache returns the hot cache for direct access
func (k *Kernel) HotCache() *memory.HotCache

// StoreInHotCache stores a message for instant retrieval
func (k *Kernel) StoreInHotCache(userID, query, response, convID string) error
```

## Background Loops

The kernel runs three background loops:

| Loop | Interval | Purpose |
|------|----------|---------|
| Ingestion | Continuous | Process NATS messages |
| Reflection | 5 min | Run reflection modules |
| Decay | 1 hour | Apply activation decay |

## Ingestion Statistics

```go
type IngestionStats struct {
    TotalProcessed    int64
    TotalErrors       int64
    TotalEntities     int64
    LastExtractionMs  int64
    LastDgraphWriteMs int64
    LastProcessedAt   time.Time
}
```
>>>>>>> 5f37bd4 (Major update: API timeout fixes, Vector-Native ingestion, Frontend integration)
