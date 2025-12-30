"""Test script to debug LLMRouter initialization in FastAPI context"""
import os
from pathlib import Path

# Load dotenv first (simulate lines 11-14 in main.py)
from dotenv import load_dotenv
env_path = Path(__file__).parent.parent / '.env'
load_dotenv(dotenv_path=env_path)
print(f'After load_dotenv: NVIDIA_API_KEY present = {bool(os.getenv("NVIDIA_API_KEY"))}')
print(f'API key starts with: {os.getenv("NVIDIA_API_KEY", "")[:20]}')

# Now import LLMRouter (like line 22 in main.py)
from llm_router import LLMRouter

# Create instance (like line 30 in main.py)
router = LLMRouter()
print(f'LLMRouter.nvidia_key present = {bool(router.nvidia_key)}')
print(f'LLMRouter.nvidia_key length = {len(router.nvidia_key) if router.nvidia_key else 0}')
print(f'LLMRouter.default_provider = {router.default_provider}')

# Test a call
import asyncio
async def test_call():
    try:
        result = await router.generate("Hello, say hi")
        print(f'RESULT: {result[:200]}')
    except Exception as e:
        print(f'ERROR: {e}')

asyncio.run(test_call())
