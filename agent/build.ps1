[CmdletBinding()]
param(
    [switch]$SkipTests,
    [switch]$SkipDependencyInstall,
    [switch]$RequireWebView2
)

$ErrorActionPreference = 'Stop'
$agentRoot = $PSScriptRoot
$desktopRoot = Join-Path $agentRoot 'desktop'
$desktopFrontend = Join-Path $desktopRoot 'frontend'
$outputRoot = Join-Path $agentRoot 'bin'

function Assert-Command {
    param([Parameter(Mandatory)][string]$Name)
    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if (-not $command) {
        throw "Required command '$Name' was not found in PATH. See docs/deployment-operations.md."
    }
    return $command
}

function Invoke-Checked {
    param([Parameter(Mandatory)][scriptblock]$Command, [Parameter(Mandatory)][string]$Description)
    & $Command
    if ($LASTEXITCODE -ne 0) { throw "$Description failed with exit code $LASTEXITCODE" }
}

function Test-WebView2Runtime {
    $executables = @()
    if (${env:ProgramFiles(x86)}) { $executables += Join-Path ${env:ProgramFiles(x86)} 'Microsoft\EdgeWebView\Application\*\msedgewebview2.exe' }
    if ($env:LOCALAPPDATA) { $executables += Join-Path $env:LOCALAPPDATA 'Microsoft\EdgeWebView\Application\*\msedgewebview2.exe' }
    if (-not $executables) { return $false }
    return [bool](Get-Item -Path $executables -ErrorAction SilentlyContinue | Select-Object -First 1)
}

$go = Assert-Command 'go'
$node = Assert-Command 'node'
$npm = Assert-Command 'npm.cmd'
$wails = Get-Command 'wails' -ErrorAction SilentlyContinue
$goVersion = & $go.Source version
if ($goVersion -notmatch 'go1\.26\.4\b') { throw "Go 1.26.4 is required; found: $goVersion" }
if (-not (Test-Path -LiteralPath (Join-Path $desktopRoot 'wails.json'))) {
    throw "Wails project not found: $desktopRoot\wails.json"
}
if (-not (Test-Path -LiteralPath (Join-Path $desktopFrontend 'package-lock.json'))) {
    throw "Frontend lock file not found: $desktopFrontend\package-lock.json"
}

if (-not (Test-WebView2Runtime)) {
    $message = 'Microsoft Edge WebView2 Runtime was not detected. The build can succeed, but LogCollector.exe requires the Evergreen Runtime on the target machine.'
    if ($RequireWebView2) { throw $message }
    Write-Warning $message
}

Write-Host "Go: $goVersion"
Write-Host "Node: $(& $node.Source --version)"
if ($wails) {
    Write-Host "Wails: $(& $wails.Source version)"
} else {
    Write-Host 'Wails: module runner v2.13.0 (global CLI not installed)'
}

if (-not $SkipTests) {
    Push-Location $agentRoot
    try {
        Invoke-Checked { & $go.Source test ./... } 'Go tests'
        Invoke-Checked { & $go.Source vet ./... } 'Go vet'
    } finally { Pop-Location }
}

Push-Location $desktopFrontend
try {
    if (-not $SkipDependencyInstall) {
        Invoke-Checked { & $npm.Source ci } 'Frontend dependency installation'
    } elseif (-not (Test-Path -LiteralPath 'node_modules')) {
        throw 'SkipDependencyInstall was specified, but desktop/frontend/node_modules does not exist.'
    }
    Invoke-Checked { & $npm.Source run build } 'Desktop frontend build'
} finally { Pop-Location }

Push-Location $desktopRoot
try {
    if ($wails) {
        Invoke-Checked { & $wails.Source build -clean -platform windows/amd64 -o LogCollector.exe } 'Wails desktop build'
    } else {
        Invoke-Checked { & $go.Source run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0 build -clean -platform windows/amd64 -o LogCollector.exe } 'Wails desktop build'
    }
} finally { Pop-Location }

$wailsArtifact = Join-Path $desktopRoot 'build\bin\LogCollector.exe'
if (-not (Test-Path -LiteralPath $wailsArtifact)) {
    throw "Wails reported success, but the expected artifact is missing: $wailsArtifact"
}
New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
$releaseArtifact = Join-Path $outputRoot 'LogCollector.exe'
Copy-Item -LiteralPath $wailsArtifact -Destination $releaseArtifact -Force
$artifact = Get-Item -LiteralPath $releaseArtifact
$sha256 = (Get-FileHash -LiteralPath $releaseArtifact -Algorithm SHA256).Hash.ToLowerInvariant()
Write-Host "Built: $($artifact.FullName)"
Write-Host "Size: $($artifact.Length) bytes"
Write-Host "SHA-256: $sha256"
Write-Host 'Target runtime requirement: Microsoft Edge WebView2 Evergreen Runtime.'
