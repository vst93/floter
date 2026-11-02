package settingsui

import (
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-dn · 一个按钮的确认语比它的静息语长，于是按下的那一刻整行会重新排版。
//
// `contentAction` 用**同一个胶囊**显示静息标签和 1.6s 的确认语（`Copy script`
// → `Script copied`，13 字对 11 字），胶囊的宽度由孩子撑开，所以确认语一上
// 屏，右边那颗 `Export script` 就被推开 —— 一次点击动了两颗按钮，而用户只是
// 复制了一段文本。同一个表单的程序模式同理（`Copy command` / `Command copied`，
// `Copy execution plan` / `Execution plan copied`）。
//
// 这条 pin 量的不是「按钮多宽」，是**按下前后兄弟按钮的位置**：一次状态反馈
// 不许改动这一行的布局。
func contentActionFrame(t *testing.T, dark bool) (*ui.Tester, i18n.Settings) {
	t.Helper()
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{
		CopyText:         func(string) {},
		ConnectLocalTool: func(CustomTool) {},
		ExportScript:     func(string, string) {},
	})
	tt := render(t, a, 720, 1600)
	tt.SetDark(dark)
	tt.Frame()
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatal(err)
	}
	a.toolCommand = "notes-tool"
	tt.Frame()
	return tt, copy
}

func TestTheCopyButtonsDoNotReflowTheRow(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, copy := contentActionFrame(t, dark)
		export, ok := tt.Find(copy.LocalToolExportScript)
		if !ok {
			t.Fatalf("dark=%v: the export button is missing: %q", dark, tt.Texts())
		}
		copied, ok := tt.Find(copy.LocalToolCopyScript)
		if !ok {
			t.Fatalf("dark=%v: the copy button is missing: %q", dark, tt.Texts())
		}
		before := export.X
		if err := tt.Click(copy.LocalToolCopyScript); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if !tt.HasText(copy.LocalToolScriptCopied) {
			t.Fatalf("dark=%v: the copy button did not confirm: %q", dark, tt.Texts())
		}
		after, ok := tt.Find(copy.LocalToolExportScript)
		if !ok {
			t.Fatalf("dark=%v: the export button left the row: %q", dark, tt.Texts())
		}
		t.Logf("dark=%v: label %+v, confirmation %+v, export %.1f → %.1f",
			dark, copied, mustFind(t, tt, copy.LocalToolScriptCopied), before, after.X)
		if after.X != before {
			t.Errorf("dark=%v: the confirmation pushed the export button from x=%.1f to x=%.1f"+
				" — a press is not a layout change", dark, before, after.X)
		}
	}
}

// The round's visual evidence, opt-in for the same reason as every other dump:
// a test run should not litter /tmp. `FLOTER_PNG_TAG` labels the pair so the
// before/after frames of the same surface are distinguishable.
func TestDumpP13DNContentRowPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, copy := contentActionFrame(t, dark)
		export, _ := tt.Find(copy.LocalToolExportScript)
		dump := func(state string) {
			img := tt.Image()
			crop := image.Rect(160, int(export.Y)-20, img.Bounds().Max.X-160, int(export.Y)+44)
			out, err := os.Create("/tmp/floter-P13-dn-contentrow-" + appearance + "-" + tag + "-" + state + ".png")
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(out, img.SubImage(crop)); err != nil {
				t.Fatal(err)
			}
			out.Close()
		}
		dump("rest")
		if err := tt.Click(copy.LocalToolCopyScript); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		dump("copied")
	}
}
