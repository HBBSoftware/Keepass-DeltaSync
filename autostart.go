// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

// Autostart får styresystemet til at køre `keepass-deltasync daemon` ved login,
// så baggrunds-synkroniseringen kører uden at GUI'en er åben. Mekanismen er
// forskellig per OS:
//
//   - Windows: en "ved logon"-opgave i Opgaveplanlægning (Task Scheduler)
//   - macOS:   en launchd LaunchAgent i ~/Library/LaunchAgents
//   - Linux:   en systemd --user service i ~/.config/systemd/user
//
// Alt sker i BRUGERENS eget område — ingen administrator/root nødvendig. (En
// "ved logon"-opgave i den indloggede brugers egen kontekst — LeastPrivilege +
// InteractiveToken — kræver IKKE administrator; det gør kun en opgave der skal
// køre uafhængigt af hvem der er logget på.) Opgaveplanlægning vælges frem for
// den simplere HKCU\…\Run-nøgle, fordi den giver tre ting som Run-nøglen mangler
// og som macOS/Linux-varianterne har: en startforsinkelse så netværket er oppe
// (Delay), automatisk genstart hvis daemonen dør (RestartOnFailure), og en
// logfil at fejlsøge i. (En blot-sat Run-nøgle starter daemonen én gang uden
// log; dør den — fx fordi netværket ikke er oppe endnu efter en Fast Startup-
// resume — bliver den væk til næste login, helt tavst.) Opgaven kører en skjult
// wscript-launcher (daemon-launcher.vbs) der starter daemonen uden konsolvindue,
// VENTER på den og videregiver dens exit-kode, så Opgaveplanlægning kan se et
// nedbrud og genstarte. GUI'en kører ALDRIG selv daemonen (jf. beslutningen om
// ikke at have en daemon inde i appen); den opretter blot opsætningen og
// overlader kørslen til OS'et. Ældre versioner brugte HKCU\…\Run — den ryddes
// op ved aktivering (migration).
const (
	autostartTaskName = "KeePassDeltaSync"                // Windows: Opgaveplanlægning-opgavenavn
	autostartLabel    = "com.keepassdeltasync.daemon"     // macOS: launchd-label
	autostartUnit     = "keepass-deltasync.service"       // Linux: systemd-unit

	// Ældre Windows-mekanisme (HKCU\…\Run) — beholdes kun for at kunne rydde den
	// op ved migration til Opgaveplanlægning.
	autostartRunKey   = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	autostartRunValue = "KeePassDeltaSync"
)

// autostartSupported melder om OS'et er et af de tre vi kan opsætte autostart for.
func autostartSupported() bool {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		return true
	}
	return false
}

// runHidden kører en kommando uden et blinkende konsolvindue (Windows) og
// returnerer den samlede output + fejl.
func runHidden(name string, args ...string) (string, error) {
	ctx, cancel := withTimeout(20 * time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	hideConsole(cmd)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// autostartStartNow starter daemonen med det samme via den netop oprettede
// opgave (schtasks /run), så autosync kører straks i stedet for først ved næste
// login — og som præcis samme skjulte, loggede og overvågede instans som et
// login ville starte. Bruges på Windows efter at opgaven er oprettet.
func autostartStartNow() error {
	_, err := runHidden("schtasks", "/run", "/tn", autostartTaskName)
	return err
}

// winAutostartDir er mappen til den skjulte launcher + daemon-log på Windows
// (under %LOCALAPPDATA%). Filerne her udgør "ved logon"-opgavens handling.
func winAutostartDir() (string, error) {
	base, err := os.UserCacheDir() // %LOCALAPPDATA% på Windows
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "keepass-deltasync"), nil
}

// winLauncherPaths giver stierne til de filer Windows-autostarten bruger: en
// .cmd der kører daemonen og logger, en .vbs der starter .cmd'en uden
// konsolvindue, og logfilen.
func winLauncherPaths() (cmdPath, vbsPath, logPath string, err error) {
	dir, err := winAutostartDir()
	if err != nil {
		return "", "", "", err
	}
	return filepath.Join(dir, "daemon-run.cmd"),
		filepath.Join(dir, "daemon-launcher.vbs"),
		filepath.Join(dir, "daemon.log"), nil
}

// winWriteLauncher skriver .cmd- og .vbs-launcherne og returnerer stien til
// .vbs'en (opgavens handling). .cmd'en kører `<cliPath> daemon` og omdirigerer
// stdout+stderr til logfilen; .vbs'en starter .cmd'en SKJULT (window-style 0) og
// VENTER på den (bWaitOnReturn = True), så wscript-processen lever lige så længe
// som daemonen og afslutter med dens exit-kode — det er dét Opgaveplanlægning
// overvåger for at kunne genstarte ved nedbrud.
func winWriteLauncher(cliPath string) (vbsPath string, err error) {
	dir, err := winAutostartDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	cmdPath, vbsPath, logPath, err := winLauncherPaths()
	if err != nil {
		return "", err
	}
	cmdSrc := "@echo off\r\n\"" + cliPath + "\" daemon 1>> \"" + logPath + "\" 2>&1\r\n"
	if err := os.WriteFile(cmdPath, []byte(cmdSrc), 0o644); err != nil {
		return "", err
	}
	// Tredobbelt citationstegn i VBS = ét literalt " om stien til .cmd'en.
	vbsSrc := "rc = CreateObject(\"WScript.Shell\").Run(\"\"\"" + cmdPath + "\"\"\", 0, True)\r\nWScript.Quit rc\r\n"
	if err := os.WriteFile(vbsPath, []byte(vbsSrc), 0o644); err != nil {
		return "", err
	}
	return vbsPath, nil
}

// winTaskXML bygger XML'en til "ved logon"-opgaven for den aktuelle bruger:
// startforsinkelse (Delay) så netværket er oppe, ubegrænset levetid
// (ExecutionTimeLimit PT0S) og automatisk genstart hvis daemonen dør
// (RestartOnFailure). LogonType=InteractiveToken gør at opgaven kører i
// brugerens egen session og dermed har adgang til OS-keyringen (passwords).
func winTaskXML(vbsPath string) (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	usr := xmlEscape(u.Username)
	args := xmlEscape("\"" + vbsPath + "\"")
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>KeePass Delta-Sync baggrunds-synkronisering</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>` + usr + `</UserId>
      <Delay>PT30S</Delay>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>` + usr + `</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <StartWhenAvailable>true</StartWhenAvailable>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Enabled>true</Enabled>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
    <AllowStartOnDemand>true</AllowStartOnDemand>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>wscript.exe</Command>
      <Arguments>` + args + `</Arguments>
    </Exec>
  </Actions>
</Task>
`, nil
}

// xmlEscape escaper de tegn der ikke må stå rå i XML-tekst/attributter (et
// brugernavn eller en sti kan i princippet indeholde & eller <).
func xmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;",
	).Replace(s)
}

// utf16LE koder en streng som UTF-16 little-endian med BOM — det format schtasks
// forventer når en opgave importeres via /xml.
func utf16LE(s string) []byte {
	codes := utf16.Encode([]rune(s))
	b := make([]byte, 0, 2+len(codes)*2)
	b = append(b, 0xFF, 0xFE) // BOM
	for _, c := range codes {
		b = append(b, byte(c), byte(c>>8))
	}
	return b
}

// winRegisterTask skriver opgave-XML'en til en midlertidig fil og importerer den
// med schtasks /create (/f overskriver en evt. tidligere opgave).
func winRegisterTask(vbsPath string) error {
	xml, err := winTaskXML(vbsPath)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "kds-task-*.xml")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(utf16LE(xml)); err != nil {
		f.Close()
		return err
	}
	f.Close()
	if out, err := runHidden("schtasks", "/create", "/tn", autostartTaskName, "/xml", tmp, "/f"); err != nil {
		return fmt.Errorf("%s", firstLine(out, err))
	}
	return nil
}

// macPlistPath / linuxUnitPath giver stien til den fil vi skriver/sletter.
func macPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", autostartLabel+".plist"), nil
}

func linuxUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", autostartUnit), nil
}

// autostartInstalled melder om autostarten allerede er sat op. På Windows spørges
// Opgaveplanlægning; på macOS/Linux er det vores egen fil der afgør det (vi
// skriver den ved aktivering og sletter den ved deaktivering).
func autostartInstalled() (bool, error) {
	switch runtime.GOOS {
	case "windows":
		_, err := runHidden("schtasks", "/query", "/tn", autostartTaskName)
		return err == nil, nil
	case "darwin":
		p, err := macPlistPath()
		if err != nil {
			return false, err
		}
		return fileExists(p), nil
	case "linux":
		// Spørg systemd om unitten faktisk er *enabled* — ikke bare om filen
		// ligger der. En unit-fil kan sagtens findes uden at være enabled (fx
		// hvis `enable` fejlede), og så autostarter den ikke. is-enabled skriver
		// "enabled" til stdout og afslutter 0 når den er slået til.
		out, _ := runHidden("systemctl", "--user", "is-enabled", autostartUnit)
		return strings.TrimSpace(out) == "enabled", nil
	}
	return false, fmt.Errorf("%s", runtime.GOOS)
}

// autostartEnable opretter autostarten der kører `<cliPath> daemon` ved login.
func autostartEnable(cliPath string) error {
	switch runtime.GOOS {
	case "windows":
		// Skriv den skjulte launcher og registrér "ved logon"-opgaven der kører den.
		vbsPath, err := winWriteLauncher(cliPath)
		if err != nil {
			return err
		}
		if err := winRegisterTask(vbsPath); err != nil {
			return err
		}
		// Migration: fjern den gamle HKCU\…\Run-post hvis den findes (bedst-muligt),
		// så vi ikke har to mekanismer der hver starter en daemon.
		_, _ = runHidden("reg", "delete", autostartRunKey, "/v", autostartRunValue, "/f")
		// Opgavens trigger virker først ved næste login — start derfor daemonen NU
		// (samme skjulte, loggede instans). (macOS/Linux starter den allerede via
		// hhv. RunAtLoad og `enable --now`.) Bedst-muligt: en fejl her forhindrer
		// ikke at autostarten er sat op.
		_ = autostartStartNow()
		return nil
	case "darwin":
		p, err := macPlistPath()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(macPlist(cliPath)), 0o644); err != nil {
			return err
		}
		if out, err := runHidden("launchctl", "load", "-w", p); err != nil {
			return fmt.Errorf("%s", firstLine(out, err))
		}
		return nil
	case "linux":
		p, err := linuxUnitPath()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(linuxUnit(cliPath)), 0o644); err != nil {
			return err
		}
		if out, err := runHidden("systemctl", "--user", "daemon-reload"); err != nil {
			return fmt.Errorf("%s", firstLine(out, err))
		}
		if out, err := runHidden("systemctl", "--user", "enable", "--now", autostartUnit); err != nil {
			return fmt.Errorf("%s", firstLine(out, err))
		}
		return nil
	}
	return fmt.Errorf("%s", runtime.GOOS)
}

// autostartDisable fjerner autostarten igen.
func autostartDisable() error {
	switch runtime.GOOS {
	case "windows":
		// Stop en kørende instans bedst-muligt, slet så opgaven og ryd
		// launcher-filerne + en evt. gammel Run-post.
		_, _ = runHidden("schtasks", "/end", "/tn", autostartTaskName)
		if out, err := runHidden("schtasks", "/delete", "/tn", autostartTaskName, "/f"); err != nil {
			return fmt.Errorf("%s", firstLine(out, err))
		}
		if cmdPath, vbsPath, _, err := winLauncherPaths(); err == nil {
			_ = os.Remove(cmdPath)
			_ = os.Remove(vbsPath)
		}
		_, _ = runHidden("reg", "delete", autostartRunKey, "/v", autostartRunValue, "/f")
		return nil
	case "darwin":
		p, err := macPlistPath()
		if err != nil {
			return err
		}
		// Læs af kernen først (fejler harmløst hvis ikke loadet), så slet filen.
		_, _ = runHidden("launchctl", "unload", "-w", p)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	case "linux":
		p, err := linuxUnitPath()
		if err != nil {
			return err
		}
		_, _ = runHidden("systemctl", "--user", "disable", "--now", autostartUnit)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		_, _ = runHidden("systemctl", "--user", "daemon-reload")
		return nil
	}
	return fmt.Errorf("%s", runtime.GOOS)
}

// firstLine giver en kort fejltekst: første ikke-tomme linje af kommando-output
// (det er typisk den læsbare fejl fra schtasks/launchctl/systemctl), ellers err.
func firstLine(out string, err error) string {
	for _, ln := range strings.Split(out, "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			return ln
		}
	}
	return err.Error()
}

// macPlist bygger LaunchAgent-plisten. RunAtLoad starter den ved login, KeepAlive
// genstarter den hvis den dør.
func macPlist(cliPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>daemon</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
`, autostartLabel, cliPath)
}

// linuxUnit bygger systemd --user-unitten. WantedBy=default.target gør at den
// startes ved login når den er enabled — svarende til Windows-opgavens
// LogonTrigger. Genstartspolitikken spejler Windows-opgavens 30s-forsinkelse +
// RestartOnFailure: lige efter login er netværket og OS-keyringen ofte ikke
// klar endnu, så daemonen fejler og skal prøve igen om lidt.
func linuxUnit(cliPath string) string {
	return fmt.Sprintf(`[Unit]
Description=KeePass Delta-Sync baggrunds-synkronisering
# Ingen ordering mod network-online.target: det target findes ikke i --user-
# manageren og ignoreres. I stedet lader vi Restart klare "netværk/keyring ikke
# klar endnu ved login" (se [Service]). StartLimitIntervalSec=0 slår systemd's
# start-grænse fra, så Restart=always kan blive ved med at prøve — ellers brænder
# den standard-grænsen på 5 forsøg/10s af på få sekunder og ender død (så kører
# der ingen proces efter genstart, mens filen stadig ligger der).
StartLimitIntervalSec=0

[Service]
ExecStart=%s daemon
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
`, cliPath)
}
