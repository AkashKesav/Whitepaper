import httpx
import asyncio
import json
import os

async def query_latest():
    # DGRAPH_URL = os.getenv("DGRAPH_URL", "http://rmk-dgraph-alpha:8080/query")
    # For local testing from host:
    DGRAPH_URL = os.getenv("DGRAPH_URL", "http://localhost:8180/query")
    query = """
    {
      latest(func: type(Entity), orderdesc: created_at, first: 5) {
        name
        dgraph.type
        created_at
        description
      }
    }
    """
    
    print(f"Querying DGraph at: {DGRAPH_URL}")
    async with httpx.AsyncClient() as client:
        try:
            resp = await client.post(DGRAPH_URL, json={"query": query})
            resp.raise_for_status()
            data = resp.json()
            
            entities = data.get("data", {}).get("latest", [])
            print(f"\nFound {len(entities)} entities:")
            for e in entities:
                print(f"- [{e.get('created_at')}] {e.get('name')} ({e.get('dgraph.type')})")
                
        except Exception as e:
            print(f"ERROR: {e}")

if __name__ == "__main__":
    asyncio.run(query_latest())
