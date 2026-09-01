package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrdersTriggerBeforeService(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huddle.yaml")
	raw := []byte(`resources:
  - id: service
    type: systemd
    name: example.service
    reload_when_changed: [config]
  - id: config
    type: file
    path: /tmp/example.conf
    content: hello
`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Resources[0].ID != "config" || cfg.Resources[1].ID != "service" {
		t.Fatalf("unexpected order: %s, %s", cfg.Resources[0].ID, cfg.Resources[1].ID)
	}
}

func TestLoadRejectsDependencyCycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huddle.yaml")
	raw := []byte(`resources:
  - id: one
    type: systemd
    name: one.service
    requires: [two]
  - id: two
    type: systemd
    name: two.service
    requires: [one]
`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path); err == nil {
		t.Fatal("expected dependency cycle error")
	}
}
