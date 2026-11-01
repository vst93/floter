package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-cy · `docs/liquid-glass.md` §6.68 —— meta 行收尾的那个状态字，和它收尾的
// 三个事实是同一档字号。
//
// 参考的 `.extension-row__meta` 声明 `font-size: var(--text-caption)`
// （`main:src/styles/extensions.css:960-970`），`.extension-status` 只声明 padding /
// line-height / radius（`:1007-1015`），字号是从 meta 行**继承**来的。本仓把它写成
// `t.FontSize-1`（12）而事实写 `tokens.Caption`（11），于是同一行上最后那个词比前面
// 三个词大一级，并且行盒高一级：实测改前事实 `h=15`、状态字 `h=20` —— 整条 meta 行
// 的横向尺寸（以及标题行的高度）被收尾那个词决定，标题行白长 5px，卡里每一行都跟着
// 往下挪 5px。改后四个节点同一个 `y=390`、同一个 `h=15`。

// metaLineFacts is the measured fact nodes of the meta line, in reference order.
var metaLineFacts = []string{"1.2.3 · dev.floter.probed", "· Extension package"}

// TestTheMetaLinesClosingWordIsOneRegister pins the register: the state word
// that closes the meta line has the same line box as the three facts it
// closes, so the line's cross size is the caption's and not the closing
// word's.
func TestTheMetaLinesClosingWordIsOneRegister(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		tt := metaLineTester(t, dark, false)
		var fact ui.Rect
		for i, want := range metaLineFacts {
			r, ok := tt.Find(want)
			if !ok {
				t.Fatalf("%s: the meta line is missing %q: %q", name, want, tt.Texts())
			}
			if i == 0 {
				fact = r
				continue
			}
			if r.Y != fact.Y || r.H != fact.H {
				t.Errorf("%s: %q sits at y=%v h=%v, the first fact at y=%v h=%v: the facts are not one line",
					name, want, r.Y, r.H, fact.Y, fact.H)
			}
		}
		state, ok := tt.Find("· Running")
		if !ok {
			t.Fatalf("%s: the meta line's state word is missing: %q", name, tt.Texts())
		}
		if state.H != fact.H {
			t.Errorf("%s: the closing state word is %v tall, the facts it closes are %v: the last word is a step larger than the line it closes",
				name, state.H, fact.H)
		}
		if state.Y != fact.Y {
			t.Errorf("%s: the state word starts at y=%v, the facts at y=%v: the line is taller than its own text",
				name, state.Y, fact.Y)
		}
		// The same register in the row's control slot: the runtime-unavailable
		// badge is the reference's `.extension-status--broken` chip, so its
		// label is a caption too, not a step above one.
		badge, ok := tt.Find("Tool unavailable")
		if !ok {
			t.Fatalf("%s: the unavailable badge is missing: %q", name, tt.Texts())
		}
		if badge.H != fact.H {
			t.Errorf("%s: the unavailable badge's label is %v tall, the meta facts %v: the two state marks of one row speak two sizes",
				name, badge.H, fact.H)
		}
	}
}

// TestTheBrokenChipsLabelIsACaption pins the boxed arm: a broken row's warm
// chip carries a caption label, the same size as a meta fact — the chip's own
// padding is what makes the chip taller, which is the reference's own geometry
// (`.extension-status--broken { padding: 2px 6px }`, `:1020-1025`), not a
// larger font.
func TestTheBrokenChipsLabelIsACaption(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		tt := metaLineTester(t, dark, true)
		fact, ok := tt.Find(metaLineFacts[0])
		if !ok {
			t.Fatalf("%s: the meta line is missing %q: %q", name, metaLineFacts[0], tt.Texts())
		}
		chip, ok := tt.Find("Broken")
		if !ok {
			t.Fatalf("%s: the broken chip is missing: %q", name, tt.Texts())
		}
		if chip.H != fact.H {
			t.Errorf("%s: the broken chip's label is %v tall, a meta fact %v: the chip's height comes from its padding, not from a larger font",
				name, chip.H, fact.H)
		}
	}
}

// metaLineTester renders the integrations page with one row whose meta line is
// fully populated. `broken` swaps the state word for the boxed warm chip.
func metaLineTester(t *testing.T, dark, broken bool) *ui.Tester {
	t.Helper()
	store := newStore(t)
	if dark {
		if err := store.Update(func(s *settings.Settings) { s.Theme = "dark" }); err != nil {
			t.Fatal(err)
		}
	}
	a := New(store, Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.probed", Name: "Probed Tool", Enabled: true,
			Version:     "1.2.3",
			Description: "Runs the probe suite",
			Broken:      broken,
		}}
	}
	tt := render(t, a, 720, 1700)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt
}

// TestDumpP13CyMetaPNGs is this round's visual evidence: the integrations page
// with one card whose meta line closes with a state word. Opt-in
// (`FLOTER_PNG=1`) so the suite does not write files.
func TestDumpP13CyMetaPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-cy-meta-<appearance>.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := metaLineTester(t, dark, false)
		f, err := os.Create("/tmp/floter-P13-cy-meta-" + appearance + ".png")
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
