package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func installFormDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JX_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jx-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInstallFormRegistryBusy(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			calls := 0
			a.Actions.InstallFromRegistry = func(string, string) { calls++ }
			tt := noticeExitPage(t, a, dark)
			a.installName, a.installVersion = "review-current", "^1.0.0"
			a.BeginPackageInstall("registry")
			tt.Frame()
			installFormDump(t, tt, "registry-busy", dark)
			if err := tt.Click(a.copy().IntegrationsInstall); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 0 {
				t.Errorf("busy registry permits repeat: %d", calls)
			}
			a.FinishPackageInstall("registry")
			tt.Frame()
			if err := tt.Click(a.copy().IntegrationsInstall); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 {
				t.Errorf("unlocked retry calls=%d", calls)
			}
		})
	}
}

func TestInstallFormBlankPackage(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			calls := 0
			a.Actions.InstallFromRegistry = func(string, string) { calls++ }
			tt := noticeExitPage(t, a, dark)
			a.installName = " \t "
			tt.Frame()
			optionExitFocus(t, tt, a.copy().IntegrationsPackageHint)
			tt.Key(0, ui.KeyTab) // version
			tt.Frame()
			tt.Key(0, ui.KeyTab) // skip disabled Install
			tt.Frame()
			installFormDump(t, tt, "registry-blank", dark)
			if tt.Focused(a.copy().IntegrationsInstall) {
				t.Error("blank install remains a keyboard stop")
			}
			if err := tt.Click(a.copy().IntegrationsInstall); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 0 {
				t.Errorf("blank request dispatched %d", calls)
			}
			a.installName = "review-current"
			tt.Frame()
			if err := tt.Click(a.copy().IntegrationsInstall); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 {
				t.Errorf("valid package did not unlock: %d", calls)
			}
		})
	}
}

func TestInstallFormRegistryKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			a.Actions.Close = func() {}
			calls := 0
			a.Actions.InstallFromRegistry = func(string, string) { calls++; a.BeginPackageInstall("registry") }
			tt := noticeExitPage(t, a, dark)
			a.installName = "review-current"
			tt.Frame()
			optionExitFocus(t, tt, a.copy().IntegrationsInstall)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			tt.Frame()
			installFormDump(t, tt, "registry-keyboard", dark)
			if !tt.Focused(a.copy().IntegrationsPackageHint) {
				t.Error("waiting registry lost keyboard focus")
			}
			tt.Type("next-package")
			tt.Frame()
			if a.installName != "next-package" || calls != 1 {
				t.Errorf("waiting editor: name=%q calls=%d", a.installName, calls)
			}
			a.FinishPackageInstall("registry")
			tt.Frame()
			if !tt.Focused(a.copy().IntegrationsPackageHint) {
				t.Error("completion stole editor focus")
			}
			optionExitFocus(t, tt, a.copy().Close)
			a.BeginPackageInstall("registry")
			tt.Frame()
			if !tt.Focused(a.copy().Close) {
				t.Error("registry stole unrelated focus")
			}
		})
	}
}

func TestInstallFormRecoveryKeepsNewDraft(t *testing.T) {
	for _, edit := range []string{"name", "version", "cleared"} {
		t.Run(edit, func(t *testing.T) {
			a := noticeApp(t, false)
			var recoverDraft func()
			a.Actions.InstallFromRegistry = func(n, v string) { recoverDraft = a.RegistryInstallRecovery(n, v) }
			a.installName, a.installVersion = "review-current", "^1.0.0"
			tt := noticeExitPage(t, a, false)
			if err := tt.Click(a.copy().IntegrationsInstall); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click(a.copy().IntegrationsPackageHint); err != nil {
				t.Fatal(err)
			}
			if edit == "version" {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			tt.Type("2")
			tt.Frame()
			if edit == "cleared" {
				tt.Key(0, ui.KeyBackspace)
				tt.Frame()
			}
			name, version := a.installName, a.installVersion
			recoverDraft()
			tt.Frame()
			if a.installName != name || a.installVersion != version {
				t.Errorf("old result overwrote %s: %q / %q", edit, a.installName, a.installVersion)
			}
		})
	}
}
