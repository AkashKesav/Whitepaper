"""
Synthesis SLM - Synthesizes insights and briefs from facts.
"""
from typing import Optional

from llm_router import LLMRouter


class SynthesisSLM:
    """Advanced analytical model for insight generation."""

    def __init__(self, router: LLMRouter):
        self.router = router

    async def synthesize(
        self,
        query: str,
        context: Optional[str],
        facts: list,
        insights: list,
        alerts: list,
    ) -> dict:
        """Synthesize a coherent brief from facts and insights."""
        
        facts_text = "\n".join([f"- {f.get('name', '')}: {f.get('description', '')}" for f in facts[:10]])
        insights_text = "\n".join([f"- {i.get('summary', '')}" for i in insights[:5]])
        alerts_text = "\n".join([f"- {a}" for a in alerts[:3]])

        prompt = f"""Create a concise, synthesized response to this query using the available context.

Query: {query}

Known Facts:
{facts_text or "No specific facts available."}

Insights:
{insights_text or "No specific insights."}

Proactive Alerts:
{alerts_text or "None."}

Create a brief that:
1. Directly answers the query
2. Incorporates relevant facts naturally
3. Mentions any important alerts if relevant
4. Is conversational and helpful

Return JSON:
{{"brief": "your synthesized response", "confidence": 0.0-1.0}}"""

        result = await self.router.extract_json(prompt)
        
        if result and "brief" in result:
            return result
        
        return {"brief": "I can help with that, but I don't have specific information.", "confidence": 0.3}

    async def evaluate_connection(
        self,
        node1: dict,
        node2: dict,
        path_exists: bool,
        path_length: int,
    ) -> dict:
        """Evaluate if two nodes have an emergent insight."""
        
        prompt = f"""Analyze if these two pieces of information have a meaningful, non-obvious connection.

Item 1: {node1.get('name', '')} ({node1.get('type', '')})
Description: {node1.get('description', 'No description')}

Item 2: {node2.get('name', '')} ({node2.get('type', '')})
Description: {node2.get('description', 'No description')}

Already connected: {path_exists} (path length: {path_length})

Look for:
1. Potential conflicts (allergies vs food preferences)
2. Hidden dependencies
3. Causal relationships
4. Temporal patterns

Return JSON:
{{
  "has_insight": true/false,
  "insight_type": "warning|opportunity|dependency|pattern",
  "summary": "brief description of the insight",
  "action_suggestion": "what to do about it",
  "confidence": 0.0-1.0
}}"""

        result = await self.router.extract_json(prompt)
        
        if result and "has_insight" in result:
            return result
        
        return {"has_insight": False, "insight_type": "", "summary": "", "action_suggestion": "", "confidence": 0.0}
