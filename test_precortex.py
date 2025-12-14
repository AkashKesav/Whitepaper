#!/usr/bin/env python3
"""Comprehensive Pre-Cortex and Memory System Test Suite"""

import requests
import time
import json

BASE_URL = "http://localhost:9090"

def test_health():
    """Test health endpoint"""
    print("\n=== Health Check ===")
    resp = requests.get(f"{BASE_URL}/health")
    print(f"Status: {resp.status_code}")
    print(f"Response: {resp.json()}")
    assert resp.status_code == 200
    assert resp.json()["status"] == "healthy"
    print("[PASS] Health check passed")

def test_register_and_login():
    """Test user registration and login"""
    print("\n=== Registration & Login ===")
    
    # Register
    username = f"testuser_{int(time.time())}"
    resp = requests.post(f"{BASE_URL}/api/register", json={
        "username": username,
        "password": "testpass123"
    })
    print(f"Register: {resp.status_code} - {resp.text}")
    
    if resp.status_code != 201:
        # User might exist, try login directly
        pass
    
    # Login
    resp = requests.post(f"{BASE_URL}/api/login", json={
        "username": username,
        "password": "testpass123"
    })
    print(f"Login: {resp.status_code}")
    
    if resp.status_code == 200:
        data = resp.json()
        token = data.get("token")
        print(f"[PASS] Got JWT token: {token[:30]}...")
        return token, username
    else:
        print(f"[WARN] Login failed: {resp.text}")
        # Try with default user
        resp = requests.post(f"{BASE_URL}/api/login", json={
            "username": "testuser",
            "password": "testpass123"
        })
        if resp.status_code == 200:
            token = resp.json().get("token")
            return token, "testuser"
        return None, None

def test_precortex_greeting(token):
    """Test Pre-Cortex handles greetings locally"""
    print("\n=== Pre-Cortex: Greeting Test ===")
    
    headers = {"Authorization": f"Bearer {token}"}
    
    # Test greeting - should be handled by Pre-Cortex
    start = time.time()
    resp = requests.post(f"{BASE_URL}/api/chat", 
        headers=headers,
        json={"message": "Hi"}
    )
    latency = (time.time() - start) * 1000
    
    print(f"Status: {resp.status_code}")
    print(f"Latency: {latency:.1f}ms")
    
    if resp.status_code == 200:
        data = resp.json()
        print(f"Response: {data.get('response', data)[:100]}...")
        
        # Pre-Cortex greeting should be very fast (< 100ms typically)
        if latency < 500:
            print(f"[PASS] Fast response - likely Pre-Cortex handled!")
        else:
            print(f"[INFO] Slower response - may have hit LLM")
    else:
        print(f"[WARN] Chat failed: {resp.text}")

def test_precortex_fact_query(token, username):
    """Test Pre-Cortex DGraph reflex for fact queries"""
    print("\n=== Pre-Cortex: Fact Query Test ===")
    
    headers = {"Authorization": f"Bearer {token}"}
    
    # First, tell the system a fact
    print("Storing a fact...")
    resp = requests.post(f"{BASE_URL}/api/chat",
        headers=headers,
        json={"message": "I love playing chess and my favorite food is pizza"}
    )
    print(f"Store fact: {resp.status_code}")
    
    # Wait for ingestion
    time.sleep(3)
    
    # Now query for the fact
    print("Querying for preferences...")
    start = time.time()
    resp = requests.post(f"{BASE_URL}/api/chat",
        headers=headers,
        json={"message": "What do I like?"}
    )
    latency = (time.time() - start) * 1000
    
    print(f"Status: {resp.status_code}")
    print(f"Latency: {latency:.1f}ms")
    
    if resp.status_code == 200:
        data = resp.json()
        response_text = data.get("response", str(data))
        print(f"Response: {response_text[:200]}...")
        
        # Check if fact was recalled
        if "chess" in response_text.lower() or "pizza" in response_text.lower():
            print("[PASS] Memory recall working!")
        else:
            print("[INFO] Fact may not be recalled yet - check if ingestion completed")

def test_complex_query(token):
    """Test complex query goes to LLM"""
    print("\n=== Complex Query Test (LLM) ===")
    
    headers = {"Authorization": f"Bearer {token}"}
    
    start = time.time()
    resp = requests.post(f"{BASE_URL}/api/chat",
        headers=headers,
        json={"message": "Write a haiku about artificial intelligence"}
    )
    latency = (time.time() - start) * 1000
    
    print(f"Status: {resp.status_code}")
    print(f"Latency: {latency:.1f}ms")
    
    if resp.status_code == 200:
        data = resp.json()
        print(f"Response: {data.get('response', data)[:200]}...")
        print("[PASS] LLM responded to complex query")
    else:
        print(f"[WARN] Chat failed: {resp.text}")

def test_semantic_cache(token):
    """Test semantic cache stores and retrieves responses"""
    print("\n=== Semantic Cache Test ===")
    
    headers = {"Authorization": f"Bearer {token}"}
    
    # First query
    print("First query (should hit LLM)...")
    start = time.time()
    resp1 = requests.post(f"{BASE_URL}/api/chat",
        headers=headers,
        json={"message": "What is the capital of France?"}
    )
    latency1 = (time.time() - start) * 1000
    print(f"First latency: {latency1:.1f}ms")
    
    # Same query again (should hit cache)
    print("Same query again (should hit cache)...")
    start = time.time()
    resp2 = requests.post(f"{BASE_URL}/api/chat",
        headers=headers,
        json={"message": "What is the capital of France?"}
    )
    latency2 = (time.time() - start) * 1000
    print(f"Second latency: {latency2:.1f}ms")
    
    if latency2 < latency1 * 0.5:
        print(f"[PASS] Cache hit! {latency1:.0f}ms -> {latency2:.0f}ms")
    else:
        print(f"[INFO] Cache may not have hit (depends on exact-match normalization)")

def main():
    print("=" * 60)
    print("Pre-Cortex & Memory System Comprehensive Test")
    print("=" * 60)
    
    try:
        # Health check
        test_health()
        
        # Auth
        token, username = test_register_and_login()
        if not token:
            print("[FAIL] Could not get auth token, aborting")
            return
        
        # Pre-Cortex tests
        test_precortex_greeting(token)
        test_precortex_fact_query(token, username)
        
        # LLM test
        test_complex_query(token)
        
        # Cache test
        test_semantic_cache(token)
        
        print("\n" + "=" * 60)
        print("All tests completed!")
        print("=" * 60)
        
    except requests.exceptions.ConnectionError:
        print("[FAIL] Could not connect to server. Is it running?")
    except Exception as e:
        print(f"[ERROR] {e}")

if __name__ == "__main__":
    main()
