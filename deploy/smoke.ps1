<#
.SYNOPSIS
Runs the app and validator binaries side by side on loopback and checks the
HTTP surface.

.DESCRIPTION
Builds both binaries into a scratch directory, generates an ephemeral validator
key pair, starts the validator gateway and the app, then probes health, the
dashboard, and the vendored assets. Everything is torn down at the end.

.PARAMETER AppPort
Loopback port for the app. Defaults to 18095.

.PARAMETER ValidatorPort
Loopback port for the validator gateway. Defaults to 18096.

.EXAMPLE
./deploy/smoke.ps1
#>
[CmdletBinding()]
param(
    [int]$AppPort = 18095,
    [int]$ValidatorPort = 18096
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dir = Join-Path ([System.IO.Path]::GetTempPath()) "medvault-smoke"

if (Test-Path $dir) {
    Remove-Item -Recurse -Force $dir
}
New-Item -ItemType Directory -Force -Path $dir | Out-Null

$appExe = Join-Path $dir "medvault-app.exe"
$validatorExe = Join-Path $dir "medvault-validator.exe"

Push-Location $root
try {
    Write-Host "building binaries"
    go build -o $appExe ./cmd/app
    if ($LASTEXITCODE -ne 0) { throw "build app failed" }
    go build -o $validatorExe ./cmd/validator
    if ($LASTEXITCODE -ne 0) { throw "build validator failed" }
    go build -o (Join-Path $dir "medvault-validator-keygen.exe") ./cmd/validator-keygen
    if ($LASTEXITCODE -ne 0) { throw "build validator-keygen failed" }
}
finally {
    Pop-Location
}

$keyPair = (& (Join-Path $dir "medvault-validator-keygen.exe") -short).Trim()
if ($LASTEXITCODE -ne 0) { throw "validator keygen failed" }
$parts = $keyPair.Split(" ")
if ($parts.Length -ne 2) { throw "unexpected keygen output: $keyPair" }

$secret = "a1" * 32

$env:MEDVAULT_MASTER_AES256_KEY_HEX = $secret
$env:MEDVAULT_MASTER_ROOT_SECRET_HEX = $secret
$env:MEDVAULT_STEGO_HMAC_KEY_HEX = $secret
$env:MEDVAULT_SESSION_SECRET_HEX = $secret
$env:MEDVAULT_APP_ADDR = "127.0.0.1:$AppPort"
$env:MEDVAULT_VALIDATOR_ADDR = "127.0.0.1:$ValidatorPort"
$env:MEDVAULT_VALIDATOR_URL = "http://127.0.0.1:$ValidatorPort"
$env:MEDVAULT_DB_PATH = Join-Path $dir "medvault.db"
$env:MEDVAULT_VAULT_DIR = Join-Path $dir "encrypted_labs"
$env:MEDVAULT_STEGO_DIR = Join-Path $dir "stego_scans"
$env:MEDVAULT_BIOMETRIC_DIR = Join-Path $dir "biometrics"
$env:MEDVAULT_VIGENERE_KEY = "MEDVAULT-CLASSICAL-KEY-2026"
$env:MEDVAULT_TRANSPOSITION_KEY = "KLINIK"
$env:MEDVAULT_VALIDATOR_PRIVKEY_HEX = $parts[0]
$env:MEDVAULT_VALIDATOR_PUBKEY_HEX = $parts[1]
$env:MEDVAULT_WEBAUTHN_RP_DISPLAY_NAME = "MedVault Smoke"
$env:MEDVAULT_WEBAUTHN_RPID = "localhost"
$env:MEDVAULT_WEBAUTHN_ORIGINS = "http://localhost:$AppPort"
$env:MEDVAULT_SESSION_TTL_MINUTES = "60"

$validator = $null
$app = $null
$failures = 0

function Assert-True {
    param([bool]$Condition, [string]$Label)
    if ($Condition) {
        Write-Host "  ok   $Label"
    }
    else {
        Write-Host "  FAIL $Label"
        $script:failures += 1
    }
}

try {
    Write-Host "starting validator on 127.0.0.1:$ValidatorPort"
    $validator = Start-Process -FilePath $validatorExe -PassThru -WindowStyle Hidden
    Start-Sleep -Milliseconds 1200

    Write-Host "starting app on 127.0.0.1:$AppPort"
    $app = Start-Process -FilePath $appExe -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 2

    Write-Host "probing"
    try {
        $validatorHealth = Invoke-RestMethod "http://127.0.0.1:$ValidatorPort/healthz" -TimeoutSec 5
        Assert-True ($validatorHealth.status -eq "ok") "validator healthz reports ok"
        Assert-True ($validatorHealth.validator_pubkey_hex -eq $parts[1]) "validator healthz exposes the expected public key"
    }
    catch {
        Assert-True $false "validator healthz reachable ($($_.Exception.Message))"
    }

    try {
        $appHealth = Invoke-RestMethod "http://127.0.0.1:$AppPort/healthz" -TimeoutSec 5
        Assert-True ($appHealth.ok -eq $true) "app healthz returns ok"
        Assert-True ($appHealth.data.validator_gateway_configured -eq $true) "app reports a configured validator gateway"
    }
    catch {
        Assert-True $false "app healthz reachable ($($_.Exception.Message))"
    }

    try {
        $index = Invoke-WebRequest "http://127.0.0.1:$AppPort/" -TimeoutSec 5 -UseBasicParsing
        Assert-True ($index.StatusCode -eq 200) "dashboard index served"
        Assert-True ($index.Content -match "MedVault Go") "dashboard carries the product name"
        Assert-True ($index.Content -match "/vendor/tailwind-browser.min.js") "dashboard references the vendored Tailwind runtime"
        Assert-True ($index.Content -match "/vendor/alpine.min.js") "dashboard references the vendored Alpine runtime"
    }
    catch {
        Assert-True $false "dashboard index reachable ($($_.Exception.Message))"
    }

    try {
        $script = Invoke-WebRequest "http://127.0.0.1:$AppPort/js/app.js" -TimeoutSec 5 -UseBasicParsing
        Assert-True ($script.StatusCode -eq 200) "dashboard controller served"
        Assert-True ($script.Content -match "PKCS8_ED25519_PREFIX") "controller carries the browser dongle signing path"
    }
    catch {
        Assert-True $false "dashboard controller reachable ($($_.Exception.Message))"
    }

    try {
        $vendor = Invoke-WebRequest "http://127.0.0.1:$AppPort/vendor/alpine.min.js" -TimeoutSec 5 -UseBasicParsing
        Assert-True ($vendor.RawContentLength -gt 10000) "vendored Alpine asset served"
    }
    catch {
        Assert-True $false "vendored Alpine asset reachable ($($_.Exception.Message))"
    }

    try {
        # Windows PowerShell 5.1 has no -SkipHttpErrorCheck, so a 401 surfaces
        # as a terminating error whose response carries the status code.
        $unauthStatus = 0
        try {
            $unauth = Invoke-WebRequest "http://127.0.0.1:$AppPort/api/v1/patients" -TimeoutSec 5
            $unauthStatus = $unauth.StatusCode
        }
        catch {
            if ($null -ne $_.Exception.Response) {
                $unauthStatus = [int]$_.Exception.Response.StatusCode
            }
            else {
                throw
            }
        }
        Assert-True ($unauthStatus -eq 401) "unauthenticated patient list is rejected with 401"
    }
    catch {
        Assert-True $false "unauthenticated probe ran ($($_.Exception.Message))"
    }

    try {
        $csp = (Invoke-WebRequest "http://127.0.0.1:$AppPort/healthz" -TimeoutSec 5 -UseBasicParsing).Headers["Content-Security-Policy"]
        Assert-True ([bool]$csp) "security headers are present"
    }
    catch {
        Assert-True $false "security header probe ran ($($_.Exception.Message))"
    }
}
finally {
    foreach ($process in @($app, $validator)) {
        if ($null -ne $process -and -not $process.HasExited) {
            Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        }
    }
    Write-Host "logs in $dir"
}

if ($failures -gt 0) {
    Write-Host ""
    Write-Host "$failures check(s) failed"
    exit 1
}

Write-Host ""
Write-Host "smoke test passed"
