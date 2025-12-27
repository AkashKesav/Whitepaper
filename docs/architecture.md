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
│  │ HTTP Server │ │ WS Handler  │ │ MK Client   │ │ Pre-Cortex             ││
│  └─────────────┘ └─────────────┘ └──────┬──────┘ └───────────┬─────────────┘│
│                                         │          Cognitive  │              │
│                                         │          Firewall   │              │
└─────────────────────────────────────────┼─────────────────────┼──────────────┘
                      ┌───────────────────┘                     │
                      │ Consultation           ┌────────────────┘
                      ▼                        │ Pass-through (Complex only)
┌────────────────────────────────────────┐    ▼
│      MEMORY KERNEL ("The Subconscious")│    ┌────────────────────────────────┐
│  ┌──────────────────────────────────┐  │    │    PRE-CORTEX COMPONENTS       │
│  │         Ingestion Pipeline       │◄─┤    │  ┌────────────────────────────┐│
│  └──────────────────────────────────┘  │    │  │    Semantic Cache          ││
│  ┌──────────────────────────────────┐  │    │  │   (Vector Similarity)      ││
│  │        Reflection Engine         │  │    │  └────────────────────────────┘│
│  │  ┌────────┐ ┌────────┐          │  │    │  ┌────────────────────────────┐│
│  │  │Synthesis│ │Curation│          │  │    │  │    Intent Classifier       ││
│  │  └────────┘ └────────┘          │  │    │  │  (Greeting/Nav/Fact/LLM)   ││
│  │  ┌────────┐ ┌────────┐          │  │    │  └────────────────────────────┘│
│  │  │Anticip.│ │Priority│          │  │    │  ┌────────────────────────────┐│
│  │  └────────┘ └────────┘          │  │    │  │    DGraph Reflex Engine    ││
│  └──────────────────────────────────┘  │    │  │  (Direct Graph Queries)   ││
│  ┌──────────────────────────────────┐  │    │  └────────────────────────────┘│
│  │       Consultation Handler       │  │    └─────────────────┬──────────────┘
│  └──────────────────────────────────┘  │                      │ LLM Required
└────────────────────┬───────────────────┘                      ▼
                     │                       ┌────────────────────────────────┐
          ┌──────────┼──────────┐            │        AI SERVICES             │
          │          │          │            │  ┌────────────────────────────┐│
          ▼          ▼          ▼            │  │       LLM Router           ││
     ┌────────┐ ┌────────┐ ┌────────┐        │  │  OpenAI│Anthropic│Ollama  ││
     │ DGraph │ │  NATS  │ │ Redis  │        │  └────────────────────────────┘│
     │ (Graph)│ │(Stream)│ │(Cache) │        │  ┌────────────────────────────┐│
     └────────┘ └────────┘ └────────┘        │  │   Extraction│Synthesis     ││
                                             │  │   Document Ingestion       ││
                                             │  └────────────────────────────┘│
                                             └────────────────────────────────┘
```

## Pre-Cortex Layer

The Pre-Cortex is a **cognitive firewall** that intercepts requests before they reach the LLM:

1. **Semantic Cache**: Returns cached responses for similar past queries (90%+ similarity)
2. **Intent Classifier**: Routes greetings, navigation, and fact queries to deterministic handlers
3. **DGraph Reflex**: Answers simple fact queries directly from the Knowledge Graph

**Result**: Only complex queries reach the LLM, reducing costs by up to 90%.

See [Pre-Cortex Documentation](./pre-cortex.md) for details.

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

| Layer             | Technology       | Purpose                     |
| ----------------- | ---------------- | --------------------------- |
| **Frontend**      | HTML/JS          | Chat interface              |
| **FEA**           | Go + Gorilla     | Low-latency conversation    |
| **MK**            | Go + gRPC        | Persistent background agent |
| **AI Services**   | Python + FastAPI | LLM orchestration           |
| **Graph DB**      | DGraph           | Knowledge Graph storage     |
| **Message Queue** | NATS JetStream   | Async transcript streaming  |
| **Cache**         | Redis            | Hot path caching            |
| **Container**     | Docker           | Service isolation           |

## Service Communication

| From    | To          | Protocol  | Purpose                      |
| ------- | ----------- | --------- | ---------------------------- |
| Chat UI | FEA         | HTTP/WS   | User conversation            |
| FEA     | MK          | HTTP      | Consultation queries         |
| FEA     | AI Services | HTTP      | Response generation          |
| FEA     | NATS        | JetStream | Transcript streaming         |
| MK      | NATS        | JetStream | Transcript consumption       |
| MK      | DGraph      | gRPC      | Graph operations             |
| MK      | Redis       | TCP       | Caching                      |
| MK      | AI Services | HTTP      | Entity extraction, synthesis |

## Port Assignments

| Service         | Port  | Description        |
| --------------- | ----- | ------------------ |
| Front-End Agent | 3000  | Chat UI + API      |
| Memory Kernel   | 9000  | Consultation API   |
| AI Services     | 8000  | SLM endpoints      |
| DGraph Alpha    | 8080  | Graph UI           |
| DGraph Alpha    | 9080  | gRPC               |
| DGraph Zero     | 5080  | Cluster management |
| NATS            | 4222  | Client connections |
| NATS            | 8222  | HTTP monitoring    |
| Redis           | 6379  | Cache              |
| Ollama          | 11434 | Local LLM          |
