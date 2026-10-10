package extensions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capabilityFixture installs an integration whose tool reports its version
// and answers --help.
func capabilityFixture(t *testing.T, script string) (Paths, Integration) {
	t.Helper()
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(paths.Extensions, "dev.floter.caps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.caps", "name": "Caps",
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tool.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.caps": {"id": "dev.floter.caps", "name": "Caps",
	    "state": "enabled", "enabled": true, "manifestPath": "`+filepath.Join(dir, manifestFileName)+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	inventory := LoadInventory(paths)
	integration, ok := inventory.WithID("dev.floter.caps")
	if !ok {
		t.Fatal("the integration did not load")
	}
	return paths, integration
}

func TestProbeCapabilitiesAllPass(t *testing.T) {
	_, integration := capabilityFixture(t, `#!/bin/sh
case "$1" in
  --version) echo "caps 1.2.3" ;;
  --help) echo "usage: caps" ;;
esac
`)
	report, err := ProbeCapabilities(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != "caps 1.2.3" {
		t.Errorf("version = %q", report.Version)
	}
	if len(report.SupportedFeatures) != 2 {
		t.Fatalf("features = %v", report.SupportedFeatures)
	}
	if report.SupportedFeatures[0] != ProbeVersion || report.SupportedFeatures[1] != ProbeHelp {
		t.Errorf("features = %v", report.SupportedFeatures)
	}
	if len(report.Limitations) != 0 {
		t.Errorf("limitations = %v", report.Limitations)
	}
}

func TestProbeCapabilitiesRecordsLimitations(t *testing.T) {
	_, integration := capabilityFixture(t, `#!/bin/sh
case "$1" in
  --version) echo "caps 1.2.3" ;;
  --help) echo "usage" >&2; exit 2 ;;
esac
`)
	report, err := ProbeCapabilities(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	// The version probe passed; the help probe failed with exit code 2.
	if report.Version != "caps 1.2.3" {
		t.Errorf("version = %q", report.Version)
	}
	if len(report.SupportedFeatures) != 1 || report.SupportedFeatures[0] != ProbeVersion {
		t.Errorf("features = %v", report.SupportedFeatures)
	}
	if len(report.Limitations) != 1 || !strings.Contains(report.Limitations[0], "help") {
		t.Errorf("limitations = %v", report.Limitations)
	}
}

func TestProbeCapabilitiesVersionFailsIsUnknown(t *testing.T) {
	_, integration := capabilityFixture(t, `#!/bin/sh
case "$1" in
  --version) exit 1 ;;
  --help) echo "usage" ;;
esac
`)
	report, err := ProbeCapabilities(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != UnknownVersion {
		t.Errorf("version = %q, want unknown", report.Version)
	}
	// The help probe still passed and is listed.
	found := false
	for _, feature := range report.SupportedFeatures {
		if feature == ProbeHelp {
			found = true
		}
	}
	if !found {
		t.Errorf("features = %v", report.SupportedFeatures)
	}
}

func TestProbeCapabilitiesTimesOut(t *testing.T) {
	_, integration := capabilityFixture(t, `#!/bin/sh
sleep 30
`)
	report, err := ProbeCapabilities(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != UnknownVersion {
		t.Errorf("version = %q", report.Version)
	}
	if len(report.Limitations) == 0 {
		t.Error("no limitations were recorded for a hung probe")
	}
}
