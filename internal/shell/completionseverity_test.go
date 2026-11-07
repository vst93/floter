package shell

import (
	"image/png"
	"os"
	"testing"
	"time"

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
		f.a.notifyClipboardClear(false)
		return i18n.NotificationsFor("zh").ClipboardClearFailed(i18n.For("zh").Settings.ClipboardPlugin)
	})
}
