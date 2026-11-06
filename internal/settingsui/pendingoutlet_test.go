package settingsui

import (
	"errors"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func pendingOutletDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IA_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ia-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkPendingOutlet(t *testing.T, kind, change string) {
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
				if kind == "save" {
					in.Config = []ConfigField{{Key: "token", Label: "Token", Type: "text"}}
					in.ConfigValues = map[string]any{"token": "stored"}
				}
				if kind == "repair" {
					in.Broken = true
				}
				if kind == "reconnect" {
					in.RuntimeAvailable, in.Running, in.RuntimeOwnership = false, false, "system"
				}
				if kind == "toggle" {
					in.Enabled = false
				}
				a.Integrations = func() []Integration { return []Integration{in} }
				calls, opened, checked, adopted, closed := 0, 0, 0, 0, 0
				var finish func(error)
				a.Actions.SaveConfiguration = func(_ string, _ map[string]any, done func(error)) { calls++; finish = done }
				a.Actions.RepairIntegration = func(_ string, done func(error)) { calls++; finish = done }
				a.Actions.ReconnectIntegration = a.Actions.RepairIntegration
				a.Actions.SetIntegrationEnabled = func(_ string, _ bool, done func(error)) { calls++; finish = done }
				a.Actions.OpenConfiguration = func(id string, cmd []string) {
					if id != in.ID || len(cmd) != 1 || cmd[0] != "settings" {
						t.Errorf("wrong configuration target: %s %v", id, cmd)
					}
					opened++
				}
				a.Actions.DiagnoseIntegration = func(string) { checked++ }
				a.Actions.AdoptIntegration = func(string) { adopted++ }
				a.Actions.Close = func() { closed++ }
				copy := a.copy()
				tt := noticeExitPage(t, a, dark)
				opener := copy.IntegrationsRecheck
				if kind == "save" {
					if err := tt.Click(copy.ConfigOpen); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
					tt.Frame()
					word := mustFind(t, tt, "Token")
					tt.ClickAt(word.X+240, word.Y+word.H/2)
					tt.Frame()
					if !tt.Focused("Token") {
						t.Fatal("Token did not focus")
					}
					tt.Key(ui.Cmd, ui.KeyA)
					tt.Type("edited")
					tt.Frame()
					opener = copy.ConfigSave
				}
				if kind == "reconnect" {
					opener = copy.IntegrationsReconnect
				}
				if kind == "toggle" {
					opener = copy.IntegrationsEnable
				}
				// Reach the real entry through Tab, then dispatch exactly once.
				for i := 0; i < 35 && !tt.Focused(opener); i++ {
					tt.Key(0, ui.KeyTab)
					tt.Frame()
				}
				if !tt.Focused(opener) {
					t.Fatalf("entry %q not focused", opener)
				}
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				tt.Frame()
				if finish == nil || calls != 1 {
					t.Fatalf("entry did not dispatch once: %d", calls)
				}
				if elsewhere {
					for i := 0; i < 35 && !tt.Focused(copy.Close); i++ {
						tt.Key(ui.Shift, ui.KeyTab)
						tt.Frame()
						tt.Frame()
					}
					if !tt.Focused(copy.Close) {
						t.Fatal("user could not focus header exit")
					}
					tt.HoldModifiers(0)
				}
				if change == "unopenable" && !elsewhere {
					// Saving still permits editing by contract. Move back to the
					// field before its host form is replaced by a tool editor.
					for i := 0; i < 12 && !tt.Focused("Token"); i++ {
						tt.Key(ui.Shift, ui.KeyTab)
						tt.Frame()
					}
					if !tt.Focused("Token") {
						t.Fatal("pending field did not focus")
					}
					tt.HoldModifiers(0)
				}
				want := copy.IntegrationsCheck
				switch change {
				case "owner":
					in.ConfigOwner, in.ConfigOpenCommand = "tool", []string{"settings"}
					want = copy.ConfigOpenConfiguration
				case "schema":
					in.Config = nil
				case "orphan":
					in.Orphan = true
					want = copy.IntegrationsAdopt
				case "unopenable":
					in.ConfigOwner = "tool"
				case "healthy":
					in.Broken, in.RuntimeAvailable, in.Running = false, true, true
					// Discovery refreshes can arrive before the mutation callback.
					// All peer actions are still disabled until that reply lands.
					want = copy.Close
				default:
					t.Fatal("unknown refresh")
				}
				if elsewhere {
					want = copy.Close
				}
				tt.Frame()
				tt.Frame()
				if !elsewhere {
					pendingOutletDump(t, tt, kind+"-"+change, dark)
				}
				if !tt.Focused(want) {
					t.Errorf("pending %s-%s lost keyboard outlet %q", kind, change, want)
				}
				if calls != 1 || opened+checked+adopted+closed != 0 {
					t.Error("refresh dispatched an action")
				}
				finish(errors.New("The operation could not finish"))
				tt.Frame()
				tt.Frame()
				if !tt.Focused(want) {
					t.Errorf("late %s result changed keyboard outlet %q", kind, want)
				}
				if a.rowBusy(in.ID) || a.configSaveBusy[in.ID] {
					t.Error("callback did not release busy state")
				}
				if kind == "save" && change != "schema" && a.configDrafts[in.ID].values["token"] != "edited" {
					t.Error("refresh changed local draft")
				}
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				tt.Frame()
				if want == copy.Close {
					if closed != 1 || opened+checked+adopted != 0 {
						t.Error("header exit did not activate exactly once")
					}
				} else if opened+checked+adopted != 1 || closed != 0 {
					t.Errorf("outlet dispatch: open=%d check=%d adopt=%d close=%d", opened, checked, adopted, closed)
				}
				if calls != 1 {
					t.Error("callback or Space repeated original operation")
				}
			})
		}
	}
}

func TestPendingOutletSaveOwner(t *testing.T)  { checkPendingOutlet(t, "save", "owner") }
func TestPendingOutletSaveSchema(t *testing.T) { checkPendingOutlet(t, "save", "schema") }

func TestPendingOutletRecheckHealthy(t *testing.T)   { checkPendingOutlet(t, "repair", "healthy") }
func TestPendingOutletReconnectHealthy(t *testing.T) { checkPendingOutlet(t, "reconnect", "healthy") }

func TestPendingOutletSaveOrphan(t *testing.T)         { checkPendingOutlet(t, "save", "orphan") }
func TestPendingOutletEditingUnavailable(t *testing.T) { checkPendingOutlet(t, "save", "unopenable") }

func TestPendingOutletRepairOrphan(t *testing.T) { checkPendingOutlet(t, "repair", "orphan") }
func TestPendingOutletToggleOrphan(t *testing.T) { checkPendingOutlet(t, "toggle", "orphan") }
