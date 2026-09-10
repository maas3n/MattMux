$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2.0

# MattMux 1.2.0 fully-portable/offline bundle builder.
# Downloads the exact third-party builds expected by MattMux, verifies SHA-256,
# stages them under MattMuxData\tools, and creates a ZIP for use on other PCs.

$AppVersion = '1.2.0'
$FfmpegReleaseTag = 'autobuild-2026-09-08-23-15'
$FfmpegAssetName = 'ffmpeg-N-126479-g08cd8df29d-win64-gpl-shared.zip'
$FfmpegManifestSha256 = 'f64be162403094773397bfcc299a4a059507028afa7563591fd05c17d56b3214'
$MediaInfoVersion = '26.05'
$MediaInfoAssetName = 'MediaInfo_CLI_26.05_Windows_x64.zip'
$MediaInfoSha256 = 'f7f80620ce6d14f4995f0de6f98e3ef18ad29496db01899571152ee3311229f9'

$ScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$Parent = Split-Path -Parent $ScriptRoot
$FinalName = "MattMux-$AppVersion-Fully-Portable-Offline"
$Stage = Join-Path $Parent $FinalName
$ZipPath = Join-Path $Parent ($FinalName + '.zip')
$Work = Join-Path ([System.IO.Path]::GetTempPath()) ("MattMuxOffline-" + [guid]::NewGuid().ToString('N'))

function Get-Sha256([string]$Path) {
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Assert-Sha256([string]$Path, [string]$Expected, [string]$Label) {
    $actual = Get-Sha256 $Path
    if ($actual -ne $Expected.ToLowerInvariant()) {
        throw "$Label checksum mismatch.`nExpected: $Expected`nActual:   $actual"
    }
}

function Download-File([string]$Url, [string]$Destination) {
    Write-Host "Downloading $Url"
    $dir = Split-Path -Parent $Destination
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    } catch {}
    Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $Destination
}

function Copy-DirectoryFiles([string]$From, [string]$To) {
    New-Item -ItemType Directory -Force -Path $To | Out-Null
    Get-ChildItem -LiteralPath $From -File | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $To $_.Name) -Force
    }
}

try {
    if (-not (Test-Path -LiteralPath (Join-Path $ScriptRoot 'MattMux-Portable.exe'))) {
        throw 'MattMux-Portable.exe is missing. Keep this builder next to the MattMux portable files.'
    }

    New-Item -ItemType Directory -Force -Path $Work | Out-Null
    if (Test-Path -LiteralPath $Stage) { Remove-Item -LiteralPath $Stage -Recurse -Force }
    if (Test-Path -LiteralPath $ZipPath) { Remove-Item -LiteralPath $ZipPath -Force }
    New-Item -ItemType Directory -Force -Path $Stage | Out-Null

    Write-Host 'Staging MattMux portable files...'
    foreach ($name in @('MattMux-Portable.exe', 'MattMux-Portable.exe.manifest', 'MattMux.ico')) {
        Copy-Item -LiteralPath (Join-Path $ScriptRoot $name) -Destination (Join-Path $Stage $name) -Force
    }
    New-Item -ItemType Directory -Force -Path (Join-Path $Stage 'MattMuxData') | Out-Null

    $ffBase = "https://github.com/BtbN/FFmpeg-Builds/releases/download/$FfmpegReleaseTag"
    $manifest = Join-Path $Work 'checksums.sha256'
    Download-File "$ffBase/checksums.sha256" $manifest
    Assert-Sha256 $manifest $FfmpegManifestSha256 'FFmpeg checksum manifest'

    $manifestText = Get-Content -LiteralPath $manifest -Raw
    $escapedAsset = [regex]::Escape($FfmpegAssetName)
    $match = [regex]::Match($manifestText, "(?im)^([0-9a-f]{64})\s+\*?$escapedAsset\s*$")
    if (-not $match.Success) {
        throw "The verified FFmpeg manifest does not contain $FfmpegAssetName"
    }
    $ffmpegArchiveSha = $match.Groups[1].Value.ToLowerInvariant()

    $ffArchive = Join-Path $Work $FfmpegAssetName
    Download-File "$ffBase/$FfmpegAssetName" $ffArchive
    Assert-Sha256 $ffArchive $ffmpegArchiveSha 'FFmpeg archive'

    $ffExtract = Join-Path $Work 'ffmpeg-extract'
    Expand-Archive -LiteralPath $ffArchive -DestinationPath $ffExtract -Force
    $ffmpegExe = Get-ChildItem -LiteralPath $ffExtract -Filter ffmpeg.exe -File -Recurse | Select-Object -First 1
    if ($null -eq $ffmpegExe) { throw 'ffmpeg.exe was not found in the verified archive.' }
    $ffBin = $ffmpegExe.Directory.FullName
    if (-not (Test-Path -LiteralPath (Join-Path $ffBin 'ffprobe.exe'))) {
        throw 'ffprobe.exe was not found beside ffmpeg.exe in the verified archive.'
    }

    $ffDest = Join-Path $Stage 'MattMuxData\tools\ffmpeg-2026-09-08'
    Copy-DirectoryFiles $ffBin $ffDest

    $miUrl = "https://mediaarea.net/download/binary/mediainfo/$MediaInfoVersion/$MediaInfoAssetName"
    $miArchive = Join-Path $Work $MediaInfoAssetName
    Download-File $miUrl $miArchive
    Assert-Sha256 $miArchive $MediaInfoSha256 'MediaInfo archive'

    $miExtract = Join-Path $Work 'mediainfo-extract'
    Expand-Archive -LiteralPath $miArchive -DestinationPath $miExtract -Force
    $miExe = Get-ChildItem -LiteralPath $miExtract -Filter MediaInfo.exe -File -Recurse | Select-Object -First 1
    if ($null -eq $miExe) { throw 'MediaInfo.exe was not found in the verified archive.' }
    $miDest = Join-Path $Stage "MattMuxData\tools\mediainfo-$MediaInfoVersion"
    Copy-DirectoryFiles $miExe.Directory.FullName $miDest

    foreach ($required in @(
        (Join-Path $ffDest 'ffmpeg.exe'),
        (Join-Path $ffDest 'ffprobe.exe'),
        (Join-Path $miDest 'MediaInfo.exe')
    )) {
        if (-not (Test-Path -LiteralPath $required)) { throw "Required offline tool missing: $required" }
    }

    @"
MattMux $AppVersion — Fully Portable / Offline
===============================================

This package is ready to move to another Windows x64 PC without downloading
FFmpeg, FFprobe, or MediaInfo again.

Run:
  MattMux-Portable.exe

Portable data:
  MattMuxData\settings.json
  MattMuxData\MattMux.log
  MattMuxData\tools\...

Bundled tools:
  FFmpeg / FFprobe: BtbN Auto-Build 2026-09-08 23:15
    build: N-126479-g08cd8df29d, Windows x64 GPL shared
  MediaInfo CLI: 26.05, Windows x64

The tools were downloaded over HTTPS and verified before packaging. MattMux
will use the bundled copies first, so an internet connection is not required
for normal DVD/VIDEO_TS remuxing and metadata/chapter inspection.

Keep the complete folder together when moving it between computers.
"@ | Set-Content -LiteralPath (Join-Path $Stage 'README-OFFLINE.txt') -Encoding UTF8

    @"
THIRD-PARTY SOFTWARE — MattMux $AppVersion Offline Bundle
=========================================================

FFmpeg / FFprobe
  Provider/build distributor: BtbN/FFmpeg-Builds
  Release tag: $FfmpegReleaseTag
  Asset: $FfmpegAssetName
  License: GPL build; see the notices/files included with that build and FFmpeg's licensing information.
  Upstream source: https://ffmpeg.org/
  Build project: https://github.com/BtbN/FFmpeg-Builds

MediaInfo CLI
  Version: $MediaInfoVersion
  Asset: $MediaInfoAssetName
  Project: https://mediaarea.net/MediaInfo
  License information: https://mediaarea.net/en/MediaInfo/License

These third-party programs are separate projects and are not authored by MattMux.
If you redistribute this offline package publicly, review and comply with the
applicable third-party licenses, including source-code distribution obligations.
"@ | Set-Content -LiteralPath (Join-Path $Stage 'THIRD_PARTY.txt') -Encoding UTF8

    @"
Verified input archives
=======================
FFmpeg checksums.sha256  $FfmpegManifestSha256
FFmpeg archive           $ffmpegArchiveSha  $FfmpegAssetName
MediaInfo archive        $MediaInfoSha256  $MediaInfoAssetName
"@ | Set-Content -LiteralPath (Join-Path $Stage 'VERIFIED-INPUTS.txt') -Encoding ASCII

    $checksumPath = Join-Path $Stage 'CHECKSUMS.txt'
    $lines = @()
    Get-ChildItem -LiteralPath $Stage -File -Recurse |
        Where-Object { $_.FullName -ne $checksumPath } |
        Sort-Object FullName |
        ForEach-Object {
            $rel = $_.FullName.Substring($Stage.Length + 1).Replace('\','/')
            $hash = Get-Sha256 $_.FullName
            $lines += "$hash  $rel"
        }
    $lines | Set-Content -LiteralPath $checksumPath -Encoding ASCII

    Write-Host 'Creating offline ZIP...'
    Compress-Archive -Path $Stage -DestinationPath $ZipPath -CompressionLevel Optimal

    $zipHash = Get-Sha256 $ZipPath
    Write-Host ''
    Write-Host 'SUCCESS' -ForegroundColor Green
    Write-Host "Offline folder: $Stage"
    Write-Host "Offline ZIP:    $ZipPath"
    Write-Host "ZIP SHA-256:    $zipHash"
}
finally {
    if (Test-Path -LiteralPath $Work) {
        Remove-Item -LiteralPath $Work -Recurse -Force -ErrorAction SilentlyContinue
    }
}
