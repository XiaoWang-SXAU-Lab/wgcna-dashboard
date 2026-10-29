; rsWGCNA Windows installer (Inno Setup 6).
; Build the bundle first (dist-windows\wgcna-dashboard.exe + R\), then run:
;   ISCC.exe packaging\setup.iss

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
UninstallDisplayIcon={app}\wgcna-dashboard.exe

[Files]
Source: "..\dist-windows\wgcna-dashboard.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist-windows\R\*"; DestDir: "{app}\R"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\rsWGCNA"; Filename: "{app}\wgcna-dashboard.exe"
Name: "{autodesktop}\rsWGCNA"; Filename: "{app}\wgcna-dashboard.exe"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional tasks:"

[Run]
Filename: "{app}\wgcna-dashboard.exe"; Description: "Launch rsWGCNA"; Flags: nowait postinstall skipifsilent
