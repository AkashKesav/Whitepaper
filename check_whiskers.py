import httpx
import asyncio

async def check_whiskers():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Query 1: Search for Whiskers/Tuna explicitly
    query1 = """{
        search(func: anyoftext(name, "Whiskers Tuna")) {
            uid
            name
            description
            dgraph.type
            created_at
        }
    }"""
    
    # Query 2: Get ALL entities (no limit)
    query2 = """{
        all(func: has(name)) {
            uid
            name
            description
            dgraph.type
        }
    }"""
    
    async with httpx.AsyncClient() as client:
        # Search query
        resp1 = await client.post(DGRAPH_URL, json={"query": query1})
        data1 = resp1.json()
        search_results = data1.get("data", {}).get("search", [])
        
        print("=== SEARCH FOR WHISKERS/TUNA ===")
        if search_results:
            for node in search_results:
                print(f"  ✅ {node.get('name')}: {node.get('description', 'No description')}")
        else:
            print("  ❌ Not found in DGraph")
        
        # All entities
        resp2 = await client.post(DGRAPH_URL, json={"query": query2})
        data2 = resp2.json()
        all_nodes = data2.get("data", {}).get("all", [])
        
        print(f"\n=== TOTAL ENTITIES IN DGRAPH: {len(all_nodes)} ===")
        print("\nLast 10 entities:")
        for node in all_nodes[-10:]:
            print(f"  - {node.get('name')}: {node.get('description', '')[:50]}")

if __name__ == "__main__":
    asyncio.run(check_whiskers())
