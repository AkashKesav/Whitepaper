<<<<<<< HEAD
# Architecture Overview

## System Architecture

The Reflective Memory Kernel is a dual-agent cognitive architecture designed to transform AI memory from reactive retrieval to proactive reasoning.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER LAYER                                      │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                        Chat UI (localhost:3000)                          ││
│  └─────────────────────────────────────────────────────────────────────────┘│
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │ HTTP/WebSocket
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    FRONT-END AGENT ("The Consciousness")                     │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐│
│  │ HTTP Server │ │ WS Handler  │ │ MK Client   │ │ AI Client               ││
│  └─────────────┘ └─────────────┘ └──────┬──────┘ └───────────┬─────────────┘│
│                                         │                     │              │
└─────────────────────────────────────────┼─────────────────────┼──────────────┘
                     ┌────────────────────┘                     │
                     │ Consultation                             │ Generation
                     ▼                                          ▼
┌────────────────────────────────────────┐    ┌────────────────────────────────┐
│      MEMORY KERNEL ("The Subconscious")│    │        AI SERVICES             │
│  ┌──────────────────────────────────┐  │    │  ┌────────────────────────────┐│
│  │         Ingestion Pipeline       │◄─┼────┼──│    Extraction SLM          ││
│  └──────────────────────────────────┘  │    │  └────────────────────────────┘│
│  ┌──────────────────────────────────┐  │    │  ┌────────────────────────────┐│
│  │        Reflection Engine         │──┼────┼─►│    Curation SLM            ││
│  │  ┌────────┐ ┌────────┐          │  │    │  └────────────────────────────┘│
│  │  │Synthesis│ │Curation│          │  │    │  ┌────────────────────────────┐│
│  │  └────────┘ └────────┘          │  │    │  │    Synthesis SLM           ││
│  │  ┌────────┐ ┌────────┐          │  │    │  └────────────────────────────┘│
│  │  │Anticip.│ │Priority│          │  │    │  ┌────────────────────────────┐│
│  │  └────────┘ └────────┘          │  │    │  │       LLM Router           ││
│  └──────────────────────────────────┘  │    │  │  OpenAI│Anthropic│Ollama  ││
│  ┌──────────────────────────────────┐  │    │  └────────────────────────────┘│
│  │       Consultation Handler       │  │    └────────────────────────────────┘
│  └──────────────────────────────────┘  │
└────────────────────┬───────────────────┘
                     │
          ┌──────────┼──────────┐
          │          │          │
          ▼          ▼          ▼
     ┌────────┐ ┌────────┐ ┌────────┐
     │ DGraph │ │  NATS  │ │ Redis  │
     │ (Graph)│ │(Stream)│ │(Cache) │
     └────────┘ └────────┘ └────────┘
```

## Data Flow

### 1. Conversation Flow (Synchronous)

```
User ──► FEA ──► AI Services ──► Response
              │
              └──► MK (Consultation) ──► Context Brief
```

1. User sends message via Chat UI
2. FEA receives message
3. FEA consults Memory Kernel (with 2s timeout)
4. FEA calls AI Services to generate response
5. Response returned to user
6. Transcript streamed to MK asynchronously

### 2. Memory Flow (Asynchronous)

```
Transcript ──► NATS ──► MK Ingestion ──► DGraph
                              │
                              ▼
                        Entity Extraction
                              │
                              ▼
                        Graph Updates
```

### 3. Reflection Flow (Background)

```
Timer (5 min) ──► Reflection Engine
                         │
        ┌────────────────┼────────────────┐
        ▼                ▼                ▼
   Synthesis        Curation        Prioritization
   (Insights)    (Contradictions)    (Decay/Boost)
        │                │                │
        └────────────────┼────────────────┘
                         ▼
                   Graph Updates
```

## Technology Stack

| Layer | Technology | Purpose |
|-------|------------|---------|
| **Frontend** | HTML/JS | Chat interface |
| **FEA** | Go + Gorilla | Low-latency conversation |
| **MK** | Go + gRPC | Persistent background agent |
| **AI Services** | Python + FastAPI | LLM orchestration |
| **Graph DB** | DGraph | Knowledge Graph storage |
| **Message Queue** | NATS JetStream | Async transcript streaming |
| **Cache** | Redis | Hot path caching |
| **Container** | Docker | Service isolation |

## Service Communication

| From | To | Protocol | Purpose |
|------|-----|----------|---------|
| Chat UI | FEA | HTTP/WS | User conversation |
| FEA | MK | HTTP | Consultation queries |
| FEA | AI Services | HTTP | Response generation |
| FEA | NATS | JetStream | Transcript streaming |
| MK | NATS | JetStream | Transcript consumption |
| MK | DGraph | gRPC | Graph operations |
| MK | Redis | TCP | Caching |
| MK | AI Services | HTTP | Entity extraction, synthesis |

## Port Assignments

| Service | Port | Description |
|---------|------|-------------|
| Front-End Agent | 3000 | Chat UI + API |
| Memory Kernel | 9000 | Consultation API |
| AI Services | 8000 | SLM endpoints |
| DGraph Alpha | 8080 | Graph UI |
| DGraph Alpha | 9080 | gRPC |
| DGraph Zero | 5080 | Cluster management |
| NATS | 4222 | Client connections |
| NATS | 8222 | HTTP monitoring |
| Redis | 6379 | Cache |
| Ollama | 11434 | Local LLM |
=======
# System Architecture

## High-Level Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                           User Layer                                 │
│  • Web UI (Chat Interface)                                          │
│  • REST API / WebSocket                                             │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────────────┐
│              Front-End Agent ("The Consciousness")                   │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────────────┐ │
│  │  HTTP Server   │  │ WebSocket Hub  │  │  JWT Authentication   │ │
│  │  (Gorilla Mux) │  │  (Real-time)   │  │  (User & Groups)      │ │
│  └────────────────┘  └────────────────┘  └────────────────────────┘ │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                      Hot Cache (Ring Buffer)                    │ │
│  │  • Per-user message storage with embeddings                     │ │
│  │  • Semantic similarity search                                   │ │
│  │  • O(1) insertion, O(n) search                                  │ │
│  └────────────────────────────────────────────────────────────────┘ │
└──────────┬─────────────────────────────────────────┬────────────────┘
           │                                         │
           │ NATS JetStream                          │ HTTP
           │ (Transcript Events)                     │
           ▼                                         ▼
┌──────────────────────────┐            ┌─────────────────────────────┐
│    Memory Kernel         │            │      AI Services            │
│  ("The Subconscious")    │◄──────────►│      (Python/FastAPI)       │
│                          │   HTTP     │                             │
│  ┌────────────────────┐  │            │  ┌───────────────────────┐  │
│  │ Phase 1: Ingestion │  │            │  │ Entity Extraction     │  │
│  │ - Entity extraction│  │            │  │ Synthesis/Curation    │  │
│  │ - Graph writes     │  │            │  │ Embedding Generation  │  │
│  └────────────────────┘  │            │  │ LLM Router            │  │
│  ┌────────────────────┐  │            │  └───────────────────────┘  │
│  │ Phase 2: Reflection│  │            └─────────────────────────────┘
│  │ - Synthesis        │  │
│  │ - Anticipation     │  │
│  │ - Curation         │  │
│  │ - Prioritization   │  │
│  └────────────────────┘  │
│  ┌────────────────────┐  │
│  │ Phase 3: Consult   │  │
│  │ - Query processing │  │
│  │ - Brief synthesis  │  │
│  └────────────────────┘  │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────────────────┐
│                        DGraph Knowledge Graph                         │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────────┐ │
│  │  User   │ │ Entity  │ │  Fact   │ │ Insight │ │ Pattern / Rule  │ │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘ └────────┬────────┘ │
│       │          │          │          │                 │          │
│       └──────────┴──────────┴──────────┴─────────────────┘          │
│                    Activation-Weighted Edges                         │
└──────────────────────────────────────────────────────────────────────┘
```

## Component Details

### Front-End Agent (`internal/agent/`)

| File | Purpose |
|------|---------|
| `server.go` | HTTP/WebSocket handlers, route setup |
| `agent.go` | Core agent logic, conversation management |
| `jwt_middleware.go` | Authentication and user management |
| `mkclient.go` | Memory Kernel client for HTTP communication |
| `local_client.go` | Local memory kernel integration |
| `interfaces.go` | Interface definitions for dependency injection |

### Memory Kernel (`internal/kernel/`)

| File | Purpose |
|------|---------|
| `kernel.go` | Main kernel orchestrator, lifecycle management |
| `ingestion.go` | Phase 1: Transcript processing and entity extraction |
| `consultation.go` | Phase 3: Query handling and response synthesis |

### Knowledge Graph (`internal/graph/`)

| File | Purpose |
|------|---------|
| `schema.go` | Node/Edge types, data structures |
| `client.go` | DGraph client with CRUD operations |
| `queries.go` | Pre-built DQL query templates |

### Reflection Engine (`internal/reflection/`)

| File | Purpose |
|------|---------|
| `engine.go` | Reflection orchestrator |
| `synthesis.go` | Active Synthesis module |
| `anticipation.go` | Predictive Anticipation module |
| `curation.go` | Self-Curation module |
| `prioritization.go` | Dynamic Prioritization module |

### Memory Management (`internal/memory/`)

| File | Purpose |
|------|---------|
| `hot_cache.go` | In-memory ring buffer with embeddings |
| `batcher.go` | Cold path message batching |

### AI Services (`ai/`)

| File | Purpose |
|------|---------|
| `main.py` | FastAPI application and endpoints |
| `extraction_slm.py` | Entity extraction using LLM |
| `curation_slm.py` | Contradiction resolution |
| `synthesis_slm.py` | Response synthesis |
| `llm_router.py` | Multi-provider LLM abstraction |
| `embedding_service.py` | Vector embedding generation |

## Data Flow

### Chat Request Flow
1. User sends message via HTTP/WebSocket
2. Agent authenticates via JWT
3. Hot Cache searched for relevant context
4. Agent consults Memory Kernel for additional context
5. AI Services generates response
6. Response returned to user
7. Transcript streamed to Memory Kernel via NATS

### Ingestion Flow
1. NATS receives transcript event
2. AI Services extracts entities
3. Entities written to DGraph as nodes
4. Relationships created as edges
5. Activation scores initialized

### Reflection Flow (Periodic)
1. Curation module resolves contradictions
2. Synthesis module discovers insights
3. Anticipation module detects patterns
4. Prioritization module updates activations

## External Dependencies

| Service | Port | Purpose |
|---------|------|---------|
| DGraph Alpha | 9080 | Graph database |
| DGraph Zero | 5080 | DGraph cluster management |
| NATS | 4222 | Message streaming |
| Redis | 6379 | Caching and session storage |
| PostgreSQL | 5432 | User authentication data |
>>>>>>> 5f37bd4 (Major update: API timeout fixes, Vector-Native ingestion, Frontend integration)
