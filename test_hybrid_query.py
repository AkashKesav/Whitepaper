import httpx
import asyncio

async def test_hybrid_query():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Same query as consultation.go
    query = """
    {
        by_activation(func: has(name), first: 30, orderdesc: activation) @filter(NOT type(User) AND NOT type(Conversation)) {
            uid
            dgraph.type
            name
            description
            activation
        }
        by_recency(func: has(name), first: 30, orderdesc: created_at) @filter(NOT type(User) AND NOT type(Conversation)) {
            uid
            dgraph.type
            name
            description
            activation
        }
    }
    """
    
    print("Running hybrid query (same as consultation.go)...")
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        
        by_activation = data.get("data", {}).get("by_activation", [])
        by_recency = data.get("data", {}).get("by_recency", [])
        
        print(f"\n=== BY ACTIVATION (top 10 of {len(by_activation)}) ===")
        for node in by_activation[:10]:
            print(f"  [{node.get('activation', 0):.2f}] {node.get('name')}: {node.get('dgraph.type')}")
        
        print(f"\n=== BY RECENCY (top 10 of {len(by_recency)}) ===")
        for node in by_recency[:10]:
            print(f"  {node.get('name')}: {node.get('dgraph.type')}")
        
        # Check if gulab jamun is in results
        gulab_in_activation = any("gulab" in (n.get('name', '') or '').lower() for n in by_activation)
        gulab_in_recency = any("gulab" in (n.get('name', '') or '').lower() for n in by_recency)
        
        print(f"\n=== GULAB JAMUN CHECK ===")
        print(f"  In activation results: {gulab_in_activation}")
        print(f"  In recency results: {gulab_in_recency}")

if __name__ == "__main__":
    asyncio.run(test_hybrid_query())
