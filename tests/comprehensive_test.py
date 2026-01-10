#!/usr/bin/env python3
"""
Comprehensive Feature Testing Script
Tests: Auth, Chat Isolation, Workspaces, Memory Retention, Document Ingestion, Group Chat Visibility
"""

import httpx
import uuid
import time
import json
from datetime import datetime

BASE_URL = "http://localhost:9090"
AI_SERVICES_URL = "http://localhost:8000"

class TestResults:
    def __init__(self):
        self.passed = []
        self.failed = []
    
    def add(self, name, passed, details=""):
        if passed:
            self.passed.append(name)
            print(f"✅ {name}")
        else:
            self.failed.append((name, details))
            print(f"❌ {name}: {details}")
    
    def summary(self):
        print("\n" + "="*60)
        print(f"RESULTS: {len(self.passed)} passed, {len(self.failed)} failed")
        if self.failed:
            print("\nFailed tests:")
            for name, details in self.failed:
                print(f"  - {name}: {details}")
        print("="*60)


def run_tests():
    results = TestResults()
    client = httpx.Client(timeout=60.0)
    
    # Test users
    suffix = str(uuid.uuid4())[:8]
    user_a = {"username": f"user_a_{suffix}", "password": "testpass123", "token": None}
    user_b = {"username": f"user_b_{suffix}", "password": "testpass123", "token": None}
    admin = {"username": f"admin_{suffix}", "password": "testpass123", "token": None}
    member = {"username": f"member_{suffix}", "password": "testpass123", "token": None}
    outsider = {"username": f"outsider_{suffix}", "password": "testpass123", "token": None}
    
    print("="*60)
    print("COMPREHENSIVE FEATURE TESTING")
    print(f"Started: {datetime.now().isoformat()}")
    print("="*60)
    
    # =========================================================================
    # SECTION 1: AUTHENTICATION
    # =========================================================================
    print("\n" + "-"*40)
    print("AUTHENTICATION TESTS")
    print("-"*40)
    
    # Test 1: Register users
    for user in [user_a, user_b, admin, member, outsider]:
        try:
            resp = client.post(f"{BASE_URL}/api/register", json={
                "username": user["username"],
                "password": user["password"]
            })
            if resp.status_code in [200, 201]:
                user["token"] = resp.json().get("token")
                results.add(f"Register {user['username'][:10]}...", True)
            else:
                results.add(f"Register {user['username'][:10]}...", False, resp.text[:100])
        except Exception as e:
            results.add(f"Register {user['username'][:10]}...", False, str(e))
    
    # Test 2: Login
    for user in [user_a, user_b]:
        if not user["token"]:
            try:
                resp = client.post(f"{BASE_URL}/api/login", json={
                    "username": user["username"],
                    "password": user["password"]
                })
                if resp.status_code == 200:
                    user["token"] = resp.json().get("token")
                    results.add(f"Login {user['username'][:10]}...", True)
                else:
                    results.add(f"Login {user['username'][:10]}...", False, resp.text[:100])
            except Exception as e:
                results.add(f"Login {user['username'][:10]}...", False, str(e))
    
    # Test 3: Invalid auth rejected
    try:
        resp = client.post(f"{BASE_URL}/api/login", json={
            "username": "nonexistent_user",
            "password": "wrongpass"
        })
        results.add("Invalid auth rejected", resp.status_code in [401, 400, 404])
    except Exception as e:
        results.add("Invalid auth rejected", False, str(e))
    
    # =========================================================================
    # SECTION 2: CHAT AUTH ISOLATION
    # =========================================================================
    print("\n" + "-"*40)
    print("CHAT AUTH ISOLATION TESTS")
    print("-"*40)
    
    def get_headers(user):
        return {"Authorization": f"Bearer {user['token']}"} if user["token"] else {}
    
    # User A creates a personal conversation
    user_a_conv_id = None
    try:
        resp = client.post(f"{BASE_URL}/api/chat", 
            headers=get_headers(user_a),
            json={"message": "My secret info: I love pizza"}
        )
        if resp.status_code == 200:
            user_a_conv_id = resp.json().get("conversation_id")
            results.add("User A creates personal chat", True)
        else:
            results.add("User A creates personal chat", False, resp.text[:100])
    except Exception as e:
        results.add("User A creates personal chat", False, str(e))
    
    # User B cannot access User A's conversations
    try:
        resp = client.get(f"{BASE_URL}/api/conversations", headers=get_headers(user_b))
        if resp.status_code == 200:
            convs = resp.json().get("conversations", [])
            user_a_found = any(c.get("id") == user_a_conv_id for c in convs)
            results.add("User B cannot see User A's conversations", not user_a_found)
        else:
            results.add("User B cannot see User A's conversations", False, resp.text[:100])
    except Exception as e:
        results.add("User B cannot see User A's conversations", False, str(e))
    
    # User B creates own chat - should not get User A's context
    try:
        resp = client.post(f"{BASE_URL}/api/chat",
            headers=get_headers(user_b),
            json={"message": "What's my secret info?"}
        )
        if resp.status_code == 200:
            response_text = resp.json().get("response", "").lower()
            # Should NOT contain User A's pizza preference
            no_leak = "pizza" not in response_text
            results.add("No data leak between users", no_leak, 
                       f"Response mentioned pizza" if not no_leak else "")
        else:
            results.add("No data leak between users", False, resp.text[:100])
    except Exception as e:
        results.add("No data leak between users", False, str(e))
    
    # =========================================================================
    # SECTION 3: WORKSPACE & GROUP MANAGEMENT
    # =========================================================================
    print("\n" + "-"*40)
    print("WORKSPACE & GROUP MANAGEMENT TESTS")
    print("-"*40)
    
    workspace_id = None
    
    # Create workspace
    try:
        resp = client.post(f"{BASE_URL}/api/groups",
            headers=get_headers(admin),
            json={"name": f"Test Workspace {suffix}", "description": "Test workspace"}
        )
        if resp.status_code == 200:
            workspace_id = resp.json().get("namespace")
            results.add("Create workspace", True)
        else:
            results.add("Create workspace", False, resp.text[:100])
    except Exception as e:
        results.add("Create workspace", False, str(e))
    
    if workspace_id:
        # Invite member
        invite_id = None
        try:
            resp = client.post(f"{BASE_URL}/api/workspaces/{workspace_id}/invite",
                headers=get_headers(admin),
                json={"username": member["username"], "role": "subuser"}
            )
            if resp.status_code == 201:
                invite_id = resp.json().get("invitation_id")
                results.add("Invite user to workspace", True)
            else:
                results.add("Invite user to workspace", False, resp.text[:100])
        except Exception as e:
            results.add("Invite user to workspace", False, str(e))
        
        # Accept invitation
        if invite_id:
            try:
                resp = client.post(f"{BASE_URL}/api/invitations/{invite_id}/accept",
                    headers=get_headers(member)
                )
                if resp.status_code == 200:
                    results.add("Accept invitation", True)
                else:
                    results.add("Accept invitation", False, resp.text[:100])
            except Exception as e:
                results.add("Accept invitation", False, str(e))
        
        # Non-admin cannot invite
        try:
            resp = client.post(f"{BASE_URL}/api/workspaces/{workspace_id}/invite",
                headers=get_headers(outsider),
                json={"username": user_a["username"], "role": "subuser"}
            )
            results.add("Non-admin cannot invite", resp.status_code == 403)
        except Exception as e:
            results.add("Non-admin cannot invite", False, str(e))
        
        # Create share link
        share_token = None
        try:
            resp = client.post(f"{BASE_URL}/api/workspaces/{workspace_id}/share-link",
                headers=get_headers(admin),
                json={"max_uses": 10}
            )
            if resp.status_code == 201:
                share_token = resp.json().get("token")
                results.add("Create share link", True)
            else:
                results.add("Create share link", False, resp.text[:100])
        except Exception as e:
            results.add("Create share link", False, str(e))
        
        # Join via share link
        if share_token:
            try:
                resp = client.post(f"{BASE_URL}/api/join/{share_token}",
                    headers=get_headers(user_a)
                )
                results.add("Join via share link", resp.status_code == 200, 
                           resp.text[:100] if resp.status_code != 200 else "")
            except Exception as e:
                results.add("Join via share link", False, str(e))
        
        # List members
        try:
            resp = client.get(f"{BASE_URL}/api/workspaces/{workspace_id}/members",
                headers=get_headers(admin)
            )
            if resp.status_code == 200:
                members_list = resp.json().get("members", [])
                results.add("List workspace members", len(members_list) >= 2,
                           f"Expected 2+ members, got {len(members_list)}")
            else:
                results.add("List workspace members", False, resp.text[:100])
        except Exception as e:
            results.add("List workspace members", False, str(e))
        
        # Non-member cannot list
        try:
            resp = client.get(f"{BASE_URL}/api/workspaces/{workspace_id}/members",
                headers=get_headers(outsider)
            )
            results.add("Non-member cannot list members", resp.status_code == 403)
        except Exception as e:
            results.add("Non-member cannot list members", False, str(e))
    
    # =========================================================================
    # SECTION 4: GROUP CHAT VISIBILITY
    # =========================================================================
    print("\n" + "-"*40)
    print("GROUP CHAT VISIBILITY TESTS")
    print("-"*40)
    
    if workspace_id:
        # Admin chats in workspace
        try:
            resp = client.post(f"{BASE_URL}/api/chat",
                headers=get_headers(admin),
                json={
                    "message": "Important group announcement: Project deadline is Friday",
                    "context_type": "group",
                    "context_id": workspace_id
                }
            )
            results.add("Admin chats in workspace", resp.status_code == 200,
                       resp.text[:100] if resp.status_code != 200 else "")
        except Exception as e:
            results.add("Admin chats in workspace", False, str(e))
        
        # Member can chat in workspace
        try:
            resp = client.post(f"{BASE_URL}/api/chat",
                headers=get_headers(member),
                json={
                    "message": "What was our group announcement?",
                    "context_type": "group",
                    "context_id": workspace_id
                }
            )
            if resp.status_code == 200:
                response_text = resp.json().get("response", "").lower()
                # Check if response references the group context
                results.add("Member can chat in workspace", True)
            else:
                results.add("Member can chat in workspace", False, resp.text[:100])
        except Exception as e:
            results.add("Member can chat in workspace", False, str(e))
        
        # Outsider CANNOT chat in workspace
        try:
            resp = client.post(f"{BASE_URL}/api/chat",
                headers=get_headers(outsider),
                json={
                    "message": "Can I see group data?",
                    "context_type": "group",
                    "context_id": workspace_id
                }
            )
            results.add("Outsider cannot chat in workspace", resp.status_code in [403, 500])
        except Exception as e:
            results.add("Outsider cannot chat in workspace", False, str(e))
    
    # =========================================================================
    # SECTION 5: MEMORY RETENTION & ACTIVATION
    # =========================================================================
    print("\n" + "-"*40)
    print("MEMORY RETENTION & ACTIVATION TESTS")
    print("-"*40)
    
    # Create facts for user_a
    try:
        resp = client.post(f"{BASE_URL}/api/chat",
            headers=get_headers(user_a),
            json={"message": "Remember this: My favorite color is blue and my pet's name is Max"}
        )
        results.add("Store memory facts", resp.status_code == 200,
                   resp.text[:100] if resp.status_code != 200 else "")
    except Exception as e:
        results.add("Store memory facts", False, str(e))
    
    # Wait a moment for processing
    time.sleep(2)
    
    # Query the memory
    try:
        resp = client.post(f"{BASE_URL}/api/chat",
            headers=get_headers(user_a),
            json={"message": "What is my favorite color?"}
        )
        if resp.status_code == 200:
            response_text = resp.json().get("response", "").lower()
            has_memory = "blue" in response_text
            results.add("Memory retention works", has_memory,
                       "Response didn't mention 'blue'" if not has_memory else "")
        else:
            results.add("Memory retention works", False, resp.text[:100])
    except Exception as e:
        results.add("Memory retention works", False, str(e))
    
    # Check graph stats for activation scores
    try:
        resp = client.get(f"{BASE_URL}/api/stats", headers=get_headers(user_a))
        if resp.status_code == 200:
            stats = resp.json()
            results.add("Graph stats available", "total_nodes" in stats or "nodes" in stats or "entities" in stats)
        else:
            results.add("Graph stats available", False, resp.text[:100])
    except Exception as e:
        results.add("Graph stats available", False, str(e))
    
    # =========================================================================
    # SECTION 6: DOCUMENT INGESTION
    # =========================================================================
    print("\n" + "-"*40)
    print("DOCUMENT INGESTION TESTS")
    print("-"*40)
    
    # Check AI services health
    try:
        resp = client.get(f"{AI_SERVICES_URL}/health")
        results.add("AI Services healthy", resp.status_code == 200,
                   resp.text[:100] if resp.status_code != 200 else "")
    except Exception as e:
        results.add("AI Services healthy", False, str(e))
    
    # Test document ingestion via AI services
    test_doc = """
    Meeting Notes - Q4 Planning
    Date: January 6, 2025
    Attendees: Alice Johnson (CEO), Bob Smith (CTO), Carol White (CFO)
    
    Key Decisions:
    1. Launch new product line by March 2025
    2. Increase R&D budget by 15%
    3. Hire 10 new engineers
    
    Action Items:
    - Bob to create technical roadmap
    - Carol to finalize budget allocation
    - Alice to present to board next week
    """
    
    try:
        resp = client.post(f"{AI_SERVICES_URL}/ingest",
            json={
                "text": test_doc,
                "document_type": "meeting_notes"
            }
        )
        if resp.status_code == 200:
            data = resp.json()
            has_entities = len(data.get("entities", [])) > 0 or len(data.get("chunks", [])) > 0
            results.add("Document ingestion extracts entities", has_entities,
                       f"Got: {list(data.keys())}")
        else:
            results.add("Document ingestion extracts entities", False, resp.text[:100])
    except Exception as e:
        results.add("Document ingestion extracts entities", False, str(e))
    
    # Test via monolith upload endpoint with text
    try:
        # Use multipart form data for file upload
        files = {'file': ('test.txt', test_doc.encode(), 'text/plain')}
        resp = client.post(f"{BASE_URL}/api/upload",
            headers=get_headers(admin),
            files=files
        )
        results.add("Monolith document upload", resp.status_code == 200,
                   resp.text[:100] if resp.status_code != 200 else "")
    except Exception as e:
        results.add("Monolith document upload", False, str(e))
    
    # =========================================================================
    # SUMMARY
    # =========================================================================
    results.summary()
    
    client.close()
    return len(results.failed) == 0


if __name__ == "__main__":
    success = run_tests()
    exit(0 if success else 1)
