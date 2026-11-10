package shell

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

// A portable, real lifecycle process with an explicit start/release handshake.
func TestHealthCompletionProvider(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--floter-health" || i+1 >= len(os.Args) {
			continue
		}
		base := os.Args[i+1]
		if err := os.WriteFile(base+".started", nil, 0o600); err != nil {
			os.Exit(3)
		}
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) {
			if data, err := os.ReadFile(base + ".release"); err == nil {
				code, err := strconv.Atoi(string(data))
				if err != nil {
					os.Exit(4)
				}
				os.Exit(code)
			}
			time.Sleep(time.Millisecond)
		}
		os.Exit(5)
	}
}

type healthCompletion struct {
	report extensions.HealthReport
	err    error
}

func startHealthCompletion(t *testing.T, f *diagnosisFixture, name string) (string, <-chan healthCompletion) {
	t.Helper()
	base := filepath.Join(f.dir, name)
	f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: true, Args: []string{"-test.run=^TestHealthCompletionProvider$", "--", "--floter-health", base}}}
	f.write(t)
	done := make(chan healthCompletion, 1)
	go func() {
		report, err := extensions.Reprobe(context.Background(), f.paths, f.entry.ID)
		done <- healthCompletion{report, err}
	}()
	deadline := time.Now().Add(6 * time.Second)
	for {
		if _, err := os.Stat(base + ".started"); err == nil {
			return base, done
		}
		if time.Now().After(deadline) {
			t.Fatal("health subprocess never started")
		}
		time.Sleep(time.Millisecond)
	}
}

func finishHealthCompletion(t *testing.T, base string, done <-chan healthCompletion, code int) healthCompletion {
	t.Helper()
	// Publish a complete exit code: the child may read immediately after
	// creation, before WriteFile has written the first byte.
	if err := os.WriteFile(base+".release.tmp", []byte(strconv.Itoa(code)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(base+".release.tmp", base+".release"); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		return result
	case <-time.After(6 * time.Second):
		t.Fatal("health subprocess never completed")
		return healthCompletion{}
	}
}

func healthCompletionFrame(t *testing.T, f *diagnosisFixture, scenario string, dark bool) *ui.Tester {
	t.Helper()
	f.a.Integrations.Refresh(context.Background())
	f.a.Open(SurfaceSettings)
	f.a.Settings.Page = settingsui.PageIntegrations
	f.a.Settings.Actions.Detected = nil
	tt := ui.NewTester(f.a.View, 960, 1600)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	openSettingsToolForTest(t, f.a, tt)
	dumpCompletionFrame(t, tt, scenario, dark)
	return tt
}

func TestHealthCompletionOwnership(t *testing.T) {
	for _, change := range []string{"package", "manifest", "executable", "new-report", "removed"} {
		t.Run(change, func(t *testing.T) {
			for _, dark := range []bool{false, true} {
				t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
					f := newDiagnosisFixture(t, dark)
					base, done := startHealthCompletion(t, f, "ownership")
					switch change {
					case "package":
						f.entry.ManifestPath = filepath.Join(f.paths.Extensions, "replacement", "floter.extension.json")
					case "manifest":
						f.manifest.Lifecycle.Probes[0].ID = "replacement-check"
					case "executable":
						f.entry.PackageVersion = "2.0.0"
						f.entry.ExecutablePath = filepath.Join(f.dir, "replacement-tool")
					case "new-report":
						f.entry.ProbeReport.CheckedAt = "2026-11-08T00:00:00Z"
					}
					f.write(t)
					if change == "removed" {
						diagnosisWrite(t, f.paths.RepositoryFile, map[string]any{"schemaVersion": 1, "extensions": map[string]extensions.Entry{}})
					}
					before, err := os.ReadFile(f.paths.RepositoryFile)
					if err != nil {
						t.Fatal(err)
					}
					result := finishHealthCompletion(t, base, done, 7)
					after, err := os.ReadFile(f.paths.RepositoryFile)
					if err != nil {
						t.Fatal(err)
					}
					tt := healthCompletionFrame(t, f, "health-owner-"+change, dark)
					if result.err == nil || !bytes.Equal(before, after) {
						t.Errorf("%s: outdated probe overwrote current repository health (err=%v)", change, result.err)
					}
					if change != "removed" && !tt.HasText(i18n.For("zh").Settings.IntegrationsHealthy) {
						t.Error("current healthy badge was replaced by an obsolete failure")
					}
				})
			}
		})
	}
}

func TestHealthCompletionRecovery(t *testing.T) {
	for _, state := range []string{"enabled", "disabled", "other-failure"} {
		t.Run(state, func(t *testing.T) {
			for _, dark := range []bool{false, true} {
				t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
					f := newDiagnosisFixture(t, dark)
					base, done := startHealthCompletion(t, f, "fail")
					if result := finishHealthCompletion(t, base, done, 7); result.err != nil || result.report.Status != extensions.HealthUnhealthy {
						t.Fatalf("initial required failure = %+v", result)
					}
					integration, _ := extensions.LoadInventory(f.paths).WithID(f.entry.ID)
					f.entry = integration.Entry
					if f.entry.State != "broken" {
						t.Fatal("required failure did not break the integration")
					}
					want := state
					if state == "disabled" {
						f.entry.Enabled = false
					}
					if state == "other-failure" {
						code, detail := "integrity_mismatch", "包校验失败"
						f.entry.LastErrorCode, f.entry.LastErrorDetail, f.entry.BrokenReason = &code, &detail, &detail
						want = "broken"
					}
					base, done = startHealthCompletion(t, f, "recover")
					if result := finishHealthCompletion(t, base, done, 0); result.err != nil || result.report.Status != extensions.HealthHealthy {
						t.Fatalf("successful retry = %+v", result)
					}
					tt := healthCompletionFrame(t, f, "health-recovery-"+state, dark)
					integration, _ = extensions.LoadInventory(f.paths).WithID(f.entry.ID)
					if integration.Entry.State != want || integration.Running() != (state == "enabled") {
						t.Errorf("successful health retry left wrong availability: state=%q running=%v want=%q", integration.Entry.State, integration.Running(), want)
					}
					if state == "other-failure" {
						if integration.Entry.LastErrorCode == nil || *integration.Entry.LastErrorCode != "integrity_mismatch" || integration.Entry.BrokenReason == nil || *integration.Entry.BrokenReason != "包校验失败" {
							t.Error("health retry erased an unrelated package failure")
						}
					} else if integration.Entry.LastErrorCode != nil || integration.Entry.LastErrorAt != nil || integration.Entry.BrokenReason != nil {
						t.Error("recovered tool retained stale failure metadata")
					}
					if !tt.HasText(i18n.For("zh").Settings.IntegrationsHealthy) {
						t.Error("successful retry did not render the healthy badge")
					}
				})
			}
		})
	}
}
