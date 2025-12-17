import httpx
import asyncio

async def check_all_nodes():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Get ALL nodes ordered by creation time
    query = """{
        all(func: has(name), orderdesc: created_at, first: 30) {
            uid
            name
            description
            activation
            access_count
            created_at
        }
    }"""
    
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        nodes = data.get("data", {}).get("all", [])
        
        print(f"=== MOST RECENT 30 NODES ===\n")
        for i, node in enumerate(nodes, 1):
            name = node.get('name', 'N/A')
            activation = node.get('activation', 0.0)
            created = node.get('created_at', 'N/A')[:19] if node.get('created_at') else 'N/A'
            
            # Highlight nodes with 0.5 activation (our new default)
            marker = "🆕" if activation == 0.5 else "🔥"
            
            print(f"{i}. {marker} {name:<30} | Act: {activation:.2f} | Created: {created}")

if __name__ == "__main__":
    asyncio.run(check_all_nodes())
