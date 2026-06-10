// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"gitlab.com/Star95/keepass-deltasync/client/internal/config"
)

func TestResolveDeleteTarget(t *testing.T) {
	const boundUUID = "11111111-2222-3333-4444-555555555555"
	const rawUUID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	cfg := &config.Config{
		Databases: []config.Database{
			{Name: "work", RemoteID: boundUUID},
		},
	}

	tests := []struct {
		name    string
		target  string
		want    string
		wantErr bool
	}{
		{name: "local name resolves to RemoteID", target: "work", want: boundUUID},
		{name: "raw UUID passes through", target: rawUUID, want: rawUUID},
		{name: "unknown name is an error", target: "nope", wantErr: true},
		{name: "non-UUID garbage is an error", target: "not-a-uuid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDeleteTarget(cfg, tt.target)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil (result %q)", tt.target, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.target, err)
			}
			if got != tt.want {
				t.Fatalf("resolveDeleteTarget(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}
