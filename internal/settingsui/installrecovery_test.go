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
