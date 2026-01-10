import requests
import time
import json

BASE_URL = "http://localhost:9090/api"

def send_chat(message):
    print(f"\nSending: '{message}'")
    resp = requests.post(f"{BASE_URL}/chat", json={"message": message, "user_id": "test_user_semantic"})
    if resp.status_code == 200:
        data = resp.json()
        print(f"Response: {data.get('response')}")
        return data
    else:
        print(f"Error: {resp.status_code} - {resp.text}")
        return None

def get_graph(namespace="test_user_semantic"):
    resp = requests.get(f"{BASE_URL}/graph?namespace={namespace}")
    if resp.status_code == 200:
        return resp.json()['nodes']
    return []

def main():
    print("=== Testing Semantic Deduplication ===")
    
    # 1. Establish the "Canonical" entity
    send_chat("I truly admire Barack Obama as a leader.")
    
    print("Waiting for ingestion...")
    obama_node = None
    for i in range(10):
        time.sleep(2)
        nodes_1 = get_graph()
        obama_node = next((n for n in nodes_1 if "Obama" in n['label']), None)
        if obama_node:
            break
        print(f"Retry {i+1}...")
    
    if not obama_node:
        print("FAIL: Failed to create initial node for Barack Obama")
        return
        
    print(f"✅ Created Initial Node: {obama_node['label']} (ID: {obama_node['id']})")
    
    # 2. Mention a semantic equivalent
    print("\n--- Sending Semantic Variant ---")
    send_chat("President Obama was the 44th president.")
    
    print("Waiting for ingestion...")
    obama_nodes = []
    for i in range(10):
        time.sleep(2)
        nodes_2 = get_graph()
        obama_nodes = [n for n in nodes_2 if "Obama" in n['label']]
        # We expect count 1 if successful, count 2 if failed.
        # But we need to make sure the second ingestion actually finished.
        # How to know? We can check if activation increased or something?
        # Or just wait sufficiently.
        pass
    
    print(f"\nGraph Snapshot:")
    for n in obama_nodes:
        print(f" - {n['label']} (ID: {n['id']})")
        
    if len(obama_nodes) == 1:
        print("\n✅ SUCCESS: Semantic Deduplication worked! Only 1 'Obama' node exists.")
    else:
        print(f"\n❌ FAIL: Found {len(obama_nodes)} nodes (Split Brain). Deduplication failed.")

if __name__ == "__main__":
    main()
