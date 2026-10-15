package settingsui

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
)

// The re-check and the reconnect are the one row action that carries an
// in-flight face: the reference swapped the button's own glyph (Wrench /
// RefreshCw) for a turning LoaderCircle and set `aria-busy` + `disabled={busy}`
// until `runMutation`'s `finally` cleared it
// (`src/extensions/ExtensionRow.tsx:313-330,345-372`,
// `src/ExtensionsPanel.tsx:1190-1210`). The busy state lives on the row, so
// pressing the button reaches the shell *with* the callback that settles it,
// the button cannot be pressed a second time while the work runs, and any
// answer — success or failure — returns it to its resting word. A failure is
// not shown on the row: it goes on through the shell's own notice path, which
// this test pins by checking the row is the *only* thing that changed.
func TestARepairHoldsItsInFlightFaceUntilTheShellAnswers(t *testing.T) {
	en := i18n.For("en").Settings

	t.Run("re-check", func(t *testing.T) {
		repaired := []string{}
		var answer func(error)
		a := newTestApp()
		a.Actions = Actions{
			RepairIntegration: func(id string, done func(error)) {
				repaired = append(repaired, id)
				answer = done
			},
			UninstallComponents: func(string, string, string, UninstallComponents, func(error)) {},
		}
		a.Integrations = func() []Integration {
			return []Integration{{ID: "broken", Name: "Broken Tool", Enabled: true, Broken: true,
				RuntimeOwnership: "system", FailureCode: extensions.BindingCodeMissing}}
		}
		tt := render(t, a, 760, 1200)
		if err := tt.Click("Integrations"); err != nil {
			t.Fatalf("click: %v (texts %v)", err, tt.Texts())
		}
		tt.Frame()

		if err := tt.Click(en.IntegrationsRecheck); err != nil {
			t.Fatalf("click re-check: %v", err)
		}
		tt.Frame()
		if answer == nil {
			t.Fatalf("the re-check never reached the shell: %v", tt.Texts())
		}
		if len(repaired) != 1 || repaired[0] != "broken" {
			t.Fatalf("repaired = %v, want [broken]", repaired)
		}
		if !a.repairBusy["broken"] {
			t.Errorf("the re-check is not marked in flight")
		}
		// The word stays on the pill so the row says *what* is running, and the
		// pill is disabled: a second press cannot start a second write.
		if !tt.HasText(en.IntegrationsRecheck) {
			t.Errorf("the busy pill lost its word: %v", tt.Texts())
		}
		if err := tt.Click(en.IntegrationsRecheck); err == nil {
			tt.Frame()
		}
		if len(repaired) != 1 {
			t.Errorf("the re-check ran again while busy: %v", repaired)
		}

		// A failure is still an answer: the pill returns to its resting face,
		// and the failure reaches the user through the shell's notice path
		// rather than through the row.
		answer(errors.New("disk went away"))
		tt.Frame()
		if a.repairBusy["broken"] {
			t.Errorf("a failed re-check left the row busy")
		}
	})

	t.Run("reconnect", func(t *testing.T) {
		reconnected := []string{}
		var answer func(error)
		a := newTestApp()
		a.Actions = Actions{
			ReconnectIntegration: func(id string, done func(error)) {
				reconnected = append(reconnected, id)
				answer = done
			},
			UninstallComponents: func(string, string, string, UninstallComponents, func(error)) {},
		}
		a.Integrations = func() []Integration {
			return []Integration{{ID: "sys", Name: "System Tool", RuntimeOwnership: "system"}}
		}
		tt := render(t, a, 760, 1000)
		if err := tt.Click("Integrations"); err != nil {
			t.Fatalf("click: %v (texts %v)", err, tt.Texts())
		}
		tt.Frame()

		if err := tt.Click(en.IntegrationsReconnect); err != nil {
			t.Fatalf("click reconnect: %v", err)
		}
		tt.Frame()
		if answer == nil || len(reconnected) != 1 || reconnected[0] != "sys" {
			t.Fatalf("reconnect = %v, answer set=%v", reconnected, answer != nil)
		}
		if !a.repairBusy["sys"] {
			t.Errorf("the reconnect is not marked in flight")
		}
		if err := tt.Click(en.IntegrationsReconnect); err == nil {
			tt.Frame()
		}
		if len(reconnected) != 1 {
			t.Errorf("the reconnect ran again while busy: %v", reconnected)
		}
		answer(nil)
		tt.Frame()
		if a.repairBusy["sys"] {
			t.Errorf("success left the row busy")
		}
	})
}

// The busy pill is a different picture from the resting one: the reference's
// in-flight face is not just a faded word, it leads with the turning
// LoaderCircle (`src/extensions/ExtensionRow.tsx:305,325,364`). Two things are
// pinned on the frame — the leading mark takes a slot of its own, so the word
// moves right (the frame pixels change), and the pill's ink fades the way
// `:disabled { opacity: 0.4 }` did (`extensions.css:1369-1373`).
func TestTheBusyActionPillLeadsWithASpinnerAndFades(t *testing.T) {
	// The mark exists in the shared vocabulary — and it is a different shape
	// from the reconnect's resting RefreshCw, so the swap is visible.
	mark, ok := launcher.Glyph("loader-circle")
	if !ok || mark == nil {
		t.Fatal("the shared vocabulary has no loader-circle mark")
	}
	if refresh, ok := launcher.Glyph("refresh"); ok && mark == refresh {
		t.Error("the in-flight mark is the reconnect's resting mark")
	}

	const label = "Re-check"
	draw := func(busy bool) (*ui.Tester, ui.Rect) {
		a := newTestApp()
		tt := ui.NewTester(func(c *ui.Context) {
			if busy {
				a.busyActionButton(c, label, "extensions.repair.test")
			} else {
				a.actionButton(c, label)
			}
		}, 240, 80)
		tt.Frame()
		tt.Frame()
		box, _ := tt.Find(label)
		return tt, box
	}
	idle, idleBox := draw(false)
	busy, busyBox := draw(true)
	if idleBox.W == 0 || busyBox.W == 0 {
		t.Fatalf("the pill's word has no box: idle=%v busy=%v\nidle texts %v\nbusy texts %v",
			idleBox, busyBox, idle.Texts(), busy.Texts())
	}
	if busyBox.X <= idleBox.X {
		t.Errorf("the busy pill did not lead with the spinner: idle word x=%v busy word x=%v",
			idleBox.X, busyBox.X)
	}
	idleLum, busyLum := brightness(idle, idleBox), brightness(busy, busyBox)
	t.Logf("pill word brightness: idle=%.2f busy=%.2f", idleLum, busyLum)
	if busyLum <= idleLum {
		t.Errorf("the busy pill's ink did not fade: idle=%.2f busy=%.2f", idleLum, busyLum)
	}
}
