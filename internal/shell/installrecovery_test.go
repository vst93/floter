package shell

import (
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func installRecoveryDump(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JX_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jx-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func installRecoveryInput(t *testing.T, tt *ui.Tester) {
	t.Helper()
	copy := i18n.For("zh").Settings
	if err := tt.Click(copy.IntegrationsPackageHint); err != nil {
		t.Fatal(err)
	}
	tt.Type("review-current")
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	tt.Type("^1.0.0")
	tt.Frame()
}

// Defer the host dispatch until Click's frame has ended, matching the native
// event loop. The completion invalidate is also our worker lifetime boundary.
func installRecoveryRequest(t *testing.T, f *installCurrentFixture, tt *ui.Tester) (string, string) {
	t.Helper()
	run := f.a.Settings.Actions.InstallFromRegistry
	defer func() { f.a.Settings.Actions.InstallFromRegistry = run }()
	var name, version string
	f.a.Settings.Actions.InstallFromRegistry = func(n, v string) { name, version = n, v }
	if err := tt.Click(i18n.For("zh").Settings.IntegrationsInstall); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return name, version
}

func checkInstallRecovery(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newInstallCurrentFixture(t, dark)
			settled := make(chan struct{}, 4)
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			if scenario == "download" {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					http.Error(w, "registry unavailable", http.StatusServiceUnavailable)
				}))
				defer server.Close()
				f.a.Registry = &extensions.Registry{BaseURL: server.URL, Client: server.Client()}
			} else {
				f.a.Registry = installCurrentRegistry(t, f)
				f.a.confirmPermissions = func(extensions.PermissionApproval) bool { return false }
			}
			tt := installCurrentFrame(f, dark)
			installRecoveryInput(t, tt)
			name, version := installRecoveryRequest(t, f, tt)
			if name != "review-current" || version != "^1.0.0" {
				t.Fatalf("initial request = %q / %q", name, version)
			}
			f.a.Settings.Actions.InstallFromRegistry(name, version)
			adoptionWait(t, settled)
			titles := noticeTitles(f.a)
			tt.Frame()
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
			installRecoveryDump(t, tt, "registry-"+scenario, dark)
			name, version = installRecoveryRequest(t, f, tt)
			if name != "review-current" || version != "^1.0.0" {
				t.Errorf("%s lost retry draft: %q / %q", scenario, name, version)
			}
			if scenario == "cancel" && len(titles) != 0 {
				t.Errorf("cancel published result: %q", titles)
			}
			if scenario == "download" && len(titles) != 1 {
				t.Errorf("failure notices = %q", titles)
			}
		})
	}
}

func TestInstallRecoveryDownload(t *testing.T) { checkInstallRecovery(t, "download") }
func TestInstallRecoveryCancel(t *testing.T)   { checkInstallRecovery(t, "cancel") }
