package shell

import (
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/clipboard"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

// These are real action completions. The fixture never decides the severity:
// the action has to carry its outcome all the way to the visible notice.
func checkCompletionSeverity(t *testing.T, scenario string, action func(*testing.T, *diagnosisFixture) string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			f.a.Open(SurfaceSettings)
			f.a.Settings.Page = settingsui.PageIntegrations
			f.a.Settings.Actions.Detected = nil
			f.a.onScreen = func() bool { return true }
			want := action(t, f)
			var notice ui.Toast
			deadline := time.Now().Add(6 * time.Second)
			for {
				f.a.noticeMu.Lock()
				n := len(f.a.notices)
				if n > 0 {
					notice = f.a.notices[n-1]
				}
				f.a.noticeMu.Unlock()
				if n > 0 {
					if n != 1 {
						t.Errorf("%s queued %d notices, want one", scenario, n)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("action did not finish with a visible notice")
				}
				time.Sleep(time.Millisecond)
			}
			tt := ui.NewTester(f.a.View, 960, 800)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			dumpCompletionFrame(t, tt, scenario, dark)
			if notice.Title != want || !tt.HasText(want) {
				t.Errorf("%s lost the failure sentence: notice=%q want=%q", scenario, notice.Title, want)
			}
			if notice.Type != "error" || notice.Timeout != warningNotice {
				t.Errorf("%s failure wears a success mark: type=%q timeout=%s", scenario, notice.Type, notice.Timeout)
			}
			if f.a.Launcher.FeedbackVisible() || len(f.notices) != 0 {
				t.Error("visible failure leaked to another feedback surface")
			}
		})
	}
}

func dumpCompletionFrame(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JO_DUMP"); tag != "" {
		path := "/tmp/floter-P13-jo-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png"
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
}

func TestCompletionSeverityDiagnosis(t *testing.T) {
	checkCompletionSeverity(t, "diagnosis-failed", func(t *testing.T, f *diagnosisFixture) string {
		f.request(t, 0, "problem", []extensions.DiagnosisCheck{{ID: "account", Status: "error", Message: "需要重新登录"}}, true)
		f.a.Settings.Actions.DiagnoseIntegration(f.entry.ID)
		return i18n.NotificationsFor("zh").IntegrationCheckFailed(f.entry.Name)
	})
}

func TestCompletionSeverityClipboard(t *testing.T) {
	checkCompletionSeverity(t, "clipboard-failed", func(t *testing.T, f *diagnosisFixture) string {
		paths := clipboard.FromConfigRoot(t.TempDir())
		f.a.Clipboard = clipboard.NewStore(paths, 0)
		if _, _, err := f.a.Clipboard.AddText("保留这条历史"); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(paths.Index()); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(paths.Index(), "blocked"), 0o755); err != nil {
			t.Fatal(err)
		}
		if f.a.clearClipboardHistory() || len(f.a.Clipboard.Entries()) != 1 {
			t.Fatal("failed clear lost the history or claimed success")
		}
		return i18n.NotificationsFor("zh").ClipboardClearFailed(i18n.For("zh").Settings.ClipboardPlugin)
	})
}

func completionFailureDone(t *testing.T, start func(func(error))) {
	t.Helper()
	done := make(chan error, 1)
	start(func(err error) { done <- err })
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("invalid action unexpectedly succeeded")
		}
	case <-time.After(6 * time.Second):
		t.Fatal("action completion did not arrive")
	}
}

func TestCompletionSeverityReconnect(t *testing.T) {
	checkCompletionSeverity(t, "reconnect-failed", func(t *testing.T, f *diagnosisFixture) string {
		completionFailureDone(t, func(done func(error)) { f.a.Settings.Actions.ReconnectIntegration("missing-tool", done) })
		return i18n.NotificationsFor("zh").IntegrationCheckFailed("missing-tool")
	})
}

func TestCompletionSeverityRepair(t *testing.T) {
	checkCompletionSeverity(t, "repair-failed", func(t *testing.T, f *diagnosisFixture) string {
		completionFailureDone(t, func(done func(error)) { f.a.Settings.Actions.RepairIntegration("missing-tool", done) })
		return i18n.NotificationsFor("zh").IntegrationCheckFailed("missing-tool")
	})
}

func TestCompletionSeverityInstall(t *testing.T) {
	t.Run("registry", func(t *testing.T) {
		checkCompletionSeverity(t, "install-failed", func(t *testing.T, f *diagnosisFixture) string {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "registry unavailable", http.StatusServiceUnavailable)
			}))
			t.Cleanup(server.Close)
			f.a.Registry = &extensions.Registry{BaseURL: server.URL, Client: server.Client()}
			f.a.Settings.Actions.InstallFromRegistry("floter-test-tool", "1.0.0")
			return i18n.NotificationsFor("zh").IntegrationInstallFailed("floter-test-tool")
		})
	})
	t.Run("adoption", func(t *testing.T) {
		checkCompletionSeverity(t, "adopt-failed", func(t *testing.T, f *diagnosisFixture) string {
			f.a.Settings.Actions.AdoptIntegration("missing-tool")
			return i18n.NotificationsFor("zh").IntegrationInstallFailed("missing-tool")
		})
	})
}

func TestCompletionSeverityRemoval(t *testing.T) {
	for _, components := range []bool{false, true} {
		name := map[bool]string{false: "remove-failed", true: "uninstall-failed"}[components]
		t.Run(name, func(t *testing.T) {
			checkCompletionSeverity(t, name, func(t *testing.T, f *diagnosisFixture) string {
				completionFailureDone(t, func(done func(error)) {
					if components {
						f.a.Settings.Actions.UninstallComponents("missing-tool", "检查工具", settingsui.RemovalPackage, settingsui.UninstallComponents{RemoveHostConfig: true}, done)
					} else {
						f.a.Settings.Actions.UninstallIntegration("missing-tool", "检查工具", settingsui.RemovalPackage, done)
					}
				})
				return i18n.NotificationsFor("zh").IntegrationRemoveFailed("检查工具")
			})
		})
	}
}
