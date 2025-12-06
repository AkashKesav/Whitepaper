"""
LLM Router - Routes requests to appropriate LLM providers.
Supports OpenAI, Anthropic, and Ollama (local).
"""
import os
from typing import Optional

import httpx


class LLMRouter:
    """Routes LLM requests to the best available provider."""

    def __init__(self):
        self.openai_key = os.getenv("OPENAI_API_KEY")
        self.anthropic_key = os.getenv("ANTHROPIC_API_KEY")
        self.ollama_host = os.getenv("OLLAMA_HOST", "http://localhost:11434")
        
        # Determine available providers
        self.providers = []
        if self.openai_key:
            self.providers.append("openai")
        if self.anthropic_key:
            self.providers.append("anthropic")
        self.providers.append("ollama")  # Always available locally
        
        self.default_provider = self.providers[0] if self.providers else "ollama"

    async def generate(
        self,
        query: str,
        context: Optional[str] = None,
        alerts: list = None,
        provider: str = None,
        model: str = None,
    ) -> str:
        """Generate a response using the specified or default provider."""
        provider = provider or self.default_provider
        alerts = alerts or []

        # Build the prompt
        system_prompt = self._build_system_prompt(context, alerts)
        
        if provider == "openai":
            return await self._call_openai(system_prompt, query, model or "gpt-4o-mini")
        elif provider == "anthropic":
            return await self._call_anthropic(system_prompt, query, model or "claude-3-haiku-20240307")
        else:
            return await self._call_ollama(system_prompt, query, model or "llama3.2")

    def _build_system_prompt(self, context: Optional[str], alerts: list) -> str:
        """Build the system prompt with context and alerts."""
        prompt = "You are a helpful AI assistant with access to the user's memory and context."
        
        if context:
            prompt += f"\n\nRelevant context from memory:\n{context}"
        
        if alerts:
            prompt += "\n\nProactive alerts to consider:\n"
            for alert in alerts:
                prompt += f"- {alert}\n"
        
        return prompt

    async def _call_openai(self, system: str, query: str, model: str) -> str:
        """Call OpenAI API."""
        async with httpx.AsyncClient() as client:
            response = await client.post(
                "https://api.openai.com/v1/chat/completions",
                headers={"Authorization": f"Bearer {self.openai_key}"},
                json={
                    "model": model,
                    "messages": [
                        {"role": "system", "content": system},
                        {"role": "user", "content": query},
                    ],
                    "max_tokens": 1000,
                },
                timeout=30.0,
            )
            response.raise_for_status()
            data = response.json()
            return data["choices"][0]["message"]["content"]

    async def _call_anthropic(self, system: str, query: str, model: str) -> str:
        """Call Anthropic API."""
        async with httpx.AsyncClient() as client:
            response = await client.post(
                "https://api.anthropic.com/v1/messages",
                headers={
                    "x-api-key": self.anthropic_key,
                    "anthropic-version": "2023-06-01",
                },
                json={
                    "model": model,
                    "max_tokens": 1000,
                    "system": system,
                    "messages": [{"role": "user", "content": query}],
                },
                timeout=30.0,
            )
            response.raise_for_status()
            data = response.json()
            return data["content"][0]["text"]

    async def _call_ollama(self, system: str, query: str, model: str) -> str:
        """Call Ollama (local) API."""
        async with httpx.AsyncClient() as client:
            try:
                response = await client.post(
                    f"{self.ollama_host}/api/generate",
                    json={
                        "model": model,
                        "prompt": f"{system}\n\nUser: {query}\n\nAssistant:",
                        "stream": False,
                    },
                    timeout=60.0,
                )
                response.raise_for_status()
                data = response.json()
                return data.get("response", "I couldn't generate a response.")
            except Exception:
                return "I'm sorry, I couldn't connect to the local model. Please ensure Ollama is running."

    async def extract_json(
        self,
        prompt: str,
        provider: str = None,
        model: str = None,
    ) -> dict:
        """Generate and parse JSON output."""
        response = await self.generate(
            query=prompt,
            provider=provider,
            model=model,
        )
        
        # Try to parse JSON from response
        import json
        try:
            # Find JSON in response
            start = response.find("{")
            end = response.rfind("}") + 1
            if start >= 0 and end > start:
                return json.loads(response[start:end])
        except json.JSONDecodeError:
            pass
        
        # Try array
        try:
            start = response.find("[")
            end = response.rfind("]") + 1
            if start >= 0 and end > start:
                return json.loads(response[start:end])
        except json.JSONDecodeError:
            pass
        
        return {}
