//go:build !windows

package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The version-drift producer decides *whether* to re-probe: the recorded tool
// version against what the executable says now. The reference is
// `src-tauri/src/extensions/install.rs:843-975`
// `reprobe_on_tool_version_change`; the visible half is `driftnotice.go` and is
// tested there. These tests pin the eligibility gate, the "no version, no
// probe" rule, the version comparison, and the fail-soft contract.

// driftFixture installs a generated custom integration whose executable
// reports `version` on `--version`, and whose recorded `tool_version` is
// `recorded`. It reuses `reprobeFixture`'s package shape, then adds the
// version probe and the recorded version.
func driftFixture(t *testing.T, version, recorded string) (Paths, string) {
	t.Helper()
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "tool ` + version + `"
  exit 0
fi
if [ "$1" = "--help" ]; then
  echo "tool - Gadgets"
  echo "Available Plugins"
  echo "📦 one 1.0"
  echo "📦 two 1.0"
  exit 0
fi
exit 3
`
	paths, _ := reprobeFixture(t, script)
	// The reference's eligibility gate requires a *system* runtime; the
	// reprobe fixture leaves ownership unset, so this records it.
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		t.Fatal(err)
	}
	entry := repo.Extensions["dev.floter.tool"]
	entry.RuntimeOwnership = RuntimeOwnershipSystem
	repo.Extensions["dev.floter.tool"] = entry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		t.Fatal(err)
	}
	// The fixture's manifest carries no versionArgs; the drift probe runs the
	// same default `--version` the connect-time probe used.
	if recorded != "" {
		if err := SetToolVersion(paths, "dev.floter.tool", recorded); err != nil {
			t.Fatal(err)
		}
	}
	// A first derivation, so a later re-probe has a previous count to report.
	if _, err := ReprobeCommands(context.Background(), paths, "dev.floter.tool"); err != nil {
		t.Fatal(err)
	}
	return paths, "dev.floter.tool"
}

func TestCheckToolVersionDriftReprobesWhenTheVersionMoved(t *testing.T) {
	paths, id := driftFixture(t, "2.0.0", "1.0.0")
	// A sidecar count from before, so the report's previous side is real.
	before := readHelpProbeRecord(filepath.Join(paths.Extensions, id, "integration"))
	if before == nil || before.CommandCount == nil || *before.CommandCount != 3 {
		t.Fatalf("sidecar = %+v", before)
	}
	result, err := CheckToolVersionDrift(context.Background(), paths, id)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !result.ReprobeRan {
		t.Fatal("a moved version did not re-probe")
	}
	if result.ToolVersion != "2.0.0" {
		t.Errorf("tool version = %q, want 2.0.0", result.ToolVersion)
	}
	if result.Report.CommandCount != 3 {
		t.Errorf("report = %+v", result.Report)
	}
	if result.Report.PreviousCommandCount == nil || *result.Report.PreviousCommandCount != 3 {
		t.Errorf("previous = %v, want 3", result.Report.PreviousCommandCount)
	}
	// The recorded version moved with the probe, so the next listing is a
	// no-op (idempotent).
	entry := LoadInventory(paths).Integrations[0]
	if entry.Entry.ToolVersion == nil || *entry.Entry.ToolVersion != "2.0.0" {
		t.Errorf("recorded version = %v, want 2.0.0", entry.Entry.ToolVersion)
	}
	second, err := CheckToolVersionDrift(context.Background(), paths, id)
	if err != nil {
		t.Fatal(err)
	}
	if second.ReprobeRan {
		t.Error("the same version drift was probed twice")
	}
}

// A tool that reports no version is never a drift: there is nothing to
// compare against, so the probe must stay off rather than fire on every
// listing.
func TestCheckToolVersionDriftStaysQuietWithoutAVersion(t *testing.T) {
	script := `#!/bin/sh
if [ "$1" = "--help" ]; then
  echo "tool - Gadgets"
  echo "Available Plugins"
  echo "📦 one 1.0"
  exit 0
fi
exit 3
`
	paths, _ := reprobeFixture(t, script)
	if err := SetToolVersion(paths, "dev.floter.tool", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	result, err := CheckToolVersionDrift(context.Background(), paths, "dev.floter.tool")
	if err != nil {
		t.Fatal(err)
	}
	if result.ReprobeRan {
		t.Error("a tool that reports no version re-probed")
	}
	if result.ToolVersion != "" {
		t.Errorf("tool version = %q, want empty", result.ToolVersion)
	}
}

// An unchanged version is idempotent: the routine returns without probing.
func TestCheckToolVersionDriftIgnoresAnUnchangedVersion(t *testing.T) {
	paths, id := driftFixture(t, "1.0.0", "1.0.0")
	result, err := CheckToolVersionDrift(context.Background(), paths, id)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReprobeRan {
		t.Error("an unchanged version re-probed")
	}
}

// A script integration is never version-drift probed: its command list is not
// derived from a system executable the upstream can replace.
func TestCheckToolVersionDriftRefusesAScriptIntegration(t *testing.T) {
	paths, id := driftFixture(t, "2.0.0", "1.0.0")
	// Flip the recorded runtime ownership to bundled: the drift gate requires
	// a system runtime.
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		t.Fatal(err)
	}
	entry := repo.Extensions[id]
	entry.RuntimeOwnership = "bundled"
	repo.Extensions[id] = entry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		t.Fatal(err)
	}
	result, err := CheckToolVersionDrift(context.Background(), paths, id)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReprobeRan {
		t.Error("a bundled integration was version-drift probed")
	}
}

// A publisher's integration is never version-drift probed: the host would
// overwrite shipped content with its own derivation.
func TestCheckToolVersionDriftRefusesAPublisherIntegration(t *testing.T) {
	paths, id := driftFixture(t, "2.0.0", "1.0.0")
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		t.Fatal(err)
	}
	entry := repo.Extensions[id]
	entry.PublisherID = "vst"
	repo.Extensions[id] = entry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		t.Fatal(err)
	}
	result, err := CheckToolVersionDrift(context.Background(), paths, id)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReprobeRan {
		t.Error("a publisher integration was version-drift probed")
	}
	// And the shipped descriptor is untouched.
	data, readErr := os.ReadFile(filepath.Join(paths.Extensions, id, "integration", "provider-description.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	descriptor, parseErr := ParseDescription(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if len(descriptor.Commands) != 3 {
		t.Errorf("the shipped descriptor changed: %+v", descriptor.Commands)
	}
}

// A tool that moved away is not a command drift: the binding/reconnect path
// owns a missing executable, and this routine must stay quiet.
func TestCheckToolVersionDriftIgnoresAMissingExecutable(t *testing.T) {
	paths, id := driftFixture(t, "2.0.0", "1.0.0")
	executable := filepath.Join(paths.Extensions, id, "integration", "tool.sh")
	if err := os.Remove(executable); err != nil {
		t.Fatal(err)
	}
	result, err := CheckToolVersionDrift(context.Background(), paths, id)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReprobeRan {
		t.Error("a vanished executable was version-drift probed")
	}
}

// A disabled integration is not probed: the reference's `drift_reprobe_eligible`
// requires `entry.enabled`.
func TestCheckToolVersionDriftRefusesADisabledIntegration(t *testing.T) {
	paths, id := driftFixture(t, "2.0.0", "1.0.0")
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		t.Fatal(err)
	}
	entry := repo.Extensions[id]
	entry.Enabled = false
	entry.State = "disabled"
	repo.Extensions[id] = entry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		t.Fatal(err)
	}
	result, err := CheckToolVersionDrift(context.Background(), paths, id)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReprobeRan {
		t.Error("a disabled integration was version-drift probed")
	}
}
