package settingsui

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func destinationDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IN_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-in-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkRunDestination(t *testing.T, scenario string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local", Params: []Param{{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "12"}}}
			present := true
			a.Integrations = func() []Integration {
				if present {
					return []Integration{in}
				}
				return nil
			}
			closed, adopted, runs := 0, 0, 0
			a.Actions.Close = func() { closed++ }
			a.Actions.AdoptIntegration = func(string) { adopted++ }
			a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure { runs++; return nil }
			a.runOpen = map[string]bool{in.ID: true}
			tt := noticeExitPage(t, a, dark)
			focus := "Target"
			mode, reason, _ := strings.Cut(scenario, "-")
			if mode == "submit" {
				focus = a.copy().RunLabel
			} else if mode == "disclosure" {
				focus = a.copy().RunAction
			}
			optionExitFocus(t, tt, focus)
			want := a.copy().Close
			switch scenario {
			case "removed":
				present = false
			case "orphan":
				in.Orphan, in.Params = true, nil
				want = a.copy().IntegrationsAdopt
			case "busy-kind":
				in.Params[0].Kind = extensions.ParamNumber
				a.repairBusy = map[string]bool{in.ID: true}
				want = a.copy().RunCancel
			default:
				if mode == "submit" {
					want = a.copy().RunCancel
				}
				switch reason {
				case "stopped":
					in.Running = false
				case "broken":
					in.Broken = true
				case "busy":
					a.repairBusy = map[string]bool{in.ID: true}
				case "invalid":
					in.Params[0].Kind, in.Params[0].Options = extensions.ParamSelect, []string{"valid"}
				case "removed":
					present = false
				case "orphan":
					in.Orphan = true
					want = a.copy().IntegrationsAdopt
				case "withdrawn":
					a.Actions.RunWithParams = nil
				}
			}
			tt.Frame()
			tt.Frame()
			destinationDump(t, tt, "run-"+scenario, dark)
			if !tt.Focused(want) {
				t.Errorf("%s run lost keyboard destination %q", scenario, want)
			}
			if runs != 0 || closed != 0 || adopted != 0 || a.runValues[in.ID]["target"] != "12" {
				t.Error("refresh acted or replaced answer")
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			switch scenario {
			case "removed":
				if closed != 1 {
					t.Error("removed run cannot exit by keyboard")
				}
			case "orphan":
				if adopted != 1 {
					t.Error("orphan run cannot reach adoption by keyboard")
				}
			default:
				if mode == "disclosure" {
					if reason == "orphan" {
						if adopted != 1 {
							t.Error("withdrawn disclosure cannot reach adoption")
						}
					} else if closed != 1 {
						t.Error("disabled disclosure cannot exit by keyboard")
					}
				} else if a.runOpen[in.ID] {
					t.Error("unavailable run cannot cancel by keyboard")
				}
			}
		})
	}
}

func TestRefreshDestinationRunRemoved(t *testing.T)  { checkRunDestination(t, "removed") }
func TestRefreshDestinationRunOrphan(t *testing.T)   { checkRunDestination(t, "orphan") }
func TestRefreshDestinationRunBusyKind(t *testing.T) { checkRunDestination(t, "busy-kind") }

func TestRefreshDestinationRunSubmitDisabled(t *testing.T) {
	for _, reason := range []string{"stopped", "broken", "busy", "invalid"} {
		t.Run(reason, func(t *testing.T) { checkRunDestination(t, "submit-"+reason) })
	}
}
func TestRefreshDestinationRunDisclosureDisabled(t *testing.T) {
	for _, reason := range []string{"stopped", "broken", "busy", "removed", "orphan", "withdrawn"} {
		t.Run(reason, func(t *testing.T) { checkRunDestination(t, "disclosure-"+reason) })
	}
}

func TestRefreshDestinationCommandAliasReorder(t *testing.T) {
	checkCommandDestination(t, "alias-reorder")
}
func TestRefreshDestinationCommandToggleReorder(t *testing.T) {
	checkCommandDestination(t, "toggle-reorder")
}

func TestRefreshDestinationCommandRemoved(t *testing.T) {
	for _, scenario := range []string{"removed", "undeclared", "card-removed"} {
		t.Run(scenario, func(t *testing.T) { checkCommandDestination(t, scenario) })
	}
}
func TestRefreshDestinationCommandOrphan(t *testing.T) { checkCommandDestination(t, "orphan") }

func TestRefreshDestinationDuplicateCommands(t *testing.T) {
	a := noticeApp(t, true)
	in := commandListIntegration()
	in.Commands[1].ID = in.Commands[0].ID
	a.Integrations = func() []Integration { return []Integration{in} }
	toggled := ""
	a.Actions.SetCommandEnabled = func(_, id string, on bool) { toggled = id }
	tt := noticeExitPage(t, a, true)
	optionExitFocus(t, tt, a.copy().IntegrationsCommandEnable("Build"))
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if toggled != "jv" {
		t.Fatal("duplicate command declarations lost keyboard switch")
	}
}

func TestRefreshDestinationDoesNotSteal(t *testing.T) {
	for _, scenario := range []string{"run-removed", "run-orphan", "run-disabled", "command-removed", "command-orphan"} {
		t.Run(scenario, func(t *testing.T) {
			a := noticeApp(t, true)
			in := commandListIntegration()
			in.RuntimeAvailable = true
			in.Commands = in.Commands[:1]
			in.Params = []Param{{ID: "target", Label: "Target", Kind: extensions.ParamText}}
			present := true
			a.Integrations = func() []Integration {
				if present {
					return []Integration{in}
				}
				return nil
			}
			a.Actions.Close = func() { t.Error("refresh unexpectedly closed settings") }
			a.Actions.AdoptIntegration = func(string) { t.Error("refresh unexpectedly adopted integration") }
			a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure {
				t.Error("refresh unexpectedly ran command")
				return nil
			}
			a.runOpen = map[string]bool{in.ID: true}
			tt := noticeExitPage(t, a, true)
			label := "Target"
			if strings.HasPrefix(scenario, "command-") {
				label = a.copy().IntegrationsCommandAliasFor("JSON Viewer")
			}
			optionExitFocus(t, tt, label)
			optionExitFocus(t, tt, a.copy().Close)
			switch scenario {
			case "run-removed":
				present = false
			case "run-orphan", "command-orphan":
				in.Orphan, in.Params = true, nil
			case "run-disabled":
				in.Running = false
			case "command-removed":
				in.Commands = nil
			}
			tt.Frame()
			tt.Frame()
			if !tt.Focused(a.copy().Close) {
				t.Fatal("unfocused refresh stole header keyboard position")
			}
		})
	}
}

func checkCommandDestination(t *testing.T, scenario string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			in := commandListIntegration()
			in.RuntimeAvailable = true
			in.Commands = in.Commands[:2]
			present := true
			a.Integrations = func() []Integration {
				if present {
					return []Integration{in}
				}
				return nil
			}
			closed, adopted := 0, 0
			a.Actions.Close = func() { closed++ }
			a.Actions.AdoptIntegration = func(string) { adopted++ }
			var toggled, aliasID, aliasValue string
			a.Actions.SetCommandEnabled = func(_, id string, on bool) { toggled = id }
			a.Actions.SetCommandAlias = func(_, id, value string) { aliasID, aliasValue = id, value }
			tt := noticeExitPage(t, a, dark)
			label := a.copy().IntegrationsCommandAliasFor("JSON Viewer")
			if scenario == "toggle-reorder" {
				label = a.copy().IntegrationsCommandEnable("JSON Viewer")
			}
			optionExitFocus(t, tt, label)
			want := label
			switch scenario {
			case "alias-reorder", "toggle-reorder":
				in.Commands[0], in.Commands[1] = in.Commands[1], in.Commands[0]
			case "removed":
				in.Commands = in.Commands[1:]
				want = a.copy().Close
			case "undeclared":
				in.Commands[0].Declared = false
				want = a.copy().Close
			case "card-removed":
				present = false
				want = a.copy().Close
			case "orphan":
				in.Orphan = true
				want = a.copy().IntegrationsAdopt
			}
			tt.Frame()
			tt.Frame()
			destinationDump(t, tt, "command-"+scenario, dark)
			if !tt.Focused(want) {
				t.Errorf("%s command lost keyboard destination %q", scenario, want)
			}
			if toggled != "" || aliasID != "" || closed != 0 || adopted != 0 {
				t.Fatal("refresh dispatched an action")
			}
			switch scenario {
			case "alias-reorder":
				tt.Type("jview")
				tt.Frame()
				a.Flush()
				if aliasID != "jv" || aliasValue != "jview" {
					t.Errorf("reordered alias edited wrong command: id=%q value=%q", aliasID, aliasValue)
				}
			case "toggle-reorder":
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if toggled != "jv" {
					t.Errorf("reordered switch toggled wrong command %q", toggled)
				}
			case "orphan":
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if adopted != 1 {
					t.Error("orphan command cannot reach adoption by keyboard")
				}
			default:
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if closed != 1 {
					t.Error("retired command cannot exit by keyboard")
				}
			}
		})
	}
}
