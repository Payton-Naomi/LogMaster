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
$releaseRoot = Join-Path $agentRoot 'release'
$wailsConfigPath = Join-Path $desktopRoot 'wails.json'

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
if (-not (Test-Path -LiteralPath $wailsConfigPath)) {
    throw "Wails project not found: $desktopRoot\wails.json"
}
if (-not (Test-Path -LiteralPath (Join-Path $desktopFrontend 'package-lock.json'))) {
    throw "Frontend lock file not found: $desktopFrontend\package-lock.json"
}
$wailsConfig = Get-Content -LiteralPath $wailsConfigPath -Raw -Encoding UTF8 | ConvertFrom-Json
$productVersion = [string]$wailsConfig.info.productVersion
if ($productVersion -notmatch '^\d+\.\d+\.\d+$') {
    throw "Invalid product version in ${wailsConfigPath}: '$productVersion'"
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
    # Wails starts the Go application to inspect exported bindings. Isolate that
    # short-lived process from an operator's active portable/user SQLite file.
    $previousBindingsRoot = $env:LOGMASTER_BUILD_BINDINGS_ROOT
    $bindingsRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("logmaster-wails-bindings-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force -Path $bindingsRoot | Out-Null
    $env:LOGMASTER_BUILD_BINDINGS_ROOT = $bindingsRoot
    if ($wails) {
        Invoke-Checked { & $wails.Source build -clean -platform windows/amd64 -o LogCollector.exe } 'Wails desktop build'
    } else {
        Invoke-Checked { & $go.Source run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0 build -clean -platform windows/amd64 -o LogCollector.exe } 'Wails desktop build'
    }
} finally {
    if ($null -eq $previousBindingsRoot) { Remove-Item Env:LOGMASTER_BUILD_BINDINGS_ROOT -ErrorAction SilentlyContinue } else { $env:LOGMASTER_BUILD_BINDINGS_ROOT = $previousBindingsRoot }
    if ($bindingsRoot -and (Test-Path -LiteralPath $bindingsRoot)) { Remove-Item -LiteralPath $bindingsRoot -Recurse -Force }
    Pop-Location
}

$wailsArtifact = Join-Path $desktopRoot 'build\bin\LogCollector.exe'
if (-not (Test-Path -LiteralPath $wailsArtifact)) {
    throw "Wails reported success, but the expected artifact is missing: $wailsArtifact"
}
New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
$releaseArtifact = Join-Path $outputRoot "LogCollector-$productVersion.exe"
Copy-Item -LiteralPath $wailsArtifact -Destination $releaseArtifact -Force
$generatedConfig = Join-Path ([System.IO.Path]::GetTempPath()) ("logmaster-config-" + [guid]::NewGuid().ToString('N'))
try {
    # The packaged binary is a Windows GUI executable; invoking it from a
    # non-interactive build shell does not reliably preserve command arguments.
    # Run the same Go entry point's export branch instead.
    Push-Location $desktopRoot
    try {
        Invoke-Checked { & $go.Source run . --export-config $generatedConfig } 'Default configuration export'
    } finally { Pop-Location }
    $portableConfig = Join-Path $outputRoot 'config'
    New-Item -ItemType Directory -Force -Path $portableConfig | Out-Null
    Get-ChildItem -LiteralPath $generatedConfig -File | ForEach-Object {
        $destination = Join-Path $portableConfig $_.Name
        if (-not (Test-Path -LiteralPath $destination)) {
            Copy-Item -LiteralPath $_.FullName -Destination $destination
        }
    }
} finally {
    if (Test-Path -LiteralPath $generatedConfig) { Remove-Item -LiteralPath $generatedConfig -Recurse -Force }
}
$artifact = Get-Item -LiteralPath $releaseArtifact
$sha256 = (Get-FileHash -LiteralPath $releaseArtifact -Algorithm SHA256).Hash.ToLowerInvariant()
$packageDirectory = Join-Path $releaseRoot "LogCollector-$productVersion"
if (Test-Path -LiteralPath $packageDirectory) { Remove-Item -LiteralPath $packageDirectory -Recurse -Force }
New-Item -ItemType Directory -Force -Path $packageDirectory | Out-Null
Copy-Item -LiteralPath $releaseArtifact -Destination (Join-Path $packageDirectory $releaseArtifact.Name) -Force
$packageConfig = Join-Path $packageDirectory 'config'
New-Item -ItemType Directory -Force -Path $packageConfig | Out-Null
Get-ChildItem -LiteralPath (Join-Path $outputRoot 'config') -File | Copy-Item -Destination $packageConfig -Force
$packageReadme = Join-Path $packageDirectory '交付说明.txt'
$packageReadmeContent = @"
LogMaster 采集端 $productVersion

1. 双击 LogCollector-$productVersion.exe 启动。
2. 程序会自动读取同级 config 目录中的设置与项目、任务、关键字配置。
3. 服务迁移时，请在程序关闭后修改 config\settings-config.json 的 backendUrl，再重新启动。
4. 本地配置可直接编辑 YAML；在程序内修改会自动同步写入对应文件。
"@
Set-Content -LiteralPath $packageReadme -Value $packageReadmeContent -Encoding UTF8
$zipPath = Join-Path $releaseRoot "LogCollector-$productVersion.zip"
if (Test-Path -LiteralPath $zipPath) { Remove-Item -LiteralPath $zipPath -Force }
Compress-Archive -LiteralPath $packageDirectory -DestinationPath $zipPath -Force
$packageFiles = @(
    (Join-Path $outputRoot 'config\settings-config.json'),
    (Join-Path $outputRoot 'config\project-config.yaml'),
    (Join-Path $outputRoot 'config\task-config.yaml'),
    (Join-Path $outputRoot 'config\keyword-config.yaml'),
    (Join-Path $packageDirectory $releaseArtifact.Name),
    $zipPath
)
foreach ($file in $packageFiles) {
    if (-not (Test-Path -LiteralPath $file) -or (Get-Item -LiteralPath $file).Length -le 0) {
        throw "Delivery verification failed: $file"
    }
}
Write-Host "Built: $($artifact.FullName)"
Write-Host "Size: $($artifact.Length) bytes"
Write-Host "SHA-256: $sha256"
Write-Host "Delivery directory: $packageDirectory"
Write-Host "Delivery ZIP: $zipPath"
Write-Host 'Target runtime requirement: Microsoft Edge WebView2 Evergreen Runtime.'
