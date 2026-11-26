<#
.SYNOPSIS
Opens an SSH Port Forwarding Tunnel to the live VPS instance.

.DESCRIPTION
Forwards remote VPS pod services to your local machine:
- 18095 -> 127.0.0.1:8095 (medvault-app)
- 18096 -> 127.0.0.1:8096 (medvault-validator)

Allows testing local scripts, running e2e tests, or browsing the live database
and validator directly on http://localhost:18095 without exposing extra VPS ports.

.PARAMETER Remote
SSH alias or host. Defaults to 'medvault-vps'.

.EXAMPLE
./deploy/tunnel.ps1
#>
[CmdletBinding()]
param(
    [string]$Remote = "medvault-vps"
)

Write-Host "=== MedVault VPS SSH Tunnel ===" -ForegroundColor Cyan
Write-Host "Opening tunnel from $Remote to localhost:" -ForegroundColor Yellow
Write-Host "  -> http://localhost:18095 => medvault-app" -ForegroundColor Green
Write-Host "  -> http://localhost:18096 => medvault-validator" -ForegroundColor Green
Write-Host ""
Write-Host "Press Ctrl+C to close the tunnel." -ForegroundColor Gray
Write-Host ""

ssh -N -L 18095:127.0.0.1:8095 -L 18096:127.0.0.1:8096 $Remote
