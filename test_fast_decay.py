"""
Quick test for fast decay (1 minute interval)
"""
import requests
import time

BASE_URL = "http://127.0.0.1:3000/api"
KERNEL_URL = "http://127.0.0.1:9000/api"

def register_and_login():
    username = "fast_decay_test"
    password = "test123"
    
    resp = requests.post(f"{BASE_URL}/register", json={"username": username, "password": password})
    if resp.status_code == 200:
        return resp.json()["token"]
    
    resp = requests.post(f"{BASE_URL}/login", json={"username": username, "password": password})
    return resp.json()["token"]

def chat(token, message, conv_id=None):
    headers = {"Authorization": f"Bearer {token}"}
    payload = {"message": message}
    if conv_id:
        payload["conversation_id"] = conv_id
    resp = requests.post(f"{BASE_URL}/chat", headers=headers, json=payload)
    return resp.json()

def trigger_reflection():
    resp = requests.post(f"{KERNEL_URL}/reflect")
    return resp.status_code == 200

print("=" * 70)
print("FAST DECAY TEST (1 minute intervals)")
print("=" * 70)

# Setup
print("\n[1] Authenticating...")
token = register_and_login()
print("✓ Token obtained")

# Create memory
print("\n[2] Creating memory: 'I love raspberry pie'")
resp = chat(token, "I love raspberry pie")
conv_id = resp["conversation_id"]
print(f"✓ Response: {resp['response'][:60]}...")

# Wait for ingestion
print("\n[3] Waiting 3 seconds for ingestion...")
time.sleep(3)

# Boost activation
print("\n[4] Boosting activation with 3 accesses...")
for i in range(3):
    chat(token, "What do I love?", conv_id=conv_id)
    print(f"  Access #{i+1}: ✓")
    time.sleep(0.2)

print("\n[5] Current state:")
print("  - 'raspberry pie' node has high activation (~0.5 + 3*0.15 = 0.95)")
print("  - last_accessed = NOW")

# Wait 90 seconds for decay to kick in
print("\n[6] Waiting 90 seconds for decay...")
print("  - Decay runs every 1 minute")
print("  - Protection window is 1 minute")
print("  - After 90s, decay should have run at least once")

for i in range(9):
    time.sleep(10)
    print(f"  {(i+1)*10}s elapsed...")

# Trigger manual reflection to force decay check
print("\n[7] Triggering manual reflection/decay...")
if trigger_reflection():
    print("✓ Reflection triggered")
else:
    print("⚠ Trigger may have failed")

time.sleep(2)

# Test if memory is still strong or decayed
print("\n[8] Testing memory retrieval...")
resp = chat(token, "What dessert do I like?", conv_id=conv_id)
print(f"Response: {resp['response']}")

if "raspberry" in resp['response'].lower() or "pie" in resp['response'].lower():
    print("\n✓ Memory retrieved (may have decayed but still present)")
else:
    print("\n✓ Memory may have decayed significantly")

print("\n" + "=" * 70)
print("FAST DECAY CONFIGURATION:")
print("=" * 70)
print("""
✓ Decay runs every: 1 MINUTE (was 1 hour)
✓ Protection window: 1 MINUTE (was 24 hours)
✓ Decay rate: 0.5% per day (unchanged)

This allows you to see decay effects within minutes instead of days!

Note: Decay amount per day is still only 0.5%, so even after 90 seconds,
the decay is minimal. To see bigger effects, you'd need to:
1. Wait longer (multiple hours)
2. Increase DecayRate in schema.go (e.g., 0.05 = 5%/day)
""")
print("=" * 70)
