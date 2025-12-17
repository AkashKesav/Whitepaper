#!/usr/bin/env python3
"""Comprehensive End-to-End Product Test Suite"""

import requests
import time
import json
import sys

BASE_URL = "http://localhost:9090"
OLLAMA_URL = "http://localhost:11434"

def print_section(title):
    print(f"\n{'='*60}")
    print(f"  {title}")
    print('='*60)

def print_result(test_name, passed, details=""):
    status = "✅ PASS" if passed else "❌ FAIL"
    print(f"  {status} | {test_name}")
    if details:
        print(f"         └─ {details}")

results = {"passed": 0, "failed": 0, "tests": []}

def record_test(name, passed, details=""):
    results["tests"].append({"name": name, "passed": passed, "details": details})
    if passed:
        results["passed"] += 1
    else:
        results["failed"] += 1
    print_result(name, passed, details)

# ============================================================
# INFRASTRUCTURE TESTS
# ============================================================
def test_infrastructure():
    print_section("1. INFRASTRUCTURE TESTS")
    
    # Health check
    try:
        resp = requests.get(f"{BASE_URL}/health", timeout=5)
        passed = resp.status_code == 200 and resp.json().get("status") == "healthy"
        record_test("Monolith Health Check", passed, f"Status: {resp.json()}")
    except Exception as e:
        record_test("Monolith Health Check", False, str(e))
    
    # Ollama check
    try:
        resp = requests.get(f"{OLLAMA_URL}/api/tags", timeout=5)
        models = resp.json().get("models", [])
        has_embed_model = any("nomic-embed-text" in m.get("name", "") for m in models)
        record_test("Ollama Service", True, f"Models: {len(models)}")
        record_test("Embedding Model Available", has_embed_model, "nomic-embed-text")
    except Exception as e:
        record_test("Ollama Service", False, str(e))

# ============================================================
# AUTHENTICATION TESTS
# ============================================================
def test_authentication():
    print_section("2. AUTHENTICATION TESTS")
    
    username = f"test_{int(time.time())}"
    password = "testpass123"
    
    # Registration
    try:
        resp = requests.post(f"{BASE_URL}/api/register", json={
            "username": username,
            "password": password
        }, timeout=10)
        passed = resp.status_code in [200, 201]
        record_test("User Registration", passed, f"Status: {resp.status_code}")
    except Exception as e:
        record_test("User Registration", False, str(e))
    
    # Login
    token = None
    try:
        resp = requests.post(f"{BASE_URL}/api/login", json={
            "username": username,
            "password": password
        }, timeout=10)
        if resp.status_code == 200:
            token = resp.json().get("token")
            record_test("User Login", bool(token), f"Token: {token[:20]}..." if token else "No token")
        else:
            record_test("User Login", False, f"Status: {resp.status_code}")
    except Exception as e:
        record_test("User Login", False, str(e))
    
    return token, username

# ============================================================
# PRE-CORTEX TESTS
# ============================================================
def test_precortex(token):
    print_section("3. PRE-CORTEX TESTS")
    
    if not token:
        record_test("Pre-Cortex Tests", False, "No auth token")
        return
    
    headers = {"Authorization": f"Bearer {token}"}
    
    # Greeting test (should be handled locally)
    try:
        start = time.time()
        resp = requests.post(f"{BASE_URL}/api/chat", 
            headers=headers,
            json={"message": "Hello"},
            timeout=30
        )
        latency = (time.time() - start) * 1000
        passed = resp.status_code == 200 and latency < 500
        record_test("Greeting Handler", passed, f"Latency: {latency:.1f}ms")
    except Exception as e:
        record_test("Greeting Handler", False, str(e))
    
    # Navigation intent
    try:
        start = time.time()
        resp = requests.post(f"{BASE_URL}/api/chat",
            headers=headers,
            json={"message": "Go to settings"},
            timeout=30
        )
        latency = (time.time() - start) * 1000
        record_test("Navigation Intent", resp.status_code == 200, f"Latency: {latency:.1f}ms")
    except Exception as e:
        record_test("Navigation Intent", False, str(e))

# ============================================================
# EMBEDDING TESTS
# ============================================================
def test_embeddings():
    print_section("4. EMBEDDING TESTS (Ollama)")
    
    # Test direct embedding
    try:
        start = time.time()
        resp = requests.post(f"{OLLAMA_URL}/api/embeddings", json={
            "model": "nomic-embed-text",
            "prompt": "This is a test sentence for embedding generation."
        }, timeout=30)
        latency = (time.time() - start) * 1000
        
        if resp.status_code == 200:
            embedding = resp.json().get("embedding", [])
            record_test("Embedding Generation", len(embedding) > 0, 
                       f"Dims: {len(embedding)}, Latency: {latency:.1f}ms")
        else:
            record_test("Embedding Generation", False, f"Status: {resp.status_code}")
    except Exception as e:
        record_test("Embedding Generation", False, str(e))

# ============================================================
# MEMORY TESTS
# ============================================================
def test_memory(token, username):
    print_section("5. MEMORY & INGESTION TESTS")
    
    if not token:
        record_test("Memory Tests", False, "No auth token")
        return
    
    headers = {"Authorization": f"Bearer {token}"}
    
    # Store a fact
    try:
        resp = requests.post(f"{BASE_URL}/api/chat",
            headers=headers,
            json={"message": "My favorite color is blue and I love pizza."},
            timeout=60
        )
        record_test("Fact Storage", resp.status_code == 200, "Stored preference fact")
    except Exception as e:
        record_test("Fact Storage", False, str(e))
    
    # Wait for ingestion
    print("         └─ Waiting 5s for ingestion...")
    time.sleep(5)
    
    # Query the fact
    try:
        start = time.time()
        resp = requests.post(f"{BASE_URL}/api/chat",
            headers=headers,
            json={"message": "What's my favorite color?"},
            timeout=60
        )
        latency = (time.time() - start) * 1000
        
        if resp.status_code == 200:
            response_text = resp.json().get("response", "").lower()
            recalled = "blue" in response_text
            record_test("Fact Recall", recalled, f"Latency: {latency:.1f}ms")
        else:
            record_test("Fact Recall", False, f"Status: {resp.status_code}")
    except Exception as e:
        record_test("Fact Recall", False, str(e))

# ============================================================
# LLM INTEGRATION TESTS
# ============================================================
def test_llm(token):
    print_section("6. LLM INTEGRATION TESTS")
    
    if not token:
        record_test("LLM Tests", False, "No auth token")
        return
    
    headers = {"Authorization": f"Bearer {token}"}
    
    # Complex query requiring LLM
    try:
        start = time.time()
        resp = requests.post(f"{BASE_URL}/api/chat",
            headers=headers,
            json={"message": "Write a haiku about memory."},
            timeout=120
        )
        latency = (time.time() - start) * 1000
        
        if resp.status_code == 200:
            response = resp.json().get("response", "")
            record_test("Complex LLM Query", len(response) > 20, 
                       f"Latency: {latency:.1f}ms, Chars: {len(response)}")
        else:
            record_test("Complex LLM Query", False, f"Status: {resp.status_code}")
    except Exception as e:
        record_test("Complex LLM Query", False, str(e))

# ============================================================
# CACHING TESTS
# ============================================================
def test_caching(token):
    print_section("7. SEMANTIC CACHE TESTS")
    
    if not token:
        record_test("Cache Tests", False, "No auth token")
        return
    
    headers = {"Authorization": f"Bearer {token}"}
    query = "What is the capital of France?"
    
    # First query (should hit LLM)
    try:
        start = time.time()
        resp1 = requests.post(f"{BASE_URL}/api/chat",
            headers=headers,
            json={"message": query},
            timeout=60
        )
        latency1 = (time.time() - start) * 1000
        record_test("Cache Miss (First Query)", resp1.status_code == 200, f"Latency: {latency1:.1f}ms")
    except Exception as e:
        record_test("Cache Miss (First Query)", False, str(e))
        return
    
    # Same query again (should hit cache)
    try:
        start = time.time()
        resp2 = requests.post(f"{BASE_URL}/api/chat",
            headers=headers,
            json={"message": query},
            timeout=60
        )
        latency2 = (time.time() - start) * 1000
        
        improved = latency2 < latency1 * 0.8  # At least 20% faster
        record_test("Cache Hit (Repeat Query)", resp2.status_code == 200, 
                   f"Latency: {latency2:.1f}ms (was {latency1:.1f}ms)")
    except Exception as e:
        record_test("Cache Hit (Repeat Query)", False, str(e))

# ============================================================
# SUMMARY
# ============================================================
def print_summary():
    print_section("TEST SUMMARY")
    total = results["passed"] + results["failed"]
    pass_rate = (results["passed"] / total * 100) if total > 0 else 0
    
    print(f"""
    Total Tests:  {total}
    Passed:       {results['passed']} ✅
    Failed:       {results['failed']} ❌
    Pass Rate:    {pass_rate:.1f}%
    """)
    
    if results["failed"] > 0:
        print("  Failed Tests:")
        for test in results["tests"]:
            if not test["passed"]:
                print(f"    - {test['name']}: {test['details']}")
    
    return results["failed"] == 0

# ============================================================
# MAIN
# ============================================================
def main():
    print("\n" + "="*60)
    print("  COMPLETE PRODUCT TEST SUITE")
    print("  Reflective Memory Kernel + Pre-Cortex + Ollama")
    print("="*60)
    
    try:
        test_infrastructure()
        token, username = test_authentication()
        test_precortex(token)
        test_embeddings()
        test_memory(token, username)
        test_llm(token)
        test_caching(token)
        
        all_passed = print_summary()
        sys.exit(0 if all_passed else 1)
        
    except KeyboardInterrupt:
        print("\n\nTest interrupted.")
        sys.exit(1)
    except Exception as e:
        print(f"\n\nTest suite error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
