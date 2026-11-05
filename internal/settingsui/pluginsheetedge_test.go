package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
)

// P13-gu · 插件配置浮层的通用 schema 边界（`docs/liquid-glass.md` §6.163–§6.168）。
//
// 这块浮层是**第三方输入**的渲染器：`plugincfg.Schema` 是插件自己声明的，
// 空标签、空标题、0.1 的步长、一个不在选项表里的旧值都合法。本轮把这一类
// 边界一次读进帧。

// sheetApp builds one App and one tester drawing a sheet the test can swap.
func sheetApp(t *testing.T, sheet *PluginSheet, w, h int) (*App, *ui.Tester) {
	t.Helper()
	a := New(newStore(t), Actions{})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ui.Transparent)
		a.PluginSheet(c, *sheet)
	}, w, h)
	tt.Frame()
	tt.Frame()
	return a, tt
}

func longSchema(plugin string) *plugincfg.Schema {
	s := &plugincfg.Schema{Plugin: plugin, Title: plugin}
	for i := 0; i < 12; i++ {
		s.Fields = append(s.Fields, plugincfg.Field{
			Key: "field" + string(rune('a'+i)), Label: "field" + string(rune('a'+i)), Kind: plugincfg.Text,
		})
	}
	return s
}

// 163-1 · 一张面的滚动位置不得接到下一张面上。
//
// `a.PluginScroll` 是 App 上的一格，`ui.Scroll` 的 key 是常量：插件 A 滚到
// 底之后切到插件 B，B 的第一帧就落在 A 的偏移上。与 P13-gs 的数字草稿同族。
func TestThePluginSheetForgetsTheScrollWhenThePluginChanges(t *testing.T) {
	sheet := &PluginSheet{Schema: longSchema("plug.a"), Values: plugincfg.Values{},
		Change: func(string, any) {}}
	a, tt := sheetApp(t, sheet, 720, 260)
	if a.PluginScroll.MaxY <= 0 {
		t.Fatalf("the fixture does not scroll: MaxY=%v", a.PluginScroll.MaxY)
	}
	// Scroll A to its bottom the way the pointer does.
	a.PluginScroll.Y = a.PluginScroll.MaxY
	tt.Frame()
	if a.PluginScroll.Y == 0 {
		t.Fatalf("the sheet did not keep its own offset: MaxY=%v", a.PluginScroll.MaxY)
	}
	// Redrawing the *same* plugin must keep it (a scroll is the user's).
	tt.Frame()
	if a.PluginScroll.Y == 0 {
		t.Fatal("a redraw of the same plugin reset the scroll")
	}
	// Switching plugins starts the new one at its own top.
	sheet.Schema = longSchema("plug.b")
	tt.Frame()
	if a.PluginScroll.Y != 0 {
		t.Fatalf("plugin B opened at plugin A's offset: Y=%v", a.PluginScroll.Y)
	}
	if !tt.HasText("plug.b") {
		t.Fatalf("plugin B's header is not on screen: %v", tt.Texts())
	}
}

// 164-1/164-2 · 滑块的读数就是会被保存的那个数。
func TestTheSliderReadoutIsTheNumberThatGetsSaved(t *testing.T) {
	cases := []struct {
		name  string
		field plugincfg.Field
		value float64
		want  string
	}{
		{"fraction", plugincfg.Field{Step: 0.1}, 0.4, "0.4"},
		{"fraction-unit", plugincfg.Field{Step: 0.1, Unit: "items"}, 0.3, "0.3 items"},
		{"quantized", plugincfg.Field{Step: 10, Unit: "items"}, 12.7, "10 items"},
		{"whole", plugincfg.Field{Step: 1, Unit: "days"}, 30, "30 days"},
		{"unstepped", plugincfg.Field{}, 300, "300"},
	}
	for _, tc := range cases {
		if got := boundedText(tc.field, tc.value); got != tc.want {
			t.Errorf("%s: boundedText(%v, %v) = %q, want %q", tc.name, tc.field.Step, tc.value, got, tc.want)
		}
	}
	// The readout and the dispatched value must agree on a real slider frame.
	schema := &plugincfg.Schema{Plugin: "plug.s", Title: "plug.s", Fields: []plugincfg.Field{
		{Key: "ratio", Label: "ratio", Kind: plugincfg.Slider, Min: 0, Max: 1, Step: 0.1},
	}}
	sheet := &PluginSheet{Schema: schema, Values: plugincfg.Values{"ratio": 0.4}, Change: func(string, any) {}}
	_, tt := sheetApp(t, sheet, 720, 300)
	if !tt.HasText("0.4") {
		t.Fatalf("a 0.1-step slider read %v, want 0.4", tt.Texts())
	}
}

// 165-1 · 没有标签的字段回落到 key（集成配置表单早就是这么做的）。
func TestAFieldWithoutALabelFallsBackToItsKey(t *testing.T) {
	schema := &plugincfg.Schema{Plugin: "plug.l", Title: "plug.l", Fields: []plugincfg.Field{
		{Key: "size", Kind: plugincfg.Number, Min: 1, Max: 99, Step: 1, Unit: "items"},
		{Key: "clear_history", Kind: plugincfg.Action, Confirm: "Sure", Cancel: "No", Failed: "failed"},
	}}
	sheet := &PluginSheet{Schema: schema, Values: plugincfg.Values{"size": 5.0},
		Change: func(string, any) {}, Arm: func(string) {}, Run: func(string) {}, Unarm: func(string) {}}
	_, tt := sheetApp(t, sheet, 720, 400)
	// The number row's label column.
	if !tt.HasText("size") {
		t.Errorf("a field with no label drew an empty label column: %v", tt.Texts())
	}
	// The action's own pill: an empty label made it a zero-width pill.
	if !tt.HasText("clear_history") {
		t.Errorf("an action with no label drew an empty pill: %v", tt.Texts())
	}
}

// 166-1 · 没有名字的 schema 不留一格 25.2px 的空表头。
func TestAnUntitledSchemaDoesNotDrawAnEmptyHeader(t *testing.T) {
	field := plugincfg.Field{Key: "size", Label: "size", Kind: plugincfg.Number, Min: 1, Max: 99, Step: 1}
	build := func(title string) float32 {
		schema := &plugincfg.Schema{Plugin: "plug.t", Title: title, Fields: []plugincfg.Field{field}}
		sheet := &PluginSheet{Schema: schema, Values: plugincfg.Values{"size": 5.0}, Change: func(string, any) {}}
		_, tt := sheetApp(t, sheet, 720, 400)
		rect, ok := tt.Find("size")
		if !ok {
			t.Fatalf("the field's label is not on screen (title %q): %v", title, tt.Texts())
		}
		return rect.Y
	}
	titled, untitled := build("plug.t"), build("")
	if untitled >= titled {
		t.Fatalf("an untitled schema did not save the header row: titled=%v untitled=%v", titled, untitled)
	}
	if drop := titled - untitled; drop < 20 || drop > 40 {
		t.Fatalf("the header row is %v tall, want the 25.2px row plus its gap", drop)
	}
}

// 165-2 · 越界的输入：框里写的必须是插件真正拿到的那个数。
func TestAnOutOfRangeNumberShowsWhatThePluginGot(t *testing.T) {
	schema := &plugincfg.Schema{Plugin: "plug.n", Title: "plug.n", Fields: []plugincfg.Field{
		{Key: "days", Label: "history", Kind: plugincfg.Number, Min: 0, Max: 100, Step: 1, Unit: "days"},
	}}
	var changes []change
	sheet := &PluginSheet{Schema: schema, Values: plugincfg.Values{"days": 5.0},
		Change: func(key string, value any) { changes = append(changes, change{key, value}) }}
	a, tt := sheetApp(t, sheet, 720, 300)
	unit := mustFind(t, tt, "days")
	tt.ClickAt(unit.X-30, unit.Y+unit.H/2)
	tt.Frame()
	if !tt.Focused("history") {
		t.Fatalf("the number box did not take focus: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnd)
	for range 8 {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Type("999")
	tt.Frame()
	tt.Frame()
	if len(changes) == 0 || changes[len(changes)-1] != (change{"days", float64(100)}) {
		t.Fatalf("the clamped value did not reach the plugin: %v", changes)
	}
	// The box paints its draft, not a `Text` node, so the draft is the frame's
	// record of what the user is looking at.
	if got := a.pluginNumberDrafts["plug.n\x00days"]; got != "100" {
		t.Fatalf("the box still shows the number the plugin never got: draft=%q", got)
	}
}

// 167-1 · 选到一个 ID 为空串的选项，回调必须落地。
//
// `choose` 的派发此前守着 `chosen != ""`：ID 是调用方给的身份，空串是合法的
// 一档（「不设置」/「off」），于是「选了它」被当成「没有选」——下拉框显示新值、
// 设置一个字节没动。`choose` 是设置页所有下拉的公共入口（字体、调色板、
// 语言、动作……），所以这条守卫影响的是**整个设置页**。
func TestAChooseAppliesAnOptionWhoseIDIsEmpty(t *testing.T) {
	a := New(newStore(t), Actions{})
	var applied []string
	options := []i18n.Option{{ID: "", Label: "off"}, {ID: "on", Label: "on"}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		ui.Column(c).FillWidth().Padding(20).Children(func() {
			a.choose(c, "Choice", "", options, "on", func(id string) { applied = append(applied, id) })
		})
	}, 520, 200)
	tt.Frame()
	trigger := mustFind(t, tt, "on")
	tt.ClickAt(trigger.X+trigger.W/2, trigger.Y+trigger.H/2)
	tt.Frame()
	item := mustFind(t, tt, "off")
	tt.ClickAt(item.X+item.W/2, item.Y+item.H/2)
	tt.Frame()
	if len(applied) != 1 || applied[0] != "" {
		t.Fatalf("choosing the empty-ID option dispatched %v, want one empty id", applied)
	}
}

// TestDumpP13GuUntitled is the opt-in evidence for §6.166: the same sheet with
// and without a title, so the 25.2px header row is visible (or not) by eye.
func TestDumpP13GuUntitled(t *testing.T) {
	if os.Getenv("FLOTER_P13GU_DUMP") == "" {
		t.Skip("opt-in screenshot dump: FLOTER_P13GU_DUMP=1")
	}
	for _, dark := range []bool{false, true} {
		schema := &plugincfg.Schema{Plugin: "plug.t", Title: "", Fields: []plugincfg.Field{
			{Key: "size", Label: "size", Kind: plugincfg.Number, Min: 1, Max: 99, Step: 1, Unit: "items"},
			{Key: "days", Label: "days", Kind: plugincfg.Number, Min: 0, Max: 100, Step: 1, Unit: "days"},
		}}
		sheet := &PluginSheet{Schema: schema, Values: plugincfg.Values{"size": 5.0, "days": 30.0},
			Change: func(string, any) {}}
		_, tt := sheetApp(t, sheet, 720, 260)
		tt.SetDark(dark)
		tt.Frame()
		name := fmt.Sprintf("/tmp/floter-P13-gu-untitled-%s.png", appearanceName(dark))
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
		t.Logf("%s: first label Y=%v", name, mustFind(t, tt, "size").Y)
	}
}
