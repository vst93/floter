package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

// A real portable provider subprocess. Request files and release markers let
// tests order completions without guessing how fast a child process runs.
func TestDiagnosisCurrentProvider(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--floter-diagnosis-fixture" || i+2 >= len(os.Args) {
			continue
		}
		dir, op := os.Args[i+1], os.Args[i+2]
		if op == "describe" {
			fmt.Print(`{"protocolVersion":"1.0","provider":{"id":"dev.floter.check","name":"检查工具","version":"1.0"},"commands":[]}`)
			os.Exit(0)
		}
		for n := 0; n < 16; n++ {
			base := filepath.Join(dir, fmt.Sprintf("request-%d", n))
			if err := os.Mkdir(base+".claimed", 0o755); err != nil {
				continue
			}
			data, err := os.ReadFile(base + ".json")
			if err != nil {
				os.Exit(2)
			}
			if err := os.WriteFile(base+".started", nil, 0o600); err != nil {
				os.Exit(3)
			}
			deadline := time.Now().Add(4 * time.Second)
			for {
				if _, err := os.Stat(base + ".release"); err == nil {
					fmt.Print(string(data))
					os.Exit(0)
				}
				if time.Now().After(deadline) {
					os.Exit(4)
				}
				time.Sleep(time.Millisecond)
			}
		}
		os.Exit(5)
	}
}

type diagnosisFixture struct {
	a        *App
	paths    extensions.Paths
	entry    extensions.Entry
	manifest extensions.Manifest
	dir      string
	notices  chan string
}

func newDiagnosisFixture(t *testing.T, dark bool) *diagnosisFixture {
	t.Helper()
	f := &diagnosisFixture{paths: extensions.FromRoot(t.TempDir()), dir: t.TempDir(), notices: make(chan string, 16)}
	if err := f.paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(f.paths.Extensions, "dev.floter.check")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.manifest = extensions.Manifest{SchemaVersion: "2.0", ID: "dev.floter.check", Name: "检查工具"}
	f.manifest.Distribution.Type = "local"
	f.manifest.Runtime.Type = "system"
	f.manifest.Runtime.ExecutableNames = []string{"floter-diagnosis-fixture"}
	f.manifest.Provider.Type = "executable"
	f.manifest.Provider.ArgsPrefix = []string{"-test.run=^TestDiagnosisCurrentProvider$", "--", "--floter-diagnosis-fixture", f.dir}
	f.entry = extensions.Entry{ID: f.manifest.ID, Name: f.manifest.Name, Enabled: true, State: "enabled", ExecutablePath: program, ManifestPath: filepath.Join(pkg, "floter.extension.json")}
	f.entry.ProbeReport = &extensions.HealthReport{Status: extensions.HealthHealthy, CheckedAt: "2026-11-07T20:00:00Z"}
	f.write(t)
	s := settings.Default()
	s.Language, s.Theme = "zh", map[bool]string{false: "light", true: "dark"}[dark]
	f.a = New(Options{Store: settings.NewStore(s), Paths: f.paths, Notify: func(_, body string) { f.notices <- body }})
	f.a.Integrations.Refresh(context.Background())
	invalidateIntegrationFacts()
	t.Cleanup(invalidateIntegrationFacts)
	return f
}

func diagnosisWrite(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *diagnosisFixture) write(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(f.entry.ManifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	diagnosisWrite(t, f.entry.ManifestPath, f.manifest)
	diagnosisWrite(t, f.paths.RepositoryFile, map[string]any{"schemaVersion": 1, "extensions": map[string]extensions.Entry{f.entry.ID: f.entry}})
}

func (f *diagnosisFixture) request(t *testing.T, n int, status string, checks []extensions.DiagnosisCheck, release bool) {
	t.Helper()
	diagnosisWrite(t, filepath.Join(f.dir, fmt.Sprintf("request-%d.json", n)), extensions.Diagnosis{Status: status, Checks: checks})
	if release {
		f.release(t, n)
	}
}

func (f *diagnosisFixture) release(t *testing.T, n int) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.dir, fmt.Sprintf("request-%d.release", n)), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *diagnosisFixture) waitNotice(t *testing.T) string {
	t.Helper()
	select {
	case body := <-f.notices:
		return body
	case <-time.After(6 * time.Second):
		t.Fatal("diagnosis completion did not arrive")
		return ""
	}
}

func (f *diagnosisFixture) frame(t *testing.T, scenario string, dark bool) *ui.Tester {
	t.Helper()
	f.a.Open(SurfaceSettings)
	f.a.Settings.Page = settingsui.PageIntegrations
	f.a.Settings.Actions.Detected = nil
	tt := ui.NewTester(f.a.View, 960, 1600)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	if tag := os.Getenv("FLOTER_P13JN_DUMP"); tag != "" {
		path := "/tmp/floter-P13-jn-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png"
		file, err := os.Create(path)
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
	return tt
}

func checkDiagnosisProjection(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			f.request(t, 0, "maintenance", []extensions.DiagnosisCheck{{ID: "network", Status: "ok", Message: "连接正常"}, {ID: "account", Status: "warning", Message: "需要重新登录"}}, true)
			f.a.Settings.Actions.DiagnoseIntegration(f.entry.ID)
			f.waitNotice(t)
			if scenario == "health-refresh" {
				f.entry.ProbeReport.Status = extensions.HealthUnhealthy
				f.write(t)
				f.a.Integrations.Refresh(context.Background())
			}
			if scenario == "health-language" {
				if err := f.a.Store.Update(func(s *settings.Settings) { s.Language = "en" }); err != nil {
					t.Fatal(err)
				}
			}
			tt := f.frame(t, scenario, dark)
			row := f.a.integrationList()[0]
			switch scenario {
			case "status":
				if row.DiagnosisStatus != "maintenance" || !tt.HasText("Maintenance") {
					t.Error("provider verdict never reaches the diagnosis heading")
				}
			case "checks":
				if len(row.DiagnosisChecks) != 2 || row.DiagnosisChecks[0].ID != "network" || row.DiagnosisChecks[1].ID != "account" || !tt.HasText("连接正常") || !tt.HasText("需要重新登录") {
					t.Error("provider checks never reach the diagnostic grid")
				}
			case "health-refresh":
				want := i18n.For("zh").Settings.IntegrationsUnhealthy
				if row.Health != want || !tt.HasText(want) {
					t.Errorf("diagnosis overwrote current health: %q want %q", row.Health, want)
				}
			case "health-language":
				want := i18n.For("en").Settings.IntegrationsHealthy
				if row.Health != want || !tt.HasText(want) || strings.Contains(row.Health, "健康") {
					t.Errorf("diagnosis health kept previous language: %q want %q", row.Health, want)
				}
			}
		})
	}
}

func TestDiagnosisCurrentVerdictIsVisible(t *testing.T) { checkDiagnosisProjection(t, "status") }
func TestDiagnosisCurrentChecksAreVisible(t *testing.T) { checkDiagnosisProjection(t, "checks") }

func TestDiagnosisCurrentHealthRefresh(t *testing.T)  { checkDiagnosisProjection(t, "health-refresh") }
func TestDiagnosisCurrentHealthLanguage(t *testing.T) { checkDiagnosisProjection(t, "health-language") }
