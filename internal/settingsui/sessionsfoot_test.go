package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-cz · 会话页的满态：列表与页脚的关系（先量，不改）。
//
// 「空态 / 满态」这一族里，会话页的**空态**已经收过两轮（P13-aj 的三件东西、
// P13-ap 的出口按种类取间距），**满态**那一半一直是空格子——P13-aq 的「仍待」
// 第 4 条②点名的就是它：「会话多条时列表与页脚的关系」。
//
// 量法：`render(a, 720, 760)` → 点侧栏 `Sessions` → 取一帧；x=238 这一列在卡片
// 里、避开了前导 chip（止于 230）与标签列（起于 242），于是同一列上「卡片 /
// 组注 / 页脚胶囊」三块各自的面都能单独扫出来。浅色档实测（默认 interface step）：
//
//	卡片盒        y 177..318
//	组注（hint）盒 y 327..349
//	页脚胶囊盒     y 371..401（31px = 30u 的 MinHeight + 一对 1px 的边）
//
// 三段盒对盒的间距：
//
//	卡底 → 注      8px  = `Space(2)`（`sectionGroup` 的组内 gutter）
//	注底 → 胶囊    22px = `Space(5.5)`（`pageColumn` 的页 gap）
//
// 这两档落在 §4.3 那条阶梯上（8px 同一块 < 12px 同一段的下一行 < 22px 一个可以
// 按的新东西），和空态那一支（P13-ap 的 0.5u / 3u 两档）是同一句话：**间距按
// 「这个东西是什么」取，不按「它在第几行」取**。所以本轮**不改**——把量到的账钉
// 住；谁要动这两个数，得先解释为什么它是别的种类的东西。
//
// 第二条 pin 回答另一半：「页脚是不是钉在视口底」。换成一枚会话之后两个间隙
// 一个像素都不动 —— 页脚是**跟着内容走**的，不是一条 fixed 底栏，也不读行数。

// blockAbove walks up column x from fromY past the ground and returns the top and
// bottom of the first flat block over it. The card's inner hairlines are a
// different colour from its face, so this reads the card's *box*; the same call
// on the page button's column reads the pill's box.
func blockAbove(tt *ui.Tester, x, fromY int, ground ui.Color) (top, bottom int) {
	blank := func(y int) bool { return pixelAt(tt, x, y) == ground }
	y := fromY
	for y >= 0 && blank(y) {
		y--
	}
	bottom = y
	for y >= 0 && !blank(y) {
		y--
	}
	top = y + 1
	return top, bottom
}

// sessionsFrame renders the Sessions page in its full state with n sessions and
// returns the frame plus the four boxes the footer's rhythm is read off.
func sessionsFrame(t *testing.T, n int) (tt *ui.Tester, hint, pill ui.Rect, cardTop, cardBottom, pillTop, pillBottom int) {
	t.Helper()
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{
		CloseSessionAt: func(int) {},
		SelectSession:  func(int) {},
		NewSession:     func() {},
	})
	a.Sessions = func() []Session {
		all := []Session{
			{ID: 1, Title: "zsh", Running: true, Active: true, Index: 0},
			{ID: 2, Title: "npm run dev", Running: true, Index: 1},
			{ID: 3, Title: "go test", Running: false, Index: 2},
		}
		return all[:n]
	}
	tt = render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	var ok bool
	if hint, ok = tt.Find(en.SessionsActive); !ok {
		t.Fatalf("the section hint is missing: %q", tt.Texts())
	}
	if pill, ok = tt.Find(en.SessionsNew); !ok {
		t.Fatalf("the page's way out is missing: %q", tt.Texts())
	}
	// cardColumn is inside the card, right of the leading chip and left of the
	// label column, so every pixel of it belongs to a plane and not to a glyph.
	const cardColumn = 238
	gapGround := pixelAt(tt, cardColumn, int(hint.Y)-3)
	cardTop, cardBottom = blockAbove(tt, cardColumn, int(hint.Y)-3, gapGround)
	// pillColumn is the page button's own left padding: 14u in from the pill's
	// edge, i.e. six pixels left of the label's box.
	pillColumn := int(pill.X) - 6
	below := pixelAt(tt, pillColumn, int(pill.Y)+30)
	pillTop, pillBottom = blockAbove(tt, pillColumn, int(pill.Y)+30, below)
	return tt, hint, pill, cardTop, cardBottom, pillTop, pillBottom
}

// The two gaps on the sessions page's full state: the card to its hint (the
// group's own gutter) and the hint to the page button (the page gap).
func TestTheSessionsPageFooterRidesItsContent(t *testing.T) {
	_, hint, _, cardTop, cardBottom, pillTop, pillBottom := sessionsFrame(t, 3)
	if got := int(hint.Y) - (cardBottom + 1); got != 8 {
		t.Errorf("the hint stands %dpx under the card, want the group's 8u gutter (card box %d..%d, hint at %.0f)",
			got, cardTop, cardBottom, hint.Y)
	}
	if got := pillTop - int(hint.Y+hint.H); got != 22 {
		t.Errorf("the page button stands %dpx under the hint, want the page column's 22u gap (hint box ends at %.0f, pill at %d)",
			got, hint.Y+hint.H, pillTop)
	}
	if h := pillBottom - pillTop + 1; h != 31 {
		t.Errorf("the page button's box is %dpx tall, want the 30u pill plus its two 1px edges (%d..%d)", h, pillTop, pillBottom)
	}
	// Three rows: the card is the rows plus their hairlines, i.e. it must grow
	// well past one row (a card that did not grow would hide the list).
	if cardBottom-cardTop < 100 {
		t.Errorf("three sessions drew a card only %dpx tall (%d..%d)", cardBottom-cardTop, cardTop, cardBottom)
	}
}

// The same two numbers with a single session: the footer rides the content, it
// is not a bar pinned to the viewport, and the rhythm does not read the row
// count.
func TestTheSessionsPageFooterDoesNotReadTheRowCount(t *testing.T) {
	_, hint1, _, top1, bottom1, pillTop1, _ := sessionsFrame(t, 1)
	_, hint3, _, top3, bottom3, pillTop3, _ := sessionsFrame(t, 3)
	if gap1, gap3 := int(hint1.Y)-(bottom1+1), int(hint3.Y)-(bottom3+1); gap1 != gap3 {
		t.Errorf("the card-to-hint gap follows the row count: %dpx for one session, %dpx for three", gap1, gap3)
	}
	if gap1, gap3 := pillTop1-int(hint1.Y+hint1.H), pillTop3-int(hint3.Y+hint3.H); gap1 != gap3 {
		t.Errorf("the hint-to-button gap follows the row count: %dpx for one session, %dpx for three", gap1, gap3)
	}
	if bottom3-top3 <= bottom1-top1 {
		t.Errorf("three sessions did not grow the card over one: %d vs %d", bottom3-top3, bottom1-top1)
	}
}
