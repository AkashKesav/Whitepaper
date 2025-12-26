# Hybrid Memory Architecture

Unified "Atomic & Holistic" memory system combining **Cognee** (structured fact extraction) with **Microsoft GraphRAG** (community-based insights) for the Reflective Memory Kernel.

## Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        HYBRID MEMORY ARCHITECTURE                            │
│                                                                              │
│                           ┌─────────────┐                                    │
│                           │ Data Router │                                    │
│                           └──────┬──────┘                                    │
│                    ┌─────────────┴─────────────┐                             │
│                    ▼                           ▼                             │
│         ┌──────────────────┐        ┌──────────────────┐                     │
│         │   TRACK A        │        │   TRACK B        │                     │
│         │   (Cognee)       │        │   (GraphRAG)     │                     │
│         │                  │        │                  │                     │
│         │  📊 Structured   │        │  📄 Unstructured │                     │
│         │  SQL, JSON, APIs │        │  PDFs, Logs, Docs│                     │
│         │                  │        │                  │                     │
│         │  🎯 Atomic Facts │        │  🔮 Holistic     │                     │
│         │  Exact Recall    │        │  Pattern Insight │                     │
│         └────────┬─────────┘        └────────┬─────────┘                     │
│                  │                           │                               │
│                  └───────────┬───────────────┘                               │
│                              ▼                                               │
│                    ┌──────────────────┐                                      │
│                    │     DGraph       │                                      │
│                    │  Knowledge Graph │                                      │
│                    └──────────────────┘                                      │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Track Comparison

| Feature | Track A: Cognee | Track B: GraphRAG |
|---------|-----------------|-------------------|
| **Input Data** | SQL Dumps, JSON, APIs | PDFs, Logs, Long-form Text |
| **Goal** | Exact Fact Reconstruction | Thematic Understanding |
| **Key Unit** | Entity (Node) | Insight (Cluster Summary) |
| **Technique** | 1:1 Mapping, Relation Extraction | Community Detection (Leiden) |
| **Query Type** | "What is X's email?" | "What are common complaints?" |

## DGraph Schema

```graphql
# --- Track A: Atomic Facts (Cognee) ---
type Entity {
    uid: string
    name: string @index(term)
    type: string @index(exact)           # "Person", "Order", "ErrorLog"
    description: string @index(fulltext)
    embedding: float32vector @index(hnsw)
    namespace: string @index(exact)
    activation: float
    
    # Relationships
    knows: [Entity]
    related_to: [Entity]
    works_at: [Entity]
    
    # Track B Integration
    member_of_community: int @index(int)
}

# --- Track B: Global Insights (GraphRAG) ---
type Insight {
    uid: string
    summary: string @index(fulltext)     # Community report
    level: int                           # 0=Root, 1=Sub-community
    community_id: int @index(int)
    confidence: float
    
    # Links Insights to Entity Groups
    describes: [Entity] @reverse
}
```

## Pipeline Phases

### Phase 1: Ingestion Router (Go)

Routes incoming data to the appropriate processing track.

```go
// internal/ingest/router.go
func (r *Router) Route(data DataPacket) error {
    switch data.Type {
    case "STRUCTURED_SQL", "STRUCTURED_JSON":
        return r.nats.Publish("memory.process.facts", data)
    case "UNSTRUCTURED_TEXT", "UNSTRUCTURED_PDF":
        return r.nats.Publish("memory.process.graphrag", data)
    default:
        // Auto-detect based on content analysis
        return r.autoRoute(data)
    }
}
```

### Phase 2A: Cognee Worker (Python)

Processes structured data into precise graph nodes.

**Trigger**: `memory.process.facts`

```python
@app.post("/process-facts")
async def process_facts(data: FactRequest):
    """
    Track A: Structured Data → Entity Nodes
    1. Parse JSON/SQL row
    2. Map fields to Entity attributes
    3. Generate embeddings
    4. Return RDF N-Quads
    """
    entities = []
    for row in data.rows:
        entity = ExtractedEntity(
            name=row.get("name"),
            type=infer_type(row),
            description=format_description(row),
            embedding=await embed(row),
        )
        entities.append(entity)
    return {"entities": entities, "format": "nquads"}
```

### Phase 2B: GraphRAG Workflow (3 Steps)

#### Step B1: Entity Extraction

**Trigger**: `memory.process.graphrag`

```python
@app.post("/extract-graphrag")
async def extract_graphrag(request: GraphRAGRequest):
    """
    Extract entities and relations from unstructured text.
    Initial insertion WITHOUT community IDs.
    """
    chunks = chunk_text(request.content, size=512)
    entities = []
    relations = []
    
    for chunk in chunks:
        result = await llm.extract_entities_and_relations(chunk)
        entities.extend(result.entities)
        relations.extend(result.relations)
    
    return {"entities": entities, "relations": relations}
```

#### Step B2: Community Detection (Leiden)

Periodic job triggered by Go Kernel (cron: every 10 min or 1K new nodes).

**Go Kernel** exports lightweight graph:
```go
func (k *Kernel) TriggerClustering(ctx context.Context) error {
    // Export nodes and edges
    graph, _ := k.graphClient.ExportLightweightGraph(ctx)
    
    // Send to Python clustering worker
    return k.nats.Publish("memory.calc.communities", graph)
}
```

**Python Math Worker** runs Leiden algorithm:
```python
# workers/clustering.py
import leidenalg
import igraph

@app.post("/cluster")
async def cluster_graph(request: ClusterRequest):
    """Run Leiden community detection algorithm."""
    g = igraph.Graph(directed=True)
    g.add_vertices(request.nodes)
    g.add_edges(request.edges)
    
    # Leiden algorithm for community detection
    partition = leidenalg.find_partition(
        g, 
        leidenalg.ModularityVertexPartition,
        resolution_parameter=1.0
    )
    
    # Map nodes to community IDs
    communities = {}
    for idx, community in enumerate(partition):
        for node in community:
            communities[g.vs[node]["name"]] = idx
    
    return {"communities": communities, "total": len(partition)}
```

**Go Kernel** writes back community IDs:
```go
func (k *Kernel) ApplyCommunities(result ClusterResult) error {
    for uid, communityID := range result.Communities {
        k.graphClient.UpdateCommunity(ctx, uid, communityID)
    }
    return nil
}
```

#### Step B3: Community Summarization

Generate Insight nodes from community members.

```go
func (k *Kernel) SummarizeCommunity(ctx context.Context, communityID int) error {
    // 1. Get all nodes in this community
    nodes, _ := k.graphClient.GetCommunityMembers(ctx, communityID)
    
    // 2. Build context from node descriptions
    context := buildContext(nodes)
    
    // 3. Ask LLM to summarize
    summary, _ := k.aiClient.SummarizeCommunity(context)
    
    // 4. Create Insight node
    insight := &graph.Insight{
        Summary:     summary,
        CommunityID: communityID,
        Level:       0,
        Confidence:  0.85,
    }
    
    // 5. Link Insight to all community members
    return k.graphClient.CreateInsightWithLinks(ctx, insight, nodes)
}
```

## Hybrid Query System

### Query Type 1: Atomic Recall (Cognee)

```graphql
# "What is the IP address of server DB-01?"
{
  server(func: eq(name, "DB-01")) @filter(eq(type, "Server")) {
    name
    ip_address
    description
  }
}
```

### Query Type 2: Holistic Recall (GraphRAG)

```graphql
# "How is our infrastructure stability this week?"
{
  insights(func: has(summary)) @filter(gt(confidence, 0.7)) {
    summary
    community_id
    describes {
      name
      type
    }
  }
}
```

### Query Type 3: Hybrid (Best of Both)

```go
func (k *Kernel) HybridQuery(ctx context.Context, query string) (*HybridResult, error) {
    // 1. Vector search on Entities (Cognee)
    entities, _ := k.searchEntities(ctx, query, 10)
    
    // 2. Vector search on Insights (GraphRAG)
    insights, _ := k.searchInsights(ctx, query, 5)
    
    // 3. Combine and rank
    return &HybridResult{
        Facts:    entities,  // Precise answers
        Context:  insights,  // Thematic understanding
    }, nil
}
```

## NATS Topics

| Topic | Publisher | Subscriber | Purpose |
|-------|-----------|------------|---------|
| `memory.process.facts` | Router | Cognee Worker | Track A ingestion |
| `memory.process.graphrag` | Router | GraphRAG Worker | Track B ingestion |
| `memory.calc.communities` | Kernel (cron) | Clustering Worker | Leiden algorithm |
| `memory.summarize.community` | Kernel | AI Service | Generate insights |

## Configuration

```bash
# Hybrid Architecture Settings
HYBRID_MODE=true
COGNEE_ENABLED=true
GRAPHRAG_ENABLED=true

# Clustering Settings
LEIDEN_RESOLUTION=1.0
CLUSTER_INTERVAL=10m          # Re-cluster every 10 minutes
CLUSTER_MIN_NODES=100         # Minimum nodes before clustering

# Community Summarization
SUMMARIZE_TOP_N=50            # Max nodes per community summary
INSIGHT_MIN_CONFIDENCE=0.7
```

## File Structure

```
internal/
├── ingest/
│   └── router.go           # Data routing logic
├── clustering/
│   ├── trigger.go          # Cron job for clustering
│   └── apply.go            # Write back community IDs
└── hybrid/
    └── query.go            # Hybrid query engine

ai/
├── clustering_worker.py    # Leiden algorithm
├── graphrag_extractor.py   # Unstructured extraction
└── community_summarizer.py # Insight generation
```

## Example: Full Pipeline Flow

```
1. PDF Log Upload → Router detects "UNSTRUCTURED_TEXT"
       ↓
2. GraphRAG Extractor → Entities: [Server-01, Error-503, User-Alice]
       ↓
3. DGraph Upsert → Nodes stored (no community yet)
       ↓
4. [10 min later] Leiden Clustering → Community 7 assigned
       ↓
5. Summarizer → "Community 7 shows 503 errors affecting User-Alice on Server-01"
       ↓
6. Insight Node Created → Linked to all 3 entities
       ↓
7. Query: "What's wrong with Alice's access?"
   → Returns: Insight summary + specific entity facts
```

## See Also

- [Migration Layer](./migration-layer.md) — ECL pipeline for structured data
- [Reflection Engine](./reflection-engine.md) — Active synthesis module
- [Knowledge Graph](./knowledge-graph.md) — DGraph schema details
- [AI Services](./ai-services.md) — LLM orchestration
