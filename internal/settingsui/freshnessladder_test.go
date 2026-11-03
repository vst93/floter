package settingsui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/theme"
)

// P13-fg（`docs/liquid-glass.md` §6.128）：集成页抽屉里那块新鲜度格子的两格值。
//
// 这块格子同时装着**答案**（`Succeeded`、`3 min. ago`）与**占位**（`Probing…`、
// `Unknown`、`Never probed`），而 `metadataCell` 此前把值的墨写死成 `TextSecondary`：
// 实测五个结果词与两种时间值全落在同一档（离档 0-3 级），于是「失败了」与「成功了」
// 同重、「还没探测」与「探测过了」同重。格子自己的阶梯很宽（`secondary → strong`
// 83-86 级、`secondary → muted` 18-23 级），所以这一轮只做一件事：**把每个词挂回
// 它自己的档**，几何一个字节不动。

// freshnessLadderState is one of the five words the Result cell can hold, with
// the register it must wear: an answer (`TextSecondary`), an answer that has to
// be read (`TextStrong`), or a placeholder where an answer would be
// (`TextMuted`).
type freshnessLadderState struct {
	name   string
	result extensions.FreshnessResult
	step   string
}

var freshnessLadderStates = []freshnessLadderState{
	{"success", extensions.FreshnessSuccess, "secondary"},
	{"degraded", extensions.FreshnessDegraded, "strong"},
	{"failed", extensions.FreshnessFailed, "strong"},
	{"running", extensions.FreshnessRunning, "muted"},
	{"unknown", extensions.FreshnessUnknown, "muted"},
}

// freshnessLadderFixture opens the Integrations page with one integration whose
// freshness the caller can move between frames, so all five words are read off
// the same grid (same face, same width, same neighbours).
func freshnessLadderFixture(t *testing.T, dark bool) (*ui.Tester, theme.Tokens, *extensions.Freshness, i18n.Settings) {
	t.Helper()
	en := i18n.For("en").Settings
	count := 6
	freshness := extensions.Freshness{
		AtSeconds: 1700000000, Source: extensions.FreshnessProbe,
		Result: extensions.FreshnessSuccess, CommandCount: &count,
		Delta: extensions.DeltaUnchanged,
	}
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
	}, Actions{
		Freshness: func(id string) (extensions.Freshness, bool) {
			return freshness, id == "dev.floter.tool"
		},
	})
	tt, tokens := renderIn(t, a, 720, 900, dark)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("open Integrations: %v", err)
	}
	tt.Frame()
	return tt, tokens, &freshness, en
}

// freshnessSteps is the ladder the two cells are read against: the three
// neutral inks plus the alarm, so a failure that started spending `Warning`
// would be named as such instead of silently passing as "strong".
func freshnessSteps(face ui.Color, tokens theme.Tokens) map[string]ui.Color {
	return map[string]ui.Color{
		"strong":    over(face, tokens.TextStrong),
		"secondary": over(face, tokens.TextSecondary),
		"muted":     over(face, tokens.Theme.TextMuted),
		"tertiary":  over(face, tokens.TextTertiary),
		"warning":   over(face, tokens.Theme.Warning),
	}
}

// TestTheFreshnessResultWordIsAnAnswerOrAPlaceholder pins the whole family at
// once — which is the reason §6.127's 127-5 was deferred: moving one arm would
// break the row's own timeline. It also pins that this round changed **only**
// ink: the five words sit in one and the same box.
func TestTheFreshnessResultWordIsAnAnswerOrAPlaceholder(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		tt, tokens, freshness, en := freshnessLadderFixture(t, appearance.dark)
		words := map[string]string{
			"success":  en.FreshnessResultSuccess,
			"degraded": en.FreshnessResultDegraded,
			"failed":   en.FreshnessResultFailed,
			"running":  en.FreshnessResultRunning,
			"unknown":  en.FreshnessResultUnknown,
		}
		var first ui.Rect
		for i, state := range freshnessLadderStates {
			freshness.Result = state.result
			freshness.Delta = extensions.DeltaUnchanged
			tt.Frame()
			word := words[state.name]
			box, ok := tt.Find(word)
			if !ok {
				t.Fatalf("[%v] the %s word %q is missing from the grid: %q",
					appearance.name, state.name, word, tt.Texts())
			}
			if i == 0 {
				first = box
			} else if box != first {
				t.Errorf("[%v] the %s word's cell is %v but the success word's is %v: "+
					"this round changes the ink of the value and nothing else",
					appearance.name, state.name, box, first)
			}
			face := boxFace(tt.Image(), box)
			steps := freshnessSteps(face, tokens)
			ink := boxInk(tt.Image(), box, face)
			got, off := nearestStep(ink, steps)
			if got != state.step {
				t.Errorf("[%v] the %s word is painted %v — nearest the %s step (%d levels off), "+
					"want the %s step %v: the grid has to say which value is an answer and which "+
					"one is a placeholder",
					appearance.name, state.name, ink, got, off, state.step, steps[state.step])
			}
		}

		// The three steps must actually be three steps apart, not three names for
		// one ink: read them off the same face and compare.
		face := boxFace(tt.Image(), first)
		steps := freshnessSteps(face, tokens)
		if d := channelDelta(steps["secondary"], steps["muted"]); d < 12 {
			t.Errorf("[%v] the answer step and the placeholder step are only %d levels apart: "+
				"a placeholder that close to an answer is not a second register", appearance.name, d)
		}
		if d := channelDelta(steps["secondary"], steps["strong"]); d < 40 {
			t.Errorf("[%v] the answer step and the read-me step are only %d levels apart: "+
				"a failed derivation would read as a settled one", appearance.name, d)
		}
	}
}

// TestTheNeverProbedCellIsQuieterThanAProbedOne pins the other cell of the same
// grid: `Never probed` is the reference's placeholder, and it used to wear the
// same ink as a real relative time — the grid's two cells disagreed about what
// a placeholder is.
func TestTheNeverProbedCellIsQuieterThanAProbedOne(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		tt, tokens, freshness, en := freshnessLadderFixture(t, appearance.dark)

		freshness.AtSeconds = time.Now().Unix()
		tt.Frame()
		probedBox, ok := tt.Find(en.FreshnessJustNow)
		if !ok {
			// The fixture's timestamp is in 2023, so the cell reads a relative age
			// rather than "just now"; find it by the cell's own label instead.
			t.Fatalf("[%v] the probed cell does not read %q: %q",
				appearance.name, en.FreshnessJustNow, tt.Texts())
		}

		freshness.AtSeconds = 0
		tt.Frame()
		neverBox, ok := tt.Find(en.FreshnessNever)
		if !ok {
			t.Fatalf("[%v] the never-probed cell is missing %q: %q",
				appearance.name, en.FreshnessNever, tt.Texts())
		}
		if neverBox != probedBox {
			t.Errorf("[%v] the never-probed cell is %v but the probed one was %v: "+
				"the value's own box may not move with its ink",
				appearance.name, neverBox, probedBox)
		}

		face := boxFace(tt.Image(), neverBox)
		steps := freshnessSteps(face, tokens)
		neverInk := boxInk(tt.Image(), neverBox, face)
		if got, off := nearestStep(neverInk, steps); got != "muted" {
			t.Errorf("[%v] %q is painted %v — nearest the %s step (%d levels off), "+
				"want the placeholder step %v: nothing ever probed this list, and the cell "+
				"has to say so in its own register",
				appearance.name, en.FreshnessNever, neverInk, got, off, steps["muted"])
		}
	}
}

// TestDumpP13FGFreshnessPNGs is the round's own frame: the freshness grid in
// both appearances, once with a settled answer and once with a failed one.
func TestDumpP13FGFreshnessPNGs(t *testing.T) {
	if os.Getenv("FLOTER_P13FG_DUMP") == "" {
		t.Skip("opt-in: FLOTER_P13FG_DUMP=1 writes floter-P13-fg-freshness-<appearance>.png")
	}
	dir := os.Getenv("FLOTER_P13FG_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	tag := os.Getenv("FLOTER_P13FG_TAG")
	if tag == "" {
		tag = "after"
	}
	out := filepath.Join(dir, tag)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		tt, _, freshness, en := freshnessLadderFixture(t, dark)
		freshness.Result = extensions.FreshnessFailed
		tt.Frame()
		anchor, ok := tt.Find(en.Freshness)
		if !ok {
			t.Fatalf("the block label is missing: %q", tt.Texts())
		}
		box := ui.Rect{X: anchor.X - 18, Y: anchor.Y - 18, W: 420, H: 132}
		img := tt.Image()
		r := image.Rect(int(box.X), int(box.Y), int(box.X+box.W), int(box.Y+box.H))
		crop := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				c := imagePixel(img, x, y)
				i := crop.PixOffset(x-r.Min.X, y-r.Min.Y)
				crop.Pix[i], crop.Pix[i+1], crop.Pix[i+2], crop.Pix[i+3] = c.R, c.G, c.B, c.A
			}
		}
		name := filepath.Join(out, "floter-P13-fg-freshness-"+appearanceOf(dark)+".png")
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, crop); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", name)
	}
}
