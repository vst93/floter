package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func checkConditionalOutletVisibility(t *testing.T, field string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := declarationVisibilityApp(t, dark, 5)
			copy := a.copy()
			a.toolParams[0].Options = "small\nlarge"
			a.toolParams[0].Placeholder = "保留提示"
			if field == "options" {
				a.toolParams[0].Kind = extensions.ParamSelect
			}
			var setup func(*ui.Context)
			var target, viewport ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				if setup != nil {
					f := setup
					setup = nil
					f(c)
				}
				viewport = c.Resolve(a.Body.Handle).Bounds()
				target = c.Resolve(a.paramRefusalTargets(c, &a.toolParams[0]).id).Bounds()
			})
			setup = func(c *ui.Context) { c.Resolve(a.paramRefusalTargets(c, &a.toolParams[0]).id).Focus().ScrollIntoView() }
			tt.Frame()
			label := copy.ParamsPlaceholder
			if field == "options" {
				label = copy.ParamsOptions
			}
			for n := 0; n < 10 && !tt.Focused(label); n++ {
				tt.Key(0, ui.KeyTab)
			}
			if !tt.Focused(label) {
				t.Fatal("conditional input setup failed")
			}
			// Scrolling while an editor owns the keyboard is legal. A refresh
			// must reveal its replacement only when that editor disappears.
			a.Body.Y = a.Body.MaxY
			tt.Frame()
			if field == "options" {
				a.toolParams[0].Kind = extensions.ParamText
			} else {
				a.toolParams[0].Kind = extensions.ParamBoolean
			}
			tt.Frame()
			tt.Frame()
			declarationVisibilityDump(t, tt, "conditional-"+field, dark)
			// The ID and kind share the same 30px grid row. Probe this record's
			// handle instead of the first repeated caption elsewhere in the list.
			declarationVisible(t, a, tt, copy.ParamsKind, target, viewport)
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyHome)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if a.toolParams[0].Kind != extensions.ParamText || a.toolParams[0].Options != "small\nlarge" || a.toolParams[0].Placeholder != "保留提示" {
				t.Error("type outlet lost declaration draft or keyboard")
			}
		})
	}
}

func TestConditionalOutletVisibilityOptions(t *testing.T) {
	checkConditionalOutletVisibility(t, "options")
}
func TestConditionalOutletVisibilityPlaceholder(t *testing.T) {
	checkConditionalOutletVisibility(t, "placeholder")
}

func checkModeOutletVisibility(t *testing.T, script bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := declarationVisibilityApp(t, dark, 5)
			a.toolScript, a.toolLanguage = "console.log('weather');", "javascript"
			if script {
				a.toolMode = routeScript
			}
			program, source := a.toolProgram, a.toolScript
			var setup func(*ui.Context)
			var target, viewport ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				if setup != nil {
					f := setup
					setup = nil
					f(c)
				}
				target = c.Resolve(a.contentFocus(c, "mode-program").source).Bounds()
				viewport = c.Resolve(a.Body.Handle).Bounds()
			})
			key := "plan"
			if script {
				key = "script"
			}
			setup = func(c *ui.Context) { c.Resolve(a.contentFocus(c, key).source).Focus().ScrollIntoView() }
			tt.Frame()
			tt.Frame()
			a.Body.Y = a.Body.MaxY
			tt.Frame()
			if script {
				a.toolMode = ""
			} else {
				a.toolMode = routeScript
			}
			tt.Frame()
			tt.Frame()
			name := "program"
			if script {
				name = "script"
			}
			declarationVisibilityDump(t, tt, "mode-"+name, dark)
			declarationVisible(t, a, tt, a.copy().LocalToolMode, target, viewport)
			tt.Key(0, ui.KeyTab)
			if script {
				tt.Key(0, ui.KeyRight)
			} else {
				tt.Key(0, ui.KeySpace)
			}
			tt.Frame()
			if script != (a.toolMode == routeScript) || a.toolProgram != program || a.toolScript != source {
				t.Error("mode outlet lost draft or keyboard")
			}
		})
	}
}
