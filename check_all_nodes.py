import httpx
import asyncio
import os
import json

async def check_all_nodes():
    DGRAPH_URL = "http://localhost:8180/query"
    
    query = """
    {
      all_facts(func: has(name), first: 50) @filter(NOT type(User)) {
        uid
        dgraph.type
        name
        description
        tags
      }
    }
    """
    
    print(f"Querying DGraph at: {DGRAPH_URL}")
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        resp.raise_for_status()
        data = resp.json()
        
        nodes = data.get("data", {}).get("all_facts", [])
        print(f"\nTotal nodes found: {len(nodes)}")
        
        # Show first 10
        for i, node in enumerate(nodes[:10]):
            print(f"\n{i+1}. {node.get('name')}")
            print(f"   Type: {node.get('dgraph.type')}")
            print(f"   Desc: {node.get('description', '')[:50]}")

if __name__ == "__main__":
    asyncio.run(check_all_nodes())
