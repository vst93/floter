package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/clipboard"
	"floter/internal/settings"
)

// P13-fd · 空态那句话的居中带，比同一块面板的列表少算了一条底沿。
//
// 同一张卡里有两个页面可以什么都没有（§6.47）：结果区与插件模式的空态。列表页的
// 下内边距是 `Space(0.5)+statusBand(c)`（面板自己的 tail + 停靠带，见 `launcher.go`
// 里列表那条 `Padding`），而安静页那一列此前是 `Space(3)` —— 同一个底沿两个数。
// 后果两条，都量得出来（真窗口 720×charged，en/zh × 深/浅逐值相同）：
//
//   - 那句话比它所在的带高 7~10px 居中：打字无结果 18.7 / 25.9（上/下），
//     剪贴板空态 15.1 / 25.9。
//   - 按住 ⌘ 让停靠带出现时，列表一行不动，这句话却往下跳 **13.5px**
//     （`Y=69.1` → `82.6`，charged 113 → 140）。
//
// 修法是让两页读同一个表达式；下面两条 pin 量的就是这两条后果，**不是**实现细节：
// 一条量居中（三臂的上下空气），一条量「带子来了它也不动」。

// emptyDensityArm is one page of the quiet family: the sentence it draws, the top
// edge of the band it belongs to (the field's floor, or the chips row's bottom),
// and whether the docked band is showing.
type emptyDensityArm struct {
	name  string
	text  func(a *App) string
	build func(t *testing.T) *App
	band  bool
	// bandTopUnits is the top edge of the band the sentence centres in — the
	// column's own top padding (`listEdge`): the field row plus the panel's
	// breath (56u + 4u), plus the chips band (20u) when one is drawn. The
	// sentence is 23.2px below the field's *visible* floor on the no-chips
	// page, because that 4u breath is the panel's own top inset.
	bandTopUnits float64
}

func emptyDensityArms() []emptyDensityArm {
	miss := emptyDensityArm{
		name: "miss", bandTopUnits: fieldRowUnits + breathUnits,
		text: func(a *App) string { return a.copy().NoResults },
		build: func(t *testing.T) *App {
			a := testApp()
			a.Query = "zzzz"
			return a
		},
	}
	held := miss
	held.name, held.band = "miss+band", true
	held.build = func(t *testing.T) *App {
		a := testApp()
		a.Query = "zzzz"
		return a
	}
	chips := emptyDensityArm{
		name: "clipboard-empty", bandTopUnits: fieldRowUnits + breathUnits + 5,
		text: func(a *App) string { return a.copy().ClipboardEmpty },
		build: func(t *testing.T) *App {
			a := testApp()
			a.Clipboard = clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), clipboard.MaxMaxItems)
			a.EnterClipboard()
			return a
		},
	}
	return []emptyDensityArm{miss, held, chips}
}

// emptyDensityFrame draws one arm at the window the app charges for it and
// returns the tester, the charged height and the sentence's box.
func emptyDensityFrame(t *testing.T, arm emptyDensityArm, dark bool, lang string) (*ui.Tester, int, ui.Rect) {
	t.Helper()
	charged := 0
	first := arm.build(t)
	if lang != "" {
		if err := first.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
			t.Fatal(err)
		}
	}
	first.Actions.ResizeTo = func(h int) { charged = h }
	room := ui.NewTester(themedView(first), InputWindowWidth, 600)
	if dark {
		room.SetDark(true)
	}
	if arm.band {
		room.HoldModifiers(ui.Cmd)
	}
	room.Frame()
	if charged <= 0 {
		t.Fatalf("%s: the app charged no window height", arm.name)
	}

	a := arm.build(t)
	if lang != "" {
		if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
			t.Fatal(err)
		}
	}
	tt := ui.NewTester(themedView(a), InputWindowWidth, charged)
	if dark {
		tt.SetDark(true)
		tt.Advance(500 * time.Millisecond)
	}
	if arm.band {
		tt.HoldModifiers(ui.Cmd)
	}
	tt.Frame()
	tt.Frame()
	box, ok := tt.Find(arm.text(a))
	if !ok {
		t.Fatalf("%s: the empty sentence is missing: %q", arm.name, tt.Texts())
	}
	return tt, charged, box
}

// TestTheEmptySentencesBandEndsAtThePanelsOwnTail pins the sentence's air, in
// pixels: it sits between the thing above it and the card's own bottom inset,
// and the two sides are within 2px of each other.
//
// The numbers are the shipped ruler's (`designUnit()` 0.9): the sentence's box
// is 18 tall, the band starts at the field's floor (50.4) or the chips' bottom
// (72), and the card's bottom inset is the panel's tail (1.8) — plus the docked
// band (27) when it shows.
func TestTheEmptySentencesBandEndsAtThePanelsOwnTail(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, arm := range emptyDensityArms() {
			for _, lang := range []string{"", "zh"} {
				_, charged, box := emptyDensityFrame(t, arm, dark, lang)
				appearance := "light"
				if dark {
					appearance = "dark"
				}
				top := float32(arm.bandTopUnits) * 4 * float32(designUnit())
				band := float32(0)
				if arm.band {
					band = float32(launcherStatusUnits) * float32(designUnit()*4)
				}
				inset := float32(TailUnits)*4*float32(designUnit()) + band
				above := box.Y - top
				below := float32(charged) - inset - (box.Y + box.H)
				if above < 18 || above > 21 {
					t.Errorf("%s/%s/%s: the sentence sits %.1fpx below the band's top edge, "+
						"not the 19.6px of a centred statement (box %+v, band top %.1f)",
						arm.name, appearance, lang, above, box, top)
				}
				if below < 18 || below > 21 {
					t.Errorf("%s/%s/%s: the sentence ends %.1fpx above the card's bottom inset, "+
						"not the 19.6px of a centred statement — the band's bottom is not the "+
						"panel's own tail (box %+v, charged %d, inset %.1f)",
						arm.name, appearance, lang, below, box, charged, inset)
				}
				if d := above - below; d > 1.5 || d < -1.5 {
					t.Errorf("%s/%s/%s: %.1fpx above and %.1fpx below — the statement is not "+
						"centred in the band it belongs to (§6.125)", arm.name, appearance, lang, above, below)
				}
			}
		}
	}
}

// TestTheEmptySentenceDoesNotMoveWhenTheDockedBandArrives is the second half of
// the same defect, and the one a user can see: holding ⌘ on a page with no
// results used to slide the sentence 13.5px down while every row of the list
// stayed put (the list reserves the band out of its content; the quiet column
// did not).
func TestTheEmptySentenceDoesNotMoveWhenTheDockedBandArrives(t *testing.T) {
	arms := emptyDensityArms()
	quiet, docked := arms[0], arms[1]
	for _, dark := range []bool{false, true} {
		for _, lang := range []string{"", "zh"} {
			_, chargedQuiet, quietBox := emptyDensityFrame(t, quiet, dark, lang)
			_, chargedDocked, dockedBox := emptyDensityFrame(t, docked, dark, lang)
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			if chargedDocked <= chargedQuiet {
				t.Fatalf("%s/%s: the docked band charged no height (%d vs %d) — "+
					"the fixture is not showing the band", appearance, lang, chargedDocked, chargedQuiet)
			}
			if got, want := tenths(float64(dockedBox.Y)), tenths(float64(quietBox.Y)); got != want {
				t.Errorf("%s/%s: the sentence moves %.1fpx when the docked band arrives "+
					"(y=%.2f without it, %.2f with it): the quiet column must reserve the band "+
					"the way the list does (§6.125)",
					appearance, lang, dockedBox.Y-quietBox.Y, quietBox.Y, dockedBox.Y)
			}
		}
	}
}

// TestDumpP13FDEmptyDensityPNGs writes the round's before/after evidence: the
// three arms at the window each of them charges, both appearances.
func TestDumpP13FDEmptyDensityPNGs(t *testing.T) {
	dir := os.Getenv("FLOTER_P13FD_DIR")
	if dir == "" {
		t.Skip("set FLOTER_P13FD_DIR to write the round's visual evidence")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, arm := range emptyDensityArms() {
			tt, charged, box := emptyDensityFrame(t, arm, dark, "")
			path := filepath.Join(dir, "floter-P13-fd-"+arm.name+"-"+appearance+".png")
			f, err := os.Create(path) // #nosec G304 -- the caller's own directory
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			t.Logf("%s charged=%d box=%+v", path, charged, box)
		}
	}
}
