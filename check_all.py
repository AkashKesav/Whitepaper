import httpx
import asyncio

async def check_all():
    DGRAPH_URL = "http://localhost:8180/query"
    query = """{
        all(func: has(name), first: 20) {
            name
            dgraph.type
            description
        }
    }"""
    
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        nodes = data.get("data", {}).get("all", [])
        print(f"Total entities: {len(nodes)}")
        for node in nodes:
            print(f"  - {node.get('name')}: {node.get('description', '')[:40]}")

if __name__ == "__main__":
    asyncio.run(check_all())
