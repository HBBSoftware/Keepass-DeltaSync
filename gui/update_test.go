// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "testing"

func TestPickLatestGUITag(t *testing.T) {
	tags := []string{
		"client/v1.4.0",
		"gui/v0.3.2",
		"android/v9.9.9",
		"gui/v0.3.10",
		"extension/v0.2.1",
		"gui/v0.3.4",
		"gui/vnonsense",
	}
	if got := pickLatestGUITag(tags); got != "0.3.10" {
		t.Fatalf("valgte %q, ville have 0.3.10", got)
	}
	if got := pickLatestGUITag([]string{"client/v1.4.0"}); got != "" {
		t.Fatalf("ingen gui-tags, men fik %q", got)
	}
}

func TestUpdateAvailable(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"0.3.4", "0.3.5", true},
		{"0.3.4", "0.3.4", false},
		{"0.3.5", "0.3.4", false},
		{"0.3.9", "0.3.10", true},
		{"0.9.0", "1.0.0", true},
		{"", "0.3.5", false},
		{"0.3.4", "", false},
		{"0.3.4", "junk", false},
	}
	for _, c := range cases {
		if got := updateAvailable(c.current, c.latest); got != c.want {
			t.Errorf("updateAvailable(%q, %q) = %v, ville have %v", c.current, c.latest, got, c.want)
		}
	}
}
