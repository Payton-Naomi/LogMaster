[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$PortName,
    [Parameter(Mandatory)][string]$DeviceId,
    [TimeSpan]$Duration = ([TimeSpan]::FromHours(8)),
    [ValidateRange(1, 10000)][int]$LinesPerSecond = 20,
    [ValidateSet(9600, 19200, 38400, 57600, 115200, 230400, 460800, 921600)][int]$BaudRate = 115200,
    [Parameter(Mandatory)][string]$EventLog
)

$ErrorActionPreference = 'Stop'
$eventDirectory = Split-Path -Parent $EventLog
if ($eventDirectory) { New-Item -ItemType Directory -Force -Path $eventDirectory | Out-Null }

function Write-SimulatorEvent {
    param([string]$Type, [hashtable]$Data = @{})
    $record = [ordered]@{ timestamp = (Get-Date).ToUniversalTime().ToString('o'); type = $Type; session_id = $sessionId; device = $DeviceId; port = $PortName }
    foreach ($key in $Data.Keys) { $record[$key] = $Data[$key] }
    $record | ConvertTo-Json -Compress | Add-Content -LiteralPath $EventLog -Encoding utf8
}

$port = $null
$sessionId = [guid]::NewGuid().ToString('n')
$sequence = 0L
$bytes = 0L
$errors = 0
$started = Get-Date
$deadline = $started + $Duration
$nextWrite = [Diagnostics.Stopwatch]::StartNew()
$intervalMs = 1000.0 / $LinesPerSecond
$nextProgress = $started.AddSeconds(1)
Write-SimulatorEvent 'started' @{ lines_per_second = $LinesPerSecond; duration_seconds = [int]$Duration.TotalSeconds }

try {
    while ((Get-Date) -lt $deadline) {
        try {
            if (-not $port -or -not $port.IsOpen) {
                if ($port) { $port.Dispose() }
                $port = [System.IO.Ports.SerialPort]::new($PortName, $BaudRate, 'None', 8, 'One')
                $port.NewLine = "`r`n"
                $port.WriteTimeout = 1000
                $port.Open()
                Write-SimulatorEvent 'connected'
            }

            $sequence++
            $level = if ($sequence % 997 -eq 0) { 'ERROR' } elseif ($sequence % 101 -eq 0) { 'WARN' } else { 'INFO' }
            $payload = "{0:o} device={1} sequence={2} level={3} module=stability message=serial-simulator" -f (Get-Date).ToUniversalTime(), $DeviceId, $sequence, $level
            $port.WriteLine($payload)
            $bytes += [Text.Encoding]::UTF8.GetByteCount($payload + "`r`n")

            $now = Get-Date
            if ($now -ge $nextProgress) {
                Write-SimulatorEvent 'progress' @{ sequence = $sequence; bytes = $bytes; errors = $errors }
                $nextProgress = $now.AddSeconds(1)
            }

            $targetMs = $sequence * $intervalMs
            $delay = [int][Math]::Floor($targetMs - $nextWrite.Elapsed.TotalMilliseconds)
            if ($delay -gt 0) { Start-Sleep -Milliseconds ([Math]::Min($delay, 1000)) }
        } catch {
            $errors++
            Write-SimulatorEvent 'write_error' @{ sequence = $sequence; error = $_.Exception.Message }
            if ($port) { try { $port.Dispose() } catch {} }
            $port = $null
            Start-Sleep -Seconds 1
        }
    }
} finally {
    if ($port) { try { $port.Close(); $port.Dispose() } catch {} }
    Write-SimulatorEvent 'completed' @{ sequence = $sequence; bytes = $bytes; errors = $errors; elapsed_seconds = [int]((Get-Date) - $started).TotalSeconds }
}
