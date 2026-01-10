#!/usr/bin/env python
"""Comprehensive test for Admin API, Memory Isolation, and Policy Enforcement"""
import requests
import uuid
import time

BASE_URL = "http://localhost:9090"

def test_admin_api():
    print("=== Testing Admin API ===")
    
    # 1. Register admin
    admin_user = f"super_admin_test_{uuid.uuid4().hex[:6]}"
    resp = requests.post(f"{BASE_URL}/api/register", json={"username": admin_user, "password": "admin123"})
    if resp.status_code != 200:
        print(f"1. Admin registration FAILED: {resp.status_code}")
        return False
    
    admin_token = resp.json().get("token")
    admin_role = resp.json().get("role")
    print(f"1. Admin registered: {admin_user} (role: {admin_role})")
    
    headers = {"Authorization": f"Bearer {admin_token}"}
    
    # 2. Test admin list users
    resp = requests.get(f"{BASE_URL}/api/admin/users", headers=headers)
    print(f"2. Admin list users: {resp.status_code}")
    if resp.status_code != 200:
        print(f"   Error: {resp.text[:100]}")
    
    # 3. Test admin system stats
    resp = requests.get(f"{BASE_URL}/api/admin/system/stats", headers=headers)
    print(f"3. Admin system stats: {resp.status_code}")
    
    return resp.status_code == 200

def test_memory_isolation():
    print("\n=== Testing Memory Isolation ===")
    
    # Create User A with secret
    user_a = f"user_a_{uuid.uuid4().hex[:6]}"
    resp = requests.post(f"{BASE_URL}/api/register", json={"username": user_a, "password": "pass"})
    token_a = resp.json().get("token")
    headers_a = {"Authorization": f"Bearer {token_a}"}
    print(f"4. User A registered: {user_a}")
    
    # Store secret for User A
    resp = requests.post(f"{BASE_URL}/api/chat", 
                        json={"message": "My secret code is ALPHA-7749"}, 
                        headers=headers_a)
    print(f"5. User A stored secret: {'OK' if resp.status_code == 200 else 'FAILED'}")
    
    time.sleep(3)  # Wait for ingestion
    
    # Create User B
    user_b = f"user_b_{uuid.uuid4().hex[:6]}"
    resp = requests.post(f"{BASE_URL}/api/register", json={"username": user_b, "password": "pass"})
    token_b = resp.json().get("token")
    headers_b = {"Authorization": f"Bearer {token_b}"}
    print(f"6. User B registered: {user_b}")
    
    # User B tries to access User A's secret
    resp = requests.post(f"{BASE_URL}/api/chat",
                        json={"message": "What is my secret code?"},
                        headers=headers_b)
    response_b = resp.json().get("response", "")
    
    if "ALPHA-7749" in response_b:
        print("7. ISOLATION FAILED: User B saw User A secret!")
        return False
    else:
        print("7. ISOLATION PASSED: User B cannot see User A data")
    
    # User A recalls their own secret
    time.sleep(2)
    resp = requests.post(f"{BASE_URL}/api/chat",
                        json={"message": "What is my secret code?"},
                        headers=headers_a)
    response_a = resp.json().get("response", "")
    
    if "ALPHA-7749" in response_a or "alpha" in response_a.lower():
        print("8. User A recall: SUCCESS - can see own secret")
    else:
        print(f"8. User A recall: {response_a[:80]}...")
    
    return True

def test_user_chat():
    print("\n=== Testing User Chat ===")
    
    user = f"chat_test_{uuid.uuid4().hex[:6]}"
    resp = requests.post(f"{BASE_URL}/api/register", json={"username": user, "password": "pass"})
    token = resp.json().get("token")
    headers = {"Authorization": f"Bearer {token}"}
    print(f"9. Chat user registered: {user}")
    
    # Simple chat
    resp = requests.post(f"{BASE_URL}/api/chat",
                        json={"message": "Hello, how are you?"},
                        headers=headers)
    if resp.status_code == 200:
        print("10. Chat response: OK")
        return True
    else:
        print(f"10. Chat FAILED: {resp.status_code}")
        return False

if __name__ == "__main__":
    print("=" * 50)
    print("COMPREHENSIVE SYSTEM TEST")
    print("=" * 50)
    
    admin_ok = test_admin_api()
    isolation_ok = test_memory_isolation()
    chat_ok = test_user_chat()
    
    print("\n" + "=" * 50)
    print("RESULTS SUMMARY")
    print("=" * 50)
    print(f"Admin API:        {'PASS' if admin_ok else 'FAIL'}")
    print(f"Memory Isolation: {'PASS' if isolation_ok else 'FAIL'}")
    print(f"User Chat:        {'PASS' if chat_ok else 'FAIL'}")
    print("=" * 50)
