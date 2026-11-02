package settingsui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-da · 会话行的高度：P13-cz 量到的 46~47px 是**夹具**的读数，不是生产的读数。
//
// P13-cz 的「仍待」第 1 条把这个数当成缺一块地板的证据：参考的
// `.session-manager__row` 是 `min-height: 56px`
// （`main:src/styles/settings.css:1237-1244`，注释写着「a taller floor, because a
// session is two lines of text plus a footer of metadata」），而本仓的 `cardRow`
// 地板只有 44（`rowFloorPx`）。本轮把三种夹具各渲一帧量盒，结论是：
//
//	只有状态行（生产形状）  卡片 177..235 → 行 **178..233 = 56**
//	带 cwd（三行）          卡片 177..251 → 行 178..249 = 72
//	只有标题（P13-cz 夹具） 卡片 177..224 → 行 178..222 = 45
//
// 生产里 `Session.Created` 永远非零（`internal/shell/shell.go:1070` 直接抄
// `terminalui.Session.Created`），`sessionMeta` 于是至少吐出时间这一段，行至少是
// 「标题 + 状态行」两行字：标题盒 23 + 行距 1 + 状态盒 15 + 行自己的 9+9 内边距
// = 57，落在盒算术上是 **56**。也就是说参考那条 56 的地板在本仓**由内容自然满足**，
// 补一个 `Space(14)` 的常量在生产上**一个像素都不会变**——那不是一个缺口。
//
// 这两条 pin 钉住的是「它现在就是 56」，这样以后谁把行压薄（拿掉内边距、改行距、
// 改字阶）都会在这里变红，而不是等到有人再去量一遍才发现它掉到 45。

// sessionRowFrame renders one session on the sessions page and returns the frame,
// the row's own box (the card's face between its rim and its foot lines) and the
// title's text box.
func sessionRowFrame(t *testing.T, session Session) (tt *ui.Tester, rowTop, rowBottom int, title ui.Rect) {
	t.Helper()
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{
		CloseSessionAt: func(int) {},
		SelectSession:  func(int) {},
		NewSession:     func() {},
	})
	a.Sessions = func() []Session { return []Session{session} }
	tt = renderThemed(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	hint, ok := tt.Find(en.SessionsActive)
	if !ok {
		t.Fatalf("the section hint is missing: %q", tt.Texts())
	}
	// x=238 is inside the card, right of the leading chip (which ends at 232) and
	// left of the text column (which starts at 244), so the column reads the
	// card's planes and not its glyphs.
	const cardColumn = 238
	ground := pixelAt(tt, cardColumn, int(hint.Y)-3)
	cardTop, cardBottom := blockAbove(tt, cardColumn, int(hint.Y)-3, ground)
	// The card's box is its rim plus its face plus its foot: `card()` draws a 1px
	// border above and two lines below (the bottom border's own rim and its
	// shadow side), so the row lives between cardTop+1 and cardBottom-2.
	rowTop, rowBottom = cardTop+1, cardBottom-2
	var okTitle bool
	if title, okTitle = tt.Find(session.Title); !okTitle {
		t.Fatalf("the session's title is missing: %q", tt.Texts())
	}
	return tt, rowTop, rowBottom, title
}

// productionSession is the shape the shell always hands over: a title, a state
// word and a start time (`internal/shell/shell.go:1060-1072`).
func productionSession() Session {
	return Session{
		ID: 1, Title: "zsh", Running: true, Active: true, Index: 0,
		Size: "120×40", Note: "Attached",
		Created: time.Date(2026, 10, 30, 9, 5, 0, 0, time.Local),
	}
}

// The row a session actually draws is the reference's 56u box, and it gets there
// from its own floor constant: the title starts one row padding under the row's
// top in both shapes, and the box is 56 tall, taller when the session reported a
// directory.
//
// P13-dd：上面这些数（56 / 9）是 P13-da 在没有装设计主题的取景里量的（见下面
// 第一条 pin 的注释）；生产上这一行是 **49 / 8**，参考的 56 地板没有满足。
// P13-dd 重定基线：**56 → 49**，上一轮量的是**生产上不存在**的取景。
//
// `sessionRowFrame` 此前走的是 `render()`（没有 `c.SetTheme`），于是同一行里一半的字
// 读 `a.tokens(c)`（设计阶梯，caption 9.9 / body 10.8），一半读 `c.Theme()`（框架库存
// 14）——标题 15pt、状态行 14pt，行高被这两个库存数撑到 56。生产里 shell 在画面板
// 之前就 `c.SetTheme(surface.Tokens.Theme)`（`internal/shell/shell.go:4262`），所以标题
// 真正是 11.7、状态行 11.7：行 = 8（`Space(2.25)`）+ 18 + 1 + 15 + 8 = **49**。
//
// **P13-ea 重定基线：49 → 55（布局盒 56）。** 这一行现在穿参考自己那条
// `.session-manager__row { min-height: 56px }`（`sessionRowFloorPx`，字面 px），
// 那 7px 全落在状态行**下方**。
//
// 关于 P13-dz 的「处方被证伪」：那一轮给行注了 `MinHeight(Space(14))`（= 50.4）、
// 量到像素不动，于是判「盒不由行元素拥有」。本轮逐档量清楚（§6.96 的表）：
// `MinHeight(52) → 51`、`MinHeight(56) → 55`、`Height(70) → 69`——**行盒就是行给的**；
// 不动的只有 50.4 这一个值，因为它低于/等于这一行内容自身的 50.4，而帧是按整像素
// 扫的（比布局盒整体少 1：`44 → 43`、`60 → 59`、`70 → 69`），50.2 与 50.4 都截到 49。
// 所以错的是 `Space(14)` 这个**值**（u = 3.6 → 50.4，不是参考写的 56），不是行级地板。
func TestTheSessionRowsHeightAgainstTheReferencesFloor(t *testing.T) {
	_, top, bottom, title := sessionRowFrame(t, productionSession())
	// 55 是帧的读数，布局盒是 56：像素扫描比布局盒整体少一行（本文件里
	// `Height(44) → 43`、`Height(60) → 59`、`Height(70) → 69` 都是同一个偏移）。
	if h := bottom - top + 1; h != sessionRowFloorPx-1 {
		t.Errorf("the session row is %dpx tall (%d..%d), want %d on the frame's own reading "+
			"(the reference's 56px floor, one pixel under the layout box)",
			h, top, bottom, sessionRowFloorPx-1)
	}
	if got := int(title.Y) - top; got != 8 {
		t.Errorf("the title starts %dpx inside the row, want the row's own 2.25u padding (8): "+
			"the floor's air belongs under the text, not above it", got)
	}

	withDir := productionSession()
	withDir.Dir = "~/workspace/floter"
	_, dirTop, dirBottom, dirTitle := sessionRowFrame(t, withDir)
	if h := dirBottom - dirTop + 1; h <= sessionRowFloorPx-1 {
		t.Errorf("the row with a cwd is %dpx tall (%d..%d), want it to grow past the %d by its cwd line",
			h, dirTop, dirBottom, sessionRowFloorPx-1)
	}
	if got := int(dirTitle.Y) - dirTop; got != 8 {
		t.Errorf("with a cwd the title starts %dpx inside the row, want the same 8: the padding owns the top, the floor owns the rest", got)
	}
}

// The floor is the reference's literal 56px, not the u-metric `Space(14)` that
// §6.95's prescription used: at the shipped step 14u is 50.4, and 50.4 is what
// this row's content already measures, so a `Space(14)` floor draws nothing at
// all (P13-dz measured exactly that and read it as "the row does not own its
// box"; §6.96 has the ladder that disproves it). The two pins below are the
// numbers that keep the mistake from coming back, and the third one says where
// the floor's extra pixels end up: **under** the metadata line.
func TestTheSessionRowsFloorIsTheReferencesLiteral(t *testing.T) {
	if sessionRowFloorPx != 56 {
		t.Errorf("the session row's floor is %v, want the reference's literal 56px "+
			"(`.session-manager__row { min-height: 56px }`)", float32(sessionRowFloorPx))
	}
	// `Space(14)` at the shipped step (u = 3.6) is 50.4, and this row's content
	// already measures 50.4 — a floor at or under that is invisible on this row,
	// which is the whole of §6.95.
	const space14Shipped = 50.4
	if float32(sessionRowFloorPx) <= space14Shipped {
		t.Errorf("the session row's floor is %v, not above the u-metric `Space(14)` (%v): "+
			"a floor there draws nothing", float32(sessionRowFloorPx), float32(space14Shipped))
	}

	session := productionSession()
	tt, top, bottom, _ := sessionRowFrame(t, session)
	meta, ok := tt.Find(session.Note)
	if !ok {
		t.Fatalf("the state line is missing: %q", tt.Texts())
	}
	// The floor's air: from the metadata line's bottom to the row's bottom edge.
	// The metadata block ends 42.2 under the row's top (8.1 + 18 + 1 + 15), so the
	// 56 row leaves **13.8** there where its content alone left the row's own
	// 2.25u padding (8.1). The frame reads one pixel under the layout box
	// (see the test above), so it reports 12 where it used to report 8.
	if got := bottom - int(meta.Y+meta.H); got != 12 {
		t.Errorf("the state line ends %dpx above the row's bottom, want 12 on the frame's reading "+
			"(13.8 in layout: 56 − 42.2; it was 8 before the floor): "+
			"the floor's air belongs under the metadata, not inside the two-line block", got)
	}
	if meta.Y <= float32(top) {
		t.Errorf("the state line sits at y=%v, inside the row's top edge %d", meta.Y, top)
	}
}

// The cwd line and the state line are one block: the same caption register (15),
// the same text column, and the row's own 1px line gap between them. P13-cy gave
// the integrations card this same ledger (its state word had been a step larger
// than the facts it closed); this page was never measured, and here the block is
// already one register.
func TestTheSessionRowsStateLineIsOneRegisterWithItsBlock(t *testing.T) {
	session := productionSession()
	session.Dir = "~/workspace/floter"
	tt, _, _, _ := sessionRowFrame(t, session)
	cwd, ok := tt.Find(session.Dir)
	if !ok {
		t.Fatalf("the cwd line is missing: %q", tt.Texts())
	}
	meta, ok := tt.Find(session.Note)
	if !ok {
		t.Fatalf("the state line is missing: %q", tt.Texts())
	}
	if cwd.H != meta.H {
		t.Errorf("the cwd line is %v tall and the state line %v, want one caption register", cwd.H, meta.H)
	}
	if cwd.X != meta.X {
		t.Errorf("the two lines start at different columns: cwd %v, state %v", cwd.X, meta.X)
	}
	if got := int(meta.Y) - int(cwd.Y+cwd.H); got != 1 {
		t.Errorf("the state line stands %dpx under the cwd line, want the row's own 1px line gap", got)
	}
	// The facts after the state word share its register and its line.
	size, ok := tt.Find(session.Size)
	if !ok {
		t.Fatalf("the size fact is missing: %q", tt.Texts())
	}
	if size.Y != meta.Y || size.H != meta.H {
		t.Errorf("the size fact sits at y=%v h=%v, want the state word's y=%v h=%v", size.Y, size.H, meta.Y, meta.H)
	}
}
