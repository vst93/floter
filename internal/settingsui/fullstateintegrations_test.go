package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// 满态（full state）的第三格：集成页 24 张集成卡。
//
// 会话页那一格在 §6.141（P13-fv，行距 + 软边），命令板那一格在 §6.142
// （P13-fw，板的节奏 + 卡的缩进）。这一页此前只量过**一张卡**（
// `integrationrowgeometry_test.go`）与**一条命令列表**（`commandlist_test.go`），
// "很多张卡"这一态没有任何 pin —— 而它才是这一页最常见的样子（装了二十几个扩展
// 的机器）。此页的内容高度 ≈ 卡数 × 187，24 张就是 4500u 以上，比窗口高五倍。
//
// 四条判据（`docs/liquid-glass.md` §6.143）：
//
//	143-1 卡的节奏：24 张卡、23 个间隔，名字的 y 逐值相差 187.00（深浅同值）；
//	143-2 卡与卡之间是**页面的面**（`Space(2)` = 8 行），且这段面被两张卡各自的
//	      边夹住 —— 也就是说卡的下边与下一张卡的上边**不贴边**。这是本轮的判据：
//	      §6.142 修的是「板贴着卡」（一页里的一层），这里是「卡贴着卡」那一层，
//	      一旦两层各自画框贴在一起，整列 24 张卡就成了一条贯到底的双线；
//	143-3 满态的下沿：滚到底时最后一张卡的下边仍在（不是被下沿的软边吃掉），且它
//	      脚下那一整条带子**逐值等于页面的面**。这一页量出来的事实是：滚到 MaxY 时
//	      最后一张卡的下边离像素底还有 23 行空白 —— 下沿那道软边在这一页上**没有
//	      内容可淡**（§6.16「底部本来就是 6u」那句话在这一页上兑现了）。所以这条是
//	      边界，不是对软边配色的守卫：把软边的底色换成窗口地、乃至整块删掉，这一页
//	      的像素都不变（实测见本轮报告）。
//	143-4 组的计数：24 这个数字与它的分组标题同一条baseline，且没被切（`MinWidth`
//	      6.25u 是给两位数的，24 是这一页能到的最长两位数）。
func TestTheFullIntegrationListKeepsItsRhythmAndItsGaps(t *testing.T) {
	const cards = 24
	const pitch = 187.0 // 实测（u=4：卡高 179 + 卡间 `Space(2)` 8），深浅逐值相同
	const gap = 8       // 卡与卡之间那一段页面自己的面（`Space(2)`）
	copy := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			a := integrationFullApp(t, appearance, cards)
			tt, tokens := renderIn(t, a, 720, 900, dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			if a.Body.MaxY <= 0 {
				t.Fatalf("%d cards do not overflow this frame (MaxY = %v): there is no full state to "+
					"measure, so every judgement below would pass on a list that never scrolls",
					cards, a.Body.MaxY)
			}
			// The page's own face: the window ground with the body's `Sunken`
			// overlay on it — what a gap between two cards is painted in.
			face := theme.FaceOver(tokens.Theme.Background, tokens.Sunken)

			// 143-4 组的计数，在未滚动的这一帧上量（标题与数字都在框里）。
			heading, ok := tt.Find(copy.IntegrationsInstalled)
			if !ok {
				t.Fatalf("the installed group's heading is not drawn: %q", tt.Texts())
			}
			count, ok := tt.Find(fmt.Sprint(cards))
			if !ok {
				t.Fatalf("the group's count %d is not drawn: %q", cards, tt.Texts())
			}
			if count.W <= 0 || count.H <= 0 {
				t.Errorf("the count %d is drawn %vx%v — the chip has no room for it", cards, count.W, count.H)
			}
			if got := (count.Y + count.H/2) - (heading.Y + heading.H/2); absFloat(got) > 1 {
				t.Errorf("the count %d sits %.2fu off the heading's centre line — it is not on the group's "+
					"own line", cards, got)
			}
			if count.X < heading.X+heading.W {
				t.Errorf("the count starts at x=%.2f, left of the heading's right edge %.2f — the chip is "+
					"not trailing the title", count.X, heading.X+heading.W)
			}

			// 143-1 卡的节奏。
			tops := make([]float32, 0, cards)
			for i := 0; i < cards; i++ {
				r, ok := tt.Find(fmt.Sprintf("Integration-%02d", i+1))
				if !ok {
					t.Fatalf("integration %d is missing from the list: %q", i+1, tt.Texts())
				}
				tops = append(tops, r.Y)
			}
			for i := 1; i < len(tops); i++ {
				if got := tops[i] - tops[i-1]; absFloat(got-pitch) > 0.01 {
					t.Errorf("the pitch from card %d to %d is %.2fu, want %.2fu (the card's own height "+
						"plus the section's `Space(2)` gap)", i, i+1, got, pitch)
				}
			}

			// 143-2 卡与卡之间那一段面，以及夹住它的两条边。量在卡的中轴上：
			// 卡的圆角在 x=190 把最后几行削掉，于是那一段读成 8 + 4 行；中轴上
			// 唯一的「页面的面」就是两张卡之间的空隙（卡里没有一处是页面的面）。
			// 只量整张下一张卡都在框里的那几对。
			view := tt.Image().Bounds().Dy()
			for i := 0; i+1 < len(tops); i++ {
				if int(tops[i+1]) >= view {
					break
				}
				start, length := faceGapBetween(tt, 400, int(tops[i]), int(tops[i+1]), face)
				if length != gap {
					t.Errorf("between card %d and %d the page's face runs %d rows (from y=%d), want %d "+
						"(`Space(2)`) — the two cards' own edges are %s", i+1, i+2, length, start, gap,
						map[bool]string{true: "touching", false: "not the only thing between them"}[length == 0])
					continue
				}
				if above, below := pixelAt(tt, 400, start-1), pixelAt(tt, 400, start+length); above == face || below == face {
					t.Errorf("the gap between card %d and %d is not bracketed by both cards' edges "+
						"(above %v, below %v, face %v)", i+1, i+2, above, below, face)
				}
			}

			// 143-3 滚到底：下沿还在，带子不显形。
			a.Body.Y = a.Body.MaxY
			tt.Frame()
			tt.Frame()
			lastBottom := -1
			for _, x := range []int{190, 400, 690} {
				for y := view - 60; y < view; y++ {
					if y > lastBottom && pixelAt(tt, x, y) != face {
						lastBottom = y
					}
				}
			}
			if lastBottom < 0 {
				t.Fatalf("at the bottom of a %d-card list there is no card edge in the last 40 rows — "+
					"the list ends in nothing (MaxY %.2f)", cards, a.Body.MaxY)
			}
			for _, x := range []int{190, 400, 690} {
				for y := lastBottom + 1; y < view; y++ {
					if got := pixelAt(tt, x, y); got != face {
						t.Errorf("at the bottom of the list, x=%d y=%d is %v, want the page's face %v — the "+
							"soft edge is painting something that is not under it", x, y, got, face)
						break
					}
				}
			}
		})
	}
}

// faceGapBetween is the longest run of rows painted `face` strictly between two
// name lines, measured down one column of the cards' left inset (where nothing
// but the card's own plane is drawn). Its start row and length are returned so a
// caller can ask what bracketed it.
func faceGapBetween(tt *ui.Tester, x, from, to int, face ui.Color) (start, length int) {
	start, length = -1, 0
	for y := from; y < to; {
		if pixelAt(tt, x, y) != face {
			y++
			continue
		}
		s := y
		for y < to && pixelAt(tt, x, y) == face {
			y++
		}
		if y-s > length {
			start, length = s, y-s
		}
	}
	return start, length
}

// integrationFullApp is the fixture: n installed integrations, each with the
// three lines a real card carries (name + version, description, the identity
// line) and nothing else — the smallest card that still is a card.
func integrationFullApp(t *testing.T, appearance string, n int) *App {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.Integrations = func() []Integration {
		list := make([]Integration, 0, n)
		for i := 0; i < n; i++ {
			list = append(list, Integration{
				ID: fmt.Sprintf("ext-%02d", i+1), Name: fmt.Sprintf("Integration-%02d", i+1),
				Description: "does a thing", Version: "1.2.3", Enabled: true, Running: true,
				DistributionSource: "local", RuntimeOwnership: "system",
			})
		}
		return list
	}
	a.selectPage(PageIntegrations)
	return a
}

// TestDumpP13FXIntegrationFull is the opt-in frame dump: light and dark, the top
// of the list and its very bottom.
func TestDumpP13FXIntegrationFull(t *testing.T) {
	if os.Getenv("FLOTER_DUMP_P13FX") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_DUMP_P13FX=1")
	}
	dir := "/tmp/p13fx"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := integrationFullApp(t, appearance, 24)
		tt, _ := renderIn(t, a, 720, 900, dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Frame()
		for frac, tag := range map[float32]string{0: "top", 1: "bottom"} {
			a.Body.Y = a.Body.MaxY * frac
			tt.Frame()
			tt.Frame()
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("floter-P13-fx-integrations-%s-%s.png", appearance, tag)))
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
