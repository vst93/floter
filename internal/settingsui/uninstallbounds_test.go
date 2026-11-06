package settingsui

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func uninstallBoundsFrame(t *testing.T, dark bool, message string, height int) (*ui.Tester, *App) {
	t.Helper()
	a := noticeApp(t, dark)
	a.uninstallOpen = map[string]bool{"npm": true}
	a.uninstallError = map[string]string{"npm": message}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		a.uninstallDialog(c, i18n.For("en").Settings, npmTool())
	}, 720, height)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt, a
}

func TestUninstallFocusRingFitsScrollViewport(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, height := range []int{640, 300} {
			t.Run(fmt.Sprintf("%s/%d", appearanceName(dark), height), func(t *testing.T) {
				tt, _ := uninstallBoundsFrame(t, dark, "Permission denied; close the process and retry", height)
				label := i18n.For("en").Settings.IntegrationsUninstall
				for i := 0; i < 8 && !tt.Focused(label); i++ {
					tt.Key(0, ui.KeyTab)
					tt.Frame()
				}
				tt.Frame()
				noticeDump(t, tt, fmt.Sprintf("uninstall-focus-%d", height), dark)
				if !tt.Focused(label) {
					t.Fatal("uninstall is not focused")
				}
				box := mustFind(t, tt, label)
				img := tt.Image()
				// The existing framework focus ring is 2px wide, with a 1px gap.
				// Test the middle of each outside edge, away from antialiased corners.
				for _, p := range []image.Point{
					{X: int(box.X + box.W + 2), Y: int(box.Y + box.H/2)},
					{X: int(box.X + box.W/2), Y: int(box.Y + box.H + 2)},
				} {
					px := img.RGBAAt(p.X, p.Y)
					t.Logf("button=%+v point=%v pixel=%v", box, p, px)
					if int(px.B)-int(px.R) < 35 || int(px.B)-int(px.G) < 15 {
						t.Errorf("focused uninstall ring clipped at %v: %v", p, px)
					}
				}
				if height == 300 {
					tt.Scroll(300, 140, 0, -120)
					tt.Frame()
					tt.Frame()
					if after := mustFind(t, tt, label); after.Y <= box.Y {
						t.Errorf("focused actions stole manual scroll: before=%+v after=%+v", box, after)
					}
				}
			})
		}
	}
}

func TestUninstallErrorLeavesRoomForAnswers(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, sentence := range []string{"Permission denied", strings.Repeat("Close the running process and retry. ", 5)} {
			t.Run(appearanceName(dark)+"/"+fmt.Sprint(len(sentence)), func(t *testing.T) {
				tt, a := uninstallBoundsFrame(t, dark, sentence, 640)
				copy := i18n.For("en").Settings
				text, cancel := mustFind(t, tt, sentence), mustFind(t, tt, copy.PermissionsCancel)
				var padding, gap float32
				ui.NewTester(func(c *ui.Context) {
					c.SetTheme(a.tokens(c).Theme)
					padding, gap = c.Theme().Space(2)+1, c.Theme().Space(2)
				}, 10, 10).Frame()
				// A text-only error has one vertical padding and a border below it.
				actual := cancel.Y - text.Y - text.H - padding
				noticeDump(t, tt, "uninstall-gap-"+fmt.Sprint(len(sentence)), dark)
				t.Logf("error-to-actions gap=%.3f want=%.3f", actual, gap)
				if absFloat(actual-gap) > 0.01 {
					t.Errorf("failure band touches answers: gap=%.3f want=%.3f", actual, gap)
				}
				delete(a.uninstallError, "npm")
				tt.Frame()
				tt.Frame()
				clean := mustFind(t, tt, copy.PermissionsCancel)
				without, _ := uninstallBoundsFrame(t, dark, "", 640)
				if clean != mustFind(t, without, copy.PermissionsCancel) {
					t.Errorf("clearing error left a blank gap: %+v", clean)
				}
			})
		}
	}
}
