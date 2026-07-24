[CmdletBinding()]
param(
    [TimeSpan]$Duration = ([TimeSpan]::FromHours(8)),
    [TimeSpan]$SampleInterval = ([TimeSpan]::FromSeconds(10)),
    [TimeSpan]$FaultDuration = ([TimeSpan]::FromSeconds(30)),
    [string[]]$WriterPorts = @('COM11', 'COM13', 'COM15', 'COM17'),
    [string[]]$DeviceIds = @('DUT-01', 'DUT-02', 'DUT-03', 'DUT-04'),
    [ValidateRange(1, 10000)][int]$LinesPerSecond = 20,
    [string]$CollectorExecutable = '',
    [string[]]$CollectorArguments = @(),
    [string]$MetricsUrl = 'http://127.0.0.1:9000/metrics',
    [string]$SpoolDirectory = '',
    [ValidateSet('serial', 'network', 'restart', 'disk')][string[]]$Faults = @('serial', 'restart'),
    [string]$MockServerExecutable = '',
    [string[]]$MockServerArguments = @(),
    [switch]$AllowDiskPressure,
    [ValidateRange(0, 1073741824)][long]$DiskPressureBytes = 0,
    [double]$MaximumMemoryGrowthPercent = 25,
    [switch]$SkipCollectorMetrics,
    [string]$OutputRoot = ''
)

$ErrorActionPreference = 'Stop'
$RequireCollectorMetrics = -not $SkipCollectorMetrics
if (-not $CollectorExecutable) { $CollectorExecutable = Join-Path $PSScriptRoot '..\bin\LogCollector.exe' }
if (-not $SpoolDirectory) { $SpoolDirectory = Join-Path $PSScriptRoot '..\data\spool' }
if (-not $OutputRoot) { $OutputRoot = Join-Path $PSScriptRoot '..\tests\results' }
if ($WriterPorts.Count -ne 4 -or $DeviceIds.Count -ne 4) { throw 'Exactly four writer ports and four device IDs are required.' }
if ($Duration.TotalSeconds -lt 10) { throw 'Duration must be at least 10 seconds.' }
if ($SampleInterval.TotalSeconds -lt 1) { throw 'SampleInterval must be at least one second.' }
if (-not (Test-Path -LiteralPath $CollectorExecutable)) { throw "Collector executable not found: $CollectorExecutable" }
if ($Faults -contains 'network' -and (-not $MockServerExecutable -or -not (Test-Path -LiteralPath $MockServerExecutable))) {
    throw 'Network fault injection requires an existing -MockServerExecutable that the harness can stop and restart.'
}

$runId = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
$runDirectory = Join-Path $OutputRoot $runId
New-Item -ItemType Directory -Force -Path $runDirectory | Out-Null
$eventsPath = Join-Path $runDirectory 'events.ndjson'
$samplesPath = Join-Path $runDirectory 'samples.ndjson'
$reportPath = Join-Path $runDirectory 'report.md'
$collectorStdout = Join-Path $runDirectory 'collector.stdout.log'
$collectorStderr = Join-Path $runDirectory 'collector.stderr.log'
$simulatorScript = Join-Path $PSScriptRoot 'Start-SerialSimulator.ps1'
$powershell = Get-Command powershell.exe -ErrorAction Stop
$simulators = @{}
$collector = $null
$mockServer = $null
$pressureFile = Join-Path $runDirectory 'disk-pressure.bin'
$samples = [Collections.Generic.List[object]]::new()
$unexpectedCollectorExit = $false
$started = Get-Date
$deadline = $started + $Duration
$faultSchedule = @{ serial = 0.20; network = 0.40; restart = 0.60; disk = 0.75 }
$faultState = @{}

function Write-RunEvent {
    param([string]$Type, [hashtable]$Data = @{})
    $record = [ordered]@{ timestamp = (Get-Date).ToUniversalTime().ToString('o'); type = $Type }
    foreach ($key in $Data.Keys) { $record[$key] = $Data[$key] }
    $record | ConvertTo-Json -Compress | Add-Content -LiteralPath $eventsPath -Encoding utf8
}

function Start-Simulator {
    param([int]$Index)
    $eventLog = Join-Path $runDirectory ("simulator-{0}.ndjson" -f ($Index + 1))
    $arguments = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $simulatorScript, '-PortName', $WriterPorts[$Index], '-DeviceId', $DeviceIds[$Index], '-Duration', $Duration.ToString(), '-LinesPerSecond', $LinesPerSecond, '-EventLog', $eventLog)
    $simulators[$Index] = Start-Process -FilePath $powershell.Source -ArgumentList $arguments -PassThru -WindowStyle Hidden
    Write-RunEvent 'simulator_started' @{ channel = $Index + 1; pid = $simulators[$Index].Id; port = $WriterPorts[$Index] }
}

function Start-Collector {
    try {
        $script:collector = Start-Process -FilePath $CollectorExecutable -ArgumentList $CollectorArguments -PassThru -RedirectStandardOutput $collectorStdout -RedirectStandardError $collectorStderr
    } catch {
        "Output capture unavailable: $($_.Exception.Message)" | Add-Content -LiteralPath $collectorStderr -Encoding utf8
        $script:collector = Start-Process -FilePath $CollectorExecutable -ArgumentList $CollectorArguments -PassThru
        Write-RunEvent 'collector_output_capture_disabled' @{ error = $_.Exception.Message }
    }
    Write-RunEvent 'collector_started' @{ pid = $script:collector.Id }
}

function Start-MockServer {
    if (-not $MockServerExecutable) { return }
    $script:mockServer = Start-Process -FilePath $MockServerExecutable -ArgumentList $MockServerArguments -PassThru -WindowStyle Hidden
    Write-RunEvent 'mock_server_started' @{ pid = $script:mockServer.Id }
}

function Stop-OwnedProcess {
    param($Process, [string]$Reason)
    if ($Process -and -not $Process.HasExited) {
        Stop-Process -Id $Process.Id -Force
        $Process.WaitForExit(10000) | Out-Null
        Write-RunEvent 'process_stopped' @{ pid = $Process.Id; reason = $Reason }
    }
}

function Get-MetricValue {
    param([string]$Text, [string]$Name)
    $match = [regex]::Match($Text, "(?m)^$([regex]::Escape($Name))(?:\{[^}]*\})?\s+([0-9.eE+-]+)$")
    if ($match.Success) { return [double]$match.Groups[1].Value }
    return $null
}

function Get-DeviceMetricValue {
    param([string]$Text, [string]$DeviceId)
    $lines = $Text -split "`n" | Where-Object { $_ -match '^logmaster_serial_rx_bytes_total\{' -and $_ -match ('device_sn=' + [regex]::Escape(('"' + $DeviceId + '"'))) }
    $total = 0.0
    foreach ($line in $lines) {
        if ($line -match '\s([0-9.eE+-]+)\s*$') { $total += [double]$Matches[1] }
    }
    return $total
}

function Invoke-DiskPressure {
    if (-not $AllowDiskPressure -or $DiskPressureBytes -le 0) {
        Write-RunEvent 'fault_skipped' @{ fault = 'disk'; reason = 'Use -AllowDiskPressure and a positive -DiskPressureBytes to authorize bounded allocation.' }
        return
    }
    $drive = Get-PSDrive -Name ([IO.Path]::GetPathRoot($runDirectory).TrimEnd('\').TrimEnd(':'))
    if ($drive.Free -lt ($DiskPressureBytes + 2GB)) { throw 'Insufficient free space margin for the requested disk-pressure file.' }
    $buffer = New-Object byte[] (1MB)
    $stream = [IO.File]::Open($pressureFile, 'CreateNew', 'Write', 'None')
    try {
        $remaining = $DiskPressureBytes
        while ($remaining -gt 0) {
            $count = [int][Math]::Min($buffer.Length, $remaining)
            $stream.Write($buffer, 0, $count)
            $remaining -= $count
        }
        $stream.Flush($true)
    } finally { $stream.Dispose() }
    Write-RunEvent 'fault_started' @{ fault = 'disk'; bytes = $DiskPressureBytes }
}

Write-RunEvent 'run_started' @{ run_id = $runId; duration_seconds = [int]$Duration.TotalSeconds; lines_per_second = $LinesPerSecond; faults = $Faults }
$runError = $null
try {
    Start-MockServer
    0..3 | ForEach-Object { Start-Simulator $_ }
    Start-Collector

    while ((Get-Date) -lt $deadline) {
        $now = Get-Date
        $elapsed = $now - $started
        $fraction = $elapsed.TotalSeconds / $Duration.TotalSeconds

        foreach ($fault in $Faults) {
            if (-not $faultState.ContainsKey($fault) -and $fraction -ge $faultSchedule[$fault]) {
                $faultState[$fault] = @{ started = $now; restored = $false }
                switch ($fault) {
                    'serial' { Stop-OwnedProcess $simulators[0] 'serial fault injection'; Write-RunEvent 'fault_started' @{ fault = 'serial'; channel = 1 } }
                    'network' {
                        if ($mockServer) { Stop-OwnedProcess $mockServer 'network fault injection'; Write-RunEvent 'fault_started' @{ fault = 'network' } }
                        else { $faultState[$fault].restored = $true; Write-RunEvent 'fault_skipped' @{ fault = 'network'; reason = 'MockServerExecutable was not provided.' } }
                    }
                    'restart' { Stop-OwnedProcess $collector 'forced restart injection'; Write-RunEvent 'fault_started' @{ fault = 'restart' } }
                    'disk' { Invoke-DiskPressure }
                }
            }
        }

        foreach ($fault in @($faultState.Keys)) {
            $state = $faultState[$fault]
            if (-not $state.restored -and ($now - $state.started) -ge $FaultDuration) {
                switch ($fault) {
                    'serial' { Start-Simulator 0 }
                    'network' { Start-MockServer }
                    'restart' { Start-Collector }
                    'disk' { if (Test-Path -LiteralPath $pressureFile) { Remove-Item -LiteralPath $pressureFile -Force } }
                }
                $state.restored = $true
                Write-RunEvent 'fault_restored' @{ fault = $fault }
            }
        }

        if ($collector -and $collector.HasExited -and (-not $faultState.ContainsKey('restart') -or $faultState['restart'].restored)) {
            $unexpectedCollectorExit = $true
        }

        $metricsText = ''
        try { $metricsText = (Invoke-WebRequest -UseBasicParsing -Uri $MetricsUrl -TimeoutSec 3).Content } catch {}
        $spoolFiles = @(Get-ChildItem -LiteralPath $SpoolDirectory -Recurse -File -ErrorAction SilentlyContinue)
        $collector.Refresh()
        $sample = [ordered]@{
            timestamp = $now.ToUniversalTime().ToString('o')
            elapsed_seconds = [int]$elapsed.TotalSeconds
            collector_alive = [bool](-not $collector.HasExited)
            collector_working_set_bytes = if ($collector.HasExited) { 0 } else { $collector.WorkingSet64 }
            collector_cpu_seconds = if ($collector.HasExited) { 0 } else { $collector.TotalProcessorTime.TotalSeconds }
            spool_files = $spoolFiles.Count
            spool_bytes = [long](($spoolFiles | Measure-Object Length -Sum).Sum)
            reported_memory_bytes = Get-MetricValue $metricsText 'logmaster_process_memory_bytes'
            disk_free_bytes = Get-MetricValue $metricsText 'logmaster_disk_free_bytes'
            device_rx_bytes = [ordered]@{
                $DeviceIds[0] = Get-DeviceMetricValue $metricsText $DeviceIds[0]
                $DeviceIds[1] = Get-DeviceMetricValue $metricsText $DeviceIds[1]
                $DeviceIds[2] = Get-DeviceMetricValue $metricsText $DeviceIds[2]
                $DeviceIds[3] = Get-DeviceMetricValue $metricsText $DeviceIds[3]
            }
        }
        $sample | ConvertTo-Json -Compress | Add-Content -LiteralPath $samplesPath -Encoding utf8
        $samples.Add([pscustomobject]$sample)
        Start-Sleep -Milliseconds ([int]$SampleInterval.TotalMilliseconds)
    }
} catch {
    $runError = $_
    Write-RunEvent 'run_error' @{ error = $_.Exception.Message }
} finally {
    foreach ($process in @($simulators.Values)) { Stop-OwnedProcess $process 'test completed' }
    Stop-OwnedProcess $collector 'test completed'
    Stop-OwnedProcess $mockServer 'test completed'
    if (Test-Path -LiteralPath $pressureFile) { Remove-Item -LiteralPath $pressureFile -Force }

    $producerRows = @()
    for ($i = 0; $i -lt 4; $i++) {
        $eventLog = Join-Path $runDirectory ("simulator-{0}.ndjson" -f ($i + 1))
        $records = if (Test-Path -LiteralPath $eventLog) { @(Get-Content -LiteralPath $eventLog | ForEach-Object { $_ | ConvertFrom-Json }) } else { @() }
        $sessionEnds = @($records | Where-Object { $_.sequence -ne $null -and $_.bytes -ne $null } | Group-Object session_id | ForEach-Object { $_.Group | Select-Object -Last 1 })
        $lines = [long](($sessionEnds | Measure-Object sequence -Sum).Sum)
        $bytes = [long](($sessionEnds | Measure-Object bytes -Sum).Sum)
        $producerRows += [pscustomobject]@{ channel = $i + 1; device = $DeviceIds[$i]; port = $WriterPorts[$i]; lines = $lines; bytes = $bytes; lines_per_second = [Math]::Round($lines / [Math]::Max(1, $Duration.TotalSeconds), 2); average_line_bytes = if ($lines) { [Math]::Round($bytes / $lines, 2) } else { 0 } }
    }
    $memorySamples = @($samples | Where-Object { $_.collector_working_set_bytes -gt 0 })
    $memoryGrowth = if ($memorySamples.Count -ge 2 -and $memorySamples[0].collector_working_set_bytes -gt 0) { [Math]::Round((($memorySamples[-1].collector_working_set_bytes - $memorySamples[0].collector_working_set_bytes) * 100.0 / $memorySamples[0].collector_working_set_bytes), 2) } else { 0 }
    $elapsedSeconds = [int]((Get-Date) - $started).TotalSeconds
    $lastSample = $samples | Select-Object -Last 1
    $collectorChannelsWithData = if ($lastSample) { @($DeviceIds | Where-Object { [double]$lastSample.device_rx_bytes.$_ -gt 0 }).Count } else { 0 }
    $metricsPassed = -not $RequireCollectorMetrics -or $collectorChannelsWithData -eq 4
    $passed = -not $runError -and -not $unexpectedCollectorExit -and ($producerRows | Where-Object lines -le 0).Count -eq 0 -and $memoryGrowth -le $MaximumMemoryGrowthPercent -and $metricsPassed
    $lines = @(
        '# LogMaster stability test report', '',
        "- Run ID: $runId", "- Result: **$(if ($passed) { 'PASS' } else { 'FAIL' })**", "- Requested duration: $Duration", "- Recorded duration: $elapsedSeconds seconds", "- Faults: $($Faults -join ', ')", "- Sample interval: $SampleInterval", "- Memory growth: $memoryGrowth% (limit $MaximumMemoryGrowthPercent%)", "- Collector channels with received-byte evidence: $collectorChannelsWithData/4", "- Unexpected collector exit: $unexpectedCollectorExit", "- Error: $(if ($runError) { $runError.Exception.Message } else { 'none' })", '',
        '## Per-channel throughput', '', '| Channel | Device | Writer port | Lines | Bytes | Lines/s | Average line bytes |', '| --- | --- | --- | ---: | ---: | ---: | ---: |'
    )
    foreach ($row in $producerRows) { $lines += "| $($row.channel) | $($row.device) | $($row.port) | $($row.lines) | $($row.bytes) | $($row.lines_per_second) | $($row.average_line_bytes) |" }
    $lines += @('', '## Evidence', '', '- `events.ndjson`: lifecycle and fault injection events.', '- `samples.ndjson`: process, spool, memory and disk samples.', '- `simulator-N.ndjson`: generated line and reconnect evidence for each channel.', '- `collector.stdout.log` and `collector.stderr.log`: collector process output.', '', '> A PASS proves that the configured harness assertions passed. Physical-device, Win10/Win11 and WebView2 compatibility still require the manual sign-off fields in the test report template.')
    Set-Content -LiteralPath $reportPath -Value $lines -Encoding utf8
    Write-RunEvent 'run_completed' @{ passed = $passed; report = $reportPath }
}

Write-Host "Evidence: $runDirectory"
if ($runError) { throw $runError }
if (-not $passed) { throw "Stability assertions failed. See $reportPath" }
