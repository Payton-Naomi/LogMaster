$ErrorActionPreference = 'Stop'
$agentRoot = Split-Path -Parent $PSScriptRoot
$required = @(
    'build.ps1', 'config_template.yaml', 'tools\Start-SerialSimulator.ps1', 'tools\Invoke-StabilityTest.ps1',
    'docs\user-guide.md', 'docs\deployment-operations.md', 'docs\troubleshooting.md',
    'docs\known-limitations.md', 'docs\api-integration.md', 'tests\test-report-template.md'
)

foreach ($relativePath in $required) {
    $path = Join-Path $agentRoot $relativePath
    if (-not (Test-Path -LiteralPath $path)) { throw "Missing delivery artifact: $relativePath" }
}

$errors = @()
foreach ($script in @('build.ps1', 'tools\Start-SerialSimulator.ps1', 'tools\Invoke-StabilityTest.ps1')) {
    $tokens = $null
    $parseErrors = $null
    [Management.Automation.Language.Parser]::ParseFile((Join-Path $agentRoot $script), [ref]$tokens, [ref]$parseErrors) | Out-Null
    $errors += $parseErrors
}
if ($errors.Count) { throw ($errors | ForEach-Object Message | Out-String) }

$config = Get-Content -Raw -LiteralPath (Join-Path $agentRoot 'config_template.yaml')
$deviceCount = ([regex]::Matches($config, 'device_sn:')).Count
if ($deviceCount -ne 8) { throw "config_template.yaml must contain eight device slots; found $deviceCount" }
if ($config -notmatch 'DUT-04' -or $config -notmatch 'DUT-08-RESERVED') { throw 'The four active slots and four reserved slots are not documented.' }

Write-Host 'Delivery artifact checks passed.'
