package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func commandEditDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IO_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-io-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCommandBusyAliasReturnsKeyboard(t *testing.T)  { checkCommandBusyExit(t, false) }
func TestCommandBusySwitchReturnsKeyboard(t *testing.T) { checkCommandBusyExit(t, true) }

func checkCommandBusyExit(t *testing.T, toggle bool) {
	for _, dark := range []bool{false, true} {
		for _, operation := range []string{"repair", "disable", "remove", "uninstall", "toggle"} {
			t.Run(appearanceName(dark)+"/"+operation, func(t *testing.T) {
				a := noticeApp(t, dark)
				in := commandListIntegration()
				in.Commands = in.Commands[:2]
				a.Integrations = func() []Integration { return []Integration{in} }
				closed, writes := 0, 0
				a.Actions.Close = func() { closed++ }
				a.Actions.SetCommandAlias = func(string, string, string) { writes++ }
				a.Actions.SetCommandEnabled = func(string, string, bool) { writes++ }
				tt := noticeExitPage(t, a, dark)
				label, name := a.copy().IntegrationsCommandAliasFor("JSON Viewer"), "alias-busy-"+operation
				if toggle {
					label, name = a.copy().IntegrationsCommandEnable("JSON Viewer"), "switch-busy-"+operation
				}
				optionExitFocus(t, tt, label)
				busy := map[string]bool{in.ID: true}
				switch operation {
				case "repair":
					a.repairBusy = busy
				case "disable":
					a.disableBusy = busy
				case "remove":
					a.removeBusy = busy
				case "uninstall":
					a.uninstallBusy = busy
				case "toggle":
					a.toggleBusy = busy
				}
				tt.Frame()
				tt.Frame()
				commandEditDump(t, tt, name, dark)
				if !tt.Focused(a.copy().Close) {
					t.Error("busy command stranded keyboard instead of returning to Close")
				}
				if writes != 0 || closed != 0 {
					t.Fatal("busy refresh dispatched an action")
				}
				delete(busy, in.ID)
				tt.Frame()
				if !tt.Focused(a.copy().Close) {
					t.Error("unlock reclaimed the keyboard from its exit")
				}
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if closed != 1 || writes != 0 {
					t.Errorf("busy roundtrip cannot exit: closed=%d writes=%d", closed, writes)
				}
			})
		}
	}
}

func TestCommandBusyDoesNotStealUnrelatedFocus(t *testing.T) {
	a := noticeApp(t, true)
	in := commandListIntegration()
	a.Integrations = func() []Integration { return []Integration{in} }
	a.Actions.Close = func() {}
	tt := noticeExitPage(t, a, true)
	optionExitFocus(t, tt, a.copy().IntegrationsCommandAliasFor("JSON Viewer"))
	optionExitFocus(t, tt, a.copy().Close)
	a.repairBusy = map[string]bool{in.ID: true}
	tt.Frame()
	if !tt.Focused(a.copy().Close) {
		t.Fatal("busy command stole unrelated focus")
	}
}
