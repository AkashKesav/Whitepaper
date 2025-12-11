"""
Quick Memory Verification Test
"""
import requests
import time
import uuid

BASE_URL = "http://127.0.0.1:3000/api"

# Create unique user
username = f"verify_{uuid.uuid4().hex[:8]}"
password = "test123"

print("="*60)
print("MEMORY VERIFICATION TEST")
print("="*60)

# Register
print(f"\n[1] Registering user: {username}")
resp = requests.post(f"{BASE_URL}/register", json={"username": username, "password": password})
token = resp.json().get("token")
print(f"    Token: {token[:20]}...")

# Store secret
secret = f"ZEBRA-{uuid.uuid4().hex[:8].upper()}"
print(f"\n[2] Storing secret: {secret}")
headers = {"Authorization": f"Bearer {token}"}
resp = requests.post(f"{BASE_URL}/chat", 
                    json={"message": f"My secret animal code is {secret}. Remember this for me."},
                    headers=headers, timeout=60)
data = resp.json()
conv_id = data.get("conversation_id")
print(f"    Response: {data.get('response', 'N/A')[:60]}...")

# Wait for ingestion
print("\n[3] Waiting 8 seconds for ingestion...")
time.sleep(8)

# Recall
print("\n[4] Asking: What is my secret animal code?")
resp = requests.post(f"{BASE_URL}/chat",
                    json={"message": "What is my secret animal code?", "conversation_id": conv_id},
                    headers=headers, timeout=60)
data = resp.json()
response_text = data.get("response", "")
print(f"    Response: {response_text}")

# Check result
print("\n[5] Result:")
if secret in response_text:
    print("    ✅ SUCCESS! Secret was recalled exactly!")
elif secret.split('-')[0] in response_text.upper():
    print("    ⚠️ PARTIAL: First part of secret found")
else:
    print("    ❌ FAILED: Secret not found in response")

print("="*60)
