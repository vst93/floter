package terminalui

import (
	"runtime"
	"testing"

	"floter/internal/glassmap"
	"floter/internal/settings"
	"floter/internal/theme"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func TestTerminalPaletteBackingKeepsItsOwnColor(t *testing.T) {
	names := []string{"inherit"}
	for name := range palettes {
		names = append(names, name)
	}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, appTheme := range []string{"light", "dark"} {
			for _, name := range names {
				for _, step := range []string{"off", "frosted", "liquid"} {
					for _, opacity := range []uint8{66, 83, 100} {
						s := settings.Default()
						s.Theme, s.TerminalTheme, s.GlassStep, s.TerminalOpacity = appTheme, name, step, opacity
						a := New(settings.NewStore(s), Actions{}, nil)
						a.Sessions = []*Session{{Term: &terminal.Terminal{}}}
						a.Active = 0
						tok := theme.ForTerminalPreferences(s, appTheme == "dark", ui.Preferences{})
						spec := glassmap.TerminalSpecForPreferencesOS(goos, step, opacity, false)
						light, dark := readableTerminalThemesForOS(s, goos)
						rawLight, rawDark := themeFor(name)
						if light.Palette != rawLight.Palette || (dark != nil && dark.Palette != rawDark.Palette) {
							t.Fatal("opacity adaptation changed ANSI colors")
						}
						colors := light
						if appTheme == "dark" && dark != nil {
							colors = dark
						}
						frame := a.FrameBackground(tok)
						original, _ := themeFor(name)
						if name != "inherit" && frame != original.Background {
							t.Fatal("terminal palette mixed with app background")
						}
						for _, desktop := range []ui.Color{ui.RGB(0, 0, 0), ui.RGB(255, 255, 255)} {
							bg := panelFill(name, true, spec, tok).Over(frame.Alpha(spec.TerminalFrameAlpha()).Over(desktop))
							for inkName, ink := range map[string]ui.Color{"foreground": colors.Foreground, "cursor": colors.Cursor} {
								floor := 4.5
								if inkName == "cursor" {
									floor = 3
								}
								if ratio := theme.ContrastRatio(ink, bg); ratio < floor {
									t.Fatalf("%s/%s/%s/%s/%d %s %.2f", goos, appTheme, name, step, opacity, inkName, ratio)
								}
							}
							if ratio := theme.ContrastRatio(colors.SelectionText, colors.Selection.Over(bg)); ratio < 4.5 {
								t.Fatalf("%s/%s selection %.2f", appTheme, name, ratio)
							}
							if ratio := theme.ContrastRatio(tok.TextSecondary, drawerFill(tok).Over(bg)); ratio < 4.5 {
								t.Fatalf("%s/%s drawer %.2f", appTheme, name, ratio)
							}
						}
					}
				}
			}
		}
	}
}

func TestTerminalControlsStayReadableAtCanvasMinimum(t *testing.T) {
	names := []string{"inherit"}
	for name := range palettes {
		names = append(names, name)
	}
	for _, appearance := range []string{"light", "dark"} {
		for _, name := range names {
			s := settings.Default()
			s.Theme, s.TerminalTheme, s.TerminalOpacity, s.GlassStep = appearance, name, 66, "frosted"
			a := New(settings.NewStore(s), Actions{}, nil)
			for _, running := range []bool{false, true} {
				if running {
					a.Sessions = []*Session{{Term: &terminal.Terminal{}}}
					a.Active = 0
				}
				tt := ui.NewTester(func(c *ui.Context) {
					tokens := tokensOf(a, c)
					c.SetTheme(tokens.Theme)
					inks := a.canvasInks(c)
					spec := glassmap.TerminalSpecForPreferencesOS(runtime.GOOS, s.GlassStep, s.TerminalOpacity, false)
					for _, desktop := range []ui.Color{ui.RGB(0, 0, 0), ui.RGB(255, 255, 255)} {
						bg := panelFill(name, running, spec, tokens).Over(a.FrameBackground(tokens).Alpha(spec.TerminalFrameAlpha()).Over(desktop))
						if !running && theme.ContrastRatio(tokens.TextSecondary, bg) < 4.5 {
							t.Fatal("empty terminal text lost contrast")
						}
						bar := inks.Face.Over(bg)
						for _, wash := range []ui.Color{{}, inks.Wash, inks.Press} {
							if ratio := theme.ContrastRatio(inks.Ink, wash.Over(bar)); ratio < 4.5 {
								t.Fatalf("%s/%s/running=%v bar %.2f", appearance, name, running, ratio)
							}
						}
					}
				}, 200, 100)
				tt.Frame()
			}
		}
	}
}

func TestTerminalOpacityRefreshesInkWithoutResettingFont(t *testing.T) {
	s := settings.Default()
	s.TerminalOpacity = 100
	a := New(settings.NewStore(s), Actions{}, nil)
	opts := a.options()
	previous := appearanceOf(opts)
	if err := a.Store.Update(func(s *settings.Settings) { s.TerminalOpacity = 66 }); err != nil {
		t.Fatal(err)
	}
	recorder := &appearanceRecorder{}
	previous.apply(recorder, a.options())
	if recorder.themes != 1 || recorder.fonts != 0 {
		t.Fatalf("opacity refresh: %+v", recorder)
	}
	previous.apply(recorder, a.options())
	if recorder.themes != 1 {
		t.Fatal("unchanged opacity rebuilt the terminal appearance")
	}
}

func TestTerminalInksFollowItsIndependentOpacity(t *testing.T) {
	s := settings.Default()
	s.Theme, s.MainOpacity, s.TerminalOpacity = "dark", 100, 66
	a := New(settings.NewStore(s), Actions{}, nil)
	var first, second, third ui.Color
	tt := ui.NewTester(func(c *ui.Context) { first = tokensOf(a, c).TextSecondary }, 200, 100)
	tt.Frame()
	initial := first
	if err := a.Store.Update(func(s *settings.Settings) { s.MainOpacity = 72 }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	second = first
	if err := a.Store.Update(func(s *settings.Settings) { s.TerminalOpacity = 100 }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	third = first
	s.MainOpacity = 72
	expect := theme.ForTerminalPreferences(s, true, ui.Preferences{}).TextSecondary
	if second != expect || second != initial || third == second {
		t.Fatalf("terminal ink: low %v expected %v high %v", second, expect, third)
	}
}
