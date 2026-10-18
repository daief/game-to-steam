; Compile with ISCC.exe /DAppVersion=1.2.3 /DAppVersionNumeric=1.2.3.0 setup.iss
#ifndef AppVersion
  #define AppVersion "0.2.0"
#endif
#ifndef AppVersionNumeric
  #define AppVersionNumeric "0.2.0.0"
#endif
#ifndef SourceDir
  #define SourceDir "..\build\windows-amd64"
#endif
#ifndef OutputDir
  #define OutputDir "..\dist"
#endif

[Setup]
AppId={{1B5FB42D-7DDB-405D-BC65-75B077A49F75}
AppName=game-to-steam
AppVersion={#AppVersion}
VersionInfoVersion={#AppVersionNumeric}
AppPublisher=game-to-steam
DefaultDirName={localappdata}\Programs\game-to-steam
DisableDirPage=no
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir={#OutputDir}
OutputBaseFilename=game-to-steam-{#AppVersion}-windows-amd64-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
SetupIconFile=..\assets\icon.ico
UninstallDisplayIcon={app}\game-to-steam.exe
CloseApplications=no
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; Flags: unchecked

[Files]
Source: "{#SourceDir}\game-to-steam.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\epic-bridge.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autodesktop}\game-to-steam"; Filename: "{app}\game-to-steam.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\game-to-steam.exe"; WorkingDir: "{app}"; Description: "Launch game-to-steam"; Flags: nowait postinstall skipifsilent
