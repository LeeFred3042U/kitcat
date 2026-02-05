# Stress test for concurrent kitcat add operations
# This script verifies that the index remains valid under concurrent writes

Write-Host "=== Kitcat Concurrent Write Stress Test ===" -ForegroundColor Cyan
Write-Host ""

# Path to kitcat executable
$KITCAT_EXE = "d:\Github\kitcat\kitcat.exe"

# Create a temporary test directory
$TEST_DIR = New-Item -ItemType Directory -Path (Join-Path $env:TEMP "kitcat_stress_test_$(Get-Random)")
Write-Host "Test directory: $TEST_DIR"
Set-Location $TEST_DIR

# Initialize a kitcat repository
Write-Host "Initializing kitcat repository..."
& $KITCAT_EXE init
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Failed to initialize kitcat repository" -ForegroundColor Red
    exit 1
}

# Create test files
Write-Host "Creating test files..."
1..20 | ForEach-Object {
    "Test content $_" | Out-File -FilePath "file_$_.txt" -Encoding utf8
}

# Run concurrent add operations
Write-Host ""
Write-Host "Running 20 concurrent 'kitcat add' operations..." -ForegroundColor Yellow
Write-Host "This tests the file locking and atomic write mechanisms..."
Write-Host ""

# Use PowerShell jobs for concurrent execution
$jobs = 1..20 | ForEach-Object {
    $file = "file_$_.txt"
    Start-Job -ScriptBlock {
        param($file, $dir, $kitcatExe)
        Set-Location $dir
        & $kitcatExe add $file 2>&1
    } -ArgumentList $file, $TEST_DIR.FullName, $KITCAT_EXE
}

# Wait for all jobs to complete
$jobs | Wait-Job | Out-Null
$jobs | Remove-Job

Write-Host ""
Write-Host "All operations completed. Verifying index integrity..." -ForegroundColor Yellow
Write-Host ""

# Verify the index file exists
$indexPath = ".kitcat\index"
if (-not (Test-Path $indexPath)) {
    Write-Host "ERROR: Index file does not exist!" -ForegroundColor Red
    exit 1
}

# Verify the index is valid JSON
try {
    $index = Get-Content $indexPath -Raw | ConvertFrom-Json
} catch {
    Write-Host "ERROR: Index file is not valid JSON!" -ForegroundColor Red
    Write-Host "Index contents:"
    Get-Content $indexPath
    exit 1
}

# Count entries in the index
$indexCount = ($index.PSObject.Properties | Measure-Object).Count
Write-Host "Index contains $indexCount entries"

# Verify all 20 files are in the index
$EXPECTED_COUNT = 20
if ($indexCount -ne $EXPECTED_COUNT) {
    Write-Host "WARNING: Expected $EXPECTED_COUNT entries, but found $indexCount" -ForegroundColor Yellow
    Write-Host "This might indicate lost updates due to race conditions"
    Write-Host ""
    Write-Host "Index contents:"
    $index | ConvertTo-Json
    exit 1
}

# Verify each file is in the index
Write-Host "Verifying all files are tracked..."
$MISSING_FILES = 0
1..20 | ForEach-Object {
    $fileName = "file_$_.txt"
    if (-not ($index.PSObject.Properties.Name -contains $fileName)) {
        Write-Host "ERROR: $fileName is missing from the index!" -ForegroundColor Red
        $MISSING_FILES++
    }
}

if ($MISSING_FILES -gt 0) {
    Write-Host ""
    Write-Host "ERROR: $MISSING_FILES files are missing from the index!" -ForegroundColor Red
    exit 1
}

# Clean up
Set-Location ..
Remove-Item -Recurse -Force $TEST_DIR

Write-Host ""
Write-Host "SUCCESS: All tests passed!" -ForegroundColor Green
Write-Host "  - Index is valid JSON"
Write-Host "  - All 20 files are tracked"
Write-Host "  - No corruption detected"
Write-Host ""
Write-Host "The atomic write implementation is working correctly under concurrent load." -ForegroundColor Green
