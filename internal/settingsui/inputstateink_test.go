package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/settings"
)

func inputStateInkFrame(t *testing.T, dark bool, draw func(*App, *ui.Context)) (*App, *ui.Tester) {
	t.Helper()
	s := settings.Default()
	s.Theme, s.Language = appearanceName(dark), "zh"
	a := New(settings.NewStore(s), Actions{})
	a.toggleBusy = map[string]bool{"local.input": true}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		ui.Column(c).Padding(20).FillWidth().Children(func() {
			a.extensionMetrics(func() { draw(a, c) })
		})
	}, 720, 320)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return a, tt
}

func inputStateInk(t *testing.T, dark bool, name string, draw func(*App, *ui.Context)) {
	t.Helper()
	_, tt := inputStateInkFrame(t, dark, draw)
	inputStateDump(t, tt, name, dark)
	// The configuration label ends at x=138. Sample the text-bearing middle
	// of its trailing input, excluding the border and outer focus band.
	r := mustFind(t, tt, "锁定字段")
	levels := inkSpan(tt, ui.Rect{X: 170, Y: r.Y, W: 230, H: r.H})
	t.Logf("%s/%s: disabled ink=%d", name, appearanceName(dark), levels)
	if levels < 70 || levels > 88 {
		t.Errorf("%s: disabled input ink=%d, want shared 70..88 rung", name, levels)
	}
}

func TestInputStateLockedConfigUsesDisabledInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, kind := range []string{"text", "number"} {
			t.Run(appearanceName(dark)+"/"+kind, func(t *testing.T) {
				inputStateInk(t, dark, "locked-config-"+kind, func(a *App, c *ui.Context) {
					field := ConfigField{Key: "value", Label: "锁定字段", Type: kind}
					var value any = "1234567890"
					if kind == "number" {
						value = 1234567890.0
					}
					draft := &configDraft{values: map[string]any{field.Key: value}}
					a.configField(c, a.copy(), Integration{ID: "local.input"}, draft, field)
				})
			})
		}
	}
}

func TestInputStateLockedAliasUsesDisabledInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, tt := inputStateInkFrame(t, dark, func(a *App, c *ui.Context) {
				a.commandAliasEditor(c, a.copy(), "local.input", Command{ID: "scan", Name: "扫描", Alias: "1234567890"})
			})
			inputStateDump(t, tt, "locked-alias", dark)
			r := mustFind(t, tt, a.copy().IntegrationsCommandAliasFor("扫描"))
			levels := inkSpan(tt, ui.Rect{X: r.X + 10, Y: r.Y + 3, W: 200, H: r.H - 6})
			t.Logf("%s alias: disabled ink=%d", appearanceName(dark), levels)
			if levels < 70 || levels > 88 {
				t.Errorf("alias disabled ink=%d, want shared 70..88 rung", levels)
			}
			if err := tt.Click(a.copy().IntegrationsCommandAliasFor("扫描")); err != nil {
				t.Fatal(err)
			}
			tt.Type("blocked")
			tt.Frame()
			if len(a.pendingAliases) != 0 {
				t.Fatal("locked alias queued a write")
			}
		})
	}
}

func TestInputStateLockedRunParameterUsesDisabledInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, tt := inputStateInkFrame(t, dark, func(a *App, c *ui.Context) {
				a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure { return nil }
				a.runOpen = map[string]bool{"local.input": true}
				if a.runValues == nil {
					a.runValues = map[string]map[string]string{"local.input": {"target": "1234567890"}}
				}
				a.runForm(c, a.copy(), Integration{ID: "local.input", Running: true, Enabled: true, Params: []Param{{ID: "target", Label: "锁定字段", Kind: extensions.ParamText}}})
			})
			inputStateDump(t, tt, "locked-run", dark)
			r := mustFind(t, tt, "锁定字段")
			levels := inkSpan(tt, ui.Rect{X: 480, Y: r.Y, W: 195, H: r.H})
			t.Logf("%s run: disabled ink=%d", appearanceName(dark), levels)
			if levels < 70 || levels > 88 {
				t.Errorf("run parameter disabled ink=%d, want shared 70..88 rung", levels)
			}
			tt.ClickAt(568, r.Y+r.H/2)
			tt.Type("blocked")
			tt.Frame()
			if a.runValues["local.input"]["target"] != "1234567890" {
				t.Fatal("locked run parameter changed")
			}
		})
	}
}
