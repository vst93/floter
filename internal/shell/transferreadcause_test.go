package shell

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func transferCauseFrame(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = map[bool]string{false: "light", true: "dark"}[dark] }); err != nil {
		t.Fatal(err)
	}
	a.Open(SurfaceSettings)
	a.Settings.Page = settingsui.PageIntegrations
	// Transfer tests exercise their installed fixture. Host discovery can
	// insert unrelated rows between pointer-down and pointer-up; keep that
	// independent asynchronous data source out of this fixture's geometry.
	a.Settings.Actions.Detected = nil
	tt := ui.NewTester(a.View, 720, 1000)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func transferCauseDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JF_DUMP"); tag != "" {
		appearance := map[bool]string{false: "light", true: "dark"}[dark]
		f, err := os.Create("/tmp/floter-P13-jf-" + name + "-" + appearance + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIntegrationImportReadFailureHasCauseAndRetry(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
			path := filepath.Join(t.TempDir(), "removed-backup.json")
			_, want := os.ReadFile(path)
			a.openFileDialog = func() (string, error) { return path, nil }
			a.Settings.Page = settingsui.PageIntegrations
			clearNotices(a)
			tt := transferCauseFrame(t, a, dark)
			a.Settings.Page = settingsui.PageIntegrations
			tt.Frame()
			tt.Frame()
			a.Settings.Actions.ImportIntegrations()
			waitForTransfer(t, a)
			tt.Frame()
			tt.Frame()
			transferCauseDump(t, tt, "import-read", dark)
			if !tt.HasText(want.Error()) {
				t.Errorf("file read failure hides its cause: %s", want)
			}
			if a.Settings.TransferBusy != "" || a.Settings.TransferReport != nil {
				t.Error("failed read retained busy or fabricated a report")
			}
			if err := os.WriteFile(path, []byte(`{"version":2,"extensions":[]}`), 0o600); err != nil {
				t.Fatal(err)
			}
			a.Settings.Actions.ImportIntegrations()
			waitForTransfer(t, a)
			if a.Settings.TransferReport == nil || a.Settings.TransferReport.Path != path {
				t.Error("read failure prevented a valid retry")
			}
		})
	}
}

func TestIntegrationExportRepositoryFailureHasCauseAndRetry(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "backup.json")
			pickers := 0
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { pickers++; return destination, nil })
			original, err := os.ReadFile(paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(paths.RepositoryFile, []byte("broken repository"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, want := extensions.ExportSync(paths)
			if want == nil {
				t.Fatal("broken repository was accepted")
			}
			a.Settings.Page = settingsui.PageIntegrations
			clearNotices(a)
			tt := transferCauseFrame(t, a, dark)
			a.Settings.Page = settingsui.PageIntegrations
			tt.Frame()
			tt.Frame()
			a.Settings.Actions.ExportIntegrations()
			waitForTransfer(t, a)
			tt.Frame()
			tt.Frame()
			transferCauseDump(t, tt, "export-repository", dark)
			if !tt.HasText(want.Error()) {
				t.Errorf("repository export failure hides its cause: %s", want)
			}
			if pickers != 0 || a.Settings.TransferBusy != "" {
				t.Errorf("failed repository export opened %d pickers or retained busy", pickers)
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Error("failed export wrote a file")
			}
			if err := os.WriteFile(paths.RepositoryFile, original, 0o600); err != nil {
				t.Fatal(err)
			}
			a.Settings.Actions.ExportIntegrations()
			waitForTransfer(t, a)
			data, err := os.ReadFile(destination)
			if err != nil {
				t.Fatal(err)
			}
			document, err := extensions.ReadSync(data)
			if err != nil || len(document.Extensions) != 1 || pickers != 1 {
				t.Errorf("repository retry failed: entries=%d pickers=%d err=%v", len(document.Extensions), pickers, err)
			}
		})
	}
}
