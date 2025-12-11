import requests
import time
import uuid

BASE_URL = "http://localhost:3000/api"

def register_user(username, password):
    resp = requests.post(f"{BASE_URL}/register", json={"username": username, "password": password})
    if resp.status_code == 409: # Already exists
        resp = requests.post(f"{BASE_URL}/login", json={"username": username, "password": password})
    
    if resp.status_code != 200:
        print(f"Failed to auth {username}: {resp.text}")
        return None
    return resp.json()["token"]

def chat(token, message):
    headers = {"Authorization": f"Bearer {token}"}
    resp = requests.post(f"{BASE_URL}/chat", json={"message": message}, headers=headers)
    if resp.status_code != 200:
        print(f"Chat failed: {resp.text}")
        return None
    return resp.json()["response"]

def test_separation():
    # 1. Setup Users
    suffix = str(uuid.uuid4())[:8]
    user_a = f"user_a_{suffix}"
    user_b = f"user_b_{suffix}"
    password = "password123"

    print(f"Registering {user_a}...")
    token_a = register_user(user_a, password)
    print(f"Registering {user_b}...")
    token_b = register_user(user_b, password)

    if not token_a or not token_b:
        print("Registration failed, aborting.")
        return

    # 2. User A sets a unique fact
    secret_code = f"Omega-{suffix}"
    print(f"\n[User A] stating fact: 'The secret project codename is {secret_code}'")
    chat(token_a, f"The secret project codename is {secret_code}")
    
    # Wait for ingestion
    print("Waiting 5s for ingestion...")
    time.sleep(5)

    # 3. User B tries to retrieve it
    print(f"\n[User B] asking: 'What is the secret project codename?'")
    response_b = chat(token_b, "What is the secret project codename?")
    
    print(f"\n[User B] Response: {response_b}")

    # 4. Analysis
    if secret_code in response_b:
        print("\n❌ FAIL: Separation broken! User B accessed User A's secret.")
    else:
        print("\n✅ PASS: Separation intact (or retrieval failed). User B did not see the secret.")

if __name__ == "__main__":
    test_separation()
