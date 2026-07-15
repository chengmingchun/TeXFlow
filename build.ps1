$ErrorActionPreference = "Stop"

if (-not (Get-Command wails3 -ErrorAction SilentlyContinue)) {
    Write-Host "Installing Wails 3 CLI..."
    go install github.com/wailsapp/wails/v3/cmd/wails3@latest
}

wails3 build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Built: $PSScriptRoot\bin\ResumeStudio.exe"
