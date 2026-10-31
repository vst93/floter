package settingsui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-az · 设置页 10 处提示戴上浮面（`docs/liquid-glass.md` §6.20 的 20-3）。
//
// 这 10 处此前都走框架的 `Element.Tooltip`（`ui/tooltip.go:96-117`）：圆角
// `Space(1.25)` = 5px（键帽那一档，不是浮面的 8）、`inverse()` 的**不透明反色**面
// （本仓没设 `ui.Theme.Inverse`，所以深色档退回 `t.Text` = 一枚近白片子）、一层
// `0 2px 8px 0.2`（既不是环境影也不是接触影）、**没有边**。20-1（终端页，P13-ax）与
// 20-2（启动器，P13-ay）已经换成 `theme.Tokens.Tip`；这一轮是最后一块。
//
// 像素 pin 读的是**面**，不是「大致变了」：设置页的真实地面
// （`realPageGround`）上渲染一个锚点，指针歇 700ms 后取提示矩形里出现次数最多的
// 那个颜色 —— 浮面必须**逐通道等于** `FaceOver(地面, Float)`，而反色片子按构造
// 是相反极性，一条都过不了。
const (
	tipAnchor = "Rest on me"
	tipWords  = "Pasted from the clipboard"
)

// tipProbe renders one anchor with a `tokens.Tip` above it on the settings page's
// own ground and returns the tester, the appearance's tokens, the anchor's box,
// the ground colour and the frame before the pointer arrives.
func tipProbe(t *testing.T, dark bool) (*ui.Tester, theme.Tokens, ui.Rect, ui.Color, *image.RGBA) {
	t.Helper()
	ground := realPageGround(t, dark)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	var box ui.Rect
	var ref ui.Handle
	var tok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tok = a.tokens(c)
		c.SetTheme(tok.Theme)
		c.Root().Background(ground)
		ui.Column(c).FillWidth().FillHeight().Padding(24).AlignItems(ui.Start).Children(func() {
			// 60px of nothing above the anchor: the tip attaches upward
			// (`AnchorTop`/`AnchorBottom`), and a tip with no room above would
			// flip below and land on the anchor.
			ui.Box(c).Size(1, 60)
			tok.Tip(c, ui.Text(c, tipAnchor).FontSize(tok.Theme.FontSize).TextColor(tok.Theme.Text), tipWords).Bind(&ref)
			box = c.Resolve(ref).Bounds()
		})
	}, 360, 200)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt, tok, box, ground, copyFrame(tt.Image())
}

// restPointer moves the pointer onto the anchor's centre and waits out the
// framework's 600ms rest, so the tip is in the frame.
func restPointer(tt *ui.Tester, box ui.Rect) {
	tt.Move(box.X+box.W/2, box.Y+box.H/2)
	tt.Frame()
	tt.Advance(700 * time.Millisecond)
	tt.Frame()
}

func copyFrame(img *image.RGBA) *image.RGBA {
	out := image.NewRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	return out
}

// changedAbove is the box of every pixel that moved by more than 8 levels in any
// channel, restricted to the rows above maxY — above the anchor only the tip can
// paint.
func changedAbove(before, after *image.RGBA, maxY int) (ui.Rect, bool) {
	x0, y0, x1, y1 := 1<<30, 1<<30, -1, -1
	b := after.Bounds()
	for y := b.Min.Y; y < maxY && y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p, q := imagePixel(before, x, y), imagePixel(after, x, y)
			d := diff8(p.R, q.R) + diff8(p.G, q.G) + diff8(p.B, q.B)
			if d <= 8 {
				continue
			}
			if x < x0 {
				x0 = x
			}
			if y < y0 {
				y0 = y
			}
			if x > x1 {
				x1 = x
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	if x1 < x0 || y1 < y0 {
		return ui.Rect{}, false
	}
	return ui.Rect{X: float32(x0), Y: float32(y0), W: float32(x1 - x0 + 1), H: float32(y1 - y0 + 1)}, true
}

func level(v uint8) int { return int(v) }

func diff8(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

// dominantFace is the colour that covers most of r — the face of a pane, since
// the label's glyphs and the two 1px lines are a minority of it — and the box
// those pixels occupy, which is the face rectangle without its crown row.
func dominantFace(img *image.RGBA, r ui.Rect) (ui.Color, ui.Rect, int) {
	counts := map[ui.Color]int{}
	var best ui.Color
	bestN := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			p := imagePixel(img, x, y)
			counts[p]++
			if counts[p] > bestN {
				best, bestN = p, counts[p]
			}
		}
	}
	x0, y0, x1, y1 := 1<<30, 1<<30, -1, -1
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if imagePixel(img, x, y) != best {
				continue
			}
			if x < x0 {
				x0 = x
			}
			if y < y0 {
				y0 = y
			}
			if x > x1 {
				x1 = x
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	return best, ui.Rect{X: float32(x0), Y: float32(y0), W: float32(x1 - x0 + 1), H: float32(y1 - y0 + 1)}, bestN
}

func rgbString(c ui.Color) string {
	return fmt.Sprintf("(%d,%d,%d a%d)", level(c.R), level(c.G), level(c.B), level(c.A))
}

// TestTheSettingsTipsAreFloatingPanes is the material pin: the face is the
// appearance's `Float` step composed over the settings page's own ground, and the
// crown row above it is the `EdgeHighlight` line — the same pair the drawer, the
// dialogs, the notes, the terminal page and the launcher wear.
func TestTheSettingsTipsAreFloatingPanes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		dark := dark
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tt, tok, box, ground, before := tipProbe(t, dark)
			restPointer(tt, box)
			after := tt.Image()
			tipBox, ok := changedAbove(before, after, int(box.Y))
			if !ok {
				t.Fatalf("resting the pointer on the anchor painted nothing above it — no tip")
			}
			face, faceBox, n := dominantFace(after, tipBox)
			if n < 200 {
				t.Fatalf("the biggest patch in the tip's box is only %d pixels (%s) — that is glyphs and lines, not a pane", n, rgbString(face))
			}
			want := theme.FaceOver(ground, tok.Float)
			if face != want {
				t.Errorf("the tip's face is %s; the pane on this ground is FaceOver(%s, Float) = %s "+
					"(the framework's chip is inverse() = Text = %s)",
					rgbString(face), rgbString(ground), rgbString(want), rgbString(tok.Theme.Text))
			}
			// The crown: the inset line sits on top of the face, so the row
			// immediately above the first face-coloured row is it.
			crown := imagePixel(after, int(faceBox.X+faceBox.W/2), int(faceBox.Y)-1)
			d := (litSum(crown) - litSum(face)) / 3
			t.Logf("%s: ground %s face %s crown %s (%+d levels) tip %v",
				name, rgbString(ground), rgbString(face), rgbString(crown), d, tipBox)
			// §3：受光边是这块面「可以浮起来」的记号。两个外观都读得出。实测这一行：
			// 浅 +4（250 → 255）、深 +46（20 → 67）；把 `EdgeHighlight` 那行拿掉（M3），
			// 同一行就退成 `InputStroke` 发丝：浅 −23（250 → 227）。两行是同一行，冠线压在上面。
			if d < 1 {
				t.Errorf("the %s crown is %+d levels against the face (%s → %s); a floating pane's "+
					"top edge is `EdgeHighlight` and has to be readable", name, d, rgbString(crown), rgbString(face))
			}
			if os.Getenv("FLOTER_P13AZ_DUMP") != "" {
				dumpTip(t, name, after)
			}
		})
	}
}

func dumpTip(t *testing.T, name string, img *image.RGBA) {
	t.Helper()
	path := filepath.Join(os.TempDir(), "floter-P13-az-settings-tip-"+name+".png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", path)
}

// TestATruncatedIntegrationNameStillReadsInFull is the behavioural half: site 4
// (the integration row's name) is a *text* anchor, not a button, and it is the
// one place the full package name can be read. The material changed; the tip
// still arrives after the 600ms rest and still carries the whole string.
func TestATruncatedIntegrationNameStillReadsInFull(t *testing.T) {
	const long = "com.vst93.floter.extensions.markdown-preview-themes"
	tt := integrationsPageWith(t, long, 460)
	if got := countOfText(tt, long); got != 1 {
		t.Fatalf("the name is drawn %d times before the pointer rests, want 1 (the row itself)", got)
	}
	name, ok := tt.Find(long)
	if !ok {
		t.Fatalf("the long integration name is not drawn: %q", tt.Texts())
	}
	tt.Move(name.X+name.W/2, name.Y+name.H/2)
	tt.Advance(700 * time.Millisecond)
	tt.Frame()
	if got := countOfText(tt, long); got != 2 {
		t.Errorf("resting the pointer on a truncated integration name drew it %d times, want 2 "+
			"(the row and its tip): %q", got, tt.Texts())
	}
}

// TestTheSettingsPageHasNoFrameworkTips is the census: `internal/settingsui` holds
// exactly ten tips and every one of them is a `tokens.Tip`. The list below is the
// ten anchors, so a tip that moves to a new file or a new call site is a red pin
// rather than a silent eleventh site that keeps the framework's chip.
var settingsTipCensus = map[string]int{
	"rows.go":       3, // sessionIconButton · sessionKillConfirm · rowTitle's truncated name
	"settingsui.go": 6, // integration name · Run's reason · homepage rowAction · repairAction ×2 · freshness dot
	"config.go":     1, // the required `*` marker
}

func TestTheSettingsPageHasNoFrameworkTips(t *testing.T) {
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	total, lines := 0, 0
	perFile := map[string]int{}
	for _, n := range names {
		src, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		body := string(src)
		if got := strings.Count(body, ".Tooltip("); got != 0 {
			t.Errorf("%s still rides the framework's chip in %d place(s): `Element.Tooltip` "+
				"draws a 5px corner, an opaque inverse fill and one 0 2px 8px 0.2 shadow "+
				"(§6.20 20-3)", n, got)
		}
		if got := strings.Count(body, "tokens.Tip(c, "); got > 0 {
			perFile[n] = got
			total += got
			lines += strings.Count(body, "tokens.Tip(c, ")
		}
	}
	for n, want := range settingsTipCensus {
		if got := perFile[n]; got != want {
			t.Errorf("%s carries %d `tokens.Tip` calls, want %d", n, got, want)
		}
	}
	if total != 10 {
		t.Errorf("the settings page carries %d tips, want 10 (the §6.20 20-3 list)", total)
	}
	if lines != total {
		t.Errorf("counted %d tip lines against %d calls — a call spans lines here", lines, total)
	}
}
