#!/usr/bin/env python3
"""
Activation Boost Verification Test
Checks if recalling a memory increases its activation score.
Uses /api/search to get node stats.
"""

import httpx
import uuid
import time
import json

BASE_URL = "http://localhost:9090"

def run_test():
    client = httpx.Client(timeout=30.0)
    suffix = str(uuid.uuid4())[:8]
    username = f"act_{suffix}" # Short name
    password = "password123"
    
    # 1. Register
    print(f"Registering {username}...")
    resp = client.post(f"{BASE_URL}/api/register", json={"username": username, "password": password})
    if resp.status_code not in [200, 201]:
        print(f"❌ Register failed: {resp.status_code} {resp.text[:100]}")
        return False
    token = resp.json().get("token")
    headers = {"Authorization": f"Bearer {token}"}
    
    # 2. Store Fact
    # Make it unique so search finds it easily
    unique_fruit = f"Durian_{suffix}"
    fact_text = f"My favorite fruit is {unique_fruit}"
    print(f"Storing fact: '{fact_text}'")
    resp = client.post(f"{BASE_URL}/api/chat", json={"message": f"Remember this: {fact_text}"}, headers=headers)
    if resp.status_code != 200:
        print(f"❌ Chat store failed: {resp.text}")
        return False
        
    # 3. Wait for Ingestion (5s interval -> wait 12s for safety)
    print("Waiting 12s for ingestion...")
    time.sleep(12)
    
    print(f"Searching for '{unique_fruit}'...")
    try:
        resp = client.get(f"{BASE_URL}/api/search", params={"q": unique_fruit}, headers=headers)
        if resp.status_code != 200:
            print(f"❌ Search failed: {resp.status_code} {resp.text}")
            return False
            
        print(f"DEBUG Response: {resp.text[:200]}")
        try:
            data = resp.json()
        except:
            print("❌ Failed to parse JSON")
            return False

        if data is None:
             print("❌ API returned null JSON")
             return False
             
        if isinstance(data, list):
            results = data
        else:
            results = data.get("results")
            if results is None:
                results = []
            
        if not results:
            print("❌ Fact node not found via Search!")
            return False
            
        node = results[0]
        initial_activation = node.get("activation", 0.5)
        uid = node.get("uid")
        
        print(f"✅ Found Node: {uid}")
        print(f"Initial Activation: {initial_activation}")
        
    except Exception as e:
        print(f"❌ Exception during search: {e}")
        return False

    # 5. Recall the fact (Consultation)
    print("Recalling fact (Consultation)...")
    resp = client.post(f"{BASE_URL}/api/chat", json={"message": f"What is my favorite fruit?"}, headers=headers)
    recall_text = resp.json().get("response", "")
    print(f"AI Response: {recall_text}")
    
    # 6. Wait a moment for async update (if any)
    time.sleep(2)
    
    # 7. Check Activation Again
    print("Checking activation again...")
    resp = client.get(f"{BASE_URL}/api/search", params={"q": unique_fruit}, headers=headers)
    
    data = resp.json()
    if isinstance(data, list):
        results = data
    else:
        results = data.get("results")
        if results is None: results = []
        
    if not results:
        print("❌ Fact node not found via Search (2nd time)!")
        return False
        
    node_after = results[0]
    
    final_activation = node_after.get("activation", 0.5)
    
    print(f"Final Activation: {final_activation}")
    
    # If the logic is working, final_activation should be > initial_activation
    # Default boost is likely 0.1 or similar (config.BoostPerAccess)
    
    if final_activation > initial_activation:
        print(f"✅ SUCCESS: Activation increased ({initial_activation} -> {final_activation})")
        return True
    else:
        print(f"❌ FAILURE: Activation did not increase ({initial_activation} -> {final_activation})")
        return False

if __name__ == "__main__":
    success = run_test()
    exit(0 if success else 1)
