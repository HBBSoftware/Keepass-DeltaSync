// SPDX-License-Identifier: GPL-3.0-or-later

// Kommando genicon bygger app-ikonet (icon.png) ud fra projektets fælles ikon —
// samme guldnøgle-med-sync-pile som Android-klienten bruger. De to lag
// (baggrund-gradient + forgrund-motiv) er kopieret fra klientens adaptive icon
// og indlejret her med go:embed, så værktøjet er selvstændigt og reproducerbart.
//
// Lagene komponeres oven på hinanden, og hjørnerne rundes let, så det ligner et
// rigtigt desktop-app-ikon. Kør:
//
//	go run ./tools/genicon
package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
)

//go:embed layers/background.png
var backgroundPNG []byte

//go:embed layers/foreground.png
var foregroundPNG []byte

const (
	out    = "icon.png"
	corner = 0.18 // hjørneradius som andel af kantlængden
)

func main() {
	bg := decode(backgroundPNG)
	fg := decode(foregroundPNG)

	b := bg.Bounds()
	canvas := image.NewRGBA(b)

	// Komponér: baggrund først, så forgrunden ovenpå (alpha-over).
	draw.Draw(canvas, b, bg, b.Min, draw.Src)
	draw.Draw(canvas, b, fg, fg.Bounds().Min, draw.Over)

	// Rund hjørnerne med blød kant (antialiasing via 1px dækningsovergang).
	w, h := b.Dx(), b.Dy()
	rad := math.Min(float64(w), float64(h)) * corner
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cov := coverage(float64(x)+0.5, float64(y)+0.5, 0, 0, float64(w), float64(h), rad)
			if cov >= 1 {
				continue
			}
			c := canvas.RGBAAt(b.Min.X+x, b.Min.Y+y)
			c.R = uint8(float64(c.R) * cov)
			c.G = uint8(float64(c.G) * cov)
			c.B = uint8(float64(c.B) * cov)
			c.A = uint8(float64(c.A) * cov)
			canvas.SetRGBA(b.Min.X+x, b.Min.Y+y, c)
		}
	}

	f, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, canvas); err != nil {
		panic(err)
	}
}

func decode(data []byte) image.Image {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	return img
}

// coverage returnerer hvor stor en del af pixlen (0..1) der ligger inden for den
// afrundede firkant — 1 helt inde, 0 helt ude, blødt over en pixels bredde i
// kanten.
func coverage(px, py, x0, y0, x1, y1, rad float64) float64 {
	cx := clamp(px, x0+rad, x1-rad)
	cy := clamp(py, y0+rad, y1-rad)
	d := math.Hypot(px-cx, py-cy)
	return clamp01(rad + 0.5 - d)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clamp01(v float64) float64 { return clamp(v, 0, 1) }
