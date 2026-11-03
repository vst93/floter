package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-ey · 设置面板的「说明句」一族：一页上两个寄存器，还有一句比自己的标题
// 往左探出 2px（`docs/liquid-glass.md` §6.120）。
//
// Integrations 页上，同一角色（分组标题下面、列表上面那一句解释）此前有**两个**
// 寄存器：Base plugins 的 `settings-section__hint`（正文档、`--text-tertiary`、
// 2u 缩进）与 Detected 的 `.extensions-section-hint`（caption 档、4u 缩进）。
// 实测（760×1400，深浅逐值相同）：两句的行盒 17 vs **15**；而 Base plugins 那句
// 的首个字形停在 X=188.8，它自己的标题在 X=190.8 —— **说明句比标题左 2px**。
//
// 这些 pin 量的是**读出来的**位置与行盒，不是请求的值：一句解释的角色不该由它
// 落在哪一组决定。

// hintPage draws the Integrations page over one app, with one base plugin group
// and one detected tool, so both explanations are on the same scroll.
func hintPage(t *testing.T, dark bool, w, h int) (*ui.Tester, i18n.Settings) {
	t.Helper()
	en := i18n.For("en").Settings
	a := New(settings.NewStore(settings.Default()), Actions{
		OpenPluginConfig: func(string) {},
		Detected: func() []DetectedTool {
			return []DetectedTool{{Name: "ripgrep", Path: "/usr/bin/rg", RuntimeAvailable: true}}
		},
		ConnectDetected: func(DetectedTool) {},
	})
	a.Integrations = func() []Integration { return nil }
	tt, _ := renderIn(t, a, w, h, dark)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("open Integrations: %v", err)
	}
	tt.Frame()
	return tt, en
}

// lineBoxOf is the height of one line of a text, read off the ink: the box's own
// height divided by the number of ink bands it holds.
func lineBoxOf(tt *ui.Tester, box ui.Rect) (float32, int) {
	bands := inkBands(tt, int(box.X), int(box.X+box.W), int(box.Y), int(box.Y+box.H))
	if len(bands) == 0 {
		return 0, 0
	}
	return box.H / float32(len(bands)), len(bands)
}

func TestThePanelsExplanationsStartOnTheirTitlesOwnLine(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, en := hintPage(t, dark, 760, 1400)
		pairs := []struct {
			name, hint, title string
		}{
			{"base", en.BasePluginsHint, en.BasePlugins},
			{"detected", en.DetectedHint, en.DetectedTitle},
		}
		for _, p := range pairs {
			hint, ok := tt.Find(p.hint)
			if !ok {
				t.Fatalf("%s: %s hint missing", appearanceName(dark), p.name)
			}
			title, ok := tt.Find(p.title)
			if !ok {
				t.Fatalf("%s: %s title missing", appearanceName(dark), p.name)
			}
			if d := hint.X - title.X; d < -0.5 || d > 0.5 {
				t.Errorf("%s: the %s explanation starts %.1fpx off its title's line (hint %.1f, title %.1f)",
					appearanceName(dark), p.name, d, hint.X, title.X)
			}
		}
	}
}

func TestThePanelsExplanationsShareOneLineBox(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, en := hintPage(t, dark, 760, 1400)
		baseHint, ok := tt.Find(en.BasePluginsHint)
		if !ok {
			t.Fatalf("%s: the base-plugins hint is missing", appearanceName(dark))
		}
		detHint, ok := tt.Find(en.DetectedHint)
		if !ok {
			t.Fatalf("%s: the detected hint is missing", appearanceName(dark))
		}
		baseLine, baseLines := lineBoxOf(tt, baseHint)
		detLine, detLines := lineBoxOf(tt, detHint)
		if baseLines == 0 || detLines == 0 {
			t.Fatalf("%s: an explanation painted no ink (base %d bands, detected %d)",
				appearanceName(dark), baseLines, detLines)
		}
		// P13-ey 重定基线：Detected 那句此前是 caption 档，行盒 15；两句现在同为
		// 正文档（17），差在 0.5px 以内。
		if d := detLine - baseLine; d < -0.5 || d > 0.5 {
			t.Errorf("%s: the two explanations are %.1fpx apart in line box (base %.1f over %d lines, detected %.1f over %d lines)",
				appearanceName(dark), d, baseLine, baseLines, detLine, detLines)
		}
		if detLine < 16.5 {
			t.Errorf("%s: the detected explanation's line box is %.1f — still the caption step the round removed",
				appearanceName(dark), detLine)
		}
	}
}

func TestTheExplanationsOnBothPagesWearTheSameStep(t *testing.T) {
	// The settings cards' own group hint (`sectionHint`) is the pane's reference
	// register: the body step. A panel hint that measures differently is a
	// second register, whatever its own CSS said.
	tt, en := hintPage(t, false, 760, 1400)
	detHint, ok := tt.Find(en.DetectedHint)
	if !ok {
		t.Fatal("the detected hint is missing")
	}
	detLine, _ := lineBoxOf(tt, detHint)

	page := New(settings.NewStore(settings.Default()), Actions{
		Detected: func() []DetectedTool { return nil },
	})
	page.Page = PageShortcuts
	tt2, _ := renderIn(t, page, 760, 1400, false)
	card, ok := tt2.Find(en.ShortcutsAppHint)
	if !ok {
		t.Fatalf("the shortcuts group hint is missing: %v", tt2.Texts())
	}
	cardLine, _ := lineBoxOf(tt2, card)
	if d := detLine - cardLine; d < -0.5 || d > 0.5 {
		t.Errorf("the panel's explanation (%.1f) and the cards' own group hint (%.1f) are %.1fpx apart: two registers again",
			detLine, cardLine, d)
	}
}

// TestDumpP13EYHintRegisters is the round's visual evidence (opt-in:
// FLOTER_P13EY_DUMP=1).
func TestDumpP13EYHintRegisters(t *testing.T) {
	if os.Getenv("FLOTER_P13EY_DUMP") == "" {
		t.Skip("set FLOTER_P13EY_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13EY_TAG")
	if tag == "" {
		tag = "after"
	}
	dir := os.Getenv("FLOTER_P13EY_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	for _, dark := range []bool{false, true} {
		tt, _ := hintPage(t, dark, 760, 1400)
		name := filepath.Join(dir, fmt.Sprintf("floter-P13-ey-integrations-%s-%s.png", appearanceName(dark), tag))
		f, err := os.Create(name)
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
		t.Logf("wrote %s", name)
	}
}
