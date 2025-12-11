"""
Check cross-contamination
"""
import requests

DGRAPH_URL = "http://127.0.0.1:8180/query"

query = '''
{
  # Get ALL nodes with name India
  india_nodes(func: eq(name, "India")) {
    uid
    name
    namespace
    dgraph.type
  }
}
'''

resp = requests.post(DGRAPH_URL, json={"query": query})
print(resp.json())
