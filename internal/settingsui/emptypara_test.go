package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-fs · 空态里那两句自己的距离：一对是「标记与它那句」，一对是「理由自己折行」。
//
// §4.3 把空态的三对缝量成了 8 / 12 / 23px，但**只在会话页那一套字形上量过**
// （`emptyink_test.go` 走的是 `a.View` + `PageSessions`，标记是 `square-terminal`）。
// 空态是三个调用点共用的一块区域，标记由调用点挑：会话页 `square-terminal`、
// 自定义快捷键 `keyboard`、集成页 `package`。标记的**盒**是 22u，可是墨不填满盒子——
// 墨有多高是**字形自己的纵横比**决定的，于是「标记与它那句」这一对在墨上的距离
// 随字形摆，而它正是 §4.3 里「同一块」的那一档。
//
// 实测（`emptyState` 真画法，720 宽，深浅两套逐值相同）：
//
//	标记（调用点）        墨高/盒     标记→句子   句子→理由
//	square-terminal       17 / 19.8    9          13
//	keyboard              15 / 19.8   11          12
//
// 两句话：
//
//	① 盒的规则只有一个数（`head` 的 4u），墨上的差是字形自己的：宽字形（`keyboard`
//	   的墨只有 15 行）在同一个盒里上下留白更多，于是这一对在墨上远 2px。
//	② 这一对仍然是**最近**的一对（9 < 13、11 < 12），但 `keyboard` 那一处只剩 1px：
//	   §4.3 立的「至少近 2px」在它身上不成立。要抹平就得按字形各给一个 head gap
//	   ——那是把「一个数」变成「一张表」，判 `故意不做`（见合约 §6.138 的 138-3）。
//
// 另一轴：理由是一段 320u 度量的正文，长到两行时，**段内的行距**必须比它离上一件
// 东西更近，否则一段话散成两句。实测（en，自定义快捷键）：段内墨缝 5px、行距 16.0px
// （= Caption × 1.5），而句子→理由 12px。zh 那句更短，一行（267px < 288px 度量）。

type paraReading struct {
	region            ui.Rect
	title, hint       ui.Rect
	bands             [][2]int
	hintLines         [][2]int
	titleBand         [2]int
	markToTitle       float32
	titleToHint       float32
	pitch             float32
	hintLineGap       float32
	hintLineCount     int
	shippedMarkInkRow int
}

// emptyParaProbe draws one arm of the empty region through the product's own
// `emptyState` — with the mark that arm actually ships — and reads its ink.
func emptyParaProbe(t *testing.T, lang string, dark bool, arm string) paraReading {
	t.Helper()
	copy := i18n.For(lang).Settings
	a := New(newStore(t), Actions{})
	var regionRef ui.Handle
	var out paraReading
	var mark, title, hint string
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		th := c.Theme()
		var floor float32
		var plate bool
		var action func()
		switch arm {
		case "sessions":
			face := sessionsEmptyFaceFor(copy, false, false)
			mark, title, hint = face.Icon, face.Title, face.Hint
			floor, plate = th.Space(40), true
			action = func() { a.pageButton(c, copy.SessionsNew) }
		case "shortcuts":
			mark, title, hint = "keyboard", copy.ShortcutsNone, copy.ShortcutsEmptyHint
			action = func() { ui.Text(c, copy.ShortcutsAddFirst) }
		case "integrations":
			mark, title = "package", copy.IntegrationsEmpty
		default:
			t.Fatalf("unknown arm %q", arm)
		}
		e := a.emptyState(c, mark, title, hint, 0, floor, plate, action)
		e.Bind(&regionRef)
		defer func() { out.region = c.Resolve(regionRef).Bounds() }()
	}, 720, 460)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()

	out.title, _ = tt.Find(title)
	if hint != "" {
		out.hint, _ = tt.Find(hint)
	}
	out.bands = inkBands(tt, int(out.region.X), int(out.region.X+out.region.W),
		int(out.region.Y), int(out.region.Y+out.region.H))
	if len(out.bands) == 0 {
		t.Fatalf("%s/%s: the region has no ink", lang, arm)
	}
	for _, b := range out.bands {
		cy := float32(b[0]+b[1]) / 2
		switch {
		case cy >= out.hint.Y-0.5 && cy <= out.hint.Y+out.hint.H+0.5:
			out.hintLines = append(out.hintLines, b)
		case cy >= out.title.Y-0.5 && cy <= out.title.Y+out.title.H+0.5:
			if out.titleBand == [2]int{} {
				out.titleBand = b
			}
		}
	}
	out.hintLineCount = len(out.hintLines)
	out.markToTitle = float32(out.titleBand[0]-out.bands[0][1]) - 1
	if len(out.hintLines) > 0 {
		out.titleToHint = float32(out.hintLines[0][0]-out.titleBand[1]) - 1
	}
	if len(out.hintLines) > 1 {
		out.pitch = float32(out.hintLines[1][0] - out.hintLines[0][0])
		out.hintLineGap = float32(out.hintLines[1][0]-out.hintLines[0][1]) - 1
	}
	out.shippedMarkInkRow = out.bands[0][1] - out.bands[0][0] + 1
	return out
}

// TestTheMarkHangsOffItsSentenceOnEveryShippedMark: §4.3's "the mark belongs to
// the sentence it introduces" is a rule about the **pair**, and the pair is
// (glyph, sentence). The sessions page was the only arm anyone measured, so the
// pin below carries all three shipped marks — and the exact ink distances each
// one resolves, because the swing between them is the glyph's own aspect ratio,
// not a gap the region can pick.
func TestTheMarkHangsOffItsSentenceOnEveryShippedMark(t *testing.T) {
	want := map[string]float32{"sessions": 9, "shortcuts": 11, "integrations": 9}
	for _, dark := range []bool{false, true} {
		for _, arm := range []string{"sessions", "shortcuts", "integrations"} {
			r := emptyParaProbe(t, "en", dark, arm)
			if d := r.markToTitle - want[arm]; d > 1.1 || d < -1.1 {
				t.Errorf("%s (dark=%v): the mark hangs %.1fpx off its sentence, measured %.1f — "+
					"the shipped mark's ink moved (it is %d rows tall in a 19.8px box)",
					arm, dark, r.markToTitle, want[arm], r.shippedMarkInkRow)
			}
			// What the mark buys, per arm. The sessions and integrations marks
			// (square-terminal / package, 17 ink rows) hang 9px off their sentence
			// while the why sits 12px under it — the ladder §4.3 landed. The
			// shortcuts mark (`keyboard`, 15 ink rows in the same 19.8px box) resolves
			// the same 4u box gap 2px wider, which lands it exactly on the why's 11px:
			// the documented flat case. Pinned as *equal* rather than papered over —
			// see §6.138 row 138-3, where equalising it is 需拍板.
			ladder := map[string]float32{"sessions": 3, "shortcuts": 0}
			if r.hintLineCount > 0 {
				if d := (r.titleToHint - r.markToTitle) - ladder[arm]; d > 1.1 || d < -1.1 {
					t.Errorf("%s (dark=%v): the mark hangs %.1fpx off its sentence and the why %.1fpx further — "+
						"this arm's pinned ladder is %.0fpx (mark ink %d rows in a 19.8px box)",
						arm, dark, r.markToTitle, r.titleToHint-r.markToTitle, ladder[arm], r.shippedMarkInkRow)
				}
			}
			t.Logf("%s dark=%v: mark ink %d rows, mark→sentence %.1fpx, sentence→why %.1fpx",
				arm, dark, r.shippedMarkInkRow, r.markToTitle, r.titleToHint)
		}
	}
}

// TestAWrappedWhyIsStillOneParagraph: the why is the only part of the region
// that can be more than one line (a 320u measure). A paragraph whose own line
// pitch is looser than the distance to the statement above it has stopped
// being a paragraph — so the pins hold the paragraph's **inner** ink gap and
// its pitch, and hold them against the pair above.
func TestAWrappedWhyIsStillOneParagraph(t *testing.T) {
	for _, dark := range []bool{false, true} {
		en := emptyParaProbe(t, "en", dark, "shortcuts")
		if en.hintLineCount != 2 {
			t.Fatalf("dark=%v: the en why resolved %d ink lines, want 2 — the measure or the "+
				"copy moved, and this pin's subject is the wrap", dark, en.hintLineCount)
		}
		// Two numbers, two owners. The **ink** pitch (16.0) is the font's own line
		// advance for `Caption`: `LineHeight(1.5)` → `LineHeight(1.2)` paints the
		// same 16.0 (measured; M3 in the report), so the pitch is not a knob this
		// region owns. The **box** height is the knob: 30.0 = 2 × 15.0, and that is
		// what `LineHeight` moves (and what the region's own height follows).
		if d := en.pitch - 16.0; d > 0.6 || d < -0.6 {
			t.Errorf("dark=%v: the why's painted line pitch is %.1fpx, want the Caption font's own "+
				"advance 16.0 (not `LineHeight` — that moves the box only)", dark, en.pitch)
		}
		if d := en.hint.H - 30.0; d > 0.6 || d < -0.6 {
			t.Errorf("dark=%v: the two-line why's box is %.1f tall, want 2 × 15.0 = 30.0 "+
				"(`Caption` × `LineHeight(1.5)`)", dark, en.hint.H)
		}
		if en.titleToHint-en.hintLineGap < 4 {
			t.Errorf("dark=%v: the why's own lines are %.1fpx apart and the statement %.1fpx above — "+
				"the wrapped why reads as two statements, not one paragraph",
				dark, en.hintLineGap, en.titleToHint)
		}
		// zh ships the same sentence shorter: one line, and that is content, not
		// a different rhythm.
		zh := emptyParaProbe(t, "zh", dark, "shortcuts")
		if zh.hintLineCount != 1 {
			t.Errorf("dark=%v: the zh why wrapped to %d lines — the 320u measure is no longer "+
				"the en/zh boundary", dark, zh.hintLineCount)
		}
		if zh.hint.W > en.hint.W {
			t.Errorf("dark=%v: the zh why is %.1f wide and the en %.1f — the measure is not shared",
				dark, zh.hint.W, en.hint.W)
		}
		t.Logf("dark=%v: en why %v box %.1f pitch %.1f inner gap %.1f, statement %.1f above; zh %.1f wide",
			dark, en.hintLines, en.hint.H, en.pitch, en.hintLineGap, en.titleToHint, zh.hint.W)
	}
}

// TestDumpP13FSEmptyParagraphPNGs is the opt-in evidence dump (FLOTER_P13FS_DIR):
// the three shipped marks, both appearances.
func TestDumpP13FSEmptyParagraphPNGs(t *testing.T) {
	dir := os.Getenv("FLOTER_P13FS_DIR")
	if dir == "" {
		t.Skip("set FLOTER_P13FS_DIR to dump the empty-region mark evidence")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		a := New(newStore(t), Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			th := c.Theme()
			ui.Column(c).Gap(th.Space(2)).Children(func() {
				face := sessionsEmptyFaceFor(en, false, false)
				a.emptyState(c, face.Icon, face.Title, face.Hint, 0, th.Space(40), true,
					func() { a.pageButton(c, en.SessionsNew) })
				a.emptyState(c, "keyboard", en.ShortcutsNone, en.ShortcutsEmptyHint, 0, 0, false,
					func() { ui.Text(c, en.ShortcutsAddFirst) })
				a.emptyState(c, "package", en.IntegrationsEmpty, "", 0, 0, false, nil)
			})
		}, 720, 560)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Frame()
		name := "light"
		if dark {
			name = "dark"
		}
		path := filepath.Join(dir, fmt.Sprintf("floter-P13-fs-emptyparagraph-%s.png", name))
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
