package settingsui

import (
	"floter/internal/settings"
	"fmt"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"testing"
)

// P13-hl: neutral disabled labels share one pre-fade ink. The 70..88
// rendered band sits above the existing 60-level floor and below enabled ink.
func assertDisabledInk(t *testing.T, draw func(*App, *ui.Context) ui.Element) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		_, r := dimProbe(t, dark, draw)
		levels := r.inkContrast / 3
		t.Logf("%s: ink contrast %d", appearanceName(dark), levels)
		if levels < 70 || levels > 88 {
			t.Errorf("%s: disabled ink contrast = %d, want shared 70..88 rung", appearanceName(dark), levels)
		}
	}
}

func disabledLabelSites() []struct {
	name string
	draw func(*App, *ui.Context) ui.Element
} {
	return []struct {
		name string
		draw func(*App, *ui.Context) ui.Element
	}{
		{"action", func(a *App, c *ui.Context) ui.Element { return a.disabledActionButton(c, "Check") }},
		{"busy", func(a *App, c *ui.Context) ui.Element { return a.busyActionButton(c, "Refresh", "hl") }},
		{"configuration", func(a *App, c *ui.Context) ui.Element {
			th := a.cardTheme(c)
			return a.extensionGlyphActionButton(c, "link", "Open configuration", th.Space(7.5), th.Space(3.75), a.tokens(c).Body, true).Disabled(true)
		}},
		{"extension-icon", func(a *App, c *ui.Context) ui.Element { return a.extensionIconButton(c, "settings", "Configure", true) }},
		{"session-icon", func(a *App, c *ui.Context) ui.Element {
			return wrapControl(c, func() { a.sessionIconButton(c, "play", "Resume", false, false, true, func() {}) })
		}},
		{"confirmation", func(a *App, c *ui.Context) ui.Element { return a.barButton(c, "Discard", false, true, nil, nil) }},
		{"uninstall", func(a *App, c *ui.Context) ui.Element {
			return a.uninstallActionButton(c, "Uninstall", false, true, nil)
		}},
		{"fixed-part", func(a *App, c *ui.Context) ui.Element {
			return a.uninstallPartRow(c, uninstallPart{Label: "Floter program", Hint: "Always removed", Fixed: true})
		}},
		{"danger-icon", func(a *App, c *ui.Context) ui.Element {
			return a.barButton(c, "Remove", true, false, mustGlyph("power"), nil)
		}},
		{"disabled-icon-label", func(a *App, c *ui.Context) ui.Element {
			return a.barButton(c, "Remove", false, true, mustGlyph("power"), nil)
		}},
	}
}

func TestDisabledActionLabelsShareInk(t *testing.T) {
	for _, site := range disabledLabelSites()[:3] {
		t.Run(site.name, func(t *testing.T) { assertDisabledInk(t, site.draw) })
	}
}

func TestDisabledExtensionIconSharesInk(t *testing.T) {
	assertDisabledInk(t, disabledLabelSites()[3].draw)
}

func TestDisabledSessionIconSharesInk(t *testing.T) {
	assertDisabledInk(t, disabledLabelSites()[4].draw)
}

func TestDisabledConfirmationLabelSharesInk(t *testing.T) {
	assertDisabledInk(t, disabledLabelSites()[5].draw)
}

func TestBusyUninstallLabelSharesInk(t *testing.T) {
	assertDisabledInk(t, disabledLabelSites()[6].draw)
}

func TestFixedUninstallHintKeepsReadableInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, hint := range []string{"Always removed", "应用本体始终移除"} {
			store := settings.NewStore(settings.Default())
			if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{})
			ground := realPageGround(t, dark)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(ground)
				ui.Column(c).Padding(24).Width(360).Children(func() {
					a.uninstallPartRow(c, uninstallPart{Label: "Floter", Hint: hint, Fixed: true})
				})
			}, 420, 160)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			box := mustFind(t, tt, hint)
			face := pageLuma(tt, 330, box.Y+box.H/2)
			ink := transferBusyInk(tt, face, box)
			t.Logf("%s %s: hint contrast %d", appearanceName(dark), hint, ink)
			if ink < 70 || ink > 88 {
				t.Errorf("%s: fixed hint contrast %d, want 70..88", appearanceName(dark), ink)
			}
		}
	}
}

func TestConfirmationIconUsesLabelSemantics(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, disabled := range []bool{false, true} {
			_, r := oneFadeProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
				return a.barButton(c, "Remove", true, disabled, mustGlyph("power"), nil)
			})
			t.Logf("%s disabled=%v: strongest ink %+v", appearanceName(dark), disabled, r.ink)
			if r.ink.R <= r.ink.G || r.ink.R <= r.ink.B {
				t.Errorf("%s disabled=%v: confirmation icon lost warning hue: %+v", appearanceName(dark), disabled, r.ink)
			}
		}
	}
	assertDisabledInk(t, disabledLabelSites()[9].draw)
}

// Opt-in evidence uses actual component builders and identical geometry.
func TestDumpP13HLDisabledInk(t *testing.T) {
	if os.Getenv("FLOTER_P13HL_DUMP") == "" {
		t.Skip("opt-in screenshot: FLOTER_P13HL_DUMP=1")
	}
	tag := os.Getenv("FLOTER_P13HL_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		ground := realPageGround(t, dark)
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			c.Root().Background(ground)
			ui.Column(c).Padding(24).Width(360).Gap(10).Children(func() {
				for _, site := range disabledLabelSites() {
					wrapControl(c, func() { site.draw(a, c) })
				}
			})
		}, 420, 460)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		name := fmt.Sprintf("/tmp/floter-P13-hl-disabled-ink-%s-%s.png", appearanceName(dark), tag)
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Log(name)
	}
}
