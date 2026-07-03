; SPDX-License-Identifier: GPL-3.0-or-later
;
; Inno Setup-script for en SAMLET Windows-installer der lægger BÅDE
; GUI'en (keepass-deltasync-gui.exe) OG kommandolinje-klienten
; (keepass-deltasync.exe) i samme mappe. GUI'en er en tynd wrapper der finder
; CLI'en ved siden af sig selv (se locateCLI i cli.go), så placeringen i samme
; mappe er dét der binder de to sammen — ingen PATH-opsætning nødvendig.
;
; Kildekode-komponenten (valgfri, fravalgt som standard) lægger et zip-arkiv af
; hvert repo i en source\-undermappe.
;
; Alt installeres i BRUGERENS eget område uden administrator (PrivilegesRequired
; = lowest), i tråd med at autostart/daemon også kører per-bruger uden admin.
;
; Byg med (stierne peger på et "staging"-træ med de byggede filer):
;   ISCC.exe /DStageDir=<sti\til\stage> /DOutDir=<sti\til\output> ^
;            /DAppVer=0.3.1 /DCliVer=v1.7.0 keepass-deltasync.iss
;
; Staging-træet forventes at se sådan ud:
;   <StageDir>\app\keepass-deltasync-gui.exe
;   <StageDir>\app\keepass-deltasync.exe
;   <StageDir>\source\keepass-deltasync-gui-src.zip
;   <StageDir>\source\keepass-deltasync-src.zip
;   <StageDir>\icon.ico
;   <StageDir>\LICENSE

#ifndef StageDir
  #define StageDir "dist"
#endif
#ifndef OutDir
  #define OutDir "out"
#endif
#ifndef AppVer
  #define AppVer "0.3.1"
#endif
#ifndef CliVer
  #define CliVer "bundled"
#endif

#define AppName "KeePass Delta-Sync"
#define AppPublisher "HBB Software"
#define AppURL "https://deltasync.bjoerck-braun.dk/"
#define GuiExe "keepass-deltasync-gui.exe"

[Setup]
; AppId binder installationer sammen (opgradering/afinstallation). Skift ALDRIG.
AppId={{7A1E9C42-3B8D-4F16-A5C0-9D2E4B6F8A31}
AppName={#AppName}
AppVersion={#AppVer}
AppVerName={#AppName} {#AppVer}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}
VersionInfoVersion={#AppVer}
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
; Per-bruger, ingen administrator (kan skiftes af brugeren hvis de vil have
; en maskine-bred installation og har rettigheder til det).
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
LicenseFile={#StageDir}\LICENSE
SetupIconFile={#StageDir}\icon.ico
UninstallDisplayIcon={app}\{#GuiExe}
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutDir}
OutputBaseFilename=KeePass-Delta-Sync-Setup-{#AppVer}

[Languages]
Name: "danish"; MessagesFile: "compiler:Languages\Danish.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Components]
Name: "core"; Description: "Program (GUI + kommandolinje-klient)"; Types: full compact custom; Flags: fixed
Name: "source"; Description: "Kildekode (zip af begge projekter)"; Types: full

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
; Kernen: begge programmer i SAMME mappe — det er dét der lader GUI'en finde CLI'en.
Source: "{#StageDir}\app\keepass-deltasync-gui.exe"; DestDir: "{app}"; Components: core; Flags: ignoreversion
Source: "{#StageDir}\app\keepass-deltasync.exe";     DestDir: "{app}"; Components: core; Flags: ignoreversion
Source: "{#StageDir}\LICENSE";                       DestDir: "{app}"; Components: core; Flags: ignoreversion; DestName: "LICENSE.txt"
; Valgfri kildekode.
Source: "{#StageDir}\source\keepass-deltasync-gui-src.zip"; DestDir: "{app}\source"; Components: source; Flags: ignoreversion
Source: "{#StageDir}\source\keepass-deltasync-src.zip";     DestDir: "{app}\source"; Components: source; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#GuiExe}"
Name: "{group}\{cm:UninstallProgram,{#AppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#GuiExe}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#GuiExe}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent
