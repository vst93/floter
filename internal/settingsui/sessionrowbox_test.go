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
	tt = render(t, a, 720, 760)
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
// from its own content rather than from a floor constant: the title starts one
// row padding (9) under the row's top in both shapes, and the box is 56 when the
// session reported no directory, taller when it did.
func TestTheSessionRowMeetsTheReferencesFloor(t *testing.T) {
	_, top, bottom, title := sessionRowFrame(t, productionSession())
	if h := bottom - top + 1; h != 56 {
		t.Errorf("the session row is %dpx tall (%d..%d), want the reference's 56u floor", h, top, bottom)
	}
	if got := int(title.Y) - top; got != 9 {
		t.Errorf("the title starts %dpx inside the row, want the row's own 2.25u padding (9)", got)
	}

	withDir := productionSession()
	withDir.Dir = "~/workspace/floter"
	_, dirTop, dirBottom, dirTitle := sessionRowFrame(t, withDir)
	if h := dirBottom - dirTop + 1; h <= 56 {
		t.Errorf("the row with a cwd is %dpx tall (%d..%d), want it to grow past the 56u floor by its cwd line",
			h, dirTop, dirBottom)
	}
	if got := int(dirTitle.Y) - dirTop; got != 9 {
		t.Errorf("with a cwd the title starts %dpx inside the row, want the same 9: the padding owns the top, the floor owns the rest", got)
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
