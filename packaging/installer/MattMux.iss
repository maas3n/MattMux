#define MyAppName "MattMux"
#ifndef MyAppVersion
  #define MyAppVersion "1.2.0"
#endif
#define MyAppExeName "MattMux.exe"

[Setup]
AppId={{50A8860E-7C1D-4E39-AF0D-9D877D7FB49C}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
DefaultDirName={autopf}\MattMux
DefaultGroupName=MattMux
DisableProgramGroupPage=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=admin
OutputDir=..\..\dist
OutputBaseFilename=MattMux-{#MyAppVersion}-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\{#MyAppExeName}
VersionInfoVersion={#MyAppVersion}.0
VersionInfoProductName=MattMux
VersionInfoDescription=MattMux DVD remuxer

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Files]
Source: "..\..\MattMux.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\src\MattMux.exe.manifest"; DestDir: "{app}"; DestName: "MattMux.exe.manifest"; Flags: ignoreversion
Source: "..\..\src\MattMux.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\MattMux"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"
Name: "{autodesktop}\MattMux"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Launch MattMux"; Flags: nowait postinstall skipifsilent
