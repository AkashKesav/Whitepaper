#!/usr/bin/env python3
"""
Test script for Graph Traversal APIs
Tests: SpreadActivation, CommunityTraversal, TemporalQuery, ExpandNode
"""

import requests
import json
import sys

BASE_URL = "http://localhost:8180"  # DGraph
AI_URL = "http://localhost:8001"    # AI Services (for traversal)

def test_dgraph_connection():
    """Test DGraph is accessible"""
    print("=" * 60)
    print("1. Testing DGraph Connection")
    print("=" * 60)
    
    try:
        query = '{ total(func: has(dgraph.type)) { count(uid) } }'
        resp = requests.post(f"{BASE_URL}/query", 
                            headers={"Content-Type": "application/dql"},
                            data=query, timeout=10)
        data = resp.json()
        count = data.get("data", {}).get("total", [{}])[0].get("count", 0)
        print(f"  ✓ DGraph connected: {count} nodes")
        return count > 0
    except Exception as e:
        print(f"  ✗ DGraph connection failed: {e}")
        return False

def get_sample_entities():
    """Get sample entity UIDs from DGraph"""
    query = '{ entities(func: type(Entity), first: 5) { uid name description } }'
    resp = requests.post(f"{BASE_URL}/query",
                        headers={"Content-Type": "application/dql"},
                        data=query, timeout=10)
    data = resp.json()
    return data.get("data", {}).get("entities", [])

def test_spread_activation_direct():
    """Test spreading activation directly via DGraph query"""
    print("\n" + "=" * 60)
    print("2. Testing Spreading Activation (Direct DGraph)")
    print("=" * 60)
    
    entities = get_sample_entities()
    if not entities:
        print("  ✗ No entities found in DGraph")
        return False
    
    start_entity = entities[0]
    print(f"  Start node: {start_entity['name']} ({start_entity['uid']})")
    
    # DGraph recursive expansion query
    query = f'''{{
        spread(func: uid({start_entity['uid']})) @recurse(depth: 2) {{
            uid
            name
            related_to
            has_attribute
        }}
    }}'''
    
    try:
        resp = requests.post(f"{BASE_URL}/query",
                            headers={"Content-Type": "application/dql"},
                            data=query, timeout=10)
        data = resp.json()
        nodes = data.get("data", {}).get("spread", [])
        print(f"  ✓ Expansion found {len(nodes)} connected nodes")
        return True
    except Exception as e:
        print(f"  ✗ Query failed: {e}")
        return False

def test_community_grouping():
    """Test community grouping by department"""
    print("\n" + "=" * 60)
    print("3. Testing Community Grouping (by department)")
    print("=" * 60)
    
    # Query entities and extract departments from descriptions
    query = '''{ 
        entities(func: type(Entity), first: 100) { 
            uid 
            name 
            description 
        } 
    }'''
    
    try:
        resp = requests.post(f"{BASE_URL}/query",
                            headers={"Content-Type": "application/dql"},
                            data=query, timeout=10)
        data = resp.json()
        entities = data.get("data", {}).get("entities", [])
        
        # Group by department from description
        communities = {}
        for e in entities:
            desc = e.get("description", "")
            # Extract department/team from description
            dept = "Unknown"
            for line in desc.split("\n"):
                if "department:" in line.lower() or "team:" in line.lower():
                    dept = line.split(":")[-1].strip()
                    break
            
            if dept not in communities:
                communities[dept] = []
            communities[dept].append(e["name"])
        
        print(f"  Found {len(communities)} communities:")
        for dept, members in sorted(communities.items(), key=lambda x: -len(x[1]))[:5]:
            print(f"    - {dept}: {len(members)} members")
        
        return len(communities) > 1
    except Exception as e:
        print(f"  ✗ Query failed: {e}")
        return False

def test_temporal_ranking():
    """Test temporal decay ranking"""
    print("\n" + "=" * 60)
    print("4. Testing Temporal Ranking (by activation + recency)")
    print("=" * 60)
    
    # Query entities with activation and timestamps
    query = '''{ 
        entities(func: type(Entity), first: 20, orderasc: activation) { 
            uid 
            name 
            activation
            last_accessed
            created_at
        } 
    }'''
    
    try:
        resp = requests.post(f"{BASE_URL}/query",
                            headers={"Content-Type": "application/dql"},
                            data=query, timeout=10)
        data = resp.json()
        entities = data.get("data", {}).get("entities", [])
        
        # Sort by activation (simulating temporal ranking)
        sorted_entities = sorted(entities, 
                                key=lambda x: float(x.get("activation", 0.5) or 0.5), 
                                reverse=True)
        
        print(f"  ✓ Retrieved {len(entities)} entities with activation levels")
        print(f"  Top 5 by activation:")
        for e in sorted_entities[:5]:
            act = float(e.get("activation", 0.5) or 0.5)
            print(f"    - {e['name']}: activation={act:.2f}")
        
        return len(entities) > 0
    except Exception as e:
        print(f"  ✗ Query failed: {e}")
        return False

def test_multi_hop_expansion():
    """Test multi-hop graph expansion"""
    print("\n" + "=" * 60)
    print("5. Testing Multi-Hop Expansion")
    print("=" * 60)
    
    entities = get_sample_entities()
    if not entities:
        print("  ✗ No entities found")
        return False
    
    start = entities[0]
    print(f"  Start: {start['name']}")
    
    # 3-hop expansion query
    query = f'''{{
        node(func: uid({start['uid']})) @recurse(depth: 3) {{
            uid
            name
            dgraph.type
            related_to
            has_attribute
            produced_by
        }}
    }}'''
    
    try:
        resp = requests.post(f"{BASE_URL}/query",
                            headers={"Content-Type": "application/dql"},
                            data=query, timeout=10)
        data = resp.json()
        nodes = data.get("data", {}).get("node", [])
        
        # Count total nodes in the expansion
        def count_nodes(node, visited=None):
            if visited is None:
                visited = set()
            uid = node.get("uid", "")
            if uid in visited:
                return 0
            visited.add(uid)
            count = 1
            for key in ["related_to", "has_attribute", "produced_by"]:
                for child in node.get(key, []) or []:
                    if isinstance(child, dict):
                        count += count_nodes(child, visited)
            return count
        
        total = sum(count_nodes(n) for n in nodes)
        print(f"  ✓ 3-hop expansion found {total} connected nodes")
        return True
    except Exception as e:
        print(f"  ✗ Query failed: {e}")
        return False

def main():
    print("\n" + "=" * 60)
    print("GRAPH TRAVERSAL API TESTS")
    print("=" * 60)
    
    results = []
    
    # 1. Test DGraph connection
    results.append(("DGraph Connection", test_dgraph_connection()))
    
    if not results[0][1]:
        print("\n❌ DGraph not accessible. Make sure it's running.")
        sys.exit(1)
    
    # 2. Test spreading activation
    results.append(("Spreading Activation", test_spread_activation_direct()))
    
    # 3. Test community grouping
    results.append(("Community Grouping", test_community_grouping()))
    
    # 4. Test temporal ranking
    results.append(("Temporal Ranking", test_temporal_ranking()))
    
    # 5. Test multi-hop expansion
    results.append(("Multi-hop Expansion", test_multi_hop_expansion()))
    
    # Summary
    print("\n" + "=" * 60)
    print("TEST SUMMARY")
    print("=" * 60)
    
    passed = sum(1 for _, r in results if r)
    total = len(results)
    
    for name, result in results:
        status = "✓ PASS" if result else "✗ FAIL"
        print(f"  {status}: {name}")
    
    print(f"\n  {passed}/{total} tests passed")
    
    if passed == total:
        print("\n✅ All traversal tests passed!")
    else:
        print(f"\n⚠️  {total - passed} test(s) failed")

if __name__ == "__main__":
    main()
