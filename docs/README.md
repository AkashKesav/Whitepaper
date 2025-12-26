<<<<<<< HEAD
# Technical Documentation

Welcome to the Reflective Memory Kernel technical documentation. This documentation covers the architecture, components, APIs, and deployment of the system.

## Documentation Structure

| Document | Description |
|----------|-------------|
| [Architecture Overview](./architecture.md) | System design, dual-agent model, data flow |
| [Knowledge Graph](./knowledge-graph.md) | DGraph schema, node/edge types, constraints |
| [Memory Kernel](./memory-kernel.md) | The "subconscious" - ingestion, reflection, consultation |
| [Reflection Engine](./reflection-engine.md) | The four reflection modules in detail |
| [Front-End Agent](./frontend-agent.md) | The "consciousness" - conversational interface |
| [AI Services](./ai-services.md) | Python SLM orchestration and LLM routing |
| [API Reference](./api-reference.md) | REST API endpoints and WebSocket protocols |
| [Deployment Guide](./deployment.md) | Docker, Kubernetes, configuration |
| [Configuration](./configuration.md) | Environment variables and tuning |

## Quick Links

- [Getting Started](./deployment.md#quick-start)
- [API Endpoints](./api-reference.md)
- [Graph Schema](./knowledge-graph.md#schema)
- [Reflection Algorithms](./reflection-engine.md)

## Core Concepts

### The Dual-Agent Model

The Reflective Memory Kernel implements a cognitive architecture with two agents:

1. **Front-End Agent (FEA)** - "The Consciousness"
   - Lightweight, low-latency conversational interface
   - Optimized for real-time user interaction
   - Consults the Memory Kernel for context

2. **Memory Kernel (MK)** - "The Subconscious"  
   - Persistent, asynchronous background agent
   - Builds and maintains the Knowledge Graph
   - Performs "digital rumination" to discover insights

### The Three-Phase Loop

The Memory Kernel operates on a continuous three-phase loop:

```
Phase 1: Ingestion ──→ Phase 2: Reflection ──→ Phase 3: Consultation
     ↑                                                    │
     └────────────────────────────────────────────────────┘
```

1. **Ingestion**: Receives conversation transcripts, extracts entities, writes to graph
2. **Reflection**: Asynchronous rumination - synthesis, anticipation, curation, prioritization
3. **Consultation**: Answers queries with pre-synthesized insights, not raw facts

### Agent-Augmented Generation (AAG)

Unlike RAG (Retrieval-Augmented Generation) which returns raw text chunks, AAG returns:
- Pre-synthesized briefs
- Proactive alerts
- Pattern-based predictions
- Curated, contradiction-free facts
=======
# Reflective Memory Kernel - Documentation

A transformative AI memory architecture that implements Agent-Augmented Generation (AAG) through a three-phase memory kernel system.

## Quick Navigation

| Document | Description |
|----------|-------------|
| [Architecture](./architecture.md) | System design, components, and data flow |
| [Memory Kernel](./memory-kernel.md) | Three-phase loop: Ingestion, Reflection, Consultation |
| [Knowledge Graph](./knowledge-graph.md) | DGraph schema, node types, and edge relationships |
| [Reflection Engine](./reflection-engine.md) | Four reflection modules for intelligent processing |
| [API Reference](./api-reference.md) | HTTP/WebSocket endpoints and request/response formats |
| [AI Services](./ai-services.md) | Python FastAPI services for LLM operations |
| [Deployment](./deployment.md) | Docker Compose setup and configuration |

## System Overview

The Reflective Memory Kernel consists of two main agents:

### Front-End Agent ("The Consciousness")
- User-facing conversational interface
- Low-latency responses via Hot Path caching
- WebSocket support for real-time streaming
- JWT-based authentication

### Memory Kernel ("The Subconscious")
- Three-phase asynchronous processing loop
- Knowledge graph management via DGraph
- Reflection engine for insight discovery
- NATS JetStream message processing

## Technology Stack

| Component | Technology |
|-----------|------------|
| Core Backend | Go 1.22+ |
| AI Services | Python 3.11+ / FastAPI |
| Knowledge Graph | DGraph |
| Message Queue | NATS JetStream |
| Cache | Redis |
| Embeddings | NVIDIA NIM / Local |

## Getting Started

```bash
# Start all services
docker-compose up -d

# Access points
# Chat UI: http://localhost:3000
# Memory Kernel API: http://localhost:9000
# AI Services: http://localhost:8000
# DGraph UI: http://localhost:8080
```

## Core Concepts

### Hot Path vs Cold Path
- **Hot Path**: In-memory ring buffer with semantic search for instant retrieval
- **Cold Path**: Batched summarization and persistent storage in DGraph

### Activation-Based Prioritization
- Nodes have activation scores that boost/decay based on usage
- High-frequency topics remain easily accessible
- Stale memories decay but are never deleted

### Proactive Intelligence
- Pattern detection for anticipating user needs
- Insight synthesis from disparate facts
- Contradiction resolution for data integrity
>>>>>>> 5f37bd4 (Major update: API timeout fixes, Vector-Native ingestion, Frontend integration)
