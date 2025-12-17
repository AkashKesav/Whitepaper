"""
Find all nodes related to India or country
"""
import requests

DGRAPH_URL = "http://127.0.0.1:8180/query"

query = '''
{
  # Search for any node with "india" in name or description
  india_all(func: alloftext(name, "india")) {
    uid
    name
    dgraph.type
    namespace
    description
  }
  
  # Search for "country" mentions
  country_all(func: alloftext(name, "country")) {
    uid
    name
    dgraph.type
    namespace
    description
  }
  
  # Search for "favorite" mentions
  favorite_all(func: alloftext(name, "favorite")) {
    uid
    name
    dgraph.type
    namespace
  }
  
  # Check if akash exists at all
  find_akash(func: regexp(name, /akash/i)) {
    uid
    name
    dgraph.type
    namespace
  }
  
  # Get all nodes with namespace containing akash
  akash_ns(func: regexp(namespace, /akash/i)) {
    uid
    name
    dgraph.type
    namespace
  }
}
'''

resp = requests.post(DGRAPH_URL, json={"query": query})
data = resp.json()

print("="*70)
print("SEARCH RESULTS")
print("="*70)

for key in ["india_all", "country_all", "favorite_all", "find_akash", "akash_ns"]:
    print(f"\n{key.upper()}:")
    nodes = data.get("data", {}).get(key, [])
    if nodes:
        for n in nodes[:10]:
            print(f"  {n.get('name', 'N/A'):<25} | ns: {n.get('namespace', 'N/A'):<20} | {n.get('dgraph.type', [])}")
    else:
        print("  (none found)")

print("="*70)
