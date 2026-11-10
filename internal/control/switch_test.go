package control

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/settings"
	"floter/internal/theme"
	"github.com/egoist/mygo/ui"
)

func TestSwitchOnFillStaysDistinctAcrossThemesAndOpacity(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, opacity := range []uint8{66, 72, 86, 100} {
			for _, active := range []bool{false, true} {
				s := settings.Default()
				s.MainOpacity = opacity
				s.Theme = "light"
				if dark {
					s.Theme = "dark"
				}
				on := true
				tt := ui.NewTester(func(c *ui.Context) {
					tokens := theme.For(s, dark)
					c.SetTheme(tokens.Theme)
					c.Root().Background(tokens.Theme.Background)
					ui.Column(c).Padding(20).Children(func() { Switch(c, &on).Label("switch") })
				}, 100, 65)
				tt.SetWindowActive(active)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				r, _ := tt.Find("switch")
				px := tt.Image().RGBAAt(int(r.X+6), int(r.Y+r.H/2))
				want := SwitchOnFill(dark)
				if px.R != want.R || px.G != want.G || px.B != want.B || px.A != 255 {
					t.Fatalf("dark=%v opacity=%d active=%v: track=%v want=%v", dark, opacity, active, px, want)
				}
				linear := func(v uint8) float64 {
					x := float64(v) / 255
					if x <= 0.04045 {
						return x / 12.92
					}
					return math.Pow((x+0.055)/1.055, 2.4)
				}
				luminance := 0.2126*linear(px.R) + 0.7152*linear(px.G) + 0.0722*linear(px.B)
				if contrast := 1.05 / (luminance + 0.05); contrast < 4 {
					t.Fatalf("white thumb contrast=%0.2f", contrast)
				}
				if err := tt.Click("switch"); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if on {
					t.Fatal("pointer did not toggle")
				}
				off := tt.Image().RGBAAt(int(r.X+r.W-6), int(r.Y+r.H/2))
				if int(off.B)-int(off.R) > 15 {
					t.Fatalf("off state still blue: %v", off)
				}
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if !on {
					t.Fatal("keyboard did not toggle")
				}
			}
		}
	}
}

func TestSwitchThemeChangesImmediately(t *testing.T) {
	s := settings.Default()
	s.Theme = "light"
	s.MainOpacity = 72
	on := true
	tt := ui.NewTester(func(c *ui.Context) {
		tokens := theme.For(s, false)
		c.SetTheme(tokens.Theme)
		Switch(c, &on).Label("switch")
	}, 100, 50)
	tt.Frame()
	r, _ := tt.Find("switch")
	s.Theme = "dark"
	tt.Frame()
	px := tt.Image().RGBAAt(int(r.X+6), int(r.Y+r.H/2))
	want := SwitchOnFill(true)
	if px.R != want.R || px.G != want.G || px.B != want.B {
		t.Fatalf("theme change left stale switch fill: %v", px)
	}
}

func TestDumpP15SwitchStates(t *testing.T) {
	dir := os.Getenv("FLOTER_P15_DUMP")
	if dir == "" {
		t.Skip("optional visual evidence")
	}
	for _, appearance := range []string{"light", "dark"} {
		for _, opacity := range []uint8{72, 100} {
			s := settings.Default()
			s.Theme = appearance
			s.MainOpacity = opacity
			off, on, disabled := false, true, true
			tt := ui.NewTester(func(c *ui.Context) {
				tokens := theme.For(s, false)
				c.SetTheme(tokens.Theme)
				c.Root().Background(tokens.Theme.Background)
				ui.Column(c).Padding(20).Gap(16).Children(func() {
					for i, p := range []*bool{&off, &on, &disabled} {
						ui.Row(c).Gap(20).AlignItems(ui.Center).Children(func() {
							ui.Text(c, []string{"Off", "On", "Disabled on"}[i]).Width(100)
							tokens.LensInset(Switch(c, p)).Disabled(i == 2)
						})
					}
				})
			}, 220, 150)
			tt.Frame()
			tt.Advance(time.Second)
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("switch-%s-%d.png", appearance, opacity)))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSwitchDoesNotChangeWindowTransparency(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		tok := theme.For(settings.Default(), false)
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(24, 24, 26, 0.72))
		on := true
		Switch(c, &on)
	}, 200, 100)
	tt.Frame()
	if a := tt.Image().RGBAAt(180, 80).A; a < 182 || a > 185 {
		t.Fatalf("switch changed the window's background alpha to %d", a)
	}
}
