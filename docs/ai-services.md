# AI Services

The AI Services layer provides SLM (Small Language Model) orchestration for entity extraction, curation, synthesis, and response generation.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      AI SERVICES (FastAPI)                       │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                      LLM Router                              ││
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐││
│  │  │   OpenAI    │ │  Anthropic  │ │        Ollama           │││
│  │  │  GPT-4/3.5  │ │   Claude    │ │    (Local Models)       │││
│  │  └─────────────┘ └─────────────┘ └─────────────────────────┘││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────────┐ │
│  │ Extraction   │ │  Curation    │ │       Synthesis          │ │
│  │     SLM      │ │     SLM      │ │          SLM             │ │
│  └──────────────┘ └──────────────┘ └──────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## LLM Router

The router intelligently selects the best available LLM provider.

### Provider Priority

1. **OpenAI** (if API key set) - Best quality
2. **Anthropic** (if API key set) - Good for analysis
3. **Ollama** (always available) - Local, free, private

### Configuration

```python
class LLMRouter:
    def __init__(self):
        self.openai_key = os.getenv("OPENAI_API_KEY")
        self.anthropic_key = os.getenv("ANTHROPIC_API_KEY")
        self.ollama_host = os.getenv("OLLAMA_HOST", "http://localhost:11434")

        # Determine available providers
        self.providers = []
        if self.openai_key:
            self.providers.append("openai")
        if self.anthropic_key:
            self.providers.append("anthropic")
        self.providers.append("ollama")

        self.default_provider = self.providers[0]
```

### Usage

```python
# Basic generation
response = await router.generate(
    query="What is the capital of France?",
    context="User is learning geography",
    alerts=["User prefers concise answers"]
)

# With specific provider
response = await router.generate(
    query="Analyze this text",
    provider="anthropic",
    model="claude-3-haiku-20240307"
)

# JSON extraction
result = await router.extract_json(prompt)
```

## Extraction SLM

Extracts structured entities and relationships from conversation text.

### Endpoint

**POST /extract**

### Request

```json
{
  "user_query": "My partner Alex loves Thai food",
  "ai_response": "That's great! Thai cuisine has wonderful flavors.",
  "context": "Previous conversation about dinner plans"
}
```

### Response

```json
[
  {
    "name": "Alex",
    "type": "Entity",
    "attributes": { "role": "partner" },
    "relations": [
      {
        "type": "LIKES",
        "target_name": "Thai Food",
        "target_type": "Entity"
      }
    ]
  },
  {
    "name": "Thai Food",
    "type": "Entity",
    "attributes": { "category": "cuisine" },
    "relations": []
  }
]
```

### Prompt Template

```
Analyze this conversation and extract structured entities.

User said: "{user_query}"
AI responded: "{ai_response}"

Extract entities in this JSON format:
[
  {
    "name": "entity name",
    "type": "Entity|Fact|Event|Preference",
    "attributes": {"key": "value"},
    "relations": [
      {
        "type": "RELATION_TYPE",
        "target_name": "related entity",
        "target_type": "Entity|Fact"
      }
    ]
  }
]

Relation types: PARTNER_IS, FAMILY_MEMBER, HAS_MANAGER, WORKS_ON,
                LIKES, DISLIKES, IS_ALLERGIC_TO, PREFERS, HAS_INTEREST

Return ONLY the JSON array, no explanation.
```

## Curation SLM

Resolves contradictions between conflicting facts.

### Endpoint

**POST /curate**

### Request

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

### Response

```json
{
  "winner_index": 2,
  "reason": "More recent information supersedes older data"
}
```

### Prompt Template

```
You are a fact verification expert. Two facts contradict each other.

Fact 1:
- Name: {node1_name}
- Description: {node1_description}
- Created: {node1_created_at}

Fact 2:
- Name: {node2_name}
- Description: {node2_description}
- Created: {node2_created_at}

Determine which fact should be kept. Consider:
1. More recent information usually supersedes older
2. More specific information is more reliable
3. Direct statements override implications

Return JSON: {"winner_index": 1 or 2, "reason": "brief explanation"}
```

## Synthesis SLM

Creates coherent briefs from facts and insights.

### Endpoint

**POST /synthesize**

### Request

```json
{
  "query": "What should I know about Alex?",
  "context": "Planning dinner",
  "facts": [
    { "name": "Alex", "description": "User's partner" },
    { "name": "Thai Food", "description": "Alex's favorite cuisine" }
  ],
  "insights": [{ "summary": "Thai food may contain peanuts - allergy risk" }],
  "proactive_alerts": ["User has peanut allergy"]
}
```

### Response

```json
{
  "brief": "Alex is your partner who loves Thai food. Important note: since you have a peanut allergy, be careful when ordering Thai cuisine as it often contains peanuts.",
  "confidence": 0.92
}
```

### Insight Evaluation

**POST /synthesize-insight**

Evaluates if two nodes have an emergent connection.

```json
{
  "node1_name": "Thai Food",
  "node1_type": "Entity",
  "node2_name": "Peanuts",
  "node2_type": "Entity",
  "path_exists": false,
  "path_length": 0
}
```

Response:

```json
{
  "has_insight": true,
  "insight_type": "warning",
  "summary": "Thai cuisine commonly contains peanuts",
  "action_suggestion": "Warn user about peanut content when Thai food is mentioned",
  "confidence": 0.89
}
```

## Response Generation

**POST /generate**

Generates conversational responses for the Front-End Agent.

### Request

```json
{
  "query": "What's for dinner?",
  "context": "Alex likes Thai food. User has peanut allergy.",
  "proactive_alerts": ["If Thai food is suggested, mention peanut allergy risk"]
}
```

### Response

```json
{
  "response": "How about Thai food? Alex loves it! Just remember to check for peanuts since you're allergic."
}
```

## API Reference

| Endpoint               | Method | Description                                |
| ---------------------- | ------ | ------------------------------------------ |
| `/extract`             | POST   | Extract entities from text                 |
| `/curate`              | POST   | Resolve contradictions                     |
| `/synthesize`          | POST   | Create coherent brief                      |
| `/synthesize-insight`  | POST   | Evaluate potential insight                 |
| `/generate`            | POST   | Generate response                          |
| `/cognify-batch`       | POST   | Batch entity extraction for migration      |
| `/summarize_batch`     | POST   | Wisdom layer conversation crystallization  |
| `/summarize-community` | POST   | Layer 2 community summarization            |
| `/summarize-global`    | POST   | Layer 3 global overview                    |
| `/expand-query`        | POST   | Extract entity names and search terms      |
| `/extract-vision`      | POST   | Vision-based entity extraction from images |
| `/ingest-document`     | POST   | Tiered document ingestion                  |
| `/ingest-vector-tree`  | POST   | Vector-native document ingestion           |
| `/embed`               | POST   | Generate embedding vector                  |
| `/semantic-search`     | POST   | Semantic similarity search                 |
| `/health`              | GET    | Health check                               |

---

## Batch Processing Endpoints

### POST /cognify-batch

Batch extract entities from SQL/JSON records for database migration.

**Request:**

```json
{
  "items": [
    {
      "source_id": "emp_123",
      "source_table": "employees",
      "content": "John Smith is a Senior Engineer in the Backend team",
      "raw_data": { "id": 123, "name": "John Smith" }
    }
  ]
}
```

**Response:**

```json
[
  {
    "source_id": "emp_123",
    "entities": [
      {
        "name": "John Smith",
        "type": "Entity",
        "description": "Senior Engineer in Backend team",
        "tags": ["employees", "imported"]
      }
    ],
    "relations": []
  }
]
```

---

### POST /summarize_batch

Summarize a batch of conversation text for the Wisdom Layer (Cold Path).

**Request:**

```json
{
  "text": "User: What's my cat's name?\nAI: Your cat is named Luna.\nUser: She likes to sleep on my laptop.\nAI: That's adorable!",
  "type": "crystallize"
}
```

**Response:**

```json
{
  "summary": "Key facts: Luna: User's cat; sleeps on laptop",
  "entities": [
    { "name": "Luna", "type": "Entity", "description": "User's cat" }
  ]
}
```

---

## GraphRAG Layer Endpoints

### POST /summarize-community

Layer 2: Generate summary for a group of related entities (team, department, etc.).

**Request:**

```json
{
  "community_name": "Engineering Team",
  "community_type": "team",
  "entities": [
    { "name": "Alice", "role": "Tech Lead", "skills": ["Go", "Python"] },
    { "name": "Bob", "role": "Engineer", "skills": ["Python", "ML"] }
  ],
  "max_summary_length": 500
}
```

**Response:**

```json
{
  "community_name": "Engineering Team",
  "community_type": "team",
  "member_count": 2,
  "key_members": ["Alice", "Bob"],
  "summary": "Small engineering team led by Alice, focused on Python and Go development.",
  "key_facts": ["2 team members", "Strong Python expertise"],
  "common_skills": ["Python", "Go", "ML"]
}
```

---

### POST /summarize-global

Layer 3: Generate global overview from community summaries.

**Request:**

```json
{
  "namespace": "company_acme",
  "community_summaries": [
    {
      "community_name": "Engineering",
      "member_count": 15,
      "common_skills": ["Python"]
    },
    { "community_name": "Sales", "member_count": 10, "common_skills": ["CRM"] }
  ],
  "total_entities": 100
}
```

**Response:**

```json
{
  "namespace": "company_acme",
  "title": "Overview: company_acme",
  "executive_summary": "Dataset contains 100 entities organized into 2 communities...",
  "total_entities": 100,
  "total_communities": 2,
  "key_insights": ["Total entities: 100", "Communities: 2"],
  "top_skills": ["Python", "CRM"],
  "compression_ratio": 50.0
}
```

---

## Query Enhancement Endpoints

### POST /expand-query

Use LLM to extract entity names and search terms from a natural language query.

**Request:**

```json
{
  "query": "What's my favorite metal and what's my cat's name?"
}
```

**Response:**

```json
{
  "original_query": "What's my favorite metal and what's my cat's name?",
  "search_terms": ["favorite", "metal", "cat", "name"],
  "entity_names": ["Luna", "Platinum"]
}
```

---

## Vision Extraction Endpoints

### POST /extract-vision

Extract entities from an image using vision LLM (e.g., charts, diagrams, tables).

**Request:**

```json
{
  "image_base64": "iVBORw0KGgoAAAANSUhEUgAA...",
  "prompt": "Analyze this organizational chart and extract all people and relationships."
}
```

**Response:**

```json
{
  "raw_response": "This organizational chart shows...",
  "entities": [
    { "name": "John Smith", "type": "Person" },
    { "name": "Engineering", "type": "Department" }
  ],
  "relationships": [
    { "from": "John Smith", "to": "Engineering", "type": "leads" }
  ],
  "insights": ["CEO reports directly to the board."]
}
```

---

## Document Ingestion Endpoints

### POST /ingest-document

Ingest a document with tiered extraction for cost efficiency.

**Tiers:**

- **Tier 1**: Rule-based extraction (regex, spaCy NER) - FREE
- **Tier 2**: Smart chunking with clustering - CHEAP
- **Tier 3**: LLM extraction on cluster representatives - EXPENSIVE
- **Vision**: Vision LLM for complex diagrams - EXPENSIVE

**Request:**

```json
{
  "content_base64": "JVBERi0xLjQK...",
  "document_type": "pdf"
}
```

Or for text:

```json
{
  "text": "John Smith joined ACME Corp in 2023 as Senior Engineer.",
  "document_type": "text"
}
```

**Response:**

```json
{
  "entities": [
    {
      "name": "John Smith",
      "type": "Person",
      "confidence": 0.95,
      "source": "spacy"
    }
  ],
  "relationships": [],
  "stats": {
    "tier1_entities": 5,
    "tier2_clusters": 3,
    "tier3_llm_calls": 1,
    "vision_calls": 0
  },
  "summary": "Document contains 5 entities extracted via tiered processing."
}
```

---

### POST /ingest-vector-tree

Ingest a document using the Vector-Native Architecture. Skips expensive LLM steps in favor of mathematical compression.

**Request:**

```json
{
  "content_base64": "JVBERi0xLjQK...",
  "document_type": "pdf"
}
```

**Response:**

```json
{
    "entities": [],
    "relationships": [],
    "stats": {"chunks": 50, "clusters": 5},
    "summary": "Document compressed using vector tree.",
    "vector_tree": {
        "root_embedding": [...],
        "children": [...]
    }
}
```

---

## Embedding Endpoints

### POST /embed

Generate embedding vector for text.

**Request:**

```json
{
  "text": "What is machine learning?"
}
```

**Response:**

```json
{
    "embedding": [0.123, -0.456, 0.789, ...]
}
```

---

### POST /semantic-search

Find semantically similar candidates to a query.

**Request:**

```json
{
  "query": "machine learning algorithms",
  "candidates": [
    { "text": "Deep learning models", "data": { "id": 1 } },
    { "text": "Italian cuisine recipes", "data": { "id": 2 } }
  ],
  "top_k": 5,
  "threshold": 0.3
}
```

**Response:**

```json
{
  "results": [
    { "text": "Deep learning models", "similarity": 0.85, "data": { "id": 1 } }
  ]
}
```

---

## Environment Variables

```bash
# LLM Providers
OPENAI_API_KEY=sk-...          # Optional: OpenAI API key
ANTHROPIC_API_KEY=sk-...       # Optional: Anthropic API key
OLLAMA_HOST=http://ollama:11434  # Ollama endpoint

# Server
PORT=8000                       # Server port
```

## Running Locally

```bash
cd ai
pip install -r requirements.txt
python main.py
```

## Docker

```dockerfile
FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["python", "main.py"]
```

## Cost Optimization

The SLM approach reduces costs by:

1. **Model Selection**: Use cheaper models for simple tasks

   - Extraction: GPT-3.5-turbo or local Llama
   - Curation: Logic-focused, small model
   - Synthesis: More capable model when needed

2. **Batching**: Combine multiple extractions in one call

3. **Caching**: Cache synthesis results in Redis

4. **Local Fallback**: Ollama provides free local inference

5. **Tiered Document Ingestion**: Use free rule-based extraction first, only escalate to LLM when needed
