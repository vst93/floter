package launcher

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-bo · 插件关闭那一页的答案，此前画在列表行的静息墨里。
//
// `clipboardItems`（`search.go`）在剪贴板插件关掉时返回**一条** `status` 行
// ——`Title` 是 `copy.ClipboardDisabled`「剪贴板历史已关闭。可在"设置 > 集成"中启用。」。
// 那是这一页**唯一的内容**：没有列表、没有页脚、没有 chips。可它走的是
// `row()` 的 `item.status` 臂，墨是 `TextMuted` 480——与输入框自己的占位句
// 「Type to search」**逐字节相同**（实测浅 116 / 深 164，两套外观、中英两语）。
//
// 同一件事在本仓有两条路径：**浏览器**插件关掉时走 `emptyMessage` → 居中陈述
// （`TextStrong`，实测浅 27 / 深 237）。两条路径差 **89 / 73 级**，而这一页唯一的
// 答案恰好是轻的那一条。
//
// 这一页的墨阶梯只有**两颗齿**：安静那颗（`TextMuted`，占位句与分组标题
// 「Recently launched」的邻档）与陈述那颗（`TextStrong`）。中间的 `TextSecondary`
// 不是一个可用的中间档——两套外观上它和 `TextMuted` 只差 **8 / 8 级**，而且在
// **深色档上它比 `TextMuted` 更暗**（实测 156 对 164），也就是同一页上
// 「次一档」在两个外观里方向相反。所以本轮是换齿，不是抬档。

// noteBandProbe renders the clipboard-off page for one appearance and returns the
// tester plus the boxes of the note's sentence and its icon column.
func noteBandProbe(t *testing.T, dark bool) (*ui.Tester, ui.Rect, ui.Rect) {
	t.Helper()
	a := testApp()
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	clipboardOff(t, a)
	a.clipboard = true
	a.Query = "clipboard "
	tt := renderIn(t, a, dark)
	sentence, ok := tt.Find(a.copy().ClipboardDisabled)
	if !ok {
		t.Fatalf("the note's sentence is missing (%s): %q", appearance, tt.Texts())
	}
	// The 28u icon column is the row's first child; the sentence starts after the
	// 8u padding + 28u column + 10u gap = 46u.
	iconBox := ui.Rect{X: sentence.X - 42, Y: sentence.Y, W: 36, H: sentence.H}
	return tt, sentence, iconBox
}

// inkExtremes reads the darkest and brightest pixel level inside a box.
func inkExtremes(img *image.RGBA, box ui.Rect) (int, int) {
	lo, hi := -1, -1
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		for x := int(box.X); x < int(box.X+box.W); x++ {
			if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
				continue
			}
			r, g, b, a := img.At(x, y).RGBA()
			if a>>8 == 0 {
				continue
			}
			l := (int(r>>8) + int(g>>8) + int(b>>8)) / 3
			if lo < 0 || l < lo {
				lo = l
			}
			if hi < 0 || l > hi {
				hi = l
			}
		}
	}
	return lo, hi
}

// inkLevel is the ink's own depth against its ground: a light appearance reads
// the darkest pixel (ink is darker than paper), a dark one reads the brightest.
func inkLevel(lo, hi int, dark bool) int {
	if dark {
		return hi
	}
	return lo
}

// groundLevel samples the panel's own face away from any ink.
func groundLevel(tt *ui.Tester) int {
	lo, hi := inkExtremes(tt.Image(), ui.Rect{X: 4, Y: 300, W: 8, H: 20})
	if lo == hi {
		return hi
	}
	return hi
}

// noteStatementFloor is the level a page-level statement has to reach on this
// page. The launcher's statement tooth is `TextStrong`; the pin's own bar is set
// below it (180 levels off the ground on light, 180 above it on dark) so the
// rung is pinned, not the token's exact composite.
const noteStatementFloor = 180

// TestTheOffPluginsNoteIsOnTheStatementsTooth — ① the sentence of the launcher's
// one-page answer is a statement, not a list row's resting note. Before P13-bo it
// read 137 / 141 levels off the ground (the same number as the field's own
// placeholder); a statement on this page is ≥180.
func TestTheOffPluginsNoteIsOnTheStatementsTooth(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, sentence, _ := noteBandProbe(t, dark)
		ground := groundLevel(tt)
		lo, hi := inkExtremes(tt.Image(), sentence)
		level := inkLevel(lo, hi, dark)
		contrast := level - ground
		if !dark {
			contrast = ground - level
		}
		t.Logf("%s: note sentence %d vs ground %d = %d levels", appearanceOf(dark), level, ground, contrast)
		if contrast < noteStatementFloor {
			t.Errorf("the off-plugin note (%s) sits %d levels off the ground (%d vs %d); "+
				"a page whose whole content is one sentence must state it at the statement rung (≥%d) — "+
				"it used to read the same as the field's own placeholder",
				appearanceOf(dark), contrast, level, ground, noteStatementFloor)
		}
	}
}

// TestTheOffPluginsNoteMatchesTheOtherPagesAnswer — ② the same statement (the
// plugin is off) must not be drawn at two levels depending on which plugin is
// off. This is measured against the browser page, which reaches the note through
// `emptyMessage`: the two sentences are different text, so the pin reads their
// ink *levels*, not their pixels.
func TestTheOffPluginsNoteMatchesTheOtherPagesAnswer(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, sentence, _ := noteBandProbe(t, dark)
		lo, hi := inkExtremes(tt.Image(), sentence)
		note := inkLevel(lo, hi, dark)

		// The browser-off page reaches the same sentence through `emptyMessage`.
		a := testApp()
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		a.browser = true
		a.Query = "browser "
		a.browserAsked = true
		a.browserAskedFor = a.browserQuery()
		a.browserFound = BrowserResults{Found: true, Disabled: true}
		bt := renderIn(t, a, dark)
		br, ok := bt.Find(a.copy().BrowserDisabled)
		if !ok {
			t.Fatalf("the browser note is missing (%s): %q", appearance, bt.Texts())
		}
		blo, bhi := inkExtremes(bt.Image(), br)
		browser := inkLevel(blo, bhi, dark)
		t.Logf("%s: clipboard-off note %d, browser-off note %d (%d apart)", appearanceOf(dark), note, browser, browser-note)
		if diff := browser - note; diff < -40 || diff > 40 {
			t.Errorf("the same statement («the plugin is off») is %d levels apart depending on which plugin "+
				"(%s): the clipboard page draws %d, the browser page %d; two paths for one sentence is two designs",
				diff, appearanceOf(dark), note, browser)
		}
	}
}

// TestTheNoteIconIsOneStepUnderItsSentence — ③ §4.7's footnote: the mark is one
// step under the sentence it introduces, on *both* appearances. Before P13-bo the
// icon kept `TextTertiary`, which was the step under the *old* sentence
// (`TextMuted`) — once the sentence moved to `TextStrong`, `TextTertiary` read 52
// levels *over* it on the dark appearance.
func TestTheNoteIconIsOneStepUnderItsSentence(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, sentence, icon := noteBandProbe(t, dark)
		ground := groundLevel(tt)
		// Contrast, not raw level: on the light sheet ink is *darker* than the
		// paper, so the two appearances' levels run in opposite directions. What
		// §4.7 pins is the *depth* of each mark against the ground it sits on.
		slo, shi := inkExtremes(tt.Image(), sentence)
		ilo, ihi := inkExtremes(tt.Image(), icon)
		sc := contrastOf(inkLevel(slo, shi, dark), ground)
		ic := contrastOf(inkLevel(ilo, ihi, dark), ground)
		t.Logf("%s: sentence %d levels off the ground, icon column %d (%d apart)", appearanceOf(dark), sc, ic, sc-ic)
		// A *step*, not a cliff: below the sentence, and within one rung of it.
		// `TextTertiary` (the pre-P13-bo value) reads 117 / 125 levels under it —
		// that is a different register, not the next step down.
		if gap := sc - ic; gap < 40 || gap > 110 {
			t.Errorf("the note's icon column (%s) sits %d levels under its own sentence "+
				"(%d vs %d off the ground); §4.7's footnote puts the mark one step beneath the "+
				"sentence it introduces — 40..110", appearanceOf(dark), gap, ic, sc)
		}
	}
}

// TestTheNoteKeepsTheListsOwnBand — ④ structural pin: the note is still a row on
// the list's status band — no plate, no pointer state, not a result row's
// register. The whole box is re-measured so a later round cannot quietly give it
// a plate or the result row's weight.
func TestTheNoteKeepsTheListsOwnBand(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, sentence, _ := noteBandProbe(t, dark)
		// The note is one line at the body step: its own text box is the body's
		// line box, not the result row title's.
		if sentence.H > 24 {
			t.Errorf("the note's box (%s) is %v tall; a status note is one body line on the 30u band, "+
				"not a result row", appearanceOf(dark), sentence.H)
		}
		// The note carries no frame: the tail of its own row, past the sentence,
		// is the panel's own face — no plate, no keyline, no chip. Sample the row
		// at the sentence's own vertical centre from the panel's left edge up to
		// the icon column.
		tail := ui.Rect{X: 4, Y: sentence.Y + sentence.H/2, W: sentence.X - 50, H: 2}
		lo, hi := inkExtremes(tt.Image(), tail)
		if hi-lo > 6 {
			t.Errorf("the note's band (%s) has chrome on it: the row above the sentence runs %d..%d "+
				"(span %d); a status note is information, not a door — no plate, no keyline",
				appearanceOf(dark), lo, hi, hi-lo)
		}
	}
}

// TestTheNoteIsNotAResultRowsRegister — ⑤ the structural half of ④: the note must
// not drift onto the result row's register. A result row's title is
// `FontSize+2` / 580 (`rowBody`); the note is the body step at 480, one line,
// ellipsized. This is a source pin, so a later round that "unifies" the two by
// accident names the line it did it on.
func TestTheNoteIsNotAResultRowsRegister(t *testing.T) {
	src, err := os.ReadFile("launcher.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	// The whole status arm, from its guard to the end of its text element.
	start := strings.Index(text, "if item.status {")
	if start < 0 {
		t.Fatal("the status arm is gone from launcher.go")
	}
	arm := text[start:]
	end := strings.Index(arm, "\n\t\treturn\n\t}")
	if end < 0 {
		t.Fatal("could not find the end of the status arm")
	}
	arm = arm[:end]
	for _, want := range []string{
		"FontSize(t.FontSize).FontWeight(480)", // the body step, not the title's +2/580
		"TextColor(tokens.TextStrong)",         // P13-bo: the statement tooth
		"SingleLine()",                         // one line, as the reference's nowrap
		"TextColor(tokens.TextSecondary)",      // the icon column, one step under
	} {
		if !strings.Contains(arm, want) {
			t.Errorf("the status arm no longer contains %q; the off-plugin note is a page's statement "+
				"on the list's own 30u band, not a result row", want)
		}
	}
	if strings.Contains(arm, "FontSize(t.FontSize + 2)") || strings.Contains(arm, "FontWeight(580)") {
		t.Error("the status arm now wears a result row's title register (+2 / 580); " +
			"a note is not a result")
	}
	if strings.Contains(arm, "Background(") || strings.Contains(arm, "LitFace(") || strings.Contains(arm, "Lens(") {
		t.Error("the status arm grew a plate or a lit face; a status note is information, not a door " +
			"(it has no pointer state and no key)")
	}
}

// The round's opt-in visual evidence.
func TestDumpP13BONote(t *testing.T) {
	if os.Getenv("FLOTER_P13BO_DUMP") == "" {
		t.Skip("set FLOTER_P13BO_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13BO_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		tt, sentence, icon := noteBandProbe(t, dark)
		slo, shi := inkExtremes(tt.Image(), sentence)
		ilo, ihi := inkExtremes(tt.Image(), icon)
		t.Logf("%s (%s): sentence %d..%d, icon %d..%d, ground %d",
			appearanceOf(dark), tag, slo, shi, ilo, ihi, groundLevel(tt))
		path := filepath.Join("/tmp", "floter-P13-bo-clipboardoff-"+appearanceOf(dark)+"-"+tag+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", path)
	}
}

// appearanceOf names an appearance for a log line; the launcher package's own
// test helper for this is local to another file's theme, so this round keeps its
// own name.
func appearanceOf(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

// contrastOf is how far a mark's level sits off its ground, whichever way the
// appearance runs (light ink is darker than the paper, dark ink is brighter).
func contrastOf(level, ground int) int {
	if level > ground {
		return level - ground
	}
	return ground - level
}
