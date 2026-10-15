package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// The sidebar entry wears one of the reference's three ink rungs
// (`main:src/styles/settings.css`): `.settings-sidebar__item` rests on
// `--text-muted` (`:137`), the `:hover` rule lifts it to `--text-primary`
// (`:152-156`), and `.settings-sidebar__item--active` keeps the chosen page on
// `--text-primary` too (`:166-170`). The accent survives on the chosen item's
// icon alone (`:172-174`); the label never carries it, and the two live states
// never keep the resting muted ink.
//
// The port kept `--text-muted` under the pointer as well, so hovering a page
// never lit it, and it painted the chosen page in `--text-strong` — a hair
// dimmer than the reference's own `--text-primary`. This drives the live
// "Sessions" entry (not the page on screen, so its rest state is observable)
// through all three and reads the label's own ink: muted over its plane at
// rest, `--text-primary` under the pointer, and `--text-primary` when chosen.
// Mutation check: drop the `row.Hovered()` arm and the hover read stays muted
// (the `--text-muted` composite), failing the pointer assertion.
func TestTheSidebarItemLiftsItsInkOnHoverAndSelection(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 580)
	tokens := theme.For(settings.Default(), false)

	// The sidebar is the 148u column on the left; the word also names the page
	// once Sessions is chosen, so pin the box to the sidebar.
	label := func() ui.Rect {
		r, ok := tt.Find("Sessions")
		if !ok || r.X > 140 {
			t.Fatalf("the sidebar's Sessions entry is missing (rect %+v, texts %q)", r, tt.Texts())
		}
		return r
	}

	// At rest the entry is muted ink over the column's plane.
	rest := label()
	restInk := panelInk(tt, rest)
	if want := panelOver(tokens.Theme.TextMuted, panelSurface(tt, rest)); !panelClose(restInk, want, 20) {
		t.Errorf("the resting entry's ink = %v, want --text-muted over its plane (%v)", restInk, want)
	}

	// The pointer lifts the whole entry to `--text-primary`: the ink drops to
	// the primary register and no longer matches `--text-muted`.
	tt.Move(rest.X+rest.W/2, rest.Y+rest.H/2)
	tt.Frame()
	hover := label()
	hoverSurface := panelSurface(tt, hover)
	hoverInk := panelInk(tt, hover)
	if want := panelOver(tokens.Theme.Text, hoverSurface); !panelClose(hoverInk, want, 20) {
		t.Errorf("the hovered entry's ink = %v, want --text-primary (%v)", hoverInk, want)
	}
	if muted := panelOver(tokens.Theme.TextMuted, hoverSurface); panelClose(hoverInk, muted, 20) {
		t.Errorf("hover left the ink at --text-muted (%v); the reference lifts it to --text-primary", hoverInk)
	}
	if channelDelta(hoverInk, restInk) < 40 {
		t.Errorf("hover did not lift the ink: rest %v → hover %v", restInk, hoverInk)
	}

	// Choosing the page keeps the primary register; park the pointer away
	// again so the chosen pane — not a hover — is what is read.
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Move(rest.X+rest.W/2, rest.Y-40)
	tt.Frame()
	chosen := label()
	chosenInk := panelInk(tt, chosen)
	if want := panelOver(tokens.Theme.Text, panelSurface(tt, chosen)); !panelClose(chosenInk, want, 20) {
		t.Errorf("the chosen entry's ink = %v, want --text-primary (%v)", chosenInk, want)
	}
	if channelDelta(chosenInk, restInk) < 40 {
		t.Errorf("the chosen page kept the resting ink: rest %v → chosen %v", restInk, chosenInk)
	}
}

// panelInk is the colour a label is actually drawn in: the pixel of its box
// farthest from the plane it sits on (antialiased edges sit between the two).
func panelInk(tt *ui.Tester, r ui.Rect) ui.Color {
	img := tt.Image()
	surface := panelSurface(tt, r)
	var ink ui.Color
	best := -1
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := y*img.Stride + x*4
			c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			if d := channelDelta(c, surface); d > best {
				best = d
				ink = c
			}
		}
	}
	return ink
}

// panelSurface is the most common colour in a label's box: the plane the
// glyphs sit on.
func panelSurface(tt *ui.Tester, r ui.Rect) ui.Color {
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
	return mode
}

// panelOver draws one straight-alpha token over an opaque plane, the way the
// renderer paints a text colour onto its surface.
func panelOver(over, base ui.Color) ui.Color {
	al := float32(over.A) / 255
	mix := func(o, b uint8) uint8 { return uint8(float32(o)*al + float32(b)*(1-al) + 0.5) }
	return ui.Color{R: mix(over.R, base.R), G: mix(over.G, base.G), B: mix(over.B, base.B), A: 255}
}

// panelClose reports whether two pixels agree within tol on every channel.
func panelClose(a, b ui.Color, tol int) bool {
	return channelDelta(a, b) <= tol
}
