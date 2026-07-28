package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vmsilvamolina/yaktui/internal/addons"
)

func withXDGConfigHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestDefaultPathUsesXDGConfigHome(t *testing.T) {
	withXDGConfigHome(t, "/tmp/xdg-test")
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/xdg-test", "yaktui", "config.yaml")
	if path != want {
		t.Fatalf("DefaultPath() = %q, want %q", path, want)
	}
}

func TestDefaultPathFallsBackToHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "yaktui", "config.yaml")
	if path != want {
		t.Fatalf("DefaultPath() = %q, want %q", path, want)
	}
}

func TestLoadAddonDefinitionsNoFile(t *testing.T) {
	withXDGConfigHome(t, t.TempDir())

	defs, err := LoadAddonDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != len(addons.Builtins) {
		t.Fatalf("expected %d builtin defs, got %d", len(addons.Builtins), len(defs))
	}
}

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	confDir := filepath.Join(dir, "yaktui")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAddonDefinitionsOverridesBuiltin(t *testing.T) {
	dir := t.TempDir()
	withXDGConfigHome(t, dir)
	writeConfig(t, dir, `
addons:
  - name: kyverno
    displayName: Kyverno
    enabled: "false"
    group: kyverno.io
    version: v1
    resources:
      - kind: ClusterPolicy
        resource: clusterpolicies
        clusterScoped: true
      - kind: Policy
        resource: policies
        clusterScoped: false
`)

	defs, err := LoadAddonDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != len(addons.Builtins) {
		t.Fatalf("expected override to keep same count %d, got %d", len(addons.Builtins), len(defs))
	}
	found := false
	for _, d := range defs {
		if d.Name == "kyverno" {
			found = true
			if d.Enabled != "false" {
				t.Fatalf("expected overridden enabled=false, got %q", d.Enabled)
			}
		}
	}
	if !found {
		t.Fatal("expected kyverno to still be present")
	}
}

func TestLoadAddonDefinitionsAddsNewEntry(t *testing.T) {
	dir := t.TempDir()
	withXDGConfigHome(t, dir)
	writeConfig(t, dir, `
addons:
  - name: cert-manager-certificates
    displayName: Certificates
    enabled: auto
    group: cert-manager.io
    version: v1
    resources:
      - kind: Certificate
        resource: certificates
        clusterScoped: false
`)

	defs, err := LoadAddonDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != len(addons.Builtins)+1 {
		t.Fatalf("expected %d defs, got %d", len(addons.Builtins)+1, len(defs))
	}
}

func TestLoadAddonDefinitionsMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	withXDGConfigHome(t, dir)
	writeConfig(t, dir, "addons: [this is not valid: yaml: at all:")

	_, err := LoadAddonDefinitions()
	if err == nil {
		t.Fatal("expected error for malformed yaml")
	}
}
