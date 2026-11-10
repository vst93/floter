package settingsui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestCommandDraftSurvivesRedrawAndFurtherTyping(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			in := commandListIntegration()
			in.Commands = in.Commands[:2]
			a.Integrations = func() []Integration { return []Integration{in} }
			var saved [][3]string
			a.Actions.SetCommandAlias = func(ext, id, value string) {
				saved = append(saved, [3]string{ext, id, value})
				in.Commands[0].Alias = value
			}
			tt := noticeExitPage(t, a, dark)
			label := a.copy().IntegrationsCommandAliasFor("JSON Viewer")
			optionExitFocus(t, tt, label)
			tt.Type("json")
			tt.Frame()
			tt.Frame()
			commandEditDump(t, tt, "alias-redraw", dark)
			if got := refreshRead(tt); got != "json" {
				t.Errorf("redraw replaced the pending alias: %q, want json", got)
			}
			tt.Key(0, ui.KeyEnd)
			tt.Type("view")
			tt.Frame()
			if len(saved) != 0 {
				t.Fatal("typing wrote before the debounce deadline")
			}
			tt.Advance(179 * time.Millisecond)
			if len(saved) != 0 {
				t.Fatal("alias wrote before 180ms")
			}
			tt.Advance(2 * time.Millisecond)
			if len(saved) != 1 || saved[0] != [3]string{"ext", "jv", "jsonview"} {
				t.Errorf("continued typing lost pending prefix: %v", saved)
			}
		})
	}
}

func TestCommandDraftReturnsWithRedeclaredEditor(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, scenario := range []string{"undeclared", "orphan", "removed", "empty"} {
			t.Run(appearanceName(dark)+"/"+scenario, func(t *testing.T) {
				a := noticeApp(t, dark)
				in := commandListIntegration()
				in.Commands = in.Commands[:2]
				in.Commands[0].Alias = "stored"
				present := true
				a.Integrations = func() []Integration {
					if present {
						return []Integration{in}
					}
					return nil
				}
				var saved [][3]string
				a.Actions.Close = func() {}
				a.Actions.SetCommandAlias = func(ext, id, value string) { saved = append(saved, [3]string{ext, id, value}) }
				tt := noticeExitPage(t, a, dark)
				label := a.copy().IntegrationsCommandAliasFor("JSON Viewer")
				optionExitFocus(t, tt, label)
				tt.Key(ui.Cmd, ui.KeyA)
				want := "draft"
				if scenario == "empty" {
					tt.Key(0, ui.KeyBackspace)
					want = ""
				} else {
					tt.Type(want)
				}
				tt.Frame()
				switch scenario {
				case "undeclared", "empty":
					in.Commands[0].Declared = false
				case "orphan":
					in.Orphan = true
				case "removed":
					present = false
				}
				tt.Frame()
				in.Commands[0].Declared, in.Orphan, present = true, false, true
				tt.Frame()
				tt.Frame()
				commandEditDump(t, tt, "alias-return-"+scenario, dark)
				if len(saved) != 0 {
					t.Fatal("redeclaring wrote before the debounce deadline")
				}
				if scenario == "removed" {
					if err := tt.Click(a.copy().IntegrationsManage + " " + in.Name); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
				}
				optionExitFocus(t, tt, label)
				if got := refreshRead(tt); got != want {
					t.Errorf("redeclared editor lost its pending alias: %q, want %q", got, want)
				}
				tt.Key(0, ui.KeyEnd)
				tt.Type("x")
				tt.Frame()
				a.Flush()
				if len(saved) != 1 || saved[0] != [3]string{"ext", "jv", want + "x"} {
					t.Errorf("restored editor wrote a different draft: %v", saved)
				}
			})
		}
	}
}
