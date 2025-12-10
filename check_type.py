import httpx
import asyncio

async def check_type_index():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Query by UID directly
    query = """
    {
        node(func: uid(0x5b42)) {
            uid
            dgraph.type
            name
            activation
        }
        
        # Count all types
        preferences(func: type(Preference)) {
            count(uid)
        }
        
        facts(func: type(Fact)) {
            count(uid)
        }
        
        entities(func: type(Entity)) {
            count(uid)
        }
    }
    """
    
    print("Checking type indices...")
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        print(resp.json())

if __name__ == "__main__":
    asyncio.run(check_type_index())
