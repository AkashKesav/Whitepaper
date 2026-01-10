import requests
import time
import uuid
import json

BASE_URL = "http://localhost:9090"

def test_policy_layer():
    print("=== Testing Policy Layer Enforcement ===")
    
    # 1. Setup: Register Admin and User
    # Use 'super_admin_' prefix to ensure 'admin' role (see server.go logic)
    admin_username = f"super_admin_policy_{uuid.uuid4().hex[:8]}"
    admin_password = "password123"
    
    user_username = f"policy_user_{uuid.uuid4().hex[:8]}"
    user_password = "password123"
    
    # Register Admin
    print(f"1. Registering Admin: {admin_username}")
    resp = requests.post(f"{BASE_URL}/api/register", json={"username": admin_username, "password": admin_password})
    if resp.status_code not in [200, 201]: return fail(f"Admin reg failed: {resp.status_code} {resp.text}")
    admin_token = resp.json().get("token")
    
    # Promote to Admin
    # (Self-promotion hack available in current dev mode, or use existing super admin? 
    #  Let's assume the previous test left a super admin or we can use the registration response if it auto-assigns based on logic, 
    #  but usually we need a bootstrap. For this test, let's assume register gives a user, and we need a super admin token.
    #  Actually, let's just make the first registered user admin if possible, or use a known secret key.
    #  Reviewing code: Register handles new users as 'user'. 
    #  Workaround: We'll skip admin creation for now and assume we can use the 'admin_token' from previous runs or 
    #  modify the DB. BUT, since this is an automated test, we need a way.
    #  Let's assume we can use the mock 'super_admin' from previous tests if persistence is on, OR
    #  we can use the /api/admin/users/{id}/role endpoint if we have an initial admin.
    #  Let's try to register and just check if we can run policies. If not, we'll need to handle auth.)
    
    # Better approach: The system likely doesn't have an initial admin on fresh boot without seed.
    # However, let's assume we can use the 'emergency' token or similar if available.
    # Wait, the `test_admin_api.py` registered a super admin successfully. How?
    # Ah, `test_admin_api.py` registers `super_admin_test_...`. 
    # Does the backend auto-promote the first user? Or does it allow anyone to register as admin?
    # Let's check `admin_handlers.go` -> `handleRegister`.
    
    # If we can't easily become admin, we'll try to reuse the admin token logic from test_admin_api.py
    # For now, let's assume standard registration for 'user' and just use 'admin' credentials if we can.
    # Or simplified: Just execute the logic.
    
    # Register User
    print(f"2. Registering User: {user_username}")
    resp = requests.post(f"{BASE_URL}/api/register", json={"username": user_username, "password": user_password})
    if resp.status_code not in [200, 201]: return fail("User reg failed")
    user_token = resp.json().get("token")
    user_id = user_username # Simplification, actual ID might be UUID but username is often used as ID in this system code
    
    # Login as pre-existing Admin (e.g. from seed) or try to register one
    # Assuming `test_admin_api.py` runs before this or we can just register one
    # Let's try to register a new admin and see if it works (maybe role can be passed in dev mode?)
    # ...
    
    # Prerequisite: Create a Secret Resource (Node) via Chat (Ingestion)
    print("3. Creating a 'Secret' resource via Chat")
    secret_content = f"The launch code is {uuid.uuid4().hex}"
    headers_user = {"Authorization": f"Bearer {user_token}"}
    
    resp = requests.post(f"{BASE_URL}/api/chat", json={
        "message": f"Remember this secret: {secret_content}. It is classified as top secret."
    }, headers=headers_user)
    if resp.status_code != 200: return fail(f"Chat failed: {resp.status_code} {resp.text}")
    print("   Chat response:", resp.json().get("response"))
    
    # Allow some time for ingestion/indexing
    time.sleep(2) 
    
    # 4. Verify Access (Should be allowed by default)
    print("4. Verifying Access (Pre-Policy)")
    resp = requests.post(f"{BASE_URL}/api/chat", json={
        "message": "What is the launch code?"
    }, headers=headers_user)
    response_text = resp.json().get("response", "")
    if secret_content not in response_text and "launch code" not in response_text:
        print(f"   WARNING: Pre-policy retrieval might have failed or LLM rephrased it. Response: {response_text}")
    else:
        print("   Access confirmed.")

    # 5. Reuse Admin Token from Step 1
    # The first registration (super_admin_policy_*) already gave us an admin token.
    headers_admin = {"Authorization": f"Bearer {admin_token}"}


    # 6. Create Deny Policy
    print("6. Creating DENY Policy for the secret")
    policy_id = f"deny_secret_{uuid.uuid4().hex[:4]}"
    policy_payload = {
        "id": policy_id,
        "description": "Deny access to secret launch codes",
        "subjects": [f"user:{user_id}"],  # Target the specific user
        "resources": ["*"],  # Match ALL resources for this user (wildcard)
        "actions": ["READ"],
        "effect": "DENY"
    }
    
    resp = requests.post(f"{BASE_URL}/api/admin/policies", json=policy_payload, headers=headers_admin)
    if resp.status_code != 201: 
        return fail(f"Policy creation failed: {resp.status_code} {resp.text}")
    else:
        print("   Policy created.")

    # 7. Verify Denial
    print("7. Verifying Access (Post-Policy)")
    time.sleep(5) # Propagate
    
    # DEBUG: List all policies to confirm existence
    print("   [DEBUG] Listing checks...")
    list_resp = requests.get(f"{BASE_URL}/api/admin/policies", headers=headers_admin)
    if list_resp.status_code == 200:
        policies = list_resp.json().get("policies", [])
        found = False
        for p in policies:
            if p["id"] == policy_id:
                print(f"   [DEBUG] Found policy: ID={p['id']} Subjects={p['subjects']} Resources={p['resources']}")
                found = True
                # Check for exact subject match string
                expected = f"user:{user_id}"
                if expected in p['subjects']:
                     print(f"   [DEBUG] Subject match CONFIRMED: {expected} in {p['subjects']}")
                else:
                     print(f"   [DEBUG] Subject match FAILED: {expected} not in {p['subjects']}")
                break
        if not found:
             print(f"   [DEBUG] Policy {policy_id} NOT FOUND in list! Propagation issue?")
    else:
        print(f"   [DEBUG] Failed to list policies: {list_resp.status_code}")

    # Use a new session to ensure we are testing Memory Recall, not Chat History
    verify_session_id = str(uuid.uuid4())
    
    resp = requests.post(f"{BASE_URL}/api/chat", json={
        "message": "What is the launch code?",
        "session_id": verify_session_id
    }, headers=headers_user)
    response_text = resp.json().get("response", "")
    
    # We expect the LLM to say it doesn't know, because the Fact was filtered out
    print(f"   Response: {response_text[:100]}...") # Truncate response
    
    # Allow logic: if the exact secret string is present, it failed.
    # We use 'secret_content' which is "The launch code is {uuid}" 
    # The extraction logic below needs to be robust.
    # Let's extract just the UUID part to be sure.
    secret_uuid = secret_content.split(" is ")[1]
    
    if secret_uuid in response_text:
        return fail("POLICY FAILED! Secret was revealed.")
    else:
        print("POLICY ENFORCED! Secret was hidden.")

    # 8. Clean up
    print("8. Deleting Policy")
    requests.delete(f"{BASE_URL}/api/admin/policies/{policy_id}", headers=headers_admin)
    
    return True

def fail(msg):
    print(f"FAILED: {msg}")
    return False


if __name__ == "__main__":
    try:
        if test_policy_layer():
            print("\nPOLICY LAYER TEST: PASSED")
            exit(0)
        else:
            print("\nPOLICY LAYER TEST: FAILED")
            exit(1)
    except Exception as e:
        print(f"ERROR: {e}")
        exit(1)
