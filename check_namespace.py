"""
Debug: Check specific user namespace
"""
import requests

DGRAPH_URL = "http://127.0.0.1:8180/query"

# Find akash's namespace and related nodes
query = '''
{
  all_users(func: type(User)) {
    uid
    name
    namespace
  }
  
  akash_related(func: eq(namespace, "user_akash")) {
    uid
    name
    dgraph.type
    namespace
    activation
    description
  }
}
'''

resp = requests.post(DGRAPH_URL, json={"query": query})
data = resp.json()

print("ALL USERS:")
users = data.get("data", {}).get("all_users", [])
for u in users:
    print(f"  {u.get('name'):<30} | ns: {u.get('namespace')}")

print("\nNODES IN 'user_akash' NAMESPACE:")
akash_nodes = data.get("data", {}).get("akash_related", [])
if akash_nodes:
    for n in akash_nodes[:20]:
        print(f"  {n.get('name'):<30} | {n.get('dgraph.type')} | {n.get('description', '')[:40]}")
else:
    print("  (No nodes found in user_akash namespace!)")
    print("\n  This means India was NOT stored in akash's namespace!")
