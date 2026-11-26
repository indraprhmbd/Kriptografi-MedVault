<#
.SYNOPSIS
Fast Direct Deploy to VPS without Git Push / GitHub Actions.

.DESCRIPTION
1. Cross-compiles static Linux amd64 binaries locally on Windows (medvault-app and medvault-validator).
2. SCPs the binaries directly to the remote VPS host (default: medvault-vps).
3. Executes container rebuild and pod restart on VPS via SSH.
4. Performs immediate live health check and runs smoke test.

.PARAMETER AppOnly
Only compile and deploy medvault-app (even faster when only touching web UI or app handlers).

.PARAMETER Remote
SSH alias or host. Defaults to 'medvault-vps'.

.PARAMETER SkipSmoke
Skip running smoke verification after deployment.

.EXAMPLE
./deploy/fast-ship.ps1
./deploy/fast-ship.ps1 -AppOnly
#>
[CmdletBinding()]
param(
    [switch]$AppOnly,
    [string]$Remote = "medvault-vps",
    [switch]$SkipSmoke
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null

$stopwatch = [System.Diagnostics.Stopwatch]::StartNew()

Write-Host "=== MedVault Fast Direct Deploy ===" -ForegroundColor Cyan

# 1. Local Cross-Compilation
Write-Host "[1/4] Cross-compiling static Linux binaries..." -ForegroundColor Yellow

$origCgo = $env:CGO_ENABLED
$origOs = $env:GOOS
$origArch = $env:GOARCH

try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"

    Push-Location $root

    Write-Host "  -> Building medvault-app..." -ForegroundColor Gray
    go build -trimpath -ldflags "-s -w" -o (Join-Path $dist "medvault-app") "./cmd/app"
    if ($LASTEXITCODE -ne 0) { throw "Build failed for medvault-app" }

    if (-not $AppOnly) {
        Write-Host "  -> Building medvault-validator..." -ForegroundColor Gray
        go build -trimpath -ldflags "-s -w" -o (Join-Path $dist "medvault-validator") "./cmd/validator"
        if ($LASTEXITCODE -ne 0) { throw "Build failed for medvault-validator" }
    }
} finally {
    Pop-Location
    $env:CGO_ENABLED = $origCgo
    $env:GOOS = $origOs
    $env:GOARCH = $origArch
}

# 2. Transfer binaries to VPS
Write-Host "[2/4] Uploading binaries via SCP to $Remote..." -ForegroundColor Yellow

scp -q (Join-Path $dist "medvault-app") "${Remote}:/home/medvault/bin/medvault-app.new"
if ($LASTEXITCODE -ne 0) { throw "SCP failed for medvault-app" }

if (-not $AppOnly) {
    scp -q (Join-Path $dist "medvault-validator") "${Remote}:/home/medvault/bin/medvault-validator.new"
    if ($LASTEXITCODE -ne 0) { throw "SCP failed for medvault-validator" }
}

# 3. Remote swap and restart
Write-Host "[3/4] Swapping binaries and recreating pod containers..." -ForegroundColor Yellow

$remoteScript = @"
set -euo pipefail

mv -f /home/medvault/bin/medvault-app.new /home/medvault/bin/medvault-app
chmod 0755 /home/medvault/bin/medvault-app

if [ -f /home/medvault/bin/medvault-validator.new ]; then
  mv -f /home/medvault/bin/medvault-validator.new /home/medvault/bin/medvault-validator
  chmod 0755 /home/medvault/bin/medvault-validator
fi

sudo -n /usr/bin/podman build -t medvault-app:latest -f /home/medvault/deploy/Containerfile.app /home/medvault/bin >/dev/null
if [ "$AppOnly" != "True" ]; then
  sudo -n /usr/bin/podman build -t medvault-validator:latest -f /home/medvault/deploy/Containerfile.validator /home/medvault/bin >/dev/null
fi

sudo -n /usr/bin/podman rm -f medvault-app >/dev/null 2>&1 || true

if [ "$AppOnly" != "True" ]; then
  sudo -n /usr/bin/podman rm -f medvault-validator >/dev/null 2>&1 || true
  sudo -n /usr/bin/podman run -d \
    --pod medvault-pod \
    --name medvault-validator \
    --env-file /home/medvault/.env \
    -v /home/medvault/storage:/data:Z \
    --health-cmd "wget -qO- http://127.0.0.1:8096/healthz >/dev/null || exit 1" \
    --health-interval 30s \
    --health-retries 3 \
    medvault-validator:latest >/dev/null
fi

sudo -n /usr/bin/podman run -d \
  --pod medvault-pod \
  --name medvault-app \
  --env-file /home/medvault/.env \
  -v /home/medvault/storage:/data:Z \
  --health-cmd "wget -qO- http://127.0.0.1:8095/healthz >/dev/null || exit 1" \
  --health-interval 30s \
  --health-retries 3 \
  medvault-app:latest >/dev/null

for i in {1..20}; do
  if curl -fsS http://127.0.0.1:8095/healthz >/dev/null 2>&1; then
    echo "HEALTHY"
    exit 0
  fi
  sleep 1
done

echo "HEALTH_TIMEOUT" >&2
sudo -n /usr/bin/podman logs --tail 20 medvault-app >&2
exit 1
"@

$tmpScript = [System.IO.Path]::GetTempFileName()
[System.IO.File]::WriteAllText($tmpScript, $remoteScript.Replace("`r`n", "`n"))
scp -q $tmpScript "${Remote}:/tmp/fast-deploy.sh"
Remove-Item $tmpScript

ssh $Remote "bash /tmp/fast-deploy.sh; rm -f /tmp/fast-deploy.sh"

if ($LASTEXITCODE -ne 0) {
    throw "Remote restart failed"
}

# 4. Live Smoke Verification
Write-Host "[4/4] Verifying production health..." -ForegroundColor Yellow
$healthResp = (curl.exe -fsS https://medvault.indraprhmbd.my.id/healthz | ConvertFrom-Json)
Write-Host "  -> Service: $($healthResp.data.service)" -ForegroundColor Green
Write-Host "  -> Validator Configured: $($healthResp.data.validator_gateway_configured)" -ForegroundColor Green

if (-not $SkipSmoke) {
    Write-Host "  -> Verifying assets & web routes on production..." -ForegroundColor Gray
    $assets = @("/healthz", "/login", "/js/app.js", "/vendor/alpine.min.js")
    foreach ($path in $assets) {
        $code = (curl.exe -s -o /dev/null -w "%{http_code}" "https://medvault.indraprhmbd.my.id$path")
        if ($code -ne "200") {
            throw "Route $path returned HTTP $code"
        }
        Write-Host "     [OK] $path" -ForegroundColor Green
    }
}

$stopwatch.Stop()
Write-Host ""
Write-Host "Successfully deployed to VPS in $($stopwatch.Elapsed.TotalSeconds.ToString("0.0"))s!" -ForegroundColor Cyan
