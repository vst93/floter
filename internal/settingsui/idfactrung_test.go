package settingsui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// P13-do · 工具表单里那一行只读的集成 ID：它是这张表里唯一改不了的一行，
// 此前却用 About 卡那套**反过来的层级**（名字 caption / 值 emphasis）。
//
// `keyValue` 是 About 事实表的一行（P13-an §6.11：名字是值的旁注、值是主角），
// 但它也被借去画编辑表单顶上的那一行集成 ID（`settingsui.go:5148`，R9-3：
// 后端铸造、不可改）。于是同一张卡里出现了两套语法：ID 行的标题是
// **caption 9.9px / 500 / TextSecondary**，它下面每一行的标题是
// **emphasis 11.8px / 580 / TextStrong**；而 ID 的值是 emphasis / 580 /
// TextStrong —— 比它下面每一个可编辑字段里的字都响。
//
// 判据（本轮的新条目）：**只读不是一种墨，是一种结构** —— 一行没有控件面、
// 没有描边、没有可点的槽，就是只读。它在这张表里仍然是一行设置行的标题 +
// 一个值，所以标题回到这张表的标题档，值退到程序自己起的名那一档
// （P13-bt §6.38 `dataRow`：mono caption / 500 / `TextMuted`，因为这是一串
// 后端铸造的标识符，和检测到的程序路径同类）。

// editFormFixture opens Integrations → Manage → Edit… on an existing generated tool, so
// the form draws its read-only ID row above the editable rows.
func editFormFixture(t *testing.T, dark bool) (*ui.Tester, i18n.Settings) {
	t.Helper()
	copy := i18n.For("en").Settings
	tool := CustomTool{
		Name: "Ripgrep", Program: "/usr/bin/rg", Command: "rg",
		Args: "--floter", Version: "1.0.0",
		Platforms: []string{"darwin", "linux"},
		Params:    []extensions.ParamDefinition{{ID: "target", Kind: "text", Flag: "--target"}},
	}
	a := New(newStore(t), Actions{
		EditCustomTool:   func(string, CustomTool) {},
		ConnectLocalTool: func(CustomTool) {},
		ToolForEdit: func(id string) (CustomTool, bool) {
			if id != "local.abc" {
				return CustomTool{}, false
			}
			return tool, true
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "local.abc", Name: "Ripgrep", Enabled: true, Running: true, Generated: true,
		}}
	}
	tt, _ := renderIn(t, a, 720, 2200, dark)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.IntegrationsManage + " Ripgrep"); err != nil {
		t.Fatalf("manage: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Edit\u2026"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	tt.Frame()
	return tt, copy
}

// inkRowsOf is inkRows for a caller that only wants the pair in a log line.
func inkRowsOf(tt *ui.Tester, r ui.Rect) [2]float32 {
	top, bottom := inkRows(tt, r)
	return [2]float32{top, bottom}
}

// TestTheIntegrationIDKeepsTheFormRowGrammar pins the read-only ID row against
// the title the editable row below it wears: one card, one row grammar. The
// only mark of "you cannot change this" is the missing control face.
func TestTheIntegrationIDKeepsTheFormRowGrammar(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, copy := editFormFixture(t, dark)
		idName := mustFind(t, tt, copy.LocalToolId)
		idValue := mustFind(t, tt, "local.abc")
		// The row right under the ID: the tool's own name field.
		fieldLabel := mustFind(t, tt, copy.LocalToolName)
		nameInk, valueInk, fieldInk := inkSpan(tt, idName), inkSpan(tt, idValue), inkSpan(tt, fieldLabel)
		t.Logf("%s: id label %d / id value %d / field label %d (x %.1f / %.1f / %.1f; title rows %v, field title rows %v)",
			appearance, nameInk, valueInk, fieldInk, idName.X, idValue.X, fieldLabel.X,
			inkRowsOf(tt, idName), inkRowsOf(tt, fieldLabel))
		if d := nameInk - fieldInk; d > 20 || d < -20 {
			t.Errorf("%s: the read-only row's title resolves %d levels but the editable row's title "+
				"under it resolves %d — one card, one row grammar (§6.84)", appearance, nameInk, fieldInk)
		}
		if valueInk >= nameInk {
			t.Errorf("%s: the read-only value resolves %d levels and the row's own title only %d — "+
				"in a settings row the title is the loud line, the value is the answer (§6.84)",
				appearance, valueInk, nameInk)
		}
		if fieldInk-nameInk > 60 {
			t.Errorf("%s: the row title of the read-only row is %d levels quieter than the title of "+
				"an editable row (%d vs %d) — the read-only mark is the missing control face, "+
				"not a smaller type size (§6.84)", appearance, fieldInk-nameInk, nameInk, fieldInk)
		}
		// One optical line: the two ink bands overlap, as on every other row.
		nameTop, nameBottom := inkRows(tt, idName)
		valueTop, valueBottom := inkRows(tt, idValue)
		shorter := min(nameBottom-nameTop, valueBottom-valueTop)
		overlap := min(nameBottom, valueBottom) - max(nameTop, valueTop)
		if nameTop < 0 || valueTop < 0 {
			t.Fatalf("%s: the row drew no ink inside one of its boxes", appearance)
		}
		if overlap < 0.6*shorter {
			t.Errorf("%s: the title's ink spans rows %v-%v and its value rows %v-%v — they overlap by "+
				"%.0f of the shorter band, so the pair reads as two lines (§6.84)",
				appearance, nameTop, nameBottom, valueTop, valueBottom, overlap)
		}
	}
}

// TestDumpP13DOIntegrationID writes the frame this round measures: the ID row
// and the two editable rows under it, both appearances.
func TestDumpP13DOIntegrationID(t *testing.T) {
	if os.Getenv("FLOTER_P13DO_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13DO_DUMP=1 to write /tmp/floter-P13-do-idrow-*.png")
	}
	tag := os.Getenv("FLOTER_P13DO_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, copy := editFormFixture(t, dark)
		idName := mustFind(t, tt, copy.LocalToolId)
		idValue := mustFind(t, tt, "local.abc")
		fieldLabel := mustFind(t, tt, copy.LocalToolName)
		t.Logf("%s: id label ink %d, id value ink %d, name label ink %d",
			appearance, inkSpan(tt, idName), inkSpan(tt, idValue), inkSpan(tt, fieldLabel))
		writePNG(t, filepath.Join("/tmp", fmt.Sprintf("floter-P13-do-idrow-%s-%s.png", appearance, tag)), tt)
	}
}
