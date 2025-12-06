"""
Extraction SLM - Extracts structured entities from conversations.
"""
from typing import Optional

from llm_router import LLMRouter


class ExtractionSLM:
    """Lightweight extraction model for entity and relationship extraction."""

    def __init__(self, router: LLMRouter):
        self.router = router

    async def extract(
        self,
        user_query: str,
        ai_response: str,
        context: Optional[str] = None,
    ) -> list:
        """Extract entities and relationships from a conversation turn."""
        
        prompt = f"""Analyze this conversation and extract structured entities.

User said: "{user_query}"
AI responded: "{ai_response}"

Extract entities in this JSON format:
[
  {{
    "name": "entity name",
    "type": "Entity|Fact|Event|Preference",
    "attributes": {{"key": "value"}},
    "relations": [
      {{
        "type": "RELATION_TYPE",
        "target_name": "related entity",
        "target_type": "Entity|Fact"
      }}
    ]
  }}
]

Relation types: PARTNER_IS, FAMILY_MEMBER, HAS_MANAGER, WORKS_ON, WORKS_AT, LIKES, DISLIKES, IS_ALLERGIC_TO, PREFERS, HAS_INTEREST

Return ONLY the JSON array, no explanation."""

        result = await self.router.extract_json(prompt)
        
        if isinstance(result, list):
            return result
        return []
