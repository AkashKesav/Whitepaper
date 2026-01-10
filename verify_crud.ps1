# Admin CRUD Verification Script

$baseUrl = "http://localhost:9090/api"

function Invoke-ApiRequest {
    param (
        [string]$Method,
        [string]$Path,
        [hashtable]$Body = @{},
        [string]$Token = ""
    )
    
    $headers = @{ "Content-Type" = "application/json" }
    if ($Token) {
        $headers["Authorization"] = "Bearer $Token"
    }

    $jsonBody = $Body | ConvertTo-Json -Depth 10
    
    Write-Host "[$Method] $Path" -ForegroundColor Cyan
    try {
        if ($Method -eq "GET" -or $Method -eq "DELETE") {
             $response = Invoke-RestMethod -Uri "$baseUrl$Path" -Method $Method -Headers $headers -ErrorAction Stop
        } else {
             $response = Invoke-RestMethod -Uri "$baseUrl$Path" -Method $Method -Headers $headers -Body $jsonBody -ErrorAction Stop
        }
        Write-Host "Success" -ForegroundColor Green
        return $response
    } catch {
        Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.Exception.Response) {
             $stream = $_.Exception.Response.GetResponseStream()
             $reader = New-Object System.IO.StreamReader($stream)
             $responseBody = $reader.ReadToEnd()
             Write-Host "Response Body: $responseBody" -ForegroundColor Yellow
        }
    }
}

# 1. Register/Login as Super Admin
Write-Host "`n--- Authentication ---" -ForegroundColor Magenta
Invoke-ApiRequest -Method "POST" -Path "/register" -Body @{ username="super_admin_verify"; password="password123" } | Out-Null
$login = Invoke-ApiRequest -Method "POST" -Path "/login" -Body @{ username="super_admin_verify"; password="password123" }
$token = $login.token

if (-not $token) {
    Write-Host "Failed to login. Exiting." -ForegroundColor Red
    exit
}
Write-Host "Logged in with Token" -ForegroundColor Gray

# 2. User CRUD
Write-Host "`n--- User CRUD ---" -ForegroundColor Magenta
$user = @{ username="verify_user"; password="password123"; role="user" }
Invoke-ApiRequest -Method "POST" -Path "/admin/users" -Body $user -Token $token

# Verify user exists (simplistic check via delete as get requires list parsing)
Invoke-ApiRequest -Method "DELETE" -Path "/admin/users/verify_user" -Token $token


# 3. Affiliate CRUD
Write-Host "`n--- Affiliate CRUD ---" -ForegroundColor Magenta
$affiliate = @{ code="VERIFY_AFF"; user="verify_user"; commission_rate=0.2 }
Invoke-ApiRequest -Method "POST" -Path "/admin/affiliates" -Body $affiliate -Token $token
Invoke-ApiRequest -Method "DELETE" -Path "/admin/affiliates/VERIFY_AFF" -Token $token


# 4. Campaign CRUD
Write-Host "`n--- Campaign CRUD ---" -ForegroundColor Magenta
$campaign = @{ id="camp_verify"; name="Verify Campaign"; type="email"; target_audience="all"; status="draft" }
Invoke-ApiRequest -Method "POST" -Path "/admin/operations/campaigns" -Body $campaign -Token $token
Invoke-ApiRequest -Method "DELETE" -Path "/admin/operations/campaigns/camp_verify" -Token $token

Write-Host "`nVerification Complete" -ForegroundColor Green
