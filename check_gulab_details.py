import httpx
import asyncio

async def check_gulab_details():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Get full details of gulab jamun entity
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
    }
    """
    
    print("Getting full details of 'gulab jamun' entity...")
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        
        search_results = data.get("data", {}).get("search", [])
        
        if search_results:
            for node in search_results:
                print(f"\n=== ENTITY DETAILS ===")
                print(f"  UID: {node.get('uid')}")
                print(f"  Name: {node.get('name')}")
                print(f"  Type: {node.get('dgraph.type')}")
                print(f"  Description: {node.get('description')}")
                print(f"  Tags: {node.get('tags')}")
                print(f"  Activation: {node.get('activation')}")
                print(f"  Created: {node.get('created_at')}")
        else:
            print("  No results found!")

if __name__ == "__main__":
    asyncio.run(check_gulab_details())
