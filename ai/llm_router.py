"""
LLM Router - Routes requests to appropriate LLM providers.
Supports NVIDIA NIM, OpenAI, Anthropic, and Ollama (local).
"""
import os
from typing import Optional

import httpx


class LLMRouter:
    """Routes LLM requests to the best available provider."""

    def __init__(self):
        raw_key = os.getenv("NVIDIA_API_KEY")
        self.nvidia_key = raw_key.strip() if raw_key else None
        print(f"DEBUG: Loaded NVIDIA_KEY: Present={bool(self.nvidia_key)}, Len={len(self.nvidia_key) if self.nvidia_key else 0}, RawLen={len(raw_key) if raw_key else 0}", flush=True)
        
        # Determine available providers
        self.openai_key = os.getenv("OPENAI_API_KEY")
        self.anthropic_key = os.getenv("ANTHROPIC_API_KEY")
        self.ollama_host = os.getenv("OLLAMA_HOST", "http://localhost:11434")
        print(f"DEBUG: ENV KEYS: {list(os.environ.keys())}", flush=True)
        
        # Determine available providers
        self.providers = []
        if self.nvidia_key:
            self.providers.append("nvidia")
        if self.openai_key:
            self.providers.append("openai")
        if self.anthropic_key:
            self.providers.append("anthropic")
        self.providers.append("ollama")  # Always available locally
        
        self.default_provider = "nvidia" # Forced per user request

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
        
        print(f"DEBUG: Using provider={provider}, model={model}", flush=True)
        
        if provider == "ollama" or provider is None:
            return await self._call_ollama(system_prompt, query, model or "dolphin-llama3")
        elif provider == "nvidia":
            return await self._call_nvidia(system_prompt, query, model or "deepseek-ai/deepseek-v3.1")
        elif provider == "openai":
            return await self._call_openai(system_prompt, query, model or "gpt-4o-mini")
        elif provider == "anthropic":
            return await self._call_anthropic(system_prompt, query, model or "claude-3-haiku-20240307")
        else:
            return await self._call_ollama(system_prompt, query, model or "qwen3:4b")

    def _build_system_prompt(self, context: Optional[str], alerts: list) -> str:
        """Build the system prompt with context and alerts."""
        prompt = (
            "You are a helpful AI assistant with access to the user's personal memory database. "
            "When answering questions, you MUST check the MEMORY CONTEXT section below first. "
            "If the answer is in the MEMORY CONTEXT, use it to answer directly."
        )
        
        if context and context.strip() and "No relevant memories" not in context:
            prompt += f"\n\n### MEMORY CONTEXT (ANSWER FROM THIS!):\n{context}\n### END MEMORY CONTEXT\n\n"
            prompt += "IMPORTANT: The information above is from the user's memory. Use it to answer their question!"
        else:
            prompt += "\n\n### MEMORY CONTEXT:\n(No memories found)\n###\n\n"
            prompt += "Say: 'I don't have that stored yet. Would you like to tell me?'"
        
        if alerts:
            prompt += "\n\nAlerts:\n"
            for alert in alerts:
                prompt += f"- {alert}\n"
        
        print(f"DEBUG SYSTEM PROMPT (first 300 chars): {prompt[:300]}...", flush=True)
        return prompt

    async def _call_nvidia(self, system: str, query: str, model: str) -> str:
        """Call NVIDIA NIM API (OpenAI-compatible)."""
        async with httpx.AsyncClient() as client:
            response = await client.post(
                "https://integrate.api.nvidia.com/v1/chat/completions",
                headers={
                    "Authorization": f"Bearer {self.nvidia_key}",
                    "Content-Type": "application/json",
                },
                json={
                    "model": model,
                    "messages": [
                        {"role": "system", "content": system},
                        {"role": "user", "content": query},
                    ],
                    "max_tokens": 1024,
                    "temperature": 0.7,
                },
                timeout=120.0,
            )
            response.raise_for_status()
            data = response.json()
            content = data["choices"][0]["message"].get("content")
            if content is None:
                # Check for refusal or other fields
                return "I apologize, but I cannot generate a response to this query."
            return content

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
                    timeout=120.0,
                )
                response.raise_for_status()
                result = response.json()
                response_text = result.get("response", "")
                print(f"DEBUG: Ollama Raw Response: {response_text}", flush=True)
                return response_text
            except Exception as e:
                print(f"DEBUG: Ollama connection error: {e}", flush=True)
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
        print(f"DEBUG: extract_json RAW RESPONSE: {repr(response)}")
        print(f"DEBUG: extract_json response type: {type(response)}")
        
        if response is None:
            return {}

        # Try to parse JSON from response
        import json
        import re
        
        # Find first '[' or '{'
        match = re.search(r'[\[\{]', response)
        if not match:
            return {}
            
        start_idx = match.start()
        
        # Robust extraction: Try parsing from end_idx backwards
        # This handles cases where the model appends text or repeats schema containing brackets
        text_to_parse = response[start_idx:]
        found_json = None
        
        # Try to find valid JSON by stripping from the end
        if text_to_parse:
             # Heuristic: Find all closing brackets and try them as end points
             char = response[start_idx]
             closer = "]" if char == "[" else "}"
             
             # Find all occurrences of the closer
             indices = [i for i, c in enumerate(text_to_parse) if c == closer]
             indices.reverse() # Try largest valid block first
             
             for idx in indices:
                 try:
                     candidate = text_to_parse[:idx+1]
                     found_json = json.loads(candidate)
                     return found_json
                 except (json.JSONDecodeError, ValueError):
                     continue

        # Fallback to original logic
        try:
            char = response[start_idx]
            if char == '[':
                end_idx = response.rfind("]") + 1
                return json.loads(response[start_idx:end_idx])
            else:
                end_idx = response.rfind("}") + 1
                return json.loads(response[start_idx:end_idx])
        except (json.JSONDecodeError, ValueError):
            print(f"DEBUG: Failed to parse JSON from: {response}")
            return {}
