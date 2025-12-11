import requests
import time
import json

BASE_URL = "http://127.0.0.1:3000/api"
KERNEL_URL = "http://127.0.0.1:9000/api"

def register(username, password):
    resp = requests.post(f"{BASE_URL}/register", json={"username": username, "password": password})
    return resp.json().get("token")

def login(username, password):
    resp = requests.post(f"{BASE_URL}/login", json={"username": username, "password": password})
    return resp.json().get("token")

def chat(token, message, conv_id=None):
    headers = {"Authorization": f"Bearer {token}"}
    payload = {"message": message}
    if conv_id:
        payload["conversation_id"] = conv_id
    resp = requests.post(f"{BASE_URL}/chat", headers=headers, json=payload)
    return resp.json()

def share(token, conv_id, target_user):
    headers = {"Authorization": f"Bearer {token}"}
    payload = {"conversation_id": conv_id, "target_username": target_user}
    resp = requests.post(f"{BASE_URL}/share", headers=headers, json=payload)
    return resp.status_code

def main():
    print(">>> Registering Users...")
    token_a = register("user_a", "password123")
    token_b = register("user_b", "password123")
    
    if not token_a or not token_b:
        print("Registration failed. Logging in...")
        token_a = login("user_a", "password123")
        token_b = login("user_b", "password123")

    print(f"Token A: {token_a[:10]}...")
    print(f"Token B: {token_b[:10]}...")

    # 1. User A creates a memory
    print("\n>>> User A creating memory...")
    resp = chat(token_a, "My favorite color is electric blue.")
    conv_id = resp["conversation_id"]
    print(f"Conversation ID: {conv_id}")
    print(f"Response: {resp['response']}")
    
    time.sleep(2) # Wait for ingestion

    # verify retrieval
    print("\n>>> User A verifying memory...")
    resp = chat(token_a, "What is my favorite color?", conv_id=conv_id)
    print(f"A's Recall: {resp['response']}")
    if "blue" not in resp['response'].lower():
        print("WARNING: User A failed to recall own memory!")

    # 2. User B tries to access A's memory (Should Fail)
    print("\n>>> User B trying to access A's memory...")
    resp = chat(token_b, "What is User A's favorite color?")
    print(f"B's Access Attempt: {resp['response']}")
    if "blue" in resp['response'].lower():
        print("FAILURE: User B accessed User A's private memory!")
    else:
        print("SUCCESS: Isolation working.")

    # 3. User A Shares with User B
    print("\n>>> User A sharing conversation with User B...")
    status = share(token_a, conv_id, "user_b")
    if status == 200:
        print("Share successful.")
    else:
        print(f"Share failed with status {status}")

    time.sleep(1)

    # 4. User B tries again (Should Succeed)
    print("\n>>> User B trying to access SHARED memory...")
    resp = chat(token_b, "What is User A's favorite color?")
    print(f"B's Access Attempt 2: {resp['response']}")
    if "blue" in resp['response'].lower():
        print("SUCCESS: User B accessed shared memory!")
    else:
        print("FAILURE: Sharing did not enable access.")

    # 5. Access Count Verification (Activation Boost)
    # We will trigger A checking memory multiple times
    print("\n>>> Boosting activation via multiple accesses...")
    for i in range(5):
        chat(token_a, "Tell me about my favorite color again.", conv_id=conv_id)
        # Check kernel stats/logs if possible, or just trust the detailed logs
    
    print("Done.")

if __name__ == "__main__":
    main()
