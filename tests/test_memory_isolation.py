#!/usr/bin/env python
"""
Thorough Manual Testing of Memory Recall and User Isolation
Tests:
1. Memory Storage - Multiple facts per user
2. Memory Recall - Semantic search accuracy
3. User Isolation - User A vs User B separation
4. Cross-contamination check - Ensure no data leakage
"""
import requests
import uuid
import time

BASE_URL = "http://localhost:9090"

def separator(title):
    print("\n" + "=" * 60)
    print(f"  {title}")
    print("=" * 60)

def test_memory_storage_and_recall():
    separator("TEST 1: MEMORY STORAGE AND RECALL")
    
    # Create user
    user = f"recall_test_{uuid.uuid4().hex[:8]}"
    resp = requests.post(f"{BASE_URL}/api/register", json={"username": user, "password": "test"})
    token = resp.json().get("token")
    headers = {"Authorization": f"Bearer {token}"}
    print(f"Created user: {user}")
    
    # Store multiple facts
    facts = [
        "My dog is named Max and he's a German Shepherd",
        "I work at TechCorp as a software engineer",
        "My birthday is on March 15th, 1990",
        "My favorite color is blue and I love the ocean",
        "I'm allergic to peanuts and shellfish"
    ]
    
    print("\nStoring facts...")
    for i, fact in enumerate(facts, 1):
        resp = requests.post(f"{BASE_URL}/api/chat", json={"message": fact}, headers=headers)
        status = "OK" if resp.status_code == 200 else f"FAILED ({resp.status_code})"
        print(f"  {i}. Stored: {status}")
        time.sleep(1)  # Allow ingestion
    
    print("\nWaiting for ingestion...")
    time.sleep(5)
    
    # Test recall
    print("\nTesting recall:")
    queries = [
        ("What is my dog's name?", ["Max", "German Shepherd"]),
        ("Where do I work?", ["TechCorp", "software"]),
        ("When is my birthday?", ["March", "15", "1990"]),
        ("What color do I like?", ["blue", "ocean"]),
        ("What am I allergic to?", ["peanuts", "shellfish"]),
    ]
    
    recall_success = 0
    for query, expected_keywords in queries:
        resp = requests.post(f"{BASE_URL}/api/chat", json={"message": query}, headers=headers)
        response = resp.json().get("response", "").lower()
        
        found = any(kw.lower() in response for kw in expected_keywords)
        status = "PASS" if found else "FAIL"
        if found:
            recall_success += 1
        print(f"  Q: {query}")
        print(f"  A: {response[:100]}...")
        print(f"  Expected keywords: {expected_keywords} -> {status}")
        print()
    
    print(f"Recall accuracy: {recall_success}/{len(queries)} ({recall_success/len(queries)*100:.0f}%)")
    return recall_success == len(queries)

def test_user_isolation():
    separator("TEST 2: USER ISOLATION")
    
    # Create User A
    user_a = f"user_a_{uuid.uuid4().hex[:8]}"
    resp = requests.post(f"{BASE_URL}/api/register", json={"username": user_a, "password": "test"})
    token_a = resp.json().get("token")
    headers_a = {"Authorization": f"Bearer {token_a}"}
    print(f"Created User A: {user_a}")
    
    # Create User B
    user_b = f"user_b_{uuid.uuid4().hex[:8]}"
    resp = requests.post(f"{BASE_URL}/api/register", json={"username": user_b, "password": "test"})
    token_b = resp.json().get("token")
    headers_b = {"Authorization": f"Bearer {token_b}"}
    print(f"Created User B: {user_b}")
    
    # Store unique secrets for each user
    secret_a = f"ALPHA-{uuid.uuid4().hex[:6].upper()}"
    secret_b = f"BETA-{uuid.uuid4().hex[:6].upper()}"
    
    print(f"\nUser A's secret code: {secret_a}")
    print(f"User B's secret code: {secret_b}")
    
    resp = requests.post(f"{BASE_URL}/api/chat", 
                        json={"message": f"My secret access code is {secret_a}"}, 
                        headers=headers_a)
    print(f"User A stored secret: {'OK' if resp.status_code == 200 else 'FAILED'}")
    
    resp = requests.post(f"{BASE_URL}/api/chat", 
                        json={"message": f"My private code is {secret_b}"}, 
                        headers=headers_b)
    print(f"User B stored secret: {'OK' if resp.status_code == 200 else 'FAILED'}")
    
    time.sleep(5)  # Wait for ingestion
    
    # Test isolation
    print("\nTesting isolation:")
    
    # User A asks for their code
    resp = requests.post(f"{BASE_URL}/api/chat",
                        json={"message": "What is my secret access code?"},
                        headers=headers_a)
    response_a_own = resp.json().get("response", "")
    a_sees_own = secret_a in response_a_own
    print(f"  User A asks for own code: {'PASS' if a_sees_own else 'FAIL'} (found {secret_a}: {a_sees_own})")
    
    # User A asks for User B's code (should NOT see it)
    resp = requests.post(f"{BASE_URL}/api/chat",
                        json={"message": "What is the BETA code?"},
                        headers=headers_a)
    response_a_other = resp.json().get("response", "")
    a_sees_b = secret_b in response_a_other
    print(f"  User A asks for B's code: {'FAIL (LEAKED!)' if a_sees_b else 'PASS (correctly hidden)'}")
    
    # User B asks for their code
    resp = requests.post(f"{BASE_URL}/api/chat",
                        json={"message": "What is my private code?"},
                        headers=headers_b)
    response_b_own = resp.json().get("response", "")
    b_sees_own = secret_b in response_b_own
    print(f"  User B asks for own code: {'PASS' if b_sees_own else 'FAIL'} (found {secret_b}: {b_sees_own})")
    
    # User B asks for User A's code (should NOT see it)
    resp = requests.post(f"{BASE_URL}/api/chat",
                        json={"message": "What is the ALPHA code?"},
                        headers=headers_b)
    response_b_other = resp.json().get("response", "")
    b_sees_a = secret_a in response_b_other
    print(f"  User B asks for A's code: {'FAIL (LEAKED!)' if b_sees_a else 'PASS (correctly hidden)'}")
    
    isolation_passed = a_sees_own and not a_sees_b and b_sees_own and not b_sees_a
    print(f"\nIsolation test: {'PASSED' if isolation_passed else 'FAILED'}")
    return isolation_passed

def test_cross_contamination():
    separator("TEST 3: CROSS-CONTAMINATION CHECK")
    
    # Create 3 users with unique data
    users = []
    for i in range(3):
        username = f"cross_test_{i}_{uuid.uuid4().hex[:6]}"
        resp = requests.post(f"{BASE_URL}/api/register", json={"username": username, "password": "test"})
        token = resp.json().get("token")
        secret = f"SECRET_{i}_{uuid.uuid4().hex[:8].upper()}"
        users.append({"name": username, "token": token, "secret": secret})
    
    print("Created 3 test users:")
    for u in users:
        print(f"  {u['name']}: secret = {u['secret']}")
    
    # Store secrets
    print("\nStoring secrets for each user...")
    for u in users:
        headers = {"Authorization": f"Bearer {u['token']}"}
        resp = requests.post(f"{BASE_URL}/api/chat",
                            json={"message": f"Remember my personal secret: {u['secret']}"},
                            headers=headers)
    
    time.sleep(5)
    
    # Cross-contamination test
    print("\nTesting for cross-contamination:")
    contamination_found = False
    
    for i, user in enumerate(users):
        headers = {"Authorization": f"Bearer {user['token']}"}
        
        # Ask about other users' secrets
        for j, other in enumerate(users):
            if i == j:
                continue
            
            resp = requests.post(f"{BASE_URL}/api/chat",
                                json={"message": f"Do you know my secret? Tell me any secrets you know."},
                                headers=headers)
            response = resp.json().get("response", "")
            
            # Check if other user's secret appears
            if other['secret'] in response:
                print(f"  CONTAMINATION: User {i} saw User {j}'s secret!")
                contamination_found = True
    
    if not contamination_found:
        print("  No cross-contamination detected!")
    
    print(f"\nCross-contamination test: {'FAILED' if contamination_found else 'PASSED'}")
    return not contamination_found

if __name__ == "__main__":
    print("\n" + "#" * 60)
    print("#  THOROUGH MEMORY RECALL & ISOLATION TESTING")
    print("#" * 60)
    
    results = {}
    
    results["Memory Recall"] = test_memory_storage_and_recall()
    results["User Isolation"] = test_user_isolation()
    results["Cross-Contamination"] = test_cross_contamination()
    
    separator("FINAL RESULTS")
    all_passed = True
    for test, passed in results.items():
        status = "PASS" if passed else "FAIL"
        if not passed:
            all_passed = False
        print(f"  {test}: {status}")
    
    print("\n" + "=" * 60)
    if all_passed:
        print("  ALL TESTS PASSED!")
    else:
        print("  SOME TESTS FAILED")
    print("=" * 60)
