"""
Activation Value Monitor
This script tracks activation values before and after decay to verify it's working
"""
import requests
import time
import json
from datetime import datetime

BASE_URL = "http://127.0.0.1:3000/api"
KERNEL_URL = "http://127.0.0.1:9000/api"

def register_and_login():
    username = "activation_monitor"
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

def get_stats(token):
    """Get kernel stats which includes high activation nodes"""
    headers = {"Authorization": f"Bearer {token}"}
    resp = requests.get(f"{KERNEL_URL}/stats", headers=headers)
    return resp.json()

def trigger_reflection():
    """Manually trigger reflection cycle which includes decay"""
    resp = requests.post(f"{KERNEL_URL}/reflect")
    return resp.status_code == 200

print("=" * 80)
print("ACTIVATION VALUE MONITOR - Tracking Decay Effects")
print("=" * 80)

# Step 1: Setup
print("\n[STEP 1] Authenticating...")
token = register_and_login()
print(f"✓ Token obtained")

# Step 2: Create some memories with different activation levels
print("\n[STEP 2] Creating memories with different activation levels...")

memories = [
    "I love pizza with extra cheese",
    "My favorite color is turquoise", 
    "I enjoy playing tennis on weekends",
]

conv_id = None
for i, memory in enumerate(memories):
    resp = chat(token, memory, conv_id)
    if not conv_id:
        conv_id = resp["conversation_id"]
    print(f"  Memory {i+1}: ✓")
    time.sleep(0.5)

# Step 3: Boost some memories more than others
print("\n[STEP 3] Boosting memories with different access counts...")
print("  Memory 1 (pizza): 5 accesses")
for _ in range(5):
    chat(token, "What food do I like?", conv_id)
    time.sleep(0.2)

print("  Memory 2 (turquoise): 2 accesses")
for _ in range(2):
    chat(token, "What color do I like?", conv_id)
    time.sleep(0.2)

print("  Memory 3 (tennis): 0 additional accesses")
print("✓ Boost complete")

# Wait for ingestion
print("\n[STEP 4] Waiting 5 seconds for ingestion...")
time.sleep(5)

# Step 4: Get initial stats
print("\n[STEP 5] Getting initial stats...")
initial_stats = get_stats(token)

print(f"\nInitial Stats:")
print(f"  Entity count: {initial_stats.get('Entity_count', 0)}")
print(f"  High activation nodes: {initial_stats.get('high_activation_nodes', 0)}")
print(f"  Fact count: {initial_stats.get('Fact_count', 0)}")

# Store timestamp
initial_time = datetime.now()
print(f"\n  Timestamp: {initial_time.strftime('%H:%M:%S')}")

# Step 5: Wait for decay
print("\n[STEP 6] Waiting 2 minutes for decay to run...")
print("  (Decay runs every 1 minute)")
print("  (Protection window: 1 minute)")

for i in range(12):
    time.sleep(10)
    elapsed = (i+1) * 10
    print(f"  {elapsed}s elapsed... ", end="")
    if elapsed % 60 == 0:
        print("(decay should have run)")
    else:
        print()

# Step 6: Trigger manual decay to be sure
print("\n[STEP 7] Triggering manual reflection/decay...")
if trigger_reflection():
    print("✓ Reflection triggered")
else:
    print("⚠ Trigger may have failed")

time.sleep(3)

# Step 7: Get final stats
print("\n[STEP 8] Getting final stats...")
final_stats = get_stats(token)
final_time = datetime.now()

print(f"\nFinal Stats:")
print(f"  Entity count: {final_stats.get('Entity_count', 0)}")
print(f"  High activation nodes: {final_stats.get('high_activation_nodes', 0)}")
print(f"  Fact count: {final_stats.get('Fact_count', 0)}")
print(f"\n  Timestamp: {final_time.strftime('%H:%M:%S')}")

# Step 8: Calculate changes
elapsed_seconds = (final_time - initial_time).total_seconds()
elapsed_minutes = elapsed_seconds / 60

print("\n" + "=" * 80)
print("DECAY ANALYSIS")
print("=" * 80)

print(f"\nTime Elapsed: {elapsed_seconds:.1f} seconds ({elapsed_minutes:.2f} minutes)")
print(f"Expected Decay Cycles: ~{int(elapsed_minutes)} (runs every 1 minute)")

# Compare stats
entity_change = final_stats.get('Entity_count', 0) - initial_stats.get('Entity_count', 0)
high_act_change = final_stats.get('high_activation_nodes', 0) - initial_stats.get('high_activation_nodes', 0)

print(f"\nChanges:")
print(f"  Entity count: {initial_stats.get('Entity_count', 0)} → {final_stats.get('Entity_count', 0)} ({entity_change:+d})")
print(f"  High activation nodes (≥0.7): {initial_stats.get('high_activation_nodes', 0)} → {final_stats.get('high_activation_nodes', 0)} ({high_act_change:+d})")

if high_act_change < 0:
    print(f"\n✓ DECAY DETECTED! {abs(high_act_change)} node(s) dropped below 0.7 activation")
elif high_act_change == 0:
    print(f"\n⚠ No change in high activation count")
    print("  Possible reasons:")
    print("  - Decay amount too small (0.5%/day)")
    print("  - Nodes still above 0.7 threshold")
    print("  - Time elapsed too short")
else:
    print(f"\n⚠ High activation count increased (new nodes created)")

# Theoretical calculation
days_elapsed = elapsed_seconds / (24 * 60 * 60)
decay_rate = 0.005  # 0.5% per day
expected_multiplier = (1 - decay_rate) ** days_elapsed

print(f"\n" + "=" * 80)
print("THEORETICAL DECAY CALCULATION")
print("=" * 80)
print(f"\nTime elapsed: {elapsed_seconds:.1f}s = {days_elapsed:.6f} days")
print(f"Decay rate: {decay_rate * 100}% per day")
print(f"Expected decay multiplier: (1 - {decay_rate})^{days_elapsed:.6f} = {expected_multiplier:.10f}")
print(f"\nFor a node with activation 0.8:")
print(f"  Before: 0.8000")
print(f"  After:  {0.8 * expected_multiplier:.10f}")
print(f"  Change: {0.8 - (0.8 * expected_multiplier):.10f} ({((1 - expected_multiplier) * 100):.6f}%)")

print("\n" + "=" * 80)
print("CONCLUSION")
print("=" * 80)

if days_elapsed < 0.01:  # Less than ~15 minutes
    print("\n⚠ Time elapsed is very short (< 15 minutes)")
    print("  With 0.5%/day decay rate, changes will be tiny")
    print("  Recommendation: Wait longer OR increase decay rate in schema.go")
else:
    print("\n✓ Sufficient time elapsed for measurable decay")
    print("  Check 'High activation nodes' count for evidence of decay")

print("\n" + "=" * 80)
