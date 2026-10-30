package settingsui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/control"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-o · 凹槽的一对线：`LensInset` 的底边**不是** `LensRim`。
//
// 一个凹槽（开关的轨道 / 分段控件的槽）是同一个透镜倒过来看：光积在**底边**、
// 顶边是暗的（`docs/liquid-glass.md` §3 的 `LensInset` 一段）。深色档一直读得出
// 这件事（底内线 69 对面 36，+33），浅色档**从来没有人量过**——量下来它根本不存在：
//
//	浅色 地面 247 ｜ 槽面 232 ｜ 顶内线 184（−48）｜ 底内线 221（**−11**）
//
// 顶暗、底也暗 = 两条暗线的画框，不是「洞里积光」。根因是两条半透明线叠在同一行：
// 槽自己的发丝线（`TrackEdge` 浅色黑 0.08）先把 232 压到 213，再叠 0.09 的白只剩
// 217 —— 受光边赢不了它下面那条环。参考实现浅色档本来就是近白
// （`floter-main/src/styles/base.css:899`，`--glass-lens-rim-base: 0.9`），是本仓
// P8 那一代把整族光泽压到 0.04 时一起压掉的。
//
// 所以这一轮给凹槽一条自己的受光边（`theme.Tokens.TrackRim`），浅色档 0.09 → 0.70。
// 深色档两者同值 0.09，**不动**（它那一半已经读得出来，缺的一半是面·对侧，P13-n
// 用 `lensEdge` 0.20 补过了）。
//
// 量在**真页面的地面**上：`lensProbe` 的中灰地面会把一条白线的抬高量错一倍
// （见 counterside_test.go 的说明），而凹槽的受光边正是「比槽面亮多少」这件事。

// grooveLighter is liteBrighter for a *hued* fill: the on switch's track is the
// accent, and a neutral white line over it lifts two channels while lowering
// the third (measured 143,183,255 → 157,189,250). The lit edge is still light —
// the sum is what says so, the same way liteDarker reads a shade over a hue.
func grooveLighter(a, b ui.Color) bool {
	return int(a.R)+int(a.G)+int(a.B) > int(b.R)+int(b.G)+int(b.B)
}

// readGroove reads one groove's three pixels off the rendered frame: the
// darkest pixel in its top boundary band (the recess's dark line), the
// brightest in its bottom band (the lit line), and the fill between them.
//
// Bands, not rows: both widgets are laid out on fractional heights (the
// segmented track is 30.6u, a switch 18u at a fractional y), so a 1px inset line
// straddles two rows and lands at a different offset in each. A band read also
// has to *skip the ground*, or the light appearance would report the sheet
// itself (247) as the groove's brightest pixel — the ground is brighter than the
// groove's lit edge there, and that is the point of the pin below.
func readGroove(img *image.RGBA, box ui.Rect, x int) (top, bottom, fill ui.Color) {
	ground := imagePixel(img, 2, 2)
	groundish := func(c ui.Color) bool {
		return absInt(int(c.R)-int(ground.R)) <= 2 && absInt(int(c.G)-int(ground.G)) <= 2 &&
			absInt(int(c.B)-int(ground.B)) <= 2
	}
	fill = imagePixel(img, x, int(box.Y+box.H/2))
	sum := func(c ui.Color) int { return int(c.R) + int(c.G) + int(c.B) }
	seen := false
	for dy := -1; dy <= 2; dy++ {
		q := imagePixel(img, x, int(box.Y)+dy)
		if groundish(q) {
			continue
		}
		if !seen || sum(q) < sum(top) {
			top, seen = q, true
		}
	}
	seen = false
	for dy := -2; dy <= 1; dy++ {
		q := imagePixel(img, x, int(box.Y+box.H)+dy)
		if groundish(q) {
			continue
		}
		if !seen || sum(q) > sum(bottom) {
			bottom, seen = q, true
		}
	}
	return top, bottom, fill
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// groove is one measured recess: its three pixels plus whether its fill is a
// hue (the on switch's accent track) rather than the neutral `Track`.
type groove struct {
	name        string
	top, bottom ui.Color
	fill        ui.Color
	hued        bool
}

// grooveProbe renders the three shipped grooves on the settings page's own
// ground: the segmented control's track (`control.SegmentedIndex`), a switch at
// rest (`a.switchControl` off → the neutral `Track`) and the same switch on (its
// track is the accent). All three are the real widgets, not hand-built copies:
// a pin that painted its own reference stack would stay green while the widget
// drew something else.
func grooveProbe(t *testing.T, dark bool) (grooves []groove, ground ui.Color) {
	t.Helper()
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	ground = realPageGround(t, dark)
	var segBox, offBox, onBox ui.Rect
	var segRef, offRef, onRef ui.Handle
	sel, off, on := 1, false, true
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Width(320).Gap(20).Children(func() {
			control.SegmentedIndex(c, a.tokens(c), &sel, []string{"Aaa", "Bbb", "Ccc"}, true, false,
				c.Theme().Space(0.5), c.Theme().Space(0.5)).Bind(&segRef)
			a.switchControl(c, &off).Bind(&offRef)
			a.switchControl(c, &on).Bind(&onRef)
			segBox = c.Resolve(segRef).Bounds()
			offBox = c.Resolve(offRef).Bounds()
			onBox = c.Resolve(onRef).Bounds()
		})
	}, 420, 200)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	img := tt.Image()
	// The segmented track: x+50 is inside the first (unchosen) segment, past
	// its label and far from both stadium corners. The chosen segment is the
	// middle one, so no raised pane crosses this column either.
	segTop, segBottom, segFill := readGroove(img, segBox, int(segBox.X)+50)
	// The switches: a switch's thumb sits at one end, never in the middle, so
	// the column is taken at the *other* end — past the 9u stadium cap and clear
	// of the knob. The off switch's thumb is on the left (x+21 is the right end
	// of a 32.4u track), the on switch's on the right (x+10 is its left end).
	offTop, offBottom, offFill := readGroove(img, offBox, int(offBox.X+offBox.W)-11)
	onTop, onBottom, onFill := readGroove(img, onBox, int(onBox.X)+10)
	return []groove{
		{"segmented track", segTop, segBottom, segFill, false},
		{"switch at rest", offTop, offBottom, offFill, false},
		{"switch on", onTop, onBottom, onFill, true},
	}, ground
}

// TestTheGrooveLightsItsBottomEdge pins §3 的 `TrackRim`：凹槽的底边必须比槽面亮
// **≥ 6 级**，两套外观、三个凹槽。6 是这一支的门槛（`liteFaceFloor` 的一半以上：
// 受光边是「一条线」，比面差窄，但必须在 1x 屏上读得出来）。
//
// 强调色的轨道上「更亮」不是逐通道的：一条中性白线压在 accent 上会抬 R/G、压 B
// （实测 143,183,255 → 157,189,250），所以那一条按「R 与 G 各抬 ≥ 6 且总和不为负」
// 判——它仍然是一道白光，只是底色带色相。
//
// Mutations:
//   - 浅色 `trackRim` 回 0.09（= 借 `lensRim`）→ 浅色 −11，红；
//   - `LensInset` 的底边线换成 `LensEdge`（没有受光边）→ 两套外观都红；
//   - `LensInset` 里 `TrackEdge` 与 `TrackRim` 换序（环画在最后）→ 浅色红。
func TestTheGrooveLightsItsBottomEdge(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			grooves, ground := grooveProbe(t, appearance.dark)
			for _, g := range grooves {
				if !grooveLighter(g.bottom, g.fill) {
					t.Errorf("[%s] the %s's groove has no lit bottom (%v against its own fill %v): `LensInset` "+
						"lights the bottom edge (liquid-glass.md §3 的 TrackRim)", appearance.name, g.name, g.bottom, g.fill)
					continue
				}
				lift := int(g.bottom.R) - int(g.fill.R)
				if g.hued {
					lift = min(int(g.bottom.R)-int(g.fill.R), int(g.bottom.G)-int(g.fill.G))
				}
				if lift < 6 {
					t.Errorf("[%s] the %s's lit bottom lifts its fill by only %d levels (%v over %v on the page "+
						"ground %v): at 1x that is the same pixel, and the groove reads as a frame of two dark "+
						"lines instead of a hole with light in it (liquid-glass.md §3 的 TrackRim)",
						appearance.name, g.name, lift, g.bottom, g.fill, ground)
				}
				t.Logf("[%s] %-15s ground %v fill %v top %v bottom %v (+%d)",
					appearance.name, g.name, ground, g.fill, g.top, g.bottom, lift)
			}
		})
	}
}

// TestTheGroovesLitEdgeStaysUnderTheSheet pins the other half of §1 禁令 3 on the
// light appearance: the groove's lit edge may not outshine the sheet it is cut
// into. A recess catches light; it does not sparkle. This is the same rule that
// kept `LensRim` at 0.09 on raised faces (P13-n) — there the line would have
// washed an accent CTA's top row, here it would have turned the groove into a
// glass brick with a bright white rim.
//
// Mutation: 浅色 `trackRim` → 1.0（纯白）→ 底内线 251 > 地面 247，红。
func TestTheGroovesLitEdgeStaysUnderTheSheet(t *testing.T) {
	grooves, ground := grooveProbe(t, false)
	for _, g := range grooves {
		if grooveLighter(g.bottom, ground) {
			t.Errorf("the %s's lit edge (%v) is brighter than the sheet it is cut into (%v): a groove catches "+
				"light, it does not sparkle (liquid-glass.md §1 禁令 3 / §3 的 TrackRim)", g.name, g.bottom, ground)
		}
		t.Logf("%-15s bottom %v under ground %v", g.name, g.bottom, ground)
	}
}

// TestDumpP13OGroovePNGs is this round's visual evidence: the same two grooves
// drawn before and after, on the real page ground. Opt-in (`FLOTER_PNG=1`).
func TestDumpP13OGroovePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-o-groove-<appearance>.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tokens := theme.ForPreferences(settings.Default(), dark, ui.Preferences{})
		ground := realPageGround(t, dark)
		// The stack before this round: the same four layers with the raised
		// face's rim on the groove's bottom.
		before := func(e ui.Element) ui.Element {
			e.InsetShadow(0, 0, 0, 1, tokens.TrackEdge)
			e.InsetShadow(0, 1, 0, 0, tokens.LensEdge)
			e.InsetShadow(0, 1, 2, 0, tokens.ShadowContact)
			e.InsetShadow(0, -1, 0, 0, tokens.LensRim)
			return e
		}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(ground)
			ui.Column(c).Padding(24).Gap(10).Children(func() {
				for _, row := range []struct {
					label string
					wrap  func(ui.Element) ui.Element
				}{
					{"before: LensRim 0.09", before},
					{fmt.Sprintf("after: TrackRim %.2f", float32(tokens.TrackRim.A)/255), tokens.LensInset},
				} {
					ui.Text(c, row.label).FontSize(tokens.Caption).TextColor(tokens.TextSecondary)
					// Two grooves at the two sizes the app ships: a switch's
					// track (the neutral `Track`) and the accent fill of an on
					// switch, both 999px stadiums. The material is drawn on its
					// own here — the shipped widgets' readings are the two pins
					// above, this picture is only the before/after.
					row.wrap(ui.Row(c).Width(220).Height(24).Radius(999).Background(tokens.Track))
					row.wrap(ui.Row(c).Width(220).Height(24).Radius(999).Background(c.Theme().Accent))
				}
			})
		}, 300, 220)
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P13-o-groove-"+appearance+".png")
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
	}
}
