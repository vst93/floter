package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
)

// The detached window's bar is the reference's two keys
// (`DetachedPluginApp.tsx:284-315`): the re-run key is a *word*
// (`pluginWindow.rerun`) and the close key is an *icon* key
// (`<X size={16} strokeWidth={1.8}>`), the word `pluginWindow.close` surviving
// only as the key's title/aria-label. A rewrite that drew both as text would
// read as two identical word keys; this pins the difference.
func TestTheCloseKeyIsTheReferenceIconKey(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	copy := i18n.For("en").Launcher
	detached := &detachedOutput{
		title: "jv --floter",
		entry: &extensions.CommandEntry{IntegrationID: "local.tool", Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}},
		view:  launcher.OutputView{Title: "jv --floter", Text: "{\"a\": 1}"},
	}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()

	closeRect, ok := tt.Find(copy.OutputWindowClose)
	if !ok {
		t.Fatalf("the close key is missing its accessible name: %q", tt.Texts())
	}
	rerunRect, ok := tt.Find(copy.OutputWindowRerun)
	if !ok {
		t.Fatalf("the re-run key is missing: %q", tt.Texts())
	}

	// The close key's accessible name is present, but its face draws the X:
	// the ink is roughly square, where the re-run *word* is wide and flat.
	closeRatio := shellInkRatio(t, tt, closeRect)
	rerunRatio := shellInkRatio(t, tt, rerunRect)
	if closeRatio >= 1.7 {
		t.Errorf("the close key draws a word, not the reference's X (ink ratio %.2f)", closeRatio)
	}
	if rerunRatio <= closeRatio {
		t.Errorf("the re-run key is not the wider text key (close %.2f, rerun %.2f)", closeRatio, rerunRatio)
	}
}

// The re-run key is `disabled` for the whole run
// (`DetachedPluginApp.tsx:293-306`, `.plugin-window__bar-button:disabled {
// opacity: 0.5 }`). The old build dimmed the key's own face; a rewrite that
// only guarded the click left the key looking live.
func TestTheRerunKeyDimsWhileARunIsInFlight(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	copy := i18n.For("en").Launcher
	detached := &detachedOutput{
		title: "jv --floter",
		entry: &extensions.CommandEntry{IntegrationID: "local.tool", Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}},
		view:  launcher.OutputView{Title: "jv --floter", Text: "{\"a\": 1}"},
	}
	ink := func(running bool) (ui.Color, ui.Color) {
		detached.running = running
		tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
		tt.Frame()
		r, ok := tt.Find(copy.OutputWindowRerun)
		if !ok {
			t.Fatalf("the re-run key is missing: %q", tt.Texts())
		}
		bg, _ := shellModalColor(tt, r)
		return shellInkColor(tt, r), bg
	}
	idleInk, idleBG := ink(false)
	runInk, _ := ink(true)
	if shellColorDistance(runInk, idleBG) >= shellColorDistance(idleInk, idleBG) {
		t.Errorf("the re-run key did not dim while running: idle %v, running %v",
			idleInk, runInk)
	}
}

// The close key still closes: the nil window of a test still forgets the
// entry, and the key is reachable by its accessible name (the word is not lost
// just because the face is an icon).
func TestTheCloseKeyClosesTheWindow(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	detached := &detachedOutput{
		key:   "k",
		title: "jv --floter",
		entry: &extensions.CommandEntry{IntegrationID: "local.tool", Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}},
		view:  launcher.OutputView{Title: "jv --floter", Text: "{\"a\": 1}"},
	}
	a.detached = append(a.detached, detached)
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	if err := tt.Click(i18n.For("en").Launcher.OutputWindowClose); err != nil {
		t.Fatal(err)
	}
	if len(a.detached) != 0 {
		t.Errorf("the close key left the window behind: %d entries", len(a.detached))
	}
}

// shellInkRatio is the aspect ratio of the ink inside a rect: a word is wide
// and flat, an X is square.
func shellInkRatio(t *testing.T, tt *ui.Tester, r ui.Rect) float64 {
	t.Helper()
	minX, minY := int(r.X)+int(r.W), int(r.Y)+int(r.H)
	maxX, maxY := int(r.X), int(r.Y)
	img := tt.Image()
	bg, _ := shellModalColor(tt, r)
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := y*img.Stride + x*4
			c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			if shellColorDistance(c, bg) < 900 {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < minX || maxY < minY {
		t.Fatalf("no ink in %+v", r)
	}
	return float64(maxX-minX+1) / float64(maxY-minY+1)
}

// shellInkColor is the colour a text rect is actually drawn in: the pixel of
// the rect farthest from its background.
func shellInkColor(tt *ui.Tester, r ui.Rect) ui.Color {
	img := tt.Image()
	background, _ := shellModalColor(tt, r)
	var ink ui.Color
	best := -1
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := y*img.Stride + x*4
			c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			if d := shellColorDistance(c, background); d > best {
				best = d
				ink = c
			}
		}
	}
	return ink
}

func shellModalColor(tt *ui.Tester, r ui.Rect) (ui.Color, int) {
	img := tt.Image()
	counts := map[ui.Color]int{}
	var mode ui.Color
	best := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := y*img.Stride + x*4
			c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			counts[c]++
			if counts[c] > best {
				best = counts[c]
				mode = c
			}
		}
	}
	return mode, best
}

func shellColorDistance(a, b ui.Color) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	return dr*dr + dg*dg + db*db
}
