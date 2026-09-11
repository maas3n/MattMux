$ErrorActionPreference = 'Stop'
New-Item -ItemType Directory -Force evidence | Out-Null
Start-Transcript -Path evidence/windows-runtime.txt
Add-Type @'
using System;
using System.Runtime.InteropServices;
public class WinAudit {
 [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr w, int id);
 [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr SendMessage(IntPtr w, uint msg, IntPtr wp, string lp);
 [DllImport("user32.dll", EntryPoint="SendMessageW")] public static extern IntPtr Msg(IntPtr w, uint msg, IntPtr wp, IntPtr lp);
 [DllImport("user32.dll")] public static extern bool IsWindowEnabled(IntPtr w);
 [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr w);
}
'@
Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
function Capture([string]$name) {
 $r = [System.Windows.Forms.SystemInformation]::VirtualScreen
 $b = [System.Drawing.Bitmap]::new($r.Width,$r.Height)
 $g = [System.Drawing.Graphics]::FromImage($b)
 try { $g.CopyFromScreen($r.Left,$r.Top,0,0,$b.Size); $b.Save((Join-Path $PWD "evidence/$name.png")) }
 finally { $g.Dispose(); $b.Dispose() }
}
Invoke-WebRequest 'https://github.com/maas3n/MattMux/releases/download/v1.2.0/MattMux-1.2.0-Portable.zip' -OutFile portable.zip
if ((Get-FileHash portable.zip -Algorithm SHA256).Hash.ToLowerInvariant() -ne '70078fad8f8af7d71b9280bc840fd1c2d56d2351034d2733b8d94f28fd8dbeff') { throw 'Published ZIP checksum mismatch' }
Expand-Archive portable.zip -DestinationPath portable
$exe = (Get-ChildItem portable -Filter MattMux-Portable.exe -Recurse).FullName
$ffprobe = (Get-ChildItem portable -Filter ffprobe.exe -Recurse).FullName
$env:PATH = (Split-Path $ffprobe) + ';' + $env:PATH
foreach ($kind in @('folder','iso')) {
 $source = if ($kind -eq 'folder') { (Resolve-Path fixture/disc).Path } else { (Resolve-Path fixture/disc.iso).Path }
 $outDir = New-Item -ItemType Directory -Force "evidence/out-$kind"
 $p = Start-Process -FilePath $exe -PassThru
 try {
  $deadline = (Get-Date).AddSeconds(30)
  do { Start-Sleep -Milliseconds 250; $p.Refresh() } while ($p.MainWindowHandle -eq 0 -and !$p.HasExited -and (Get-Date) -lt $deadline)
  if ($p.HasExited -or $p.MainWindowHandle -eq 0) { throw 'Published GUI did not open' }
  $w = $p.MainWindowHandle
  [void][WinAudit]::SetForegroundWindow($w)
  [void][WinAudit]::SendMessage([WinAudit]::GetDlgItem($w,1001),0x000C,[IntPtr]::Zero,$source)
  [void][WinAudit]::SendMessage([WinAudit]::GetDlgItem($w,1004),0x000C,[IntPtr]::Zero,$outDir.FullName)
  [void][WinAudit]::Msg([WinAudit]::GetDlgItem($w,1012),0x00F1,[IntPtr]1,[IntPtr]::Zero)
  [void][WinAudit]::Msg([WinAudit]::GetDlgItem($w,1007),0x00F5,[IntPtr]::Zero,[IntPtr]::Zero)
  $deadline = (Get-Date).AddSeconds(90)
  do { Start-Sleep -Milliseconds 250; $count = [WinAudit]::Msg([WinAudit]::GetDlgItem($w,1006),0x0146,[IntPtr]::Zero,[IntPtr]::Zero).ToInt32() } while (($count -lt 1 -or ![WinAudit]::IsWindowEnabled([WinAudit]::GetDlgItem($w,1009))) -and (Get-Date) -lt $deadline)
  Capture "windows-$kind-scanned"
  if ($count -ne 1) { throw "Expected one title; got $count" }
  [void][WinAudit]::Msg([WinAudit]::GetDlgItem($w,1009),0x00F5,[IntPtr]::Zero,[IntPtr]::Zero)
  $final = Join-Path $outDir.FullName 'disc.mkv'
  $deadline = (Get-Date).AddSeconds(90)
  while (!(Test-Path $final) -and (Get-Date) -lt $deadline) { Start-Sleep -Milliseconds 250 }
  Capture "windows-$kind-remux"
  if (!(Test-Path $final)) { throw 'GUI remux did not produce final output' }
  if ((Get-Item $final).Length -eq 0) { throw 'Output is empty' }
  & $ffprobe -v error -show_streams -show_chapters -of json $final > "evidence/windows-$kind-probe.json"
  if ($LASTEXITCODE -ne 0) { throw 'Output probing failed' }
  Write-Output "Published Windows GUI $kind scan/remux PASS"
 } finally {
  if (!$p.HasExited) { Stop-Process -Id $p.Id -Force }
 }
}
python audit/compare-output.py evidence/out-folder/disc.mkv evidence/out-iso/disc.mkv
if ($LASTEXITCODE -ne 0) { throw 'Folder/ISO parity failed' }
Get-ChildItem portable -Filter '*.log' -Recurse | Copy-Item -Destination evidence
Stop-Transcript
