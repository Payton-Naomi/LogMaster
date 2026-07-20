$ErrorActionPreference = 'Stop'
$goVersion = go version
if ($goVersion -notmatch 'go1\.26\.4\b') {
    throw "Go 1.26.4 is required; found: $goVersion"
}
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
New-Item -ItemType Directory -Force -Path bin | Out-Null
go build -trimpath -ldflags='-s -w' -o bin/agent.exe ./cmd/agent
go build -trimpath -ldflags='-s -w' -o bin/mockserver.exe ./cmd/mockserver
Write-Host 'Built bin/agent.exe and bin/mockserver.exe'
