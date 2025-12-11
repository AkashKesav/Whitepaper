"""
Simple DGraph query to find all nodes and their namespaces
"""
import requests

resp = requests.post("http://127.0.0.1:8180/query", json={
    "query": '''
    {
        all(func: has(namespace), first: 30) {
            uid
            name
            dgraph.type
            namespace
        }
    }
    '''
})

data = resp.json().get("data", {}).get("all", [])
print(f"Found {len(data)} nodes with namespace field:\n")

for node in data:
    print(f"{node.get('name', 'N/A'):<35} | ns: {node.get('namespace', 'N/A'):<25} | {node.get('dgraph.type', [])}")
