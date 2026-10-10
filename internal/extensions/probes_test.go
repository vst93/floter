package extensions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// probesFixture installs an integration whose manifest declares two probes:
// one required and one optional, run through a shell script that reports
// which arguments it saw.
func probesFixture(t *testing.T, requiredStatus, optionalStatus string) (Paths, string) {
	t.Helper()
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(paths.Extensions, "dev.floter.probed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.probed", "name": "Probed",
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]},
	  "lifecycle": {"probes": [
	    {"id": "version", "args": ["--version"], "required": true, "timeoutMs": 5000},
	    {"id": "completion", "args": ["--completion"], "required": false, "timeoutMs": 5000}
	  ]}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  --version) exit " + requiredStatus + " ;;\n" +
		"  --completion) echo boom >&2; exit " + optionalStatus + " ;;\n" +
		"esac\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "tool.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.probed": {"id": "dev.floter.probed", "name": "Probed",
	    "state": "enabled", "enabled": true, "packageVersion": "1.0.0",
	    "manifestPath": "`+filepath.Join(dir, manifestFileName)+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return paths, dir
}

func TestReprobeAllPassIsHealthy(t *testing.T) {
	paths, _ := probesFixture(t, "0", "0")
	report, err := Reprobe(context.Background(), paths, "dev.floter.probed")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != HealthHealthy || len(report.Probes) != 2 || report.CheckedAt == "" {
		t.Fatalf("report = %+v", report)
	}
	for _, record := range report.Probes {
		if !record.Passed || record.ExitCode != 0 {
			t.Errorf("probe = %+v", record)
		}
	}
	// The report is persisted on the entry, under the old build's field.
	inventory := LoadInventory(paths)
	integration, ok := inventory.WithID("dev.floter.probed")
	if !ok || integration.Entry.ProbeReport == nil {
		t.Fatalf("entry = %+v", integration.Entry)
	}
	if integration.Entry.ProbeReport.Status != HealthHealthy {
		t.Errorf("persisted status = %q", integration.Entry.ProbeReport.Status)
	}
}

func TestReprobeRequiredFailIsUnhealthyAndBroken(t *testing.T) {
	paths, _ := probesFixture(t, "3", "0")
	report, err := Reprobe(context.Background(), paths, "dev.floter.probed")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != HealthUnhealthy {
		t.Fatalf("status = %q", report.Status)
	}
	// The entry is broken, with the reason recorded.
	inventory := LoadInventory(paths)
	integration, _ := inventory.WithID("dev.floter.probed")
	if integration.Entry.State != "broken" {
		t.Fatalf("entry = %+v", integration.Entry)
	}
	if integration.Entry.BrokenReason == nil || !strings.Contains(*integration.Entry.BrokenReason, "lifecycle probe") {
		t.Errorf("reason = %+v", integration.Entry.BrokenReason)
	}
	if integration.Entry.LastErrorCode == nil || *integration.Entry.LastErrorCode != "probe_failed" {
		t.Errorf("error code = %+v", integration.Entry.LastErrorCode)
	}
	// The command contributes nothing while the integration is broken.
	store := OpenStore(paths)
	if got := store.CommandEntries(); len(got) != 0 {
		t.Errorf("a broken integration contributed commands: %+v", got)
	}
}

func TestReprobeOptionalFailIsDegraded(t *testing.T) {
	paths, _ := probesFixture(t, "0", "7")
	report, err := Reprobe(context.Background(), paths, "dev.floter.probed")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != HealthDegraded {
		t.Fatalf("status = %q", report.Status)
	}
	// The optional probe's stderr is captured and capped.
	var completion ProbeRecord
	for _, record := range report.Probes {
		if record.ID == "completion" {
			completion = record
		}
	}
	if completion.Passed || !strings.Contains(completion.Stderr, "boom") {
		t.Errorf("completion = %+v", completion)
	}
	// A degraded integration still contributes commands.
	inventory := LoadInventory(paths)
	integration, _ := inventory.WithID("dev.floter.probed")
	if integration.Entry.State != "enabled" {
		t.Errorf("a degraded integration was marked broken: %+v", integration.Entry)
	}
}

func TestReprobeRefusesWhatItCannotProbe(t *testing.T) {
	paths, dir := probesFixture(t, "0", "0")
	// A probe that hangs is killed by its own timeout.
	if err := os.WriteFile(filepath.Join(dir, "tool.sh"), []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	report, err := Reprobe(context.Background(), paths, "dev.floter.probed")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != HealthUnhealthy {
		t.Fatalf("a hung probe = %+v", report)
	}
	for _, record := range report.Probes {
		if record.DurationMs > 15_000 {
			t.Errorf("the timeout did not hold: %+v", record)
		}
	}

	// No declared probes proves nothing and changes nothing.
	bare := FromRoot(t.TempDir())
	if err := bare.Ensure(); err != nil {
		t.Fatal(err)
	}
	bareDir := filepath.Join(bare.Extensions, "dev.floter.bare")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bareDir, manifestFileName), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.bare", "name": "Bare",
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bareDir, "tool.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bare.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.bare": {"id": "dev.floter.bare", "name": "Bare",
	    "state": "enabled", "enabled": true, "manifestPath": "`+filepath.Join(bareDir, manifestFileName)+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Reprobe(context.Background(), bare, "dev.floter.bare"); err != ErrNoProbes {
		t.Errorf("a bare lifecycle = %v", err)
	}
	// An unknown id and a swapped manifest are refused.
	if _, err := Reprobe(context.Background(), bare, "dev.floter.gone"); err != ErrNoIntegration {
		t.Errorf("an unknown id = %v", err)
	}
}
