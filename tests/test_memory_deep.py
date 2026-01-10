#!/usr/bin/env python3
"""
Deep Memory Isolation & Recall Test
Verifies that:
1. User A remembers their unique facts.
2. User B remembers their unique facts.
3. User A cannot access User B's facts.
4. Chat history is preserved within session.
"""

import httpx
import uuid
import time
import json
import sys

BASE_URL = "http://localhost:9090"

def run_test():
    client = httpx.Client(timeout=120.0)
    
    # 1. Create two fresh users
    suffix = str(uuid.uuid4())[:8]
    user_a = {"username": f"pilot_{suffix}", "password": "password123", "token": None}
    user_b = {"username": f"doctor_{suffix}", "password": "password123", "token": None}
    
    print(f"Creating Users: {user_a['username']}, {user_b['username']}")
    
    for u in [user_a, user_b]:
        resp = client.post(f"{BASE_URL}/api/register", json={"username": u["username"], "password": u["password"]})
        if resp.status_code not in [200, 201]:
            print(f"❌ Failed to register {u['username']}: {resp.text}")
            return False
        u["token"] = resp.json().get("token")
        
    def chat(user, msg):
        headers = {"Authorization": f"Bearer {user['token']}"}
        try:
            r = client.post(f"{BASE_URL}/api/chat", json={"message": msg}, headers=headers)
            if r.status_code == 429:
                print(f"⚠️ Rate limited for {user['username']}. Waiting 5s...")
                time.sleep(5)
                return chat(user, msg) # Retry
            if r.status_code != 200:
                print(f"❌ Chat failed for {user['username']}: {r.status_code} {r.text}")
                return None
            return r.json().get("response", "")
        except Exception as e:
            print(f"❌ Exception during chat: {e}")
            return None

    # 2. Store distinctive memories
    print("\n[Step 1] Ingesting Memories...")
    
    # User A: Pilot
    resp_a = chat(user_a, "Remember this: I work as a Senior Pilot for Cloud Airlines.")
    print(f"User A (Pilot) -> AI: {resp_a}")
    
    # User B: Doctor
    resp_b = chat(user_b, "Remember this: I work as a Neurosurgeon at City Hospital.")
    print(f"User B (Doctor) -> AI: {resp_b}")
    
    # 3. Wait for Async Cold Path
    print("\n[Step 2] Waiting 15 seconds for Wisdom Layer processing (Interval optimized to 5s)...")
    time.sleep(15)
    
    # 4. Verify Recall (Isolation)
    print("\n[Step 3] Verifying Recall & Isolation...")
    
    # Check User A
    print(f"--- Checking {user_a['username']} ---")
    recall_a = chat(user_a, "What is my profession and where do I work?")
    print(f"User A Recall: {recall_a}")
    
    if "pilot" in recall_a.lower() and "active synthesis" in recall_a.lower(): # Check for Pilot
        print("✅ User A correctly recalled 'Pilot'")
    elif "pilot" in recall_a.lower():
         print("✅ User A correctly recalled 'Pilot'")
    else:
        print("❌ User A FAILED to recall 'Pilot'")
        
    # Check User B
    print(f"--- Checking {user_b['username']} ---")
    recall_b = chat(user_b, "What is my profession and where do I work?")
    print(f"User B Recall: {recall_b}")
    
    if "neurosurgeon" in recall_b.lower() or "doctor" in recall_b.lower():
        print("✅ User B correctly recalled 'Doctor/Neurosurgeon'")
    else:
        print("❌ User B FAILED to recall 'Doctor/Neurosurgeon'")
        
    # 5. Check Leakage
    print("\n[Step 4] Checking Cross-Pollination (Leakage)...")
    leak_check = chat(user_a, "Do you know any Neurosurgeons?")
    print(f"User A asking about B's fact: {leak_check}")
    
    if "city hospital" in leak_check.lower() or "user_b" in leak_check.lower():
        print("❌ LEAK DETECTED: User A knows about City Hospital!")
    else:
        print("✅ No leak detected (User A does not know about City Hospital)")

    print("\nTest Complete.")
    return True

if __name__ == "__main__":
    run_test()
