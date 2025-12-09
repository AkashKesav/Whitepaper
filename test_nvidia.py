import os
import asyncio
import httpx
from dotenv import load_dotenv

load_dotenv()

async def test():
    key = os.getenv("NVIDIA_API_KEY")
    print(f"Key found: {'Yes' if key else 'No'}")
    if not key:
        return

    print("Testing DeepSeek V3 via NVIDIA...")
    async with httpx.AsyncClient() as client:
        try:
            response = await client.post(
                "https://integrate.api.nvidia.com/v1/chat/completions",
                headers={
                    "Authorization": f"Bearer {key}",
                    "Content-Type": "application/json",
                },
                json={
                    "model": "deepseek-ai/deepseek-v3.1",
                    "messages": [{"role": "user", "content": "Say Hello"}],
                    "max_tokens": 10,
                },
                timeout=10.0,
            )
            print(f"Status: {response.status_code}")
            print(f"Response: {response.text}")
        except Exception as e:
            print(f"Error: {e}")

if __name__ == "__main__":
    asyncio.run(test())
