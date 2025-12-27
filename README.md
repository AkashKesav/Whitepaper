# Reflective Memory Kernel

A transformative AI memory architecture that moves from reactive RAG to proactive Agent-Augmented Generation (AAG).

> 📚 **[Technical Documentation](./docs/README.md)** - Comprehensive docs for architecture, APIs, and deployment.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        User Layer                                │
└─────────────────────────┬───────────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│              Front-End Agent ("The Consciousness")               │
│  • Low-latency conversational interface                          │
│  • Consults Memory Kernel for context                            │
│  • Streams transcripts asynchronously                            │
└──────────────┬────────────────────────────────┬─────────────────┘
               │                                │
    ┌──────────▼──────────┐         ┌──────────▼──────────┐
    │    NATS JetStream   │         │   AI Services       │
    │  (Transcript Stream)│         │   (Python/FastAPI)  │
    └──────────┬──────────┘         └─────────────────────┘
               │
┌──────────────▼──────────────────────────────────────────────────┐
│              Memory Kernel ("The Subconscious")                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Phase 1: Ingestion ─────────────────────────────────────────│ │
│  │ • Receives transcripts from NATS                           │ │
│  │ • Extracts entities via AI                                 │ │
│  │ • Writes to Knowledge Graph                                │ │
│  └────────────────────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Phase 2: Reflection (Async Rumination) ─────────────────────│ │
│  │ • Active Synthesis: Discovers emergent insights            │ │
│  │ • Predictive Anticipation: Detects behavioral patterns     │ │
│  │ • Self-Curation: Resolves contradictions                   │ │
│  │ • Dynamic Prioritization: Activation boost/decay           │ │
│  └────────────────────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Phase 3: Consultation ──────────────────────────────────────│ │
│  │ • Synthesizes pre-computed insights                        │ │
│  │ • Returns coherent briefs, not raw facts                   │ │
│  └────────────────────────────────────────────────────────────┘ │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                   DGraph Knowledge Graph                         │
│  • Nodes: User, Entity, Fact, Insight, Pattern, Rule            │
│  • Edges: Relationships with activation scores                   │
│  • Self-reordering topology based on access patterns            │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.22+ (for local development)
- Python 3.11+ (for AI services development)

### Start the System

```bash
# Clone and start all services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f memory-kernel
```

### Access Points

- **Chat UI**: http://localhost:3000
- **Memory Kernel API**: http://localhost:9000
- **AI Services API**: http://localhost:8000
- **DGraph UI**: http://localhost:8080

## Configuration

Set environment variables in `.env`:

```env
# LLM Providers (optional - uses Ollama by default)
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-...

# Infrastructure (defaults shown)
DGRAPH_URL=dgraph-alpha:9080
NATS_URL=nats://nats:4222
REDIS_URL=redis:6379
QDRANT_URL=http://qdrant:6333
MINIMAX_API_KEY=your_key_here  # Required for Vision features
```

## API Examples

### Chat Endpoint

```bash
curl -X POST http://localhost:3000/api/chat \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user1", "message": "My partner Alex loves Thai food"}'
```

### Consult Memory Kernel

```bash
curl -X POST http://localhost:9000/api/consult \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user1", "query": "What does Alex like?", "include_insights": true}'
```

### Trigger Reflection

```bash
curl -X POST http://localhost:9000/api/reflect
```

## Key Features

### 1. Self-Curation

The system automatically resolves contradictions:

- "My manager is Bob" (January)
- "My manager is Alice" (June)
  → Automatically archives "Bob", keeps "Alice" as current

### 2. Active Synthesis

Discovers hidden connections:

- "Alex loves Thai food" + "I have a peanut allergy"
  → Creates insight: "Thai food may contain peanuts"

### 3. Predictive Anticipation

Learns behavioral patterns:

- Every Monday: User says "Time for Project Alpha review" (negative sentiment)
  → On Monday: Proactively prepares project brief

### 4. Dynamic Prioritization

- High-frequency topics get boosted activation
- Stale memories decay over time
- Core identity traits remain accessible

### 5. Vector-Native Intelligence (New)

- **Pre-Cortex**: Semantic caching layer intercepts queries to prevent redundant LLM calls.
- **Hybrid Retrieval**: Combines Graph traversal (Nodes) with Vector Similarity (Qdrant) for 100% recall.
- **Vision Integration**: Automatically extracts and understands charts/diagrams from PDFs using Minimax.

## Development

### Local Go Development

```bash
# Install dependencies
go mod tidy

# Run Memory Kernel
go run ./cmd/kernel

# Run Front-End Agent
go run ./cmd/agent
```

### Local Python Development

```bash
cd ai
pip install -r requirements.txt
python main.py
```

## License

MIT License
