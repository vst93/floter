package shell

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

type installCurrentFixture struct {
	a        *App
	dir      string
	manifest extensions.Manifest
}

func newInstallCurrentFixture(t *testing.T, dark bool) *installCurrentFixture {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(extensions.StubToolLookup(func(...string) (string, bool) { return program, true }))
	t.Cleanup(extensions.StubToolCandidates(func(...string) []extensions.ExecutableCandidate {
		return []extensions.ExecutableCandidate{{Name: "review-tool", Path: program}}
	}))
	s := settings.Default()
	s.Language, s.Theme = "zh", map[bool]string{false: "light", true: "dark"}[dark]
	f := &installCurrentFixture{dir: t.TempDir()}
	f.manifest = extensions.Manifest{SchemaVersion: "2.0", ID: "dev.floter.review-current", Name: "工作区工具", Permissions: []string{extensions.PermissionEnvironment}}
	f.manifest.Distribution.Type = "local"
	f.manifest.Runtime.Type = "system"
	f.manifest.Runtime.ExecutableNames = []string{program}
	f.manifest.Provider.Type, f.manifest.Provider.Descriptor = "static-descriptor", "provider-description.json"
	diagnosisWrite(t, filepath.Join(f.dir, "floter.extension.json"), f.manifest)
	diagnosisWrite(t, filepath.Join(f.dir, "provider-description.json"), map[string]any{"protocolVersion": "1.0", "provider": map[string]string{"id": f.manifest.ID, "name": f.manifest.Name, "version": "1.0.0"}, "commands": []any{}})
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	diagnosisWrite(t, paths.RepositoryFile, map[string]any{"schemaVersion": 1, "extensions": map[string]any{}})
	f.a = New(Options{Store: settings.NewStore(s), Paths: paths})
	f.a.onScreen = func() bool { return true }
	f.a.startDriftReprobe()
	f.a.RefreshIntegrations(context.Background())
	f.a.Open(SurfaceSettings)
	f.a.Settings.Page = settingsui.PageIntegrations
	f.a.Settings.Actions.Detected = nil
	f.a.Settings.Actions.ChooseFolder = func() (string, error) { return f.dir, nil }
	t.Cleanup(invalidateIntegrationFacts)
	return f
}

func installCurrentFrame(f *installCurrentFixture, dark bool) *ui.Tester {
	tt := ui.NewTester(f.a.View, 960, 1600)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func installCurrentDump(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JW_DUMP"); tag != "" {
		file, err := os.Create("/tmp/floter-P13-jw-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

// Queue dispatch until Click's frame has finished, as the native event loop
// does; the headless host otherwise executes onMain on its worker goroutine.
func installCurrentClick(t *testing.T, f *installCurrentFixture, tt *ui.Tester, recommended bool) {
	t.Helper()
	label := i18n.For("zh").Settings.InstallFolderChoose
	run := f.a.Settings.Actions.InstallFromFolder
	var target string
	if recommended {
		label = i18n.For("zh").Settings.IntegrationsConnect
		run = f.a.Settings.Actions.ConnectRecommended
		f.a.Settings.Actions.ConnectRecommended = func(id string) { target = id }
	} else {
		f.a.Settings.Actions.InstallFromFolder = func(dir string) { target = dir }
	}
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if recommended {
		f.a.Settings.Actions.ConnectRecommended = run
	} else {
		f.a.Settings.Actions.InstallFromFolder = run
	}
	if target != "" {
		run(target)
	}
}

func checkInstallCurrentBusy(t *testing.T, recommended bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newInstallCurrentFixture(t, dark)
			started, release := make(chan struct{}, 4), make(chan struct{})
			resume := sync.OnceFunc(func() { close(release) })
			defer resume()
			var calls atomic.Int32
			review := func(extensions.Prepared) bool { calls.Add(1); started <- struct{}{}; <-release; return false }
			f.a.reviewInstall, f.a.reviewRecommended = review, review
			settled := make(chan struct{}, 8)
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			tt := installCurrentFrame(f, dark)
			installCurrentClick(t, f, tt, recommended)
			adoptionWait(t, started)
			tt.Frame()
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
			scenario := "local-busy"
			if recommended {
				scenario = "recommended-busy"
			}
			installCurrentDump(t, tt, scenario, dark)
			// Replace the dispatch with a counter: disabled UI must never call it.
			repeated := 0
			if recommended {
				f.a.Settings.Actions.ConnectRecommended = func(string) { repeated++ }
			} else {
				f.a.Settings.Actions.InstallFromFolder = func(string) { repeated++ }
			}
			label := i18n.For("zh").Settings.InstallFolderChoose
			if recommended {
				label = i18n.For("zh").Settings.IntegrationsConnect
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if repeated != 0 {
				t.Errorf("pending %s permits repeat: calls=%d", scenario, repeated)
			}
			// A direct duplicate host dispatch is rejected too.
			if recommended {
				f.a.connectRecommended("io.github.vst93.v")
			} else {
				f.a.installPackageFolder(f.dir)
			}
			resume()
			// Fixed host produces one begin and one finish; old implementation has
			// no lifecycle signal, so its UI assertion above is the reproduction.
			if repeated != 0 {
				return
			}
			adoptionWait(t, settled)
			adoptionWait(t, settled)
			if calls.Load() != 1 {
				t.Errorf("pending %s opened %d reviews", scenario, calls.Load())
			}
			tt.Frame()
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if repeated != 1 {
				t.Errorf("settled %s lost retry: calls=%d", scenario, repeated)
			}
		})
	}
}

func TestInstallCurrentLocalBusy(t *testing.T)       { checkInstallCurrentBusy(t, false) }
func TestInstallCurrentRecommendedBusy(t *testing.T) { checkInstallCurrentBusy(t, true) }
