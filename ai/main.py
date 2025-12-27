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
from document_ingester import DocumentIngester, IngestDocumentRequest, IngestDocumentResponse


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize services on startup."""
    app.state.llm_router = LLMRouter()
    app.state.extraction = ExtractionSLM(app.state.llm_router)
    app.state.curation = CurationSLM(app.state.llm_router)
    app.state.synthesis = SynthesisSLM(app.state.llm_router)
    app.state.embedding = EmbeddingService()
    app.state.document_ingester = DocumentIngester(llm_router=app.state.llm_router)
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


class VisionExtractRequest(BaseModel):
    """Request for vision-based entity extraction from images."""
    image_base64: str  # Base64-encoded image
    prompt: Optional[str] = None  # Custom prompt (uses default if not provided)


class VisionExtractResponse(BaseModel):
    """Response from vision extraction."""
    raw_response: str
    entities: list[dict] = []
    relationships: list[dict] = []
    insights: list[str] = []

# Migration batch processing models
class CognifyItem(BaseModel):
    source_id: str
    source_table: str
    content: str
    raw_data: dict = {}


class CognifyResult(BaseModel):
    source_id: str
    entities: list[ExtractedEntity] = []
    relations: list = []


class CognifyBatchRequest(BaseModel):
    items: list[CognifyItem]


# Batch summarization for Wisdom Layer
class SummarizeBatchRequest(BaseModel):
    text: str
    type: Optional[str] = "crystallize"


class SummarizeBatchResponse(BaseModel):
    summary: str
    entities: list[ExtractedEntity]


# Endpoints
@app.post("/cognify-batch", response_model=list[CognifyResult])
async def cognify_batch(request: CognifyBatchRequest):
    """Batch extract entities from SQL/JSON records for migration. NO FALLBACKS - LLM required."""
    results = []
    for item in request.items:
        # Use extraction to get entities - NO FALLBACK, will raise on failure
        entities = await app.state.extraction.extract(
            user_query=item.content,
            ai_response="Imported from database",
            context=f"Source: {item.source_table}",
        )
        
        # Convert to ExtractedEntity format
        entity_list = []
        for e in entities:
            entity_list.append(ExtractedEntity(
                name=e.get("name", item.source_id),
                type=e.get("type", "Entity"),
                description=e.get("description", ""),
                tags=e.get("tags", [item.source_table, "imported"]),
                attributes=e.get("attributes", {}),
                relations=e.get("relations", []),
            ))
        
        results.append(CognifyResult(
            source_id=item.source_id,
            entities=entity_list,
            relations=[],
        ))
    return results


# =============================================================================
# LAYER 2: Community Summarization
# =============================================================================

class CommunityRequest(BaseModel):
    """Request to summarize a group of entities (team, department, etc.)"""
    community_name: str
    community_type: str  # "team", "department", "company", etc.
    entities: list[dict]  # List of entity data to summarize
    max_summary_length: int = 500


class CommunitySummary(BaseModel):
    """Summary of a community/cluster of entities"""
    community_name: str
    community_type: str
    member_count: int
    key_members: list[str]  # Top N names
    summary: str
    key_facts: list[str]
    common_skills: list[str] = []
    common_attributes: dict = {}


@app.post("/summarize-community", response_model=CommunitySummary)
async def summarize_community(request: CommunityRequest):
    """Layer 2: Generate summary for a group of related entities."""
    try:
        # Extract key information from entities
        names = [e.get("name", e.get("full_name", "Unknown")) for e in request.entities]
        
        # Collect common skills
        all_skills = []
        for e in request.entities:
            skills = e.get("skills", [])
            all_skills.extend(skills if isinstance(skills, list) else [])
        skill_counts = {}
        for s in all_skills:
            skill_counts[s] = skill_counts.get(s, 0) + 1
        common_skills = sorted(skill_counts.keys(), key=lambda x: skill_counts[x], reverse=True)[:10]
        
        # Build context for LLM summarization
        entity_summaries = []
        for e in request.entities[:20]:  # Limit to first 20 for LLM context
            name = e.get("name", e.get("full_name", "Unknown"))
            role = e.get("role", "")
            skills = e.get("skills", [])[:3]
            entity_summaries.append(f"- {name}: {role} ({', '.join(skills) if skills else 'N/A'})")
        
        prompt = f"""Summarize this {request.community_type} called "{request.community_name}" with {len(request.entities)} members:

Members (sample):
{chr(10).join(entity_summaries)}

Common skills: {', '.join(common_skills[:5])}

Generate:
1. A concise summary (max {request.max_summary_length} chars)
2. 3-5 key facts about this group"""

        # Use LLM for summary - NO FALLBACK, LLM required
        from llm_router import get_llm_response
        response = await get_llm_response(prompt, max_tokens=300)
        
        if not response:
            raise HTTPException(status_code=500, detail="LLM failed to generate summary")
        
        lines = response.strip().split("\n")
        summary_text = lines[0] if lines else f"{request.community_name} has {len(request.entities)} members"
        key_facts = [l.strip("- ") for l in lines[1:] if l.strip()][:5]
        
        return CommunitySummary(
            community_name=request.community_name,
            community_type=request.community_type,
            member_count=len(request.entities),
            key_members=names[:10],
            summary=summary_text[:request.max_summary_length],
            key_facts=key_facts,
            common_skills=common_skills,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# =============================================================================
# LAYER 3: Global Overview Summarization
# =============================================================================

class GlobalOverviewRequest(BaseModel):
    """Request to generate global overview from community summaries"""
    namespace: str
    community_summaries: list[CommunitySummary]
    total_entities: int
    total_relationships: int = 0


class GlobalOverview(BaseModel):
    """Top-level summary of entire dataset"""
    namespace: str
    title: str
    executive_summary: str
    total_entities: int
    total_communities: int
    key_insights: list[str]
    top_skills: list[str]
    compression_ratio: float  # Original vs stored


@app.post("/summarize-global", response_model=GlobalOverview)
async def summarize_global(request: GlobalOverviewRequest):
    """Layer 3: Generate global overview from community summaries."""
    try:
        # Aggregate from community summaries
        all_skills = []
        all_key_facts = []
        community_names = []
        
        for cs in request.community_summaries:
            all_skills.extend(cs.common_skills)
            all_key_facts.extend(cs.key_facts)
            community_names.append(f"{cs.community_name} ({cs.member_count} members)")
        
        # Calculate top skills across communities
        skill_counts = {}
        for s in all_skills:
            skill_counts[s] = skill_counts.get(s, 0) + 1
        top_skills = sorted(skill_counts.keys(), key=lambda x: skill_counts[x], reverse=True)[:10]
        
        # Generate executive summary
        total_members = sum(cs.member_count for cs in request.community_summaries)
        
        exec_summary = f"Dataset '{request.namespace}' contains {request.total_entities} entities "
        exec_summary += f"organized into {len(request.community_summaries)} communities. "
        if top_skills:
            exec_summary += f"Primary skills: {', '.join(top_skills[:5])}. "
        exec_summary += f"Communities: {', '.join(community_names[:5])}."
        
        # Generate key insights
        key_insights = [
            f"Total entities: {request.total_entities}",
            f"Communities identified: {len(request.community_summaries)}",
            f"Top skill: {top_skills[0]}" if top_skills else "No skills data",
        ]
        
        # Add insights from community summaries
        for fact in all_key_facts[:5]:
            if fact not in key_insights:
                key_insights.append(fact)
        
        return GlobalOverview(
            namespace=request.namespace,
            title=f"Overview: {request.namespace}",
            executive_summary=exec_summary,
            total_entities=request.total_entities,
            total_communities=len(request.community_summaries),
            key_insights=key_insights[:10],
            top_skills=top_skills,
            compression_ratio=request.total_entities / max(len(request.community_summaries), 1),
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


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


@app.post("/summarize_batch", response_model=SummarizeBatchResponse)
async def summarize_batch(request: SummarizeBatchRequest):
    """Summarize a batch of conversation text and extract entities.
    
    Used by the Wisdom Layer (Cold Path) to crystallize conversations into
    high-density knowledge nodes.
    """
    try:
        print(f"DEBUG /summarize_batch: processing {len(request.text)} chars", flush=True)
        
        # Parse conversation text into turns (format: "User: ...\nAI: ...\n")
        all_entities = []
        lines = request.text.strip().split('\n')
        
        user_query = ""
        ai_response = ""
        
        for line in lines:
            line = line.strip()
            if line.startswith("User:"):
                # If we have a complete pair, process it
                if user_query and ai_response:
                    entities = await app.state.extraction.extract(user_query, ai_response)
                    all_entities.extend(entities)
                user_query = line[5:].strip()
                ai_response = ""
            elif line.startswith("AI:"):
                ai_response = line[3:].strip()
        
        # Process the last pair
        if user_query and ai_response:
            entities = await app.state.extraction.extract(user_query, ai_response)
            all_entities.extend(entities)
        
        # Generate summary from extracted entities
        if all_entities:
            summary_parts = [f"{e.get('name', 'Unknown')}: {e.get('description', '')}" for e in all_entities]
            summary = "Key facts: " + "; ".join(summary_parts[:5])  # Limit to 5
        else:
            summary = "No significant entities extracted from this conversation batch."
        
        print(f"DEBUG /summarize_batch: extracted {len(all_entities)} entities", flush=True)
        
        return SummarizeBatchResponse(
            summary=summary,
            entities=[ExtractedEntity(**e) for e in all_entities]
        )
    except Exception as e:
        import traceback
        print(f"SUMMARIZE_BATCH ERROR: {e}", flush=True)
        print(traceback.format_exc(), flush=True)
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


@app.post("/extract-vision", response_model=VisionExtractResponse)
async def extract_vision(request: VisionExtractRequest):
    """
    Extract entities from an image using vision LLM (MiniMax M2).
    
    Use this for charts, diagrams, tables, and figures in documents.
    """
    try:
        # Default prompt for entity extraction
        prompt = request.prompt or """Analyze this image from a document. Extract:

1. **Type**: Is this a chart, diagram, table, or figure?
2. **Title**: What is the title or caption?
3. **Key Entities**: List all named entities (people, places, concepts, metrics)
4. **Relationships**: What relationships or connections are shown?
5. **Data Points**: Extract any numerical data or statistics
6. **Insight**: What is the main takeaway or conclusion?

Return as JSON:
{
  "type": "chart|diagram|table|figure",
  "title": "...",
  "entities": [{"name": "...", "type": "Person|Concept|Metric|Location"}],
  "relationships": [{"from": "...", "to": "...", "type": "..."}],
  "data_points": [{"label": "...", "value": "..."}],
  "insight": "..."
}"""

        print(f"DEBUG /extract-vision: image_size={len(request.image_base64)} bytes", flush=True)
        
        # Call vision model
        raw_response = await app.state.llm_router.generate_vision(
            image_base64=request.image_base64,
            prompt=prompt,
        )
        
        print(f"DEBUG /extract-vision response: {raw_response[:200]}...", flush=True)
        
        # Try to parse JSON from response
        import json
        entities = []
        relationships = []
        insights = []
        
        try:
            # Find JSON in response
            import re
            json_match = re.search(r'\{.*\}', raw_response, re.DOTALL)
            if json_match:
                parsed = json.loads(json_match.group())
                entities = parsed.get("entities", [])
                relationships = parsed.get("relationships", [])
                insight = parsed.get("insight", "")
                if insight:
                    insights.append(insight)
        except (json.JSONDecodeError, ValueError) as e:
            print(f"DEBUG: Failed to parse vision JSON: {e}", flush=True)
            # Extract insight from raw response
            insights.append(raw_response[:500])
        
        return VisionExtractResponse(
            raw_response=raw_response,
            entities=entities,
            relationships=relationships,
            insights=insights,
        )
    except Exception as e:
        print(f"DEBUG /extract-vision error: {e}", flush=True)
        import traceback
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/ingest-document", response_model=IngestDocumentResponse)
async def ingest_document(request: IngestDocumentRequest):
    """
    Ingest a document with tiered extraction for cost efficiency.
    
    Uses:
    - Tier 1: Rule-based extraction (regex, spaCy NER) - FREE
    - Tier 2: Smart chunking with clustering - CHEAP
    - Tier 3: LLM extraction on cluster representatives - EXPENSIVE
    - Vision: MiniMax M2 for complex diagrams - EXPENSIVE
    """
    try:
        import base64
        import tempfile
        import os
        
        ingester = app.state.document_ingester
        
        if request.text:
            print(f"DEBUG /ingest-document: text ingestion, {len(request.text)} chars", flush=True)
            result = await ingester.ingest_text(request.text)
        elif request.content_base64 and request.document_type == "pdf":
            print(f"DEBUG /ingest-document: PDF ingestion", flush=True)
            pdf_bytes = base64.b64decode(request.content_base64)
            with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as f:
                f.write(pdf_bytes)
                temp_path = f.name
            try:
                result = await ingester.ingest_pdf(temp_path)
            finally:
                os.unlink(temp_path)
        else:
            raise HTTPException(status_code=400, detail="Provide 'text' or 'content_base64' with document_type='pdf'")
        
        entities = [{"name": e.name, "type": e.entity_type, "description": e.description, "confidence": e.confidence, "source": e.source} for e in result.entities]
        relationships = [{"from": r.from_entity, "to": r.to_entity, "type": r.relation_type, "confidence": r.confidence} for r in result.relationships]
        
        print(f"DEBUG /ingest-document: extracted {len(entities)} entities, stats={result.stats}", flush=True)
        
        return IngestDocumentResponse(entities=entities, relationships=relationships, stats=result.stats, summary=result.summary)
    except Exception as e:
        print(f"DEBUG /ingest-document error: {e}", flush=True)
        import traceback
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/ingest-vector-tree", response_model=IngestDocumentResponse)
async def ingest_vector_tree(request: IngestDocumentRequest):
    """
    Ingest a document using the Vector-Native Architecture.
    Skips expensive LLM steps in favor of Mathematical compression.
    """
    try:
        import base64
        import tempfile
        import os
        
        ingester = app.state.document_ingester
        print(f"DEBUG /ingest-vector-tree: PDF ingestion (Math Mode)", flush=True)

        if not (request.content_base64 and request.document_type == "pdf"):
             raise HTTPException(status_code=400, detail="Only PDF content_base64 is supported for Vector Tree mode.")

        pdf_bytes = base64.b64decode(request.content_base64)
        with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as f:
            f.write(pdf_bytes)
            temp_path = f.name
        
        try:
            # Call the new Math-based method
            result = await ingester.ingest_pdf_with_tree(temp_path)
        finally:
            os.unlink(temp_path)
        
        return IngestDocumentResponse(
            entities=[], 
            relationships=[], 
            stats=result.stats, 
            summary=result.summary,
            vector_tree=result.vector_tree
        )
    except Exception as e:
        print(f"DEBUG /ingest-vector-tree error: {e}", flush=True)
        import traceback
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))

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


# =============================================================================
# DOCUMENT INGESTION - Vector-Native Architecture
# =============================================================================

@app.post("/ingest", response_model=IngestDocumentResponse)
async def ingest_document(request: IngestDocumentRequest):
    """
    Ingest a document using Vector-Native Hierarchical architecture.
    
    - Chunks document into 200-word segments
    - Generates embeddings for each chunk
    - Extracts entities using tiered approach
    - Builds hierarchical vector tree
    """
    try:
        ingester: DocumentIngester = app.state.document_ingester
        
        if request.text:
            # Plain text ingestion
            result = await ingester.ingest_text(request.text)
        elif request.content_base64:
            # Base64 encoded content (future: PDF support)
            import base64
            content = base64.b64decode(request.content_base64).decode('utf-8', errors='ignore')
            result = await ingester.ingest_text(content)
        else:
            raise HTTPException(status_code=400, detail="Either text or content_base64 is required")
        
        # Convert result to response format
        entities = [
            {
                "name": e.name,
                "type": e.entity_type,
                "description": e.description,
                "confidence": e.confidence,
                "source": e.source
            }
            for e in result.entities
        ]
        
        relationships = [
            {
                "from_entity": r.from_entity,
                "to_entity": r.to_entity,
                "relation_type": r.relation_type,
                "confidence": r.confidence
            }
            for r in result.relationships
        ]
        
        print(f"DEBUG /ingest: processed document with {len(entities)} entities, {len(result.chunks)} chunks", flush=True)
        
        return IngestDocumentResponse(
            entities=entities,
            relationships=relationships,
            stats=result.stats,
            summary=result.summary,
            vector_tree=result.vector_tree if result.vector_tree else None
        )
        
    except Exception as e:
        print(f"DEBUG /ingest error: {e}", flush=True)
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
