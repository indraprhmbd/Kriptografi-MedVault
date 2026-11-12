<#
.SYNOPSIS
Cross compiles the three MedVault Go binaries for the Linux VPS.

.DESCRIPTION
The VPS host runs Go 1.22, but the project requires a newer toolchain. Every
binary is therefore built on Windows with CGO disabled and copied over.

.EXAMPLE
./deploy/build.ps1
#>
[CmdletBinding()]
param(
    [string]$OutputDir = "dist"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$target = Join-Path $root $OutputDir
New-Item -ItemType Directory -Force -Path $target | Out-Null

Push-Location $root
try {
    $previous = @{
        CGO_ENABLED = $env:CGO_ENABLED
        GOOS        = $env:GOOS
        GOARCH      = $env:GOARCH
    }

    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"

    $targets = @(
        @{ Name = "medvault-app"; Package = "./cmd/app" },
        @{ Name = "medvault-validator"; Package = "./cmd/validator" },
        @{ Name = "medvault-medkeygen"; Package = "./cmd/medkeygen" },
        @{ Name = "medvault-validator-keygen"; Package = "./cmd/validator-keygen" }
    )

    foreach ($item in $targets) {
        $out = Join-Path $target $item.Name
        Write-Host "building $($item.Name)"
        go build -trimpath -ldflags "-s -w" -o $out $item.Package
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $($item.Package)"
        }
    }
}
finally {
    foreach ($key in $previous.Keys) {
        if ($null -eq $previous[$key]) {
            Remove-Item "Env:$key" -ErrorAction SilentlyContinue
        }
        else {
            Set-Item "Env:$key" $previous[$key]
        }
    }
    Pop-Location
}

Write-Host ""
Get-ChildItem $target | Where-Object { -not $_.PSIsContainer } |
    Select-Object Name, @{ Name = "MiB"; Expression = { [math]::Round($_.Length / 1MB, 2) } } |
    Format-Table -AutoSize