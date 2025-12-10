import httpx
import asyncio

async def check_gulab_jamun():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Search for gulab jamun
    query = """
    {
      search(func: anyoftext(name, "gulab jamun")) {
        uid
        name
        dgraph.type
        description
        tags
        activation
        created_at
      }
      recent(func: has(name), first: 10, orderdesc: created_at) @filter(NOT type(User)) {
        uid
        name
        dgraph.type
        description
        created_at
      }
    }
    """
    
    print("Searching DGraph for 'gulab jamun'...")
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        
        search_results = data.get("data", {}).get("search", [])
        recent = data.get("data", {}).get("recent", [])
        
        print(f"\n=== SEARCH RESULTS FOR 'gulab jamun' ===")
        if search_results:
            for node in search_results:
                print(f"  - {node.get('name')}: {node.get('description')}")
        else:
            print("  No results found!")
        
        print(f"\n=== 10 MOST RECENT ENTITIES ===")
        for node in recent:
            print(f"  - {node.get('name')}: {node.get('description', '')[:50]}")

if __name__ == "__main__":
    asyncio.run(check_gulab_jamun())
