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


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize services on startup."""
    app.state.llm_router = LLMRouter()
    app.state.extraction = ExtractionSLM(app.state.llm_router)
    app.state.curation = CurationSLM(app.state.llm_router)
    app.state.synthesis = SynthesisSLM(app.state.llm_router)
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
    facts: list = []
    insights: list = []
    proactive_alerts: list = []


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
    proactive_alerts: list = []


class GenerateResponse(BaseModel):
    response: str


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
        response = await app.state.llm_router.generate(
            query=request.query,
            context=request.context,
            alerts=request.proactive_alerts,
        )
        return GenerateResponse(response=response)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy"}


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8000"))
    uvicorn.run(app, host="0.0.0.0", port=port)
