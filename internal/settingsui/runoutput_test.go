package settingsui

import (
	"image"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// R9-2 · the last background run's output block, drawn as the reference's
// `.extension-row__output` (`extensions.css:1199-1267`,
// `src/extensions/ExtensionRow.tsx:478-527`).
//
// Four differences the port carried: the block was a raised card rather than
// the reference's `--surface-sunken` recess, the captured text was set in the
// user's own interface font at the *body* step rather than the terminal's
// monospace at the caption step, the folded affordance was a whole ruled
// settings row with an action pill rather than a text control led by a
// chevron, and the truncation aside wore the warning ink where the reference
// keeps it tertiary. This pins the four.

// outputBlockApp is the fixture: one connected integration with a recorded
// background run, its output block either folded or open.
func outputBlockApp(open bool) *App {
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Permissions: []string{"environment"},
		Enforced:    map[string]bool{"environment": true},
		OutputMode:  "background",
	}, Actions{})
	// A successful run leaves the block folded (only a failed one unfolds it),
	// so the open fixture unfolds it explicitly.
	a.SetRunResult("dev.floter.tool", RunResult{Stdout: "/usr/bin/rg\n", Success: true})
	a.setRunOutputOpen("dev.floter.tool", open)
	return a
}

// The folded affordance is the reference's text control, not a settings row:
// its box is the row's own full-width line, it is led by a chevron, and its
// height is the reference's own 28px min-height rather than a 44u settings
// row.
func TestTheFoldedOutputAffordanceIsATextControl(t *testing.T) {
	en := i18n.For("en").Settings
	a := outputBlockApp(false)
	tt := integrationPage(t, a, 720, 1400)

	perm := mustFind(t, tt, en.IntegrationsPermissions)
	view := mustFind(t, tt, en.RunViewOutput)
	// The reference's toggle is `justify-self: start` on the row's own
	// full-width line: its word starts on the row labels' own column (the block
	// it opens carries the same inset), not centered in a card row.
	if delta := view.X - perm.X; absFloat(delta) > 1 {
		t.Errorf("the folded affordance starts at x=%v, the row labels at x=%v (delta %v); it must sit on the row's own content column",
			view.X, perm.X, delta)
	}
	// A card row's floor is 44u (or the integration card's 66u title floor);
	// the reference's toggle is a `min-height: 28px` text control.
	if view.H > 32 {
		t.Errorf("the folded affordance is %v tall, want the reference's ~28px text control", view.H)
	}
	// Clicking it unfolds the block.
	if err := tt.Click(en.RunViewOutput); err != nil {
		t.Fatalf("the affordance: %v", err)
	}
	tt.Frame()
	if !a.outputOpen["dev.floter.tool"] {
		t.Errorf("the affordance did not open the block")
	}
}

// The open block is a `--surface-sunken` recess under a `--glass-control-edge`
// hairline — never the raised `--glass-control` card the port gave it.
//
// The fill is transparent-ish over the card, so a pixel read would composite
// it away; the test reads the block's own face through its one draw point
// (`runOutputFace`) and pins it to the reference's token, and separately pins
// the *border* on the drawn block (a 1px `--glass-control-edge` hairline is
// what makes the recess visible at all).
func TestTheOutputBlockIsTheReferenceRecess(t *testing.T) {
	en := i18n.For("en").Settings
	a := outputBlockApp(true)
	tokens := theme.For(a.Store.Snapshot(), false)
	if got, want := runOutputFace(tokens), tokens.Sunken; got != want {
		t.Errorf("the block's face is %v, want the reference's --surface-sunken %v", got, want)
	}
	if runOutputFace(tokens) == tokens.Control {
		t.Errorf("the block wears the raised --glass-control plane %v rather than the recess", tokens.Control)
	}
	// And the border is drawn: the block's head sits inside the row's content
	// column, its box's left edge on the card's row inset — one whole card row
	// right of the card's frame.
	tt := integrationPage(t, a, 720, 2000)
	head := mustFind(t, tt, en.RunLastOutput)
	perm := mustFind(t, tt, en.IntegrationsPermissions)
	// The head's own column: the row's inset (the block's line) plus the
	// block's own 8px padding — the block is the row's full-width line, not a
	// card.
	if delta := head.X - perm.X; absFloat(delta-8) > 2 {
		t.Errorf("the block's head starts at x=%v, the row labels at x=%v (delta %v); want the row's line + the block's 8px padding",
			head.X, perm.X, delta)
	}
	// The block's box left edge, one border further in from the head's text:
	// the reference's 1px `--glass-control-edge` hairline is what makes the
	// transparent recess visible at all.
	left := int(head.X) - 9
	img := tt.Image()
	y := int(head.Y + head.H/2)
	edge := imagePixel(img, left, y)
	inside := imagePixel(img, left+3, y)
	if channelDelta(edge, inside) < 8 {
		t.Errorf("the block draws no left edge at x=%d (edge %v, inside %v); it must carry the reference's 1px hairline",
			left, edge, inside)
	}
}

// The captured text is the terminal's monospace at the caption step: it is a
// program's own output, so its line box is shorter than the block's own head
// label (which is also caption) only by the head's weight — but the decisive
// read is that the text is *not* the body step the port set it in, and that it
// is selectable.
func TestTheCapturedTextIsTheTerminalRegister(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	en := i18n.For("en").Settings
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Permissions: []string{"environment"},
		Enforced:    map[string]bool{"environment": true},
		OutputMode:  "background",
	}, Actions{})
	// One line, so the drawn box is one line box and the registers compare
	// directly.
	a.SetRunResult("dev.floter.tool", RunResult{Stdout: "/usr/bin/rg", Success: true})
	a.setRunOutputOpen("dev.floter.tool", true)
	tt := integrationPage(t, a, 720, 2000)

	body, ok := tt.Find("/usr/bin/rg")
	if !ok {
		t.Fatalf("the captured text was not drawn: %q", tt.Texts())
	}
	head := mustFind(t, tt, en.RunLastOutput)
	// Both registers are the *caption* step here, so their line boxes agree;
	// the port's body-step text was taller than the caption head. That is the
	// observable difference: a body-step line box can never equal a caption
	// one.
	if absFloat(body.H-head.H) > 0.5 {
		t.Errorf("the captured text's line box is %v against the caption head's %v; the text is not the caption step",
			body.H, head.H)
	}
	// The port set the text in the *body* step, which is one rung up: a body
	// line box is taller than the caption head's. Read the difference against
	// a body-step line the same page already draws, so the assertion cannot
	// pass merely because both registers happen to be captions.
	sublabel := mustFind(t, tt, "Read the environment (Floter enforces)")
	if body.H >= sublabel.H {
		t.Errorf("the captured text's line box %v is not below the row sublabel's body step %v",
			body.H, sublabel.H)
	}

	// A run that produced nothing keeps the block's normal line box: the
	// reference's `.extension-row__output-empty` is italic muted *text*, not a
	// collapsed blank (`extensions.css:1259-1267`).
	b := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Permissions: []string{"environment"},
		Enforced:    map[string]bool{"environment": true},
		OutputMode:  "background",
	}, Actions{})
	b.SetRunResult("dev.floter.tool", RunResult{Success: true})
	b.setRunOutputOpen("dev.floter.tool", true)
	tb := integrationPage(t, b, 720, 2000)
	empty, ok := tb.Find(en.RunNoOutput)
	if !ok {
		t.Fatalf("the empty placeholder is missing: %q", tb.Texts())
	}
	if empty.H < 6 {
		t.Errorf("the empty placeholder's line box is %v tall; it must keep the block's normal line box", empty.H)
	}
}

// The truncation aside is the reference's tertiary mark, never the warning
// ink: it says "the retained capture stops here", not "something failed".
func TestTheTruncationAsideIsNotAWarning(t *testing.T) {
	en := i18n.For("en").Settings
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Permissions: []string{"environment"},
		Enforced:    map[string]bool{"environment": true},
		OutputMode:  "background",
	}, Actions{})
	a.SetRunResult("dev.floter.tool", RunResult{Stdout: "hello\n", Truncated: true, Success: true})
	a.setRunOutputOpen("dev.floter.tool", true)
	tt := integrationPage(t, a, 720, 2000)

	tokens := theme.For(a.Store.Snapshot(), false)
	for _, label := range []string{en.RunOutputTruncated, en.RunOutputTruncatedTail} {
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q is missing: %q", label, tt.Texts())
		}
		// The label's own ink is the tertiary step. Read the darkest pixel in
		// its box: anti-aliasing never makes text *lighter* than its colour, so
		// the darkest pixel is the ink, and it must be nearer the tertiary mark
		// than the warning one.
		ink := darkestPixel(tt.Image(), rect)
		if channelDelta(ink, tokens.TextTertiary) > channelDelta(ink, tokens.Theme.Warning) {
			t.Errorf("%q is painted %v: nearer the warning ink %v than the tertiary mark %v",
				label, ink, tokens.Theme.Warning, tokens.TextTertiary)
		}
	}
}

// darkestPixel is the ink of a drawn label: the pixel in its box with the
// lowest channel sum.
func darkestPixel(img *image.RGBA, r ui.Rect) ui.Color {
	best := ui.Color{R: 255, G: 255, B: 255, A: 255}
	bestSum := 1 << 30
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := imagePixel(img, x, y)
			if sum := int(c.R) + int(c.G) + int(c.B); sum < bestSum {
				best, bestSum = c, sum
			}
		}
	}
	return best
}

// The block's own words are the reference's, so a language switch re-renders
// them: the port's keys were its own, and the reference's five are the ones
// the block must carry.
func TestTheOutputBlockCarriesTheReferenceWords(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings
	for _, want := range []struct {
		en, zh, key string
	}{
		{"Last output", "最近一次输出", "customLastOutput"},
		{"View output", "查看输出", "customViewOutput"},
		{"Hide output", "收起输出", "customHideOutput"},
		{"(No output)", "（无输出）", "customNoOutput"},
		{"Output was truncated", "输出已被截断", "customOutputTruncated"},
	} {
		if !strings.Contains(en.RunLastOutput+en.RunViewOutput+en.RunHideOutput+en.RunNoOutput+en.RunOutputTruncated, want.en) {
			t.Errorf("the en dictionary is missing the reference's %q (%s)", want.en, want.key)
		}
		if !strings.Contains(zh.RunLastOutput+zh.RunViewOutput+zh.RunHideOutput+zh.RunNoOutput+zh.RunOutputTruncated, want.zh) {
			t.Errorf("the zh dictionary is missing the reference's %q (%s)", want.zh, want.key)
		}
	}
}
