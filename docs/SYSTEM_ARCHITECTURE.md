# Reflective Memory Kernel - Complete System Documentation

## Overview

The **Reflective Memory Kernel (RMK)** is a distributed AI memory system that provides persistent, evolvable memory for AI agents. It decouples computation (LLM inference) from memory (knowledge graph), enabling AI assistants to maintain identity and knowledge across sessions.

### Brain Analogy

| Brain Component | RMK Component | Function |
|-----------------|---------------|----------|
| Consciousness | Agent | Real-time chat, user interaction |
| Hippocampus | Memory Kernel | Memory formation and retrieval |
| Default Mode Network | Reflection Engine | Background consolidation |
| Long-term Memory | DGraph | Persistent knowledge storage |
| Working Memory | Redis | Fast, recent context |

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              MONOLITH CONTAINER                             │
│  ┌─────────────────────┐    ┌─────────────────────┐    ┌─────────────────┐ │
│  │     AGENT           │    │   MEMORY KERNEL     │    │ REFLECTION ENG  │ │
│  │  (Consciousness)    │◄──►│  (Hippocampus)      │◄──►│ (DMN)           │ │
│  │                     │    │                     │    │                 │ │
│  │ • REST/WebSocket    │    │ • Ingestion         │    │ • Curation      │ │
│  │ • JWT Auth          │    │ • Consultation      │    │ • Prioritization│ │
│  │ • Chat Handler      │    │ • Wisdom Layer      │    │ • Anticipation  │ │
│  └─────────┬───────────┘    └──────────┬──────────┘    └────────┬────────┘ │
└────────────┼────────────────────────────┼───────────────────────┼──────────┘
             │ Zero-Copy                  │                       │
             ▼                            ▼                       ▼
┌────────────────────────┐    ┌────────────────────┐    ┌─────────────────────┐
│     AI SERVICES        │    │       DGRAPH       │    │   REDIS / NATS      │
│  (Python + NVIDIA LLM) │    │  (Knowledge Graph) │    │  (Cache + Messages) │
└────────────────────────┘    └────────────────────┘    └─────────────────────┘
```

---

## Core Components

### 1. Agent (Frontend Gateway)

**Location:** `internal/agent/`

The Agent handles all user-facing interactions:

- **REST API** (`/api/*`) - Chat, authentication, groups
- **WebSocket** (`/ws/chat`) - Real-time streaming
- **JWT Authentication** - Stateless user identity

**Key Files:**
- `server.go` - HTTP routing and handlers
- `agent.go` - Core agent logic
- `mkclient.go` - Memory Kernel client (Zero-Copy)

### 2. Memory Kernel

**Location:** `internal/kernel/`

The Memory Kernel handles all memory operations:

- **Ingestion Pipeline** - Stores new memories
- **Consultation Handler** - Retrieves relevant memories
- **Wisdom Layer** - Background entity extraction

**Key Files:**
- `kernel.go` - Core kernel initialization
- `ingestion.go` - Memory storage pipeline
- `consultation.go` - Memory retrieval
- `wisdom/worker.go` - Cold path processing

### 3. Reflection Engine

**Location:** `internal/reflection/`

Background processing that runs every 5 minutes:

- **Curation Module** - Resolves contradicting facts
- **Prioritization Module** - Updates memory importance scores
- **Anticipation Module** - Predicts future queries

**Key Files:**
- `engine.go` - Reflection cycle orchestration
- `curation.go` - Contradiction resolution
- `prioritization.go` - Activation score management
- `anticipation.go` - Query prediction

### 4. AI Services

**Location:** `ai/`

Python FastAPI service with NVIDIA LLM integration:

| Endpoint | Purpose |
|----------|---------|
| `/generate` | Generate chat responses |
| `/extract` | Extract entities from single turn |
| `/summarize_batch` | Batch entity extraction (Wisdom Layer) |
| `/curate` | Resolve contradictions |
| `/synthesize` | Create memory brief |
| `/semantic_search` | Find similar facts |

---

## Data Flow

### Complete Request Lifecycle

```
User Message
     │
     ▼
┌────────────────────────────────────────────────────────────────┐
│ STEP 1: CONSULTATION (Remember)                                │
│                                                                │
│ Agent → Kernel.Consult()                                       │
│   ├─→ Redis: Fetch recent context (Hot Path, ~5ms)            │
│   ├─→ DGraph: Query relevant facts (Warm Path, ~50ms)         │
│   └─→ AI Service: /synthesize → Memory Brief                  │
└────────────────────────────────────────────────────────────────┘
     │
     ▼
┌────────────────────────────────────────────────────────────────┐
│ STEP 2: GENERATION (Think)                                     │
│                                                                │
│ Agent → AI Service: /generate                                  │
│   ├─→ Builds prompt with MEMORY CONTEXT                       │
│   └─→ NVIDIA LLM generates response                           │
└────────────────────────────────────────────────────────────────┘
     │
     ▼
┌────────────────────────────────────────────────────────────────┐
│ STEP 3: INGESTION (Store)                                      │
│                                                                │
│ Agent → Kernel.Ingest()                                        │
│   ├─→ HOT PATH: Cache in Redis (immediate)                    │
│   └─→ COLD PATH: Queue for Wisdom Layer (30s batch)           │
│         ├─→ AI Service: /summarize_batch                      │
│         └─→ DGraph: Store entities                            │
└────────────────────────────────────────────────────────────────┘
     │
     ▼
Response to User
```

---

## Memory Layers

### Hot Path (Redis)
- **Latency:** ~5ms
- **Purpose:** Recent conversation context
- **Duration:** Last 10-20 messages
- **Use Case:** Immediate recall

### Warm Path (DGraph Query)
- **Latency:** ~50ms
- **Purpose:** Structured knowledge retrieval
- **Duration:** Permanent
- **Use Case:** "What's the user's favorite food?"

### Cold Path (Wisdom Layer)
- **Latency:** Background (30-second batches)
- **Purpose:** Entity extraction and summarization
- **Duration:** Permanent + summarized
- **Use Case:** Long-term memory consolidation

---

## Knowledge Graph Schema

### Node Types

```
User        - User identity
Group       - Shared memory space
Fact        - Extracted knowledge
Entity      - Named entity (person, place, thing)
Preference  - User preference
Event       - Conversation event
```

### Key Predicates

```dql
namespace: string @index(exact) .    # Privacy boundary
name: string @index(term, exact) .   # Searchable name
description: string @index(fulltext) . # Full-text search
activation: float @index(float) .    # Priority score (0-1)
created_at: datetime @index(hour) .  # Temporal index
```

### Example Graph Structure

```
┌──────────────────┐
│  USER: alex      │
│  namespace:      │
│   "user_alex"    │
└────────┬─────────┘
         │ likes
         ▼
┌──────────────────┐     ┌──────────────────┐
│  FACT: Chess     │────►│  CONVERSATION    │
│  type: Preference│     │  user_query:     │
│  activation: 0.85│     │   "I love chess" │
└──────────────────┘     └──────────────────┘
```

---

## Namespace Isolation

Every node has a `namespace` field ensuring complete data privacy:

| Namespace | Access |
|-----------|--------|
| `user_alex` | Only Alex can see |
| `group_engineering` | Only group members |

```go
// Query enforces namespace isolation
query := `{
  facts(func: type(Fact)) @filter(eq(namespace, $namespace)) {
    name
    description
  }
}`
```

---

## Reflection Engine Modules

### Curation Module

Resolves contradicting facts:

```
1. Find nodes with conflicting "functional edges"
   (e.g., two different "favorite_food" values)

2. Call /curate AI endpoint to determine winner

3. Create "supersedes" edge from winner to loser
```

### Prioritization Module

Updates activation scores:

```
1. Frequently accessed facts → Higher priority

2. Old unused facts → Lower priority (decay)

3. High activation = Retrieved first in consultation
```

### Anticipation Module

Predicts future queries (Time Travel):

```
1. Analyze user typing patterns

2. Pre-compute likely query results

3. Cache speculation for instant response
```

---

## Docker Services

| Service | Port | Purpose |
|---------|------|---------|
| `rmk-monolith` | 9090 | Agent + Kernel + Reflection |
| `rmk-ai-services` | 8000 | Python AI (NVIDIA LLM) |
| `rmk-dgraph-alpha` | 9080 | DGraph Database |
| `rmk-dgraph-zero` | 5080 | DGraph Coordinator |
| `rmk-redis` | 6379 | Hot Context Cache |
| `rmk-nats` | 4222 | Message Bus |

---

## API Reference

### Chat

```http
POST /api/chat
Authorization: Bearer <jwt_token>

{
  "message": "Hello, I love chess",
  "context_type": "user",      // or "group"
  "context_id": ""             // group_id if context_type=group
}
```

### Authentication

```http
POST /api/register
{ "username": "alex", "password": "secret" }

POST /api/login
{ "username": "alex", "password": "secret" }
→ { "token": "eyJhbG..." }
```

### Groups

```http
GET /api/groups              # List user's groups
POST /api/groups             # Create group
DELETE /api/groups/{id}      # Delete group (admin only)
POST /api/groups/{id}/members    # Add member
DELETE /api/groups/{id}/members/{username}  # Remove member
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DGRAPH_ADDRESS` | `localhost:9080` | DGraph connection |
| `REDIS_ADDRESS` | `localhost:6379` | Redis connection |
| `NATS_URL` | `nats://localhost:4222` | NATS connection |
| `AI_SERVICES_URL` | `http://localhost:8000` | AI service URL |
| `JWT_SECRET` | (required) | JWT signing key |
| `PORT` | `9090` | API server port |

### Kernel Configuration

```go
Config{
    ReflectionInterval:     5 * time.Minute,   // Reflection cycle
    ActivationDecayRate:    0.05,              // Memory decay
    IngestionBatchSize:     50,                // Hot path batch
    IngestionFlushInterval: 10 * time.Second,  // Hot path flush
    WisdomBatchSize:        50,                // Cold path batch
    WisdomFlushInterval:    30 * time.Second,  // Cold path flush
}
```

---

## Running the System

### Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker logs rmk-monolith -f

# Rebuild after code changes
docker-compose up --build --force-recreate -d
```

### Health Check

```bash
curl http://localhost:9090/health
# → {"status": "healthy"}
```

---

## Key Design Decisions

### 1. Zero-Copy Consultation

The Agent and Kernel run in the same process (Monolith). Instead of HTTP calls, they use **direct function calls** for zero-copy memory access.

### 2. Namespace-Based Isolation

Privacy is enforced at the **data layer** (DGraph), not just the application layer. Every query includes namespace filtering.

### 3. Hot/Cold Path Split

Immediate responses use Redis (Hot Path). Background processing extracts entities and stores them in DGraph (Cold Path).

### 4. Activation-Based Retrieval

Nodes have activation scores that decay over time. Frequently accessed memories stay "hot" and are retrieved first.

---

## File Structure

```
/
├── cmd/
│   └── monolith/main.go       # Entry point
├── internal/
│   ├── agent/                 # Frontend gateway
│   ├── kernel/                # Memory operations
│   │   └── wisdom/            # Cold path processing
│   ├── graph/                 # DGraph client
│   └── reflection/            # Background processing
├── ai/
│   ├── main.py               # AI service entry
│   ├── extraction_slm.py     # Entity extraction
│   ├── synthesis_slm.py      # Brief synthesis
│   └── curation_slm.py       # Contradiction resolution
├── static/                    # Frontend files
├── docker-compose.yml         # Service orchestration
└── Dockerfile.monolith        # Go container build
```
