package terminalui

import (
	"image/png"
	"os"
	"strconv"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/frameaudit"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// P11-at · **抽屉不让出浮条那条带。**
//
// 面板是一列：画布 `Grow(1)`、抽屉（自然高度）、标题条（`Absolute` 浮在顶上，
// `Space(7)` 高）。抽屉按网格的自然高度要位置——不设下限时，窗口缩到终端自己
// 允许的最小高度（`settings.MinTerminalHeight` = 360）时画布被压到 0、抽屉从
// y=0 开始画，正好画在浮着的标题条底下：实测 720×360 时标题 `Terminal` 与
// 抽屉第一行的 `Font family` 压同一块像素（重叠 81×8u），抽屉自己的关闭钮与
// 浮条的 X 叠成一个（28×18u）。画布的地板是浮条那一条带：抽屉放不下就排在它
// 下面、被面板底边裁掉，绝不上移。
func TestTheDrawerKeepsTheBarsBand(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		for _, size := range [][2]int{
			{settings.MinTerminalWidth, settings.MinTerminalHeight},
			{720, settings.MinTerminalHeight},
			{720, 600},
		} {
			tag := lang + " " + strconv.Itoa(size[0]) + "x" + strconv.Itoa(size[1])
			copy := i18n.For(lang).Terminal
			scopy := i18n.For(lang).Settings
			a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
			withSession(a, &terminal.Terminal{})
			a.Drawer = true
			if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
				t.Fatalf("%s: %v", tag, err)
			}
			tt := render(t, a, size[0], size[1])

			strip, ok := tt.Find(scopy.GroupTerminal)
			if !ok {
				t.Fatalf("%s: the drawer's strip is not on the frame: %q", tag, tt.Texts())
			}
			// The bar is the floating band at the panel's top edge; its close
			// control is the tallest thing on it and its box is exactly the
			// band's height (`barButton` is `Space(7)` square).
			barX, ok := tt.Find(copy.Close)
			if !ok {
				t.Fatalf("%s: the bar's close control is not on the frame: %q", tag, tt.Texts())
			}
			if strip.Y < barX.Y+barX.H-0.5 {
				t.Errorf("%s: the drawer starts at y=%.1f, the bar's band ends at y=%.1f — "+
					"the drawer's first row is painted under the floating bar",
					tag, strip.Y, barX.Y+barX.H)
			}
			// And the drawer's own dismiss is not the bar's dismiss: two X's on
			// the same pixels would be one control the pointer cannot tell apart.
			drawerX, ok := tt.Find(copy.SettingsClose)
			if !ok {
				t.Fatalf("%s: the drawer's close control is not on the frame: %q", tag, tt.Texts())
			}
			ox, oy := frameaudit.Overlap(drawerX, barX)
			if ox > 1 && oy > 1 {
				t.Errorf("%s: the drawer's close control %+v paints over the bar's %+v (overlap %.1fx%.1f)",
					tag, drawerX, barX, ox, oy)
			}
			// Nothing in the drawer's band lands on another run's pixels.
			auditDrawerBandLocalized(t, tag, tt, scopy.GroupTerminal)
		}
	}
}

// auditDrawerBandLocalized is auditDrawerBand for a language whose strip label
// is not the English one.
func auditDrawerBandLocalized(t *testing.T, tag string, tt *ui.Tester, label string) {
	t.Helper()
	auditDrawerBandFrom(t, tag, tt, label)
}

// The P11-at evidence: the terminal page at the window's own minimum size with
// the drawer open, both appearances. Opt-in (`FLOTER_PNG=1`) like the other
// dumps: a test run should not litter /tmp.
func TestDumpP11ATDrawerBandPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
		withSession(a, &terminal.Terminal{})
		a.Drawer = true
		tt := render(t, a, settings.MinTerminalWidth, settings.MinTerminalHeight)
		if dark {
			tt.SetDark(true)
			tt.Frame()
		}
		path := "/tmp/floter-P11-at-drawer-min-" + appearance + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}
