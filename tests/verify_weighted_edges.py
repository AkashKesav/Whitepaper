import requests
import time
import json

# Internal Docker URLs
BASE_URL = "http://monolith:9090/api"
DGRAPH_URL = "http://dgraph-alpha:8080/query"

def send_chat(message):
    print(f"\nSending: '{message}'")
    try:
        resp = requests.post(f"{BASE_URL}/chat", json={"message": message, "user_id": "test_user_weights"})
        if resp.status_code == 200:
            data = resp.json()
            print(f"Response: {data.get('response')}")
            return data
        else:
            print(f"Error: {resp.status_code} - {resp.text}")
            return None
    except Exception as e:
        print(f"Chat Request Failure: {e}")
        return None

def get_node_details(uid):
    query = """
    {
        node(func: uid(%s)) {
            uid
            name
            friend_of @facets(weight) {
                uid
                name
                weight
            }
        }
    }
    """ % uid
    
    try:
        resp = requests.post(DGRAPH_URL, json={"query": query})
        if resp.status_code == 200:
            return resp.json().get('data', {}).get('node', [])
    except Exception as e:
        print(f"DGraph Query Error: {e}")
    return []

def get_graph(namespace="test_user_weights"):
    try:
        resp = requests.get(f"{BASE_URL}/graph?namespace={namespace}")
        if resp.status_code == 200:
            return resp.json()['nodes']
    except Exception as e:
        print(f"Graph Request Failure: {e}")
    return []

def main():
    print("=== Testing Weighted Activation (Internal) ===")
    
    # 1. Ingest a relationship
    send_chat("Alice is a friend of Bob.")
    
    print("Waiting for ingestion...")
    alice_uid = ""
    for i in range(10):
        time.sleep(2)
        nodes = get_graph()
        alice_node = next((n for n in nodes if "Alice" in n['label']), None)
        if alice_node:
            alice_uid = alice_node['id']
            break
        print(f"Retry {i+1}...")
    
    if not alice_uid:
        print("FAIL: Failed to find Alice node")
        return
        
    print(f"✅ Found Alice Node: {alice_uid}")
    
    # 2. Query DGraph directly
    print("\n--- Querying DGraph for Edge Weight ---")
    details = get_node_details(alice_uid)
    
    found_weight = False
    for n in details:
        if 'friend_of' in n:
            for friend in n['friend_of']:
                print(f"Raw Edge: {friend}")
                if 'friend_of|weight' in friend:
                     w = friend['friend_of|weight']
                     print(f"✅ Found Weight Facet: {w}")
                     if w > 0.7: # Expect roughly 0.8
                         print("✅ SUCCESS: Weight matches expected value")
                         found_weight = True
                elif 'weight' in friend:
                     w = friend['weight']
                     print(f"✅ Found Weight Facet: {w}")
                     if w > 0.7:
                         found_weight = True
                         print("✅ SUCCESS: Weight matches expected value")

    if not found_weight:
        print("❌ FAIL: No weight facet found on edge.")
        print(f"Full Details: {details}")

if __name__ == "__main__":
    main()
