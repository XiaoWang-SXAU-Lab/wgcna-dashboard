; rsWGCNA Windows installer (Inno Setup 6).
; Build the bundle first (dist-windows\rsWGCNA.exe + regutrace.exe +
; rs-wgcna-worker.exe + R\), then run:
;   ISCC.exe packaging\setup.iss
;
; The three EXEs are sidecars: the Go app executes them from {app}, never from
; a temporary directory. Sign all three and the installer before release.

[Setup]
AppName=rsWGCNA
AppVersion=0.6.0
AppPublisher=XiaoWang-SXAU-Lab
AppPublisherURL=https://github.com/XiaoWang-SXAU-Lab
DefaultDirName={localappdata}\rsWGCNA
DefaultGroupName=rsWGCNA
OutputDir=..\dist-windows
OutputBaseFilename=rsWGCNA-setup-0.6.0
Compression=lzma2
SolidCompression=yes
ArchitecturesAllowed=x64compatible
PrivilegesRequired=lowest
WizardStyle=modern
DisableProgramGroupPage=yes
SetupIconFile=icon.ico
UninstallDisplayIcon={app}\rsWGCNA.exe

[Files]
Source: "..\dist-windows\rsWGCNA.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist-windows\regutrace.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist-windows\rs-wgcna-worker.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist-windows\R\*"; DestDir: "{app}\R"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\rsWGCNA"; Filename: "{app}\rsWGCNA.exe"
Name: "{autodesktop}\rsWGCNA"; Filename: "{app}\rsWGCNA.exe"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional tasks:"

[Run]
Filename: "{app}\rsWGCNA.exe"; Description: "Launch rsWGCNA"; Flags: nowait postinstall skipifsilent
