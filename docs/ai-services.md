<<<<<<< HEAD
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
        "attributes": {"role": "partner"},
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
        "attributes": {"category": "cuisine"},
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
        {"name": "Alex", "description": "User's partner"},
        {"name": "Thai Food", "description": "Alex's favorite cuisine"}
    ],
    "insights": [
        {"summary": "Thai food may contain peanuts - allergy risk"}
    ],
    "proactive_alerts": [
        "User has peanut allergy"
    ]
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
    "proactive_alerts": [
        "If Thai food is suggested, mention peanut allergy risk"
    ]
}
```

### Response

```json
{
    "response": "How about Thai food? Alex loves it! Just remember to check for peanuts since you're allergic."
}
```

## API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/extract` | POST | Extract entities from text |
| `/curate` | POST | Resolve contradictions |
| `/synthesize` | POST | Create coherent brief |
| `/synthesize-insight` | POST | Evaluate potential insight |
| `/generate` | POST | Generate response |
| `/health` | GET | Health check |

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
=======
# AI Services

Python FastAPI server providing LLM orchestration for entity extraction, curation, synthesis, embeddings, and response generation.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       FastAPI Application                        │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌───────────┐  │
│  │  /extract   │ │  /curate    │ │ /synthesize │ │ /generate │  │
│  └──────┬──────┘ └──────┬──────┘ └──────┬──────┘ └─────┬─────┘  │
│         │               │               │               │        │
│  ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐       │        │
│  │ Extraction  │ │  Curation   │ │  Synthesis  │       │        │
│  │    SLM      │ │    SLM      │ │    SLM      │       │        │
│  └──────┬──────┘ └──────┬──────┘ └──────┬──────┘       │        │
│         │               │               │               │        │
│         └───────────────┴───────────────┴───────────────┘        │
│                                 │                                 │
│                         ┌───────▼───────┐                        │
│                         │   LLM Router  │                        │
│                         └───────┬───────┘                        │
│                                 │                                 │
│    ┌────────────────────────────┼────────────────────────────┐   │
│    │                            │                            │   │
│    ▼                            ▼                            ▼   │
│ ┌──────┐                   ┌──────┐                    ┌──────┐  │
│ │Ollama│                   │NVIDIA│                    │OpenAI│  │
│ │Local │                   │ NIM  │                    │ API  │  │
│ └──────┘                   └──────┘                    └──────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### main.py
FastAPI application with all endpoint definitions.

### extraction_slm.py
Entity extraction from conversation transcripts.

```python
class ExtractionSLM:
    async def extract(
        self,
        user_query: str,
        ai_response: str,
        context: Optional[str] = None
    ) -> List[ExtractedEntity]
```

**Extracted Entity Structure**:
```python
class ExtractedEntity:
    name: str           # Entity name
    type: str           # Entity/Fact/Preference/Event
    description: str    # Optional description
    tags: List[str]     # Searchable tags
    attributes: dict    # Additional properties
    relations: List     # Relationships to other entities
```

### curation_slm.py
Contradiction resolution between conflicting facts.

```python
class CurationSLM:
    async def resolve(
        self,
        node1: dict,  # {name, description, created_at}
        node2: dict   # {name, description, created_at}
    ) -> CurationResponse
```

### synthesis_slm.py
Response synthesis and insight evaluation.

```python
class SynthesisSLM:
    async def synthesize(
        self,
        query: str,
        context: Optional[str],
        facts: List[dict],
        insights: List[dict],
        alerts: List[str]
    ) -> SynthesisResponse

    async def evaluate_connection(
        self,
        node1: dict,
        node2: dict,
        path_exists: bool,
        path_length: int
    ) -> InsightResponse
```

### llm_router.py
Multi-provider LLM abstraction layer.

```python
class LLMRouter:
    async def generate(
        self,
        query: str,
        context: Optional[str] = None,
        alerts: Optional[List[str]] = None,
        provider: str = "ollama",
        model: str = None
    ) -> str

    async def extract_json(
        self,
        prompt: str,
        provider: str = "nvidia",
        model: str = None
    ) -> dict
```

**Supported Providers**:
| Provider | Models | Use Case |
|----------|--------|----------|
| Ollama | llama3.1, mistral | Local development |
| NVIDIA NIM | llama-3.1-70b-instruct | Production quality |
| OpenAI | gpt-4, gpt-3.5-turbo | Fallback option |

### embedding_service.py
Vector embedding generation for semantic search.

```python
class EmbeddingService:
    async def get_embedding(self, text: str) -> List[float]
    
    def find_most_similar(
        self,
        query_embedding: List[float],
        candidates: List[dict],
        top_k: int = 5,
        threshold: float = 0.3
    ) -> List[dict]
```

## Endpoints

| Endpoint | Purpose |
|----------|---------|
| `POST /extract` | Extract entities from conversation |
| `POST /curate` | Resolve contradictions |
| `POST /synthesize` | Create coherent brief from facts |
| `POST /synthesize-insight` | Evaluate potential insight |
| `POST /generate` | Generate conversational response |
| `POST /embed` | Generate embedding vector |
| `POST /semantic-search` | Find similar candidates |
| `POST /expand-query` | Extract search terms from query |
| `GET /health` | Health check |

## Configuration

Environment variables:

```bash
# Required
NVIDIA_API_KEY=nvapi-...

# Optional
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-...
OLLAMA_HOST=http://localhost:11434
PORT=8000
```

## Dependencies

```txt
fastapi>=0.104.0
uvicorn>=0.24.0
pydantic>=2.0.0
httpx>=0.25.0
numpy>=1.24.0
```

## Running Locally

```bash
cd ai
pip install -r requirements.txt
python main.py
# Server starts on http://localhost:8000
```

## Docker

```dockerfile
FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY . .
CMD ["python", "main.py"]
```
>>>>>>> 5f37bd4 (Major update: API timeout fixes, Vector-Native ingestion, Frontend integration)
