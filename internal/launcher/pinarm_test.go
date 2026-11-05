package launcher

import "testing"

// P13-gw · 输出页页头的那枚钉住键（`HistoryPin`）属于**正文臂**。
//
// 这一页的文字臂与列表臂共用一个页头（`outputBody` 的 strip）。钉住键此前不看
// 臂：列表的 `Text` 是**序列化协议**（`{"output":[…]}`），点下去钉住的就是这串
// 线格式——窗口里出现的是插件的传输格式，不是它给人的答案。参考实现按同一条规则
// 拒绝列表：`pin-entry.ts` 的 `resolvePluginView` 答 "list" → `null` → 无钉住键。
//
// 列表臂并不因此少一个出口：它旁边就有自己的分离键（`DetachOutput`，保住列表）。
func TestThePinKeyBelongsToTheTextArm(t *testing.T) {
	listApp := testApp()
	listApp.Actions.PinText = func(string, string) {}
	listApp.Actions.DetachOutput = func(OutputView) {}
	list := runListOutput(t, listApp, `[{"id":"r0","title":"Alpha"}]`)
	list.Frame()
	if rect, ok := list.Find(listApp.copy().HistoryPin); ok {
		t.Fatalf("a list still offers the text pin at %+v: it would pin the wire format %q",
			rect, list.Texts())
	}
	if _, ok := list.Find(listApp.copy().OutputDetach); !ok {
		t.Fatalf("the list arm lost its own way out: %q", list.Texts())
	}

	textApp := testApp()
	textApp.Actions.PinText = func(string, string) {}
	textApp.Actions.DetachOutput = func(OutputView) {}
	text := runTextOutput(t, textApp, "hello")
	text.Frame()
	if _, ok := text.Find(textApp.copy().HistoryPin); !ok {
		t.Fatalf("the text arm lost its pin key: %q", text.Texts())
	}
}
