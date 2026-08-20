# SPDX-License-Identifier: GPL-3.0-or-later
#
# Bygger den SAMLEDE Windows-installer (GUI + kommandolinje-klient i én fil).
# Trin:
#   1. byg GUI'en med `fyne package` (indlejrer ikon + version)
#   2. byg CLI'en fra client/ i samme checkout (ren Go)
#   3. lav zip-arkiv af begge komponenters kildekode (valgfri installer-komponent)
#   4. lav en icon.ico til selve installeren
#   5. kør ISCC og læg KeePass-Delta-Sync-Setup-<ver>.exe i out\
#
# Kør fra installer-mappen:  pwsh -File build.ps1
# Forudsætter: Go, `fyne` (go install fyne.io/tools/cmd/fyne@latest), en gcc
# (mingw-w64) til CGO, og Inno Setup 6 (ISCC.exe).

param(
  [string]$GuiRepo = (Resolve-Path "$PSScriptRoot\..").Path,
  # Monorepo-roden. GUI'en og CLI'en er to komponenter i SAMME checkout —
  # tidligere pegede denne på et søster-repo, hvilket var en udokumenteret
  # afhængighed mellem to kloner.
  [string]$Repo    = (Resolve-Path "$PSScriptRoot\..\..").Path,
  [string]$Gcc     = "$env:LOCALAPPDATA\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin",
  [string]$Iscc    = "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe"
)
$ErrorActionPreference = 'Stop'
$stage = Join-Path $PSScriptRoot 'dist'
$out   = Join-Path $PSScriptRoot 'out'
Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path "$stage\app","$stage\source",$out | Out-Null

# Versioner: GUI fra FyneApp.toml, CLI fra git-tag (client/vX.Y.Z).
$appVer = (Select-String -Path "$GuiRepo\FyneApp.toml" -Pattern 'Version\s*=\s*"([^"]+)"').Matches[0].Groups[1].Value
$cliVer = (git -C $Repo describe --tags --match 'client/*').Trim() -replace '^client/',''
Write-Host "GUI $appVer / CLI $cliVer"

# 1. GUI (fyne package → indlejret ikon + windowsgui, ingen konsol)
$env:PATH = "$Gcc;" + (Join-Path (go env GOPATH) 'bin') + ";$env:PATH"
$env:CGO_ENABLED = '1'; $env:CC = "$Gcc\gcc.exe"
Push-Location $GuiRepo
Remove-Item "$GuiRepo\KeePass Delta-Sync.exe" -ErrorAction SilentlyContinue
fyne package --os windows --release
Move-Item -Force "$GuiRepo\KeePass Delta-Sync.exe" "$stage\app\keepass-deltasync-gui.exe"
Pop-Location

# 2. CLI (ren Go, ingen CGO)
Push-Location "$Repo\client"
$env:CGO_ENABLED = '0'
go build -ldflags="-s -w -X main.version=$cliVer" -o "$stage\app\keepass-deltasync.exe" ./cmd/keepass-deltasync
Pop-Location

# 3. kildekode-zips (kun git-sporet indhold, ingen .git / byggeartefakter).
# `HEAD:gui` / `HEAD:client` arkiverer netop den ene komponents undertræ, med
# stier relative til den — samme zip-indhold som da de var hvert sit repo.
git -C $Repo archive --format=zip -o "$stage\source\keepass-deltasync-gui-src.zip" HEAD:gui
git -C $Repo archive --format=zip -o "$stage\source\keepass-deltasync-src.zip" HEAD:client

# 4. icon.ico (256px PNG indlejret i en ICO — Vista+)
Add-Type -AssemblyName System.Drawing
$src = [System.Drawing.Image]::FromFile("$GuiRepo\icon.png")
$bmp = New-Object System.Drawing.Bitmap 256,256
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$g.DrawImage($src,0,0,256,256); $g.Dispose(); $src.Dispose()
$ms = New-Object System.IO.MemoryStream
$bmp.Save($ms,[System.Drawing.Imaging.ImageFormat]::Png); $png = $ms.ToArray(); $ms.Dispose(); $bmp.Dispose()
$fs = [System.IO.File]::Create("$stage\icon.ico"); $bw = New-Object System.IO.BinaryWriter($fs)
$bw.Write([UInt16]0); $bw.Write([UInt16]1); $bw.Write([UInt16]1)
$bw.Write([Byte]0); $bw.Write([Byte]0); $bw.Write([Byte]0); $bw.Write([Byte]0)
$bw.Write([UInt16]1); $bw.Write([UInt16]32); $bw.Write([UInt32]$png.Length); $bw.Write([UInt32]22)
$bw.Write($png); $bw.Flush(); $fs.Close()

# LICENSE til licens-siden
Copy-Item "$GuiRepo\LICENSE" "$stage\LICENSE" -Force

# 5. compile
& $Iscc "/DStageDir=$stage" "/DOutDir=$out" "/DAppVer=$appVer" "/DCliVer=$cliVer" "$PSScriptRoot\keepass-deltasync.iss"
if ($LASTEXITCODE -ne 0) { throw "ISCC fejlede ($LASTEXITCODE)" }
Write-Host "`nFærdig: $out\KeePass-Delta-Sync-Setup-$appVer.exe"
