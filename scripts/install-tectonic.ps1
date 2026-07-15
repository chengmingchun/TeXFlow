$ErrorActionPreference = "Stop"

$version = "0.16.9"
$root = Split-Path -Parent $PSScriptRoot
$targetDir = Join-Path $root "tools\tectonic"
$target = Join-Path $targetDir "tectonic.exe"

if (Test-Path $target) {
    exit 0
}

New-Item -ItemType Directory -Force $targetDir | Out-Null
$archive = Join-Path $targetDir "tectonic.zip"
$url = "https://github.com/tectonic-typesetting/tectonic/releases/download/tectonic@$version/tectonic-$version-x86_64-pc-windows-msvc.zip"

Write-Host "Downloading Tectonic $version..."
Invoke-WebRequest -UseBasicParsing $url -OutFile $archive
try {
    Expand-Archive -LiteralPath $archive -DestinationPath $targetDir -Force
} finally {
    Remove-Item -LiteralPath $archive -Force -ErrorAction SilentlyContinue
}

if (-not (Test-Path $target)) {
    throw "Tectonic installation did not produce $target"
}
