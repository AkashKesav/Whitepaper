import requests
import time
import uuid
import json

BASE_URL = "http://localhost:9090"

def test_admin_api():
    print("=== Testing Admin API & RBAC ===")
    
    # 1. Register Super Admin
    admin_username = f"super_admin_test_{uuid.uuid4().hex[:8]}"
    admin_password = "password123"
    print(f"\n1. Registering Admin: {admin_username}")
    
    resp = requests.post(f"{BASE_URL}/api/register", json={
        "username": admin_username,
        "password": admin_password
    })
    if resp.status_code not in [200, 201]:
        print(f"FAILED: Admin registration. Status: {resp.status_code}, Body: {resp.text}")
        return False
    
    admin_data = resp.json()
    admin_token = admin_data.get("token")
    admin_role = admin_data.get("role")
    
    if admin_role != "admin":
        print(f"FAILED: Expected role 'admin', got '{admin_role}'")
        return False
    print("SUCCESS: Admin registered with role 'admin'")
    
    # 2. Register Regular User
    user_username = f"test_user_{uuid.uuid4().hex[:8]}"
    user_password = "password123"
    print(f"\n2. Registering User: {user_username}")
    
    resp = requests.post(f"{BASE_URL}/api/register", json={
        "username": user_username,
        "password": user_password
    })
    if resp.status_code not in [200, 201]:
        print(f"FAILED: User registration. Status: {resp.status_code}")
        return False
        
    user_data = resp.json()
    user_token = user_data.get("token")
    user_role = user_data.get("role")
    
    if user_role != "user":
        print(f"FAILED: Expected role 'user', got '{user_role}'")
        return False
    print("SUCCESS: User registered with role 'user'")
    
    # 3. Access Admin Protected Endpoint (System Stats)
    print("\n3. Testing Access Control (System Stats)")
    
    # Admin Access
    headers_admin = {"Authorization": f"Bearer {admin_token}"}
    resp = requests.get(f"{BASE_URL}/api/admin/system/stats", headers=headers_admin)
    if resp.status_code == 200:
        stats = resp.json()
        print(f"SUCCESS: Admin accessed stats. Users: {stats.get('total_users')}")
    else:
        print(f"FAILED: Admin denied access. Status: {resp.status_code}")
        return False
        
    # User Access (Should Fail)
    headers_user = {"Authorization": f"Bearer {user_token}"}
    resp = requests.get(f"{BASE_URL}/api/admin/system/stats", headers=headers_user)
    if resp.status_code == 403:
        print("SUCCESS: User correctly denied access to admin stats (403)")
    else:
        print(f"FAILED: User accessed admin stats! Status: {resp.status_code}")
        return False
        
    # 4. Admin Promotes User
    print(f"\n4. Promoting User {user_username} to Admin")
    resp = requests.put(
        f"{BASE_URL}/api/admin/users/{user_username}/role", 
        headers=headers_admin,
        json={"role": "admin"}
    )
    if resp.status_code == 200:
        print("SUCCESS: Role update request succeeded")
    else:
        print(f"FAILED: Role update failed. Status: {resp.status_code}")
        return False
        
    # 5. Verify User is now Admin (login again)
    print("\n5. Verifying User Promotion")
    resp = requests.post(f"{BASE_URL}/api/login", json={
        "username": user_username,
        "password": user_password
    })
    new_user_role = resp.json().get("role")
    if new_user_role == "admin":
        print(f"SUCCESS: User is now '{new_user_role}'")
    else:
        print(f"FAILED: User role is still '{new_user_role}'")
        return False
        
    # 6. New Admin Accesses Stats
    print("\n6. Testing New Admin Access")
    new_admin_token = resp.json().get("token")
    resp = requests.get(f"{BASE_URL}/api/admin/system/stats", headers={"Authorization": f"Bearer {new_admin_token}"})
    if resp.status_code == 200:
        print("SUCCESS: New admin accessed stats")
    else:
        print(f"FAILED: New admin denied access. Status: {resp.status_code}")
        return False

    print("\n=== ALL TESTS PASSED ===")

    # Phase 2 Tests
    print("\n--- Testing Phase 2: Finance, Support, Affiliates ---")
    
    # 7. Test Finance API
    print("\n[7] Testing Finance API...")
    resp = requests.get(f"{BASE_URL}/api/admin/finance/revenue", headers=headers_new_admin)
    if resp.status_code == 200:
        print("✅ Finance Revenue: OK")
        print(f"   Revenue Data: {resp.json().get('total_revenue')}")
    else:
        print(f"❌ Finance Revenue Failed: {resp.status_code} - {resp.text}")
        return False

    # 8. Test Support API
    print("\n[8] Testing Support API...")
    resp = requests.get(f"{BASE_URL}/api/admin/support/tickets", headers=headers_new_admin)
    if resp.status_code == 200:
        tickets = resp.json().get('tickets', [])
        print(f"✅ Support Tickets: OK (Count: {len(tickets)})")
    else:
        print(f"❌ Support Tickets Failed: {resp.status_code} - {resp.text}")
        return False

    # 9. Test Affiliate API
    print("\n[9] Testing Affiliate API...")
    resp = requests.get(f"{BASE_URL}/api/admin/affiliates", headers=headers_new_admin)
    if resp.status_code == 200:
        affiliates = resp.json().get('affiliates', [])
        print(f"✅ Affiliates List: OK (Count: {len(affiliates)})")
    else:
        print(f"❌ Affiliates List Failed: {resp.status_code} - {resp.text}")
        return False

    # 10. Test Operations API (Campaigns)
    print("\n[10] Testing Operations API (Campaigns)...")
    resp = requests.get(f"{BASE_URL}/api/admin/operations/campaigns", headers=headers_new_admin)
    if resp.status_code == 200:
        campaigns = resp.json().get('campaigns', [])
        print(f"✅ Campaigns List: OK (Count: {len(campaigns)})")
    else:
        print(f"❌ Campaigns List Failed: {resp.status_code} - {resp.text}")
        return False

    # 11. Test System API (Feature Flags)
    print("\n[11] Testing System API (Flags)...")
    resp = requests.get(f"{BASE_URL}/api/admin/system/flags", headers=headers_new_admin)
    if resp.status_code == 200:
        flags = resp.json().get('flags', [])
        print(f"✅ Feature Flags: OK (Count: {len(flags)})")
    else:
        print(f"❌ Feature Flags Failed: {resp.status_code} - {resp.text}")
        return False

    # 12. Test Emergency API
    print("\n[12] Testing Emergency API...")
    resp = requests.get(f"{BASE_URL}/api/admin/emergency/requests", headers=headers_new_admin)
    if resp.status_code == 200:
        requests_list = resp.json().get('requests', [])
        print(f"✅ Emergency Requests: OK (Count: {len(requests_list)})")
        
        # Test Approve
        if len(requests_list) > 0:
            req_id = requests_list[0]['id']
            print(f"    Approving Request {req_id}...")
            resp_approve = requests.post(f"{BASE_URL}/api/admin/emergency/requests/{req_id}/approve", headers=headers_new_admin)
            if resp_approve.status_code == 200:
                 print("    ✅ Approval: OK")
            else:
                 print(f"    ❌ Approval Failed: {resp_approve.status_code}")
    else:
        print(f"❌ Emergency Requests Failed: {resp.status_code} - {resp.text}")
        return False

    # 13. Test Policy API (Phase 5)
    print("\n[13] Testing Policy API...")
    
    # 13a. Create Policy
    policy_payload = {
        "id": "test_policy_1",
        "description": "Test Policy",
        "subjects": ["user:test_user"],
        "resources": ["node:123"],
        "actions": ["READ"],
        "effect": "ALLOW"
    }
    resp = requests.post(f"{BASE_URL}/api/admin/policies", json=policy_payload, headers=headers_new_admin)
    if resp.status_code == 201:
        print("    ✅ Create Policy: OK")
    else:
        print(f"    ❌ Create Policy Failed: {resp.status_code} - {resp.text}")

    # 13b. List Policies
    resp = requests.get(f"{BASE_URL}/api/admin/policies", headers=headers_new_admin)
    if resp.status_code == 200:
        policies = resp.json().get('policies', [])
        print(f"    ✅ List Policies: OK (Count: {len(policies)})")
    else:
        print(f"    ❌ List Policies Failed: {resp.status_code} - {resp.text}")

    # 13c. Get Audit Logs
    resp = requests.get(f"{BASE_URL}/api/admin/audit?limit=5", headers=headers_new_admin)
    if resp.status_code == 200:
        logs = resp.json().get('logs', [])
        print(f"    ✅ Audit Logs: OK (Count: {len(logs)})")
    else:
        print(f"    ❌ Audit Logs Failed: {resp.status_code} - {resp.text}")

    # 13d. Get Rate Limits
    resp = requests.get(f"{BASE_URL}/api/admin/rate-limits?user_id=test_user", headers=headers_new_admin)
    if resp.status_code == 200:
        status = resp.json().get('status', {})
        print(f"    ✅ Rate Limits: OK (Window Count: {len(status)})")
    else:
        print(f"    ❌ Rate Limits Failed: {resp.status_code} - {resp.text}")

    # 13e. Delete Policy
    resp = requests.delete(f"{BASE_URL}/api/admin/policies/test_policy_1", headers=headers_new_admin)
    if resp.status_code == 200:
        print("    ✅ Delete Policy: OK")
    else:
        print(f"    ❌ Delete Policy Failed: {resp.status_code} - {resp.text}")

    return True

if __name__ == "__main__":
    try:
        if test_admin_api():
            exit(0)
        else:
            exit(1)
    except Exception as e:
        print(f"ERROR: {e}")
        exit(1)
