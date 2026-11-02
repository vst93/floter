package settingsui

import (
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/scriptlang"
)

// P13-dm: the tool form's toolchain line.
//
// The line is the one the script form draws under the language picker: what
// this machine has for the language the script is written in. It is a
// **sentence**, and until P13-dm it was drawn as the control of a settings row
// whose label was the word "Language" a second time — so it was right-aligned
// into the card's control slot, 470px away from the picker it explains, and the
// longest verdict was wider than that slot.
//
// Measured before (720×1600, design theme, dark): ready `node v20.11.0` in a
// 61px slot at x=628.8; the missing verdict's own box was 331px while the
// sentence's natural width was 340 — the last two words of "…to check again."
// were replaced by the framework's ellipsis. After: x=201.8 (the picker's own
// column), natural width, wrapped at the card's text column.

// renderToolForm draws the settings panel with the Add-a-local-tool drawer open
// in script mode, in production's own frame (the design theme pinned on the
// context, as the shell does before the panel draws).
func renderToolForm(t *testing.T, runtime func(string) scriptlang.Check, w, h int, dark bool) (*ui.Tester, *App) {
	t.Helper()
	skipWithoutTheReferenceFont(t)
	a := New(newStore(t), Actions{ScriptRuntime: runtime, ConnectLocalTool: func(CustomTool) {}})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, w, h)
	tt.SetDark(dark)
	tt.Frame()
	copy := i18n.For("en").Settings
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("integrations: %v", err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatalf("add a local tool: %v (texts %q)", err, tt.Texts())
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatalf("script mode: %v (texts %q)", err, tt.Texts())
	}
	tt.Frame()
	return tt, a
}

// toolchainVerdicts are the three answers the line can carry, as the probe
// gives them: a ready toolchain, an absent one, and one still being checked.
func toolchainVerdicts(t *testing.T) []struct {
	name    string
	runtime func(string) scriptlang.Check
	says    string
} {
	t.Helper()
	copy := i18n.For("en").Settings
	return []struct {
		name    string
		runtime func(string) scriptlang.Check
		says    string
	}{
		{
			name: "ready",
			runtime: func(string) scriptlang.Check {
				return scriptlang.Check{Available: true, Path: "/usr/local/bin/node", Version: "v20.11.0"}
			},
			says: "node v20.11.0",
		},
		{
			name:    "missing",
			runtime: func(string) scriptlang.Check { return scriptlang.Check{Candidates: []string{"node"}} },
			says:    copy.LocalToolRuntimeMissing("node"),
		},
		{
			// A probe that never lands is the "checking" state: the frame asks,
			// the probe has no answer yet, and the line says so.
			name:    "checking",
			runtime: func(string) scriptlang.Check { select {} },
			says:    copy.LocalToolRuntimeChecking,
		},
	}
}

// lineInkExtent returns the leftmost and rightmost column carrying ink in a
// rect's own band, plus how many columns carry any. The background is sampled
// per scanline at bgX, a column of pure surface beside the ink.
func lineInkExtent(img *image.RGBA, r ui.Rect, bgX int) (lo, hi, cols int) {
	lo, hi = -1, -1
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		if y < 0 || y >= img.Rect.Dy() {
			continue
		}
		bg := img.RGBAAt(bgX, y)
		for x := int(r.X); x < int(r.X+r.W) && x < img.Rect.Dx(); x++ {
			if x < 0 {
				continue
			}
			px := img.RGBAAt(x, y)
			gap := chanDiff(px.R, bg.R) + chanDiff(px.G, bg.G) + chanDiff(px.B, bg.B)
			if gap < 12 {
				continue
			}
			cols++
			if lo < 0 || x < lo {
				lo = x
			}
			if x > hi {
				hi = x
			}
		}
	}
	return lo, hi, cols
}

func chanDiff(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}

// TestTheToolchainLineSitsUnderItsPicker pins the line's column: it is the
// picker's own second line, so it starts on the picker's text origin and its
// ink never reaches the control slot the row above hands to its select.
func TestTheToolchainLineSitsUnderItsPicker(t *testing.T) {
	copy := i18n.For("en").Settings
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, verdict := range toolchainVerdicts(t) {
			tt, a := renderToolForm(t, verdict.runtime, 720, 1600, dark)
			if verdict.name != "checking" {
				a.scriptRuntimeVerdict("js")
			}
			tt.Frame()

			line := mustFind(t, tt, verdict.says)
			hint := mustFind(t, tt, copy.LocalToolLanguageHint)
			lo, hi, _ := lineInkExtent(tt.Image(), line, int(line.X)-3)
			// The static hint above it is the picker's own second line: the
			// two share one text origin, the way every row's label and its
			// hint do.
			if line.X != hint.X {
				t.Errorf("[%v/%s] the toolchain line starts at x=%.1f, its picker's hint at %.1f: "+
					"a row's second line starts on the row's text origin, not in the control slot",
					appearance, verdict.name, line.X, hint.X)
			}
			// The line is below the hint and above the script editor's own
			// block: it is the picker's sub-line, not a row of its own.
			if line.Y < hint.Y+hint.H {
				t.Errorf("[%v/%s] the toolchain line (y=%.1f) is not under its picker's hint (%.1f..%.1f)",
					appearance, verdict.name, line.Y, hint.Y, hint.Y+hint.H)
			}
			script := mustFind(t, tt, copy.LocalToolScriptHint)
			if line.Y+line.H > script.Y {
				t.Errorf("[%v/%s] the toolchain line (%.1f..%.1f) runs into the script editor (y=%.1f)",
					appearance, verdict.name, line.Y, line.Y+line.H, script.Y)
			}
			// One origin, one column: the ink starts on the box's own first
			// column (a right-aligned slot left a 470px gutter before it).
			if lo != int(line.X) {
				t.Errorf("[%v/%s] the line's first ink column is %d, its box starts at %.1f",
					appearance, verdict.name, lo, line.X)
			}
			if hi < lo {
				t.Errorf("[%v/%s] the line has no ink at all", appearance, verdict.name)
			}
			t.Logf("[%v/%s] x=%.1f y=%.1f w=%.1f ink=%d..%d", appearance, verdict.name,
				line.X, line.Y, line.W, lo, hi)
		}
	}
}

// naturalInkWidth is how wide a sentence inks with nothing constraining it:
// the rung the toolchain line wears, in a frame wide enough to hold it whole.
// A line that inks less than this in the card has had its tail cut.
func naturalInkWidth(t *testing.T, a *App, text string) int {
	t.Helper()
	const w, h = 4000, 200
	tt := ui.NewTester(func(c *ui.Context) {
		tokens := a.tokens(c)
		c.SetTheme(tokens.Theme)
		c.Root().Background(ui.Transparent)
		ui.Text(c, text).FontSize(tokens.Caption).FontWeight(500).TextColor(a.cardTheme(c).TextMuted)
	}, w, h)
	tt.Frame()
	lo, hi, _ := lineInkExtent(tt.Image(), ui.Rect{X: 0, Y: 0, W: w, H: h}, w-1)
	if lo < 0 {
		t.Fatalf("%q inks nothing at all", text)
	}
	return hi - lo + 1
}

// TestTheToolchainLineIsNotCutWhenTheVerdictIsLong pins that the line is a
// sentence rather than a slot: the missing verdict inks exactly as wide in the
// card as it does with nothing constraining it. Before P13-dm its natural width
// was 340 and its control slot 331, so the last words of "…to check again."
// were replaced by the framework's ellipsis.
func TestTheToolchainLineIsNotCutWhenTheVerdictIsLong(t *testing.T) {
	copy := i18n.For("en").Settings
	says := copy.LocalToolRuntimeMissing("node")
	tt, a := renderToolForm(t, func(string) scriptlang.Check {
		return scriptlang.Check{Candidates: []string{"node"}}
	}, 720, 1600, true)
	a.scriptRuntimeVerdict("js")
	tt.Frame()
	line := mustFind(t, tt, says)
	lo, hi, _ := lineInkExtent(tt.Image(), line, int(line.X)-3)
	if lo < 0 {
		t.Fatal("the toolchain line has no ink")
	}
	got, want := hi-lo+1, naturalInkWidth(t, a, says)
	if got != want {
		t.Errorf("the verdict inks %dpx in the card but %dpx unconstrained: the sentence is cut "+
			"(P13-dm: the line wraps at the card's text column instead of living in the control slot)",
			got, want)
	}
}

// TestDumpToolchainLineForP13dm writes the script form's toolchain line in all
// three verdicts, both appearances, to /tmp. Opt-in: `FLOTER_DUMP=1`.
func TestDumpToolchainLineForP13dm(t *testing.T) {
	if os.Getenv("FLOTER_DUMP") == "" {
		t.Skip("opt-in: set FLOTER_DUMP=1 to write /tmp/floter-P13-dm-runline-*.png")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, verdict := range toolchainVerdicts(t) {
			tt, a := renderToolForm(t, verdict.runtime, 720, 1600, dark)
			if verdict.name != "checking" {
				a.scriptRuntimeVerdict("js")
			}
			tt.Frame()
			path := "/tmp/floter-P13-dm-runline-" + appearance + "-" + verdict.name + ".png"
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
