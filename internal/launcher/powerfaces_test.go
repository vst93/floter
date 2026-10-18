package launcher

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// The power confirmation's two buttons are the reference's own chips, not the
// framework's filled pills.
//
// `.launcher-system-confirm__execute` / `__cancel` (`main:src/styles/
// launcher.css:783-830`) are both quiet 24u chips — an 8u side padding pair, a
// hairline and the xs corner — and only the *execute* one paints a plane:
// `--glass-raised-warm` (an alias of `--system-icon-surface`, `base.css:611`)
// under `--glass-raised-shadow`. The cancel is transparent until hovered. The
// port had drawn both with the framework's `ui.Button`, whose theme fill is the
// generic grey control pane: the execute chip did not read as the warm primary
// and the pair did not read as the reference's chips.
//
// Mutation check: put `ui.Button` back for the execute chip and its fill lands
// on the theme's grey control pane instead of the warm plane.
func TestThePowerConfirmButtonsWearTheirOwnFaces(t *testing.T) {
	a := testApp()
	a.Actions.Power = func(string) {}
	tt := render(t, a)
	tt.Type("restart")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()

	tokens := theme.ForPreferences(a.settings(), false, ui.Preferences{})
	img := tt.Image()
	// The band's own blank plane, sampled between the glyph and the sentence:
	// what the execute chip's plane composites over.
	msg, ok := tt.Find(a.copy().PowerConfirmMessage(PowerRestart))
	if !ok {
		t.Fatalf("the confirmation is not drawn: %q", tt.Texts())
	}
	band := img.RGBAAt(int(msg.X)-2, int(msg.Y+msg.H/2))
	// The execute chip's plane is the *opaque* warm pane over the band:
	// `--system-icon-surface` composited to opacity, which is visibly warmer
	// (a different hue) than the translucent band it sits on.
	warm := inkOver(tokens.IconSurface, band)
	execute, ok := tt.Find(a.copy().PowerRestartAction)
	if !ok {
		t.Fatalf("the execute button is missing: %q", tt.Texts())
	}
	// Sample inside the chip, above the label's line box so no glyph ink
	// interferes.
	got := img.RGBAAt(int(execute.X+execute.W/2), int(execute.Y+3))
	if !nearRGB(got, warm, 8) {
		t.Errorf("the execute chip's plane = %v, want the warm raised pane %v", got, warm)
	}
	// The cancel chip is transparent (`background: transparent`): its fill is
	// the band itself.
	cancel, ok := tt.Find(a.copy().PowerCancel)
	if !ok {
		t.Fatalf("the cancel button is missing: %q", tt.Texts())
	}
	if got := img.RGBAAt(int(cancel.X+cancel.W/2), int(cancel.Y+3)); !nearRGB(got, band, 4) {
		t.Errorf("the cancel chip paints a plane: %v, want the band's own %v", got, band)
	}
}

// The warning feedback row's *fill* is the resting warm plane, not the token's
// hover rung.
//
// `.launcher-feedback--warning` (`main:src/styles/launcher.css:711-716`) sets
// `background: var(--glass-raised-warm)`, which base.css declares as an **alias
// of `--system-icon-surface`** (`base.css:611`) — the same amber plane the plain
// `.launcher-feedback` already wears (`:705`). Only the keyline, the raised
// shadow and the weight step up. The port had lifted the fill to the token's
// hover rung (`--system-icon-surface-hover`, `base.css:612`), a shade too strong
// for a row that is not being pointed at.
//
// Mutation check: swap `RaisedWarm` for `IconSurfaceHover` and the reading lands
// on the hover composite instead.
func TestTheWarningFeedbackRowPaintsTheRestingWarmPlane(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	a.WarnFeedback("boom")
	// The row enters over 160ms (`feedbackRow`'s Enter transition), so the frame
	// has to settle before the fill is read: a mid-animation frame still shows
	// the panel through it.
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	msg, ok := tt.Find("boom")
	if !ok {
		t.Fatalf("the warning row is not drawn: %q", tt.Texts())
	}
	img := tt.Image()
	// The row's own blank plane, sampled just right of the sentence: the
	// sentence grows to fill the row, so the pixels past it are the row's fill
	// with no glyph ink over them.
	got := img.RGBAAt(int(msg.X+msg.W)+4, int(msg.Y+msg.H/2))
	// The panel the row sits on: sampled well above the docked band, in the
	// result list's own recess.
	panel := img.RGBAAt(int(msg.X+msg.W)+4, int(msg.Y)-80)
	tokens := theme.ForPreferences(a.settings(), false, ui.Preferences{})
	want := inkOver(tokens.RaisedWarm, panel)
	hover := inkOver(tokens.IconSurfaceHover, panel)
	if !nearRGB(got, want, 3) {
		t.Errorf("the warning row's fill = %v, want the resting warm plane %v", got, want)
	}
	if nearRGB(got, hover, 3) {
		t.Errorf("the warning row wears the warm *hover* rung %v, not the resting plane", hover)
	}
}

// The status note's `info` mark is the reference's literal 14, not a `--u` step.
//
// `.launcher-status__icon` drew `<InfoIcon size={14}/>`
// (`main:src/launcher/LauncherResults.tsx:752-754`): 14px is the mark's own
// number and does not ride `--ui-scale`, while the 28u column around it does.
// The port had drawn the glyph at `Space(4)` (16u), two units wider and, worse,
// scaling with the interface step — so at the shipped step it read 14.4 and at
// the Large step 17.6.
//
// Mutation check: put `Space(4)` back and the drawn mark measures wider than 14
// at the shipped step.
func TestTheStatusNoteMarkIsTheReferenceLiteralFourteen(t *testing.T) {
	a, _ := clipboardApp(t)
	clipboardOff(t, a)
	a.EnterClipboard()
	tt := render(t, a)
	tt.Frame()
	sentence, ok := tt.Find(a.copy().ClipboardDisabled)
	if !ok {
		t.Fatalf("the note is not drawn: %q", tt.Texts())
	}
	// The mark is the only ink in the note's own icon column, which starts at
	// the note's left inset (12u) and is 28u wide. Measure the ink's bounding
	// box there.
	inset := float32(12)
	column := ui.Rect{X: inset, Y: sentence.Y, W: 28, H: sentence.H}
	w, h := inkBounds(tt, column)
	if w < 11 || w > 15 || h < 11 || h > 15 {
		t.Errorf("the status mark measures %vx%v, want the reference's literal 14", w, h)
	}
}

// inkBounds returns the width and height of the non-background ink in a box:
// the drawn glyph's own bounding box, in pixels.
func inkBounds(tt *ui.Tester, r ui.Rect) (int, int) {
	img := tt.Image()
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	// The plane is the box's most common colour; anything a few levels off it
	// is glyph ink (the mark is drawn in a muted register well below the plane).
	plane := imgPlane(tt, r)
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if channelDistance(img.RGBAAt(x, y), plane) < 12 {
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
	if maxX < 0 {
		return 0, 0
	}
	return maxX - minX + 1, maxY - minY + 1
}

// The row's pin control takes the reference's press, which the port never gave
// it.
//
// `.launcher-result__pin` is literally `.collapsed-card__settings` on a result
// row (`LauncherResults.tsx:1001-1005`), so it wears that control's `:active`
// rule (`main:src/styles/launcher.css:407-411`): `transform: scale(0.92)` over
// `--glass-control-press`, its shadow gone — the same press the field row's four
// trailing controls take. The port had drawn the pin with a hover fill and no
// press at all, so it was the one control in the row that did not sink under the
// pointer.
//
// Mutation check: drop the press branch and the held control keeps its full 20u
// box.
func TestTheRowPinSinksLikeTheSharedFieldControl(t *testing.T) {
	a, _ := clipboardApp(t)
	a.Actions.PinText = func(string, string) {}
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	pin, ok := tt.Find(a.copy().HistoryPin)
	if !ok {
		t.Fatalf("the pin control is not drawn: %q", tt.Texts())
	}
	if pin.W < 19.5 || pin.W > 20.5 {
		t.Fatalf("the pin's resting box is %.1f wide, want the reference's 20u", pin.W)
	}
	tt.Press(pin.X+pin.W/2, pin.Y+pin.H/2)
	tt.Frame()
	held, ok := tt.Find(a.copy().HistoryPin)
	if !ok {
		t.Fatal("the pin left the row while held")
	}
	if held.W >= pin.W {
		t.Errorf("the held pin did not sink: box %.1f wide, rest %.1f", held.W, pin.W)
	}
	tt.Release(pin.X+pin.W/2, pin.Y+pin.H/2)
}

// A favourited history row's star is painted in, not just outlined.
//
// The reference fills the same lucide Star when the entry is starred:
// `<StarIcon size={13} fill={favoriteEntry.favorite ? "currentColor" : "none"}/>`
// (`main:src/launcher/LauncherResults.tsx:987-991`). The port drew one
// stroke-only mark in both states, so a starred entry's control read as "not
// starred" at 13px — the fill is the whole statement at that size.
//
// The two states are compared on the same control: the painted star covers
// strictly more of its box than the outline does.
func TestTheFavouritedStarIsPaintedIn(t *testing.T) {
	coverage := func(favorite bool) int {
		a, store := clipboardApp(t)
		if favorite {
			entries := store.Entries()
			store.SetFavorite(entries[0].ID, true)
		}
		tt := render(t, a)
		tt.Type("clipboard")
		tt.Frame()
		tt.TypeKey(0, ui.KeyEnter, "")
		tt.Frame()
		// The rows enter over 120ms (`rowTransition`) on the tester's own
		// clock, which stands still until it is walked.
		tt.Advance(200 * time.Millisecond)
		name := "Favorite"
		if favorite {
			name = "Unfavorite"
		}
		star, ok := tt.Find(name)
		if !ok {
			t.Fatalf("the star is not drawn (favorite=%v): %q", favorite, tt.Texts())
		}
		plane := tt.Image().RGBAAt(int(star.X)-6, int(star.Y+star.H/2))
		covered := 0
		for y := int(star.Y); y < int(star.Y+star.H); y++ {
			for x := int(star.X); x < int(star.X+star.W); x++ {
				if channelDistance(tt.Image().RGBAAt(x, y), plane) > 25 {
					covered++
				}
			}
		}
		return covered
	}
	outline := coverage(false)
	painted := coverage(true)
	if painted <= outline {
		t.Errorf("the favourited star covers %d px, the outline %d — it is not painted in", painted, outline)
	}
}
