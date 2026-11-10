package shell

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"floter/internal/settings"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func TestThemePickerRebuildsWholeFrame(t *testing.T) {
	s := settings.Default()
	s.Theme, s.Language = "light", "zh"
	a := New(Options{Store: settings.NewStore(s)})
	a.Surf = SurfaceSettings
	tt := ui.NewTester(a.View, 720, 580)
	tt.Frame()
	tt.Advance(time.Second)
	for _, label := range []string{"深色", "浅色", "自动"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		before := append([]byte(nil), tt.Image().Pix...)
		tt.Frame()
		if !bytes.Equal(before, tt.Image().Pix) {
			t.Errorf("%s left stale colors until another frame", label)
		}
	}
}

func TestModifierUsesOneTerminalFooter(t *testing.T) {
	for _, query := range []string{"", "Recent", "echo unmatched-query"} {
		t.Run(query, func(t *testing.T) {
			a := New(Options{Store: settings.NewStore(settings.Default())})
			recentLauncher(a, 5)
			a.Launcher.SetQuery(query)
			opened, ran := 0, 0
			a.Launcher.Actions.OpenTerminal = func() { opened++ }
			a.Launcher.Actions.RunInTerminal = func([]string) { ran++ }
			var sizes [][2]int
			a.setWindowSize = func(w, h int) { sizes = append(sizes, [2]int{w, h}) }
			tt := ui.NewTester(a.View, 720, 580)
			tt.Frame()
			tt.Advance(time.Second)
			before := a.launcherTarget
			sizes = nil
			tt.HoldModifiers(ui.Cmd)
			results := a.Launcher.Results()
			bars := 0
			for _, row := range results {
				if row.ID == "bar:shell" || row.ID == "bar:terminal" {
					bars++
				}
			}
			if bars != 1 || a.Launcher.FeedbackVisible() {
				t.Fatalf("terminal entries=%d, extra band=%t", bars, a.Launcher.FeedbackVisible())
			}
			if a.Launcher.Selected != len(results)-1 {
				t.Fatalf("footer is not selected: %d / %d", a.Launcher.Selected, len(results))
			}
			if query == "Recent" && (before != a.launcherTarget || len(sizes) != 0) {
				t.Errorf("existing footer resized window: %d -> %d, %v", before, a.launcherTarget, sizes)
			}
			if len(sizes) > 1 || a.lastH != a.launcherTarget {
				t.Errorf("modifier resize did not settle immediately: %v", sizes)
			}
			tt.Key(ui.Cmd, ui.KeyEnter)
			if query == "" && opened != 1 || query != "" && ran != 1 {
				t.Fatalf("wrong action: opened=%d ran=%d", opened, ran)
			}
			tt.HoldModifiers(0)
			if a.launcherTarget != before {
				t.Errorf("release height=%d, want %d", a.launcherTarget, before)
			}
		})
	}
}

func TestPanelRequestsRespectScreenHeight(t *testing.T) {
	for _, areaY := range []int{-900, 0, 200} {
		for _, height := range []int{120, 580, 720} {
			y := panelY(areaY+250, areaY, 800, height)
			if y < areaY || y+height > areaY+800-12 {
				t.Errorf("panel escapes work area: origin=%d y=%d height=%d", areaY, y, height)
			}
		}
	}
	for _, area := range []float64{0, 200, 600, 900, 2160} {
		for _, surface := range []Surface{SurfaceLauncher, SurfaceSettings} {
			a := New(Options{Store: settings.NewStore(settings.Default()), WorkAreaHeight: area})
			a.Surf = surface
			var height int
			a.setWindowSize = func(w, h int) { height = h }
			a.requestWindowSize(720, 4000)
			a.flushWindowSize()
			if height > 720 || area > 0 && float64(height) > area-24 {
				t.Errorf("surface=%v screen=%v request=%d", surface, area, height)
			}
			if surface == SurfaceSettings && height > 580 {
				t.Errorf("settings height=%d", height)
			}
			if surface == SurfaceLauncher {
				a.resizeLauncher(4000)
				tt := ui.NewTester(func(c *ui.Context) { a.walkLauncherHeight(c); a.flushWindowSize() }, 720, 580)
				tt.Frame()
				if a.launcherTarget != height {
					t.Errorf("uncapped target=%d height=%d", a.launcherTarget, height)
				}
			}
		}
	}
	for _, scale := range []string{"tiny", "small", "default", "large"} {
		if SettingsHeight(scale, 900) != 580 {
			t.Errorf("%s scaled settings height", scale)
		}
	}
}

func TestCappedLauncherStillScrolls(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default()), WorkAreaHeight: 320})
	recentLauncher(a, 10)
	tt := ui.NewTester(a.View, 720, 296)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	r, _ := tt.Find("Recent 00")
	tt.Scroll(350, r.Y+r.H/2, 0, 120)
	after, ok := tt.Find("Recent 00")
	if ok && after.Y >= r.Y {
		t.Fatal("capped list cannot scroll")
	}
	if a.launcherTarget > 296 {
		t.Fatalf("window exceeded display: %d", a.launcherTarget)
	}
}

func TestThemeSwitchMatchesFreshFrame(t *testing.T) {
	for _, page := range []int{settingsui.PageGeneral, settingsui.PageAbout, settingsui.PageIntegrations} {
		t.Run(fmt.Sprint(page), func(t *testing.T) {
			s := settings.Default()
			s.Theme, s.Language = "light", "zh"
			a := New(Options{Store: settings.NewStore(s)})
			a.Surf, a.Settings.Page = SurfaceSettings, page
			tt := ui.NewTester(a.View, 720, 580)
			tt.Frame()
			tt.Advance(time.Second)
			for _, appearance := range []string{"dark", "light", "dark"} {
				if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				fresh := New(Options{Store: settings.NewStore(a.Store.Snapshot())})
				fresh.Surf, fresh.Settings.Page = SurfaceSettings, page
				ref := ui.NewTester(fresh.View, 720, 580)
				ref.Frame()
				ref.Advance(time.Second)
				got, want := tt.Image(), ref.Image()
				changed := 0
				for y := 0; y < 580; y++ {
					for x := 0; x < 720; x++ {
						if got.At(x, y) != want.At(x, y) {
							changed++
						}
					}
				}
				if changed > 0 {
					t.Errorf("%s: %d pixels differ from a fresh theme", appearance, changed)
				}
			}
		})
	}
}

func TestFittingLauncherDoesNotScroll(t *testing.T) {
	for _, scale := range []string{"tiny", "small", "default", "large"} {
		for _, variant := range []string{"recent", "matched", "feedback"} {
			for _, count := range []int{1, 5, 10} {
				t.Run(fmt.Sprintf("%s/%s/%d", scale, variant, count), func(t *testing.T) {
					s := settings.Default()
					s.UIScale = scale
					a := New(Options{Store: settings.NewStore(s)})
					recentLauncher(a, count)
					if variant == "matched" {
						a.Launcher.SetQuery("Recent")
					}
					if variant == "feedback" {
						a.Launcher.Feedback("Copied")
					}
					tt := ui.NewTester(a.View, 720, 800)
					tt.SetPreferences(ui.Preferences{ReduceMotion: true})
					tt.Frame()
					tt.SetSize(720, a.launcherTarget)
					tt.Frame()
					before, ok := tt.Find("Recent 00")
					if !ok {
						t.Fatal("first result missing")
					}
					tt.Scroll(350, before.Y+before.H/2, 0, 120)
					tt.Frame()
					after, ok := tt.Find("Recent 00")
					if !ok || after.Y != before.Y {
						t.Errorf("fitting list scrolled: %v -> %v (height %d)", before, after, a.launcherTarget)
					}
				})
			}
		}
	}
}
