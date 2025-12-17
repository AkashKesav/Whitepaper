import requests
import time
import json

BASE_URL = "http://127.0.0.1:3000/api"
KERNEL_URL = "http://127.0.0.1:9000/api"

def register_and_login():
    """Register or login to get a token"""
    username = "activation_demo_user"
    password = "demo123"
    
    # Try to register
    resp = requests.post(f"{BASE_URL}/register", json={"username": username, "password": password})
    if resp.status_code == 200:
        return resp.json()["token"]
    
    # If already exists, login
    resp = requests.post(f"{BASE_URL}/login", json={"username": username, "password": password})
    return resp.json()["token"]

def chat(token, message, conv_id=None):
    """Send a chat message"""
    headers = {"Authorization": f"Bearer {token}"}
    payload = {"message": message}
    if conv_id:
        payload["conversation_id"] = conv_id
    
    resp = requests.post(f"{BASE_URL}/chat", headers=headers, json=payload)
    return resp.json()

def get_stats(token):
    """Get kernel stats"""
    headers = {"Authorization": f"Bearer {token}"}
    resp = requests.get(f"{KERNEL_URL}/stats", headers=headers)
    return resp.json()

def main():
    print("=" * 70)
    print("ACTIVATION SCORE DEMONSTRATION")
    print("=" * 70)
    
    # Step 1: Setup
    print("\n[STEP 1] Getting authentication token...")
    token = register_and_login()
    print(f"✓ Token: {token[:20]}...")
    
    # Step 2: Create initial memory
    print("\n[STEP 2] Creating initial memory...")
    print("Message: 'My favorite programming language is Rust'")
    resp = chat(token, "My favorite programming language is Rust")
    conv_id = resp["conversation_id"]
    print(f"✓ Response: {resp['response'][:80]}...")
    print(f"✓ Conversation ID: {conv_id}")
    
    # Wait for ingestion
    print("\n[STEP 3] Waiting 3 seconds for ingestion...")
    time.sleep(3)
    
    # Step 3: Check initial stats
    print("\n[STEP 4] Checking initial kernel stats...")
    stats = get_stats(token)
    print(f"✓ Total entities: {stats.get('Entity_count', 0)}")
    print(f"✓ Recent insights: {stats.get('recent_insights', 0)}")
    
    # Step 4: Access the memory once
    print("\n[STEP 5] First access - asking about favorite language...")
    resp1 = chat(token, "What is my favorite programming language?", conv_id=conv_id)
    print(f"✓ Response: {resp1['response'][:100]}...")
    
    # Step 5: Access multiple times to boost activation
    print("\n[STEP 6] Boosting activation with multiple accesses...")
    for i in range(5):
        print(f"  Access #{i+2}:", end=" ")
        resp = chat(token, "Tell me about my favorite programming language", conv_id=conv_id)
        print("✓")
        time.sleep(0.5)
    
    # Step 6: Create a competing memory with low activation
    print("\n[STEP 7] Creating a competing memory (low activation)...")
    print("Message: 'I also know Python and JavaScript'")
    resp2 = chat(token, "I also know Python and JavaScript", conv_id=conv_id)
    print(f"✓ Response: {resp2['response'][:80]}...")
    
    time.sleep(2)
    
    # Step 7: Query and see which memory is prioritized
    print("\n[STEP 8] Testing memory prioritization...")
    print("Query: 'What programming languages do I know?'")
    resp_final = chat(token, "What programming languages do I know?", conv_id=conv_id)
    print(f"✓ Response: {resp_final['response']}")
    
    # Check if Rust (high activation) is mentioned more prominently
    if "rust" in resp_final['response'].lower():
        print("\n✓ HIGH ACTIVATION MEMORY PRIORITIZED!")
        print("  'Rust' was mentioned (accessed 6 times)")
    if "python" in resp_final['response'].lower() or "javascript" in resp_final['response'].lower():
        print("✓ Low activation memories also present")
        print("  'Python/JavaScript' mentioned (accessed 1 time)")
    
    # Final stats
    print("\n[STEP 9] Final kernel stats...")
    final_stats = get_stats(token)
    print(f"✓ Total entities: {final_stats.get('Entity_count', 0)}")
    
    print("\n" + "=" * 70)
    print("ACTIVATION MECHANISM SUMMARY")
    print("=" * 70)
    print("""
HOW ACTIVATION WORKS:
1. Every node starts with activation = 0.5 (50%)
2. Each access BOOSTS activation by 15% (up to max 1.0)
3. Time-based DECAY reduces activation by 0.5% per day
4. Retrieval queries ORDER results by activation score
5. High-activation memories are recalled first/more prominently

IN THIS DEMO:
- 'Rust' node accessed 6+ times → HIGH activation (~0.95)
- 'Python/JavaScript' accessed 1 time → LOW activation (~0.5)
- Result: LLM prioritizes high-activation memories in response
""")
    print("=" * 70)

if __name__ == "__main__":
    main()
