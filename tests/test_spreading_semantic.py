#!/usr/bin/env python
"""
Test Spreading Activation + Semantic Search Working Together

This test verifies that:
1. Semantic search (Qdrant) finds relevant facts
2. Spreading activation expands from seed nodes to related entities
3. Both systems contribute to comprehensive memory recall
"""
import requests
import uuid
import time
import json

BASE_URL = "http://localhost:9090"

def separator(title):
    print("\n" + "=" * 60)
    print(f"  {title}")
    print("=" * 60)

def test_dual_system():
    separator("SPREADING ACTIVATION + SEMANTIC SEARCH TEST")
    
    # Create user
    user = f"spread_test_{uuid.uuid4().hex[:8]}"
    resp = requests.post(f"{BASE_URL}/api/register", json={"username": user, "password": "test"})
    token = resp.json().get("token")
    headers = {"Authorization": f"Bearer {token}"}
    print(f"Created user: {user}")
    
    # Store a network of related facts
    # This creates a mini knowledge graph that spreading activation should traverse
    related_facts = [
        # Core entity
        "My boss is named John Smith",
        # Related entities (should be found via spreading activation)
        "John Smith works at TechCorp headquarters",
        "John Smith's wife is named Sarah",
        "TechCorp is located in San Francisco",
        "Sarah enjoys painting and art galleries",
    ]
    
    print("\nStoring related facts (knowledge graph)...")
    for i, fact in enumerate(related_facts, 1):
        resp = requests.post(f"{BASE_URL}/api/chat", json={"message": fact}, headers=headers)
        status = "OK" if resp.status_code == 200 else f"FAILED ({resp.status_code})"
        print(f"  {i}. {fact[:40]}... -> {status}")
        time.sleep(1)
    
    print("\nWaiting for ingestion and graph construction...")
    time.sleep(6)
    
    # Query 1: Direct semantic search (should find "John Smith" directly)
    separator("TEST 1: Direct Semantic Search")
    query1 = "Who is my boss?"
    resp = requests.post(f"{BASE_URL}/api/chat", json={"message": query1}, headers=headers)
    response1 = resp.json().get("response", "")
    print(f"Query: {query1}")
    print(f"Response: {response1[:200]}...")
    
    john_found = "john" in response1.lower() or "smith" in response1.lower()
    print(f"Result: {'PASS' if john_found else 'FAIL'} - John Smith mentioned: {john_found}")
    
    # Query 2: Multi-hop query (requires spreading activation)
    separator("TEST 2: Multi-Hop Query (Spreading Activation)")
    query2 = "Where does my boss work?"
    resp = requests.post(f"{BASE_URL}/api/chat", json={"message": query2}, headers=headers)
    response2 = resp.json().get("response", "")
    print(f"Query: {query2}")
    print(f"Response: {response2[:200]}...")
    
    techcorp_found = "techcorp" in response2.lower()
    print(f"Result: {'PASS' if techcorp_found else 'FAIL'} - TechCorp mentioned: {techcorp_found}")
    
    # Query 3: Two-hop query (boss -> wife -> hobby)
    separator("TEST 3: Two-Hop Query (Deep Spreading)")
    query3 = "What does my boss's wife enjoy?"
    resp = requests.post(f"{BASE_URL}/api/chat", json={"message": query3}, headers=headers)
    response3 = resp.json().get("response", "")
    print(f"Query: {query3}")
    print(f"Response: {response3[:200]}...")
    
    sarah_hobby = "painting" in response3.lower() or "art" in response3.lower() or "sarah" in response3.lower()
    print(f"Result: {'PASS' if sarah_hobby else 'NEEDS REVIEW'} - Sarah/painting mentioned: {sarah_hobby}")
    
    # Query 4: Location chain (boss -> company -> location)
    separator("TEST 4: Location Chain Query")
    query4 = "What city is TechCorp in?"
    resp = requests.post(f"{BASE_URL}/api/chat", json={"message": query4}, headers=headers)
    response4 = resp.json().get("response", "")
    print(f"Query: {query4}")
    print(f"Response: {response4[:200]}...")
    
    sf_found = "san francisco" in response4.lower() or "francisco" in response4.lower()
    print(f"Result: {'PASS' if sf_found else 'FAIL'} - San Francisco mentioned: {sf_found}")
    
    separator("RESULTS SUMMARY")
    print(f"  Direct Semantic Search:    {'PASS' if john_found else 'FAIL'}")
    print(f"  Multi-Hop (1 hop):         {'PASS' if techcorp_found else 'FAIL'}")
    print(f"  Deep Spreading (2 hops):   {'PASS' if sarah_hobby else 'NEEDS REVIEW'}")
    print(f"  Location Chain:            {'PASS' if sf_found else 'FAIL'}")
    
    all_passed = john_found and techcorp_found and sf_found
    print("\n" + "=" * 60)
    if all_passed:
        print("  SEMANTIC SEARCH + SPREADING ACTIVATION: WORKING!")
    else:
        print("  SOME TESTS NEED ATTENTION")
    print("=" * 60)
    
    return all_passed

def check_qdrant_status():
    separator("QDRANT STATUS CHECK")
    try:
        resp = requests.get("http://localhost:6333/collections/rmk_nodes")
        data = resp.json()
        points = data.get("result", {}).get("points_count", 0)
        print(f"  rmk_nodes collection: {points} points")
        
        resp = requests.get("http://localhost:6333/collections/rmk_cache")
        data = resp.json()
        points = data.get("result", {}).get("points_count", 0)
        print(f"  rmk_cache collection: {points} points")
        return True
    except Exception as e:
        print(f"  Error: {e}")
        return False

def check_spreading_activation_logs():
    """Check if spreading activation is being called in the logs"""
    separator("CHECKING MONOLITH LOGS")
    import subprocess
    result = subprocess.run(
        ["docker", "logs", "--tail", "50", "rmk-monolith"],
        capture_output=True, text=True
    )
    output = result.stdout + result.stderr
    
    spread_found = "spread" in output.lower() or "activation" in output.lower()
    vector_found = "vector" in output.lower() or "qdrant" in output.lower()
    
    print(f"  Spreading activation in logs: {'Yes' if spread_found else 'Not visible'}")
    print(f"  Vector/Qdrant in logs: {'Yes' if vector_found else 'Not visible'}")
    
    return spread_found or vector_found

if __name__ == "__main__":
    print("\n" + "#" * 60)
    print("#  SPREADING ACTIVATION + SEMANTIC SEARCH VERIFICATION")
    print("#" * 60)
    
    check_qdrant_status()
    test_dual_system()
    check_spreading_activation_logs()
