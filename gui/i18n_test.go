// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"reflect"
	"strings"
	"testing"
)

// Et glemt felt i en ordbog bliver til den tomme streng, og Go siger ingenting:
// i UI'en viser det sig som en knap uden tekst. Med 205 felter gange fem sprog
// er det ikke noget man opdager ved at kigge, så det fanges her i stedet.
func TestDictsComplete(t *testing.T) {
	for _, l := range langOrder {
		d := dicts[l]
		if d == nil {
			t.Errorf("%s: står i langOrder, men har ingen ordbog", l)
			continue
		}
		v := reflect.ValueOf(*d)
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).String() == "" {
				t.Errorf("%s: feltet %s er tomt", l, v.Type().Field(i).Name)
			}
		}
	}
}

// Sprogvælgeren tegnes fra langOrder. Et sprog der kun står i dicts ville
// derfor være indlæst, men uvælgeligt — og et navn der mangler i langLabels
// ville give en tom linje i vælgeren.
func TestLangOrderCoversDicts(t *testing.T) {
	inOrder := map[lang]bool{}
	for _, l := range langOrder {
		inOrder[l] = true
		if langLabels[l] == "" {
			t.Errorf("%s: mangler et navn i langLabels", l)
		}
	}
	for l := range dicts {
		if !inOrder[l] {
			t.Errorf("%s: har en ordbog, men står ikke i langOrder — den kan ikke vælges", l)
		}
	}
}

// Pladsholderne er positionelle: bytter en oversættelse om på %s og %d, eller
// taber en, får brugeren "%!d(string=...)" i stedet for sit databasenavn.
func TestPlaceholdersMatchEnglish(t *testing.T) {
	en := reflect.ValueOf(*dicts[langEN])
	for _, l := range langOrder {
		if l == langEN {
			continue
		}
		v := reflect.ValueOf(*dicts[l])
		for i := 0; i < v.NumField(); i++ {
			name := v.Type().Field(i).Name
			if want, got := verbs(en.Field(i).String()), verbs(v.Field(i).String()); want != got {
				t.Errorf("%s: feltet %s har pladsholderne %q, engelsk har %q", l, name, got, want)
			}
		}
	}
}

// verbs plukker formateringsverberne ud i den raekkefoelge de staar.
func verbs(s string) string {
	var b strings.Builder
	for i := 0; i < len(s)-1; i++ {
		if s[i] != '%' {
			continue
		}
		switch c := s[i+1]; c {
		case '%':
			i++ // "%%" er et literalt procenttegn, ikke en pladsholder
		case 's', 'd', 'q', 'v', 'f', 't':
			b.WriteByte('%')
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}
