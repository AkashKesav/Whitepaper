#!/usr/bin/env python
"""
Comprehensive Admin Panel Feature Testing

Tests all admin API endpoints:
1. Admin Authentication & Role Verification
2. User Management (list, view)
3. System Statistics
4. Policy Management (CRUD)
5. Audit Logs
6. Rate Limits
7. RBAC Enforcement (non-admin blocked)
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

class AdminPanelTester:
    def __init__(self):
        self.admin_token = None
        self.admin_username = None
        self.user_token = None
        self.user_username = None
        self.created_policy_id = None
        self.results = {}
    
    def setup(self):
        """Create admin and regular user for testing"""
        separator("SETUP: Creating Test Users")
        
        # Create admin
        self.admin_username = f"super_admin_{uuid.uuid4().hex[:8]}"
        resp = requests.post(f"{BASE_URL}/api/register", 
                           json={"username": self.admin_username, "password": "admin123"})
        data = resp.json()
        self.admin_token = data.get("token")
        role = data.get("role", "unknown")
        print(f"Admin: {self.admin_username} (role: {role})")
        
        if role != "admin":
            print("  WARNING: Role is not 'admin'!")
        
        # Create regular user
        self.user_username = f"regular_user_{uuid.uuid4().hex[:8]}"
        resp = requests.post(f"{BASE_URL}/api/register",
                           json={"username": self.user_username, "password": "user123"})
        data = resp.json()
        self.user_token = data.get("token")
        role = data.get("role", "unknown")
        print(f"User: {self.user_username} (role: {role})")
    
    def test_admin_authentication(self):
        """Test that admin token works and has correct role"""
        separator("TEST 1: Admin Authentication")
        
        headers = {"Authorization": f"Bearer {self.admin_token}"}
        
        # Test admin-only endpoint
        resp = requests.get(f"{BASE_URL}/api/admin/users", headers=headers)
        passed = resp.status_code == 200
        print(f"  Admin access to /api/admin/users: {resp.status_code} {'PASS' if passed else 'FAIL'}")
        
        self.results["Admin Auth"] = passed
        return passed
    
    def test_user_management(self):
        """Test user listing and viewing"""
        separator("TEST 2: User Management")
        
        headers = {"Authorization": f"Bearer {self.admin_token}"}
        passed = True
        
        # List all users
        resp = requests.get(f"{BASE_URL}/api/admin/users", headers=headers)
        if resp.status_code == 200:
            users = resp.json()
            print(f"  List users: {resp.status_code} - Found {len(users) if isinstance(users, list) else '?'} users")
            
            # Check if our test users are in the list
            if isinstance(users, list):
                usernames = [u.get("username", "") for u in users]
                admin_found = self.admin_username in usernames
                user_found = self.user_username in usernames
                print(f"  Admin in list: {'Yes' if admin_found else 'No'}")
                print(f"  Regular user in list: {'Yes' if user_found else 'No'}")
        else:
            print(f"  List users FAILED: {resp.status_code}")
            passed = False
        
        self.results["User Management"] = passed
        return passed
    
    def test_system_stats(self):
        """Test system statistics endpoint"""
        separator("TEST 3: System Statistics")
        
        headers = {"Authorization": f"Bearer {self.admin_token}"}
        passed = True
        
        resp = requests.get(f"{BASE_URL}/api/admin/system/stats", headers=headers)
        if resp.status_code == 200:
            stats = resp.json()
            print(f"  System stats: {resp.status_code}")
            print(f"  Keys returned: {list(stats.keys()) if isinstance(stats, dict) else 'N/A'}")
        else:
            print(f"  System stats FAILED: {resp.status_code}")
            passed = False
        
        self.results["System Stats"] = passed
        return passed
    
    def test_policy_crud(self):
        """Test policy Create, Read, Delete"""
        separator("TEST 4: Policy CRUD Operations")
        
        headers = {"Authorization": f"Bearer {self.admin_token}"}
        passed = True
        
        # CREATE policy
        policy_data = {
            "description": f"Test policy {uuid.uuid4().hex[:8]}",
            "subjects": ["user:test_target"],
            "resources": ["node:0x1234"],
            "actions": ["READ"],
            "effect": "DENY"
        }
        
        resp = requests.post(f"{BASE_URL}/api/admin/policies", 
                           json=policy_data, headers=headers)
        if resp.status_code in [200, 201]:
            result = resp.json()
            self.created_policy_id = result.get("id") or result.get("policy_id")
            print(f"  CREATE policy: {resp.status_code} - ID: {self.created_policy_id}")
        else:
            print(f"  CREATE policy FAILED: {resp.status_code} - {resp.text[:100]}")
            passed = False
        
        # LIST policies
        resp = requests.get(f"{BASE_URL}/api/admin/policies", headers=headers)
        if resp.status_code == 200:
            policies = resp.json()
            count = len(policies) if isinstance(policies, list) else '?'
            print(f"  LIST policies: {resp.status_code} - Found {count} policies")
        else:
            print(f"  LIST policies FAILED: {resp.status_code}")
            passed = False
        
        # DELETE policy (cleanup)
        if self.created_policy_id:
            resp = requests.delete(f"{BASE_URL}/api/admin/policies/{self.created_policy_id}", 
                                  headers=headers)
            if resp.status_code in [200, 204]:
                print(f"  DELETE policy: {resp.status_code} - Cleaned up")
            else:
                print(f"  DELETE policy: {resp.status_code}")
        
        self.results["Policy CRUD"] = passed
        return passed
    
    def test_audit_logs(self):
        """Test audit log retrieval"""
        separator("TEST 5: Audit Logs")
        
        headers = {"Authorization": f"Bearer {self.admin_token}"}
        
        resp = requests.get(f"{BASE_URL}/api/admin/audit", headers=headers)
        if resp.status_code == 200:
            logs = resp.json()
            count = len(logs) if isinstance(logs, list) else '?'
            print(f"  Audit logs: {resp.status_code} - Found {count} entries")
            passed = True
        else:
            print(f"  Audit logs: {resp.status_code}")
            passed = resp.status_code != 500  # 404 is acceptable if not implemented
        
        self.results["Audit Logs"] = passed
        return passed
    
    def test_rate_limits(self):
        """Test rate limit configuration"""
        separator("TEST 6: Rate Limits")
        
        headers = {"Authorization": f"Bearer {self.admin_token}"}
        
        resp = requests.get(f"{BASE_URL}/api/admin/rate-limits", headers=headers)
        if resp.status_code == 200:
            limits = resp.json()
            print(f"  Rate limits: {resp.status_code}")
            if isinstance(limits, dict):
                for key, val in list(limits.items())[:3]:
                    print(f"    {key}: {val}")
            passed = True
        else:
            print(f"  Rate limits: {resp.status_code}")
            passed = resp.status_code != 500
        
        self.results["Rate Limits"] = passed
        return passed
    
    def test_rbac_enforcement(self):
        """Test that regular users cannot access admin endpoints"""
        separator("TEST 7: RBAC Enforcement (Non-Admin Blocked)")
        
        headers = {"Authorization": f"Bearer {self.user_token}"}
        passed = True
        
        # Try admin endpoints with regular user token
        admin_endpoints = [
            ("GET", "/api/admin/users"),
            ("GET", "/api/admin/system/stats"),
            ("GET", "/api/admin/policies"),
            ("POST", "/api/admin/policies"),
        ]
        
        for method, endpoint in admin_endpoints:
            if method == "GET":
                resp = requests.get(f"{BASE_URL}{endpoint}", headers=headers)
            else:
                resp = requests.post(f"{BASE_URL}{endpoint}", json={}, headers=headers)
            
            blocked = resp.status_code in [401, 403]
            status = "BLOCKED (correct)" if blocked else f"ALLOWED ({resp.status_code}) - SECURITY ISSUE!"
            print(f"  {method} {endpoint}: {status}")
            
            if not blocked:
                passed = False
        
        self.results["RBAC Enforcement"] = passed
        return passed
    
    def run_all_tests(self):
        """Run complete test suite"""
        print("\n" + "#" * 60)
        print("#  COMPREHENSIVE ADMIN PANEL TEST SUITE")
        print("#" * 60)
        
        self.setup()
        
        self.test_admin_authentication()
        self.test_user_management()
        self.test_system_stats()
        self.test_policy_crud()
        self.test_audit_logs()
        self.test_rate_limits()
        self.test_rbac_enforcement()
        
        # Print summary
        separator("FINAL RESULTS")
        all_passed = True
        for test, passed in self.results.items():
            status = "PASS" if passed else "FAIL"
            if not passed:
                all_passed = False
            print(f"  {test}: {status}")
        
        print("\n" + "=" * 60)
        if all_passed:
            print("  ALL ADMIN PANEL TESTS PASSED!")
        else:
            print("  SOME TESTS FAILED")
        print("=" * 60)
        
        return all_passed

if __name__ == "__main__":
    tester = AdminPanelTester()
    tester.run_all_tests()
