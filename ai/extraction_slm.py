"""
Extraction SLM - Extracts structured entities from conversations.
"""
from typing import Optional

from llm_router import LLMRouter


class ExtractionSLM:
    """Lightweight extraction model for entity and relationship extraction.
    
    Uses a small, fast local model (qwen3:1.7b via Ollama) for quick extraction.
    This is much faster than using large cloud models like DeepSeek.
    """

    def __init__(self, router: LLMRouter):
        self.router = router
        # Use Ollama with small fast model for extraction
        self.provider = "ollama"
        self.model = "qwen3:4b"  # Higher quality extraction model

    async def extract(
        self,
        user_query: str,
        ai_response: str,
        context: Optional[str] = None,
    ) -> list:
        """Extract entities and relationships from a conversation turn."""
        
        # Richer prompt for 4b model
        preamble = f"""You are a smart knowledge extraction engine.
TASK: Analyze the user's input and the AI's response to extract meaningful entities, facts, or preferences.
CONTEXT: Building a long-term memory graph for the user.

User Input: "{user_query}"
AI Response: "{ai_response}"

INSTRUCTIONS:
1. Identify proper nouns (People, Places, Organizations, Products).
2. Identify specific user preferences ("I like...", "My favorite...").
3. Identify stated facts ("My sister is Emma").
4. IGNORE generic chit-chat.
5. Output ONLY a JSON array.

Schema:
[
  {{
    "name": "Exact Name (e.g. 'Platinum', 'Emma')",
    "type": "Entity|Fact|Preference",
    "description": "Contextual description (e.g. 'User's favorite metal')",
    "tags": ["keyword1", "keyword2"],
    "relations": []
  }}
]

Output:"""
        prompt = preamble

        result = await self.router.extract_json(prompt, provider=self.provider, model=self.model)
        
        if isinstance(result, list):
            return result
        return []
