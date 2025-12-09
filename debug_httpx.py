import httpx
import asyncio
import os

async def test():
    url = "http://rmk-ollama:11434/api/generate"
    print(f"Testing generation on: {url}")
    try:
        async with httpx.AsyncClient() as client:
            resp = await client.post(
                url, 
                json={
                    "model": "qwen2:0.5b", 
                    "prompt": """You are a JSON formatting script. You have no personality.
TASK: Convert the text below into the specified JSON format.
CONVERT THIS: "My favorite movie is Inception."
SCHEMA:
[
  {
    "name": "THE ACTUAL VALUE",
    "description": "context",
    "type": "Entity|Fact|Event|Preference",
    "tags": ["tag1", "tag2"]
  }
]
Return ONLY the JSON array.""", 
                    "stream": False
                },
                timeout=120.0
            )
            print(f"Status Code: {resp.status_code}")
            import json
            try:
                data = resp.json()
                print(f"EXTRACTED RESPONSE: {data.get('response', 'NO_RESPONSE_FIELD')}")
            except:
                print(f"RAW RESPONSE: {resp.text}")
    except Exception as e:
        print(f"ERROR: {type(e).__name__}: {e}")

if __name__ == "__main__":
    asyncio.run(test())
