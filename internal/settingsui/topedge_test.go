package settingsui

import (
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-gi · 一条 E1 面的顶边是**两条线**，不是一条线的两种说法（`docs/liquid-glass.md` §6.154）。
//
// §6.140 的 140-5 从 P13-fu 起挂在「需拍板」里：「同一条顶边在深浅下读成相反的东西」。
// 浅色 `边 216 / 冠 236 / 面 230`、深色 `边 54 / 冠 58 / 面 39`（地面 247 / 14），
// 读起来像「浅色的顶边是暗描边、深色的是受光边」。本轮要说明的不是哪个值对，而是
// **两行各有各的活**：
//
//   - 盒子自己那一行 `Border(1, ControlEdge)` 是**分离线**：它把面从**地面**上分出来。
//     地面的极性在两种外观下相反（纸 247 / 夜 14），所以这条线的方向必须跟着地面反相。
//   - 它下面那一行 `InsetShadow(0, 1, 0, 0, LensRim)` 是**冠线**：它压的是**面**，
//     两种外观下都亮（浅 +6 / 深 +19）。
//
// 于是 140-5 的候选——「给 E1 的顶边一条亮线」——被量成了两件具体的坏事：
// 浅色上分离线（白 0.24 压在面上 = 236）与冠线（236）**并成一条两行宽的白带**，
// 而盒子离地面只剩 11 级（出厂 31）——那正是 §1 禁令 3 说的「用白边模拟高光」；
// 深色上它把最亮的一行推到**面之外**（边 77 > 冠 58），受光边落到冠线外面去了。
//
// 两条线的**距离**也正是「一条线两种读法」的来源：浅色 216 / 236 相差 20 级，看得见两条；
// 深色 54 / 58 只差 4 级，本来就该读成一条受光边。同一个 alpha 服务不了两种地面。
//
// 谁会让这些 pin 变红：把浅色的 `controlEdge` 换成亮线（M1）、把冠线降到无光泽时代的
// 0.06（M2）、或者哪天有人把 `Border` 与 `InsetShadow` 顶行并成一行。

// topEdgeArm is what one raised face's top edge owns, in luminance levels:
// the page above it, its own border row, the rim row under it, and the face.
type topEdgeArm struct {
	ground, edge, rim, face int
}

// topEdgeMode chooses the墨 of the border row: the shipped hairline, or the
// candidate 140-5 asked for (the top edge drawn as a lit line too).
type topEdgeMode int

const (
	topEdgeShipped topEdgeMode = iota // tokens.ControlEdge: the hairline follows the ground
	topEdgeLitTop                     // the rejected candidate: a lit border row
)

// measureTopEdge builds one E1 pill — the settings action pill's own material
// (`rows.go` `extensionActionFace`: `Control` face, `ControlEdge` hairline,
// `LitFace`'s pair) — under a **fractional** top (`Space(2.25)`, the same
// 17.700 of §6.140) and reads the four rows its top edge owns.
func measureTopEdge(t *testing.T, dark bool, mode topEdgeMode) topEdgeArm {
	t.Helper()
	ground := realPageGround(t, dark)
	s := settings.Default()
	s.UIScale = "default"
	s.Theme = appearanceName(dark)
	a := New(settings.NewStore(s), Actions{})
	var ref ui.Handle
	var box ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		th := c.Theme()
		tokens := a.tokens(c)
		edge := tokens.ControlEdge
		if mode == topEdgeLitTop {
			// The only white the light sheet can spend on a raised edge is the
			// `LensRim` family's own 0.24 (§1's 0-11 row): the candidate is
			// "make the border row the same line as the crown".
			edge = ui.RGBA(255, 255, 255, 0.24)
		}
		c.Root().Background(ground)
		ui.Column(c).Padding(10.5).Width(320).Children(func() {
			ui.Column(c).Padding(th.Space(2.25)).Children(func() {
				pill := ui.ButtonBase(c).MinHeight(th.Space(7.5)).
					Padding(0, th.Space(2.5)).Radius(th.Space(3.75)).
					Border(1, edge).Background(tokens.Control)
				// The face's own pair of lines: top `LensRim`, bottom `LensEdge`.
				tokens.LitFace(pill)
				pill.Children(func() {
					ui.Text(c, "Run").FontSize(tokens.Body).FontWeight(600).
						TextColor(tokens.TextStrong)
				})
				pill.Bind(&ref)
				box = c.Resolve(ref).Bounds()
			})
		})
	}, 360, 140)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	img := tt.Image()
	if box.H < 12 || box.W < 20 {
		t.Fatalf("%s: measured a %v×%v box, not a control", appearanceName(dark), box.W, box.H)
	}
	x := int(box.X + box.W/2)
	// The framework snaps both edges with `math.Round` (`mygo@v0.7.3/ui/paint.go:160-167`).
	top := int(math.Round(float64(box.Y)))
	row := func(dy int) int { return int(imagePixel(img, x, top+dy).R) }
	return topEdgeArm{ground: row(-1), edge: row(0), rim: row(1), face: row(2)}
}

func topEdgeAbs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// TestTheTopEdgeIsTwoLinesAndTheOuterOneFollowsTheGround is the pin: the
// border row separates the face from the **ground** (so its sign follows the
// ground: darker on paper, brighter at night), the rim row lights the **face**
// in both appearances with the same sign, and the candidate that would make
// both rows lit erases the separation on the light sheet.
func TestTheTopEdgeIsTwoLinesAndTheOuterOneFollowsTheGround(t *testing.T) {
	// P13-gi 重定基线（§6.154）：这四行此前没有字面量 pin（量于 P13-gi，
	// 与 §6.140 的 `hairlineProbe` 同一把尺子）。
	want := map[string]topEdgeArm{
		"light": {ground: 247, edge: 216, rim: 236, face: 230},
		"dark":  {ground: 14, edge: 54, rim: 58, face: 39},
	}
	for _, dark := range []bool{false, true} {
		name := appearanceName(dark)
		got := measureTopEdge(t, dark, topEdgeShipped)
		if got != want[name] {
			t.Errorf("%s: the top edge reads ground %d / edge %d / rim %d / face %d, want %+v — "+
				"if the material moved, re-baseline §6.154 with the old → new pair rather than "+
				"letting the two lines drift into one",
				name, got.ground, got.edge, got.rim, got.face, want[name])
		}
		// 154-1: the outer line separates. A hairline that does not separate is
		// a line the user cannot see, whatever its colour.
		if sep := topEdgeAbs(got.edge - got.ground); sep < 24 {
			t.Errorf("%s: the outer line separates from the ground by %d levels (edge %d / ground %d), want ≥ 24",
				name, sep, got.edge, got.ground)
		}
		// 154-2: the crown lights the face, and it does so with the *same* sign
		// on both sheets — this is the half 140-5 mistook for the whole edge.
		if lift := got.rim - got.face; lift < 4 {
			t.Errorf("%s: the crown sits %+d levels over the face (%d vs %d), want ≥ +4 — "+
				"the lit edge is the mark of a face that can be touched (§3), in both appearances",
				name, lift, got.rim, got.face)
		}
		// 154-3: the separation line's sign follows the ground's own polarity.
		if (got.edge < got.ground) != (got.ground > 128) {
			t.Errorf("%s: the separation line runs the wrong way (edge %d / ground %d): "+
				"a paper ground wants a darker line and a night ground a brighter one — "+
				"the same alpha cannot separate both",
				name, got.edge, got.ground)
		}
		// 154-4: the candidate 140-5 asked for, measured on the same face. On the
		// light sheet it merges the two rows into one two-row band (the forbidden
		// 「类玻璃边」) and halves the boundary; at night it pushes the brightest
		// row *outside* the crown.
		cand := measureTopEdge(t, dark, topEdgeLitTop)
		shippedSep := topEdgeAbs(got.edge - got.ground)
		candSep := topEdgeAbs(cand.edge - cand.ground)
		if !dark {
			if cand.edge != cand.rim {
				t.Errorf("light: the rejected candidate reads edge %d / rim %d; it used to merge "+
					"the two rows into one 236 band — if they no longer merge, 140-5 is a real "+
					"choice again rather than a decision", cand.edge, cand.rim)
			}
			if candSep*2 > shippedSep {
				t.Errorf("light: the candidate keeps %d of the shipped %d levels of separation "+
					"(edge %d / ground %d) — it used to lose more than half of the boundary; "+
					"a lit border row on paper is a white edge, not a lit one",
					candSep, shippedSep, cand.edge, cand.ground)
			}
			t.Logf("light: shipped separates %d levels (edge %d / rim %d); the lit-top candidate %d (edge %d = rim %d)",
				shippedSep, got.edge, got.rim, candSep, cand.edge, cand.rim)
			continue
		}
		// At night the shipped hairline is already a bright line, so the candidate's
		// fault is the other way round: it becomes the brightest row on the face.
		if cand.edge <= cand.rim {
			t.Errorf("dark: the rejected candidate reads edge %d / rim %d; it used to put the "+
				"brightest row outside the crown (edge > rim) — that is a halo round the "+
				"face, not a lit edge on it", cand.edge, cand.rim)
		}
		t.Logf("dark: shipped separates %d levels (edge %d / rim %d); the lit-top candidate %d (edge %d / rim %d)",
			shippedSep, got.edge, got.rim, candSep, cand.edge, cand.rim)
	}
}

// TestDumpP13GiTopEdge writes the two columns the round argues about: the
// shipped top edge, and the candidate with the border row lit. Opt-in, as
// every dump in this package is:
//
//	FLOTER_P13GI_DUMP=1 go test -count=1 -run TestDumpP13GiTopEdge ./internal/settingsui/
func TestDumpP13GiTopEdge(t *testing.T) {
	if os.Getenv("FLOTER_P13GI_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13GI_DUMP=1 to write /tmp/floter-P13-gi-topedge-*.png")
	}
	for _, dark := range []bool{false, true} {
		dark := dark
		s := settings.Default()
		s.Theme = appearanceName(dark)
		a := New(settings.NewStore(s), Actions{})
		ground := realPageGround(t, dark)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			tokens := a.tokens(c)
			th := c.Theme()
			c.Root().Background(ground)
			ui.Row(c).Padding(14).Gap(th.Space(4)).Children(func() {
				for _, edge := range []ui.Color{tokens.ControlEdge, ui.RGBA(255, 255, 255, 0.24)} {
					edge := edge
					ui.Column(c).Gap(th.Space(1.5)).Children(func() {
						pill := ui.ButtonBase(c).MinHeight(th.Space(7.5)).Width(180).
							Padding(0, th.Space(2.5)).Radius(th.Space(3.75)).
							Border(1, edge).Background(tokens.Control)
						tokens.LitFace(pill)
						pill.Children(func() {
							ui.Text(c, "Run").FontSize(tokens.Body).FontWeight(600).
								TextColor(tokens.TextStrong)
						})
					})
				}
			})
		}, 440, 120)
		tt.SetDark(dark)
		tt.SetScale(3)
		tt.Frame()
		tt.Frame()
		name := "/tmp/floter-P13-gi-topedge-light.png"
		if dark {
			name = "/tmp/floter-P13-gi-topedge-dark.png"
		}
		f, err := os.Create(name)
		if err != nil {
			t.Fatalf("creating %s: %v", name, err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatalf("encoding %s: %v", name, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("closing %s: %v", name, err)
		}
	}
}
