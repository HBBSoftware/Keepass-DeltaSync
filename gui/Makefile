# keepass-deltasync-gui — byg-targets
#
# GUI'en bygger med Fyne, som kræver CGO + en C-compiler. Cross-compilering
# mellem OS'er er besværligt med CGO; byg derfor helst NATIVT på hvert OS, eller
# brug `fyne-cross` (Docker) til at krydskompilere. Se README.md.

APP     := keepass-deltasync-gui
PKG     := .
LDFLAGS := -s -w

export CGO_ENABLED := 1

.PHONY: build run clean tidy package-windows package-linux package-darwin

## build: byg en binær til det aktuelle OS
build:
	go build -ldflags "$(LDFLAGS)" -o $(APP)$(EXT) $(PKG)

## run: byg og kør
run:
	go run $(PKG)

## tidy: ryd op i afhængigheder
tidy:
	go mod tidy

## clean: fjern byggeartefakter
clean:
	rm -f $(APP) $(APP).exe
	rm -rf dist

# --- Pæne installerbare pakker via fyne-værktøjet (anbefalet til release) ---
# Kræver: go install fyne.io/tools/cmd/fyne@latest
# Metadata (navn, ikon, id, version) læses fra FyneApp.toml.
# Kør hvert target på det tilsvarende OS (CGO cross-compile er ikke trivielt).

## icon: gentegn icon.png
icon:
	go run ./tools/genicon

# Fyne navngiver output-filen efter Name i FyneApp.toml ("KeePass Delta-Sync").
# Vi beholder det pæne visningsnavn i metadata/titellinje, men omdøber selve
# filen til projekt-navnet, så den matcher CLI'ens keepass-deltasync.exe.

## package-windows: lav en Windows .exe med ikon/metadata
package-windows:
	fyne package -os windows
	mv "KeePass Delta-Sync.exe" $(APP).exe

## package-linux: lav en Linux .tar.xz
package-linux:
	fyne package -os linux

## package-darwin: lav en macOS .app
package-darwin:
	fyne package -os darwin
