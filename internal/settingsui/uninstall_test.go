package settingsui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The componentized uninstall dialog is the reference's own four rows
// (`src/extensions/ComponentizedUninstallDialog.tsx:84-115`): program files
// first — checked and disabled, because the program always goes — then host
// configuration, tool data and generated artifacts. This build's dialog had
// only the three data rows, so the disclosure that program files are removed
// anyway was nowhere on screen.
func TestComponentizedUninstallListsAllFourComponents(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings
	var installed UninstallComponents
	parts := uninstallParts(en, &installed)
	if len(parts) != 4 {
		t.Fatalf("uninstallParts returned %d rows, want 4: %+v", len(parts), parts)
	}
	wantEn := []struct{ label, hint string }{
		{en.UninstallProgram, en.UninstallProgramHint},
		{en.UninstallHostConfig, en.UninstallHostConfigHint},
		{en.UninstallToolData, en.UninstallToolDataHint},
		{en.UninstallArtifacts, en.UninstallArtifactsHint},
	}
	for i, want := range wantEn {
		if parts[i].Label != want.label || parts[i].Hint != want.hint {
			t.Errorf("row %d = %q/%q, want %q/%q", i, parts[i].Label, parts[i].Hint, want.label, want.hint)
		}
	}
	// The program row is the fixed one — checked, disabled, and writing
	// nowhere; the other three point at the draft's own fields.
	if !parts[0].Fixed || parts[0].Value != nil {
		t.Errorf("the program row is not fixed: %+v", parts[0])
	}
	for i, field := range []*bool{&installed.RemoveHostConfig, &installed.RemoveToolData, &installed.RemoveArtifacts} {
		if parts[i+1].Fixed || parts[i+1].Value != field {
			t.Errorf("row %d does not write the draft field %d: %+v", i+1, i, parts[i+1])
		}
	}
	// The two languages name the program row differently, so a test against
	// one string cannot pass by both.
	if zh.UninstallProgram == en.UninstallProgram || zh.UninstallProgramHint == en.UninstallProgramHint {
		t.Errorf("the program row is not localized: en=%q/%q zh=%q/%q",
			en.UninstallProgram, en.UninstallProgramHint, zh.UninstallProgram, zh.UninstallProgramHint)
	}
}

// The same rows reach the screen, under the reference's own title: the title
// names the integration (`uninstallComponentsTitle`, "Uninstall {name}"), not
// this build's invented "Uninstall this integration".
func TestComponentizedUninstallDrawsTheReferenceRows(t *testing.T) {
	en := i18n.For("en").Settings
	var uninstalled []UninstallComponents
	a := newTestApp()
	a.Actions = Actions{
		UninstallComponents: func(id, name, kind string, components UninstallComponents) {
			uninstalled = append(uninstalled, components)
		},
	}
	a.Integrations = func() []Integration {
		return []Integration{{ID: "npm", Name: "NPM Tool", DistributionSource: "npm"}}
	}
	tt := render(t, a, 720, 1400)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("click Integrations: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	// The row's own button expands the dialog.
	if err := tt.Click(en.IntegrationsUninstall + "…"); err != nil {
		t.Fatalf("click the uninstall button: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()

	if !tt.HasText(en.IntegrationsUninstallTitle("NPM Tool")) {
		t.Errorf("the dialog does not name the integration: %v", tt.Texts())
	}
	if tt.HasText("Uninstall this integration") {
		t.Errorf("the dialog still shows the invented title: %v", tt.Texts())
	}
	if !tt.HasText(en.IntegrationsUninstallDescription) {
		t.Errorf("the dialog is missing its hint: %v", tt.Texts())
	}
	// All four rows, in the reference's order.
	order := []string{en.UninstallProgram, en.UninstallHostConfig, en.UninstallToolData, en.UninstallArtifacts}
	previous := float32(-1)
	for _, label := range order {
		box, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the dialog is missing the %q row: %v", label, tt.Texts())
		}
		if box.Y <= previous {
			t.Errorf("the %q row is out of order (y=%v after %v)", label, box.Y, previous)
		}
		previous = box.Y
	}
	for _, hint := range []string{
		en.UninstallProgramHint, en.UninstallHostConfigHint, en.UninstallToolDataHint, en.UninstallArtifactsHint,
	} {
		if !tt.HasText(hint) {
			t.Errorf("the dialog is missing the hint %q: %v", hint, tt.Texts())
		}
	}

	// Unchecking a data category reaches the payload; the program row is not
	// a field of it (the shell removes the program unconditionally).
	if err := tt.Click(en.UninstallHostConfig); err != nil {
		t.Fatalf("click host configuration: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if err := tt.Click(en.IntegrationsUninstall); err != nil {
		t.Fatalf("click the confirm button: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if len(uninstalled) != 1 {
		t.Fatalf("the confirm button called the action %d times, want 1", len(uninstalled))
	}
	if uninstalled[0].RemoveHostConfig {
		t.Errorf("unchecking host configuration did not reach the payload: %+v", uninstalled[0])
	}
	if !uninstalled[0].RemoveToolData || !uninstalled[0].RemoveArtifacts {
		t.Errorf("the untouched categories did not stay checked: %+v", uninstalled[0])
	}
}

// The two actions keep the reference's order: Cancel first, the destructive
// button last (`ComponentizedUninstallDialog.tsx:119-141`).
func TestComponentizedUninstallKeepsCancelFirst(t *testing.T) {
	en := i18n.For("en").Settings
	a := newTestApp()
	a.Actions = Actions{UninstallComponents: func(string, string, string, UninstallComponents) {}}
	a.Integrations = func() []Integration {
		return []Integration{{ID: "npm", Name: "NPM Tool", DistributionSource: "npm"}}
	}
	tt := render(t, a, 720, 1400)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("click Integrations: %v", err)
	}
	tt.Frame()
	if err := tt.Click(en.IntegrationsUninstall + "…"); err != nil {
		t.Fatalf("click the uninstall button: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	cancel, ok := tt.Find(en.PermissionsCancel)
	if !ok {
		t.Fatalf("the dialog is missing Cancel: %v", tt.Texts())
	}
	confirm, ok := tt.Find(en.IntegrationsUninstall)
	if !ok {
		t.Fatalf("the dialog is missing the confirm button: %v", tt.Texts())
	}
	if cancel.X >= confirm.X {
		t.Errorf("Cancel (x=%v) is not before the confirm button (x=%v)", cancel.X, confirm.X)
	}
}

// The reference floats the dialog over a full-window scrim:
// `.extensions-dialog-overlay` is a fixed `inset: 0` grid at `z-index: 40`
// with `padding: 20px` and `background: var(--surface-scrim)`
// (`ComponentizedUninstallDialog.css:6-16`). The build used to draw the pane
// straight into the page, so everything behind it stayed at full strength.
// DialogBase now supplies that backdrop; the corner of the window is the scrim
// over an otherwise white pane.
func TestComponentizedUninstallScrimCoversTheWindow(t *testing.T) {
	en := i18n.For("en").Settings
	a := uninstallTestApp()
	var ref ui.Handle
	open := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Background(ui.RGB(255, 255, 255))
		a.uninstallDialog(c, en, npmTool()).Bind(&ref)
	}, 640, 480)
	open.Frame()
	// The same white window with nothing over it: the scrim is the only
	// difference at the corner.
	plain := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Background(ui.RGB(255, 255, 255))
	}, 640, 480)
	plain.Frame()
	if got, want := cornerRGB(open), cornerRGB(plain); got >= want {
		t.Errorf("the dialog's scrim does not darken the window: open %.0f, plain %.0f", got, want)
	}
}

// cornerRGB is the sum of one window corner's channels — the scrim's own
// light budget over a constant background.
func cornerRGB(tt *ui.Tester) float64 {
	c := tt.Image().RGBAAt(4, 4)
	return float64(c.R) + float64(c.G) + float64(c.B)
}

// The reference's capture-phase listener answers Escape with `onCancel`, and
// does so before the key reaches the page (`event.stopImmediatePropagation`).
// DialogBase carries that key: the dialog is gone after Escape, and the open
// state the page reads is cleared.
func TestComponentizedUninstallEscapeClosesTheDialog(t *testing.T) {
	en := i18n.For("en").Settings
	a := uninstallTestApp()
	a.uninstallOpen = map[string]bool{"npm": true}
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		if a.uninstallOpen["npm"] {
			a.uninstallDialog(c, en, npmTool()).Bind(&ref)
		}
	}, 640, 480)
	tt.Frame()
	tt.Frame()
	if !tt.HasText(en.IntegrationsUninstallDescription) {
		t.Fatalf("the dialog did not open: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.uninstallOpen["npm"] {
		t.Errorf("Escape left the dialog open")
	}
	if tt.HasText(en.IntegrationsUninstallDescription) {
		t.Errorf("the dialog is still on screen after Escape: %v", tt.Texts())
	}
}

// The reference focuses the cancel button on mount
// (`cancelRef.current?.focus({ preventScroll: true })`) and gives the focus
// back to whatever had it on unmount. DialogBase hands the focus to the pane's
// `AutoFocus` child and restores the opener when the overlay goes.
func TestComponentizedUninstallManagesFocus(t *testing.T) {
	en := i18n.For("en").Settings
	a := uninstallTestApp()
	a.uninstallOpen = map[string]bool{}
	const trigger = "Open uninstall"
	tt := ui.NewTester(func(c *ui.Context) {
		if ui.Button(c, trigger).Clicked() {
			a.uninstallOpen["npm"] = true
		}
		if a.uninstallOpen["npm"] {
			a.uninstallDialog(c, en, npmTool())
		}
	}, 720, 640)
	tt.Frame()
	if err := tt.Click(trigger); err != nil {
		t.Fatalf("click the trigger: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	tt.Frame()
	if !tt.Focused(en.PermissionsCancel) {
		t.Fatalf("the dialog did not focus Cancel on open: %v", tt.Texts())
	}
	if err := tt.Click(en.PermissionsCancel); err != nil {
		t.Fatalf("click Cancel: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	tt.Frame()
	if a.uninstallOpen["npm"] {
		t.Fatalf("Cancel left the dialog open")
	}
	if !tt.Focused(trigger) {
		t.Errorf("closing the dialog did not restore focus to the opener")
	}
}

// The reference disables both actions while `busy`
// (`ComponentizedUninstallDialog.tsx:126,142`), so a question already being
// answered cannot be answered twice. The action pill follows: the same button
// reports its press when idle and ignores it while busy.
func TestComponentizedUninstallActionsDisableWhileBusy(t *testing.T) {
	en := i18n.For("en").Settings
	a := uninstallTestApp()
	for _, busy := range []bool{false, true} {
		busy := busy
		t.Run(fmt.Sprintf("busy=%v", busy), func(t *testing.T) {
			clicked := false
			tt := ui.NewTester(func(c *ui.Context) {
				if a.uninstallActionButton(c, en.IntegrationsUninstall, true, busy, checkIcon).Clicked() {
					clicked = true
				}
			}, 320, 120)
			tt.Frame()
			if err := tt.Click(en.IntegrationsUninstall); err != nil {
				t.Fatalf("click the action: %v (texts %v)", err, tt.Texts())
			}
			tt.Frame()
			if clicked != !busy {
				t.Errorf("clicked = %v with busy=%v, want %v", clicked, busy, !busy)
			}
		})
	}
}
