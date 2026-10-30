package shell

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-y · 插件窗口 bar 上的三个键（`docs/rounds/P13-y-spec.md`）。
//
// 输出窗口（`detach.go` `detachedBar`）与便条窗口（`pinned.go` `pinnedBody`）画的是
// **同一条** `.plugin-window__bar`：同一个 8u 圆角、同一条 `Hairline` 底线、同一对
// padding 常量。它的键此前停在两条契约之间：
//
//   - 悬停是一块**平涂**（`Background(tokens.ControlHover)`），没有 1px 边、没有受光边——
//     §2 的 E1 是「面 + 一条受光边 + 对侧暗线」，而 `launcher.rowControl`（P13-h）、
//     `settingsui.headerButton`（P13-i）、`terminalui.barButton`（P13-d）自那几轮起
//     都是 E1。同一个应用里同尺寸的悬停小格不许有两种读法。
//   - 禁用是 `Disabled(true)`（框架 0.5，`ui/paint.go:140` / `ui/inline.go:109-113`）
//     **再乘** `Opacity(0.5)` = 有效 **0.25**。§1「禁用的材质」：**透明面**不叠材质，
//     只剩框架那一道 0.5（P13-s/t/u 在设置页收口的同一条算术）。
//
// 这些 pin 量的是**键自己的那一列**：冠线 / 面 / 对侧三行，与 P13-p 的 `liteMeasure`
// 同一个读法——候选线不是写死的像素，而是**当场用同一个面 + 同一条 token** 画出来再比，
// pin 里没有一处写死渲染器算出来的数值。

// barApp is the shell with one appearance set.
func barApp(t *testing.T, dark bool) *App {
	t.Helper()
	a := New(Options{Store: settings.NewStore(settings.Default())})
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	return a
}

// barDetached renders a detached output window. running puts the re-run key in
// its disabled arm (the run is in flight, `DetachedPluginApp.tsx:293-306`).
func barDetached(t *testing.T, dark, running bool) (*ui.Tester, *App) {
	t.Helper()
	a := barApp(t, dark)
	detached := &detachedOutput{
		title:   "jv --floter",
		running: running,
		entry: &extensions.CommandEntry{
			IntegrationID: "local.tool",
			Command:       extensions.Command{ID: "jv", Name: "JSON Viewer"},
		},
		view: launcher.OutputView{Title: "jv --floter", Text: "{\"a\": 1}"},
	}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.SetDark(dark)
	tt.Frame()
	return tt, a
}

// barPinned renders a pinned note: the same bar, with only its Close key.
func barPinned(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	a := barApp(t, dark)
	state := &pinned{title: "note", text: "pinned text"}
	tt := ui.NewTester(func(c *ui.Context) { a.pinnedView(c, state) }, 720, 460)
	tt.SetDark(dark)
	tt.Frame()
	return tt
}

// barKeyBox is the key's own box. `Find` answers with the element that carries
// the accessible name — the key itself — not the word drawn inside it: the
// re-run key measures 45u wide at y=9..31 in this frame, and the word's strokes
// start nine DIPs inside it.
func barKeyBox(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	box, ok := tt.Find(label)
	if !ok {
		t.Fatalf("the bar key %q is missing: %q", label, tt.Texts())
	}
	return box
}

// barReading is one key's three rows: the lit edge, the face, and the
// counter-side line, plus the bar's own face above the key (the proof the
// pointer landed on the key rather than beside it).
type barReading struct {
	crown, face, shade, barFace ui.Color
}

// barTryRead hovers (x, y) and reads the column: the key's own face is the mode
// of the window, and the rows immediately above and below that run are the two
// lines an E1 face draws. It reports false when this column is not inside a
// key's face (the mode runs off the window), which is how a column is found
// without assuming any geometry: the run *is* the key.
func barTryRead(tt *ui.Tester, x, y int) (barReading, bool) {
	tt.Move(float32(x), float32(y))
	tt.Frame()
	top, bottom := y-20, y+20
	img := tt.Image()
	counts := map[ui.Color]int{}
	var face ui.Color
	best := 0
	for yy := top; yy <= bottom; yy++ {
		c := shellPixelAt(img, x, yy)
		counts[c]++
		if counts[c] > best {
			best, face = counts[c], c
		}
	}
	start, end := -1, -1
	for yy := top; yy <= bottom; yy++ {
		if shellPixelAt(img, x, yy) == face {
			if start < 0 {
				start = yy
			}
			end = yy
		}
	}
	if start <= top || end >= bottom {
		return barReading{}, false
	}
	return barReading{
		crown:   shellPixelAt(img, x, start-1),
		face:    face,
		shade:   shellPixelAt(img, x, end+1),
		barFace: shellPixelAt(img, x, top),
	}, true
}

// barKeyColumn walks left from the bar's right edge and returns the first
// column that reads as a key: an icon key's label rect is the glyph's, not the
// key's, so the box is read off the frame instead of assumed from a padding.
func barKeyColumn(t *testing.T, tt *ui.Tester, y, right int) barReading {
	t.Helper()
	for x := right; x > right-240; x-- {
		if got, ok := barTryRead(tt, x, y); ok {
			return got
		}
	}
	t.Fatalf("no key's face runs down any column between x=%d and x=%d at y=%d", right, right-240, y)
	return barReading{}
}

// barAssertE1 is the one statement this round makes about a hovered key.
func barAssertE1(t *testing.T, appearance, name string, tokens theme.Tokens, dark bool, got barReading) {
	t.Helper()
	// The pointer has to have landed on the key: if it did not, the key's face
	// is the bar's own and everything below compares nothing.
	if shellColorDistance(got.face, got.barFace) == 0 {
		t.Fatalf("%s/%s: the key's face %v is the bar's face — the pointer never landed on it",
			appearance, name, got.face)
	}
	t.Logf("%s/%s: crown %v face %v shade %v", appearance, name, got.crown, got.face, got.shade)
	if barLuma(got.crown) <= barLuma(got.face) {
		t.Errorf("%s/%s: the hovered key has no lit edge — crown %v is not brighter than face %v",
			appearance, name, got.crown, got.face)
	}
	if barLuma(got.shade) >= barLuma(got.face) {
		t.Errorf("%s/%s: the hovered key has no counter-side line — bottom %v is not darker than face %v",
			appearance, name, got.shade, got.face)
	}
	rim := barLineOver(t, tokens, dark, got.face, tokens.LensRim)
	highlight := barLineOver(t, tokens, dark, got.face, tokens.EdgeHighlight)
	if barChannelDelta(rim, highlight) < 3 {
		t.Fatalf("%s/%s: the two candidate lines composite to %v and %v — this pin cannot tell them apart",
			appearance, name, rim, highlight)
	}
	if shellColorDistance(got.crown, rim) >= shellColorDistance(got.crown, highlight) {
		t.Errorf("%s/%s: the lit edge %v is the float rung (nearer %v than %v) — a key on a bar is E1, not E2",
			appearance, name, got.crown, highlight, rim)
	}
}

// barLineOver draws one 1px inner line over face and returns the pixel it makes:
// the candidate the measured crown is compared against. Nothing here is a
// renderer constant — the line is the token, composited by the same painter.
func barLineOver(t *testing.T, tokens theme.Tokens, dark bool, face, line ui.Color) ui.Color {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(face)
		ui.Column(c).Padding(20).Children(func() {
			ui.Row(c).FillWidth().Height(40).Background(face).
				InsetShadow(0, 1, 0, 0, line)
		})
	}, 300, 120)
	tt.SetDark(dark)
	tt.Frame()
	return shellPixelAt(tt.Image(), 150, 20)
}

// barLuma is a colour's brightness: what "lit" and "shaded" mean here.
func barLuma(c ui.Color) int {
	return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
}

// barChannelDelta is how far apart two colours are, in levels of their widest
// channel: the guard that says this pin can tell two candidate lines apart.
func barChannelDelta(a, b ui.Color) int {
	d := func(x, y uint8) int {
		if x > y {
			return int(x) - int(y)
		}
		return int(y) - int(x)
	}
	return max(d(a.R, b.R), max(d(a.G, b.G), d(a.B, b.B)))
}

// TestThePluginWindowBarKeysAreE1Faces: a hovered bar key is an E1 face — one
// lit edge on top, one counter-side line at the bottom, and the lit edge is the
// component rung (`LensRim`), not the float's (`EdgeHighlight`).
func TestThePluginWindowBarKeysAreE1Faces(t *testing.T) {
	copy := i18n.For("en").Launcher
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tokens := launcher.Resolve(settings.Default(), dark, ui.Preferences{}).Tokens

		// The re-run key is a word: six DIPs inside its left edge is its own
		// padding, clear of the strokes, and the key's box gives the height both
		// windows' keys sit at.
		detached, _ := barDetached(t, dark, false)
		word := barKeyBox(t, detached, copy.OutputWindowRerun)
		cy := int(word.Y + word.H/2)
		got, ok := barTryRead(detached, int(word.X)+6, cy)
		if !ok {
			t.Fatalf("%s: the re-run key's column does not read as a face", appearance)
		}
		barAssertE1(t, appearance, "detached re-run", tokens, dark, got)

		// Both Close keys are icon keys: probe the bar's right edge, at the same
		// height the re-run key sits at (both windows draw the same bar).
		for _, key := range []struct {
			name string
			tt   *ui.Tester
		}{
			{"detached close", detached},
			{"pinned close", barPinned(t, dark)},
		} {
			barAssertE1(t, appearance, key.name, tokens, dark, barKeyColumn(t, key.tt, cy, 700))
		}
	}
}

// TestTheDisabledBarKeyDimsOnce: the re-run key's disabled arm is the
// framework's one 0.5, not a second fade multiplied into it (0.25).
//
// `shellColorDistance` is a squared distance, so a half-alpha ink reads as a
// quarter of the contrast: the ratio is compared as its square root, which is
// the ink's own alpha.
func TestTheDisabledBarKeyDimsOnce(t *testing.T) {
	copy := i18n.For("en").Launcher
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		contrast := func(running bool) int {
			tt, _ := barDetached(t, dark, running)
			box := barKeyBox(t, tt, copy.OutputWindowRerun)
			ink := shellInkColor(tt, box)
			face := barRowMode(tt.Image(), box, int(box.Y)+2)
			return shellColorDistance(ink, face)
		}
		idle, run := contrast(false), contrast(true)
		if idle == 0 {
			t.Fatalf("%s: the resting re-run key drew no ink", appearance)
		}
		ratio := math.Sqrt(float64(run) / float64(idle))
		t.Logf("%s: idle %d running %d ratio %.2f", appearance, idle, run, ratio)
		if ratio < 0.40 || ratio > 0.60 {
			t.Errorf("%s: the disabled re-run key's ink is %.2f of the live one — one fade is 0.50 "+
				"(the framework's), two multiplied are 0.25 (idle contrast %d, running %d)",
				appearance, ratio, idle, run)
		}
	}
}

// barRowMode is the colour that dominates one row of a box, read across its
// middle band (the corners are rounded, so the band misses them).
func barRowMode(img *image.RGBA, box ui.Rect, y int) ui.Color {
	counts := map[ui.Color]int{}
	var mode ui.Color
	best := 0
	x0 := int(box.X + box.W*0.3)
	x1 := int(box.X + box.W*0.7)
	for x := x0; x < x1; x++ {
		c := shellPixelAt(img, x, y)
		counts[c]++
		if counts[c] > best {
			best, mode = counts[c], c
		}
	}
	return mode
}

// P13-y visual evidence: the bar at rest and hovered, both appearances, in /tmp.
// Opt-in (`FLOTER_PNG=1`), as the other dumps.
func TestDumpP13YBarKeysPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	copy := i18n.For("en").Launcher
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, _ := barDetached(t, dark, false)
		write := func(tag string) {
			path := filepath.Join("/tmp", "floter-P13-y-bar-"+appearance+"-"+tag+".png")
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			t.Logf("%s", path)
		}
		write("rest")
		word := barKeyBox(t, tt, copy.OutputWindowRerun)
		tt.Move(word.X+word.W/2, word.Y+word.H/2)
		tt.Frame()
		write("hover")
	}
}
