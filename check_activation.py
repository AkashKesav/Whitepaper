import httpx
import asyncio

async def quick_check():
    DGRAPH_URL = "http://localhost:8180/query"
    query = """{
        top10(func: has(name), orderdesc: activation, first: 10) {
            name
            activation
            access_count
        }
    }"""
    
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        nodes = data.get("data", {}).get("top10", [])
        
        print("TOP 10 MOST ACTIVATED NODES:\n")
        for i, node in enumerate(nodes, 1):
            name = node.get('name', 'N/A')
            activation = node.get('activation', 0.0)
            count = node.get('access_count', 0)
            print(f"{i}. {name:<20} | Activation: {activation:.3f} | Accessed: {count} times")

if __name__ == "__main__":
    asyncio.run(quick_check())
