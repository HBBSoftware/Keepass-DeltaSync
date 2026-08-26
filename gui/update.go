// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Opdateringstjek mod GitLab Releases. GUI'ens egen version kommer fra
// FyneApp.toml (Fyne embedder den i app-metadata), og udgivelserne ligger
// allerede som gui/vX.Y.Z-tags i projektets Releases. Vi henter listen, finder
// det hoejeste gui-tag og sammenligner.
//
// Tjekket er bevidst tavst: enhver fejl — offline, DNS, rate limit, aendret
// API — betyder bare "ingen banner". Et adgangskodevaerktoej skal ikke brokke
// sig over netvaerket, og brugeren kan slaa tjekket helt fra i Indstillinger.
const (
	releasesAPIURL  = "https://gitlab.com/api/v4/projects/Star95%2Fkeepass-deltasync/releases?per_page=50"
	releasesPageURL = "https://gitlab.com/Star95/keepass-deltasync/-/releases"
	guiTagPrefix    = "gui/v"
)

// glRelease er den ene felt vi bruger af GitLabs release-objekt.
type glRelease struct {
	TagName string `json:"tag_name"`
}

// currentGUIVersion laeser versionen Fyne har embeddet fra FyneApp.toml.
// Tom streng ved udviklingsbyg uden metadata — saa springer vi tjekket over.
func currentGUIVersion() string {
	app := fyne.CurrentApp()
	if app == nil {
		return ""
	}
	return strings.TrimSpace(app.Metadata().Version)
}

// latestGUIRelease henter den nyeste gui/vX.Y.Z fra GitLab. Returnerer den
// rene version ("0.3.5") uden tag-praefiks.
func latestGUIRelease(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPIURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gitlab releases: http %d", resp.StatusCode)
	}

	var rels []glRelease
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		return "", err
	}

	tags := make([]string, 0, len(rels))
	for _, r := range rels {
		tags = append(tags, r.TagName)
	}
	best := pickLatestGUITag(tags)
	if best == "" {
		return "", fmt.Errorf("no %s* release found", guiTagPrefix)
	}
	return best, nil
}

// pickLatestGUITag vaelger den hoejeste gui/vX.Y.Z blandt alle projektets tags.
// Releases-listen indeholder ogsaa client/, android/ og extension/-tags, og de
// skal ikke kunne udloese et GUI-opdateringsbanner.
func pickLatestGUITag(tags []string) string {
	var best [3]int
	var bestStr string
	for _, tag := range tags {
		if !strings.HasPrefix(tag, guiTagPrefix) {
			continue
		}
		raw := strings.TrimPrefix(tag, guiTagPrefix)
		v, ok := parseSemver(raw)
		if !ok {
			continue
		}
		if bestStr == "" || semverLess(best, v) {
			best, bestStr = v, raw
		}
	}
	return bestStr
}

// parseSemver laeser "1.2.3". Manglende led taelles som 0, saa "0.3" virker.
func parseSemver(s string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(s), "v"), ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

func semverLess(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// updateAvailable sammenligner to versionsstrenge. Kan en af dem ikke laeses,
// siger vi nej — hellere intet banner end et falsk et.
func updateAvailable(current, latest string) bool {
	c, okC := parseSemver(current)
	l, okL := parseSemver(latest)
	if !okC || !okL {
		return false
	}
	return semverLess(c, l)
}

// checkForUpdate spoerger GitLab i baggrunden og viser banneret hvis der findes
// en nyere udgivelse. Alt gaar tavst galt: er tjekket slaaet fra, mangler
// versionen, eller svarer nettet ikke, sker der ganske enkelt ingenting.
func (u *ui) checkForUpdate() {
	if !u.set.updateCheckEnabled() {
		return
	}
	current := currentGUIVersion()
	if current == "" {
		debugf("update check: no embedded version, skipping")
		return
	}
	u.async(func() any {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		latest, err := latestGUIRelease(ctx)
		if err != nil {
			debugf("update check: %v", err)
			return ""
		}
		return latest
	}, func(v any) {
		latest, _ := v.(string)
		if latest == "" || !updateAvailable(current, latest) {
			return
		}
		debugf("update check: %s available (running %s)", latest, current)
		u.showUpdateBanner(current, latest)
	})
}

// showUpdateBanner fylder linjen oeverst i vinduet: en kort besked og en knap
// der aabner projektets udgivelsesside i browseren.
func (u *ui) showUpdateBanner(current, latest string) {
	if u.updateBar == nil {
		return
	}
	msg := widget.NewLabel(fmt.Sprintf(L.UpdateAvailable, latest, current))
	msg.Wrapping = fyne.TextWrapWord

	btn := widget.NewButtonWithIcon(L.UpdateDownload, theme.DownloadIcon(), func() {
		link, err := url.Parse(releasesPageURL)
		if err != nil {
			return
		}
		_ = u.fApp.OpenURL(link)
	})
	btn.Importance = widget.HighImportance

	u.updateBar.Objects = []fyne.CanvasObject{
		container.NewBorder(nil, nil, widget.NewIcon(theme.InfoIcon()), btn, msg),
		widget.NewSeparator(),
	}
	u.updateBar.Refresh()
	u.updateBar.Show()
}
