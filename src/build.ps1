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

# main is development source. Historical source still carries the last Windows
# version constant for reproducibility, but ordinary main builds must not claim
# to be that published release. A tagged release workflow injects its tag before
# this script runs; in that case the historical marker is already gone and this
# step leaves the release version untouched.
$windowsSource = Join-Path $PSScriptRoot 'app_windows.go'
$sourceText = Get-Content -LiteralPath $windowsSource -Raw
$historicalMarker = 'appVersion = "1.2.0"'
if ($sourceText.Contains($historicalMarker)) {
    $sourceText = $sourceText.Replace($historicalMarker, 'appVersion = "dev"')
    Set-Content -LiteralPath $windowsSource -Value $sourceText -Encoding UTF8 -NoNewline
}

$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'

go test ./...
go build -trimpath -buildvcs=false -ldflags "-s -w -H=windowsgui" -o ..\MattMux.exe .
Copy-Item .\MattMux.exe.manifest ..\MattMux.exe.manifest -Force
Get-FileHash ..\MattMux.exe -Algorithm SHA256
