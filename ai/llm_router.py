"""
LLM Router - Routes requests to appropriate LLM providers.
Supports NVIDIA NIM, OpenAI, and Anthropic.
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
        self.minimax_key = os.getenv("MINIMAX_API_KEY")
        print(f"DEBUG: ENV KEYS: {list(os.environ.keys())}", flush=True)
        
        # Determine available providers
        self.providers = []
        if self.nvidia_key:
            self.providers.append("nvidia")
        if self.openai_key:
            self.providers.append("openai")
        if self.anthropic_key:
            self.providers.append("anthropic")
        if self.minimax_key:
            self.providers.append("minimax")
        
        self.default_provider = "nvidia" # Forced per user request

    async def generate(
        self,
        query: str,
        context: Optional[str] = None,
        alerts: list = [],
        provider: str = None,
        model: str = None,
        format: str = None,
        system_instruction: str = None,
    ) -> str:
        """Route query to the configured LLM provider."""
        # Use default provider if none specified
        provider = provider or self.default_provider
        
        # Use provided system instruction or build default one
        system = system_instruction or self._build_system_prompt(context, alerts)
        
        print(f"DEBUG: Using provider={provider}, model={model}", flush=True)
        
        if provider == "nvidia":
            return await self._call_nvidia(system, query, model or "deepseek-ai/deepseek-v3.2")
        elif provider == "openai":
            return await self._call_openai(system, query, model or "gpt-4o-mini")
        elif provider == "anthropic":
            return await self._call_anthropic(system, query, model or "claude-3-haiku-20240307")
        else:
            # Fallback to NVIDIA if unknown
            return await self._call_nvidia(system, query, model or "deepseek-ai/deepseek-v3.2")

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

    async def generate_vision(
        self,
        image_base64: str,
        prompt: str,
        model: str = "minimaxai/minimax-m2",
    ) -> str:
        """
        Generate response from vision-capable model with image input.
        Uses MiniMax M2 which supports multimodal (text + image) input.
        
        Args:
            image_base64: Base64-encoded image data
            prompt: Text prompt describing what to extract
            model: Vision-capable model (default: MiniMax M2)
        
        Returns:
            Model response text
        """
        # Priority: NVIDIA NIM (as requested) -> Minimax Direct
        # If NVIDIA_API_KEY is present, try to use it for Minimax-M2 or similar
        if self.nvidia_key:
             try:
                 return await self._call_nvidia_vision(prompt, image_base64, model)
             except Exception as e:
                 print(f"DEBUG: NVIDIA Vision failed: {e}. Trying fallback...", flush=True)

        # Fallback to Minimax Direct if configured and NVIDIA failed/missing
        if self.minimax_key:
             return await self._call_minimax_vision(prompt, image_base64)
             
        return "No vision provider configured."
    async def _call_nvidia_vision(self, prompt: str, image_base64: str, model: str) -> str:
        """Call NVIDIA NIM API for Vision."""
        timeout = httpx.Timeout(180.0, connect=30.0, read=180.0, write=30.0)
        async with httpx.AsyncClient(timeout=timeout) as client:
            response = await client.post(
                "https://integrate.api.nvidia.com/v1/chat/completions",
                headers={
                    "Authorization": f"Bearer {self.nvidia_key}",
                    "Content-Type": "application/json",
                },
                json={
                    "model": model, # e.g. "minimaxai/minimax-m2"
                    "messages": [{
                        "role": "user",
                        "content": [
                            {"type": "text", "text": prompt},
                            {"type": "image_url", "image_url": {
                                "url": f"data:image/jpeg;base64,{image_base64}"
                            }}
                        ]
                    }],
                    "max_tokens": 2048,
                    "temperature": 0.3,
                },
            )
            response.raise_for_status()
            data = response.json()
            content = data["choices"][0]["message"].get("content")
            if content is None:
                return "Unable to analyze image."
            
            # Strip thinking tags if present
            import re
            content = re.sub(r'<think>.*?</think>', '', content, flags=re.DOTALL).strip()
            return content

    async def _call_minimax_vision(self, prompt: str, image_base64: str) -> str:
        """Call Minimax API for Vision (abab6.5-chat)."""
        print("DEBUG: Calling Minimax Vision API...", flush=True)
        timeout = httpx.Timeout(180.0, connect=30.0, read=180.0, write=30.0)
        async with httpx.AsyncClient(timeout=timeout) as client:
            response = await client.post(
                "https://api.minimax.chat/v1/text/chatcompletion_pro",
                headers={
                    "Authorization": f"Bearer {self.minimax_key}",
                    "Content-Type": "application/json",
                },
                json={
                    "model": "abab6.5-chat",
                    "messages": [{
                        "sender_type": "USER",
                        "sender_name": "User",
                        "text": prompt,
                        "images": [image_base64] # Minimax expects list of base64 strings? Check docs if failure.
                    }],
                    "botsetting": [
                        {
                            "bot_name": "MM Assistant",
                            "content": "You are a helpful assistant capable of analyzing images."
                        }
                    ],
                    "reply_constraints": {"sender_type": "BOT", "sender_name": "MM Assistant"},
                    "temperature": 0.1,
                    "tokens_to_generate": 2048,
                },
            )
            
            # If standard endpoint fails, try OpenAI-compatible path
            if response.status_code != 200:
                 print(f"DEBUG: Minimax Native failed ({response.status_code}), trying OpenAI compat...", flush=True)
                 return await self._call_minimax_openai_compat(prompt, image_base64)
                 
            data = response.json()
            return data["reply"]

    async def _call_minimax_openai_compat(self, prompt: str, image_base64: str) -> str:
        """Call Minimax via OpenAI-compatible endpoint."""
        async with httpx.AsyncClient(timeout=60.0) as client:
            response = await client.post(
                "https://api.minimax.chat/v1/chat/completions", # Guessing endpoint
                headers={
                    "Authorization": f"Bearer {self.minimax_key}",
                    "Content-Type": "application/json",
                },
                json={
                    "model": "abab6.5-chat",
                    "messages": [{
                        "role": "user",
                        "content": [
                            {"type": "text", "text": prompt},
                            {"type": "image_url", "image_url": {
                                "url": f"data:image/jpeg;base64,{image_base64}"
                            }}
                        ]
                    }],
                }
            )
            response.raise_for_status()
            return response.json()["choices"][0]["message"]["content"]

    async def _call_nvidia(self, system: str, query: str, model: str) -> str:
        """Call NVIDIA NIM API (OpenAI-compatible)."""
        # Use explicit timeout to prevent ReadTimeout on slow LLM responses
        timeout = httpx.Timeout(180.0, connect=30.0, read=180.0, write=30.0)
        async with httpx.AsyncClient(timeout=timeout) as client:
            response = await client.post(
                "https://integrate.api.nvidia.com/v1/chat/completions",
                headers={
                    "Authorization": f"Bearer {self.nvidia_key}",
                    "Content-Type": "application/json",
                },
                json={
                    "model": model,
                    "messages": [
                        {"role": "system", "content": "You are a helpful AI assistant."},
                        {"role": "user", "content": f"{system}\n\nUser Question: {query}"},
                    ],
                    "max_tokens": 1024,
                    "temperature": 0.7,
                },
            )
            response.raise_for_status()
            data = response.json()
            content = data["choices"][0]["message"].get("content")
            if content is None:
                # Check for refusal or other fields
                return "I apologize, but I cannot generate a response to this query."
            
            # Strip thinking tags from MiniMax-M2 responses
            import re
            content = re.sub(r'<think>.*?</think>', '', content, flags=re.DOTALL).strip()
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
            format="json",
            system_instruction="You are a precise entity extraction engine. Output JSON only.",
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


# Module-level singleton and helper function for easy import
_router_instance = None

def _get_router():
    global _router_instance
    if _router_instance is None:
        _router_instance = LLMRouter()
    return _router_instance

async def get_llm_response(prompt: str, max_tokens: int = 300) -> str:
    """Module-level helper for LLM generation."""
    router = _get_router()
    return await router.generate(query=prompt, system_instruction="You are a helpful summarization assistant.")
