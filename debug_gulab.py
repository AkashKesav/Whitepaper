import httpx
import asyncio

async def debug_gulab():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Direct search with no filters
    query = """
    {
        exact(func: eq(name, "Gulab Jamun")) {
            uid
            dgraph.type
            name
            description
            activation
            created_at
        }
        
        by_type(func: type(Preference), first: 10) {
            uid
            name
            dgraph.type
            activation
        }
    }
    """
    
    print("Debugging gulab jamun...")
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        
        print(f"\n=== EXACT MATCH ===")
        exact = data.get("data", {}).get("exact", [])
        for node in exact:
            print(f"  {node}")
        
        print(f"\n=== ALL PREFERENCE TYPES ===")
        by_type = data.get("data", {}).get("by_type", [])
        for node in by_type:
            print(f"  {node.get('name')}: activation={node.get('activation')}")

if __name__ == "__main__":
    asyncio.run(debug_gulab())
