#!/usr/bin/env python3
"""Simple manual test of workspace collaboration APIs"""
import httpx
import uuid

client = httpx.Client(timeout=30.0)
BASE_URL = 'http://localhost:9090'

print("=" * 50)
print("WORKSPACE COLLABORATION MANUAL TEST")
print("=" * 50)

# 1. Register admin
suffix = str(uuid.uuid4())[:8]
admin_name = f'admin_{suffix}'
resp = client.post(f'{BASE_URL}/api/register', json={'username': admin_name, 'password': 'test123'})
print(f'1. Register Admin: {resp.status_code}')
admin_token = resp.json()['token']
admin_headers = {'Authorization': f'Bearer {admin_token}'}

# 2. Register member
member_name = f'member_{suffix}'
resp = client.post(f'{BASE_URL}/api/register', json={'username': member_name, 'password': 'test123'})
print(f'2. Register Member: {resp.status_code}')
member_token = resp.json()['token']
member_headers = {'Authorization': f'Bearer {member_token}'}

# 3. Create workspace
resp = client.post(f'{BASE_URL}/api/groups', headers=admin_headers, json={'name': 'Test WS', 'description': 'Test'})
print(f'3. Create Workspace: {resp.status_code}')
ws = resp.json()['namespace']
print(f'   Namespace: {ws}')

# 4. Invite member
resp = client.post(f'{BASE_URL}/api/workspaces/{ws}/invite', headers=admin_headers, 
                   json={'username': member_name, 'role': 'subuser'})
print(f'4. Invite Member: {resp.status_code}')
if resp.status_code == 201:
    invite_id = resp.json()['invitation_id']
    print(f'   Invitation ID: {invite_id}')
    
    # 5. Accept invitation
    resp = client.post(f'{BASE_URL}/api/invitations/{invite_id}/accept', headers=member_headers)
    print(f'5. Accept Invitation: {resp.status_code}')
else:
    print(f'   Error: {resp.text}')

# 6. List members
resp = client.get(f'{BASE_URL}/api/workspaces/{ws}/members', headers=admin_headers)
print(f'6. List Members: {resp.status_code}')
if resp.status_code == 200:
    members = resp.json().get('members', [])
    print(f'   Count: {len(members)}')

# 7. Create share link
resp = client.post(f'{BASE_URL}/api/workspaces/{ws}/share-link', headers=admin_headers, json={'max_uses': 5})
print(f'7. Create Share Link: {resp.status_code}')
if resp.status_code == 201:
    token = resp.json()['token']
    print(f'   Token: {token[:20]}...')

print("\n" + "=" * 50)
print("TEST COMPLETE")
print("=" * 50)
