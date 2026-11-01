package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-bn · 权限审计行的尾部：一枚标记，不是两枚。
//
// `permissionTier` 的每一行此前在尾槽里画**两件东西**：一枚 `alert` 警示环，紧跟一枚
// `●`/`○`，两者都染同一个墨、同一个 `enforced` 状态（`settingsui.go:1292-1293`）。
//
// 那枚环没有语义：`alert` 是 `error` 的记号（`internal/shell/notices.go:294-299` 的
// `toastMark`），而这一行说的是「这条权限属于哪一层」。实测（520u 卡，行尾墨段，深浅
// 两套同值）：
//
//	enforced  `493..506`(14) + `514..521`(8)
//	declared  `493..506`(14) + `514..520`(7)   （深色 `○` 是空心环，碎成 2+1）
//
// `493..506` 是那枚 16u 的环，`514..52x` 是那枚 11px 的盘，中间只隔 **7 级像素**——
// 两件东西读成**一枚徽标的两个部件**。
//
// 两条 pin（深浅各跑一遍，全部量像素）：
//
//	① 行尾只有**一段**墨：`slotRuns` 以行面为基准扫尾槽，段数必须恰好 1。
//	② 那一枚就是状态记号本身：`●` 带 accent 的色相（R−B ≥ 100），`○` 不带（|R−B| ≤ 14）。
//
// 出图：`/tmp/floter-P13-bn-permimark-<dark|light>-<before|after>.png`
// （`FLOTER_P13BN_DUMP=1`；`before` 由把 `ui.Icon(c, mustGlyph("alert"))` 加回去取得）。

// slotInk returns the leftmost and rightmost columns of ink in the row's
// trailing slot, and the runs between them for the report. A column counts as
// ink when any of its pixels differs from the row's own face by more than 24
// levels summed over the three channels.
//
// The assertion is a pair of **brackets** rather than a run count: a hollow
// ring's own antialiased top and bottom are below that threshold at its middle
// columns, so a single legitimate mark can report two runs (measured on the
// declared tier in dark: `515..516` + `520..520`). Counting runs would then
// fail on the fix rather than on the defect. What cannot be argued with is
// where the ink **starts**: with the extra glyph in front, the slot's ink
// begins 21 columns left of the mark (493 against 514); with one mark it begins
// on the mark's own edge.
func slotInk(tt *ui.Tester, face ui.Color, x0, x1, y0, y1 int) (left, right int, runs []string) {
	left, right = -1, -1
	inRun, start := false, 0
	for x := x0; x < x1; x++ {
		ink := false
		for y := y0; y < y1; y++ {
			if channelDelta(sample(tt, float32(x), float32(y)), face) > 24 {
				ink = true
				break
			}
		}
		if ink {
			if left < 0 {
				left = x
			}
			right = x
		}
		if ink && !inRun {
			inRun, start = true, x
		}
		if !ink && inRun {
			inRun = false
			runs = append(runs, fmt.Sprintf("%d..%d(%d)", start, x-1, x-start))
		}
	}
	if inRun {
		out := fmt.Sprintf("%d..%d", start, right)
		runs = append(runs, out)
	}
	return left, right, runs
}

// permMarkProbe renders the audit's two tiers on one card and returns the
// frame, the row faces and the two marks' boxes.
type permMark struct {
	tester                     *ui.Tester
	enforced, declared         ui.Rect
	faceEnforced, faceDeclared ui.Color
	// cardRight is the card's own trailing edge, read off the frame: the last
	// column on the mark's band that is still the row face. The slot scan
	// stops one column short of it, so the card's own edge is never mistaken
	// for a third object in the slot.
	cardRight int
}

func permMarkProbe(t *testing.T, dark bool) permMark {
	t.Helper()
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	var p permMark
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		ui.Column(c).Padding(24).Width(560).Children(func() {
			a.card(c, func() {
				a.permissionTier(c, en.PermissionsEnforcedLabel, en.PermissionsEnforcedHint, true, "en", []string{"clipboard-write"})
				a.permissionTier(c, en.PermissionsDisclosureLabel, en.PermissionsDisclosureHint, false, "en", []string{"filesystem-read"})
			})
		})
	}, 700, 560)
	tt.SetDark(dark)
	tt.Frame()
	p.tester = tt
	p.enforced = mustFind(t, tt, "\u25cf")
	p.declared = mustFind(t, tt, "\u25cb")
	// The row face is read far left of the trailing slot, on the same band.
	p.faceEnforced = sample(tt, 300, float32(int(p.enforced.Y+p.enforced.H/2)))
	p.faceDeclared = sample(tt, 300, float32(int(p.declared.Y+p.declared.H/2)))
	y := int(p.enforced.Y + p.enforced.H/2)
	for x := 300; x < tt.Image().Bounds().Max.X; x++ {
		if channelDelta(sample(tt, float32(x), float32(y)), p.faceEnforced) <= 6 {
			p.cardRight = x
		}
	}
	return p
}

// TestThePermissionRowEndsInOneMark pins the slot's cardinality: one object.
//
// Mutation: put `ui.Icon(c, mustGlyph("alert")).Size(t.Space(4), t.Space(4)).TextColor(color)`
// back in front of the disc → the slot reports two runs (a 14px ring and the
// disc) and both appearances fail.
func TestThePermissionRowEndsInOneMark(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			p := permMarkProbe(t, appearance.dark)
			for _, c := range []struct {
				name string
				box  ui.Rect
				face ui.Color
			}{
				{"enforced", p.enforced, p.faceEnforced},
				{"declared", p.declared, p.faceDeclared},
			} {
				left, right, runs := slotInk(p.tester, c.face,
					int(c.box.X)-40, p.cardRight-1,
					int(c.box.Y)-4, int(c.box.Y+c.box.H)+4)
				// ① Nothing precedes the mark: the slot's ink starts on the
				// mark's own left edge (the box is fractional, so ±2 columns).
				if want := int(c.box.X); left < want-2 || left > want+2 {
					t.Errorf("[%s] the %s row's trailing slot starts its ink at x=%d but the mark's box "+
						"begins at x=%d (runs %v): something is drawn in front of the state mark",
						appearance.name, c.name, left, want, runs)
				}
				// ② Nothing follows it either: one 11px mark is 8..12 columns
				// wide, so a slot wider than that is a second object.
				if width := right - left + 1; width > 14 {
					t.Errorf("[%s] the %s row's trailing slot paints %d columns of ink (runs %v), "+
						"more than one 11px mark: the row's state is one mark",
						appearance.name, c.name, width, runs)
				}
			}
		})
	}
}

// TestTheOneMarkIsTheStateMark pins what that single object says: the enforced
// tier's disc carries the accent, the declared tier's ring carries no hue.
//
// This is `raycast_test.go`'s §5-注脚-1 pair (「实心/空心即 ON/OFF」), re-read
// on the mark that is now the only one in the slot. A mutation that gives the
// enforced disc `TextStrong` keeps its lightness but loses the hue, so the
// R−B assertion is the one that has to fail.
func TestTheOneMarkIsTheStateMark(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			p := permMarkProbe(t, appearance.dark)
			enforced := inkColor(p.tester, p.enforced)
			declared := inkColor(p.tester, p.declared)
			if d := int(enforced.B) - int(enforced.R); d < 100 {
				t.Errorf("[%s] the enforced mark %v is only %d levels warm; the solid disc is the "+
					"accent (§5 注脚第 1 条)", appearance.name, enforced, d)
			}
			if d := int(declared.B) - int(declared.R); d > 14 || d < -14 {
				t.Errorf("[%s] the declared mark %v carries a hue (R−B %d): the hollow ring is the "+
					"neutral half of the ON/OFF pair", appearance.name, declared, d)
			}
		})
	}
}

// TestDumpP13BNPermissionMark writes the round's visual evidence. Opt-in.
func TestDumpP13BNPermissionMark(t *testing.T) {
	if os.Getenv("FLOTER_P13BN_DUMP") == "" {
		t.Skip("set FLOTER_P13BN_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13BN_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, appearance := range appearanceNames {
		p := permMarkProbe(t, appearance.dark)
		name := fmt.Sprintf("/tmp/floter-P13-bn-permimark-%s-%s.png", appearance.name, tag)
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, p.tester.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", name)
	}
}
