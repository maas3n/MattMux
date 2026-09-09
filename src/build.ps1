$ErrorActionPreference = 'Stop'

$required = [Version]'1.27.1'
$actualText = (& go version)
if ($actualText -notmatch 'go([0-9]+\.[0-9]+(?:\.[0-9]+)?)') {
    throw "Could not determine Go version. Install Go 1.27.1 or newer."
}
$actual = [Version]$Matches[1]
if ($actual -lt $required) {
    throw "MattMux release builds require Go 1.27.1 or newer. Found $actual."
}

$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'

go test ./...
go build -trimpath -buildvcs=false -ldflags "-s -w -H=windowsgui" -o ..\MattMux.exe .
Copy-Item .\MattMux.exe.manifest ..\MattMux.exe.manifest -Force
Get-FileHash ..\MattMux.exe -Algorithm SHA256
