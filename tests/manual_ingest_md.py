import asyncio
import os
import sys
from dotenv import load_dotenv

# Path setup
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'ai')))
# Load Env
root_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
load_dotenv(os.path.join(root_dir, '.env'))

from document_ingester import DocumentIngester
from llm_router import LLMRouter

async def test_md():
    print("--- Manual MD Ingestion Test ---")
    
    doc_path = os.path.join(root_dir, "test_doc.md")
    if not os.path.exists(doc_path):
        print(f"ERROR: {doc_path} not found.")
        return

    with open(doc_path, "r") as f:
        text = f.read()
            
    print(f"Loaded {len(text)} chars from test_doc.md")
    
    router = LLMRouter()
    ingester = DocumentIngester(router)
    
    print("Ingesting Text (Calling NVIDIA NIM)...")
    try:
        result = await ingester.ingest_text(text)
        
        print("\n--- Ingestion Results ---")
        print(f"Chunks: {len(result.chunks)}")
        print(f"Entities: {len(result.entities)}")
        
        for e in result.entities:
            print(f" - {e.name} ({e.entity_type}) [conf={e.confidence}]: {e.description}")
            
    except Exception as e:
        print(f"ERROR: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    asyncio.run(test_md())
