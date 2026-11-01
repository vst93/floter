package launcher

import (
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-cg · 一份**全是便条**的列表的游标（`docs/rounds/P13-cg-spec.md`、
// `docs/liquid-glass.md` §3 的只读面）。
//
// 改前实测（720 帧、真启动器地面）：`kind:"status"` 的行不可运行，于是
// `initialOutputSelection`（当时的 `firstRunnable`）返回 **-1**——整页没有任何一行
// 被画成选中的，方向键转一圈原地不动，Enter 什么也不做，屏幕上没有一句话说为什么。
// 一份报告式插件（只有 `status` 行）因此读起来不是「这一页没有可运行的东西」，
// 而是「这一页没画完」。
//
// 改后：游标落在第一行，戴**只读游标**——`RowSelected` 的面（与门那一支同一个
// token：选中是一种状态，不是一种权限）**不带** `LitFace` 的受光边（§3：灯是
// 「这块面可以被碰」的记号，便条碰不动）。门被选中时仍然点亮，两条臂在同一个
// 函数里、只差一个 `row.Runnable()`。

// notesOnlyOutput builds a list of n rows that are all notes: the protocol's
// `kind:"status"` rows, which Enter cannot run.
func notesOnlyOutput(n int) string {
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":"n%d","title":"Note %d","kind":"status"}`, i, i)
	}
	b.WriteString("]")
	return b.String()
}

func TestAnAllNotesListStillCarriesAReadOnlyCursor(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			a := testApp()
			if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
				t.Fatal(err)
			}
			tt := runListOutput(t, a, notesOnlyOutput(3))
			// 深色那一臂必须让**画布**也变深：`ui.Tester` 的底默认是白的，
			// 而深色的窗口地面是半透明的——不切画布时读到的是「深色面合成在白纸上」，
			// 那不是用户看见的那一帧（本轮实测：地面 253 对 215）。
			tt.SetDark(dark)
			tt.Frame()
			tt.Frame()
			if !a.InOutputView() || !a.output.IsList {
				t.Fatalf("the capture did not open as the list page: %+v", a.output)
			}
			// The cursor exists at all. Before this round it was -1 and every
			// row on the page was drawn as if nothing were chosen.
			if a.output.Selected != 0 {
				t.Fatalf("a list of %d notes put its cursor at %d, want 0 (no row on the page is chosen)",
					len(a.output.Rows), a.output.Selected)
			}
			img := tt.Image()
			if _, ok := tt.Find("Note 0"); !ok {
				t.Fatalf("the note rows were not drawn: %q", tt.Texts())
			}
			tokens := theme.ForPreferences(a.settings(), dark, ui.Preferences{})
			// 两个行盒是上一帧的几何（与 `foldAlpha` 读的是同一份），比从文字盒
			// 反推行高可靠：行盒就是这一行的面。
			cursorRect, restRect := a.outputRowRects[0], a.outputRowRects[1]
			if cursorRect.H == 0 || restRect.H == 0 {
				t.Fatalf("no row geometry to read: %v / %v", cursorRect, restRect)
			}
			// `litedgeColumn` 在标题墨的右边、行尾槽的左边：一条空白剖面。
			t.Logf("ground %v cursor %v face %v rest %v face %v sel %d",
				img.RGBAAt(5, 5), cursorRect, img.RGBAAt(litedgeColumn, int(cursorRect.Y+cursorRect.H/2)),
				restRect, img.RGBAAt(litedgeColumn, int(restRect.Y+restRect.H/2)), a.output.Selected)
			face := img.RGBAAt(litedgeColumn, int(cursorRect.Y+cursorRect.H/2))
			rest := img.RGBAAt(litedgeColumn, int(restRect.Y+restRect.H/2))
			// ① The cursor's face is the chosen rung, composited on this row's
			// own ground — the same token the door's cursor wears.
			if want := over(tokens.RowSelected, rest); !closeRGB(face, want) {
				t.Errorf("the note cursor's face is %v, want RowSelected %v over the resting row %v",
					face, want, rest)
			}
			// ② …and it is a real step off the resting row, not a rounding error.
			if step := channelGap(face, rest); step < 8 {
				t.Errorf("the note cursor lifts its row %v by only %d levels — a cursor nobody can see "+
					"(cursor %v face %v, rest %v face %v, ground %v)", rest, step, cursorRect, face, restRect, rest, img.RGBAAt(5, 5))
			}
			// ③ 灯没点：行顶那几行在**空白剖面**上没有一行比它自己的面更亮。
			// 读一列而不是扫整行——行角是圆角，抗锯齿会把面色和卡面之间的中间值
			// 洒在角上（本轮实测：扫整行时 `crownOf` 把 241 当成受光边，而面是
			// 238、真正的受光边是 240；那一列在行中间，没有角）。
			// §3：受光边是「这块面可以被碰」的记号，而便条按定义碰不动。
			for dy := 0; dy < 4; dy++ {
				c := img.RGBAAt(litedgeColumn, int(cursorRect.Y)+dy)
				if channelGap(c, face) >= 2 && int(c.R) > int(face.R) {
					t.Errorf("the note cursor's top edge row %d is %v, brighter than its face %v: "+
						"a note is not a door (liquid-glass.md §3)", dy, c, face)
				}
			}
		})
	}
}

// TestTheNoteCursorRunsNothingAndCopiesNothing is the behaviour half: the cursor
// is a *position*, not a permission. Enter on a note still does nothing, the
// arrows still refuse to land on a note, and the copy still takes nothing —
// which is exactly what it took before the cursor existed.
func TestTheNoteCursorRunsNothingAndCopiesNothing(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, notesOnlyOutput(3))
	if !a.InOutputView() {
		t.Fatal("the capture did not open")
	}
	if got := a.outputText(); got != "" {
		t.Errorf("an all-notes list copies %q, want nothing", got)
	}
	tt.TypeKey(0, ui.KeyDown, "")
	tt.Frame()
	if a.output.Selected != 0 {
		t.Errorf("the down arrow moved the cursor to %d on a list where nothing can run", a.output.Selected)
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.InOutputView() {
		t.Error("Enter closed the page from a note row: the cursor is a position, not a permission")
	}
}

// TestTheMixedListKeepsTheCursorOnTheDoor pins that the new arm did not take
// over the ordinary case: a list with one runnable row still starts on it, not
// on the note above it.
func TestTheMixedListKeepsTheCursorOnTheDoor(t *testing.T) {
	a := testApp()
	stdout := `[{"id":"n0","title":"Note 0","kind":"status"},` +
		`{"id":"r0","title":"Row 0","action":{"type":"insert","text":"picked"}},` +
		`{"id":"n1","title":"Note 1","kind":"status"}]`
	runListOutput(t, a, stdout)
	if !a.InOutputView() || !a.output.IsList {
		t.Fatalf("the capture did not open as the list page: %+v", a.output)
	}
	if a.output.Selected != 1 {
		t.Fatalf("the cursor is at %d, want the only runnable row (1): %+v", a.output.Selected, a.output.Rows)
	}
	if got := a.outputText(); got != "picked" {
		t.Errorf("the copy from the chosen door is %q, want %q", got, "picked")
	}
}

// The opt-in dump: the same all-notes list before and after the round, both
// appearances. Run with FLOTER_P13CG_TAG=before while `initialOutputSelection`
// returns -1 again to get the comparison pair.
func TestDumpP13CGNoteCursor(t *testing.T) {
	if os.Getenv("FLOTER_P13CG_DUMP") == "" {
		t.Skip("set FLOTER_P13CG_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13CG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		tt := runListOutput(t, a, notesOnlyOutput(3))
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		name := fmt.Sprintf("/tmp/floter-P13-cg-notecursor-%s-%s.png", appearance, tag)
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", name)
	}
}
