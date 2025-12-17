"""
Synthesis SLM - Synthesizes insights and briefs from facts.
"""
from typing import Optional

from llm_router import LLMRouter


class SynthesisSLM:
    """Advanced analytical model for insight generation."""

    def __init__(self, router: LLMRouter):
        self.router = router
        # Use Kimi K2 for superior long-context reasoning and synthesis
        self.provider = "nvidia"
        self.model = "moonshotai/kimi-k2-instruct-0905"  # Kimi K2 for deep reasoning

    async def synthesize(
        self,
        query: str,
        context: Optional[str],
        facts: list,
        insights: list,
        alerts: list,
    ) -> dict:
        """Synthesize a coherent brief from facts and insights."""
        
        # Format facts more comprehensively - use all available fields
        def format_fact(f):
            name = f.get('name', '')
            desc = f.get('description', '')
            node_type = f.get('dgraph.type', f.get('type', ''))
            attrs = f.get('attributes', {})
            
            # Build a comprehensive fact string
            parts = [f"- {name}"]
            
            if desc:
                parts.append(f": {desc}")
            elif node_type:
                # Use type as context if no description
                if isinstance(node_type, list):
                    node_type = node_type[0] if node_type else ''
                parts.append(f" ({node_type})")
            
            # Include attributes for additional context
            if attrs and isinstance(attrs, dict):
                attr_str = ", ".join([f"{k}={v}" for k, v in attrs.items() if v])
                if attr_str:
                    parts.append(f" [{attr_str}]")
            
            return "".join(parts)
        
        # Handle None values
        facts = facts or []
        insights = insights or []
        alerts = alerts or []
        
        facts_text = "\n".join([format_fact(f) for f in facts[:10]])
        insights_text = "\n".join([f"- {i.get('summary', '')}" for i in insights[:5]])
        alerts_text = "\n".join([f"- {a}" for a in alerts[:3]])

        prompt = f"""You are a memory retrieval system. Your ONLY job is to answer questions using the KNOWN FACTS below.

CRITICAL RULES:
1. If facts are provided, you MUST use them to answer
2. NEVER say "I don't have information" if facts are available
3. Quote the facts directly in your answer
4. If no facts match the query, say "I don't have that stored yet"

Query: {query}

=== KNOWN FACTS (USE THESE!) ===
{facts_text if facts_text else "No facts stored."}

=== INSIGHTS ===
{insights_text if insights_text else "None."}

=== ALERTS ===
{alerts_text if alerts_text else "None."}

EXAMPLE:
- If facts say "Bob: user's manager" and query is "Who is my manager?"
- Your answer MUST be: "Your manager is Bob."

Now answer the query using the facts above.

Return JSON:
{{"brief": "your answer using the facts", "confidence": 0.0-1.0}}"""

        result = await self.router.extract_json(prompt, provider=self.provider, model=self.model)
        
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

        result = await self.router.extract_json(prompt, provider=self.provider, model=self.model)
        
        if result and "has_insight" in result:
            return result
        
        return {"has_insight": False, "insight_type": "", "summary": "", "action_suggestion": "", "confidence": 0.0}
