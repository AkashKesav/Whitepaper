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
