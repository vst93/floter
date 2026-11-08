package shell

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
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
			// Await the host's begin and finish even when the UI assertion fails;
			// a mutation must not leave a worker reading a removed fixture.
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

func checkInstallCurrentDeclaration(t *testing.T, recommended bool, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			// Recommended materialization is confined to this test's own root.
			t.Setenv("TMPDIR", t.TempDir())
			f := newInstallCurrentFixture(t, dark)
			started := make(chan extensions.Prepared, 2)
			release := make(chan struct{})
			resume := sync.OnceFunc(func() { close(release) })
			defer resume()
			review := func(p extensions.Prepared) bool { started <- p; <-release; return true }
			f.a.reviewInstall, f.a.reviewRecommended = review, review
			settled := make(chan struct{}, 8)
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			tt := installCurrentFrame(f, dark)
			installCurrentClick(t, f, tt, recommended)
			var p extensions.Prepared
			select {
			case p = <-started:
			case <-time.After(6 * time.Second):
				t.Fatal("install did not reach review")
			}
			adoptionWait(t, settled)
			dir := f.dir
			if recommended {
				dirs, err := filepath.Glob(filepath.Join(os.TempDir(), "floter-recommended-*"))
				if err != nil || len(dirs) != 1 {
					t.Fatalf("review stage: %v %v", dirs, err)
				}
				dir = dirs[0]
			}
			current := p.Manifest
			switch change {
			case "permissions":
				current.Permissions = append(append([]string{}, current.Permissions...), extensions.PermissionClipboardRead)
				diagnosisWrite(t, filepath.Join(dir, "floter.extension.json"), current)
			case "package":
				// Keep the old manifest's bytes intact: only its package entry changes.
				current.Name = "当前工作区工具"
				diagnosisWrite(t, filepath.Join(dir, "current.json"), current)
				diagnosisWrite(t, filepath.Join(dir, "package.json"), map[string]any{"name": "current-package", "version": "2.0.0", "floter": map[string]string{"manifest": "current.json"}})
			}
			before, err := os.ReadFile(f.a.Paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			resume()
			adoptionWait(t, settled)
			after, err := os.ReadFile(f.a.Paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			titles := noticeTitles(f.a)
			tt.Frame()
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
			scenario := "local-" + change
			if recommended {
				scenario = "recommended-" + change
			}
			installCurrentDump(t, tt, scenario, dark)
			if !bytes.Equal(before, after) {
				t.Errorf("obsolete %s review rewrote repository", scenario)
			}
			if len(titles) != 0 {
				t.Errorf("obsolete %s review published a result: %q", scenario, titles)
			}
			if t.Failed() {
				return
			}
			// A new explicit review is still allowed to install the current package.
			installCurrentClick(t, f, tt, recommended)
			adoptionWait(t, settled)
			adoptionWait(t, settled)
			if installed, ok := f.a.Integrations.Inventory().WithID(p.Manifest.ID); !ok {
				t.Error("fresh review did not install")
			} else if change == "package" && !recommended && installed.Manifest.Name != current.Name {
				t.Errorf("fresh review loaded the old package entry: got=%q want=%q", installed.Manifest.Name, current.Name)
			}
		})
	}
}

func TestInstallCurrentLocalPermissions(t *testing.T) {
	checkInstallCurrentDeclaration(t, false, "permissions")
}
func TestInstallCurrentRecommendedPermissions(t *testing.T) {
	checkInstallCurrentDeclaration(t, true, "permissions")
}

func TestInstallCurrentLocalPackage(t *testing.T) {
	checkInstallCurrentDeclaration(t, false, "package")
}
func TestInstallCurrentRecommendedPackage(t *testing.T) {
	checkInstallCurrentDeclaration(t, true, "package")
}

func installCurrentRegistry(t *testing.T, f *installCurrentFixture) *extensions.Registry {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gz)
	manifest, err := json.Marshal(f.manifest)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{
		"package/floter.extension.json":     manifest,
		"package/package.json":              []byte(`{"name":"review-current","version":"1.0.0","floter":{"manifest":"floter.extension.json"}}`),
		"package/provider-description.json": []byte(`{"protocolVersion":"1.0","provider":{"id":"dev.floter.review-current","name":"工作区工具","version":"1.0.0"},"commands":[]}`),
	} {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tarball" {
			w.Write(buffer.Bytes())
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"name": "review-current", "dist-tags": map[string]string{"latest": "1.0.0"}, "versions": map[string]any{"1.0.0": map[string]any{"version": "1.0.0", "dist": map[string]string{"tarball": "http://" + r.Host + "/tarball"}}}})
	}))
	t.Cleanup(server.Close)
	return &extensions.Registry{BaseURL: server.URL, Client: server.Client()}
}

func checkInstallCurrentCancel(t *testing.T, registry bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newInstallCurrentFixture(t, dark)
			started, release, settled := make(chan struct{}), make(chan struct{}), make(chan struct{}, 4)
			resume := sync.OnceFunc(func() { close(release) })
			defer resume()
			f.a.confirmPermissions = func(extensions.PermissionApproval) bool { close(started); <-release; return false }
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			tt := installCurrentFrame(f, dark)
			if registry {
				f.a.Registry = installCurrentRegistry(t, f)
				if err := tt.Click(i18n.For("zh").Settings.IntegrationsPackageHint); err != nil {
					t.Fatal(err)
				}
				tt.Type("review-current")
				tt.Frame()
				run := f.a.Settings.Actions.InstallFromRegistry
				var name, version string
				f.a.Settings.Actions.InstallFromRegistry = func(n, v string) { name, version = n, v }
				if err := tt.Click(i18n.For("zh").Settings.IntegrationsInstall); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				f.a.Settings.Actions.InstallFromRegistry = run
				if name != "review-current" {
					t.Fatalf("registry input dispatched %q", name)
				}
				run(name, version)
			} else {
				f.a.HandleURL("floter://connect?manifest=" + f.dir)
			}
			adoptionWait(t, started)
			dir := f.dir
			if registry {
				dirs, err := filepath.Glob(filepath.Join(f.a.Paths.Root, ".download-*"))
				if err != nil || len(dirs) != 1 {
					t.Fatalf("download stage: %v %v", dirs, err)
				}
				dir = dirs[0]
			}
			current := f.manifest
			current.Permissions = nil
			diagnosisWrite(t, filepath.Join(dir, "floter.extension.json"), current)
			source, err := os.ReadFile(filepath.Join(dir, "floter.extension.json"))
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(f.a.Paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			resume()
			adoptionWait(t, settled)
			after, err := os.ReadFile(f.a.Paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			titles := noticeTitles(f.a)
			tt.Frame()
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
			scenario := "link-cancel"
			if registry {
				scenario = "registry-cancel"
			}
			installCurrentDump(t, tt, scenario, dark)
			if !bytes.Equal(before, after) {
				t.Errorf("cancelled %s rewrote repository", scenario)
			}
			// Cancellation is an answer, never a failed install or refused link.
			if len(titles) != 0 {
				t.Errorf("cancelled %s published a result: notices=%q pending=%d", scenario, titles, pendingNotices(f.a))
			}
			if registry {
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Errorf("cancelled registry retained download: %v", err)
				}
			} else if kept, err := os.ReadFile(filepath.Join(dir, "floter.extension.json")); err != nil || !bytes.Equal(source, kept) {
				t.Errorf("cancelled link changed the local source: %v", err)
			}
		})
	}
}

func TestInstallCurrentLinkCancel(t *testing.T)     { checkInstallCurrentCancel(t, false) }
func TestInstallCurrentRegistryCancel(t *testing.T) { checkInstallCurrentCancel(t, true) }
