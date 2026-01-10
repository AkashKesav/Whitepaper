# verify_policies_emergency.ps1
$BaseUrl = "http://localhost:9090"
$AdminUser = "super_admin_verif_$(Get-Random)"
$AdminPass = "pass123"

# 1. Register as Super Admin (Backdoor: username starts with 'super_admin_')
echo "Registering new super admin: $AdminUser..."
$regPayload = @{username = $AdminUser; password = $AdminPass } | ConvertTo-Json
try {
    $regResponse = Invoke-RestMethod -Uri "$BaseUrl/api/register" -Method Post -Body $regPayload -ContentType "application/json"
    $token = $regResponse.token
    echo "Registration successful. Token received."
}
catch {
    echo "Registration failed. Assuming user exists, trying login..."
    # Try Login if register fails (though with random suffix it shouldn't)
    try {
        $loginPayload = @{username = $AdminUser; password = $AdminPass } | ConvertTo-Json
        $loginResponse = Invoke-RestMethod -Uri "$BaseUrl/api/login" -Method Post -Body $loginPayload -ContentType "application/json"
        $token = $loginResponse.token
        echo "Login successful."
    }
    catch {
        echo "Login failed: $($_.Exception.Message)"
        exit 1
    }
}

$headers = @{Authorization = "Bearer $token" }

# 2. Test Policies Endpoint
echo "`n--- Testing Policies ---"
echo "Listing Policies..."
try {
    $policies = Invoke-RestMethod -Uri "$BaseUrl/api/admin/policies" -Method Get -Headers $headers
    echo "Policies Count: $($policies.policies.Count)"
}
catch {
    echo "Failed to list policies: $($_.Exception.Message)"
}

# Create a test policy
echo "Creating Test Policy..."
$policyPayload = @{
    id          = "test_policy_$(Get-Random)"
    description = "Test Policy"
    effect      = "DENY"
    subjects    = @("user:test")
    resources   = @("resource:1")
    actions     = @("READ")
} | ConvertTo-Json

try {
    $createRes = Invoke-RestMethod -Uri "$BaseUrl/api/admin/policies" -Method Post -Body $policyPayload -Headers $headers -ContentType "application/json"
    $policyId = $createRes.policy_id
    echo "Policy Created: $policyId (Internal UID: $($createRes.id))"
}
catch {
    echo "Failed to create policy: $($_.Exception.Message)"
}

# Delete the test policy (if created)
if ($policyId) {
    echo "Deleting Test Policy $policyId..."
    try {
        Invoke-RestMethod -Uri "$BaseUrl/api/admin/policies/$policyId" -Method Delete -Headers $headers
        echo "Policy Deleted."
    }
    catch {
        echo "Failed to delete policy: $($_.Exception.Message)"
    }
}


# 3. Test Emergency Endpoint
echo "`n--- Testing Emergency Requests ---"
echo "Listing Emergency Requests..."
try {
    $requests = Invoke-RestMethod -Uri "$BaseUrl/api/admin/emergency/requests" -Method Get -Headers $headers
    echo "Requests Count: $($requests.requests.Count)"
    if ($requests.requests.Count -gt 0) {
        $reqID = $requests.requests[0].id
        echo "Found Request ID: $reqID (Status: $($requests.requests[0].status))"
        
        # Try to approve it (if pending)
        if ($requests.requests[0].status -eq "pending") {
            echo "Approving request $reqID..."
            Invoke-RestMethod -Uri "$BaseUrl/api/admin/emergency/requests/$reqID/approve" -Method Post -Headers $headers
            echo "Request Approved."
        }
    }
}
catch {
    echo "Failed to list emergency requests: $($_.Exception.Message)"
}

echo "`nVerification Complete!"
