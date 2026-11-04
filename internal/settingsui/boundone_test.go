package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// 边界情况 · 第二格：列表里「恰好一件」与「一件都没有」。
//
// 满态那一族（§6.141 / §6.142 / §6.143）量的是「很多」：22 条会话、24 条命令、
// 24 张集成卡。它们全都靠**两个相邻元素之间**的那一段才成立——节奏、卡间面、
// 组的计数都在数「间隔」。N=1 时**一个间隔都没有**，N=0 时连组都不存在：
// 这两格此前没有任何 pin，而它们正是最容易长出一条「最后一个元素特例」的地方
// （最后一行不画分隔线 / 最后一张卡不画下边 / 空态头上还挂一个「已安装 0」）。
//
// 四条判据（`docs/liquid-glass.md` §6.144）：
//
//	144-1 N=1：全页只有这一张卡（`Integration-02` 不存在），组的计数 `1` 被画出、
//	      与分组标题同一条中心线、落在标题右边；
//	144-2 N=1：这张卡的**下沿仍是两行**（一条 1px 描边 + 它自己那一条影），
//	      再往下才是页面的面 —— 也就是说「它是最后一张」不影响它有没有下边；
//	144-3 N=2：唯一的那个间隔是 **8 行页面的面**，上沿被卡 1 的两行下沿夹住、
//	      下沿被卡 2 的两行上沿夹住（冠在里、描边在外），节奏 187.00；
//	144-4 N=0：这一页画的是空态，**不是**一个「Installed 0」的空组：分组标题与
//	      计数都不存在，而它上面那一行 PATH 工具仍在。
func TestTheOnlyRowOfAListStillHasItsEdges(t *testing.T) {
	copy := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			// 这一族三帧都在 720×900 里放得下（MaxY = 0）：量的是整页，
			// 不是滚到某一处的样子。
			for _, n := range []int{0, 1, 2} {
				if a := integrationFullApp(t, appearance, n); a.Body.MaxY != 0 {
					t.Fatalf("the %d-item fixture scrolls (MaxY = %v): this boundary is about a "+
						"list that fits, so a scrolling page would measure something else", n, a.Body.MaxY)
				}
			}

			// 144-4：N=0 是空态，不是一个空组。
			{
				a := integrationFullApp(t, appearance, 0)
				tt, _ := renderIn(t, a, 720, 900, dark)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				tt.Frame()
				if r, ok := tt.Find(copy.IntegrationsInstalled); ok {
					t.Errorf("the empty page still draws the group's heading %q at y=%.2f — a list with "+
						"nothing in it is an empty state, not an empty group", copy.IntegrationsInstalled, r.Y)
				}
				if _, ok := tt.Find(copy.IntegrationsEmpty); !ok {
					t.Errorf("the empty page draws no empty state: %q", tt.Texts())
				}
				if _, ok := tt.Find(copy.ShowCommandsInSearch); !ok {
					t.Errorf("the empty page lost the PATH-tools row above it: %q", tt.Texts())
				}
			}

			// 144-1 / 144-2：恰好一张卡。
			{
				a := integrationFullApp(t, appearance, 1)
				tt, tokens := renderIn(t, a, 720, 900, dark)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				tt.Frame()
				name, ok := tt.Find("Integration-01")
				if !ok {
					t.Fatalf("the only card is not drawn: %q", tt.Texts())
				}
				if _, ok := tt.Find("Integration-02"); ok {
					t.Errorf("a one-item list drew a second card: %q", tt.Texts())
				}
				heading, ok := tt.Find(copy.IntegrationsInstalled)
				if !ok {
					t.Fatalf("the group's heading is not drawn: %q", tt.Texts())
				}
				count, ok := tt.Find("1")
				if !ok {
					t.Fatalf("the group's count 1 is not drawn on a one-item list: %q", tt.Texts())
				}
				if count.W <= 0 || count.H <= 0 {
					t.Errorf("the count 1 is drawn %vx%v — the chip has no room for a single digit",
						count.W, count.H)
				}
				if got := (count.Y + count.H/2) - (heading.Y + heading.H/2); absFloat(got) > 1 {
					t.Errorf("the count 1 sits %.2fu off the heading's centre line", got)
				}
				if count.X < heading.X+heading.W {
					t.Errorf("the count starts at x=%.2f, left of the heading's right edge %.2f",
						count.X, heading.X+heading.W)
				}
				// 卡的下沿：两行 —— 描边 + 它自己的影。
				cardFace := pixelAt(tt, 400, int(name.Y)+2)
				pageFace := theme.FaceOver(tokens.Theme.Background, tokens.Sunken)
				if cardFace == pageFace {
					t.Fatalf("the card's face and the page's face are the same colour %v: the edge rows "+
						"below the card could not be counted", cardFace)
				}
				edge, gap := underCard(tt, 400, int(name.Y)+2, cardFace, pageFace, t)
				if edge != 2 {
					t.Errorf("the lone card's bottom edge is %d rows, want 2 (the 1px stroke and its own "+
						"shadow): a last card must not lose its bottom just because nothing follows it", edge)
				}
				if gap < 40 {
					t.Errorf("only %d rows of the page's face under the lone card, want at least 40 — "+
						"this fixture is supposed to end well above the page's bottom edge", gap)
				}
			}

			// 144-3：恰好两张卡 —— 唯一的那个间隔。
			{
				const pitch = 187.00 // §6.143 实测：卡高 179 + 卡间 `Space(2)` 8
				const gapRows = 8    // 卡与卡之间那一段页面的面（`Space(2)`）
				a := integrationFullApp(t, appearance, 2)
				tt, tokens := renderIn(t, a, 720, 900, dark)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				tt.Frame()
				first, ok := tt.Find("Integration-01")
				if !ok {
					t.Fatalf("the first card is not drawn: %q", tt.Texts())
				}
				second, ok := tt.Find("Integration-02")
				if !ok {
					t.Fatalf("the second card is not drawn: %q", tt.Texts())
				}
				if got := second.Y - first.Y; absFloat(got-pitch) > 0.01 {
					t.Errorf("the pitch from card 1 to 2 is %.2fu, want %.2fu", got, pitch)
				}
				cardFace := pixelAt(tt, 400, int(first.Y)+2)
				pageFace := theme.FaceOver(tokens.Theme.Background, tokens.Sunken)
				edge, gap := underCard(tt, 400, int(first.Y)+2, cardFace, pageFace, t)
				if edge != 2 {
					t.Errorf("card 1's bottom edge is %d rows, want 2", edge)
				}
				if gap != gapRows {
					t.Errorf("the only gap between two cards is %d rows of the page's face, want %d "+
						"(N=2 has N-1 = 1 gap, and it is `Space(2)`)", gap, gapRows)
				}
				// 卡 2 的上沿同样是两行，然后才是它自己的面（冠在里、描边在外）。
				top := int(first.Y) + 2 + edge + gap
				var outer, inner ui.Color
				outer, inner = pixelAt(tt, 400, top), pixelAt(tt, 400, top+1)
				if outer == pageFace || inner == pageFace {
					t.Fatalf("card 2's top edge at y=%d..%d is the page's own face (%v / %v): the two "+
						"cards are not separated by their own edges", top, top+1, outer, inner)
				}
				if got := pixelAt(tt, 400, top+2); got != cardFace {
					t.Errorf("card 2's face starts at y=%d in %v, want the card's own face %v — the top "+
						"edge is %d rows here, not 2", top+2, got, cardFace, 3)
				}
			}
		})
	}
}

// underCard walks down the column `x` from `from` and answers two numbers: how
// many rows the plane just above the first stretch of `pageFace` is *neither*
// the card's face nor the page's face (the card's own edge), and how long that
// first stretch of `pageFace` is. Text glyphs on the way down reset the edge
// count; only the run immediately above the page's face is the edge.
func underCard(tt *ui.Tester, x, from int, cardFace, pageFace ui.Color, t *testing.T) (edge, gap int) {
	t.Helper()
	for y := from; y < 899; y++ {
		switch p := pixelAt(tt, x, y); {
		case p == cardFace:
			edge = 0
			continue
		case p == pageFace:
			for y < 899 && pixelAt(tt, x, y) == pageFace {
				y++
				gap++
			}
			return edge, gap
		default:
			// Anti-aliased glyph rows sit here too; the next card-face row
			// resets the count, so only the edge under the card survives.
			edge++
		}
	}
	t.Fatalf("the column x=%d from y=%d never reached the page's face %v", x, from, pageFace)
	return 0, 0
}

// TestDumpP13FYBoundary is the opt-in frame dump: the three states of this
// boundary (nothing / one / two) in both appearances.
func TestDumpP13FYBoundary(t *testing.T) {
	if os.Getenv("FLOTER_DUMP_P13FY") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_DUMP_P13FY=1")
	}
	dir := "/tmp/p13fy"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, n := range []int{0, 1, 2} {
			a := integrationFullApp(t, appearance, n)
			tt, _ := renderIn(t, a, 720, 900, dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			name := fmt.Sprintf("floter-P13-fy-boundary-%02d-%s.png", n, appearance)
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
		}
	}
}
