; GoPort Windows installer (Inno Setup 6).
;
; Built by the release workflow from the binary cross-compiled into server/dist/:
;   ISCC.exe /DAppVersion=0.1.6 Installers\Windows\goport.iss
; Output: server\dist\goport-windows-setup.exe
;
; Installs per user, so there is no administrator prompt, into
; %LOCALAPPDATA%\Programs\GoPort, and appends that folder to the user's PATH so
; `goport` works from any new terminal. The uninstaller removes the PATH entry.

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif

[Setup]
; Never change AppId: it is how a newer installer finds and upgrades an older install.
AppId={{C87621A1-55E4-42DE-A88C-FBCC3FB7F9CC}
AppName=GoPort
AppVersion={#AppVersion}
AppVerName=GoPort {#AppVersion}
AppPublisher=GoPort
AppPublisherURL=https://goport.uz
AppSupportURL=https://goport.uz/docs
AppUpdatesURL=https://goport.uz
DefaultDirName={autopf}\GoPort
PrivilegesRequired=lowest
DisableDirPage=yes
DisableProgramGroupPage=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
ChangesEnvironment=yes
UninstallDisplayName=GoPort
; goport.exe carries no icon resource, so Installed apps shows the logo file instead.
UninstallDisplayIcon={app}\goport.ico
OutputDir=..\..\server\dist
OutputBaseFilename=goport-windows-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
; Branding, rendered from landing/app/icon.svg. Each image lists 100% and 200% DPI sizes.
SetupIconFile=goport.ico
WizardSmallImageFile=wizard-small-55.png,wizard-small-110.png
WizardImageFile=wizard-large-164x314.png,wizard-large-328x628.png

[Files]
Source: "..\..\server\dist\goport-windows-amd64.exe"; DestDir: "{app}"; DestName: "goport.exe"; Flags: ignoreversion
Source: "goport.ico"; DestDir: "{app}"; Flags: ignoreversion

[Registry]
; {olddata} is the raw existing value, so %VARIABLES% in the user's PATH survive.
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Check: NeedsAddPath(ExpandConstant('{app}'))

[Messages]
FinishedHeadingLabel=GoPort is installed
FinishedLabelNoIcons=Open a new terminal and run:%n%n    goport auth <token>%n    goport http 8080%n%nTerminals that were already open won't find the goport command until you reopen them.
FinishedLabel=Open a new terminal and run:%n%n    goport auth <token>%n    goport http 8080%n%nTerminals that were already open won't find the goport command until you reopen them.

[Code]
const
  EnvironmentKey = 'Environment';

{ Separators on both sides keep C:\Go from matching C:\GoPort. }
function PathListContains(PathList, Dir: string): Boolean;
begin
  Result := Pos(';' + Uppercase(Dir) + ';', ';' + Uppercase(PathList) + ';') > 0;
end;

function NeedsAddPath(Dir: string): Boolean;
var
  PathList: string;
begin
  if not RegQueryStringValue(HKCU, EnvironmentKey, 'Path', PathList) then
    Result := True
  else
    Result := not PathListContains(PathList, Dir) and not PathListContains(PathList, Dir + '\');
end;

procedure RemovePath(Dir: string);
var
  PathList, Padded, Needle: string;
  P: Integer;
begin
  if not RegQueryStringValue(HKCU, EnvironmentKey, 'Path', PathList) then
    Exit;

  Padded := ';' + PathList + ';';
  Needle := ';' + Dir + ';';
  P := Pos(Uppercase(Needle), Uppercase(Padded));
  if P = 0 then
    Exit;

  { Drop the entry and its leading separator, then the padding added above. }
  Delete(Padded, P, Length(Needle) - 1);
  RegWriteExpandStringValue(HKCU, EnvironmentKey, 'Path', Copy(Padded, 2, Length(Padded) - 2));
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    RemovePath(ExpandConstant('{app}'));
end;
