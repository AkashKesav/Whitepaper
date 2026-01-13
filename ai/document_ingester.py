"""
Document Ingester - Efficient document-to-memory pipeline.

Implements tiered extraction for cost efficiency:
- Tier 1: Rule-based extraction (FREE)
- Tier 2: Local embeddings + clustering (CHEAP) 
- Tier 3: LLM extraction on cluster representatives (EXPENSIVE)
"""
import base64
import io
import json
import re
from dataclasses import dataclass, field
from typing import Optional
import asyncio

# PDF processing
try:
    import fitz  # PyMuPDF
    HAS_PYMUPDF = True
except ImportError:
    HAS_PYMUPDF = False
    print("WARNING: PyMuPDF not installed. PDF processing disabled.")

# Local NER (optional)
try:
    import spacy
    nlp = spacy.load("en_core_web_sm")
    HAS_SPACY = True
except (ImportError, OSError):
    HAS_SPACY = False
    print("WARNING: spaCy not installed. Rule-based NER disabled.")

# Image analysis
try:
    from PIL import Image, ImageStat, ImageFilter
    HAS_PILLOW = True
except ImportError:
    HAS_PILLOW = False
    print("WARNING: Pillow not installed. Advanced image analysis disabled.")





@dataclass
class ExtractedEntity:
    """Entity extracted from document."""
    name: str
    entity_type: str  # Person, Organization, Location, Concept, Metric
    description: str = ""
    confidence: float = 1.0
    source: str = "rule"  # rule, llm, vision


@dataclass
class ExtractedRelationship:
    """Relationship between entities."""
    from_entity: str
    to_entity: str
    relation_type: str
    confidence: float = 1.0


@dataclass 
class DocumentChunk:
    """A chunk of document text with metadata."""
    text: str
    page_number: int
    chunk_index: int
    embedding: list[float] = field(default_factory=list)
    is_cluster_rep: bool = False


@dataclass
class ExtractedImage:
    """Image extracted from document."""
    image_base64: str
    page_number: int
    image_index: int
    complexity_score: float = 0.0
    caption: str = ""


@dataclass
class IngestionResult:
    """Result of document ingestion."""
    entities: list[ExtractedEntity]
    relationships: list[ExtractedRelationship]
    chunks: list[DocumentChunk]
    images: list[ExtractedImage]
    summary: str = ""
    stats: dict = field(default_factory=dict)
    vector_tree: dict = field(default_factory=dict) # NEW: The Hierarchical Vector Tree


class DocumentIngester:
    """
    Efficient document ingestion with tiered extraction.
    
    Cost optimization strategy:
    1. Rule-based extraction first (regex, spaCy NER)
    2. Smart chunking with semantic boundaries
    3. Cluster similar chunks, only process representatives
    4. Vision LLM only for complex diagrams
    """
    
    def __init__(self, llm_router=None, chunk_size: int = 512, chunk_overlap: int = 50):
        self.llm_router = llm_router
        self.chunk_size = chunk_size
        self.chunk_overlap = chunk_overlap
        
    async def ingest_pdf(self, pdf_path: str) -> IngestionResult:
        """
        Ingest a PDF document with tiered extraction.
        
        Args:
            pdf_path: Path to PDF file
            
        Returns:
            IngestionResult with entities, relationships, chunks
        """
        if not HAS_PYMUPDF:
            raise ImportError("PyMuPDF required for PDF processing. Install with: pip install pymupdf")
        
        # Extract text and images from PDF
        doc = fitz.open(pdf_path)
        all_text = []
        images = []
        
        for page_num, page in enumerate(doc):
            # Extract text
            text = page.get_text()
            all_text.append((page_num + 1, text))
            
            # Extract images
            for img_idx, img in enumerate(page.get_images(full=True)):
                try:
                    xref = img[0]
                    base_image = doc.extract_image(xref)
                    image_bytes = base_image["image"]
                    image_b64 = base64.b64encode(image_bytes).decode()
                    
                    images.append(ExtractedImage(
                        image_base64=image_b64,
                        page_number=page_num + 1,
                        image_index=img_idx,
                    ))
                except Exception as e:
                    print(f"Failed to extract image {img_idx} from page {page_num}: {e}")
        
        return await self._process_document(all_text, images)

    async def ingest_pdf_with_tree(self, pdf_path: str) -> IngestionResult:
        """
        NEW: Ingest PDF using the Vector-Native Architecture.
        Skips LLM Summarization. Uses Mathematical Compression.
        """
        if not HAS_PYMUPDF:
            raise ImportError("PyMuPDF required.")

        # 1. Extraction (Same as before)
        doc = fitz.open(pdf_path)
        all_text = []
        for page_num, page in enumerate(doc):
            all_text.append((page_num + 1, page.get_text()))
            
        # 2. Chunking
        chunks = self._create_chunks(all_text)
        
        # 3. Embedding (We need vectors for math to work!)
        # We assume self.llm_router has an embedding method, or we mock it.
        # For now, we'll try to use the router, or generate random if testing.
        for chunk in chunks:
            if self.llm_router:
                chunk.embedding = await self.llm_router.get_embedding(chunk.text)
            else:
                import numpy as np
                chunk.embedding = np.random.rand(1536).tolist() # Mock for testing without API
                
        # 4. Build Vector Tree
        try:
            from .vector_index.indexer import VectorIndexBuilder
        except ImportError:
            # Handle path if running from root
            from ai.vector_index.indexer import VectorIndexBuilder
            
        builder = VectorIndexBuilder()
        
        # Convert chunks to format expected by Builder
        input_chunks = [{'text': c.text, 'embedding': c.embedding} for c in chunks]
        vector_tree = builder.build_index(input_chunks)
        
        return IngestionResult(
            entities=[], # No NER needed for pure tree
            relationships=[],
            chunks=chunks,
            images=[],
            summary="Generated via Vector-Native Tree (Math Mode)",
            vector_tree=vector_tree
        )


    
    async def ingest_text(self, text: str) -> IngestionResult:
        """Ingest plain text document."""
        all_text = [(1, text)]
        return await self._process_document(all_text, [])
    
    async def _process_document(
        self, 
        pages: list[tuple[int, str]], 
        images: list[ExtractedImage]
    ) -> IngestionResult:
        """
        Process document with tiered extraction.
        """
        entities = []
        relationships = []
        llm_calls = 0
        vision_calls = 0
        
        # === TIER 1: Rule-based extraction (FREE) ===
        full_text = "\n".join([text for _, text in pages])
        tier1_entities = self._extract_rules(full_text)
        entities.extend(tier1_entities)
        
        # === TIER 2: Smart chunking ===
        chunks = self._create_chunks(pages)
        
        # Cluster chunks (simplified - in production use embeddings)
        # For now, mark every 5th chunk as representative
        for i, chunk in enumerate(chunks):
            chunk.is_cluster_rep = (i % 5 == 0)
        
        cluster_reps = [c for c in chunks if c.is_cluster_rep]
        
        # === TIER 3: LLM extraction on representatives only ===
        if self.llm_router and cluster_reps:
            for chunk in cluster_reps[:10]:  # Limit to 10 LLM calls max
                try:
                    llm_entities = await self._extract_with_llm(chunk.text)
                    entities.extend(llm_entities)
                    llm_calls += 1
                except Exception as e:
                    print(f"LLM extraction failed: {e}")
        
        # === VISION: Process complex images only ===
        if self.llm_router and images:
            for img in images[:50]:  # Higher limit, we filter by complexity
                # Calculate complexity score
                img.complexity_score = self._calculate_complexity(img.image_base64)
                
                # Check complexity threshold (e.g. > 5 means likely a chart/diagram)
                if img.complexity_score > 5.0:
                    try:
                        print(f"DEBUG: Processing complex image {img.image_index} (score={img.complexity_score:.2f})")
                        vision_entities = await self._extract_with_vision(img)
                        entities.extend(vision_entities)
                        vision_calls += 1
                    except Exception as e:
                        print(f"Vision extraction failed: {e}")
        
        # Deduplicate entities
        seen = set()
        unique_entities = []
        for e in entities:
            key = (e.name.lower(), e.entity_type)
            if key not in seen:
                seen.add(key)
                unique_entities.append(e)
        
        return IngestionResult(
            entities=unique_entities,
            relationships=relationships,
            chunks=chunks,
            images=images,
            stats={
                "pages": len(pages),
                "chunks": len(chunks),
                "cluster_reps": len(cluster_reps),
                "images": len(images),
                "tier1_entities": len(tier1_entities),
                "llm_calls": llm_calls,
                "vision_calls": vision_calls,
                "total_entities": len(unique_entities),
            }
        )
    
    
    def _calculate_complexity(self, image_b64: str) -> float:
        """
        Calculate image complexity score (0-10) using Pillow.
        High score = likely a chart, diagram, or dense content.
        Low score = blank page, simple photo, or low info.
        """
        if not HAS_PILLOW:
            # Fallback: simple size heuristic
            return min(len(image_b64) / 50000 * 5, 5.0)
            
        try:
            image_data = base64.b64decode(image_b64)
            img = Image.open(io.BytesIO(image_data)).convert('L')  # Convert to grayscale
            
            # 1. Entropy (information density)
            entropy = img.entropy()  # Typically 0-8
            
            # 2. Edge density (structure)
            edges = img.filter(ImageFilter.FIND_EDGES)
            edge_stat = ImageStat.Stat(edges)
            edge_mean = edge_stat.mean[0]  # Higher means more edges/lines
            
            # Normalize and Combine
            # Entropy > 5 is usually complex.
            # Edge mean > 50 is usually line-heavy (like charts).
            
            score = (entropy * 0.8) + (edge_mean / 20)
            
            # Cap at 10
            return min(score, 10.0)
        except Exception as e:
            print(f"Complexity calc failed: {e}")
            return 0.0

    def _extract_rules(self, text: str) -> list[ExtractedEntity]:
        """
        TIER 1: Rule-based entity extraction (FREE).
        Uses regex patterns and optionally spaCy NER.
        """
        entities = []
        
        # Email regex
        emails = re.findall(r'\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b', text)
        for email in emails:
            entities.append(ExtractedEntity(
                name=email,
                entity_type="Email",
                source="rule",
            ))
        
        # URL regex
        urls = re.findall(r'https?://[^\s<>"{}|\\^`\[\]]+', text)
        for url in urls[:10]:  # Limit
            entities.append(ExtractedEntity(
                name=url,
                entity_type="URL",
                source="rule",
            ))
        
        # Money amounts
        amounts = re.findall(r'\$[\d,]+(?:\.\d{2})?', text)
        for amount in amounts[:20]:
            entities.append(ExtractedEntity(
                name=amount,
                entity_type="Metric",
                description="Monetary value",
                source="rule",
            ))
        
        # Percentages
        percentages = re.findall(r'\d+(?:\.\d+)?%', text)
        for pct in percentages[:20]:
            entities.append(ExtractedEntity(
                name=pct,
                entity_type="Metric",
                description="Percentage",
                source="rule",
            ))
        
        # Dates (simple pattern)
        dates = re.findall(r'\b\d{1,2}[/-]\d{1,2}[/-]\d{2,4}\b', text)
        for date in dates[:20]:
            entities.append(ExtractedEntity(
                name=date,
                entity_type="Date",
                source="rule",
            ))
        
        # spaCy NER if available
        if HAS_SPACY:
            doc = nlp(text[:100000])  # Limit for performance
            for ent in doc.ents[:50]:  # Limit entities
                if ent.label_ in ("PERSON", "ORG", "GPE", "LOC", "PRODUCT", "EVENT"):
                    entities.append(ExtractedEntity(
                        name=ent.text,
                        entity_type=self._map_spacy_label(ent.label_),
                        source="rule",
                        confidence=0.8,
                    ))
        
        return entities
    
    def _map_spacy_label(self, label: str) -> str:
        """Map spaCy labels to our entity types."""
        mapping = {
            "PERSON": "Person",
            "ORG": "Organization",
            "GPE": "Location",
            "LOC": "Location",
            "PRODUCT": "Concept",
            "EVENT": "Event",
        }
        return mapping.get(label, "Concept")
    
    def _create_chunks(self, pages: list[tuple[int, str]]) -> list[DocumentChunk]:
        """
        Create semantically-aware chunks from document pages.

        Uses memchunk for high-performance semantic chunking (up to 1TB/s).
        Falls back to simple paragraph-based chunking if memchunk unavailable.
        """
        try:
            from .memchunker import MemChunker, ChunkerConfig
            return self._create_chunks_fast(pages)
        except ImportError:
            # Fallback to simple chunking
            return self._create_chunks_simple(pages)

    def _create_chunks_fast(self, pages: list[tuple[int, str]]) -> list[DocumentChunk]:
        """
        Fast chunking using memchunk (SIMD-accelerated).

        Splits at semantic boundaries (periods, newlines) for better retrieval.
        """
        from .memchunker import MemChunker, ChunkerConfig

        chunks = []
        chunk_idx = 0

        # Configure memchunker for semantic chunking
        config = ChunkerConfig(
            chunk_size=self.chunk_size,
            delimiters=b'\n.?!',  # Split at sentence/paragraph boundaries
            prefix_mode=False,  # Delimiter stays with current chunk
            consecutive=True,  # Handle consecutive newlines together
            forward_fallback=True,  # Search forward if no delimiter found
        )
        chunker = MemChunker(config)

        for page_num, text in pages:
            # Skip empty pages
            if not text.strip():
                continue

            # Use memchunk for fast semantic chunking
            chunk_results = chunker.chunk(text)

            for cr in chunk_results:
                chunk_text = cr.text.strip()
                if not chunk_text:
                    continue

                chunks.append(DocumentChunk(
                    text=chunk_text,
                    page_number=page_num,
                    chunk_index=chunk_idx,
                ))
                chunk_idx += 1

        return chunks

    def _create_chunks_simple(self, pages: list[tuple[int, str]]) -> list[DocumentChunk]:
        """
        Simple fallback chunking when memchunk unavailable.

        Uses paragraph-based splitting with overlap.
        """
        chunks = []
        chunk_idx = 0

        for page_num, text in pages:
            # Split by paragraphs first
            paragraphs = text.split('\n\n')
            current_chunk = ""

            for para in paragraphs:
                para = para.strip()
                if not para:
                    continue

                # If adding paragraph exceeds chunk size, save current chunk
                if len(current_chunk) + len(para) > self.chunk_size and current_chunk:
                    chunks.append(DocumentChunk(
                        text=current_chunk.strip(),
                        page_number=page_num,
                        chunk_index=chunk_idx,
                    ))
                    chunk_idx += 1
                    # Keep overlap
                    words = current_chunk.split()
                    overlap_words = words[-self.chunk_overlap:] if len(words) > self.chunk_overlap else []
                    current_chunk = " ".join(overlap_words) + " " + para
                else:
                    current_chunk += " " + para

            # Save remaining text
            if current_chunk.strip():
                chunks.append(DocumentChunk(
                    text=current_chunk.strip(),
                    page_number=page_num,
                    chunk_index=chunk_idx,
                ))
                chunk_idx += 1

        return chunks
    
    async def _extract_with_llm(self, text: str) -> list[ExtractedEntity]:
        """
        TIER 3: Extract entities using LLM.
        """
        prompt = f"""Extract key entities from this text. Return JSON array:
[{{"name": "...", "type": "Person|Organization|Concept|Metric|Location", "description": "..."}}]

Text:
{text[:2000]}

JSON:"""
        
        result = await self.llm_router.extract_json(prompt)
        
        entities = []
        if isinstance(result, list):
            for item in result:
                if isinstance(item, dict) and "name" in item:
                    entities.append(ExtractedEntity(
                        name=item.get("name", ""),
                        entity_type=item.get("type", "Concept"),
                        description=item.get("description", ""),
                        source="llm",
                        confidence=0.9,
                    ))
        
        return entities
    
    async def _extract_with_vision(self, image: ExtractedImage) -> list[ExtractedEntity]:
        """
        Extract entities from image using Vision LLM.
        """
        prompt = """Analyze this image. Extract entities as JSON:
[{"name": "...", "type": "Person|Concept|Metric", "description": "..."}]

Only include clearly visible named entities, metrics, or concepts."""
        
        response = await self.llm_router.generate_vision(
            image_base64=image.image_base64,
            prompt=prompt,
        )
        
        entities = []
        try:
            # Find JSON in response
            match = re.search(r'\[.*\]', response, re.DOTALL)
            if match:
                parsed = json.loads(match.group())
                for item in parsed:
                    if isinstance(item, dict) and "name" in item:
                        entities.append(ExtractedEntity(
                            name=item.get("name", ""),
                            entity_type=item.get("type", "Concept"),
                            description=item.get("description", ""),
                            source="vision",
                            confidence=0.85,
                        ))
        except (json.JSONDecodeError, ValueError):
            pass
        
        return entities


# API endpoint models
from pydantic import BaseModel


class IngestDocumentRequest(BaseModel):
    """Request to ingest a document."""
    content_base64: Optional[str] = None  # Base64-encoded document
    text: Optional[str] = None  # Plain text
    document_type: str = "text"  # text, pdf
    filename: Optional[str] = None  # Original filename (for validation)


class IngestDocumentResponse(BaseModel):
    """Response from document ingestion."""
    entities: list[dict]
    relationships: list[dict]
    chunks: list[dict]
    stats: dict
    summary: str = ""
    vector_tree: Optional[dict] = None
