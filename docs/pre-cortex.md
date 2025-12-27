# Pre-Cortex Cognitive Firewall

The Pre-Cortex layer is a "cognitive firewall" that intercepts requests before they reach the external LLM, reducing costs by up to 90% and improving response latency.

## Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER REQUEST                                    │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         PRE-CORTEX LAYER                                     │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ Step 1: SEMANTIC CACHE                                                │   │
│  │ • Exact match lookup (fastest path)                                   │   │
│  │ • Vector similarity search (0.92+ threshold)                          │   │
│  │ → HIT? Return cached response immediately                             │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                              │ MISS                                          │
│                              ▼                                               │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ Step 2: INTENT CLASSIFICATION                                         │   │
│  │ • Greeting → Handle with deterministic response                       │   │
│  │ • Navigation → Return UI action                                       │   │
│  │ • Fact Retrieval → Try DGraph Reflex                                  │   │
│  │ • Complex → Pass to LLM                                               │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                              │ COMPLEX                                       │
│                              ▼                                               │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ Step 3: DGRAPH REFLEX ENGINE                                          │   │
│  │ • Pattern-based fact extraction                                       │   │
│  │ • Direct DGraph queries                                               │   │
│  │ • Template-based response generation                                  │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                              │ NO MATCH                                      │
│                              ▼                                               │
└─────────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            LLM (External)                                    │
│                    Only reached for complex queries                          │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Why Pre-Cortex?

| Metric               | Without Pre-Cortex | With Pre-Cortex       |
| -------------------- | ------------------ | --------------------- |
| LLM Calls            | 100% of requests   | ~10% of requests      |
| Average Latency      | 2-5 seconds        | <100ms for cached     |
| Cost per 1K requests | ~$10-50            | ~$1-5                 |
| Privacy              | All data to LLM    | Most data stays local |

## Components

### 1. Semantic Cache

The semantic cache stores query-response pairs and retrieves similar past responses using vector embeddings.

**Key Features:**

- Exact match lookup using normalized queries
- Vector similarity search with configurable threshold (default: 0.92)
- Redis persistence for durability
- In-memory index for fast retrieval
- TTL-based expiration (10 minutes default)

**Configuration:**

```go
type SemanticCache struct {
    threshold float64  // Minimum similarity for cache hit (0.0-1.0)
    // Default: 0.92 (high precision to avoid wrong responses)
}
```

**How it works:**

1. **Exact Match**: First tries normalized exact string match (fastest)
2. **Vector Search**: If embedder available, generates query embedding and compares against stored vectors
3. **Store**: After LLM response, stores both exact key and vector for future matches

```go
// Check cache
response, found := cache.Check(ctx, namespace, query)
if found {
    return response // Cache HIT - no LLM needed
}

// After LLM response
cache.Store(ctx, namespace, query, llmResponse)
```

### 2. Intent Classifier

Rule-based classifier that routes requests to the appropriate handler.

**Intent Types:**

| Intent           | Description                           | Handler                |
| ---------------- | ------------------------------------- | ---------------------- |
| `GREETING`       | "Hi", "Hello", "Thanks"               | Deterministic response |
| `NAVIGATION`     | "Go to settings", "Open dashboard"    | UI action              |
| `FACT_RETRIEVAL` | "What is my email?", "List my groups" | DGraph Reflex          |
| `COMPLEX`        | Everything else                       | LLM                    |

**Patterns:**

```go
// Greeting patterns
`^(hi|hello|hey|yo|sup|greetings)[\s!.?]*$`
`^good\s+(morning|afternoon|evening|day)[\s!.?]*$`
`^(bye|goodbye|see\s+you|later|cya|farewell)[\s!.?]*$`

// Navigation patterns
`^(go\s+to|open|show|take\s+me\s+to|navigate\s+to)\s+`
`^(settings|profile|dashboard|home|groups)\s*$`

// Fact retrieval patterns
`^what\s+(is|are)\s+(my|the)`
`^(tell|show)\s+me\s+(my|about)`
`^who\s+(is|are|was)`
`^when\s+(did|was|is)`
```

### 3. DGraph Reflex Engine

Handles simple fact retrieval queries by directly querying DGraph without LLM involvement.

**Supported Queries:**

| Query Pattern                | Query Type         | Example                          |
| ---------------------------- | ------------------ | -------------------------------- |
| "What is my email?"          | `user_email`       | Returns email from User node     |
| "What is my name?"           | `user_name`        | Returns name from User node      |
| "List my groups"             | `user_groups`      | Returns groups user is member of |
| "What do I like?"            | `user_preferences` | Returns stored preferences       |
| "What do you know about me?" | `user_facts`       | Returns all stored facts         |

**Template Responses:**

```go
// Example templates
"Your email is {{.Email}}."
"Your name is {{.Name}}."
"Your groups are: {{range $i, $g := .Groups}}{{if $i}}, {{end}}{{$g.Name}}{{end}}."
```

## Configuration

```go
type Config struct {
    EnableSemanticCache bool    // Enable vector-based caching
    EnableIntentRouter  bool    // Enable intent classification
    EnableDGraphReflex  bool    // Enable direct DGraph queries
    CacheSimilarity     float64 // Minimum similarity for cache hit (0.0-1.0)
}

// Recommended defaults
func DefaultConfig() Config {
    return Config{
        EnableSemanticCache: true,
        EnableIntentRouter:  true,
        EnableDGraphReflex:  true,
        CacheSimilarity:     0.95, // High precision
    }
}
```

## API

### Handle Request

```go
// Process a user request through Pre-Cortex
// Returns (response, handled). If handled is false, pass to LLM.
response, handled := preCortex.Handle(ctx, namespace, userID, query)

if handled {
    return response.Text // Or handle response.Action for navigation
}

// Pass to LLM
llmResponse := callLLM(query)

// Save to cache for future requests
preCortex.SaveToCache(ctx, namespace, query, llmResponse)
```

### Response Types

```go
type Response struct {
    Text    string // Text response (for Greeting, FactRetrieval)
    Action  string // UI action type (for Navigation)
    Target  string // UI target (for Navigation)
    Handled bool   // Whether Pre-Cortex handled the request
}
```

**Navigation Response Example:**

```json
{
  "action": "navigate",
  "target": "settings",
  "handled": true
}
```

### Statistics

```go
// Get Pre-Cortex statistics
total, cached, reflex, llm, hitRate := preCortex.Stats()

// total: Total requests processed
// cached: Requests served from semantic cache
// reflex: Requests handled by reflex engine
// llm: Requests passed to LLM
// hitRate: Percentage of requests handled without LLM
```

## Integration

Pre-Cortex is integrated into the Memory Kernel and Front-End Agent:

```go
// In Front-End Agent handler
func (a *Agent) handleChat(w http.ResponseWriter, r *http.Request) {
    // 1. Try Pre-Cortex first
    response, handled := a.preCortex.Handle(ctx, namespace, userID, query)
    if handled {
        // Respond immediately - no LLM call needed
        json.NewEncoder(w).Encode(response)
        return
    }

    // 2. Consult Memory Kernel for context
    context := a.kernel.Consult(ctx, userID, query)

    // 3. Call LLM with context
    llmResponse := a.aiClient.Generate(ctx, query, context)

    // 4. Save to Pre-Cortex cache
    a.preCortex.SaveToCache(ctx, namespace, query, llmResponse)

    // 5. Stream response to user
    json.NewEncoder(w).Encode(llmResponse)
}
```

## Cost Optimization Strategy

The Pre-Cortex layer optimizes costs through a tiered approach:

```
Tier 1: Exact Cache Match        → FREE (instant)
Tier 2: Vector Similarity Match  → FREE (milliseconds)
Tier 3: Deterministic Response   → FREE (microseconds)
Tier 4: DGraph Fact Query        → FREE (milliseconds)
Tier 5: LLM Call                 → PAID (seconds)
```

By handling Tiers 1-4 locally, only truly complex queries reach the LLM, reducing costs by 80-90% in typical usage patterns.

## See Also

- [Architecture Overview](./architecture.md) - System architecture
- [Memory Kernel](./memory-kernel.md) - The "subconscious" agent
- [AI Services](./ai-services.md) - LLM orchestration
