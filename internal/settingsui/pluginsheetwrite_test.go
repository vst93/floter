package settingsui

import (
	"image"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
)

// sheetTester draws a plugin sheet whose inputs the test owns: the schema, the
// values and the callbacks all live in the test, so the host's own answer to a
// write can be simulated between frames (the real host normalizes what a
// control sends and hands back what the plugin actually stored).
func sheetTester(t *testing.T, a *App, sheet *PluginSheet, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ui.Transparent)
		a.PluginSheet(c, *sheet)
	}, w, h)
	tt.Frame()
	tt.Frame()
	return tt
}

// dumpSheet writes a frame to /tmp when FLOTER_PNG is set: the round's evidence
// images, never part of the ordinary suite.
func dumpSheet(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	if os.Getenv("FLOTER_PNG") == "" {
		return
	}
	path := "/tmp/floter-" + name + ".png"
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	var img *image.RGBA = tt.Image()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Logf("%s", path)
}

// P13-hk（`docs/liquid-glass.md` §6.186）：文本字段在回车那一帧把框里的原文交上
// 去，宿主把值收一遍（`plugincfg.coerce`：去首尾空白，全空白记成空值）。框此前
// 一直画着交上去的原文，于是「框写 ` /tmp/x `、插件存 `/tmp/x`」要等到失焦才对
// 齐。交上去的原文现在记一帧，宿主换了值就把草稿交回去。
func TestTheTextBoxShowsTheValueTheHostStored(t *testing.T) {
	for _, language := range []string{"en", "zh"} {
		copy := i18n.For(language).Settings
		a := New(newStore(t), Actions{})
		schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(), nil)
		values["custom_base_dir"] = ""
		key := schema.Plugin + "\x00custom_base_dir"
		var sent []string
		sheet := PluginSheet{Schema: schema, Values: values}
		sheet.Change = func(key string, value any) {
			text, _ := value.(string)
			sent = append(sent, text)
			// The host's answer: the stored value is the trimmed one.
			values["custom_base_dir"] = strings.TrimSpace(text)
		}
		tt := sheetTester(t, a, &sheet, 720, 900)
		hint := mustFind(t, tt, copy.BrowserCustomDirHint)
		tt.ClickAt(hint.X+90, hint.Y+hint.H+20)
		if !tt.Focused(copy.BrowserCustomDir) {
			t.Fatalf("%s: the text editor did not take focus", language)
		}
		tt.Type("  /tmp/profile  ")
		tt.Frame()
		tt.Key(0, ui.KeyEnter)
		sheet.Values = values
		tt.Frame()
		if len(sent) != 1 || sent[0] != "  /tmp/profile  " {
			t.Fatalf("%s: Enter did not submit the typed text: %q", language, sent)
		}
		if draft, ok := a.pluginTextDrafts[key]; ok {
			t.Fatalf("%s: the box still paints the text that was sent: %q (host holds %q)",
				language, draft, values["custom_base_dir"])
		}
		// 证据图：框里此时画的是宿主存下的 `/tmp/profile`（没有首尾空白）。
		dumpSheet(t, tt, "P13-hk-text-adopted-"+language)
	}
}

// P13-hk（§6.186）：回车交一份和宿主一样的文本不是一次改动。此前它照样派发：写
// 成功是白写一遍同一个值，被拒就在一块没改过的面上亮出「保存失败」。
func TestEnterOnUnchangedTextDoesNotWrite(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(), nil)
	values["custom_base_dir"] = "/already/stored"
	var changes []change
	sheet := PluginSheet{Schema: schema, Values: values, Change: func(key string, value any) {
		changes = append(changes, change{key, value})
	}}
	tt := sheetTester(t, a, &sheet, 720, 900)
	hint := mustFind(t, tt, copy.BrowserCustomDirHint)
	tt.ClickAt(hint.X+90, hint.Y+hint.H+20)
	if !tt.Focused(copy.BrowserCustomDir) {
		t.Fatal("the text editor did not take focus")
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(changes) != 0 {
		t.Fatalf("Enter on an unchanged field wrote it: %v", changes)
	}
	// A real edit still goes through.
	tt.Type("/next")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(changes) != 1 || changes[0].value != "/already/stored/next" {
		t.Fatalf("a real edit was not submitted: %v", changes)
	}
}

// P13-hk（§6.186）：读数是「拖一下会存下的那个数」。一个手改过的配置文件把
// `max_items` 放到界外时，轨道按框架自己的区间停在头上，读数此前照样印着界外的
// 那个数。收进界之后两者同一条账。
