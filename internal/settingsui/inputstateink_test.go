package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

func inputStateInk(t *testing.T, dark bool, name string, draw func(*App, *ui.Context)) {
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
