import httpx
import asyncio
import json
import os

async def check_platinum():
    # Helper to check localhost:8180 (mapped to 8080)
    DGRAPH_URL = os.getenv("DGRAPH_URL", "http://localhost:8180/query")
    
    query = """
    {
      platinum(func: eq(name, "Platinum")) {
        uid
        name
        dgraph.type
        description
        attributes
        created_at
      }
    }
    """
    
    print(f"Querying DGraph at: {DGRAPH_URL}")
    async with httpx.AsyncClient() as client:
        try:
            resp = await client.post(DGRAPH_URL, json={"query": query})
            resp.raise_for_status()
            data = resp.json()
            
            nodes = data.get("data", {}).get("platinum", [])
            if nodes:
                print("\nFOUND PLATINUM IN DGRAPH:")
                print(json.dumps(nodes, indent=2))
            else:
                print("\nPLATINUM NOT FOUND.")
                
        except Exception as e:
            print(f"ERROR: {e}")

if __name__ == "__main__":
    asyncio.run(check_platinum())
