# Install fleet on native Windows (PowerShell).
#
#   Local (from a cloned repo, after ./build.sh):  .\install.ps1
#   Remote:  $env:RELEASE_BASE_URL="..."; .\install.ps1
#
# Overrides via env: BIN_DIR, CMD_NAME, RELEASE_BASE_URL.
# Tip: WSL users should use ./install.sh instead (it uses the Linux binary).
$ErrorActionPreference = "Stop"

$CmdName = if ($env:CMD_NAME) { $env:CMD_NAME } else { "fleet" }
$BinDir  = if ($env:BIN_DIR) { $env:BIN_DIR } else { "$env:LOCALAPPDATA\Programs\command-center" }

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$asset = "fleet_windows_$arch.exe"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
$dest = Join-Path $BinDir "$CmdName.exe"
$local = Join-Path $scriptDir "dist\$asset"

if (Test-Path $local) {
  Write-Host "installing from local build: dist\$asset"
  Copy-Item $local $dest -Force
} elseif ($env:RELEASE_BASE_URL) {
  $url = "$($env:RELEASE_BASE_URL)/$asset"
  Write-Host "downloading $url"
  Invoke-WebRequest -Uri $url -OutFile $dest
} else {
  throw "no local dist\$asset and RELEASE_BASE_URL not set. Run .\build.sh (via WSL/git-bash) or set RELEASE_BASE_URL."
}

Write-Host "installed $CmdName -> $dest"
Write-Host "ensure '$BinDir' is on your PATH (User environment variables)."
