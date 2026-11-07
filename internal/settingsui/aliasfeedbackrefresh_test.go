package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func feedbackDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IP_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ip-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAliasFeedbackTracksTheVisibleDraft(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			in := commandListIntegration()
			in.Commands = in.Commands[:1]
			in.Commands[0].Alias, in.Commands[0].Taken = "taken", true
			a.Integrations = func() []Integration { return []Integration{in} }
			writes := 0
			a.Actions.SetCommandAlias = func(_, _ string, alias string) { writes++; in.Commands[0].Alias, in.Commands[0].Taken = alias, false }
			tt := noticeExitPage(t, a, dark)
			copy := a.copy()
			if !tt.HasText(copy.IntegrationsCommandAliasTaken) {
				t.Fatal("fixture has no conflict")
			}
			optionExitFocus(t, tt, copy.IntegrationsCommandAliasFor("JSON Viewer"))
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("available")
			feedbackDump(t, tt, "alias-feedback", dark)
			if tt.HasText(copy.IntegrationsCommandAliasTaken) {
				t.Error("old alias conflict labels a different pending draft")
			}
			if writes != 0 {
				t.Fatal("feedback wrote before deadline")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("taken")
			if !tt.HasText(copy.IntegrationsCommandAliasTaken) {
				t.Error("unchanged alias lost its real conflict")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(0, ui.KeyBackspace)
			if tt.HasText(copy.IntegrationsCommandAliasTaken) {
				t.Error("empty alias kept the old conflict")
			}
			a.Flush()
			tt.Frame()
			if writes != 1 || in.Commands[0].Alias != "" {
				t.Errorf("Flush changed: writes=%d alias=%q", writes, in.Commands[0].Alias)
			}
		})
	}
}

func TestCustomDraftRetiresWhenCommandEditorDisappears(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			tt := customStateFrame(t, a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("say obsolete")
			(*entries)[1].Action = "plugin:clipboard"
			tt.Frame()
			(*entries)[1].Action = "say refreshed"
			tt.Frame()
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			feedbackDump(t, tt, "command-type-refresh", dark)
			if got := refreshRead(tt); got != "say refreshed" {
				t.Errorf("reopened command resurrected retired draft: %q", got)
			}
			tt.Key(0, ui.KeyEnter)
			if (*entries)[1].Action != "say refreshed" {
				t.Error("Enter overwrote the new host action")
			}
		})
	}
}
