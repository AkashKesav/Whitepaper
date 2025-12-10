import httpx
import asyncio

async def drop_all():
    DGRAPH_URL = "http://localhost:8180/alter"
    
    print("Dropping all data from DGraph...")
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, data='{"drop_all": true}', headers={"Content-Type": "application/json"})
        print(f"Status: {resp.status_code}")
        print(f"Response: {resp.text}")

if __name__ == "__main__":
    asyncio.run(drop_all())
