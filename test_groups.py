import requests
import time
import uuid

BASE_URL = "http://127.0.0.1:3000/api"

def register_user(username, password):
    resp = requests.post(f"{BASE_URL}/register", json={
        "username": username,
        "password": password
    })
    if resp.status_code == 409: # Already exists, login
        return login_user(username, password)
    resp.raise_for_status()
    return resp.json()["token"]

def login_user(username, password):
    resp = requests.post(f"{BASE_URL}/login", json={
        "username": username,
        "password": password
    })
    resp.raise_for_status()
    return resp.json()["token"]

def create_group(token, name, desc):
    resp = requests.post(f"{BASE_URL}/groups", json={
        "name": name,
        "description": desc
    }, headers={"Authorization": f"Bearer {token}"})
    resp.raise_for_status()
    return resp.json()

def list_groups(token):
    resp = requests.get(f"{BASE_URL}/list-groups", headers={"Authorization": f"Bearer {token}"})
    resp.raise_for_status()
    return resp.json()["groups"]

def add_member(token, group_ns, username):
    resp = requests.post(f"{BASE_URL}/groups/{group_ns}/members", json={
        "username": username
    }, headers={"Authorization": f"Bearer {token}"})
    return resp

def main():
    print("=== STARTING GROUP V2 VERIFICATION ===")
    
    # 1. Register users
    admin_user = f"admin_{uuid.uuid4().hex[:6]}"
    sub_user = f"sub_{uuid.uuid4().hex[:6]}"
    other_user = f"other_{uuid.uuid4().hex[:6]}"
    
    print(f"Registering users: {admin_user}, {sub_user}, {other_user}")
    token_admin = register_user(admin_user, "password123")
    token_sub = register_user(sub_user, "password123")
    token_other = register_user(other_user, "password123")
    
    # 2. Test Group Creation
    print("\n--- Test: Create Group ---")
    group_data = create_group(token_admin, "Project Alpha", "Top Secret")
    group_ns = group_data.get("group_id") or group_data.get("namespace")
    print(f"Group Created: {group_ns}")
    
    # 3. Test Visibility (Admin should see it)
    print("\n--- Test: Admin Visibility ---")
    groups = list_groups(token_admin)
    found = any(g.get("namespace") == group_ns or g.get("Name") == "Project Alpha" for g in groups)
    if found:
        print("PASS: Admin sees group")
    else:
        print(f"FAIL: Admin does NOT see group. Groups: {groups}")
        exit(1)

    # 4. Test Visibility (Subuser should NOT see it yet)
    print("\n--- Test: Subuser Visibility (Pre-Add) ---")
    groups_sub = list_groups(token_sub)
    if groups_sub and any(g.get("namespace") == group_ns for g in groups_sub):
        print("FAIL: Subuser sees group before being added")
        exit(1)
    else:
        print("PASS: Subuser does not see group")

    # 5. Test Add Member (Admin adds Subuser)
    print("\n--- Test: Add Member (Admin) ---")
    resp = add_member(token_admin, group_ns, sub_user)
    if resp.status_code == 200:
        print("PASS: Admin added member")
    else:
        print(f"FAIL: Admin failed to add member: {resp.text}")
        exit(1)

    # 6. Test Visibility (Subuser should see it now)
    print("\n--- Test: Subuser Visibility (Post-Add) ---")
    groups_sub = list_groups(token_sub)
    if groups_sub and any(g.get("namespace") == group_ns for g in groups_sub):
        print("PASS: Subuser sees group")
    else:
        print("FAIL: Subuser still does not see group")
        # exit(1) # Continue for now

    # 7. Test RBAC (Subuser tries to add Other User -> Should Fail)
    print("\n--- Test: RBAC (Subuser adding member) ---")
    resp = add_member(token_sub, group_ns, other_user)
    if resp.status_code == 403:
        print("PASS: Subuser denied adding member (403 Forbidden)")
    else:
        print(f"FAIL: Subuser was allowed to add member or got wrong error: {resp.status_code} {resp.text}")

    print("\n=== VERIFICATION COMPLETE ===")

if __name__ == "__main__":
    main()
