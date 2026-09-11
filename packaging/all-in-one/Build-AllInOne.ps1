$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2.0

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$dist = Join-Path $repoRoot 'dist'
$iss = Join-Path $repoRoot 'packaging\installer\MattMux.iss'
$portable = Join-Path $dist 'MattMux-1.2.0-Portable.zip'
$thinSetup = Join-Path $dist 'MattMux-1.2.0-Thin-Setup.exe'
$output = Join-Path $dist 'MattMux-1.2.0-All-in-One.exe'
$assets = Join-Path $PSScriptRoot 'assets'

if (-not (Test-Path -LiteralPath $portable)) {
    throw "Portable package is missing: $portable"
}

$iscc = Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'
if (-not (Test-Path -LiteralPath $iscc)) {
    $cmd = Get-Command ISCC.exe -ErrorAction SilentlyContinue
    if ($null -eq $cmd) { throw 'Inno Setup compiler (ISCC.exe) was not found.' }
    $iscc = $cmd.Source
}

Remove-Item -LiteralPath $thinSetup -Force -ErrorAction SilentlyContinue
& $iscc '/DMyAppVersion=1.2.0' '/DThinSetup=1' $iss
if ($LASTEXITCODE -ne 0) { throw "Thin installer build failed with exit code $LASTEXITCODE" }
if (-not (Test-Path -LiteralPath $thinSetup)) { throw "Thin installer was not produced: $thinSetup" }

Remove-Item -LiteralPath $assets -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $assets | Out-Null
Copy-Item -LiteralPath $portable -Destination (Join-Path $assets 'MattMux-1.2.0-Portable.zip') -Force
Copy-Item -LiteralPath $thinSetup -Destination (Join-Path $assets 'MattMux-1.2.0-Thin-Setup.exe') -Force

Push-Location $PSScriptRoot
try {
    $env:CGO_ENABLED = '0'
    go build -trimpath -buildvcs=false -ldflags '-s -w -H=windowsgui' -o $output .\main.go
    if ($LASTEXITCODE -ne 0) { throw "All-in-one build failed with exit code $LASTEXITCODE" }
}
finally {
    Pop-Location
    Remove-Item -LiteralPath $assets -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $thinSetup -Force -ErrorAction SilentlyContinue
}

if (-not (Test-Path -LiteralPath $output)) { throw "All-in-one executable was not produced: $output" }
$item = Get-Item -LiteralPath $output
$hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $output).Hash.ToLowerInvariant()
Write-Host "All-in-one: $($item.FullName)"
Write-Host "Size:       $($item.Length) bytes"
Write-Host "SHA-256:    $hash"
