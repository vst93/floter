package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// measureExtensionField renders one text field at an interface step and reads
// its box height and the distance from the box's left edge to its first ink —
// the horizontal inset a value's text starts at. It draws the extension panel's
// own field through `extensionField`/`extensionMetrics` when extension is set,
// and the framework's stock field (what a General-page `a.text` row builds)
// otherwise. The field is typed into so the text, not the border or a rounded
// corner, is what the horizontal scan finds.
type fieldMeasurement struct{ height, inset float32 }

func measureField(t *testing.T, step string, extension bool) fieldMeasurement {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.UIScale = step }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	value := ""
	var ref ui.Handle
	var rect ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		ui.Column(c).FillWidth().FillHeight().Padding(24).Children(func() {
			build := func() {
				if extension {
					a.extensionField(c, &value, configFieldMetrics).Bind(&ref).Width(200)
				} else {
					a.textInput(c, &value).Bind(&ref).Width(200)
				}
			}
			if extension {
				a.extensionMetrics(build)
			} else {
				build()
			}
		})
		rect = c.Resolve(ref).Bounds()
	}, 400, 120)
	tt.Frame()
	tt.ClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
	tt.Type("WWW")
	tt.Frame()
	if value != "WWW" {
		t.Fatalf("[%s] could not type into the field: value = %q", step, value)
	}

	img := tt.Image()
	bg := imagePixel(img, int(rect.X+rect.W-8), int(rect.Y+rect.H/2))
	cy := int(rect.Y + rect.H/2)
	inset := float32(-1)
	for x := int(rect.X) + 3; x < int(rect.X+rect.W) && inset < 0; x++ {
		for y := cy - 4; y <= cy+4; y++ {
			if channelDelta(imagePixel(img, x, y), bg) > 30 {
				inset = float32(x) - rect.X
				break
			}
		}
	}
	if inset < 0 {
		t.Fatalf("[%s] found no text inside the field at %+v", step, rect)
	}
	return fieldMeasurement{height: rect.H, inset: inset}
}

// The extension panel's fields are written in literal px: a config field is a
// 28px floor with 4px/8px padding and a 999px pill (`.extension-config-field
// input`, extensions.css:2402-2417), and a custom-form field is a 30px floor
// with 3px/7px padding (`.extension-custom-form input`, extensions.css:1770-
// 1781). The framework sizes its own field from the context theme, whose
// `Spacing` follows `--ui-scale`, so the port drew the panel's fields 10%
// smaller at `small` and 10% larger at `large`. This pins the acceptance: at
// the two shipped steps the extension field's box and its text inset are the
// same (the reference's 1px border + 8px padding = a 9px inset), while a
// General-page field still follows the step.
func TestTheExtensionFieldKeepsItsMetricsAtEveryInterfaceStep(t *testing.T) {
	small, large := measureField(t, "small", true), measureField(t, "large", true)

	if absFloat(small.inset-9) > 1 || absFloat(large.inset-9) > 1 {
		t.Errorf("the extension field's text starts %vpx (small) / %vpx (large) inside its box, want the reference's 1px border + 8px padding = 9px",
			small.inset, large.inset)
	}
	if absFloat(small.inset-large.inset) > 1 {
		t.Errorf("the extension field's horizontal padding moves with the interface step: %vpx at small, %vpx at large",
			small.inset, large.inset)
	}
	if absFloat(small.height-28) > 1 || absFloat(large.height-28) > 1 {
		t.Errorf("the extension field's box is %vpx (small) / %vpx (large), want the reference's 28px floor",
			small.height, large.height)
	}
	if absFloat(small.height-large.height) > 1 {
		t.Errorf("the extension field's box height moves with the interface step: %vpx at small, %vpx at large",
			small.height, large.height)
	}

	// The contrast that proves the pin is scoped: a General-page field keeps
	// the framework's `--u` metrics and grows with the step.
	setSmall, setLarge := measureField(t, "small", false), measureField(t, "large", false)
	if setLarge.height-setSmall.height < 3 {
		t.Errorf("the settings field is %vpx at small and %vpx at large; it must keep following the interface step",
			setSmall.height, setLarge.height)
	}
	if setLarge.inset-setSmall.inset < 2 {
		t.Errorf("the settings field's padding is %vpx at small and %vpx at large; it must keep following the interface step",
			setSmall.inset, setLarge.inset)
	}
}

// The custom-tool form's segmented picker is the reference's
// `.extension-custom-mode` (`extensions.css:1752-1759`): a 3px track padding and
// a 2px gap in literal px. The port drew the framework's `Segmented` straight
// from the context theme, so the track's inset was `Space(0.5)` and followed
// the interface step. This reads the first segment label's distance from the
// track's left edge at both steps, and pins the settings-side segmented control
// beside it as the one that still scales.
func TestTheExtensionSegmentedControlKeepsItsInsetAtEveryInterfaceStep(t *testing.T) {
	measure := func(step string, extension bool) float32 {
		t.Helper()
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.UIScale = step }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		index := 0
		var ref ui.Handle
		var rect ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			ui.Column(c).FillWidth().FillHeight().Padding(24).Children(func() {
				build := func() {
					if extension {
						a.segmentedControl(c, &index, "Program", "Script").Bind(&ref)
					} else {
						a.tokens(c).LensInset(ui.Segmented(c, &index, "Program", "Script")).Bind(&ref)
					}
				}
				if extension {
					a.extensionMetrics(build)
				} else {
					build()
				}
			})
			rect = c.Resolve(ref).Bounds()
		}, 400, 120)
		tt.Frame()
		label, ok := tt.Find("Program")
		if !ok {
			t.Fatalf("[%s] the segmented control's label is missing", step)
		}
		return label.X - rect.X
	}

	small, large := measure("small", true), measure("large", true)
	if absFloat(small-large) > 1 {
		t.Errorf("the extension segmented control's inset moves with the interface step: %vpx at small, %vpx at large",
			small, large)
	}
	// The settings-side control (drawn with the scaled theme) still follows it.
	setSmall, setLarge := measure("small", false), measure("large", false)
	if setLarge-setSmall < 2 {
		t.Errorf("the settings segmented control's inset is %vpx at small and %vpx at large; it must keep following the step",
			setSmall, setLarge)
	}
}

// The extension panel's drop-down is the reference's `.extension-config-field
// select`, which shares the input's literal box (`extensions.css:2402-2417`):
// a 28px floor with 4px/8px padding and a 999px pill. The port drew the
// framework's Select straight from the context theme, so the trigger grew and
// shrank with `--ui-scale`. This pins the trigger's box at the two steps, and
// the settings-side select beside it as the one that still scales.
func TestTheExtensionSelectKeepsItsMetricsAtEveryInterfaceStep(t *testing.T) {
	measure := func(step string, extension bool) float32 {
		t.Helper()
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.UIScale = step }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		selected := ""
		var ref ui.Handle
		var rect ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			ui.Column(c).FillWidth().FillHeight().Padding(24).Children(func() {
				build := func() {
					a.chooseSelect(c, &selected, []string{"One", "Two"}, "Size").Bind(&ref)
				}
				if extension {
					a.extensionMetrics(build)
				} else {
					build()
				}
			})
			rect = c.Resolve(ref).Bounds()
		}, 400, 120)
		tt.Frame()
		return rect.H
	}

	small, large := measure("small", true), measure("large", true)
	if absFloat(small-28) > 1 || absFloat(large-28) > 1 {
		t.Errorf("the extension select is %vpx (small) / %vpx (large), want the reference's 28px floor", small, large)
	}
	if absFloat(small-large) > 1 {
		t.Errorf("the extension select's box moves with the interface step: %vpx at small, %vpx at large", small, large)
	}
	setSmall, setLarge := measure("small", false), measure("large", false)
	if setLarge-setSmall < 3 {
		t.Errorf("the settings select is %vpx at small and %vpx at large; it must keep following the step",
			setSmall, setLarge)
	}
}

// The run form's inputs live in the reference's `.extension-run-params` box,
// which pads its rows 8px (`extensions.css:1224-1236`). The port drew the form
// in a settings card, whose rows carry a 14u inset, so a parameter's row sat
// 1px + 14px = 15px inside the box rather than the reference's 1px + 8px = 9px.
// This reads the parameter label's x against the form box's left edge (the
// heading's own x) and requires the reference's 9px.
func TestTheRunFormRowsSitOnTheBoxsOwnPadding(t *testing.T) {
	en := i18n.For("en").Settings
	for _, step := range []string{"small", "large"} {
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.UIScale = step }); err != nil {
			t.Fatal(err)
		}
		a := oneIntegrationApp(Integration{
			ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
			Enabled: true, Running: true, RuntimeAvailable: true,
			Permissions: []string{"filesystem"},
			Enforced:    map[string]bool{"filesystem": true},
			Params:      []Param{{ID: "target", Label: "Target", Kind: "text"}},
		}, Actions{RunWithParams: func(string, map[string]string) *extensions.ParamFailure { return nil }})
		a.Store = store
		a.runOpen = map[string]bool{"dev.floter.tool": true}
		tt := integrationPage(t, a, 720, 1800)

		header := mustFind(t, tt, en.RunValues)
		target := mustFind(t, tt, "Target")
		if got := target.X - header.X; absFloat(got-9) > 1 {
			t.Errorf("[%s] the run form's input row starts %vpx inside its box, want the reference's 1px border + 8px padding = 9px",
				step, got)
		}
	}
}
