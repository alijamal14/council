#!/usr/bin/env pwsh

# 💡 AI Council Orchestrator Wrapper (PowerShell)
# This script is a wrapper for the native Go binary.

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$Binary = Join-Path $ScriptDir "council.exe"

# On non-Windows, the binary won't have .exe
if (-not (Test-Path $Binary)) {
    $Binary = Join-Path $ScriptDir "council"
}

if (-not (Test-Path $Binary)) {
    Write-Host "Building Council binary..." -ForegroundColor Yellow
    Push-Location $ScriptDir
    go build -o council.exe .
    Pop-Location
    $Binary = Join-Path $ScriptDir "council.exe"
}

& $Binary @args
