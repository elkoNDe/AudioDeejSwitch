; AudioDeejSwitch Inno Setup Script
#define MyAppName "AudioDeejSwitch"
#ifndef MyAppVersion
  #define MyAppVersion "1.0.0"
#endif
#ifndef OutputDir
  #define OutputDir "..\dist"
#endif
#ifndef SourceDir
  #define SourceDir "..\dist"
#endif
#define MyAppPublisher "elkoNDe"
#define MyAppURL "https://github.com/elkoNDe/AudioDeejSwitch"
#define MyAppExeName "AudioDeejSwitch.exe"

[Setup]
AppId={{AUDIODEEJSWITCH_APPID}}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
OutputDir={#OutputDir}
OutputBaseFilename=AudioDeejSwitch_Setup_{#MyAppVersion}
Compression=lzma
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64
ArchitecturesAllowed=x64

[Files]
Source: "{#SourceDir}\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\config.yaml"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: files; Name: "{app}\config.yaml"

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueName: "AudioDeejSwitch"; ValueType: string; ValueData: """{app}\{#MyAppExeName}"""; Flags: uninsdeletevalue