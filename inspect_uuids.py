import httpx
import asyncio

async def check_uuid_nodes():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Get nodes that look like UUIDs
    query = """{
        uuids(func: has(name)) {
            uid
            name
            dgraph.type
            description
            created_at
        }
    }"""
    
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        nodes = data.get("data", {}).get("uuids", [])
        
        print(f"=== ALL NODES ({len(nodes)} total) ===\n")
        
        uuid_nodes = []
        real_nodes = []
        
        for node in nodes:
            name = node.get('name', '')
            node_type = node.get('dgraph.type', [])
            
            # Check if it's a UUID
            if len(name) == 36 and name.count('-') == 4:
                uuid_nodes.append(node)
            else:
                real_nodes.append(node)
        
        print(f"❌ UUID NODES: {len(uuid_nodes)}")
        for node in uuid_nodes[:10]:
            print(f"   {node.get('name')} | Type: {node.get('dgraph.type')} | Desc: {node.get('description', 'None')}")
        
        print(f"\n✅ REAL NODES: {len(real_nodes)}")
        for node in real_nodes:
            print(f"   {node.get('name')}")

if __name__ == "__main__":
    asyncio.run(check_uuid_nodes())
