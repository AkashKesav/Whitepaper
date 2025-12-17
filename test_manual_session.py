"""
Manual Interactive Chat Test
Simulates a real user session to verify the chat system works as intended.
"""
import requests
import time

BASE_URL = "http://localhost:9090/api"

print("="*60)
print("MANUAL CHAT SESSION TEST")
print("="*60)

# Step 1: Login
print("\n[1] Logging in as 'manual_alex'...")
login_resp = requests.post(f"{BASE_URL}/login", json={
    "username": "manual_alex",
    "password": "password123"
})
token = login_resp.json().get("token")
print(f"   Token received: {token[:30]}...")

headers = {"Authorization": f"Bearer {token}"}

# Step 2: First message - introduce myself
print("\n[2] Sending: 'My name is Alex and I love playing chess'")
resp1 = requests.post(f"{BASE_URL}/chat", json={
    "message": "My name is Alex and I love playing chess"
}, headers=headers)
print(f"   Response: {resp1.json()['response']}")

time.sleep(2)

# Step 3: Second message - add more info
print("\n[3] Sending: 'I also enjoy reading mystery novels by Agatha Christie'")
resp2 = requests.post(f"{BASE_URL}/chat", json={
    "message": "I also enjoy reading mystery novels by Agatha Christie"
}, headers=headers)
print(f"   Response: {resp2.json()['response']}")

time.sleep(2)

# Step 4: Third message - add specific detail
print("\n[4] Sending: 'My favorite chess opening is the Sicilian Defense'")
resp3 = requests.post(f"{BASE_URL}/chat", json={
    "message": "My favorite chess opening is the Sicilian Defense"
}, headers=headers)
print(f"   Response: {resp3.json()['response']}")

# Wait for ingestion AND Wisdom Layer flush (30 second interval)
print("\n[5] Waiting 45 seconds for memory ingestion and wisdom processing...")
time.sleep(45)

# Step 5: Recall test
print("\n[6] Asking: 'What do you know about me and my hobbies?'")
resp4 = requests.post(f"{BASE_URL}/chat", json={
    "message": "What do you know about me and my hobbies?"
}, headers=headers)
recall_response = resp4.json()['response']
print(f"   Response: {recall_response}")

# Analysis
print("\n" + "="*60)
print("ANALYSIS")
print("="*60)

keywords = ["alex", "chess", "mystery", "agatha", "sicilian", "novels", "reading"]
found = [kw for kw in keywords if kw.lower() in recall_response.lower()]

if found:
    print(f"SUCCESS! Found keywords in recall: {found}")
else:
    print("Response did not contain expected keywords")
    print("(This may indicate the Wisdom Layer needs more time to process)")

print("\n[TEST COMPLETE]")
