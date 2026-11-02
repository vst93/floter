package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// The About page's update row is the one row on that page whose second line
// **changes**: it is the machine's answer to "is there a new version", and the
// answer arrives in one of four shapes. Until P13-dk all four were painted at
// the subtitle rung (`rowTitle`'s plain sublabel: `t.TextMuted` / body / 500),
// which is the rung that means "a sentence explaining the row above it". So
// "Up to date", "Latest version: v1.4.0", "Update failed, retry?" and "This
// build has no update feed" resolved the same ink — measured light 101 and dark
// 94 for every one of them — and the only line on the row that can carry news
// was the quietest thing on it.
//
// §6.26 already ruled on this shape for the sessions meta line: a failed run is
// a meaning, and a meaning may take a hue (`t.Warning`). These pins hold the new
// ladder — quiet / fact / warning — and, just as important, that the row itself
// did not move: same box for the version line, same box for the state line, in
// all four states.

// updateState is one face of the update row's state machine: the struct
// `updateRow` switches on (`settingsui.go:4054`), the line it prints and the rung
// that line must resolve.
type updateState struct {
	name  string
	state UpdateState
	line  string
	ink   rowInk
}

func updateStates() []updateState {
	en := i18n.For("en").Settings
	return []updateState{
		{"up-to-date", UpdateState{Checked: true}, en.UpdateUpToDate, rowInkQuiet},
		{"disabled", UpdateState{Disabled: true}, en.UpdateDisabled, rowInkQuiet},
		{"available", UpdateState{Checked: true, Available: true, Version: "1.4.0"},
			en.UpdateLatestVersion + ": v1.4.0", rowInkFact},
		{"failed", UpdateState{Checked: true, Failed: true}, en.UpdateFailedRetry, rowInkWarning},
	}
}

// updatePage renders the About page with one update state pinned on it.
func updatePage(t *testing.T, dark bool, state UpdateState) *ui.Tester {
	t.Helper()
	a := New(newStore(t), Actions{})
	a.About = aboutFixture()
	a.Update = state
	tt, _ := renderIn(t, a, 760, 1400, dark)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt
}

// inkPixel is the pixel a text box's ink resolves at: the darkest one on a light
// frame, the brightest one on a dark frame. The ink's *colour* matters here as
// much as its lightness — the warning rung is a hue, not one more grey.
func inkPixel(tt *ui.Tester, r ui.Rect, dark bool) (span int, at [3]uint8) {
	img := tt.Image()
	lo, hi := 255, -1
	var loAt, hiAt [3]uint8
	for y := int(r.Y); y < int(r.Y+r.H) && y < img.Bounds().Dy(); y++ {
		for x := int(r.X); x < int(r.X+r.W) && x < img.Bounds().Dx(); x++ {
			i := y*img.Stride + x*4
			l := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			if l < lo {
				lo, loAt = l, [3]uint8{img.Pix[i], img.Pix[i+1], img.Pix[i+2]}
			}
			if l > hi {
				hi, hiAt = l, [3]uint8{img.Pix[i], img.Pix[i+1], img.Pix[i+2]}
			}
		}
	}
	if dark {
		return hi - lo, hiAt
	}
	return hi - lo, loAt
}

// TestTheUpdateRowsStateLineSpeaksAtItsOwnRung is the round's pin: one row, four
// answers, three rungs, and a box that never moves.
func TestTheUpdateRowsStateLineSpeaksAtItsOwnRung(t *testing.T) {
	en := i18n.For("en").Settings
	versionText := en.UpdateCurrentVersion + ": v" + aboutFixture().Version
	for _, dark := range []bool{false, true} {
		appearance := appearanceName(dark)
		var versionBox, stateBox ui.Rect
		var versionInk int
		rung := map[rowInk]int{}
		for i, s := range updateStates() {
			tt := updatePage(t, dark, s.state)
			version := mustFind(t, tt, versionText)
			state := mustFind(t, tt, s.line)
			vInk, vAt := inkPixel(tt, version, dark)
			sInk, sAt := inkPixel(tt, state, dark)
			t.Logf("%s %-11s version ink %d %v | state %q ink %d %v box %v×%v x=%v y=%v",
				appearance, s.name, vInk, vAt, s.line, sInk, sAt,
				state.W, state.H, state.X, state.Y)

			// A state change is a change of *news*, never of layout: the version
			// line and its state line keep the same boxes in all four states.
			if i == 0 {
				versionBox, stateBox, versionInk = version, state, vInk
			} else {
				for _, p := range []struct {
					what      string
					got, want ui.Rect
				}{{"the version line", version, versionBox}, {"the state line", state, stateBox}} {
					if d := p.got.H - p.want.H; d > 0.5 || d < -0.5 {
						t.Errorf("%s: %q is drawn in a %vpx box in the %s state, %vpx in the first "+
							"— a state change may not reflow the row (§6.80)", appearance, p.what,
							p.got.H, s.name, p.want.H)
					}
					if d := p.got.Y - p.want.Y; d > 0.5 || d < -0.5 {
						t.Errorf("%s: %q starts at y=%v in the %s state, y=%v in the first — a "+
							"state change may not move the row (§6.80)", appearance, p.what,
							p.got.Y, s.name, p.want.Y)
					}
					if d := p.got.X - p.want.X; d > 0.5 || d < -0.5 {
						t.Errorf("%s: %q starts at x=%v in the %s state, x=%v in the first — the "+
							"label and the state line are one column (§6.80)", appearance, p.what,
							p.got.X, s.name, p.want.X)
					}
				}
				// The version line is the row's label: it does not change rung
				// when the answer does.
				if d := vInk - versionInk; d > 1 || d < -1 {
					t.Errorf("%s: the version line resolves %d in the %s state and %d in the "+
						"first — the label does not change rung when the state does (§6.80)",
						appearance, vInk, s.name, versionInk)
				}
			}
			// One rung is one ink: the two quiet states must resolve the same.
			if prev, ok := rung[s.ink]; ok {
				if d := sInk - prev; d > 1 || d < -1 {
					t.Errorf("%s: the %s state resolves %d and the other state on its rung %d — "+
						"one rung is one ink (§6.80)", appearance, s.name, sInk, prev)
				}
			}
			rung[s.ink] = sInk

			// The failed state is a meaning, so it wears a hue: the ink's own
			// channels have to be warm, on both appearances.
			if s.ink == rowInkWarning {
				if warm := int(sAt[0]) - int(sAt[2]); warm < 30 {
					t.Errorf("%s: the failed state's ink is %v — a failure is a meaning and a "+
						"meaning may take a hue (§6.26), but this one is neutral", appearance, sAt)
				}
			}
		}
		// The ladder itself: quiet < fact < warning, and by enough to read.
		quiet, fact, warning := rung[rowInkQuiet], rung[rowInkFact], rung[rowInkWarning]
		const gap = 12
		if fact-quiet < gap {
			t.Errorf("%s: the available state resolves %d against the quiet rung's %d (%+d) — "+
				"a version to fetch is a fact, one rung above a subtitle (§6.80)",
				appearance, fact, quiet, fact-quiet)
		}
		if warning-fact < gap {
			t.Errorf("%s: the failed state resolves %d against the available state's %d (%+d) — "+
				"the two are different answers and may not share a rung (§6.80)",
				appearance, warning, fact, warning-fact)
		}
		if quiet > 130 {
			t.Errorf("%s: the quiet states resolve %d — \"up to date\" and \"no feed\" are the "+
				"row's subtitle rung, not its content (§6.80)", appearance, quiet)
		}
	}
}

// TestDumpP13DKUpdateStateRows writes the round's visual evidence: the About
// page's update card in its four states, both appearances, tagged by the caller
// so a before/after pair comes out of the same build.
func TestDumpP13DKUpdateStateRows(t *testing.T) {
	if os.Getenv("FLOTER_P13DK_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13DK_DUMP=1 to write /tmp/floter-P13-dk-*.png")
	}
	skipWithoutTheReferenceFont(t)
	tag := os.Getenv("FLOTER_P13DK_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		a.About = aboutFixture()
		for _, s := range updateStates() {
			a.Update = s.state
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				a.View(c)
			}, 760, 900)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			if err := tt.Click("About"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			name := fmt.Sprintf("/tmp/floter-P13-dk-update-%s-%s-%s.png",
				appearanceName(dark), s.name, tag)
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s", name)
		}
	}
}
