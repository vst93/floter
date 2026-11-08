package settingsui

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

// A second view pumps the same UI-thread state while the host callback is
// pending. This exercises actual disclosure/sidebar events, not guessed epochs.
func checkToolCurrentDeparture(t *testing.T, action, departure string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.Page = PageIntegrations
			a.beginEdit("", callbackTool("weather"))
			var peer *ui.Tester
			if departure == "page" {
				peer = copyPageFrame(t, a, dark)
			} else {
				peer = toolRecoveryFrame(t, a, dark)
			}
			leave := func() {
				labels := []string{a.copy().LocalToolAdd, a.copy().LocalToolAdd}
				if departure == "page" {
					labels = []string{a.copy().PageGeneral, a.copy().PageIntegrations}
				}
				for _, label := range labels {
					if err := peer.Click(label); err != nil {
						t.Fatal(err)
					}
					peer.Frame()
				}
			}
			calls := 0
			label := a.copy().LocalToolChoose
			if action == "picker" {
				a.Actions.ChooseProgram = func() (string, error) { calls++; leave(); return "/usr/bin/old-selection", nil }
			} else {
				label = a.copy().LocalToolCopyCommand
				a.Actions.CopyText = func(string) { calls++; leave() }
			}
			tt := toolRecoveryFrame(t, a, dark)
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			toolCurrentDump(t, tt, action+"-"+departure, dark)
			if calls != 1 || !a.toolOpen || a.Page != PageIntegrations || a.toolProgram != "/usr/bin/weather" || a.toolCommand != "weather" {
				t.Errorf("%s %s accepted departed result: calls=%d open=%v program=%q", action, departure, calls, a.toolOpen, a.toolProgram)
			}
			if a.copiedContent != "" || tt.HasText(a.copy().LocalToolCommandCopied) {
				t.Errorf("copy revived after %s departure", departure)
			}
			// Re-selecting the current page is not a departure. The next
			// explicit operation must work even after the older one retired.
			a.Actions.ChooseProgram = func() (string, error) { calls++; a.selectPage(PageIntegrations); return "/usr/bin/current", nil }
			a.Actions.CopyText = func(string) { calls++; a.selectPage(PageIntegrations) }
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 2 || (action == "picker" && a.toolProgram != "/usr/bin/current") || (action == "copy" && !tt.HasText(a.copy().LocalToolCommandCopied)) {
				t.Error("current operation was rejected after return")
			}
		})
	}
}

func TestToolCurrentPickerFold(t *testing.T) { checkToolCurrentDeparture(t, "picker", "fold") }
func TestToolCurrentCopyFold(t *testing.T)   { checkToolCurrentDeparture(t, "copy", "fold") }

func TestToolCurrentPickerPage(t *testing.T) { checkToolCurrentDeparture(t, "picker", "page") }
func TestToolCurrentCopyPage(t *testing.T)   { checkToolCurrentDeparture(t, "copy", "page") }
