import requests
import json

def query_dgraph():
    url = "http://localhost:8180/query"
    headers = {"Content-Type": "application/dql"}
    
    # Query for all nodes created recently (or just all nodes since DB is small)
    query = """
    {
      all(func: has(dgraph.type)) {
        uid
        expand(_all_) {
            uid
            expand(_all_)
        }
        dgraph.type
      }
    }
    """
    
    try:
        response = requests.post(url, headers=headers, data=query)
        print(f"Status: {response.status_code}")
        print(f"Raw: {response.text[:500]}")
        data = response.json()
        with open("graph_dump.json", "w") as f:
            json.dump(data, f, indent=2)
        print("Dump saved to graph_dump.json")
    except Exception as e:
        print(f"Error: {e}")

if __name__ == "__main__":
    query_dgraph()
