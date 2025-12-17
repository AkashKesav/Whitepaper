import httpx, asyncio

async def count_nodes():
    async with httpx.AsyncClient() as client:
        query = '{"query": "{count(func: has(name)) {count(uid)}}"}'
        resp = await client.post("http://localhost:8180/query", content=query, 
                                headers={"Content-Type": "application/json"})
        data = resp.json()
        count = data.get("data", {}).get("count", [{}])[0].get("count", 0)
        print(f"Total nodes in DGraph: {count}")

if __name__ == "__main__":
    asyncio.run(count_nodes())
