# Migration Layer

Go-powered ECL pipeline for transforming raw database dumps into the Reflective Memory Kernel's Knowledge Graph. Inspired by [Cognee](https://github.com/topoteretes/cognee)'s cognitive memory patterns.

## Architecture

```
┌────────────────────────────────────────────────────────────────────────────┐
│                           Migration Pipeline                                │
│                                                                             │
│  ┌──────────────────┐   ┌──────────────────┐   ┌──────────────────┐        │
│  │   📥 EXTRACT     │   │   💭 COGNIFY     │   │   💾 LOAD        │        │
│  │   (Go CLI)       │──▶│   (Python AI)    │──▶│   (Go Kernel)    │        │
│  └──────────────────┘   └──────────────────┘   └──────────────────┘        │
│         │                       │                       │                   │
│         ▼                       ▼                       ▼                   │
│  ┌──────────────┐       ┌──────────────┐       ┌──────────────┐            │
│  │  SQL/JSONL   │       │ Entity NER   │       │   DGraph     │            │
│  │  CSV Reader  │       │ Summaries    │       │   Upsert     │            │
│  └──────────────┘       │ Relations    │       └──────────────┘            │
│                         └──────────────┘                                    │
└────────────────────────────────────────────────────────────────────────────┘
```

## ECL Pipeline Phases

### Phase 1: Extract (Go CLI)

Reads raw data sources and publishes standardized events to NATS.

```go
// cmd/migration/main.go
type IngestionEvent struct {
    SourceID   string                 `json:"source_id"`
    Content    string                 `json:"content"`    // Text for LLM
    RawData    map[string]interface{} `json:"raw_data"`
    Namespace  string                 `json:"namespace"`
    Timestamp  time.Time              `json:"timestamp"`
}
```

**Supported Sources**:
| Format | Description |
|--------|-------------|
| JSONL | Line-delimited JSON records |
| SQL | pg_dump/mysqldump INSERT statements |
| CSV | Comma-separated with headers |

**CLI Usage**:
```bash
go run ./cmd/migration \
  --source=data/users.jsonl \
  --type=jsonl \
  --namespace=import-2024 \
  --batch-size=100
```

### Phase 2: Cognify (Python AI Service)

Processes raw data through LLM to extract structured knowledge.

```python
@app.post("/bulk-extract")
async def bulk_extract_entities(items: list[ExtractionRequest]):
    """Batch extract entities from multiple items."""
    pass

@app.post("/cognify")
async def cognify_content(request: CognifyRequest):
    """
    Cognee-style cognification:
    1. Extract entities (NER)
    2. Generate recursive summaries
    3. Infer relationships between entities
    """
    pass
```

**CognifyRequest**:
```python
class CognifyRequest(BaseModel):
    content: str           # Text to cognify
    namespace: str         # Isolation namespace
    parent_id: str = None  # For hierarchical data
    depth: int = 1         # Recursion depth for summaries
```

**CognifyResponse**:
```python
class CognifyResponse(BaseModel):
    entities: list[ExtractedEntity]
    relationships: list[Relationship]
    summary: str
    child_summaries: list[str] = []
```

### Phase 3: Load (Go Kernel)

Receives processed data and upserts to DGraph.

```go
// internal/kernel/loader.go
func (k *Kernel) handleBulkLoad(msg *nats.Msg) {
    var batch []graph.Node
    json.Unmarshal(msg.Data, &batch)
    
    // Deduplicate against existing nodes
    existing, _ := k.graphClient.GetNodesByNames(ctx, namespace, names)
    
    // Merge or create
    for _, node := range batch {
        if existing[node.Name] != nil {
            k.mergeNode(ctx, existing[node.Name], node)
        } else {
            k.graphClient.CreateNode(ctx, &node)
        }
    }
}
```

## DataPoint Abstraction

Every piece of data is normalized to a `DataPoint` structure:

```go
// internal/migration/datapoint.go
type DataPoint struct {
    SourceID    string                 // Original ID from source
    Content     string                 // Text representation for LLM
    RawData     map[string]interface{} // Original structured data
    Embedding   []float32              // Vector (computed by AI service)
    Namespace   string                 // Isolation namespace
    ParentUID   string                 // For hierarchical data
    ChildUIDs   []string               // Graph links to children
    NodeType    graph.NodeType         // Entity, Fact, Preference, etc.
    Timestamp   time.Time
}
```

This abstraction allows treating SQL rows, PDF paragraphs, and user messages identically.

## NATS Topics

| Topic | Publisher | Subscriber | Purpose |
|-------|-----------|------------|---------|
| `memory.ingest.bulk` | Migration CLI | Processor Worker | Raw data ingestion |
| `memory.cognify` | Processor | AI Service | Entity extraction |
| `memory.load.bulk` | AI Service | Memory Kernel | Graph upserts |

## Configuration

Environment variables for migration:

```bash
# Required
NATS_URL=nats://localhost:4222
DGRAPH_URL=localhost:9080
AI_SERVICES_URL=http://localhost:8000

# Optional
MIGRATION_BATCH_SIZE=100       # Records per batch
MIGRATION_WORKERS=4            # Parallel workers
MIGRATION_NAMESPACE=default    # Default namespace
```

## Running a Migration

### 1. Prepare Data

Convert your source data to JSONL format:

```json
{"id": "user-001", "name": "Alice", "role": "Engineer", "team": "Platform"}
{"id": "user-002", "name": "Bob", "role": "Manager", "reports_to": "user-001"}
```

### 2. Start Infrastructure

```bash
docker-compose up -d dgraph-zero dgraph-alpha nats redis ai-services
```

### 3. Run Migration

```bash
go run ./cmd/migration \
  --source=data/employees.jsonl \
  --type=jsonl \
  --namespace=acme-corp \
  --batch-size=50 \
  --verbose
```

### 4. Verify in DGraph

Query the imported data:

```graphql
{
  imported(func: eq(namespace, "acme-corp")) {
    uid
    name
    dgraph.type
    description
    tags
    knows { name }
    works_at { name }
  }
}
```

## Bulk Loading Large Datasets

For datasets >100K records, use DGraph's native bulk loader:

```bash
# 1. Export cognified data to RDF
go run ./cmd/migration \
  --source=data/large_dump.sql \
  --output=output.rdf \
  --format=rdf

# 2. Use dgraph live for bulk loading
dgraph live -f output.rdf \
  --alpha localhost:9080 \
  --zero localhost:5080 \
  --batch 10000
```

## API Endpoints

New endpoints added to AI Services:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/bulk-extract` | POST | Batch entity extraction |
| `/cognify` | POST | Full cognification pipeline |
| `/cognify/batch` | POST | Batch cognification |

## Error Handling

| Error | Handling |
|-------|----------|
| Duplicate entity | Merge descriptions, union tags |
| Invalid JSON | Skip row, log to error file |
| AI timeout | Retry with exponential backoff |
| DGraph conflict | Use upsert with condition |

## File Structure

```
cmd/
└── migration/
    ├── main.go          # CLI entry point
    ├── reader.go        # Source file readers
    └── publisher.go     # NATS publisher

internal/
└── migration/
    ├── datapoint.go     # DataPoint abstraction
    ├── processor.go     # Batch processor
    └── dedup.go         # Deduplication logic

ai/
├── cognify_service.py   # Cognification logic
└── main.py              # Updated with new endpoints
```

## See Also

- [AI Services](./ai-services.md) — LLM orchestration details
- [Knowledge Graph](./knowledge-graph.md) — DGraph schema and queries
- [Reflection Engine](./reflection-engine.md) — Post-import synthesis
