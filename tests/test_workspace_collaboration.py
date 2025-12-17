#!/usr/bin/env python3
"""
Workspace Collaboration Integration Tests

Tests all collaboration features:
- User registration and authentication
- Workspace (group) creation
- Invitations (create, accept, decline)
- Share links (create, join, revoke)
- Member management (list, remove)
- Permission enforcement

Usage:
    python tests/test_workspace_collaboration.py

Prerequisites:
    - pip install httpx pytest
    - Server running at http://localhost:9090
"""

import httpx
import pytest
import uuid
import time
from typing import Optional

BASE_URL = "http://localhost:9090"


class TestUser:
    """Helper class for managing test users"""
    
    def __init__(self, username: str, password: str = "testpass123"):
        self.username = username
        self.password = password
        self.token: Optional[str] = None
    
    def register(self, client: httpx.Client) -> bool:
        """Register this user"""
        resp = client.post(f"{BASE_URL}/api/register", json={
            "username": self.username,
            "password": self.password
        })
        if resp.status_code in [200, 201]:
            self.token = resp.json().get("token")
            return True
        elif resp.status_code == 409:  # Already exists
            return self.login(client)
        return False
    
    def login(self, client: httpx.Client) -> bool:
        """Login this user"""
        resp = client.post(f"{BASE_URL}/api/login", json={
            "username": self.username,
            "password": self.password
        })
        if resp.status_code == 200:
            self.token = resp.json().get("token")
            return True
        return False
    
    def headers(self) -> dict:
        """Get auth headers"""
        return {"Authorization": f"Bearer {self.token}"} if self.token else {}


class TestWorkspaceCollaboration:
    """Integration tests for workspace collaboration"""
    
    @pytest.fixture(autouse=True)
    def setup(self):
        """Setup test users and client"""
        self.client = httpx.Client(timeout=30.0)
        
        # Create unique test users for each run
        suffix = str(uuid.uuid4())[:8]
        self.admin_user = TestUser(f"admin_{suffix}")
        self.member_user = TestUser(f"member_{suffix}")
        self.outsider_user = TestUser(f"outsider_{suffix}")
        
        # Register all users
        assert self.admin_user.register(self.client), "Failed to register admin"
        assert self.member_user.register(self.client), "Failed to register member"
        assert self.outsider_user.register(self.client), "Failed to register outsider"
        
        yield
        
        self.client.close()
    
    # =========================================================================
    # WORKSPACE CREATION TESTS
    # =========================================================================
    
    def test_create_workspace(self):
        """Test creating a new workspace"""
        resp = self.client.post(
            f"{BASE_URL}/api/groups",
            headers=self.admin_user.headers(),
            json={"name": f"Test Workspace {uuid.uuid4()}", "description": "Test"}
        )
        assert resp.status_code == 200, f"Failed: {resp.text}"
        data = resp.json()
        assert "namespace" in data
        assert data["namespace"].startswith("group_")
        return data["namespace"]
    
    def test_create_workspace_requires_auth(self):
        """Test that workspace creation requires authentication"""
        resp = self.client.post(
            f"{BASE_URL}/api/groups",
            json={"name": "Should Fail", "description": "No auth"}
        )
        # Should fail or be treated as anonymous
        assert resp.status_code in [401, 403] or "anonymous" in resp.text.lower()
    
    # =========================================================================
    # INVITATION TESTS
    # =========================================================================
    
    def test_invite_user_to_workspace(self):
        """Test inviting a user by username"""
        # Create workspace
        workspace = self._create_workspace()
        
        # Invite member user
        resp = self.client.post(
            f"{BASE_URL}/api/workspaces/{workspace}/invite",
            headers=self.admin_user.headers(),
            json={"username": self.member_user.username, "role": "subuser"}
        )
        assert resp.status_code == 201, f"Failed: {resp.text}"
        data = resp.json()
        assert data["status"] == "pending"
        assert "invitation_id" in data
        return data["invitation_id"], workspace
    
    def test_invite_nonexistent_user_fails(self):
        """Test that inviting a non-existent user fails"""
        workspace = self._create_workspace()
        
        resp = self.client.post(
            f"{BASE_URL}/api/workspaces/{workspace}/invite",
            headers=self.admin_user.headers(),
            json={"username": "nonexistent_user_12345", "role": "subuser"}
        )
        assert resp.status_code == 400
    
    def test_non_admin_cannot_invite(self):
        """Test that non-admins cannot invite users"""
        workspace = self._create_workspace()
        
        resp = self.client.post(
            f"{BASE_URL}/api/workspaces/{workspace}/invite",
            headers=self.outsider_user.headers(),
            json={"username": self.member_user.username, "role": "subuser"}
        )
        assert resp.status_code == 403
    
    def test_get_pending_invitations(self):
        """Test getting pending invitations for a user"""
        invite_id, workspace = self.test_invite_user_to_workspace()
        
        resp = self.client.get(
            f"{BASE_URL}/api/invitations",
            headers=self.member_user.headers()
        )
        assert resp.status_code == 200
        data = resp.json()
        assert "invitations" in data
        # Check our invitation is in the list
        found = any(i.get("uid") == invite_id or i.get("workspace_id") == workspace 
                   for i in data["invitations"])
        assert found, "Invitation not found in pending list"
    
    def test_accept_invitation(self):
        """Test accepting an invitation"""
        invite_id, workspace = self.test_invite_user_to_workspace()
        
        resp = self.client.post(
            f"{BASE_URL}/api/invitations/{invite_id}/accept",
            headers=self.member_user.headers()
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "accepted"
        
        # Verify user is now a member
        resp = self.client.get(
            f"{BASE_URL}/api/workspaces/{workspace}/members",
            headers=self.member_user.headers()
        )
        assert resp.status_code == 200
    
    def test_decline_invitation(self):
        """Test declining an invitation"""
        invite_id, workspace = self.test_invite_user_to_workspace()
        
        resp = self.client.post(
            f"{BASE_URL}/api/invitations/{invite_id}/decline",
            headers=self.member_user.headers()
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "declined"
    
    def test_cannot_accept_others_invitation(self):
        """Test that users can't accept invitations not meant for them"""
        invite_id, _ = self.test_invite_user_to_workspace()
        
        # Outsider tries to accept member's invitation
        resp = self.client.post(
            f"{BASE_URL}/api/invitations/{invite_id}/accept",
            headers=self.outsider_user.headers()
        )
        assert resp.status_code == 400
    
    # =========================================================================
    # SHARE LINK TESTS
    # =========================================================================
    
    def test_create_share_link(self):
        """Test creating a share link"""
        workspace = self._create_workspace()
        
        resp = self.client.post(
            f"{BASE_URL}/api/workspaces/{workspace}/share-link",
            headers=self.admin_user.headers(),
            json={"max_uses": 5, "expires_in_hours": 24}
        )
        assert resp.status_code == 201, f"Failed: {resp.text}"
        data = resp.json()
        assert "token" in data
        assert data["max_uses"] == 5
        return data["token"], workspace
    
    def test_join_via_share_link(self):
        """Test joining a workspace via share link"""
        token, workspace = self.test_create_share_link()
        
        resp = self.client.post(
            f"{BASE_URL}/api/join/{token}",
            headers=self.member_user.headers()
        )
        assert resp.status_code == 200, f"Failed: {resp.text}"
        data = resp.json()
        assert data["status"] == "joined"
        assert data["workspace_id"] == workspace
    
    def test_join_share_link_requires_auth(self):
        """Test that joining via share link requires authentication"""
        token, _ = self.test_create_share_link()
        
        # No auth header
        resp = self.client.post(f"{BASE_URL}/api/join/{token}")
        assert resp.status_code == 401 or "anonymous" in resp.text.lower()
    
    def test_revoke_share_link(self):
        """Test revoking a share link"""
        token, workspace = self.test_create_share_link()
        
        resp = self.client.delete(
            f"{BASE_URL}/api/workspaces/{workspace}/share-link/{token}",
            headers=self.admin_user.headers()
        )
        assert resp.status_code == 200
        
        # Try to use the revoked link
        resp = self.client.post(
            f"{BASE_URL}/api/join/{token}",
            headers=self.member_user.headers()
        )
        assert resp.status_code == 400  # Should fail
    
    def test_non_admin_cannot_create_share_link(self):
        """Test that non-admins cannot create share links"""
        workspace = self._create_workspace()
        
        resp = self.client.post(
            f"{BASE_URL}/api/workspaces/{workspace}/share-link",
            headers=self.outsider_user.headers(),
            json={}
        )
        assert resp.status_code == 403
    
    # =========================================================================
    # MEMBER MANAGEMENT TESTS
    # =========================================================================
    
    def test_list_workspace_members(self):
        """Test listing workspace members"""
        workspace = self._create_workspace()
        
        resp = self.client.get(
            f"{BASE_URL}/api/workspaces/{workspace}/members",
            headers=self.admin_user.headers()
        )
        assert resp.status_code == 200
        data = resp.json()
        assert "members" in data
        # Admin should be in the list
        assert any(m.get("role") == "admin" for m in data["members"])
    
    def test_non_member_cannot_list_members(self):
        """Test that non-members cannot list workspace members"""
        workspace = self._create_workspace()
        
        resp = self.client.get(
            f"{BASE_URL}/api/workspaces/{workspace}/members",
            headers=self.outsider_user.headers()
        )
        assert resp.status_code == 403
    
    def test_remove_member(self):
        """Test removing a member from workspace"""
        # Setup: Create workspace and add member
        workspace = self._create_workspace()
        self._add_member_via_invite(workspace)
        
        # Remove the member
        resp = self.client.delete(
            f"{BASE_URL}/api/workspaces/{workspace}/members/{self.member_user.username}",
            headers=self.admin_user.headers()
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "removed"
    
    def test_member_can_leave(self):
        """Test that a member can leave a workspace"""
        workspace = self._create_workspace()
        self._add_member_via_invite(workspace)
        
        # Member leaves
        resp = self.client.delete(
            f"{BASE_URL}/api/workspaces/{workspace}/members/{self.member_user.username}",
            headers=self.member_user.headers()
        )
        assert resp.status_code == 200
    
    def test_only_admin_leaves_blocked(self):
        """Test that the only admin cannot leave"""
        workspace = self._create_workspace()
        
        resp = self.client.delete(
            f"{BASE_URL}/api/workspaces/{workspace}/members/{self.admin_user.username}",
            headers=self.admin_user.headers()
        )
        assert resp.status_code == 400
        assert "only admin" in resp.text.lower()
    
    # =========================================================================
    # PERMISSION ENFORCEMENT TESTS
    # =========================================================================
    
    def test_non_member_cannot_chat_in_workspace(self):
        """Test that non-members cannot access workspace context"""
        workspace = self._create_workspace()
        
        # Outsider tries to chat in the workspace
        resp = self.client.post(
            f"{BASE_URL}/api/chat",
            headers=self.outsider_user.headers(),
            json={
                "message": "Hello workspace",
                "context_type": "group",
                "context_id": workspace
            }
        )
        # Should fail with permission error
        assert resp.status_code in [403, 500]
    
    def test_member_can_chat_in_workspace(self):
        """Test that members can access workspace context"""
        workspace = self._create_workspace()
        self._add_member_via_invite(workspace)
        
        resp = self.client.post(
            f"{BASE_URL}/api/chat",
            headers=self.member_user.headers(),
            json={
                "message": "Hello workspace",
                "context_type": "group",
                "context_id": workspace
            }
        )
        # Should succeed
        assert resp.status_code == 200
    
    # =========================================================================
    # HELPER METHODS
    # =========================================================================
    
    def _create_workspace(self) -> str:
        """Helper to create a workspace and return its namespace"""
        resp = self.client.post(
            f"{BASE_URL}/api/groups",
            headers=self.admin_user.headers(),
            json={"name": f"Test WS {uuid.uuid4()}", "description": "Test"}
        )
        assert resp.status_code == 200
        return resp.json()["namespace"]
    
    def _add_member_via_invite(self, workspace: str):
        """Helper to add member_user to a workspace via invitation"""
        # Create invitation
        resp = self.client.post(
            f"{BASE_URL}/api/workspaces/{workspace}/invite",
            headers=self.admin_user.headers(),
            json={"username": self.member_user.username, "role": "subuser"}
        )
        assert resp.status_code == 201
        invite_id = resp.json()["invitation_id"]
        
        # Accept invitation
        resp = self.client.post(
            f"{BASE_URL}/api/invitations/{invite_id}/accept",
            headers=self.member_user.headers()
        )
        assert resp.status_code == 200


# =========================================================================
# STANDALONE TEST RUNNER
# =========================================================================

def run_tests():
    """Run tests without pytest"""
    print("=" * 60)
    print("WORKSPACE COLLABORATION INTEGRATION TESTS")
    print("=" * 60)
    
    client = httpx.Client(timeout=30.0)
    
    # Check server is running
    try:
        resp = client.get(f"{BASE_URL}/health")
        if resp.status_code != 200:
            print("❌ Server not healthy")
            return False
        print("✅ Server is healthy\n")
    except Exception as e:
        print(f"❌ Cannot connect to server: {e}")
        print(f"   Make sure the server is running at {BASE_URL}")
        return False
    
    # Create test users
    suffix = str(uuid.uuid4())[:8]
    admin = TestUser(f"test_admin_{suffix}")
    member = TestUser(f"test_member_{suffix}")
    outsider = TestUser(f"test_outsider_{suffix}")
    
    print("Creating test users...")
    for user in [admin, member, outsider]:
        if user.register(client):
            print(f"  ✅ {user.username}")
        else:
            print(f"  ❌ {user.username}")
            return False
    
    results = []
    
    def run_test(name, test_func):
        try:
            test_func()
            print(f"✅ {name}")
            results.append((name, True, None))
        except AssertionError as e:
            print(f"❌ {name}: {e}")
            results.append((name, False, str(e)))
        except Exception as e:
            print(f"❌ {name}: {type(e).__name__}: {e}")
            results.append((name, False, str(e)))
    
    # Run tests
    print("\n" + "-" * 40)
    print("WORKSPACE CREATION")
    print("-" * 40)
    
    workspace_id = None
    def test_create():
        nonlocal workspace_id
        resp = client.post(
            f"{BASE_URL}/api/groups",
            headers=admin.headers(),
            json={"name": f"Test WS {uuid.uuid4()}", "description": "Integration Test"}
        )
        assert resp.status_code == 200, resp.text
        workspace_id = resp.json()["namespace"]
    
    run_test("Create workspace", test_create)
    
    if not workspace_id:
        print("Cannot continue without workspace")
        return False
    
    print(f"  → Workspace: {workspace_id}")
    
    print("\n" + "-" * 40)
    print("INVITATIONS")
    print("-" * 40)
    
    invite_id = None
    def test_invite():
        nonlocal invite_id
        resp = client.post(
            f"{BASE_URL}/api/workspaces/{workspace_id}/invite",
            headers=admin.headers(),
            json={"username": member.username, "role": "subuser"}
        )
        assert resp.status_code == 201, resp.text
        invite_id = resp.json()["invitation_id"]
    
    run_test("Invite user", test_invite)
    
    def test_pending():
        resp = client.get(f"{BASE_URL}/api/invitations", headers=member.headers())
        assert resp.status_code == 200, resp.text
    
    run_test("Get pending invitations", test_pending)
    
    def test_accept():
        if not invite_id:
            raise AssertionError("No invite to accept")
        resp = client.post(
            f"{BASE_URL}/api/invitations/{invite_id}/accept",
            headers=member.headers()
        )
        assert resp.status_code == 200, resp.text
    
    run_test("Accept invitation", test_accept)
    
    print("\n" + "-" * 40)
    print("SHARE LINKS")
    print("-" * 40)
    
    share_token = None
    def test_share_link():
        nonlocal share_token
        resp = client.post(
            f"{BASE_URL}/api/workspaces/{workspace_id}/share-link",
            headers=admin.headers(),
            json={"max_uses": 10}
        )
        assert resp.status_code == 201, resp.text
        share_token = resp.json()["token"]
    
    run_test("Create share link", test_share_link)
    
    def test_join_link():
        if not share_token:
            raise AssertionError("No share link")
        resp = client.post(
            f"{BASE_URL}/api/join/{share_token}",
            headers=outsider.headers()
        )
        assert resp.status_code == 200, resp.text
    
    run_test("Join via share link", test_join_link)
    
    print("\n" + "-" * 40)
    print("MEMBER MANAGEMENT")
    print("-" * 40)
    
    def test_list_members():
        resp = client.get(
            f"{BASE_URL}/api/workspaces/{workspace_id}/members",
            headers=admin.headers()
        )
        assert resp.status_code == 200, resp.text
        members = resp.json().get("members", [])
        assert len(members) >= 2, f"Expected at least 2 members, got {len(members)}"
    
    run_test("List members", test_list_members)
    
    def test_remove_member():
        resp = client.delete(
            f"{BASE_URL}/api/workspaces/{workspace_id}/members/{outsider.username}",
            headers=admin.headers()
        )
        assert resp.status_code == 200, resp.text
    
    run_test("Remove member", test_remove_member)
    
    # Summary
    print("\n" + "=" * 60)
    passed = sum(1 for _, p, _ in results if p)
    failed = sum(1 for _, p, _ in results if not p)
    print(f"RESULTS: {passed} passed, {failed} failed")
    print("=" * 60)
    
    client.close()
    return failed == 0


if __name__ == "__main__":
    # Try pytest first, fall back to standalone
    try:
        import pytest
        print("Running with pytest...")
        pytest.main([__file__, "-v"])
    except ImportError:
        print("pytest not installed, running standalone tests...\n")
        success = run_tests()
        exit(0 if success else 1)
