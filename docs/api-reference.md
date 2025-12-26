<<<<<<< HEAD
# API Reference

Complete API reference for all Reflective Memory Kernel services.

## Front-End Agent (Port 3000)

### REST API

#### POST /api/chat

Send a message and receive a response.

**Request:**
```http
POST /api/chat HTTP/1.1
Content-Type: application/json

{
    "user_id": "string",           // Required: User identifier
    "conversation_id": "string",   // Optional: Conversation ID (generated if not provided)
    "message": "string"            // Required: User message
}
```

**Response:**
```json
{
    "conversation_id": "conv_abc123",
    "response": "Here's what I know about that...",
    "latency_ms": 234
}
```

**Status Codes:**
| Code | Description |
|------|-------------|
| 200 | Success |
| 400 | Invalid request body |
| 500 | Internal server error |

---

#### GET /api/stats

Get agent statistics.

**Response:**
```json
{
    "active_conversations": 5,
    "total_turns": 42,
    "average_latency_ms": 180
}
```

---

#### GET /health

Health check endpoint.

**Response:**
```json
{
    "status": "healthy"
}
```

---

### WebSocket API

#### WS /ws/chat

Real-time chat connection.

**Connection:**
```
ws://localhost:3000/ws/chat?user_id=user123
```

**Client → Server Messages:**

| Type | Payload | Description |
|------|---------|-------------|
| `chat` | `{"message": "Hello"}` | Send a message |
| `ping` | - | Keep-alive ping |

**Server → Client Messages:**

| Type | Payload | Description |
|------|---------|-------------|
| `response` | `{"response": "..."}` | Chat response |
| `pong` | - | Keep-alive pong |

**Example Session:**
```javascript
const ws = new WebSocket('ws://localhost:3000/ws/chat?user_id=user1');

ws.send(JSON.stringify({
    type: 'chat',
    payload: { message: 'Hello!' }
}));

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    if (data.type === 'response') {
        console.log(data.payload.response);
    }
};
```

---

## Memory Kernel (Port 9000)

### POST /api/consult

Consult the Memory Kernel for context and insights.

**Request:**
```http
POST /api/consult HTTP/1.1
Content-Type: application/json

{
    "user_id": "string",           // Required: User identifier
    "query": "string",             // Required: Query/question
    "context": "string",           // Optional: Additional context
    "max_results": 10,             // Optional: Max facts to return
    "include_insights": true,      // Optional: Include insights
    "topic_filters": ["project"]   // Optional: Filter by topics
}
```

**Response:**
```json
{
    "request_id": "req_xyz789",
    "synthesized_brief": "Based on your history, the user prefers...",
    "relevant_facts": [
        {
            "uid": "0x1",
            "name": "Alex",
            "description": "User's partner",
            "activation": 0.85
        }
    ],
    "insights": [
        {
            "insight_type": "warning",
            "summary": "Peanut allergy risk with Thai food",
            "action_suggestion": "Mention allergy when Thai food discussed"
        }
    ],
    "patterns": [
        {
            "pattern_type": "temporal",
            "predicted_action": "User may need Project Alpha brief"
        }
    ],
    "proactive_alerts": [
        "If Thai food is mentioned, remind about peanut allergy"
    ],
    "confidence": 0.87
}
```

---

### GET /api/stats

Get Memory Kernel statistics.

**Response:**
```json
{
    "Entity_count": 45,
    "Fact_count": 123,
    "Insight_count": 12,
    "Pattern_count": 5,
    "high_activation_nodes": 8,
    "recent_insights": 3,
    "active_patterns": 2
}
```

---

### POST /api/reflect

Manually trigger a reflection cycle (for testing).

**Response:**
```json
{
    "status": "reflection triggered"
}
```

---

### GET /health

Health check endpoint.

**Response:**
```json
{
    "status": "healthy"
}
```

---

## AI Services (Port 8000)

### POST /extract

Extract entities from conversation text.

**Request:**
```json
{
    "user_query": "My partner Alex loves Thai food",
    "ai_response": "That sounds delicious!",
    "context": "Dinner planning conversation"
}
```

**Response:**
```json
[
    {
        "name": "Alex",
        "type": "Entity",
        "attributes": {"role": "partner"},
        "relations": [
            {
                "type": "LIKES",
                "target_name": "Thai Food",
                "target_type": "Entity"
            }
        ]
    }
]
```

---

### POST /curate

Resolve contradictions between facts.

**Request:**
```json
{
    "node1_name": "Manager: Bob",
    "node1_description": "User's manager is Bob",
    "node1_created_at": "2024-01-15T10:00:00Z",
    "node2_name": "Manager: Alice",
    "node2_description": "User's manager is Alice",
    "node2_created_at": "2024-06-20T14:00:00Z"
}
```

**Response:**
```json
{
    "winner_index": 2,
    "reason": "More recent timestamp"
}
```

---

### POST /synthesize

Synthesize a brief from facts and insights.

**Request:**
```json
{
    "query": "Tell me about my projects",
    "context": "Work-related query",
    "facts": [
        {"name": "Project Alpha", "description": "Main project"}
    ],
    "insights": [],
    "proactive_alerts": ["Project deadline approaching"]
}
```

**Response:**
```json
{
    "brief": "You're working on Project Alpha. Note: the deadline is approaching.",
    "confidence": 0.85
}
```

---

### POST /synthesize-insight

Evaluate potential insights between nodes.

**Request:**
```json
{
    "node1_name": "Thai Food",
    "node1_type": "Entity",
    "node1_description": "Cuisine preference",
    "node2_name": "Peanut Allergy",
    "node2_type": "Fact",
    "node2_description": "User is allergic to peanuts",
    "path_exists": false,
    "path_length": 0
}
```

**Response:**
```json
{
    "has_insight": true,
    "insight_type": "warning",
    "summary": "Thai food commonly contains peanuts",
    "action_suggestion": "Warn about peanuts when Thai food is discussed",
    "confidence": 0.91
}
```

---

### POST /generate

Generate a conversational response.

**Request:**
```json
{
    "query": "What should we have for dinner?",
    "context": "Alex loves Thai food. User has peanut allergy.",
    "proactive_alerts": ["Mention peanut risk with Thai food"]
}
```

**Response:**
```json
{
    "response": "How about Thai food since Alex loves it? Just be careful about peanuts!"
}
```

---

### GET /health

Health check.

**Response:**
```json
{
    "status": "healthy"
}
```

---

## Error Responses

All endpoints return errors in this format:

```json
{
    "detail": "Error message describing what went wrong"
}
```

Common HTTP status codes:

| Code | Description |
|------|-------------|
| 400 | Bad Request - Invalid input |
| 404 | Not Found - Resource doesn't exist |
| 500 | Internal Server Error |
| 503 | Service Unavailable - Dependency down |

---

## Rate Limits

Currently no rate limits are enforced. For production deployment, consider:

| Endpoint | Suggested Limit |
|----------|-----------------|
| `/api/chat` | 60 req/min per user |
| `/api/consult` | 120 req/min per user |
| `/extract` | 30 req/min |
=======
# API Reference

## Front-End Agent API (Port 3000)

### Authentication

#### Register User
```http
POST /api/register
Content-Type: application/json

{
  "username": "string",
  "password": "string"
}

Response 201:
{
  "message": "User registered successfully",
  "username": "string"
}
```

#### Login
```http
POST /api/login
Content-Type: application/json

{
  "username": "string",
  "password": "string"
}

Response 200:
{
  "token": "jwt-token-string",
  "username": "string"
}
```

### Chat

#### Send Message
```http
POST /api/chat
Authorization: Bearer {token}
Content-Type: application/json

{
  "user_id": "string",
  "conversation_id": "string (optional)",
  "message": "string"
}

Response 200:
{
  "conversation_id": "string",
  "response": "string",
  "latency_ms": 150
}
```

### WebSocket

#### Connect
```
WS /api/ws/chat?user_id={userId}&conversation_id={convId}
```

#### Message Format
```json
{
  "type": "chat|typing|prefetch",
  "payload": {
    "message": "string",
    "partial_text": "string"
  }
}
```

### Groups

#### Create Group
```http
POST /api/groups
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "string",
  "description": "string"
}

Response 201:
{
  "group_id": "string",
  "name": "string"
}
```

#### List User Groups
```http
GET /api/groups
Authorization: Bearer {token}

Response 200:
[
  {
    "uid": "string",
    "name": "string",
    "description": "string"
  }
]
```

#### Add Group Member
```http
POST /api/groups/{groupId}/members
Authorization: Bearer {token}
Content-Type: application/json

{
  "username": "string"
}
```

### Utility

#### Health Check
```http
GET /api/health

Response 200:
{
  "status": "healthy"
}
```

#### Get Stats
```http
GET /api/stats
Authorization: Bearer {token}

Response 200:
{
  "conversations": 10,
  "total_turns": 150,
  "avg_latency_ms": 120,
  "hot_cache": {
    "total_users": 5,
    "total_messages": 200
  }
}
```

---

## Memory Kernel API (Port 9000)

### Consultation

#### Consult Memory
```http
POST /api/consult
Content-Type: application/json

{
  "user_id": "string",
  "query": "string",
  "context": "string (optional)",
  "conversation_id": "string (optional)",
  "include_insights": true,
  "max_facts": 10
}

Response 200:
{
  "cached": false,
  "synthesized_brief": "string",
  "confidence": 0.85,
  "facts": [...],
  "insights": [...],
  "patterns": [...],
  "proactive_alerts": [...]
}
```

### Ingestion

#### Ingest Transcript
```http
POST /api/ingest
Content-Type: application/json

{
  "user_id": "string",
  "conversation_id": "string",
  "user_query": "string",
  "ai_response": "string",
  "timestamp": "2024-01-01T00:00:00Z"
}

Response 200:
{
  "status": "ingested"
}
```

### Reflection

#### Trigger Reflection
```http
POST /api/reflect

Response 200:
{
  "status": "reflection_triggered"
}
```

### Statistics

#### Get Kernel Stats
```http
GET /api/stats

Response 200:
{
  "ingestion": {
    "total_processed": 500,
    "total_errors": 2,
    "total_entities": 1200
  },
  "reflection": {
    "cycle_count": 100,
    "last_cycle_time": "2024-01-01T12:00:00Z"
  },
  "graph": {
    "total_nodes": 5000,
    "total_edges": 12000
  }
}
```

---

## AI Services API (Port 8000)

### Entity Extraction

#### Extract Entities
```http
POST /extract
Content-Type: application/json

{
  "user_query": "string",
  "ai_response": "string",
  "context": "string (optional)"
}

Response 200:
[
  {
    "name": "string",
    "type": "Entity|Fact|Preference",
    "description": "string",
    "tags": ["tag1", "tag2"],
    "attributes": {},
    "relations": []
  }
]
```

### Curation

#### Resolve Contradiction
```http
POST /curate
Content-Type: application/json

{
  "node1_name": "string",
  "node1_description": "string",
  "node1_created_at": "2024-01-01T00:00:00Z",
  "node2_name": "string",
  "node2_description": "string",
  "node2_created_at": "2024-06-01T00:00:00Z"
}

Response 200:
{
  "winner_index": 2,
  "reason": "More recent information"
}
```

### Synthesis

#### Synthesize Brief
```http
POST /synthesize
Content-Type: application/json

{
  "query": "string",
  "context": "string (optional)",
  "facts": [...],
  "insights": [...],
  "proactive_alerts": [...]
}

Response 200:
{
  "brief": "string",
  "confidence": 0.9
}
```

#### Synthesize Insight
```http
POST /synthesize-insight
Content-Type: application/json

{
  "node1_name": "string",
  "node1_type": "string",
  "node1_description": "string",
  "node2_name": "string",
  "node2_type": "string",
  "node2_description": "string",
  "path_exists": false,
  "path_length": 0
}

Response 200:
{
  "has_insight": true,
  "insight_type": "conflict|connection|pattern",
  "summary": "string",
  "action_suggestion": "string",
  "confidence": 0.8
}
```

### Generation

#### Generate Response
```http
POST /generate
Content-Type: application/json

{
  "query": "string",
  "context": "string (optional)",
  "proactive_alerts": []
}

Response 200:
{
  "response": "string"
}
```

### Embeddings

#### Generate Embedding
```http
POST /embed
Content-Type: application/json

{
  "text": "string"
}

Response 200:
{
  "embedding": [0.1, 0.2, ...]
}
```

#### Semantic Search
```http
POST /semantic-search
Content-Type: application/json

{
  "query": "string",
  "candidates": [
    {"text": "string", "data": {...}}
  ],
  "top_k": 5,
  "threshold": 0.3
}

Response 200:
{
  "results": [
    {"text": "string", "similarity": 0.85, "data": {...}}
  ]
}
```

#### Expand Query
```http
POST /expand-query
Content-Type: application/json

{
  "query": "string"
}

Response 200:
{
  "original_query": "string",
  "search_terms": ["term1", "term2"],
  "entity_names": ["Name1", "Name2"]
}
```

### Health

```http
GET /health

Response 200:
{
  "status": "healthy"
}
```
>>>>>>> 5f37bd4 (Major update: API timeout fixes, Vector-Native ingestion, Frontend integration)
