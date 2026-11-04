package settingsui

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The keyboard is the only layer that moves. P13-gf walked three 60×40 probe
// boxes: with the middle one disabled, Tab steps 0 → 2 → 0 and the disabled
// box's ring band never leaves the ground. That is the *local* rule (a disabled
// control has no keyboard position). Nobody has ever read the *global* one off a
// real page: a settings page is 1426 units tall inside a 523-unit body, so Tab
// must scroll the body down, step by step, and the wrap at the end must bring
// the body back to the top — and Shift+Tab must be the same ladder backwards.
//
// Three things are measured here, on the real General page at 720×580:
//
//  1. The downward walk: `a.Body.Y` never goes down-to-up between two stops, no
//     step is a dead step (the frame always changes), and the wrap is a real
//     backtrack — of more than a viewport — not a small downward nudge.
//  2. The second pass repeats the first: after the wrap the same ladder is walked
//     again, in the same order.
//  3. A scroll offset of x.5 does not blur. `nearest` (`mygo/ui/scroll.go:312`)
//     leaves fractional offsets on some stops, and the frame carries proof that
//     the rasteriser snaps them: the same content one stop later sits exactly 26
//     rows up, with the same strengths. So "round the scroll" is not a fix this
//     port needs, and a future one would be visible here.
//
// Measured values are in `docs/rounds/P13-gg-report.md`, contract §6.152.

// tabWalk is one stop of the keyboard's walk: where it left the body's scroll,
// and how many pixels the frame moved to get there.
type tabWalk struct {
	y       float32
	changed int
}

// tabFrameDelta counts the pixels that differ between two frames.
func tabFrameDelta(a, b *image.RGBA) int {
	n := 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			i := a.PixOffset(x, y)
			if tabDelta(int(a.Pix[i]), int(b.Pix[i])) > 6 ||
				tabDelta(int(a.Pix[i+1]), int(b.Pix[i+1])) > 6 ||
				tabDelta(int(a.Pix[i+2]), int(b.Pix[i+2])) > 6 {
				n++
			}
		}
	}
	return n
}

func tabDelta(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// tabWalkPage builds the real settings surface on a 720×580 window — the
// launcher's input width and the panel's own height, so the walk below is the
// one a user gets, not a probe's smaller world.
func tabWalkPage(t *testing.T, dark bool) (*App, *ui.Tester) {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	tt := render(t, a, 720, 580)
	if dark {
		tt.SetDark(true)
	}
	tt.Frame()
	return a, tt
}

// tabWalkSteps presses Tab n times and returns every stop.
func tabWalkSteps(t *testing.T, a *App, tt *ui.Tester, n int, shift bool) []tabWalk {
	t.Helper()
	prev := tt.Image()
	out := make([]tabWalk, 0, n)
	for i := 0; i < n; i++ {
		if shift {
			tt.Key(ui.Shift, ui.KeyTab)
		} else {
			tt.Key(0, ui.KeyTab)
		}
		tt.Frame()
		img := tt.Image()
		out = append(out, tabWalk{y: a.Body.Y, changed: tabFrameDelta(prev, img)})
		prev = img
	}
	return out
}

// TestTabWalksTheSettingsBodyDownward pins the ladder: sixteen stops down a
// 1426-unit page, the body following one stop at a time, then the wrap back to
// the top and the same ladder again.
func TestTabWalksTheSettingsBodyDownward(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a, tt := tabWalkPage(t, dark)
			// A page taller than its body is the precondition: if the page ever
			// fits, every assertion below is about a walk that cannot scroll and
			// the round's subject has moved.
			if a.Body.MaxY <= 0 {
				t.Fatalf("the body does not scroll (MaxY=%.1f): this page no longer tests a scrolling walk", a.Body.MaxY)
			}
			stops := tabWalkSteps(t, a, tt, 16, false)
			t.Logf("%s MaxY=%.1f stops=%v", name, a.Body.MaxY, stops)

			for i, s := range stops {
				// No dead step: landing on a control always changes the frame.
				if s.changed <= 100 {
					t.Errorf("stop %d changed %d pixels: the keyboard moved and the frame did not", i+1, s.changed)
				}
				if i > 0 && s.y < stops[i-1].y {
					t.Errorf("stop %d left the body at %.1f, up from %.1f: the walk scrolled backwards in the middle of the page",
						i+1, s.y, stops[i-1].y)
				}
			}
			// The walk actually used the body: the last stop is deep in the page.
			if last := stops[len(stops)-1]; last.y < a.Body.MaxY/2 {
				t.Errorf("the last of sixteen stops left the body at %.1f of %.1f: Tab never walked down the page",
					last.y, a.Body.MaxY)
			}
			// The wrap: one more Tab, and the body goes back up by more than one
			// viewport — the keyboard returned to the top of the page, it did not
			// nudge on down.
			wrap := tabWalkSteps(t, a, tt, 1, false)[0]
			t.Logf("%s wrap=%v", name, wrap)
			if wrap.y >= stops[len(stops)-1].y {
				t.Errorf("the wrap left the body at %.1f, not above the last stop's %.1f", wrap.y, stops[len(stops)-1].y)
			}
			if wrap.changed <= 100 {
				t.Errorf("the wrap changed %d pixels: the body did not move back", wrap.changed)
			}
			// 184 is where the wrap lands: the first screenful's own offset. The
			// value is pinned because it is the visible statement of "the wrap
			// shows the top of the page, with 184 units of it above the fold".
			if wrap.y != 184 {
				t.Errorf("the wrap left the body at %.1f, want 184", wrap.y)
			}
			// The second pass is the first pass again: every offset it reaches
			// appears in the first pass, in the same order.
			second := tabWalkSteps(t, a, tt, 14, false)
			t.Logf("%s second=%v", name, second)
			first := make([]float32, 0, len(stops))
			for _, s := range stops {
				first = append(first, s.y)
			}
			j := 0
			for i, s := range second {
				if s.y <= wrap.y {
					continue // still on the first screenful's own five stops
				}
				for j < len(first) && first[j] < s.y {
					j++
				}
				if j == len(first) || first[j] != s.y {
					t.Fatalf("second pass stop %d is at %.1f, which the first pass never reached in that order (first=%v)",
						i+1, s.y, first)
				}
			}
		})
	}
}

// TestShiftTabIsTheMirrorLeg pins the other direction: Shift+Tab walks the same
// page backwards, the body following it up, and one wrap at the end.
func TestShiftTabIsTheMirrorLeg(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a, tt := tabWalkPage(t, dark)
			down := tabWalkSteps(t, a, tt, 16, false)
			deep := down[len(down)-1].y
			back := tabWalkSteps(t, a, tt, 24, true)
			t.Logf("%s deep=%.1f back=%v", name, deep, back)

			wrapped := false
			for i, s := range back {
				if s.changed <= 100 {
					t.Errorf("shift-tab %d changed %d pixels: the keyboard moved and the frame did not", i+1, s.changed)
				}
				if i == 0 {
					continue
				}
				if s.y > back[i-1].y {
					// The wrap is the only transition that may go the other way,
					// and it is the end of the leg: Shift+Tab ran out of stops at
					// the top and went to the last one.
					if gap := s.y - back[i-1].y; gap < 523 {
						t.Errorf("shift-tab %d wrapped from %.1f to %.1f: %.1f is less than a viewport, so it nudged instead of wrapping",
							i+1, back[i-1].y, s.y, gap)
					}
					if back[i-1].y > 184 {
						t.Errorf("shift-tab %d wrapped at %.1f, still below the top of the page (184)", i+1, back[i-1].y)
					}
					if s.y < a.Body.MaxY-523 {
						t.Errorf("shift-tab %d wrapped to %.1f, not the end of a %.1f page", i+1, s.y, a.Body.MaxY)
					}
					wrapped = true
					break
				}
			}
			if !wrapped {
				t.Errorf("twenty-four shift-tabs from the last stop never wrapped (back=%v)", back)
			}
		})
	}
}

// tabLineRows is the frame's long horizontal lines: rows whose pixels across the
// card column are darker than the row above them by more than a level, with the
// mean darkening. A hairline is one such row; a blurred one would be two, each
// about half as strong.
func tabLineRows(img *image.RGBA) map[int]int {
	rows := map[int]int{}
	for y := 60; y < 575; y++ {
		run, sum := 0, 0
		for x := 210; x < 690; x++ {
			i := img.PixOffset(x, y)
			j := img.PixOffset(x, y-1)
			d := int(img.Pix[j]) + int(img.Pix[j+1]) + int(img.Pix[j+2]) -
				int(img.Pix[i]) - int(img.Pix[i+1]) - int(img.Pix[i+2])
			if d > 3 {
				run++
				sum += d / 3
			}
		}
		if run > 300 {
			rows[y] = sum / run
		}
	}
	return rows
}

// TestAHalfUnitScrollDrawsWholeRows is the guard against a plausible wrong fix.
// Some stops leave `a.Body.Y` on x.5 (26.5, 86.5, …), and the obvious reading is
// "a half-unit scroll blurs every hairline". The frame says otherwise: the same
// content one stop later sits a whole number of rows up, with the same strengths.
// If the rasteriser ever starts drawing at the half unit, this test goes red and
// the round that changes it has to say so.
func TestAHalfUnitScrollDrawsWholeRows(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a, tt := tabWalkPage(t, dark)
			// Three stops stay at the top; the fourth is the first that scrolls,
			// and its offset is fractional.
			tabWalkSteps(t, a, tt, 3, false)
			top := tt.Image()
			at := tabWalkSteps(t, a, tt, 1, false)[0]
			if at.y == float32(int(at.y)) {
				t.Fatalf("the fourth stop left the body at %.1f — a whole unit, so there is no half-unit scroll to read", at.y)
			}
			scrolled := tt.Image()
			t.Logf("%s fourth stop y=%.1f", name, at.y)

			a0, s0 := tabLineRows(top), tabLineRows(scrolled)
			// Two rows are not the content's: the body's own soft edges fade
			// whatever is under them, so a line that sits near an edge in one
			// frame and not in the other reads differently without anything
			// having blurred. Only rows inside the clear middle are compared.
			const clearTop, clearBottom = 110, 520
			// And the ring sits on the focused control's edges: those rows carry
			// the ring's ink in one frame and not in the other. A quiet line is
			// below the ring's own strength (the ring lifts a 30 to 48 in light,
			// a 19 to 47 in dark).
			quiet := func(m map[int]int, ring int) map[int]int {
				out := map[int]int{}
				for r, s := range m {
					if s <= ring && r > clearTop && r < clearBottom {
						out[r] = s
					}
				}
				return out
			}
			ring := 35
			if dark {
				ring = 25
			}
			q0, q1 := quiet(a0, ring), quiet(s0, ring)
			if len(q0) < 4 || len(q1) < 4 {
				t.Fatalf("%s read only %d and %d quiet lines: the probe is not looking at a laid out page", name, len(q0), len(q1))
			}
			// The content moved by one whole number of rows, the same for all of
			// it: 26 in this page's layout (the rasteriser floors the .5).
			offset := -1
			for r := range q0 {
				if _, ok := q1[r-26]; ok {
					offset = 26
				}
			}
			if offset < 0 {
				t.Fatalf("%s no line at Y=0 has a partner 26 rows up in the scrolled frame: rows %v / %v", name, q0, q1)
			}
			pairs := 0
			for r, s := range q0 {
				got, ok := q1[r-offset]
				if !ok && (r-offset <= clearTop || r-offset >= clearBottom) {
					continue
				}
				if !ok {
					t.Errorf("%s line row %d (strength %d) has no partner at %d in the scrolled frame", name, r, s, r-offset)
					continue
				}
				if tabDelta(got, s) > 1 {
					t.Errorf("%s line row %d: strength %d at the top, %d after the scroll — the same line came out differently", name, r, s, got)
				}
				pairs++
				// The line is one row, not two half rows: a split line would be
				// two adjacent quiet rows of about half the strength each.
				for _, n := range []int{r - 1, r + 1} {
					if v, ok := q0[n]; ok {
						t.Errorf("%s line row %d (strength %d) has a quiet neighbour at %d (strength %d): the line is split across two rows",
							name, r, s, n, v)
					}
				}
			}
			if pairs < 4 {
				t.Errorf("%s matched only %d lines across the scroll", name, pairs)
			}
		})
	}
}
