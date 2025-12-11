"""
Check newuser namespace specifically - fixed query
"""
import requests

DGRAPH_URL = "http://127.0.0.1:8180/query"

# Simple query for newuser namespace
resp = requests.post(DGRAPH_URL, json={
    "query": '''
    {
        newuser_ns(func: eq(namespace, "user_newuser")) {
            uid
            name
            dgraph.type
            namespace
            description
            activation
        }
    }
    '''
})

data = resp.json()
print("Raw response:", data)

nodes = data.get("data", {}).get("newuser_ns", [])
print(f"\nNodes in 'user_newuser' namespace: {len(nodes)}")
for n in nodes:
    print(f"  {n}")
