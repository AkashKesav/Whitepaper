#!/usr/bin/env python3
"""
Complete Manual Test Suite for Workspace Collaboration
Tests all endpoints step by step with detailed output
"""
import httpx
import uuid
import json

BASE_URL = 'http://localhost:9090'
client = httpx.Client(timeout=30.0)

def print_header(text):
    print("\n" + "=" * 60)
    print(f"  {text}")
    print("=" * 60)

def print_result(name, response, show_body=True):
    status = "✅" if response.status_code in [200, 201] else "❌"
    print(f"{status} {name}: {response.status_code}")
    if show_body and response.text:
        try:
            data = response.json()
            print(f"   Response: {json.dumps(data, indent=2)[:200]}")
        except:
            print(f"   Response: {response.text[:100]}")
    return response.status_code in [200, 201]

# ============================================================
print_header("WORKSPACE COLLABORATION - FULL MANUAL TEST")
# ============================================================

suffix = str(uuid.uuid4())[:8]
results = []

# ----------------------------------------------------------
print_header("Step 1: User Registration")
# ----------------------------------------------------------

# Register Admin
resp = client.post(f'{BASE_URL}/api/register', 
                   json={'username': f'admin_{suffix}', 'password': 'password123'})
results.append(("Register Admin", print_result("Register Admin", resp)))
admin_token = resp.json().get('token') if resp.status_code == 200 else None
admin_headers = {'Authorization': f'Bearer {admin_token}'} if admin_token else {}

# Register Member
resp = client.post(f'{BASE_URL}/api/register', 
                   json={'username': f'member_{suffix}', 'password': 'password123'})
results.append(("Register Member", print_result("Register Member", resp)))
member_token = resp.json().get('token') if resp.status_code == 200 else None
member_headers = {'Authorization': f'Bearer {member_token}'} if member_token else {}

# Register Outsider
resp = client.post(f'{BASE_URL}/api/register', 
                   json={'username': f'outsider_{suffix}', 'password': 'password123'})
results.append(("Register Outsider", print_result("Register Outsider", resp)))
outsider_token = resp.json().get('token') if resp.status_code == 200 else None
outsider_headers = {'Authorization': f'Bearer {outsider_token}'} if outsider_token else {}

# ----------------------------------------------------------
print_header("Step 2: Create Workspace")
# ----------------------------------------------------------

resp = client.post(f'{BASE_URL}/api/groups', headers=admin_headers,
                   json={'name': f'Test Workspace {suffix}', 'description': 'Manual Test'})
results.append(("Create Workspace", print_result("Create Workspace", resp)))
workspace_ns = resp.json().get('namespace') if resp.status_code == 200 else None
print(f"   Workspace Namespace: {workspace_ns}")

if not workspace_ns:
    print("\n❌ Cannot continue without workspace")
    exit(1)

# ----------------------------------------------------------
print_header("Step 3: Invitation Flow")
# ----------------------------------------------------------

# Create invitation
resp = client.post(f'{BASE_URL}/api/workspaces/{workspace_ns}/invite', headers=admin_headers,
                   json={'username': f'member_{suffix}', 'role': 'subuser'})
results.append(("Create Invitation", print_result("Create Invitation", resp)))
invite_id = resp.json().get('invitation_id') if resp.status_code == 201 else None
print(f"   Invitation ID: {invite_id}")

# Get pending invitations (as member)
resp = client.get(f'{BASE_URL}/api/invitations', headers=member_headers)
results.append(("Get Pending Invitations", print_result("Get Pending Invitations", resp)))

# Accept invitation
if invite_id:
    resp = client.post(f'{BASE_URL}/api/invitations/{invite_id}/accept', headers=member_headers)
    results.append(("Accept Invitation", print_result("Accept Invitation", resp)))

# ----------------------------------------------------------
print_header("Step 4: Share Link Flow")
# ----------------------------------------------------------

# Create share link
resp = client.post(f'{BASE_URL}/api/workspaces/{workspace_ns}/share-link', headers=admin_headers,
                   json={'max_uses': 5, 'expires_in_hours': 24})
results.append(("Create Share Link", print_result("Create Share Link", resp)))
share_token = resp.json().get('token') if resp.status_code == 201 else None
print(f"   Share Token: {share_token[:30] if share_token else 'N/A'}...")

# Join via share link (outsider)
if share_token:
    resp = client.post(f'{BASE_URL}/api/join/{share_token}', headers=outsider_headers)
    results.append(("Join via Share Link", print_result("Join via Share Link", resp)))

# ----------------------------------------------------------
print_header("Step 5: Member Management")
# ----------------------------------------------------------

# List members
resp = client.get(f'{BASE_URL}/api/workspaces/{workspace_ns}/members', headers=admin_headers)
results.append(("List Members", print_result("List Members", resp)))
if resp.status_code == 200:
    members = resp.json().get('members', [])
    print(f"   Total Members: {len(members)}")
    for m in members:
        print(f"     - {m.get('username', 'N/A')} ({m.get('role', 'N/A')})")

# Remove outsider
resp = client.delete(f'{BASE_URL}/api/workspaces/{workspace_ns}/members/outsider_{suffix}', 
                     headers=admin_headers)
results.append(("Remove Member", print_result("Remove Member", resp)))

# ----------------------------------------------------------
print_header("Step 6: Permission Tests")
# ----------------------------------------------------------

# Non-admin cannot invite
resp = client.post(f'{BASE_URL}/api/workspaces/{workspace_ns}/invite', headers=member_headers,
                   json={'username': 'random_user', 'role': 'subuser'})
expected_fail = resp.status_code == 403
results.append(("Non-admin Cannot Invite", expected_fail))
print(f"{'✅' if expected_fail else '❌'} Non-admin Cannot Invite: {resp.status_code} (expected 403)")

# Non-admin cannot create share link
resp = client.post(f'{BASE_URL}/api/workspaces/{workspace_ns}/share-link', headers=member_headers,
                   json={})
expected_fail = resp.status_code == 403
results.append(("Non-admin Cannot Create Link", expected_fail))
print(f"{'✅' if expected_fail else '❌'} Non-admin Cannot Create Link: {resp.status_code} (expected 403)")

# Revoke share link
if share_token:
    resp = client.delete(f'{BASE_URL}/api/workspaces/{workspace_ns}/share-link/{share_token}', 
                         headers=admin_headers)
    results.append(("Revoke Share Link", print_result("Revoke Share Link", resp)))

# ============================================================
print_header("TEST SUMMARY")
# ============================================================

passed = sum(1 for _, r in results if r)
failed = sum(1 for _, r in results if not r)
print(f"\n✅ Passed: {passed}")
print(f"❌ Failed: {failed}")
print(f"📊 Total:  {len(results)}")

if failed == 0:
    print("\n🎉 ALL TESTS PASSED!")
else:
    print("\n⚠️  Some tests failed - check output above")

client.close()
