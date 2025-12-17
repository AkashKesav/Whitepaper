import httpx
import asyncio

async def test_same_query():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Same query as updated consultation.go (no type filters)
    query = """
    {
        by_activation(func: has(name), first: 50, orderdesc: activation) {
            uid
            dgraph.type
            name
            description
            activation
        }
    }
    """
    
    print("Running same query as consultation.go (no type filters)...")
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        
        nodes = data.get("data", {}).get("by_activation", [])
        
        print(f"Total nodes: {len(nodes)}")
        
        # Find gulab jamun
        for i, node in enumerate(nodes):
            name = node.get('name', '')
            if 'gulab' in name.lower():
                print(f"\n=== FOUND GULAB JAMUN at position {i+1} ===")
                print(f"  Name: {name}")
                print(f"  Activation: {node.get('activation')}")
                return
        
        print("\nGulab Jamun NOT FOUND in top 50 by activation!")
        print("\nTop 10 nodes:")
        for node in nodes[:10]:
            print(f"  [{node.get('activation', 0)}] {node.get('name')}")

if __name__ == "__main__":
    asyncio.run(test_same_query())
