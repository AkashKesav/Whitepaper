# 3-Layer Compression Pipeline

The Reflective Memory Kernel (RMK) implements a 3-layer compression architecture that achieves **800:1+ compression** on enterprise data while preserving semantic meaning.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                    INPUT: Raw Database Records (1GB+)               │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│   LAYER 1: Local Entity Parsing                                     │
│   ─────────────────────────────                                     │
│   • Instant parsing (no LLM needed)                                 │
│   • Extract structured entities from JSON/SQL records               │
│   • Deduplicate by name → merge skills/attributes                   │
│   • Compression: ~160:1 (368K records → 2,274 entities)            │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│   LAYER 2: Community Summarization (DeepSeek LLM)                   │
│   ───────────────────────────────────────────────                   │
│   • Group entities by department/team                               │
│   • LLM generates semantic summary for each community               │
│   • Extracts key members, skills, and facts                         │
│   • Compression: ~126:1 (2,274 entities → 18 summaries)            │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│   LAYER 3: Global Overview (DeepSeek LLM)                           │
│   ────────────────────────────────────────                          │
│   • Combine all community summaries                                 │
│   • LLM generates executive summary of entire dataset               │
│   • Identifies top skills, key patterns, insights                   │
│   • Compression: 18:1 (18 summaries → 1 overview)                  │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    OUTPUT: Compressed Knowledge (1.3MB)              │
│                    Overall Compression: 811:1                        │
└─────────────────────────────────────────────────────────────────────┘
```

## Benchmark Results (1.1GB Dataset)

| Metric | Value |
|--------|-------|
| **Input Size** | 1.02 GB (368,125 records) |
| **Output Size** | 1.26 MB |
| **Compression Ratio** | **811:1** |
| **Processing Time** | 193 seconds (~3 min) |
| **LLM Calls** | 19 (18 L2 + 1 L3) |
| **Fallbacks Used** | 0 (100% LLM-powered) |

### Layer Breakdown

| Layer | Input | Output | Compression | Time |
|-------|-------|--------|-------------|------|
| L1 (Local) | 368,125 records | 2,274 entities | 161.9:1 | <1 sec |
| L2 (LLM) | 2,274 entities | 18 summaries | 126.3:1 | ~3 min |
| L3 (LLM) | 18 summaries | 1 overview | 18:1 | ~30 sec |

## LLM Configuration

The pipeline uses **DeepSeek v3.2** via NVIDIA NIM API:

```python
# llm_router.py
model = "deepseek-ai/deepseek-v3.2"
provider = "nvidia"  # NVIDIA NIM API
timeout = 120  # seconds per request
```

### Environment Variables

```bash
NVIDIA_API_KEY=nvapi-xxx  # Required for DeepSeek
```

## API Endpoints

### Layer 1: Entity Extraction

```http
POST /cognify-batch
Content-Type: application/json

{
  "items": [
    {
      "source_id": "emp-001",
      "source_table": "employees",
      "content": "{\"name\": \"Alice\", \"role\": \"Engineer\"}",
      "raw_data": {"name": "Alice", "role": "Engineer"}
    }
  ]
}
```

**Response:**
```json
[
  {
    "source_id": "emp-001",
    "entities": [
      {"name": "Alice", "type": "Entity", "description": "Engineer"}
    ]
  }
]
```

### Layer 2: Community Summarization

```http
POST /summarize-community
Content-Type: application/json

{
  "community_name": "Engineering",
  "community_type": "department",
  "entities": [...],
  "max_summary_length": 300
}
```

**Response:**
```json
{
  "community_name": "Engineering",
  "member_count": 117,
  "summary": "Engineering department with 117 members focused on...",
  "key_members": ["Alice Johnson", "Bob Smith"],
  "common_skills": ["Go", "Python", "Kubernetes"],
  "key_facts": ["10 teams", "Core infrastructure focus"]
}
```

### Layer 3: Global Overview

```http
POST /summarize-global
Content-Type: application/json

{
  "namespace": "company-data",
  "community_summaries": [...],
  "total_entities": 2274
}
```

**Response:**
```json
{
  "title": "Overview: company-data",
  "total_entities": 2274,
  "total_communities": 18,
  "executive_summary": "Dataset contains 2274 entities organized into 18 communities...",
  "top_skills": ["Git", "AWS", "Node.js", "Kubernetes"],
  "key_insights": [...]
}
```

## Usage

### Run Pipeline Script

```bash
cd /path/to/whitepaper
python3 testdata/run_compression_pipeline.py
```

### Sample Output

```
============================================================
3-LAYER COMPRESSION PIPELINE DEMO
============================================================

=== LAYER 1: Local Entity Parsing (368125 records) ===
  Input records: 368125
  Unique entities: 2274
  L1 compression: 161.9:1
  ⚡ Completed in <1 second (no LLM needed)

=== LAYER 2: Community Summarization ===
  Communities found: 18
  ✓ Engineering: 117 members (LLM summarized)
  ✓ Marketing: 129 members (LLM summarized)
  ...
  L2 compression: 126.3:1

=== LAYER 3: Global Overview ===
  ✓ Global overview generated by LLM
  Summary: Dataset contains 2274 entities in 18 communities...

=== COMPRESSION RESULTS ===
  Original data: 1048218.5 KB (368125 records)
  L1 entities:   1267.9 KB (2274 entities)
  L2 summaries:  23.2 KB (18 communities)
  L3 overview:   1.3 KB (1 document)
  ────────────────────────────
  Total compressed: 1292.5 KB
  Overall compression: 811.0:1

✅ Pipeline complete!
```

## DGraph Storage

Compressed entities are stored in DGraph for graph-based querying:

```bash
# Query entity count
curl -X POST http://localhost:8180/query \
  -H "Content-Type: application/dql" \
  -d '{ total(func: has(dgraph.type)) { count(uid) } }'

# Sample entities
curl -X POST http://localhost:8180/query \
  -H "Content-Type: application/dql" \
  -d '{ q(func: type(Entity), first: 5) { uid name description } }'
```

### Sample DGraph Response

```json
{
  "data": {
    "q": [
      {
        "uid": "0x1",
        "name": "Alice Johnson",
        "description": "team: Platform\nskills: [Go Python Kubernetes]\nrole: Senior Engineer"
      },
      {
        "uid": "0x2",
        "name": "Bob Smith",
        "description": "role: Engineering Manager\nteam: Platform"
      }
    ]
  }
}
```

## Key Benefits

1. **Massive Compression**: 800:1+ reduction in data size
2. **Semantic Preservation**: LLM ensures meaning is retained
3. **Fast Processing**: L1 instant, L2/L3 parallelizable
4. **No Fallbacks**: 100% LLM-powered summaries
5. **Graph Storage**: DGraph enables relationship queries
6. **Enterprise Scale**: Handles 1GB+ datasets in minutes

## Files

| File | Description |
|------|-------------|
| `ai/main.py` | L2/L3 API endpoints |
| `ai/llm_router.py` | DeepSeek/NVIDIA NIM integration |
| `testdata/run_compression_pipeline.py` | Demo pipeline script |
| `testdata/large_dataset.jsonl` | 1GB test dataset |
