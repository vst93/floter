package shell

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

func checkDiagnosisRepositoryFailure(t *testing.T, during bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			if !during {
				f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: false, Args: []string{"-test.run=^TestHealthCompletionProvider$"}}}
				f.write(t)
				f.a.Integrations.Refresh(context.Background())
			}
			f.request(t, 0, "ok", nil, !during)
			f.request(t, 1, "ok", nil, true)
			f.a.onScreen = func() bool { return true }
			tt := healthRefreshFrame(t, f, "unused", dark)
			clearNotices(f.a)
			corrupt := func() {
				if err := os.WriteFile(f.paths.RepositoryFile, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if !during {
				corrupt()
			}
			settled := make(chan struct{}, 8)
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			wait := func() {
				for i := 0; i < 2; i++ {
					select {
					case <-settled:
					case <-time.After(6 * time.Second):
						t.Fatal("repository failure leaves diagnosis busy")
					}
				}
			}
			dispatchDiagnosisClick(t, f, tt)
			if during {
				f.waitStarted(t, 0)
				corrupt()
				f.release(t, 0)
			}
			wait()
			// Drawing drains the shell's pending toast queue into the UI.
			toast := queuedToast(f.a, "floter-notice")
			tt.Frame()
			tt.Frame()
			scenario := map[bool]string{false: "repository-start", true: "repository-completion"}[during]
			dumpDiagnosisExecution(t, tt, scenario, dark)
			if toast.Type != "error" || toast.Title != i18n.NotificationsFor("zh").IntegrationCheckFailed(f.manifest.Name) {
				t.Errorf("diagnosis %s read failure is silent: toast=%+v", scenario, toast)
			}
			if !tt.HasText(i18n.NotificationsFor("zh").IntegrationCheckFailed(f.manifest.Name)) {
				t.Error("read failure notice never reaches the visible surface")
			}
			if !during {
				if _, err := os.Stat(filepath.Join(f.dir, "request-0.started")); err == nil {
					t.Error("unreadable repository still launches the old provider")
				}
			}
			// Repairing the file restores the same action, without reinstalling.
			f.write(t)
			f.a.RefreshIntegrations(context.Background())
			clearNotices(f.a)
			tt.Frame()
			tt.Frame()
			dispatchDiagnosisClick(t, f, tt)
			wait()
			f.a.Settings.Actions.Invalidate = nil
			if toast := queuedToast(f.a, "floter-notice"); toast.Type == "error" || f.a.integrationList()[0].Diagnosis != "ok" {
				t.Errorf("restored repository cannot complete a new check: %+v", toast)
			}
		})
	}
}

func TestDiagnosisExecutionRepositoryStartFailure(t *testing.T) {
	checkDiagnosisRepositoryFailure(t, false)
}
func TestDiagnosisExecutionRepositoryCompletionFailure(t *testing.T) {
	checkDiagnosisRepositoryFailure(t, true)
}

func TestDiagnosisExecutionObsoleteReadFailureIsQuiet(t *testing.T) {
	f := newDiagnosisFixture(t, false)
	f.a.onScreen = func() bool { return true }
	f.a.Open(SurfaceSettings)
	clearNotices(f.a)
	f.request(t, 0, "ok", nil, false)
	done := f.start(t)
	f.waitStarted(t, 0)
	f.a.beginDiagnosis(f.entry.ID) // A newer check owns the feedback now.
	if err := os.WriteFile(f.paths.RepositoryFile, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.release(t, 0)
	diagnosisWait(t, done)
	if titles := noticeTitles(f.a); len(titles) != 0 {
		t.Errorf("obsolete read failure published a notice: %q", titles)
	}
}
