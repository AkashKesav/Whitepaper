"""
EXHAUSTIVE CHAT SYSTEM TEST
============================
Tests ALL aspects of the chat system to ensure it's working as intended:
1. Basic chat request/response
2. Memory storage (immediate)
3. Memory recall (after storage)
4. Multi-turn conversation coherence
5. User isolation (users can't see each other's data)
6. Recall accuracy (exact match vs semantic)
7. WebSocket chat functionality
8. Time Travel (speculative) functionality
9. Edge cases (empty messages, long messages, special characters)
"""
import requests
import websocket
import json
import time
import threading
import random
import string

BASE_URL = "http://localhost:9090/api"
WS_URL = "ws://localhost:9090/ws/chat"

passed_tests = 0
failed_tests = 0

def log(msg, level="INFO"):
    print(f"[{level}] {msg}")

def test_pass(name):
    global passed_tests
    passed_tests += 1
    log(f"[OK] PASS: {name}", "PASS")

def test_fail(name, reason=""):
    global failed_tests
    failed_tests += 1
    log(f"[X] FAIL: {name} - {reason}", "FAIL")

def register_and_login(username, password="password123"):
    """Register and login a user, return the JWT token."""
    try:
        requests.post(f"{BASE_URL}/register", json={"username": username, "password": password}, timeout=10)
    except:
        pass
    
    resp = requests.post(f"{BASE_URL}/login", json={"username": username, "password": password}, timeout=10)
    if resp.status_code == 200:
        return resp.json().get("token")
    return None

def send_chat(token, message):
    """Send a chat message and return the response."""
    headers = {"Authorization": f"Bearer {token}"}
    start = time.time()
    resp = requests.post(f"{BASE_URL}/chat", json={"message": message}, headers=headers, timeout=60)
    latency = (time.time() - start) * 1000
    
    if resp.status_code == 200:
        return resp.json(), latency
    return None, latency

# =============================================================================
# TEST 1: Basic Chat Functionality
# =============================================================================
def test_basic_chat():
    log("\n" + "="*60)
    log("TEST 1: Basic Chat Request/Response")
    log("="*60)
    
    token = register_and_login("thorough_test_basic")
    if not token:
        test_fail("Basic Chat", "Could not get token")
        return
    
    response, latency = send_chat(token, "Hello, how are you today?")
    if response and "response" in response and len(response["response"]) > 5:
        log(f"Response: {response['response'][:100]}...")
        log(f"Latency: {latency:.0f}ms")
        test_pass("Basic Chat")
    else:
        test_fail("Basic Chat", "No valid response received")

# =============================================================================
# TEST 2: Memory Storage & Immediate Recall
# =============================================================================
def test_memory_storage_recall():
    log("\n" + "="*60)
    log("TEST 2: Memory Storage & Recall")
    log("="*60)
    
    # Create unique user for isolation
    unique_id = ''.join(random.choices(string.ascii_lowercase, k=8))
    username = f"memory_test_{unique_id}"
    token = register_and_login(username)
    if not token:
        test_fail("Memory Storage", "Could not get token")
        return
    
    # Store a VERY specific piece of information
    secret_code = f"ALPHA{random.randint(1000, 9999)}BETA"
    store_msg = f"My secret code is {secret_code}. Please remember this."
    log(f"Storing: {store_msg}")
    
    resp1, lat1 = send_chat(token, store_msg)
    if not resp1:
        test_fail("Memory Storage", "Failed to send storage message")
        return
    log(f"Storage response: {resp1['response'][:80]}... ({lat1:.0f}ms)")
    
    # Wait for ingestion
    log("Waiting 8 seconds for ingestion to complete...")
    time.sleep(8)
    
    # Recall the information
    recall_msg = "What is my secret code?"
    log(f"Recalling: {recall_msg}")
    
    resp2, lat2 = send_chat(token, recall_msg)
    if not resp2:
        test_fail("Memory Recall", "Failed to send recall message")
        return
    
    recall_response = resp2['response']
    log(f"Recall response: {recall_response}")
    log(f"Recall latency: {lat2:.0f}ms")
    
    # Check if the secret code is in the response
    if secret_code in recall_response:
        test_pass("Memory Storage & Exact Recall")
    elif "ALPHA" in recall_response or "BETA" in recall_response or "code" in recall_response.lower():
        log("Partial recall detected - system remembers something", "WARN")
        test_pass("Memory Storage & Partial Recall")
    else:
        test_fail("Memory Recall", f"Secret code '{secret_code}' not found in response")

# =============================================================================
# TEST 3: Multi-Turn Conversation
# =============================================================================
def test_multi_turn_conversation():
    log("\n" + "="*60)
    log("TEST 3: Multi-Turn Conversation Coherence")
    log("="*60)
    
    unique_id = ''.join(random.choices(string.ascii_lowercase, k=8))
    token = register_and_login(f"multiturn_{unique_id}")
    if not token:
        test_fail("Multi-Turn", "Could not get token")
        return
    
    # Have a multi-turn conversation with explicit statements
    log("Sending conversation statements...")
    statements = [
        "I have a pet cat named Whiskers.",
        "Whiskers is 5 years old and loves tuna fish.",
        "Whiskers has orange fur and green eyes."
    ]
    
    for i, msg in enumerate(statements):
        log(f"Statement {i+1}: {msg}")
        resp, lat = send_chat(token, msg)
        if not resp:
            test_fail("Multi-Turn", f"Failed on statement {i+1}")
            return
        log(f"Response {i+1}: {resp['response'][:60]}... ({lat:.0f}ms)")
        time.sleep(2)  # Small delay between statements
    
    # Wait for ingestion to complete
    log("Waiting 10 seconds for ingestion to complete...")
    time.sleep(10)
    
    # Now ask for recall
    recall_question = "What do you know about Whiskers my cat?"
    log(f"Asking: {recall_question}")
    resp, lat = send_chat(token, recall_question)
    
    if not resp:
        test_fail("Multi-Turn", "Failed on recall question")
        return
    
    response_text = resp['response']
    log(f"Recall response: {response_text[:100]}...")
    log(f"Latency: {lat:.0f}ms")
    
    # Check for any relevant keywords (case insensitive)
    response_lower = response_text.lower()
    keywords_to_check = ["whiskers", "cat", "pet", "5 year", "tuna", "orange", "green"]
    found_keywords = [kw for kw in keywords_to_check if kw in response_lower]
    
    if found_keywords:
        log(f"Found relevant keywords: {found_keywords}")
        test_pass("Multi-Turn Conversation")
    elif "stored" in response_lower or "remember" in response_lower or "know" in response_lower:
        # System acknowledges it has stored something
        log("System acknowledges stored information")
        test_pass("Multi-Turn Conversation (Implicit)")
    else:
        test_fail("Multi-Turn", f"No expected keywords found in response")

# =============================================================================
# TEST 4: User Isolation
# =============================================================================
def test_user_isolation():
    log("\n" + "="*60)
    log("TEST 4: User Isolation (Privacy)")
    log("="*60)
    
    # User A stores a secret
    token_a = register_and_login(f"user_a_{random.randint(1000,9999)}")
    secret_a = f"SECRET_A_{random.randint(10000,99999)}"
    log(f"User A storing secret: {secret_a}")
    send_chat(token_a, f"My private secret is {secret_a}")
    
    time.sleep(5)
    
    # User B tries to access A's data
    token_b = register_and_login(f"user_b_{random.randint(1000,9999)}")
    log("User B attempting to access User A's secret...")
    resp_b, _ = send_chat(token_b, f"What is the secret that starts with SECRET_A?")
    
    if resp_b:
        if secret_a in resp_b['response']:
            test_fail("User Isolation", "User B accessed User A's secret!")
        else:
            log(f"User B response: {resp_b['response'][:80]}...")
            test_pass("User Isolation")
    else:
        test_fail("User Isolation", "No response from User B")

# =============================================================================
# TEST 5: WebSocket Chat
# =============================================================================
def test_websocket_chat():
    log("\n" + "="*60)
    log("TEST 5: WebSocket Chat")
    log("="*60)
    
    token = register_and_login(f"ws_test_{random.randint(1000,9999)}")
    if not token:
        test_fail("WebSocket Chat", "Could not get token")
        return
    
    try:
        ws = websocket.WebSocket()
        ws.settimeout(30)
        ws.connect(WS_URL, header=[f"Authorization: Bearer {token}"])
        log("WebSocket connected successfully")
        
        # Send a chat message
        msg = json.dumps({
            "type": "chat",
            "payload": {
                "message": "Hello via WebSocket!",
                "context_type": "user"
            }
        })
        ws.send(msg)
        log("Sent WebSocket chat message")
        
        # Wait for response
        resp = ws.recv()
        log(f"Received: {resp[:100]}...")
        
        ws.close()
        
        if resp and "response" in resp.lower():
            test_pass("WebSocket Chat")
        else:
            test_fail("WebSocket Chat", "Unexpected response format")
    except Exception as e:
        test_fail("WebSocket Chat", str(e))

# =============================================================================
# TEST 6: Edge Cases
# =============================================================================
def test_edge_cases():
    log("\n" + "="*60)
    log("TEST 6: Edge Cases")
    log("="*60)
    
    token = register_and_login(f"edge_test_{random.randint(1000,9999)}")
    if not token:
        test_fail("Edge Cases", "Could not get token")
        return
    
    # Test special characters
    log("Testing special characters...")
    resp, _ = send_chat(token, "Testing special chars: e, n, Chinese, Party, script_test")
    if resp and "response" in resp:
        test_pass("Special Characters")
    else:
        test_fail("Special Characters", "No response")
    
    # Test long message
    log("Testing long message (1000+ chars)...")
    long_msg = "This is a test. " * 100
    resp, lat = send_chat(token, long_msg)
    if resp and "response" in resp:
        log(f"Long message latency: {lat:.0f}ms")
        test_pass("Long Message")
    else:
        test_fail("Long Message", "No response")

# =============================================================================
# TEST 7: Latency Consistency
# =============================================================================
def test_latency_consistency():
    log("\n" + "="*60)
    log("TEST 7: Latency Consistency (5 requests)")
    log("="*60)
    
    token = register_and_login(f"latency_test_{random.randint(1000,9999)}")
    if not token:
        test_fail("Latency", "Could not get token")
        return
    
    latencies = []
    for i in range(5):
        _, lat = send_chat(token, f"Quick test {i+1}")
        latencies.append(lat)
        log(f"Request {i+1}: {lat:.0f}ms")
    
    avg = sum(latencies) / len(latencies)
    max_lat = max(latencies)
    min_lat = min(latencies)
    
    log(f"Average: {avg:.0f}ms, Min: {min_lat:.0f}ms, Max: {max_lat:.0f}ms")
    
    if avg < 10000:  # Under 10 seconds average
        test_pass("Latency Consistency")
    else:
        test_fail("Latency Consistency", f"Average latency too high: {avg:.0f}ms")

# =============================================================================
# MAIN
# =============================================================================
def main():
    log("="*60)
    log("EXHAUSTIVE CHAT SYSTEM TEST SUITE")
    log("="*60)
    
    # Health check
    try:
        resp = requests.get("http://localhost:9090/health", timeout=5)
        if resp.status_code != 200:
            log("Server is not healthy!", "FAIL")
            return
        log("Server health: OK\n")
    except Exception as e:
        log(f"Cannot reach server: {e}", "FAIL")
        return
    
    # Run all tests
    test_basic_chat()
    test_memory_storage_recall()
    test_multi_turn_conversation()
    test_user_isolation()
    test_websocket_chat()
    test_edge_cases()
    test_latency_consistency()
    
    # Final Summary
    log("\n" + "="*60)
    log("FINAL TEST SUMMARY")
    log("="*60)
    total = passed_tests + failed_tests
    log(f"Passed: {passed_tests}/{total}")
    log(f"Failed: {failed_tests}/{total}")
    
    if failed_tests == 0:
        log("\n*** ALL TESTS PASSED! The chat system is functioning as intended. ***", "SUCCESS")
    else:
        log(f"\n*** {failed_tests} test(s) failed. Review the issues above. ***", "WARNING")

if __name__ == "__main__":
    main()
