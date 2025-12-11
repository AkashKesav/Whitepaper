"""
Check specifically for akash namespace
"""
import requests

resp = requests.post("http://127.0.0.1:8180/query", json={
    "query": '''
    {
        akash_ns(func: eq(namespace, "user_akash")) {
            uid
            name
            dgraph.type
            namespace
            description
        }
    }
    '''
})

data = resp.json().get("data", {}).get("akash_ns", [])
print(f"Found {len(data)} nodes in 'user_akash' namespace:")

if data:
    for node in data:
        print(f"  {node.get('name', 'N/A'):<35} | {node.get('dgraph.type', [])}")
        if node.get('description'):
            print(f"    Description: {node.get('description')[:60]}...")
else:
    print("  (no nodes found)")
    print("\n  This confirms: akash's data is NOT in the correct namespace!")
    print("  The consultation queries filter by namespace, so they return 0 results")
