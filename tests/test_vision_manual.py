import asyncio
import os
import sys

# Add project root to path
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'ai')))

from dotenv import load_dotenv
# Load .env from project root
root_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
env_path = os.path.join(root_dir, '.env')
load_dotenv(env_path)
print(f"DEBUG: Loaded .env from {env_path}")

from llm_router import LLMRouter

async def test_vision():
    print("--- Testing Vision Support via NVIDIA NIM ---")
    
    # 1. Check Env
    nvidia_key = os.getenv("NVIDIA_API_KEY")
    if not nvidia_key:
        print("WARNING: NVIDIA_API_KEY not found in environment.")
        print("Please run: export NVIDIA_API_KEY=nvapi-...")
        # We continue to verify structure, but call will fail
    
    # 2. Init Router
    router = LLMRouter()
    
    # 3. Create Sample Image (1x1 Red Pixel PNG)
    # iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKwAEQAAAABJRU5ErkJggg==
    sample_image = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKwAEQAAAABJRU5ErkJggg=="
    
    prompt = "Describe this image. Is it red?"
    
    print(f"\nSending request to NVIDIA NIM (Model: minimaxai/minimax-m2)...")
    try:
        response = await router.generate_vision(
            image_base64=sample_image,
            prompt=prompt,
            model="minimaxai/minimax-m2" 
        )
        print(f"\nResponse:\n{response}")
        print("\nSUCCESS: Vision API call completed.")
    except Exception as e:
        print(f"\nFAILURE: {e}")

if __name__ == "__main__":
    asyncio.run(test_vision())
