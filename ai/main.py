"""
AI Services - FastAPI server for SLM orchestration.
Provides extraction, curation, synthesis, and generation endpoints.
"""
import os
from typing import Optional
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from extraction_slm import ExtractionSLM
from curation_slm import CurationSLM
from synthesis_slm import SynthesisSLM
from llm_router import LLMRouter
from embedding_service import EmbeddingService


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize services on startup."""
    app.state.llm_router = LLMRouter()
    app.state.extraction = ExtractionSLM(app.state.llm_router)
    app.state.curation = CurationSLM(app.state.llm_router)
    app.state.synthesis = SynthesisSLM(app.state.llm_router)
    app.state.embedding = EmbeddingService()
    yield
    # Cleanup if needed


app = FastAPI(
    title="Reflective Memory Kernel - AI Services",
    description="SLM orchestration for extraction, curation, and synthesis",
    version="1.0.0",
    lifespan=lifespan,
)


# Request/Response models
class ExtractionRequest(BaseModel):
    user_query: str
    ai_response: str
    context: Optional[str] = None


class ExtractedEntity(BaseModel):
    name: str
    type: str
    description: Optional[str] = ""
    tags: list[str] = []
    attributes: dict = {}
    relations: list = []


class CurationRequest(BaseModel):
    node1_name: str
    node1_description: str
    node1_created_at: str
    node2_name: str
    node2_description: str
    node2_created_at: str


class CurationResponse(BaseModel):
    winner_index: int  # 1 or 2
    reason: str


class SynthesisRequest(BaseModel):
    query: str
    context: Optional[str] = None
    facts: Optional[list] = []
    insights: Optional[list] = []
    proactive_alerts: Optional[list] = []


class SynthesisResponse(BaseModel):
    brief: str
    confidence: float


class InsightRequest(BaseModel):
    node1_name: str
    node1_type: str
    node1_description: Optional[str] = None
    node2_name: str
    node2_type: str
    node2_description: Optional[str] = None
    path_exists: bool
    path_length: int


class InsightResponse(BaseModel):
    has_insight: bool
    insight_type: str = ""
    summary: str = ""
    action_suggestion: str = ""
    confidence: float = 0.0


class GenerateRequest(BaseModel):
    query: str
    context: Optional[str] = None
    proactive_alerts: Optional[list] = []


class GenerateResponse(BaseModel):
    response: str


class ExpandQueryRequest(BaseModel):
    query: str


class ExpandQueryResponse(BaseModel):
    original_query: str
    search_terms: list[str]
    entity_names: list[str]


# Endpoints
@app.post("/extract", response_model=list[ExtractedEntity])
async def extract_entities(request: ExtractionRequest):
    """Extract structured entities from conversation."""
    try:
        entities = await app.state.extraction.extract(
            request.user_query,
            request.ai_response,
            request.context,
        )
        return entities
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/curate", response_model=CurationResponse)
async def curate_contradiction(request: CurationRequest):
    """Determine which of two contradicting facts is more reliable."""
    try:
        result = await app.state.curation.resolve(
            node1={
                "name": request.node1_name,
                "description": request.node1_description,
                "created_at": request.node1_created_at,
            },
            node2={
                "name": request.node2_name,
                "description": request.node2_description,
                "created_at": request.node2_created_at,
            },
        )
        return result
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/synthesize", response_model=SynthesisResponse)
async def synthesize_brief(request: SynthesisRequest):
    """Synthesize a coherent brief from facts and insights."""
    try:
        result = await app.state.synthesis.synthesize(
            query=request.query,
            context=request.context,
            facts=request.facts,
            insights=request.insights,
            alerts=request.proactive_alerts,
        )
        return result
    except Exception as e:
        import traceback
        print(f"SYNTHESIZE ERROR: {e}", flush=True)
        print(traceback.format_exc(), flush=True)
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/synthesize-insight", response_model=InsightResponse)
async def synthesize_insight(request: InsightRequest):
    """Evaluate if two nodes have an emergent insight."""
    try:
        result = await app.state.synthesis.evaluate_connection(
            node1={"name": request.node1_name, "type": request.node1_type, "description": request.node1_description},
            node2={"name": request.node2_name, "type": request.node2_type, "description": request.node2_description},
            path_exists=request.path_exists,
            path_length=request.path_length,
        )
        return result
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/generate", response_model=GenerateResponse)
async def generate_response(request: GenerateRequest):
    """Generate a conversational response."""
    try:
        print(f"DEBUG /generate: query='{request.query[:50]}...' context_len={len(request.context) if request.context else 0}", flush=True)
        if request.context:
            print(f"DEBUG /generate CONTEXT: {request.context[:200]}...", flush=True)
        response = await app.state.llm_router.generate(
            query=request.query,
            context=request.context,
            alerts=request.proactive_alerts,
        )
        return GenerateResponse(response=response)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/expand-query", response_model=ExpandQueryResponse)
async def expand_query(request: ExpandQueryRequest):
    """Use LLM to extract entity names and search terms from a query."""
    try:
        prompt = f"""Extract entity names and search terms from this query.
Return JSON: {{"search_terms": ["term1", "term2"], "entity_names": ["Name1", "Name2"]}}

Query: "{request.query}"

Rules:
- search_terms: keywords to search (e.g., "metal", "favorite", "cat")
- entity_names: specific names that might be stored (e.g., "Platinum", "Luna", "Emma")
- Be thorough but concise
- Return ONLY the JSON, no explanation

JSON:"""
        
        result = await app.state.llm_router.extract_json(prompt, provider="nvidia", model="meta/llama-3.1-70b-instruct")
        
        search_terms = result.get("search_terms", []) if result else []
        entity_names = result.get("entity_names", []) if result else []
        
        print(f"DEBUG /expand-query: '{request.query}' -> terms={search_terms}, entities={entity_names}", flush=True)
        
        return ExpandQueryResponse(
            original_query=request.query,
            search_terms=search_terms,
            entity_names=entity_names
        )
    except Exception as e:
        print(f"DEBUG /expand-query error: {e}", flush=True)
        # Return basic keyword extraction on failure
        words = request.query.lower().replace("?", "").split()
        return ExpandQueryResponse(
            original_query=request.query,
            search_terms=[w for w in words if len(w) > 2],
            entity_names=[]
        )


# Embedding endpoints for semantic search
class EmbedRequest(BaseModel):
    text: str


class EmbedResponse(BaseModel):
    embedding: list[float]


class SemanticSearchRequest(BaseModel):
    query: str
    candidates: list[dict]  # List of {"text": str, "embedding": list[float], "data": any}
    top_k: int = 5
    threshold: float = 0.3


class SemanticSearchResponse(BaseModel):
    results: list[dict]


@app.post("/embed", response_model=EmbedResponse)
async def generate_embedding(request: EmbedRequest):
    """Generate embedding vector for text."""
    try:
        embedding = await app.state.embedding.get_embedding(request.text)
        return EmbedResponse(embedding=embedding)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/semantic-search", response_model=SemanticSearchResponse)
async def semantic_search(request: SemanticSearchRequest):
    """Find semantically similar candidates to query."""
    try:
        print(f"DEBUG /semantic-search: query='{request.query[:50]}', candidates={len(request.candidates)}", flush=True)
        
        # Get query embedding
        query_embedding = await app.state.embedding.get_embedding(request.query)
        if not query_embedding:
            print("DEBUG: Failed to get query embedding", flush=True)
            return SemanticSearchResponse(results=[])
        print(f"DEBUG: Query embedding length: {len(query_embedding)}", flush=True)
        
        # Generate embeddings for candidates that don't have them
        candidates_with_embeddings = []
        for i, candidate in enumerate(request.candidates[:20]):  # Limit to 20 for speed
            if "embedding" not in candidate or not candidate.get("embedding"):
                # Generate embedding from text field
                text = candidate.get("text", "")
                if text:
                    embedding = await app.state.embedding.get_embedding(text)
                    candidate["embedding"] = embedding
                    if i < 3:  # Debug first 3
                        print(f"DEBUG: Candidate {i} '{text[:30]}' embedding len: {len(embedding) if embedding else 0}", flush=True)
            candidates_with_embeddings.append(candidate)
        
        # Find similar candidates with LOWER threshold
        results = app.state.embedding.find_most_similar(
            query_embedding,
            candidates_with_embeddings,
            top_k=request.top_k,
            threshold=0.1  # Lowered from 0.3
        )
        print(f"DEBUG /semantic-search: found {len(results)} matches", flush=True)
        for r in results[:3]:
            print(f"DEBUG: Match '{r.get('text', '')[:30]}' similarity={r.get('similarity', 0):.3f}", flush=True)
        return SemanticSearchResponse(results=results)
    except Exception as e:
        print(f"DEBUG /semantic-search error: {e}", flush=True)
        import traceback
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy"}


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8000"))
    uvicorn.run(app, host="0.0.0.0", port=port)
