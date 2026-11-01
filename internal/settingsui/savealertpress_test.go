package settingsui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-bv · 保存失败横幅的那枚 ✕：这一族里最后一枚「悬停才长面、按下去纹丝不动」的键。
//
// 量的是三件事（每套外观各一遍，全部读像素，不读注释）：
//
//	① 悬停真的画了面（悬停面 ≠ 横幅自己的面 `Float`）；
//	② 按下真的画了**另一个**面（按下面 ≠ 悬停面）——旧实现这一条逐像素相同，
//	   这是本轮的根因；
//	③ 按下的面在**横幅面与悬停面之间**（第三档是「往静息的回退」，不是第三种颜色，
//	   `docs/liquid-glass.md` §5 的 `ControlPress`）；
//	④ 按下只改自己那一块：横幅自己的面在按下帧里一个字节都没动（`Scale(0.92)`
//	   是这块键自己的变换，不是整条带的重排）。
//
// 出图：`FLOTER_PNG=1 go test ./internal/settingsui/ -run TestDumpP13BV`，
// 落到 `/tmp/floter-P13-bv-savealert-<light|dark>-<rest|hover|press>.png`。

// saveAlertProbe is how far inside the ✕ box the material is read: the key is 24u
// with a 14u mark centred, so 2px in is the face, never the glyph.
const saveAlertProbe = 2

// saveAlertState is one pointer state's reading: the face under the ✕, the
// banner's own face (the ground the key is graded against), and the key's box.
type saveAlertState struct {
	key    ui.Color
	banner ui.Color
	box    ui.Rect
}

// saveAlertFrame builds the banner alone over the page's own ground, so the
// pointer's three states are the only thing that changes between frames.
func saveAlertFrame(t *testing.T, dark bool) (*ui.Tester, i18n.Settings, *App) {
	t.Helper()
	// A directory where the settings file should be: the store still opens (it is
	// the *writes* that fail), which is all this frame needs.
	store, _ := settings.OpenStore(t.TempDir())
	a := New(store, Actions{})
	// The banner's enter animation is not this pin's subject: park it in place.
	a.saveFailed = true
	a.saveAlertShown = true
	a.saveAlertProgress = 1
	copy := i18n.For("en").Settings
	tt := ui.NewTester(func(c *ui.Context) {
		tokens := a.tokens(c)
		c.SetTheme(tokens.Theme)
		c.Root().Background(tokens.Theme.Background)
		ui.Column(c).Padding(12).Children(func() {
			a.saveAlert(c, copy)
		})
	}, 720, 80)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt, copy, a
}

// saveAlertRead parks the pointer and reads the two faces.
func saveAlertRead(t *testing.T, dark bool, state string) saveAlertState {
	t.Helper()
	tt, copy, _ := saveAlertFrame(t, dark)
	box, ok := tt.Find(copy.Close)
	if !ok {
		t.Fatalf("dark=%v: the banner has no %q control: %v", dark, copy.Close, tt.Texts())
	}
	x, y := int(box.X)+saveAlertProbe, int(box.Y+box.H/2)
	// The banner's own face is read far to the right of the ✕, on the same row:
	// the sentence never reaches there, so it is the pane the key sits on.
	groundX := int(box.X) - 40
	switch state {
	case "rest":
		// On the banner, on no control: the pane both faces are graded against.
		tt.Move(float32(groundX), float32(y))
		tt.Frame()
	case "hover":
		tt.Move(float32(x), float32(y))
		tt.Frame()
	case "press":
		tt.Move(float32(x), float32(y))
		tt.Frame()
		tt.Press(float32(x), float32(y))
		// No colour transition is set on this key, so the frame after the press
		// is the settled face; the second frame only proves it is a state.
		tt.Frame()
		tt.Frame()
	default:
		t.Fatalf("unknown state %q", state)
	}
	return saveAlertState{
		key:    saveAlertPixel(tt.Image(), x, y),
		banner: saveAlertPixel(tt.Image(), groundX, y),
		box:    box,
	}
}

func saveAlertPixel(img image.Image, x, y int) ui.Color {
	r, g, b, _ := img.At(x, y).RGBA()
	return ui.RGBA(uint8(r>>8), uint8(g>>8), uint8(b>>8), 255)
}

// saveAlertDelta is how far apart two colours are, in levels of their widest
// channel.
func saveAlertDelta(a, b ui.Color) int {
	d := func(x, y uint8) int {
		if x > y {
			return int(x) - int(y)
		}
		return int(y) - int(x)
	}
	dr, dg, db := d(a.R, b.R), d(a.G, b.G), d(a.B, b.B)
	if dg > dr {
		dr = dg
	}
	if db > dr {
		dr = db
	}
	return dr
}

// saveAlertLuma is the perceptual weight of a colour, used only to ask "which
// side of which" — never to assert an exact composited value.
func saveAlertLuma(c ui.Color) int { return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000 }

// saveAlertBetween reports whether mid's luma sits strictly between a and b.
func saveAlertBetween(a, mid, b ui.Color) bool {
	lo, hi := saveAlertLuma(a), saveAlertLuma(b)
	if lo > hi {
		lo, hi = hi, lo
	}
	return saveAlertLuma(mid) > lo && saveAlertLuma(mid) < hi
}

// TestTheSaveAlertsCloseKeySinksWhenThePointerGoesDown is ① ② ③ ④.
func TestTheSaveAlertsCloseKeySinksWhenThePointerGoesDown(t *testing.T) {
	for _, dark := range []bool{false, true} {
		rest := saveAlertRead(t, dark, "rest")
		hover := saveAlertRead(t, dark, "hover")
		press := saveAlertRead(t, dark, "press")
		t.Logf("dark=%v: banner %v rest %v hover %v press %v", dark, rest.banner, rest.key, hover.key, press.key)
		if saveAlertDelta(hover.key, rest.key) < 4 {
			t.Errorf("dark=%v: the hovered ✕ %v is the banner's own face %v — the hover arm painted "+
				"nothing, so this pin would be grading the banner, not the key", dark, hover.key, rest.key)
			continue
		}
		if saveAlertDelta(press.key, hover.key) < 3 {
			t.Errorf("dark=%v: the ✕ under the pointer is %v whether or not the button is down (banner %v): "+
				"the press rung paints no face of its own — one step back toward the pane is what "+
				"`--glass-control-press` means (docs/liquid-glass.md §5)", dark, press.key, rest.key)
			continue
		}
		if !saveAlertBetween(rest.key, press.key, hover.key) {
			t.Errorf("dark=%v: the pressed face %v is not between the banner's own face %v and the hovered %v "+
				"— the third rung is a step back toward rest, not a third colour", dark, press.key, rest.key, hover.key)
		}
		if press.banner != hover.banner {
			t.Errorf("dark=%v: the press repainted the banner itself (%v → %v) — the third rung belongs to "+
				"the key's own box, not to the band it sits on", dark, hover.banner, press.banner)
		}
		if press.box != hover.box {
			t.Errorf("dark=%v: the press moved the key's layout box (%v → %v) — the scale is a transform, "+
				"the band must not re-flow under the pointer", dark, hover.box, press.box)
		}
	}
}

// TestDumpP13BV writes the three frames of both appearances for the eye.
func TestDumpP13BV(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to dump the P13-bv frames")
	}
	for _, dark := range []bool{false, true} {
		for _, state := range []string{"rest", "hover", "press"} {
			tt, _, _ := saveAlertFrame(t, dark)
			// saveAlertRead rebuilds its own frame; here the dump needs this one.
			box, ok := tt.Find(i18n.For("en").Settings.Close)
			if !ok {
				t.Fatalf("the banner has no close control: %v", tt.Texts())
			}
			x, y := int(box.X)+saveAlertProbe, int(box.Y+box.H/2)
			switch state {
			case "rest":
				tt.Move(float32(int(box.X)-40), float32(y))
				tt.Frame()
			case "hover":
				tt.Move(float32(x), float32(y))
				tt.Frame()
			case "press":
				tt.Move(float32(x), float32(y))
				tt.Frame()
				tt.Press(float32(x), float32(y))
				tt.Frame()
				tt.Frame()
			}
			name := "light"
			if dark {
				name = "dark"
			}
			path := fmt.Sprintf("/tmp/floter-P13-bv-savealert-%s-%s.png", name, state)
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(file, tt.Image()); err != nil {
				file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
