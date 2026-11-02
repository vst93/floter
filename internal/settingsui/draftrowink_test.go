package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-di · 半成品的那一行：它缺的是**一格键**，不是整行的亮度。
//
// 参考把「还没写完的自定义快捷键行」整行洗成 `.settings-row--muted` 的 0.62
// （`settings.css:564-569`），这个移植照着做了。但 0.62 是个**乘法**，而这一行有
// 两档墨，乘法没法让两档同时落在阶梯上——实测（整行扫描，`litSum` 三通道和，
// 一个亮度级 = 3；本文件的两条 pin 就是把下面这张表钉住）：
//
//	外观   档           行标题墨        行内提示墨（`textMuted`）
//	浅色   未洗         27（= text）    128（= textMuted）
//	浅色   0.62         107（= textSecondary，巧合）  173（**低过** textTertiary 142）
//	浅色   0.75         79（两档之间）  —
//	浅色   0.50         133（两档之间） —
//	深色   未洗         237（= text）   133（= textMuted）
//	深色   0.62         159（textSecondary 154）       99（**低过** textTertiary 110）
//	深色   0.75         187（两档之间） —
//	深色   0.50         133（≈ textMuted，另一对档之间）
//
// 也就是说：0.62 让标题**恰好**冒充了 `textSecondary` 一档（浅色一个字节都不差），
// 而同一行里本来最安静的那一档墨被推到阶梯之外（没有哪个 token 是这个亮度）。
// 换因子只是把这两件事换个位置。更糟的是这道淡洗掉的是**把这一行写完所必须碰的
// 控件**：选择器的受光边、输入框的面、录制键的键帽全成了 62% 亮的面——§3 说受光边
// 是「这块面可以被碰」的记号，而这是整页里唯一一块**必须**被碰的面。
//
// 所以这一行不再洗：它和一条已保存的行是同一行、同一套墨。「还没写完」由它唯一
// 缺的那一格说——录制槽是空的（`TestCustomShortcutsPage` 钉的是那个人能看到的事实）。

// rowInk is one row's scan: the plane it is drawn on, the darkest pixel inside it
// and the brightest one. On the light appearance the ink is the darkest thing in
// a row and the plane the brightest; on the dark one it is the other way round.
type rowInk struct {
	face, lo, hi int
}

// ink reports the row's ink and its distance from the row's own plane, in
// litSum units, whichever side of the plane that ink falls on.
func (r rowInk) ink() (value, contrast int) {
	if r.face-r.lo > r.hi-r.face {
		return r.lo, r.face - r.lo
	}
	return r.hi, r.hi - r.face
}

// draftRowProbe draws one custom-shortcut row on a clean plane — the row being
// made and a saved one, from the same functions the page uses — and returns the
// scan. `whole` fills the draft in completely; a draft that is not whole is the
// state the muting used to be keyed on.
func draftRowProbe(t *testing.T, dark, draft, whole bool) rowInk {
	t.Helper()
	skipWithoutTheReferenceFont(t)
	ground := realPageGround(t, dark)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	entry := settings.CustomShortcut{Key: "Cmd+J", Action: "plugin:clipboard"}
	a := New(store, Actions{
		CustomShortcuts: func() []settings.CustomShortcut {
			return []settings.CustomShortcut{entry}
		},
	})
	a.customDraftOpen = true
	if whole {
		a.customKey, a.customAction = entry.Key, entry.Action
	}
	text := i18n.For("en").Settings
	var row ui.Rect
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(24).Width(460).Children(func() {
			ui.Column(c).Children(func() {
				if draft {
					a.customDraftRow(c, text)
					return
				}
				a.customShortcutRow(c, text, entry, 0)
			}).Bind(&ref)
			row = c.Resolve(ref).Bounds()
		})
	}, 520, 260)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	img := tt.Image()
	if row.W < 100 || row.H < 20 {
		t.Fatalf("dark=%v draft=%v: measured a %v×%v row, not a row", dark, draft, row.W, row.H)
	}
	// The mode of the row box is the plane it is drawn on; going in two rows and
	// two columns keeps the rounded corners and the 1px rules out of the count.
	hist := map[int]int{}
	lo, hi := 1<<30, -1
	for y := int(row.Y) + 2; y < int(row.Y+row.H)-2; y++ {
		for x := int(row.X) + 2; x < int(row.X+row.W)-2; x++ {
			q := litSum(imagePixel(img, x, y))
			hist[q]++
			if q < lo {
				lo = q
			}
			if q > hi {
				hi = q
			}
		}
	}
	face, seen := 0, -1
	for q, n := range hist {
		if n > seen {
			face, seen = q, n
		}
	}
	if hi-lo < 6 {
		t.Fatalf("dark=%v draft=%v: the row has no ink at all (every pixel within %d units)", dark, draft, hi-lo)
	}
	return rowInk{face: face, lo: lo, hi: hi}
}

// TestTheUnfinishedShortcutRowWearsTheSameInkAsASavedOne is the round's pin: the
// row that is not whole is drawn in the ladder's own inks, not through a
// multiplication of them.
func TestTheUnfinishedShortcutRowWearsTheSameInkAsASavedOne(t *testing.T) {
	for _, dark := range []bool{false, true} {
		saved := draftRowProbe(t, dark, false, false)
		draft := draftRowProbe(t, dark, true, false)
		whole := draftRowProbe(t, dark, true, true)

		savedInk, savedContrast := saved.ink()
		draftInk, draftContrast := draft.ink()
		wholeInk, _ := whole.ink()
		t.Logf("dark=%v  saved %d/%d  draft %d/%d  whole %d  (face %d/%d/%d)",
			dark, savedInk, savedContrast, draftInk, draftContrast, wholeInk, saved.face, draft.face, whole.face)

		// litSum is a three-channel sum: one brightness level is three of these.
		const level = 3
		// The two rows are the same row. A saved row and the row being made are
		// drawn from the same helpers with the same tokens, so their strongest ink
		// is the same rung — the draft's is not a 0.62 multiple of it.
		if d := draftInk - savedInk; d > level || d < -level {
			t.Errorf("dark=%v: the unfinished row's ink is %d, the saved row's is %d (%+d, %.1f levels apart). `.settings-row--muted` washed the whole row by 0.62, which puts this ink on a rung the token set does not have — light 27 → 107, dark 237 → 159 — and pushes the row's quiet ink below `textTertiary` (light 128 → 173, dark 133 → 99). A multiplication cannot be a rung for two inks at once",
				dark, draftInk, savedInk, d, float64(d)/float64(level))
		}
		// And it is the same distance from its plane: the wash moved the ink
		// toward the plane (gravity shortened the row's own contrast), which is
		// what made the row read as "nothing here" rather than "not finished".
		if d := draftContrast - savedContrast; d > level || d < -level {
			t.Errorf("dark=%v: the unfinished row's ink sits %d units off its plane, a saved row's sits %d (%+d). De-emphasis by multiplication flattens the row's own contrast by that factor",
				dark, draftContrast, savedContrast, d)
		}
		// The whole draft is the control for the same claim on the other side:
		// filling the row in never changed its ink either.
		if d := wholeInk - savedInk; d > level || d < -level {
			t.Errorf("dark=%v: a whole draft's ink is %d against the saved row's %d (%+d)", dark, wholeInk, savedInk, d)
		}
	}
}

// TestDumpP13DIDraftRow writes the round's visual evidence: the shortcuts page
// with the row being made open, both appearances, tagged by the caller so a
// before/after pair comes out of the same build.
func TestDumpP13DIDraftRow(t *testing.T) {
	if os.Getenv("FLOTER_P13DI_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13DI_DUMP=1 to write /tmp/floter-P13-di-*.png")
	}
	skipWithoutTheReferenceFont(t)
	tag := os.Getenv("FLOTER_P13DI_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
			t.Fatal(err)
		}
		entries := []settings.CustomShortcut{{Key: "Cmd+Shift+P", Action: "plugin:clipboard"}}
		a := New(store, Actions{
			CustomShortcuts: func() []settings.CustomShortcut { return entries },
		})
		a.selectPage(PageShortcuts)
		a.beginCustomDraft()
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, 720, 1100)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Scroll(400, 300, 0, 900)
		tt.Frame()
		name := fmt.Sprintf("/tmp/floter-P13-di-draftrow-%s-%s.png", appearanceName(dark), tag)
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", name)
	}
}
