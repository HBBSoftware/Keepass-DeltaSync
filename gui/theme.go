// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Tema-tilstande gemt i gui.json (settings.Theme).
const (
	themeSystem = "system" // følg OS'ets lyse/mørke indstilling (default)
	themeLight  = "light"
	themeDark   = "dark"
)

// forcedVariantTheme pakker standard-temaet, men tvinger en bestemt variant
// (lys/mørk) uanset hvad OS'et er sat til. Fyne følger normalt OS'et; det her
// lader brugeren overstyre det fra Indstillinger.
type forcedVariantTheme struct {
	base    fyne.Theme
	variant fyne.ThemeVariant
}

func (t forcedVariantTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.base.Color(n, t.variant)
}
func (t forcedVariantTheme) Font(s fyne.TextStyle) fyne.Resource     { return t.base.Font(s) }
func (t forcedVariantTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return t.base.Icon(n) }
func (t forcedVariantTheme) Size(n fyne.ThemeSizeName) float32       { return t.base.Size(n) }

// applyTheme sætter app-temaet ud fra den valgte tilstand. "system" (eller en
// ukendt/tom værdi) bruger standard-temaet, der selv følger OS'et.
func applyTheme(app fyne.App, mode string) {
	base := theme.DefaultTheme()
	switch mode {
	case themeLight:
		app.Settings().SetTheme(forcedVariantTheme{base: base, variant: theme.VariantLight})
	case themeDark:
		app.Settings().SetTheme(forcedVariantTheme{base: base, variant: theme.VariantDark})
	default:
		app.Settings().SetTheme(base)
	}
}
