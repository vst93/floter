package shell

import (
	"context"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/settingsui"
)

// These are inventory refresh events, not preconstructed memo keys. The
// direct Store.Refresh is also used by diagnosis, repair and reconnect.
func checkFactsIdentityRefresh(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			const id = "dev.floter.identity"
			paths := extensions.FromRoot(t.TempDir())
			if err := paths.Ensure(); err != nil {
				t.Fatal(err)
			}
			pkg := filepath.Join(paths.Extensions, id)
			if err := os.MkdirAll(pkg, 0o755); err != nil {
				t.Fatal(err)
			}
			program, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			manifest := extensions.Manifest{SchemaVersion: "2.0", ID: id, Name: "声明刷新工具"}
			manifest.Distribution.Type = "local"
			manifest.Runtime = extensions.Runtime{Type: "system", ExecutableNames: []string{"floter-no-such-identity-runtime"}, Language: "floter-no-such-language", Path: "tool.sh"}
			manifest.Provider.Type, manifest.Provider.Descriptor = "static-descriptor", "commands.json"
			entry := extensions.Entry{ID: id, Name: manifest.Name, Enabled: true, State: "enabled", ManifestPath: filepath.Join(pkg, "floter.extension.json"), ExecutablePath: program}
			now := time.Now()
			entry.ProbeReport = &extensions.HealthReport{Status: extensions.HealthHealthy, CheckedAt: now.Add(-48 * time.Hour).UTC().Format(time.RFC3339)}
			write := func(path string, value any) {
				t.Helper()
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			writePackage := func(dir string) {
				t.Helper()
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				write(filepath.Join(dir, "floter.extension.json"), manifest)
				if err := os.WriteFile(filepath.Join(dir, "commands.json"), []byte(`{"protocolVersion":"1.0","provider":{"id":"dev.floter.identity","name":"声明刷新工具","version":"1.0"},"commands":[]}`), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			writeRepo := func() {
				t.Helper()
				write(paths.RepositoryFile, map[string]any{"schemaVersion": 1, "extensions": map[string]extensions.Entry{id: entry}})
			}
			switch scenario {
			case "runtime-type", "runtime-language":
				manifest.Runtime.Type = "script"
			case "runtime-names":
				entry.ExecutablePath = ""
			}
			writePackage(pkg)
			writeRepo()
			if scenario == "row-path" {
				write(filepath.Join(pkg, "help-probe.json"), map[string]any{"probedAt": now.Unix(), "commandCount": 2, "previousCommandCount": 2})
			}
			s := settings.Default()
			s.Language, s.Theme = "zh", map[bool]string{false: "light", true: "dark"}[dark]
			a := New(Options{Store: settings.NewStore(s), Paths: paths})
			a.Integrations.Refresh(context.Background())
			invalidateIntegrationFacts()
			t.Cleanup(invalidateIntegrationFacts)
			// Freeze only the clock, not the inventory: the next frame must use
			// identity invalidation and cannot pass by waiting for the 2s TTL.
			a.now = func() time.Time { return now }
			rows := a.integrationList()
			if len(rows) != 1 {
				t.Fatalf("fixture rows=%d", len(rows))
			}
			beforeAvailable := rows[0].RuntimeAvailable
			beforeFreshness, _ := a.integrationFreshness(id)
			rowOnly := scenario == "row-path" || scenario == "row-health"
			if rowOnly {
				// Exercise the supported individual-row Freshness callback without
				// warming it through a full-list probe first.
				a.Settings.Integrations = func() []settingsui.Integration { return rows }
			}
			tt := transferCauseFrame(t, a, dark)
			switch scenario {
			case "runtime-type":
				manifest.Runtime.Type = "system"
			case "runtime-language":
				manifest.Runtime.Language = "shell"
				if runtime.GOOS == "windows" {
					manifest.Runtime.Language = "powershell"
				}
			case "runtime-names":
				manifest.Runtime.ExecutableNames = extensions.InterpreterNames("shell")
				if runtime.GOOS == "windows" {
					manifest.Runtime.ExecutableNames = extensions.InterpreterNames("powershell")
				}
			case "runtime-platform":
				manifest.Platforms = []string{"windows"}
				if runtime.GOOS == "windows" {
					manifest.Platforms = []string{"linux"}
				}
			case "health-result", "row-health":
				entry.ProbeReport.Status = extensions.HealthUnhealthy
			case "health-time":
				entry.ProbeReport.CheckedAt = now.UTC().Format(time.RFC3339)
			case "row-path":
				pkg = filepath.Join(t.TempDir(), "moved")
				entry.ManifestPath = filepath.Join(pkg, "floter.extension.json")
			default:
				t.Fatalf("unknown scenario %q", scenario)
			}
			writePackage(pkg)
			writeRepo()
			if scenario == "row-path" {
				write(filepath.Join(pkg, "help-probe.json"), map[string]any{"probedAt": now.Unix(), "commandCount": 9, "previousCommandCount": 2})
			}
			a.Integrations.Refresh(context.Background())
			current, ok := a.Integrations.Inventory().WithID(id)
			if !ok {
				t.Fatal("refresh removed the target")
			}
			_, runtimeErr := extensions.ResolveRuntime(current)
			wantAvailable := runtimeErr == nil
			wantFreshness := extensions.FreshnessOf(paths, current)
			tt.Frame()
			tt.Frame()
			if tag := os.Getenv("FLOTER_P13JM_DUMP"); tag != "" {
				file, err := os.Create("/tmp/floter-P13-jm-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(file, tt.Image()); err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			copy := i18n.For("zh").Settings
			switch scenario {
			case "runtime-type", "runtime-language", "runtime-names", "runtime-platform":
				if beforeAvailable == wantAvailable {
					t.Fatalf("fixture did not change runtime availability: %v (%v)", wantAvailable, runtimeErr)
				}
				got := a.integrationList()[0]
				if got.RuntimeAvailable != wantAvailable {
					t.Errorf("%s: refreshed declaration kept obsolete runtime badge: got=%v want=%v", scenario, got.RuntimeAvailable, wantAvailable)
				}
				if tt.HasText(copy.RuntimeUnavailable) == wantAvailable {
					t.Errorf("%s: actual card paints obsolete unavailable badge", scenario)
				}
			case "health-result", "row-health":
				got, ok := a.integrationFreshness(id)
				if !ok || got.Result != wantFreshness.Result || !tt.HasText(copy.FreshnessResultFailed) {
					t.Errorf("%s: refreshed report still paints old result: got=%s want=%s texts=%q", scenario, got.Result, wantFreshness.Result, tt.Texts())
				}
			case "health-time":
				got, ok := a.integrationFreshness(id)
				if !ok || got.AtSeconds != wantFreshness.AtSeconds || !tt.HasText(copy.FreshnessJustNow) {
					t.Errorf("health-time: refreshed check still paints old timestamp: got=%d want=%d", got.AtSeconds, wantFreshness.AtSeconds)
				}
			case "row-path":
				got, ok := a.integrationFreshness(id)
				if !ok || got.CommandCount == nil || *got.CommandCount != 9 || !tt.HasText("9") {
					t.Errorf("row-path: new package still paints old command count: got=%+v old=%+v", got, beforeFreshness)
				}
			}
		})
	}
}

func TestFactsIdentityRuntimeTypeRefresh(t *testing.T) { checkFactsIdentityRefresh(t, "runtime-type") }
func TestFactsIdentityRuntimeLanguageRefresh(t *testing.T) {
	checkFactsIdentityRefresh(t, "runtime-language")
}

func TestFactsIdentityRuntimeNamesRefresh(t *testing.T) {
	checkFactsIdentityRefresh(t, "runtime-names")
}
func TestFactsIdentityRuntimePlatformRefresh(t *testing.T) {
	checkFactsIdentityRefresh(t, "runtime-platform")
}

func TestFactsIdentityHealthResultRefresh(t *testing.T) {
	checkFactsIdentityRefresh(t, "health-result")
}
func TestFactsIdentityHealthTimeRefresh(t *testing.T) { checkFactsIdentityRefresh(t, "health-time") }
