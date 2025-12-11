"""
Check if India was stored in DGraph
"""
import requests
import json

DGRAPH_URL = "http://127.0.0.1:8180/query"

# Check for India node
query = '''
{
  india(func: anyofterms(name, "india India")) {
    uid
    name
    dgraph.type
    namespace
    activation
    description
  }
  
  preferences(func: type(Preference)) {
    uid
    name
    dgraph.type
    namespace
    activation
  }
  
  recent(func: has(name), orderdesc: updated_at, first: 10) {
    uid
    name
    dgraph.type
    namespace
    updated_at
  }
}
'''

resp = requests.post(DGRAPH_URL, json={"query": query})
data = resp.json()

print("="*60)
print("DGRAPH QUERY RESULTS")
print("="*60)

print("\n[1] India nodes:")
india_nodes = data.get("data", {}).get("india", [])
if india_nodes:
    for node in india_nodes:
        print(f"  - {node}")
else:
    print("  (none found)")

print("\n[2] Preference nodes:")
prefs = data.get("data", {}).get("preferences", [])
for pref in prefs[:10]:
    print(f"  - {pref.get('name')} | ns: {pref.get('namespace')} | act: {pref.get('activation')}")

print("\n[3] Recent nodes (by updated_at):")
recent = data.get("data", {}).get("recent", [])
for node in recent:
    print(f"  - {node.get('name')} | type: {node.get('dgraph.type')} | ns: {node.get('namespace')}")

print("="*60)
