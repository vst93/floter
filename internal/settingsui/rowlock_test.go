package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// lockedRowIntegration is the whole-row lock's fixture row: one runnable,
// generated integration whose every action and whose switch is wired.
func lockedRowIntegration() Integration {
	return Integration{
		ID: "v", Name: "V Tools", Enabled: false, Running: true,
		RuntimeAvailable: true, Generated: true, DistributionSource: "local",
	}
}

// lockedRowApp is the fixture for the whole-row lock: the row above with every
// action wired, so each control can be pressed and its callback counted. The
// caller adds the in-flight mark it wants the row to carry.
func lockedRowApp(calls *[]string, load func(*App)) *App {
	a := oneIntegrationApp(lockedRowIntegration(), Actions{
		RunWithParams: func(string, map[string]string) *extensions.ParamFailure {
			*calls = append(*calls, "run")
			return nil
		},
		DiagnoseIntegration:   func(string) { *calls = append(*calls, "check") },
		ReprobeCommands:       func(string) { *calls = append(*calls, "reprobe") },
		EditCustomTool:        func(string, CustomTool) { *calls = append(*calls, "edit") },
		ToolForEdit:           func(string) (CustomTool, bool) { return CustomTool{}, true },
		UninstallIntegration:  func(string, string, string, func(error)) { *calls = append(*calls, "uninstall") },
		SetIntegrationEnabled: func(string, bool, func(error)) { *calls = append(*calls, "enable") },
	})
	if load != nil {
		load(a)
	}
	return a
}

// The reference keeps one `operation` for the panel and disables *every* control
// in a row from it: `const busy = Boolean(operation)` then `disabled={busy}` on
// the Connect, re-check, reconnect, Run, menu and switch controls
// (`src/extensions/ExtensionRow.tsx:131,301-419,447`). The old port only dimmed
// the button that was pressed, leaving the rest of the cluster live, so a
// re-check could race an uninstall. Every other operation's in-flight mark
// (`disableBusy`, `removeBusy`, `uninstallBusy`, `toggleBusy`) locks the same
// cluster.
func TestEveryRowActionLocksWhileAnOperationIsInFlight(t *testing.T) {
	en := i18n.For("en").Settings

	press := func(t *testing.T, a *App) {
		t.Helper()
		tt := integrationPage(t, a, 720, 1200)
		for _, label := range []string{
			en.RunAction, en.IntegrationsCheck, en.IntegrationsReprobe,
			en.EditLabel, lockedRowIntegration().removalLabel(en),
		} {
			if err := tt.Click(label); err != nil {
				t.Fatalf("click %q: %v (texts %v)", label, err, tt.Texts())
			}
			tt.Frame()
		}
		if err := tt.Click(en.IntegrationsEnable); err != nil {
			t.Fatalf("click the switch: %v (texts %v)", err, tt.Texts())
		}
		tt.Frame()
	}

	// At rest every control answers: three actions reach the shell, Edit opens
	// the form, the removal door arms its bar, and the switch writes.
	resting := []string{}
	restingApp := lockedRowApp(&resting, nil)
	press(t, restingApp)
	want := []string{"run", "check", "reprobe", "enable"}
	if len(resting) != len(want) {
		t.Fatalf("at rest the row fired %v, want %v", resting, want)
	}
	for i := range want {
		if resting[i] != want[i] {
			t.Fatalf("at rest the row fired %v, want %v", resting, want)
		}
	}
	if restingApp.toolEditing != "v" {
		t.Errorf("Edit\u2026 did not open the form: toolEditing=%q", restingApp.toolEditing)
	}
	if !restingApp.removeConfirm["v"] {
		t.Errorf("the removal door did not arm its bar")
	}

	// Each in-flight mark locks the cluster: the same presses fire nothing, and
	// no door even opens.
	for _, mark := range []string{"disableBusy", "removeBusy", "uninstallBusy", "toggleBusy"} {
		t.Run(mark, func(t *testing.T) {
			calls := []string{}
			a := lockedRowApp(&calls, func(a *App) {
				switch mark {
				case "disableBusy":
					a.disableBusy = map[string]bool{"v": true}
				case "removeBusy":
					a.removeBusy = map[string]bool{"v": true}
				case "uninstallBusy":
					a.uninstallBusy = map[string]bool{"v": true}
				case "toggleBusy":
					a.toggleBusy = map[string]bool{"v": true}
				}
			})
			press(t, a)
			if len(calls) != 0 {
				t.Fatalf("a busy row still fired %v with %s set", calls, mark)
			}
			if a.toolEditing != "" {
				t.Errorf("Edit\u2026 opened the form with %s set: toolEditing=%q", mark, a.toolEditing)
			}
			if a.removeConfirm["v"] {
				t.Errorf("the removal door armed its bar with %s set", mark)
			}
			if a.disableConfirm["v"] {
				t.Errorf("the busy switch opened the disable confirmation with %s set", mark)
			}
		})
	}

	// The lock is the in-flight mark, not the row: clearing it frees the
	// cluster again.
	calls := []string{}
	a := lockedRowApp(&calls, nil)
	a.removeBusy = map[string]bool{"v": true}
	tt := integrationPage(t, a, 720, 1200)
	if err := tt.Click(en.IntegrationsCheck); err != nil {
		t.Fatalf("click check while busy: %v", err)
	}
	tt.Frame()
	if len(calls) != 0 {
		t.Fatalf("check fired while the row was busy: %v", calls)
	}
	delete(a.removeBusy, "v")
	tt.Frame()
	if err := tt.Click(en.IntegrationsCheck); err != nil {
		t.Fatalf("click check after the mark cleared: %v", err)
	}
	tt.Frame()
	if len(calls) != 1 || calls[0] != "check" {
		t.Fatalf("check did not fire once the mark cleared: %v", calls)
	}
}

// The reference swaps a busy switch's thumb for a turning LoaderCircle and
// disables it until the write lands (`settings-switch--loading`,
// `disabled={busy}`; `src/extensions/ExtensionRow.tsx:443-455`). The old port
// drew the resting thumb through the whole write and let the switch be pressed
// again. The busy face is a different picture from the resting one, and it
// cannot be flipped.
func TestTheBusySwitchSwapsItsThumbForASpinner(t *testing.T) {
	en := i18n.For("en").Settings
	draw := func(busy bool) (*ui.Tester, ui.Rect, *bool) {
		a := newTestApp()
		on := false
		tt := ui.NewTester(func(c *ui.Context) {
			if busy {
				a.busySwitchControl(c, &on, "extensions.toggle.test").Label(en.IntegrationsEnable)
			} else {
				a.switchControl(c, &on).Label(en.IntegrationsEnable)
			}
		}, 240, 80)
		tt.Frame()
		tt.Frame()
		box, _ := tt.Find(en.IntegrationsEnable)
		return tt, box, &on
	}

	idle, idleBox, idleOn := draw(false)
	busy, busyBox, busyOn := draw(true)
	if idleBox.W == 0 || busyBox.W == 0 {
		t.Fatalf("the switch has no box: idle=%v busy=%v", idleBox, busyBox)
	}
	if absFloat(idleBox.X-busyBox.X) > 1 || absFloat(idleBox.Y-busyBox.Y) > 1 ||
		absFloat(idleBox.W-busyBox.W) > 1 || absFloat(idleBox.H-busyBox.H) > 1 {
		t.Fatalf("the busy switch moved or resized: idle=%v busy=%v", idleBox, busyBox)
	}
	// The frame is a different picture: the bead is gone from the busy face.
	if diff := frameDiff(idle, busy, idleBox); diff < 20 {
		t.Errorf("the busy switch is the same picture as the resting one (%d differing pixels)", diff)
	}
	// It is also unpressable: the click does not flip the binding.
	if err := busy.Click(en.IntegrationsEnable); err != nil {
		t.Fatalf("click the busy switch: %v", err)
	}
	busy.Frame()
	if *busyOn {
		t.Errorf("the busy switch flipped while the write was in flight")
	}
	// The resting switch, for contrast, answers the same press.
	if err := idle.Click(en.IntegrationsEnable); err != nil {
		t.Fatalf("click the resting switch: %v", err)
	}
	idle.Frame()
	if !*idleOn {
		t.Errorf("the resting switch did not flip")
	}
}

// A write that is already in flight reaches the switch through the row's own
// busy marks too: `disableBusy` (the confirmation band's answer) and
// `toggleBusy` (the enable write) both draw the spinner, so the switch is not
// left looking pressable while the shell is working.
func TestTheRowSwitchWearsItsBusyFaceWhileTheWriteIsInFlight(t *testing.T) {
	en := i18n.For("en").Settings
	for _, mark := range []string{"disableBusy", "toggleBusy"} {
		t.Run(mark, func(t *testing.T) {
			a := lockedRowApp(&[]string{}, nil)
			a.Integrations = func() []Integration {
				return []Integration{{ID: "v", Name: "V Tools", Enabled: true, Running: true,
					RuntimeAvailable: true, Generated: true, DistributionSource: "local"}}
			}
			switch mark {
			case "disableBusy":
				a.disableBusy = map[string]bool{"v": true}
			case "toggleBusy":
				a.toggleBusy = map[string]bool{"v": true}
			}
			tt := integrationPage(t, a, 720, 1200)
			if _, ok := tt.Find(en.IntegrationsEnable); !ok {
				t.Fatalf("the switch is missing: %v", tt.Texts())
			}
			// Pressing it is a no-op while the write runs.
			if err := tt.Click(en.IntegrationsEnable); err != nil {
				t.Fatalf("click the switch: %v", err)
			}
			tt.Frame()
			if a.disableConfirm["v"] {
				t.Errorf("the busy switch opened the disable confirmation")
			}
		})
	}
}

// A broken row's switch is disabled: the reference kept the control in place
// but set `disabled={busy || extension.state === "broken"}`
// (`src/extensions/ExtensionRow.tsx:447`), so a broken integration cannot be
// toggled — turning it on writes a flag the runtime still cannot honour. The
// old port let the switch be flipped and ran `set_enabled` anyway.
func TestABrokenRowCannotBeToggled(t *testing.T) {
	en := i18n.For("en").Settings
	toggled := 0
	a := oneIntegrationApp(Integration{
		ID: "v", Name: "V Tools", Enabled: true, Running: false, Broken: true,
		RuntimeAvailable: false, FailureCode: extensions.BindingCodeMissing,
	}, Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) { toggled++ },
	})
	tt := integrationPage(t, a, 720, 1200)
	if err := tt.Click(en.IntegrationsEnable); err != nil {
		t.Fatalf("the broken row's switch is missing: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if toggled != 0 {
		t.Errorf("a broken row's switch fired set_enabled %d times", toggled)
	}
	if a.disableConfirm["v"] {
		t.Errorf("a broken row's switch opened the disable confirmation")
	}
}

// frameDiff counts the pixels two frames differ by inside a box.
func frameDiff(a, b *ui.Tester, box ui.Rect) int {
	ia, ib := a.Image(), b.Image()
	n := 0
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		if y < 0 || y >= ia.Rect.Dy() || y >= ib.Rect.Dy() {
			continue
		}
		for x := int(box.X); x < int(box.X+box.W); x++ {
			if x < 0 || x >= ia.Rect.Dx() || x >= ib.Rect.Dx() {
				continue
			}
			i := y*ia.Stride + x*4
			j := y*ib.Stride + x*4
			if ia.Pix[i] != ib.Pix[j] || ia.Pix[i+1] != ib.Pix[j+1] || ia.Pix[i+2] != ib.Pix[j+2] {
				n++
			}
		}
	}
	return n
}
