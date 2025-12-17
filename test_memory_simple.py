"""
Simple Memory Test - Check if memories are being stored and retrieved
"""
import requests
import time

BASE_URL = "http://127.0.0.1:3000/api"

def register_and_login():
    username = "memory_test_user_123"
    password = "test123"
    
    resp = requests.post(f"{BASE_URL}/register", json={"username": username, "password": password})
    if resp.status_code == 200:
        return resp.json()["token"], username
    
    resp = requests.post(f"{BASE_URL}/login", json={"username": username, "password": password})
    if resp.status_code == 200:
        return resp.json()["token"], username
    
    return None, username

def chat(token, message, conv_id=None):
    headers = {"Authorization": f"Bearer {token}"}
    payload = {"message": message}
    if conv_id:
        payload["conversation_id"] = conv_id
    
    resp = requests.post(f"{BASE_URL}/chat", headers=headers, json=payload, timeout=60)
    print(f"Chat response status: {resp.status_code}")
    return resp.json() if resp.status_code == 200 else None

print("="*60)
print("SIMPLE MEMORY TEST")
print("="*60)

# Step 1: Get token
print("\n[1] Authenticating...")
token, username = register_and_login()
if not token:
    print("FAILED: Could not authenticate")
    exit(1)
print(f"OK: Got token for {username}")

# Step 2: Store a memory with unique identifier
print("\n[2] Storing memory...")
unique_secret = "SECRETCODE-ALPHA-7890-ZETA"
resp = chat(token, f"My secret code is {unique_secret}. Please remember this.")
if not resp:
    print("FAILED: Chat failed")
    exit(1)
conv_id = resp.get("conversation_id")
print(f"OK: Memory stored. ConvID: {conv_id}")
print(f"Response: {resp.get('response', 'N/A')[:80]}...")

# Step 3: Wait for ingestion
print("\n[3] Waiting 10 seconds for ingestion...")
time.sleep(10)

# Step 4: Try to recall the memory
print("\n[4] Testing recall...")
resp = chat(token, "What is my secret code?", conv_id=conv_id)
if not resp:
    print("FAILED: Recall chat failed")
    exit(1)

response_text = resp.get("response", "")
print(f"Response: {response_text}")

# Step 5: Check if the secret is in the response
print("\n[5] Checking if memory was recalled...")
if unique_secret in response_text:
    print("SUCCESS: Exact secret found in response!")
elif "SECRETCODE" in response_text.upper() or "ALPHA" in response_text.upper() or "7890" in response_text:
    print("PARTIAL: Some parts of secret found in response")
elif "secret" in response_text.lower() or "code" in response_text.lower():
    print("WEAK: Response mentions secret/code but doesn't recall exact value")
    print("This suggests memory ingestion may have worked but retrieval is incomplete")
else:
    print("FAILED: Secret not found in response")
    print("This suggests memory is not being stored or retrieved properly")

# Step 6: Check kernel logs
print("\n[6] Checking system status...")
import subprocess
result = subprocess.run(
    ["docker", "logs", "rmk-memory-kernel", "--tail", "20"],
    capture_output=True, text=True
)
print("Recent kernel logs:")
print(result.stdout[-500:] if result.stdout else result.stderr[-500:])

print("\n" + "="*60)
