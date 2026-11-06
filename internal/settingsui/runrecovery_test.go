package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func recoveryDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HT_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ht-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunRecoveryNewParameterDefaults(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, integration := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "original"})
			tt := runStateFrame(t, a, integration, dark)
			a.runValues["runner"]["target"] = "my edit"
			integration.Params = append(integration.Params,
				Param{ID: "format", Label: "Format", Kind: extensions.ParamSelect, Default: "json", Options: []string{"text", "json"}},
				Param{ID: "limit", Label: "Limit", Kind: extensions.ParamNumber, Default: "12"},
			)
			tt.Frame()
			tt.Frame()
			recoveryDump(t, tt, "new-parameter", dark)
			if got := a.runValues["runner"]; got["target"] != "my edit" || got["format"] != "json" || got["limit"] != "12" {
				t.Errorf("new parameters did not receive defaults without replacing edits: %v", got)
			}
		})
	}
}

func TestRunRecoveryRemovedParameterCannotReachCommand(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, integration := runStateApp(
				Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "current"},
				Param{ID: "obsolete", Label: "Obsolete", Kind: extensions.ParamText, Default: "old"},
			)
			tt := runStateFrame(t, a, integration, dark)
			integration.Params = integration.Params[:1]
			tt.Frame()
			calls := 0
			a.Actions.RunWithParams = func(_ string, values map[string]string) *extensions.ParamFailure {
				calls++
				if len(values) != 1 || values["target"] != "current" {
					t.Errorf("removed parameter reached command: %v", values)
				}
				return nil
			}
			if err := tt.Click(i18n.For("en").Settings.RunLabel); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 {
				t.Errorf("runs=%d, want 1", calls)
			}
			integration.Params = append(integration.Params, Param{ID: "obsolete", Label: "Re-added", Kind: extensions.ParamText, Default: "fresh"})
			tt.Frame()
			tt.Frame()
			if got := a.runValues["runner"]["obsolete"]; got != "fresh" {
				t.Errorf("re-added parameter resurrected old answer: %q", got)
			}
		})
	}
}

func TestRunRecoveryRetypedParameterForgetsUndo(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, integration := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "8"})
			tt := runStateFrame(t, a, integration, dark)
			label := mustFind(t, tt, "Target")
			tt.ClickAt(568, label.Y+label.H/2)
			tt.Frame()
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("24")
			tt.Frame()
			integration.Params[0].Kind = extensions.ParamNumber
			tt.Frame()
			tt.Frame()
			tt.ClickAt(568, label.Y+label.H/2)
			tt.Frame()
			tt.Key(ui.Cmd, ui.KeyZ)
			tt.Frame()
			recoveryDump(t, tt, "retyped-undo", dark)
			if got := a.runValues["runner"]["target"]; got != "24" {
				t.Errorf("new number editor resurrected text undo: %q", got)
			}
		})
	}
}

func TestRunRecoveryCommandOwnsSubmittedAnswers(t *testing.T) {
	a, integration := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "first"})
	var submitted map[string]string
	calls := 0
	a.Actions.RunWithParams = func(_ string, values map[string]string) *extensions.ParamFailure {
		calls++
		submitted = values
		return nil
	}
	tt := runStateFrame(t, a, integration, false)
	if err := tt.Click(i18n.For("en").Settings.RunLabel); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	label := mustFind(t, tt, "Target")
	tt.ClickAt(568, label.Y+label.H/2)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("second")
	tt.Frame()
	if submitted["target"] != "first" {
		t.Errorf("later edit rewrote submitted command: %v", submitted)
	}
	submitted["target"] = "host-owned"
	tt.Frame()
	if got := a.runValues["runner"]["target"]; got != "second" {
		t.Errorf("command map mutated live form: %q", got)
	}
	if calls != 1 {
		t.Errorf("runs=%d, want 1", calls)
	}
}

func TestRunRecoveryCancelReturnsToDisclosure(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, integration := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "stored"})
			a.Integrations = func() []Integration { return []Integration{*integration} }
			a.Store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) })
			calls := 0
			a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure { calls++; return nil }
			tt := integrationPage(t, a, 900, 1400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			copy := i18n.For("en").Settings
			a.runValues["runner"]["target"] = "edited"
			tt.Frame()
			if err := tt.Click(copy.RunCancel); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			recoveryDump(t, tt, "run-cancel", dark)
			if !tt.Focused(copy.RunAction) {
				t.Error("cancelled run form lost disclosure focus")
			}
			if a.runOpen["runner"] || calls != 0 {
				t.Errorf("cancel open=%v runs=%d", a.runOpen["runner"], calls)
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			if !a.runOpen["runner"] || a.runValues["runner"]["target"] != "edited" {
				t.Error("keyboard reopen lost parameter draft")
			}
		})
	}
}
