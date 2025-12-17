# Test Memory Pipeline with Progress Monitoring
# This script stores information, polls DGraph until entity is found, then retrieves

param(
    [string]$StorageMessage = "My cat's name is Luna.",
    [string]$RetrieveQuery = "What is my cat's name?",
    [string]$SearchTerm = "Luna",
    [int]$MaxWaitSeconds = 120
)

$BaseAPI = "http://localhost:3000/api/chat"
$DGraphAPI = "http://localhost:8180/query"

# Create persistent session
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  MEMORY PIPELINE TEST" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# STEP 1: Store Information
Write-Host "[STEP 1] STORING INFORMATION" -ForegroundColor Yellow
Write-Host "Message: $StorageMessage" -ForegroundColor White
Write-Host ""

$body = @{ message = $StorageMessage } | ConvertTo-Json
$response = Invoke-RestMethod -Uri $BaseAPI -Method POST -ContentType "application/json" -Body $body -WebSession $session
Write-Host "Agent Response: $($response.response)" -ForegroundColor Green
Write-Host ""

# STEP 2: Wait for Ingestion (poll until entity appears in DGraph)
Write-Host "[STEP 2] WAITING FOR INGESTION..." -ForegroundColor Yellow
Write-Host "Searching DGraph for: '$SearchTerm'" -ForegroundColor Gray
Write-Host ""

$startTime = Get-Date
$found = $false
$dots = 0

while (-not $found) {
    $elapsed = ((Get-Date) - $startTime).TotalSeconds
    
    if ($elapsed -gt $MaxWaitSeconds) {
        Write-Host ""
        Write-Host "[TIMEOUT] Entity not found after ${MaxWaitSeconds}s" -ForegroundColor Red
        break
    }
    
    # Build DGraph query to check if entity exists
    $dgraphQuery = "{ result(func: eq(name, `"$SearchTerm`")) { uid name description } }"
    $body = @{ query = $dgraphQuery } | ConvertTo-Json
    
    try {
        $dgResult = Invoke-RestMethod -Uri $DGraphAPI -Method POST -ContentType "application/json" -Body $body -ErrorAction SilentlyContinue
        # DEBUG PRINT
        Write-Host "DEBUG: $($dgResult | ConvertTo-Json -Depth 10)" -ForegroundColor DarkGray
        
        if ($dgResult.data.result -and $dgResult.data.result.Count -gt 0) {
            $found = $true
            $entityName = $dgResult.data.result[0].name
            $entityUid = $dgResult.data.result[0].uid
            $entityDesc = $dgResult.data.result[0].description
        }
    } catch {
        Write-Host "Error querying DGraph: $_" -ForegroundColor Red
    }
    
    if (-not $found) {
        $dots = ($dots + 1) % 4
        $dotsStr = "." * ($dots + 1)
        $elapsedStr = [math]::Round($elapsed, 0)
        Write-Host "`rPolling DGraph$dotsStr ($elapsedStr s elapsed)   " -NoNewline
        Start-Sleep -Milliseconds 500
    }
}

Write-Host ""
if ($found) {
    $totalTime = [math]::Round(((Get-Date) - $startTime).TotalSeconds, 1)
    Write-Host ""
    Write-Host "[SUCCESS] Entity ingested in ${totalTime}s!" -ForegroundColor Green
    Write-Host "  Name: $entityName" -ForegroundColor Cyan
    Write-Host "  Description: $entityDesc" -ForegroundColor Cyan
    Write-Host "  UID: $entityUid" -ForegroundColor Gray
    Write-Host ""
    
    # STEP 3: Retrieve Information
    Write-Host "[STEP 3] RETRIEVING INFORMATION" -ForegroundColor Yellow
    Write-Host "Query: $RetrieveQuery" -ForegroundColor White
    Write-Host ""
    
    $body = @{ message = $RetrieveQuery } | ConvertTo-Json
    $response = Invoke-RestMethod -Uri $BaseAPI -Method POST -ContentType "application/json" -Body $body -WebSession $session
    
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "ANSWER:" -ForegroundColor Green
    Write-Host $response.response -ForegroundColor White
    Write-Host "========================================" -ForegroundColor Cyan
} else {
    Write-Host ""
    Write-Host "[FAILED] Ingestion did not complete" -ForegroundColor Red
    Write-Host "The entity '$SearchTerm' was not found in DGraph." -ForegroundColor Red
    Write-Host "Check the kernel and ai-services logs for errors." -ForegroundColor Yellow
}
Write-Host ""
