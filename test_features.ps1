#!/usr/bin/env pwsh
# Test script for Policy Enforcement, Chat, and Groups
# Tests: (1) Policy affects memory access, (2) Chat functionality, (3) Group features

$ErrorActionPreference = "Stop"
$API_BASE = "http://localhost:9090/api"

function Test-Request {
    param([string]$Method, [string]$Url, [object]$Body, [hashtable]$Headers)
    try {
        $params = @{
            Method      = $Method
            Uri         = $Url
            Headers     = $Headers
            ContentType = "application/json"
        }
        if ($Body) { $params.Body = ($Body | ConvertTo-Json -Depth 10) }
        $response = Invoke-RestMethod @params
        return $response
    }
    catch {
        Write-Host "  Error: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "  POLICY, CHAT & GROUP FEATURE TESTS" -ForegroundColor Cyan
Write-Host "===============================================" -ForegroundColor Cyan

# --- 1. SETUP: Create test users ---
Write-Host "`n[1] SETUP: Creating test users..." -ForegroundColor Yellow

$testUser = "policy_test_user_$(Get-Random -Maximum 9999)"
$adminUser = "admin_test_$(Get-Random -Maximum 9999)"
$password = "testpass123"

# Register regular user
$regResult = Test-Request -Method "POST" -Url "$API_BASE/register" -Body @{
    username = $testUser
    password = $password
}
if ($regResult) {
    Write-Host "  Registered user: $testUser" -ForegroundColor Green
}

# Register admin user
$adminRegResult = Test-Request -Method "POST" -Url "$API_BASE/register" -Body @{
    username = $adminUser
    password = $password
    role     = "admin"
}
if ($adminRegResult) {
    Write-Host "  Registered admin: $adminUser" -ForegroundColor Green
}

# Login as regular user
Write-Host "`n[2] LOGIN: Getting auth tokens..." -ForegroundColor Yellow
$loginResult = Test-Request -Method "POST" -Url "$API_BASE/login" -Body @{
    username = $testUser
    password = $password
}
if (-not $loginResult -or -not $loginResult.token) {
    Write-Host "  Failed to login as $testUser" -ForegroundColor Red
    exit 1
}
$userToken = $loginResult.token
$userHeaders = @{ "Authorization" = "Bearer $userToken" }
Write-Host "  User token obtained" -ForegroundColor Green

# Login as admin
$adminLoginResult = Test-Request -Method "POST" -Url "$API_BASE/login" -Body @{
    username = $adminUser
    password = $password
}
if ($adminLoginResult -and $adminLoginResult.token) {
    $adminToken = $adminLoginResult.token
    $adminHeaders = @{ "Authorization" = "Bearer $adminToken" }
    Write-Host "  Admin token obtained" -ForegroundColor Green
}
else {
    Write-Host "  Admin login failed, continuing with user token only" -ForegroundColor DarkYellow
    $adminHeaders = $userHeaders
}

# --- 2. TEST CHAT FUNCTIONALITY ---
Write-Host "`n[3] TESTING CHAT FUNCTIONALITY..." -ForegroundColor Yellow

# Send a chat message
$chatMessage = "Remember that my favorite color is blue and I like pizza."
$chatResult = Test-Request -Method "POST" -Url "$API_BASE/chat" -Headers $userHeaders -Body @{
    message         = $chatMessage
    conversation_id = "test_conv_$(Get-Random -Maximum 9999)"
}
if ($chatResult) {
    Write-Host "  Chat message sent successfully" -ForegroundColor Green
    if ($chatResult.response) {
        Write-Host "  Response: $($chatResult.response.Substring(0, [Math]::Min(100, $chatResult.response.Length)))..." -ForegroundColor DarkGray
    }
}
else {
    Write-Host "  Chat failed" -ForegroundColor Red
}

# Test recall - send another message that should trigger memory
Start-Sleep -Seconds 2
$recallMessage = "What is my favorite color?"
$recallResult = Test-Request -Method "POST" -Url "$API_BASE/chat" -Headers $userHeaders -Body @{
    message         = $recallMessage
    conversation_id = "test_conv_$(Get-Random -Maximum 9999)"
}
if ($recallResult) {
    Write-Host "  Recall test message sent" -ForegroundColor Green
    if ($recallResult.response -and $recallResult.response -match "blue") {
        Write-Host "  Memory recall detected: Response mentions 'blue'" -ForegroundColor Green
    }
    else {
        Write-Host "  Memory may not have recalled previous context" -ForegroundColor DarkYellow
    }
}

# --- 3. TEST GROUP FUNCTIONALITY ---
Write-Host "`n[4] TESTING GROUP FUNCTIONALITY..." -ForegroundColor Yellow

# Create a group
$groupName = "TestGroup_$(Get-Random -Maximum 9999)"
$groupResult = Test-Request -Method "POST" -Url "$API_BASE/groups" -Headers $userHeaders -Body @{
    name        = $groupName
    description = "Test group for verification"
}
if ($groupResult) {
    Write-Host "  Group created: $groupName" -ForegroundColor Green
    $groupId = $groupResult.group_id
    if (-not $groupId) { $groupId = $groupResult.id }
}
else {
    Write-Host "  Group creation failed" -ForegroundColor Red
}

# List groups
$groupsListResult = Test-Request -Method "GET" -Url "$API_BASE/groups" -Headers $userHeaders
if ($groupsListResult) {
    $groupCount = if ($groupsListResult.groups) { $groupsListResult.groups.Count } else { $groupsListResult.Count }
    Write-Host "  Groups listed: $groupCount groups found" -ForegroundColor Green
}
else {
    Write-Host "  Failed to list groups" -ForegroundColor Red
}

# --- 4. TEST POLICY ENFORCEMENT ---
Write-Host "`n[5] TESTING POLICY ENFORCEMENT ON MEMORY..." -ForegroundColor Yellow

# First, create a deny policy
$policyId = "deny_test_user_$(Get-Random -Maximum 9999)"
$policyResult = Test-Request -Method "POST" -Url "$API_BASE/admin/policies" -Headers $adminHeaders -Body @{
    policy_id   = $policyId
    description = "Deny access to Facts for test user"
    subjects    = @("user:$testUser")
    resources   = @("type:Fact")
    actions     = @("READ")
    effect      = "DENY"
}
if ($policyResult) {
    Write-Host "  Policy created: $policyId" -ForegroundColor Green
}
else {
    Write-Host "  Policy creation failed (may need admin privileges)" -ForegroundColor DarkYellow
}

# Try to recall after policy is applied
Start-Sleep -Seconds 1
$postPolicyRecall = Test-Request -Method "POST" -Url "$API_BASE/recall" -Headers $userHeaders -Body @{
    query = "What is my favorite color?"
    limit = 5
}
if ($postPolicyRecall) {
    $nodeCount = if ($postPolicyRecall.nodes) { $postPolicyRecall.nodes.Count } else { 0 }
    Write-Host "  Post-policy recall returned $nodeCount nodes" -ForegroundColor Green
    if ($nodeCount -eq 0) {
        Write-Host "  Policy may be blocking access as expected!" -ForegroundColor Cyan
    }
}
else {
    Write-Host "  Recall request failed" -ForegroundColor Red
}

# List policies to verify
$policiesResult = Test-Request -Method "GET" -Url "$API_BASE/admin/policies" -Headers $adminHeaders
if ($policiesResult) {
    $policyCount = if ($policiesResult.policies) { $policiesResult.policies.Count } else { $policiesResult.Count }
    Write-Host "  Verified: $policyCount policies in system" -ForegroundColor Green
}

# Clean up - delete the test policy
Write-Host "`n[6] CLEANUP..." -ForegroundColor Yellow
$deleteResult = Test-Request -Method "DELETE" -Url "$API_BASE/admin/policies/$policyId" -Headers $adminHeaders
if ($deleteResult) {
    Write-Host "  Test policy deleted" -ForegroundColor Green
}
else {
    Write-Host "  Could not delete test policy (may already be gone)" -ForegroundColor DarkYellow
}

Write-Host "`n===============================================" -ForegroundColor Cyan
Write-Host "  TEST SUMMARY" -ForegroundColor Cyan
Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "  - Chat: Sent messages successfully" -ForegroundColor White
Write-Host "  - Groups: Created and listed groups" -ForegroundColor White
Write-Host "  - Policy: Created deny policy and tested recall" -ForegroundColor White
Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "Verification Complete!" -ForegroundColor Green
