import httpx
import asyncio

async def find_gulab_position():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Get more nodes to find gulab jamun's position
    query = """
    {
        all_nodes(func: has(name), first: 100, orderdesc: activation) {
            uid
            dgraph.type
            name
            activation
        }
    }
    """
    
    print("Finding gulab jamun position in activation order...")
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        
        nodes = data.get("data", {}).get("all_nodes", [])
        
        for i, node in enumerate(nodes):
            name = node.get('name', '')
            if 'gulab' in name.lower():
                print(f"\n=== FOUND GULAB JAMUN ===")
                print(f"  Position: {i+1} of {len(nodes)}")
                print(f"  Name: {name}")
                print(f"  Activation: {node.get('activation')}")
                print(f"  Type: {node.get('dgraph.type')}")
                return
        
        print("Gulab Jamun not found in top 100 by activation!")

if __name__ == "__main__":
    asyncio.run(find_gulab_position())
