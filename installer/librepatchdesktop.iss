; Inno Setup script: a normal Windows installer (Start menu entry, optional desktop icon, uninstaller and an
; Add/Remove Programs entry). Build: iscc /DAppVersion=1.2.3 installer\librepatchdesktop.iss
#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif

[Setup]
AppId={{7B1E4C52-3A0D-4E1B-9C77-5D2F6A9E0B14}
AppName=LibrePatchDesktop
AppVersion={#AppVersion}
AppPublisher=LibrePatch
AppPublisherURL=https://github.com/auzring1/LibrePatchDesktop
DefaultDirName={autopf}\LibrePatchDesktop
DefaultGroupName=LibrePatchDesktop
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=LibrePatchDesktop-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\librepatchdesktop.exe
CloseApplications=yes
LicenseFile=..\LICENSE

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: unchecked

[Files]
Source: "..\dist\librepatchdesktop.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\LibrePatchDesktop"; Filename: "{app}\librepatchdesktop.exe"
Name: "{autodesktop}\LibrePatchDesktop"; Filename: "{app}\librepatchdesktop.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\librepatchdesktop.exe"; Description: "Start LibrePatchDesktop"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; Close the running window before the files are removed.
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM librepatchdesktop.exe"; Flags: runhidden; RunOnceId: "StopApp"
