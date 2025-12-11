"""
Check user's data in DGraph
"""
import requests
import json

DGRAPH_URL = "http://127.0.0.1:8180/query"

# Check for akash user and their relationships
query = '''
{
  akash_user(func: anyofterms(name, "akash")) {
    uid
    name
    dgraph.type
    namespace
    knows {
      uid
      name
      dgraph.type
    }
  }
  
  all_entities(func: type(Entity), first: 20) {
    uid
    name
    dgraph.type
    namespace
    description
    activation
  }
  
  all_preferences(func: type(Preference), first: 20) {
    uid
    name
    dgraph.type
    namespace
    description
  }
  
  country_search(func: anyofterms(name, "india country favorite")) {
    uid
    name
    dgraph.type
    namespace
    description
  }
}
'''

resp = requests.post(DGRAPH_URL, json={"query": query})
data = resp.json()

print("="*70)
print("USER DATA CHECK")
print("="*70)

print("\n[1] Akash user node:")
akash = data.get("data", {}).get("akash_user", [])
for node in akash:
    print(f"  Name: {node.get('name')}")
    print(f"  Namespace: {node.get('namespace')}")
    knows = node.get("knows", [])
    print(f"  Knows: {len(knows)} entities")
    for k in knows[:5]:
        print(f"    - {k.get('name')} ({k.get('dgraph.type')})")

print("\n[2] All Entity nodes:")
entities = data.get("data", {}).get("all_entities", [])
for e in entities[:10]:
    ns = e.get('namespace', 'N/A')
    print(f"  - {e.get('name'):<30} | ns: {ns:<20} | act: {e.get('activation')}")

print("\n[3] All Preference nodes:")
prefs = data.get("data", {}).get("all_preferences", [])
for p in prefs[:10]:
    ns = p.get('namespace', 'N/A')
    print(f"  - {p.get('name'):<30} | ns: {ns:<20}")

print("\n[4] Country/India search:")
country = data.get("data", {}).get("country_search", [])
for c in country:
    print(f"  - {c.get('name')} | ns: {c.get('namespace')} | {c.get('description', '')[:50]}")

if not country:
    print("  (No India/country nodes found!)")

print("="*70)
