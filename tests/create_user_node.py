import httpx
import asyncio

async def create_user():
    DGRAPH_URL = "http://localhost:18080/mutate?commitNow=true"
    
    # N-Quads mutation
    mutation = """
        _:user <name> "browser_test_user" .
        _:user <namespace> "user_browser_test_user" .
        _:user <email> "test@example.com" .
        _:user <activation> "1.0" .
        _:user <confidence> "1.0" .
        _:user <dgraph.type> "User" .
    """
    
    async with httpx.AsyncClient() as client:
        # Note: formatting slightly different for RDF mutation endpoint usually
        # But we can wrap it in Content-Type: application/rdf
        resp = await client.post("http://localhost:18080/mutate?commitNow=true", 
                               content=mutation,
                               headers={"Content-Type": "application/rdf"})
        print(f"Status: {resp.status_code}")
        print(f"Response: {resp.text}")

if __name__ == "__main__":
    asyncio.run(create_user())
