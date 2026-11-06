package settingsui

import (
	"errors"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func missingOutletDump(t *testing.T, tt *ui.Tester, kind string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IB_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ib-" + kind + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

// Exercise discovery removing an entire row, not just replacing controls in
// a row that still renders. A returning ID must not inherit abandoned focus.
func checkMissingOutlet(t *testing.T, kind string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, elsewhere := range []bool{false, true} {
			place := "entry"
			if elsewhere {
				place = "elsewhere"
			}
			t.Run(appearanceName(dark)+"/"+place, func(t *testing.T) {
				a := noticeApp(t, dark)
				in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
				switch kind {
				case "save", "discard":
					in.Config = []ConfigField{{Key: "token", Label: "Token", Type: "text"}}
					in.ConfigValues = map[string]any{"token": "stored"}
				case "repair":
					in.Broken = true
				case "reconnect":
					in.RuntimeAvailable, in.Running, in.RuntimeOwnership = false, false, "system"
				case "toggle":
					in.Enabled = false
				case "uninstall":
					in.DistributionSource = "npm"
				}
				present := true
				a.Integrations = func() []Integration {
					if present {
						return []Integration{in}
					}
					return nil
				}
				calls, closed, checked := 0, 0, 0
				var finish func(error)
				start := func(id string, done func(error)) {
					if id != in.ID {
						t.Errorf("wrong operation target: %q", id)
					}
					calls++
					finish = done
				}
				a.Actions.SaveConfiguration = func(id string, _ map[string]any, done func(error)) { start(id, done) }
				a.Actions.RepairIntegration = start
				a.Actions.ReconnectIntegration = start
				a.Actions.SetIntegrationEnabled = func(id string, _ bool, done func(error)) { start(id, done) }
				a.Actions.UninstallIntegration = func(id, _, _ string, done func(error)) { start(id, done) }
				a.Actions.UninstallComponents = func(id, _, _ string, _ UninstallComponents, done func(error)) { start(id, done) }
				a.Actions.DiagnoseIntegration = func(string) { checked++ }
				a.Actions.Close = func() { closed++ }
				copy := a.copy()
				tt := noticeExitPage(t, a, dark)
				focus := func(label string) {
					t.Helper()
					for i := 0; i < 35 && !tt.Focused(label); i++ {
						tt.Key(0, ui.KeyTab)
						tt.Frame()
					}
					if !tt.Focused(label) {
						t.Fatalf("cannot reach %q", label)
					}
				}
				activate := func() { tt.Key(0, ui.KeySpace); tt.Frame(); tt.Frame() }
				opener := copy.IntegrationsRecheck
				switch kind {
				case "save", "discard":
					if err := tt.Click(copy.ConfigOpen); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
					tt.Frame()
					word := mustFind(t, tt, "Token")
					tt.ClickAt(word.X+240, word.Y+word.H/2)
					tt.Frame()
					if !tt.Focused("Token") {
						t.Fatal("field not focused")
					}
					tt.Key(ui.Cmd, ui.KeyA)
					tt.Type("edited")
					tt.Frame()
					opener = copy.ConfigSave
					if kind == "discard" {
						opener = copy.ConfigOpen
					}
				case "reconnect":
					opener = copy.IntegrationsReconnect
				case "toggle", "disable":
					opener = copy.IntegrationsEnable
				case "remove", "uninstall":
					opener = in.removalLabel(copy)
				}
				focus(opener)
				activate()
				if kind == "disable" || kind == "remove" || kind == "uninstall" {
					if !tt.Focused(copy.IntegrationsCancel) {
						t.Fatal("confirmation did not open")
					}
					tt.Key(0, ui.KeyTab)
					tt.Frame()
					activate()
					if kind == "uninstall" {
						tt.Key(0, ui.KeyTab)
						tt.Frame()
						activate()
					}
				}
				wantCalls := 1
				if kind == "discard" {
					wantCalls = 0
					if !a.configDiscard[in.ID] {
						t.Fatal("discard question did not open")
					}
				} else if finish == nil {
					t.Fatal("operation did not dispatch")
				}
				if calls != wantCalls {
					t.Fatalf("unexpected initial dispatches: %d", calls)
				}
				if elsewhere {
					// Move outside the row while it still exists. Discovery must
					// respect this explicit move instead of moving to the header.
					if kind == "uninstall" {
						// The modal traps focus, so this control case settles first.
						finish(nil)
						finish = nil
						tt.Frame()
						tt.Frame()
					}
					focus(copy.ShowCommandsInSearch)
				}
				present = false
				tt.Frame()
				tt.Frame()
				if !elsewhere {
					missingOutletDump(t, tt, kind, dark)
				}
				want := copy.Close
				if elsewhere {
					want = copy.ShowCommandsInSearch
				}
				if !tt.Focused(want) {
					t.Errorf("missing %s row lost keyboard outlet %q", kind, want)
				}
				if calls != wantCalls || closed != 0 || checked != 0 {
					t.Error("discovery dispatched an action")
				}
				present = true
				tt.Frame()
				tt.Frame()
				if !tt.Focused(want) {
					t.Errorf("returning %s ID reclaimed abandoned focus", kind)
				}
				if finish != nil {
					finish(errors.New("The operation could not finish"))
					tt.Frame()
					tt.Frame()
					if !tt.Focused(want) {
						t.Errorf("late %s result stole keyboard focus", kind)
					}
				}
				if kind == "save" || kind == "discard" {
					if a.configDrafts[in.ID].values["token"] != "edited" {
						t.Error("disappearance changed draft")
					}
				}
				if !elsewhere {
					activate()
					if closed != 1 {
						t.Errorf("missing %s outlet cannot close settings: %d", kind, closed)
					}
				}
				if calls != wantCalls || checked != 0 {
					t.Error("return or callback repeated the operation")
				}
			})
		}
	}
}

func TestMissingOutletSave(t *testing.T)    { checkMissingOutlet(t, "save") }
func TestMissingOutletDiscard(t *testing.T) { checkMissingOutlet(t, "discard") }

func TestMissingOutletRecheck(t *testing.T)   { checkMissingOutlet(t, "repair") }
func TestMissingOutletReconnect(t *testing.T) { checkMissingOutlet(t, "reconnect") }
