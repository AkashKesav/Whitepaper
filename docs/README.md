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
