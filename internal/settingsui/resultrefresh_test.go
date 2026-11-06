package settingsui

import (
	"errors"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func resultRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HY_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hy-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkResultRefresh(t *testing.T, kind, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
			if kind == "uninstall" {
				in.DistributionSource = "npm"
			}
			a.Integrations = func() []Integration { return []Integration{in} }
			var finish func(error)
			calls, closed := 0, 0
			a.Actions.SetIntegrationEnabled = func(_ string, _ bool, done func(error)) { calls++; finish = done }
			a.Actions.UninstallIntegration = func(_, _, _ string, done func(error)) { calls++; finish = done }
			a.Actions.UninstallComponents = func(_, _, _ string, _ UninstallComponents, done func(error)) { calls++; finish = done }
			a.Actions.DiagnoseIntegration = func(string) {}
			a.Actions.AdoptIntegration = func(string) {}
			a.Actions.Close = func() { closed++ }
			copy := a.copy()
			tt := noticeExitPage(t, a, dark)
			opener := copy.IntegrationsEnable
			if kind != "disable" {
				opener = in.removalLabel(copy)
			}
			if err := tt.Click(opener); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			steps := 1
			if kind == "uninstall" {
				steps = 2
			}
			for i := 0; i < steps; i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				tt.Frame()
			}
			if finish == nil || calls != 1 {
				t.Fatal("answer did not dispatch")
			}
			want := copy.IntegrationsCheck
			switch change {
			case "broken":
				in.Broken = true
			case "orphan":
				in.Orphan = true
				want = copy.IntegrationsAdopt
			case "kind":
				in.DistributionSource = "local"
				want = in.removalLabel(copy)
			case "componentized":
				in.DistributionSource = "npm"
				want = copy.IntegrationsCancel
			}
			tt.Frame()
			tt.Frame()
			if change == "componentized" {
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				tt.Frame()
				resultRefreshDump(t, tt, kind+"-"+change, dark)
				if a.uninstallOpen[in.ID] || !a.removeConfirm[in.ID] || calls != 1 {
					t.Error("kind refresh allowed a second removal answer while busy")
				}
				finish(nil)
				return
			}
			finish(errors.New("The operation could not finish"))
			tt.Frame()
			tt.Frame()
			resultRefreshDump(t, tt, kind+"-"+change, dark)
			if !tt.Focused(want) {
				t.Errorf("changed %s %s result lost keyboard destination %q", kind, change, want)
			}
			if a.anyConfirmOpen() || a.rowBusy(in.ID) || calls != 1 {
				t.Error("invalid result left a question or busy operation")
			}
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			if closed != 1 {
				t.Errorf("invalid result swallowed Escape: %d", closed)
			}
		})
	}
}

func TestResultRefreshDisableBroken(t *testing.T) { checkResultRefresh(t, "disable", "broken") }
func TestResultRefreshDisableOrphan(t *testing.T) { checkResultRefresh(t, "disable", "orphan") }

func checkSaveResult(t *testing.T, retry bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Config: []ConfigField{{Key: "token", Label: "Token", Type: "text"}}, ConfigValues: map[string]any{"token": "stored"}}
			a.Integrations = func() []Integration { return []Integration{in} }
			var finish func(error)
			calls := 0
			a.Actions.SaveConfiguration = func(_ string, _ map[string]any, done func(error)) { calls++; finish = done }
			copy := a.copy()
			tt := noticeExitPage(t, a, dark)
			if err := tt.Click(copy.ConfigOpen); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			word := mustFind(t, tt, "Token")
			tt.ClickAt(word.X+240, word.Y+word.H/2)
			tt.Frame()
			if !tt.Focused("Token") {
				t.Fatal("input did not focus")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("edited")
			tt.Frame()
			for i := 0; i < 8 && !tt.Focused(copy.ConfigSave); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(copy.ConfigSave) {
				t.Fatal("save not focused")
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			if calls != 1 || finish == nil {
				t.Fatal("save not dispatched")
			}
			finish(errors.New("Save failed; retry"))
			tt.Frame()
			tt.Frame()
			name := "save-focus"
			if retry {
				name = "save-retry"
				if err := tt.Click(copy.ConfigSave); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
				if calls != 2 || !a.configSaveBusy[in.ID] {
					t.Fatal("retry not dispatched")
				}
				if tt.HasText("Save failed; retry") {
					t.Error("retry still paints the previous failed attempt")
				}
			} else {
				if !tt.Focused(copy.ConfigSave) {
					t.Error("save failure insertion lost keyboard retry focus")
				}
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				tt.Frame()
				if calls != 2 {
					t.Errorf("keyboard retry dispatched %d calls", calls)
				}
			}
			resultRefreshDump(t, tt, name, dark)
		})
	}
}

func TestResultSaveFailureKeyboard(t *testing.T)    { checkSaveResult(t, false) }
func TestResultSaveRetryClearsFailure(t *testing.T) { checkSaveResult(t, true) }
