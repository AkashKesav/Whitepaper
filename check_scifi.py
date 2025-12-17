import httpx
import asyncio

async def search_scifi():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Search for science fiction
    query = """{
        search(func: anyoftext(name, "science fiction book reading")) {
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
        results = data.get("data", {}).get("search", [])
        
        print("=== SEARCH FOR 'SCIENCE FICTION' ===")
        if results:
            for node in results:
                print(f"✅ {node.get('name')}")
                print(f"   Activation: {node.get('activation', 'N/A')}")
                print(f"   Description: {node.get('description', 'N/A')}")
                print(f"   Access Count: {node.get('access_count', 0)}")
                print()
        else:
            print("❌ NOT FOUND - Extraction may have failed")
            print("\nChecking extraction logs...")

if __name__ == "__main__":
    asyncio.run(search_scifi())
