// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

// TestBase64Bytes_RoundTrip verificerer at Base64Bytes serialiseres som
// base64-string i TOML og dekoder tilbage til samme bytes.
func TestBase64Bytes_RoundTrip(t *testing.T) {
	original := Base64Bytes{0x00, 0x01, 0xff, 0xab, 0xcd, 0xef}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(struct {
		Key Base64Bytes `toml:"key"`
	}{Key: original}); err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded struct {
		Key Base64Bytes `toml:"key"`
	}
	if _, err := toml.Decode(buf.String(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal([]byte(decoded.Key), []byte(original)) {
		t.Fatalf("round-trip mismatch: got %x, want %x", decoded.Key, original)
	}
}

// TestBase64Bytes_EmptyOmitted verificerer at en tom Base64Bytes round-trip'er
// til nil — vi vil ikke have tomme device_private_key-felter i config'en på
// disk-form, og en frisk Load skal give nil ikke len(0)-slice.
func TestBase64Bytes_EmptyRoundTripsToNil(t *testing.T) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(struct {
		Key Base64Bytes `toml:"key,omitempty"`
	}{Key: nil}); err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded struct {
		Key Base64Bytes `toml:"key,omitempty"`
	}
	if _, err := toml.Decode(buf.String(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Key != nil {
		t.Fatalf("decoded non-nil from empty: %x", decoded.Key)
	}
}

// TestServer_DevicePrivateKeyRoundTrip verificerer at en Server med privat
// key kan gemmes og loades tilbage via Save/Load.
func TestServer_DevicePrivateKeyRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir()) // Windows fallback for os.UserConfigDir

	priv := Base64Bytes(bytes.Repeat([]byte{0xab}, 32))
	cfg := &Config{
		Server: Server{
			URL:              "https://example.com",
			DeviceToken:      "abc",
			DevicePrivateKey: priv,
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !bytes.Equal([]byte(loaded.Server.DevicePrivateKey), []byte(priv)) {
		t.Fatalf("private key mismatch after round-trip: got %x, want %x",
			loaded.Server.DevicePrivateKey, priv)
	}
}

// TestServer_LegacyConfigLoadsWithoutKey verificerer at en config-fil uden
// device_private_key (= legacy enrolled enhed) loader korrekt med nil-felt.
func TestServer_LegacyConfigLoadsWithoutKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := writeLegacyConfig(t, path); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.URL != "https://legacy.example" {
		t.Fatalf("url not loaded: %q", cfg.Server.URL)
	}
	if cfg.Server.DeviceToken != "legacy-token" {
		t.Fatalf("token not loaded: %q", cfg.Server.DeviceToken)
	}
	if cfg.Server.DevicePrivateKey != nil {
		t.Fatalf("expected nil DevicePrivateKey on legacy config, got %x", cfg.Server.DevicePrivateKey)
	}
}

func writeLegacyConfig(t *testing.T, path string) error {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(`[server]
url = "https://legacy.example"
device_token = "legacy-token"
`), 0o600)
}
