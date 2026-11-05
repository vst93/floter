package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

func runStateApp(params ...Param) (*App, *Integration) {
	a := New(settings.NewStore(settings.Default()), Actions{
		RunWithParams: func(string, map[string]string) *extensions.ParamFailure { return nil },
	})
	integration := &Integration{ID: "runner", Name: "Runner", Running: true, Enabled: true, Params: params}
	a.runOpen = map[string]bool{integration.ID: true}
	return a, integration
}

func runStateFrame(t *testing.T, a *App, integration *Integration, dark bool) *ui.Tester {
	t.Helper()
	ground := realPageGround(t, dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Children(func() {
			a.extensionMetrics(func() { a.runForm(c, i18n.For("en").Settings, *integration) })
		})
	}, 720, 520)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func runStateDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HA_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ha-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunFormSubmitHonorsAvailability(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, state := range []string{"repair", "disable", "remove", "uninstall", "toggle", "stopped", "broken", "orphan"} {
			t.Run(appearanceName(dark)+"/"+state, func(t *testing.T) {
				a, integration := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText})
				calls := 0
				a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure { calls++; return nil }
				switch state {
				case "repair":
					a.repairBusy = map[string]bool{"runner": true}
				case "disable":
					a.disableBusy = map[string]bool{"runner": true}
				case "remove":
					a.removeBusy = map[string]bool{"runner": true}
				case "uninstall":
					a.uninstallBusy = map[string]bool{"runner": true}
				case "toggle":
					a.toggleBusy = map[string]bool{"runner": true}
				case "stopped":
					integration.Running = false
				case "broken":
					integration.Broken = true
				case "orphan":
					integration.Orphan = true
				}
				tt := runStateFrame(t, a, integration, dark)
				if err := tt.Click(i18n.For("en").Settings.RunLabel); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if state == "repair" {
					runStateDump(t, tt, "submit", dark)
				}
				if calls != 0 {
					t.Errorf("unavailable %s form dispatched %d runs", state, calls)
				}
				a.repairBusy, a.disableBusy, a.removeBusy, a.uninstallBusy, a.toggleBusy = nil, nil, nil, nil, nil
				integration.Running, integration.Broken, integration.Orphan = true, false, false
				tt.Frame()
				before := calls
				if err := tt.Click(i18n.For("en").Settings.RunLabel); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if calls != before+1 {
					t.Errorf("available form calls=%d, want %d", calls, before+1)
				}
			})
		}
	}
}

func TestRunFormTextInputsHonorRowLock(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, kind := range []string{extensions.ParamText, extensions.ParamNumber, extensions.ParamPath} {
			t.Run(appearanceName(dark)+"/"+kind, func(t *testing.T) {
				a, integration := runStateApp(Param{ID: "target", Label: "Target", Kind: kind})
				a.repairBusy = map[string]bool{"runner": true}
				tt := runStateFrame(t, a, integration, dark)
				label := mustFind(t, tt, "Target")
				// The label and input share the accessible name; Find selects
				// the left label first. Hit the 220px field in the right slot.
				tt.ClickAt(568, label.Y+label.H/2)
				tt.Frame()
				tt.Type("123")
				tt.Frame()
				runStateDump(t, tt, "input-"+kind, dark)
				if got := a.runValues["runner"]["target"]; got != "" || tt.Focused("Target") {
					t.Errorf("busy %s input accepted editing: value=%q focused=%v", kind, got, tt.Focused("Target"))
				}
				delete(a.repairBusy, "runner")
				tt.Frame()
				tt.ClickAt(568, label.Y+label.H/2)
				tt.Frame()
				tt.Type("4")
				tt.Frame()
				if got := a.runValues["runner"]["target"]; got == "" {
					t.Error("unlocked input did not accept text")
				}
			})
		}
	}
}
