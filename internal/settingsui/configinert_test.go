package settingsui

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

func configEntryFrame(t *testing.T, dark bool, language string, command, handler bool) (*ui.Tester, ui.Rect, ui.Rect) {
	t.Helper()
	s := settings.Default()
	s.Theme, s.Language = appearanceName(dark), language
	a := New(settings.NewStore(s), Actions{})
	if handler {
		a.Actions.OpenConfiguration = func(string, []string) {}
	}
	integration := Integration{ID: "local.ripgrep"}
	if command {
		integration.ConfigOpenCommand = []string{"rg", "--help"}
	}
	copy := i18n.For(language).Settings
	ground := realPageGround(t, dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Children(func() {
			a.card(c, func() { a.configOpenToolRow(c, copy, integration) })
		})
	}, 720, 140)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt, mustFind(t, tt, copy.ConfigSection), mustFind(t, tt, copy.ConfigOpenConfiguration)
}

func TestTheConfigurationEntryOnlyLiftsWhenItCanOpen(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, language := range []string{"en", "zh"} {
			for _, state := range []struct {
				name             string
				command, handler bool
			}{{"no command", false, true}, {"no handler", true, false}, {"available", true, true}} {
				tt, label, button := configEntryFrame(t, dark, language, state.command, state.handler)
				rest := rowInsetFace(tt.Image(), label)
				ink := inkSpan(tt, label)
				settlePointer(tt, label)
				hover := rowInsetFace(tt.Image(), label)
				// The label column has its own Clicked target. Press the row's
				// empty leading inset to exercise the row's Pressed arm itself.
				tt.Press(float32(label.X-10), float32(label.Y+label.H/2))
				tt.Frame()
				tt.Advance(300 * time.Millisecond)
				tt.Frame()
				pressed := rowInsetFace(tt.Image(), label)
				openable := state.command && state.handler
				if !openable && (hover != rest || pressed != rest) {
					t.Errorf("%s/%s/%s: unavailable row changed face: rest=%v hover=%v pressed=%v",
						appearanceName(dark), language, state.name, rest, hover, pressed)
				}
				if openable && (hover == rest || pressed == rest || hover == pressed) {
					t.Errorf("%s/%s: available row lost its distinct hover/press faces: %v %v %v",
						appearanceName(dark), language, rest, hover, pressed)
				}
				copy := i18n.For(language).Settings
				if got := mustFind(t, tt, copy.ConfigSection); got != label {
					t.Errorf("label moved: %v -> %v", label, got)
				}
				if got := mustFind(t, tt, copy.ConfigOpenConfiguration); got != button {
					t.Errorf("button moved: %v -> %v", button, got)
				}
				if !openable && inkSpan(tt, label) != ink {
					t.Error("unavailable row faded its label along with the control")
				}
				t.Logf("%s/%s/%s: face %v -> %v -> %v; label=%v button=%v",
					appearanceName(dark), language, state.name, rest, hover, pressed, label, button)
			}
		}
	}
}

// Opt-in capture of the real, unavailable configuration row under the pointer.
func TestDumpP13GoConfigurationEntry(t *testing.T) {
	tag := os.Getenv("FLOTER_P13GO_DUMP")
	if tag == "" {
		t.Skip("opt-in screenshot dump: FLOTER_P13GO_DUMP=before or after")
	}
	for _, dark := range []bool{false, true} {
		tt, label, _ := configEntryFrame(t, dark, "zh", false, true)
		settlePointer(tt, label)
		name := "/tmp/floter-P13-go-settings-" + appearanceName(dark) + "-" + tag + ".png"
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Log(name)
	}
}
