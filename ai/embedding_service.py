"""
Embedding service for semantic search.
Uses Ollama's embedding models for vector generation.
"""
import os
from typing import Optional
import httpx


class EmbeddingService:
    """Generates embeddings using Ollama's embedding models."""
    
    def __init__(self):
        self.ollama_host = os.getenv("OLLAMA_HOST", "http://localhost:11434")
        self.model = "nomic-embed-text"  # Fast, high-quality embedding model
    
    async def get_embedding(self, text: str) -> list[float]:
        """Generate embedding vector for text."""
        async with httpx.AsyncClient(timeout=60.0) as client:
            try:
                response = await client.post(
                    f"{self.ollama_host}/api/embeddings",
                    json={
                        "model": self.model,
                        "prompt": text,
                    },
                )
                response.raise_for_status()
                data = response.json()
                return data.get("embedding", [])
            except Exception as e:
                print(f"Embedding error: {e}", flush=True)
                return []
    
    async def get_embeddings_batch(self, texts: list[str]) -> list[list[float]]:
        """Generate embeddings for multiple texts."""
        embeddings = []
        for text in texts:
            emb = await self.get_embedding(text)
            embeddings.append(emb)
        return embeddings
    
    def cosine_similarity(self, vec1: list[float], vec2: list[float]) -> float:
        """Calculate cosine similarity between two vectors."""
        if not vec1 or not vec2 or len(vec1) != len(vec2):
            return 0.0
        
        dot_product = sum(a * b for a, b in zip(vec1, vec2))
        norm1 = sum(a * a for a in vec1) ** 0.5
        norm2 = sum(b * b for b in vec2) ** 0.5
        
        if norm1 == 0 or norm2 == 0:
            return 0.0
        
        return dot_product / (norm1 * norm2)
    
    def find_most_similar(
        self, 
        query_embedding: list[float], 
        candidates: list[dict],  # {"text": str, "embedding": list[float], "data": any}
        top_k: int = 5,
        threshold: float = 0.5
    ) -> list[dict]:
        """Find most similar candidates to query embedding."""
        scored = []
        for candidate in candidates:
            if "embedding" in candidate and candidate["embedding"]:
                score = self.cosine_similarity(query_embedding, candidate["embedding"])
                if score >= threshold:
                    scored.append({**candidate, "similarity": score})
        
        # Sort by similarity descending
        scored.sort(key=lambda x: x["similarity"], reverse=True)
        return scored[:top_k]
