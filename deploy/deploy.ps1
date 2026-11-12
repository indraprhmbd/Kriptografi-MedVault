<#
.SYNOPSIS
Builds and ships MedVault Go to the medvault-vps host.

.DESCRIPTION
Cross compiles static Linux binaries, copies them with the Containerfiles, then
rebuilds images and recreates the Podman pod over SSH.

.PARAMETER SkipBuild
Reuse the existing dist directory.

.PARAMETER Remote
SSH alias of the target host. Defaults to medvault-vps.

.EXAMPLE
./deploy/deploy.ps1
#>
[CmdletBinding()]
param(
    [switch]$SkipBuild,
    [string]$Remote = "medvault-vps"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"

if (-not $SkipBuild) {
    & (Join-Path $PSScriptRoot "build.ps1")
    if ($LASTEXITCODE -ne 0) {
        throw "build failed"
    }
}

foreach ($binary in @("medvault-app", "medvault-validator", "medvault-medkeygen", "medvault-validator-keygen")) {
    if (-not (Test-Path (Join-Path $dist $binary))) {
        throw "$binary is missing from dist, run deploy/build.ps1 first"
    }
}

Write-Host "preparing remote directories"
ssh $Remote "mkdir -p /home/medvault/bin /home/medvault/deploy"

Write-Host "copying binaries"
scp (Join-Path $dist "medvault-app") "${Remote}:/home/medvault/bin/"
scp (Join-Path $dist "medvault-validator") "${Remote}:/home/medvault/bin/"
scp (Join-Path $dist "medvault-medkeygen") "${Remote}:/home/medvault/bin/"
scp (Join-Path $dist "medvault-validator-keygen") "${Remote}:/home/medvault/bin/"

Write-Host "copying deployment assets"
scp (Join-Path $PSScriptRoot "Containerfile.app") "${Remote}:/home/medvault/deploy/"
scp (Join-Path $PSScriptRoot "Containerfile.validator") "${Remote}:/home/medvault/deploy/"
scp (Join-Path $PSScriptRoot "remote-deploy.sh") "${Remote}:/home/medvault/deploy/"
scp (Join-Path $PSScriptRoot "init-env.sh") "${Remote}:/home/medvault/deploy/"

ssh $Remote "chmod 0755 /home/medvault/bin/medvault-* /home/medvault/deploy/*.sh"

Write-Host "running remote deployment"
ssh $Remote "bash /home/medvault/deploy/remote-deploy.sh"
if ($LASTEXITCODE -ne 0) {
    throw "remote deployment failed"
}

Write-Host ""
Write-Host "containers"
ssh $Remote "sudo -n /usr/bin/podman ps -a"
Write-Host ""
Write-Host "app health"
ssh $Remote "curl -fsS http://127.0.0.1:8095/healthz"
Write-Host ""
Write-Host "validator health"
ssh $Remote "curl -fsS http://127.0.0.1:8096/healthz"
Write-Host ""