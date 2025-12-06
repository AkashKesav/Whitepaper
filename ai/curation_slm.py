"""
Curation SLM - Resolves contradictions between facts.
"""
from llm_router import LLMRouter


class CurationSLM:
    """Logic-focused model for contradiction resolution."""

    def __init__(self, router: LLMRouter):
        self.router = router

    async def resolve(self, node1: dict, node2: dict) -> dict:
        """Determine which of two contradicting facts is more reliable."""
        
        prompt = f"""You are a fact verification expert. Two facts appear to contradict each other.

Fact 1:
- Name: {node1['name']}
- Description: {node1['description']}
- Created: {node1['created_at']}

Fact 2:
- Name: {node2['name']}
- Description: {node2['description']}
- Created: {node2['created_at']}

Determine which fact should be kept as current. Consider:
1. More recent information usually supersedes older
2. More specific information is more reliable
3. Direct statements override implications

Return JSON:
{{"winner_index": 1 or 2, "reason": "brief explanation"}}"""

        result = await self.router.extract_json(prompt)
        
        if result and "winner_index" in result:
            return result
        
        # Default to newer fact
        return {"winner_index": 2, "reason": "newer_timestamp"}
