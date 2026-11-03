package terminalui

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-fc · 终端空态：**一句话 + 两条出路**，这句话是那两条出路的标题。
//
// 病（`docs/liquid-glass.md` §6.124）：三件东西挂在同一条 `Gap(t.Space(2.5))` 上，
// 墨上实测（900×600 真页面，en/zh × 深/浅四臂逐值相同）：
//
//	标题底 → 出路 1 顶   = 11.3px   ← 这一对是「标题 → 它引入的第一条出路」
//	出路 1 底 → 出路 2 顶 = 13.6px   ← 这一对是**同级**的两条出路
//
// 标题离自己的第一条出路比两条出路彼此之间还近 2.3px：三件读成一条等距竖排清单，
// 句尾直接黏在第一个能按的行上。§4.3 的规矩是「只有这一对里有一件是**可以按的面**
// 的那一对才抬开」，于是外层抬一整档到 `Space(5)`、两条出路留在自己的内列里。
//
// 三条判据（每套外观 × 每种语言）：
//
//	① 陈述 → 出路 1 的墨缝是 **20.3px**（抬开的一档）；
//	② 出路 1 → 出路 2 的墨缝是 **13.6px**（同级不动）；
//	③ ① 比 ② 至少宽 5px —— 这条是**排序**判据，字体度量变了也不会误报，
//	   但「标题贴得比同级还近」这个病一定红。

const (
	emptySpaceStatementGap = 20.3
	emptySpacePeerGap      = 13.6
)

// emptySpaceGaps is the ink distance between two text boxes: the bottom of the
// upper one to the top of the lower one.
func emptySpaceGap(t *testing.T, tt *ui.Tester, upper, lower string) float32 {
	t.Helper()
	a, ok := tt.Find(upper)
	if !ok {
		t.Fatalf("the page does not draw %q: %v", upper, tt.Texts())
	}
	b, ok := tt.Find(lower)
	if !ok {
		t.Fatalf("the page does not draw %q: %v", lower, tt.Texts())
	}
	return b.Y - (a.Y + a.H)
}

func TestTheEmptyStatesSentenceSitsAboveItsWaysOut(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			t.Run(lang+" "+appearance, func(t *testing.T) {
				term := i18n.For(lang).Terminal
				tt := pageHeightFrame(t, "empty", lang, dark, 900, 600)
				statement := emptySpaceGap(t, tt, term.EmptyTitle, term.EmptyNew)
				peer := emptySpaceGap(t, tt, term.EmptyNew, term.EmptyBack)
				if d := statement - emptySpaceStatementGap; d > 0.6 || d < -0.6 {
					t.Errorf("%s: the statement sits %.1fpx above its first way out, not %.1f — the "+
						"empty state's outer gap moved (§6.124)", appearance, statement, emptySpaceStatementGap)
				}
				if d := peer - emptySpacePeerGap; d > 0.6 || d < -0.6 {
					t.Errorf("%s: the two ways out are %.1fpx apart, not %.1f — they are peers and their "+
						"gap is not the statement's (§6.124)", appearance, peer, emptySpacePeerGap)
				}
				if statement < peer+5 {
					t.Errorf("%s: the statement is %.1fpx from its first way out and the two ways out are "+
						"%.1fpx apart — the sentence reads as the first item of the list, not as its title "+
						"(§6.124)", appearance, statement, peer)
				}
			})
		}
	}
}

// TestTheEmptyStatesReasonStaysWithItsSentence pins the failure arm: the reason
// a session could not start belongs to the sentence, not to the ways out. The
// statement column keeps its own 2.5u seam (measured 9.0px in ink — the reason
// wears the body step, so its box starts higher than the title's), while the
// distance from the reason to the first way out is the lifted one (20.3px).
func TestTheEmptyStatesReasonStaysWithItsSentence(t *testing.T) {
	const reason = "libghostty-vt is missing"
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			term := i18n.For("en").Terminal
			a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, func(terminal.Options) (*terminal.Terminal, error) {
				return nil, errors.New(reason)
			})
			a.EnsureSession()
			tt := renderThemed(t, a, 900, 600)
			if dark {
				tt.SetDark(true)
				tt.Frame()
			}
			withReason := emptySpaceGap(t, tt, term.EmptyTitle, reason)
			toWaysOut := emptySpaceGap(t, tt, reason, term.EmptyNew)
			if withReason > 11 {
				t.Errorf("%s: the failure reason is %.1fpx below its own sentence — it reads as a new "+
					"paragraph, not as the rest of that sentence (§6.124)", appearance, withReason)
			}
			if withReason < 7 {
				t.Errorf("%s: the failure reason is %.1fpx below its own sentence — welded to the title, "+
					"two lines of one sentence read as one cramped line (§6.124)", appearance, withReason)
			}
			if d := toWaysOut - emptySpaceStatementGap; d > 0.6 || d < -0.6 {
				t.Errorf("%s: the reason sits %.1fpx above the first way out, not %.1f — the statement "+
					"block's lifted seam moved (§6.124)", appearance, toWaysOut, emptySpaceStatementGap)
			}
			if withReason >= toWaysOut {
				t.Errorf("%s: the reason is %.1fpx from its sentence and %.1fpx from the ways out — the "+
					"reason belongs to the sentence (§6.124)", appearance, withReason, toWaysOut)
			}
		})
	}
}

// TestDumpP13FCEmptySpacePNGs writes the round's evidence: the empty page, both
// appearances. Opt-in.
func TestDumpP13FCEmptySpacePNGs(t *testing.T) {
	dir := os.Getenv("FLOTER_P13FC_DIR")
	if dir == "" {
		t.Skip("set FLOTER_P13FC_DIR to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := pageHeightFrame(t, "empty", "en", dark, 900, 600)
		path := filepath.Join(dir, "floter-P13-fc-empty-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- the directory is the caller's own /tmp path
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
