package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"floter/internal/theme"
)

// The reference's action control is not the framework's filled button. R38's
// `.plugin-config-action__button` (`plugin-config.css:223-260`) is a quiet
// hairline pill: a 28u height, 10u of side padding, a `--glass-control-edge`
// hairline, a `999px` corner, the `--glass-control` plane and the
// `--text-caption` step; the danger variant takes the warning colour on its
// hairline and ink over a transparent plane. The old port drew a framework
// `ui.Button` instead — the flat control pane, 14u of side padding, a neutral
// border and the body step — so the destructive command read as the overlay's
// primary action. Rendering the face on its own reads its box and its ink
// without the surrounding sheet.
func TestThePluginActionButtonIsTheReferenceHairlinePill(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	store := newStore(t)
	a := New(store, Actions{})
	tokens := theme.ForPreferences(store.Snapshot(), false, ui.Preferences{})

	var ref ui.Handle
	var box ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.RGB(0, 0, 0))
		ui.Column(c).Padding(40, 40).Children(func() {
			ui.Row(c).Children(func() {
				a.sheetActionButton(c, "Clear history", true, func() {}).Bind(&ref)
			})
		})
		box = c.Resolve(ref).Bounds()
	}, 400, 200)
	tt.Frame()

	// The box is the reference's 28u control, not the framework Button's
	// padding-driven height.
	if box.H < 27 || box.H > 29 {
		t.Errorf("action height = %v, want the reference's 28u", box.H)
	}
	label, ok := tt.Find("Clear history")
	if !ok {
		t.Fatalf("the action label is missing: %q", tt.Texts())
	}
	// The row's own content box is the control's 10u side padding: the label
	// sits inside it with the focus ring's one extra pixel of gap.
	if want := tokens.Theme.Space(2.5); absFloat(label.X-box.X-want) > 2 {
		t.Errorf("leading inset = %v, want the reference's 10u (%v)", label.X-box.X, want)
	}
	if right := box.X + box.W - (label.X + label.W); absFloat(right-tokens.Theme.Space(2.5)) > 2 {
		t.Errorf("trailing inset = %v, want the reference's 10u (%v)", right, tokens.Theme.Space(2.5))
	}

	// The danger hairline is the warning colour, not the neutral
	// `--glass-control-edge` a framework Button would draw: sample the top
	// edge's midpoint.
	top := sample(tt, box.X+box.W/2, box.Y+0.5)
	if int(top.R) <= int(top.G) || int(top.R) <= int(top.B) {
		t.Errorf("the danger hairline is not warning ink: %v", top)
	}

	// The danger label's ink is the warning colour too (the framework Button
	// would paint `--text-primary`, a neutral). Scan the label's own rows for
	// the pixel that is furthest into the warm end of the spectrum, and read
	// its hue: warning ink has red above blue.
	warmest := ui.Color{}
	best := -256
	for y := int(label.Y); y < int(label.Y+label.H); y++ {
		for x := int(label.X); x < int(label.X+label.W); x++ {
			p := sample(tt, float32(x), float32(y))
			if d := int(p.R) - int(p.B); d > best {
				best, warmest = d, p
			}
		}
	}
	if best <= 0 {
		t.Errorf("the danger label is not warning ink: %v (warmth %d)", warmest, best)
	}

	// The corner is a pill, not a small rounded rect: the `999px` curve
	// pulls the hairline in from the box's left edge at the box's own first
	// row, while the framework's `--radius-xs` (6u) corner sit almost flush
	// with it at that height.
	leftmost := float32(-1)
	for x := int(box.X); x < int(box.X+box.W); x++ {
		if sample(tt, float32(x), box.Y+0.5).R > 30 {
			leftmost = float32(x)
			break
		}
	}
	if leftmost < 0 {
		t.Fatalf("no hairline ink found in the button's top row")
	}
	if inset := leftmost - box.X; inset < 5 {
		t.Errorf("the corner inset at the top row = %v, want a pill's wide curve", inset)
	}
}

// The live sheet's two faces, driven through the frame the app really draws:
// the resting row offers the destructive command as a danger hairline pill,
// and the armed row asks the reference's own question. The cluster is read by
// pixel so a framework `ui.Button` (a neutral, near-square, body-step control)
// cannot pass. The root is painted black so the translucent danger hairline
// composites to its own warm hue instead of over the frame's glass.
func TestThePluginActionClusterIsWarmAndAsksBeforeItActs(t *testing.T) {
	copy := i18n.For("en").Settings
	schema, values := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
	a := New(newStore(t), Actions{})
	armed := ""
	sheet := PluginSheet{
		Schema: schema, Values: values,
		Arm:   func(key string) { armed = key },
		Unarm: func(key string) { armed = "" },
	}
	render := func() *ui.Tester {
		tt := ui.NewTester(func(c *ui.Context) {
			c.Root().Background(ui.RGB(0, 0, 0))
			a.PluginSheet(c, sheet)
		}, 720, 700)
		tt.Frame()
		return tt
	}

	tt := render()
	label, ok := tt.Find(copy.ClipboardClear)
	if !ok {
		t.Fatalf("the action is missing: %q", tt.Texts())
	}
	// The danger pill's hairline is the warmest ink to the label's right — the
	// framework Button's neutral border has no warm side at all.
	best := -256
	for x := int(label.X+label.W) + 1; x < int(label.X+label.W)+40; x++ {
		p := sample(tt, float32(x), label.Y+label.H/2)
		if d := int(p.R) - int(p.B); d > best {
			best = d
		}
	}
	if best < 30 {
		t.Errorf("the action's hairline is not warm (warmth %d)", best)
	}

	// The first press arms it; the second frame asks the question.
	if err := tt.Click(copy.ClipboardClear); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if armed != "clear_history" {
		t.Fatalf("armed = %q", armed)
	}
	sheet.Armed = armed
	tt = render()
	if !tt.HasText(copy.ClipboardClearConfirm) {
		t.Errorf("the armed row does not ask %q: %q", copy.ClipboardClearConfirm, tt.Texts())
	}
	if copy.ClipboardClearConfirm == copy.ClipboardClearButton {
		t.Error("the confirmation word is the button's own word")
	}
}

// R59's `.plugin-config-text` (`plugin-config.css:97-118`) is a 28u slot cut
// into the glass: a `--glass-field` fill, a `--glass-control-edge` hairline, a
// `--radius-sm` corner, the body step and 8u of side padding. The old port
// handed the field to the framework's `ui.TextInput`, whose own padded
// surface is none of those numbers. The live browser sheet is the one schema
// with a `text` field, so the row is read there — its label and its field box
// by pixel.
func TestThePluginTextFieldIsTheReferenceGlassSlot(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	copy := i18n.For("en").Settings
	store := newStore(t)
	a := New(store, Actions{})
	schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(), nil)
	sheet := PluginSheet{Schema: schema, Values: values}
	tt := renderSheet(t, a, sheet, 720, 1400)

	if !tt.HasText(copy.BrowserCustomDir) {
		t.Fatalf("the text field's label is missing: %q", tt.Texts())
	}
	hint, ok := tt.Find(copy.BrowserCustomDirHint)
	if !ok {
		t.Fatalf("the text field's help is missing: %q", tt.Texts())
	}
	// The field is a stacked row: its slot sits under the label and its help,
	// so scan a column below the help for the recessed plane — the run of
	// rows noticeably brighter than the card's own `--glass-control` surface.
	card := sample(tt, 300, hint.Y-4)
	rows := []int{}
	for y := int(hint.Y+hint.H) + 2; y < int(hint.Y+hint.H)+80; y++ {
		if p := sample(tt, 300, float32(y)); int(p.R)-int(card.R) > 40 {
			rows = append(rows, y)
		}
	}
	if len(rows) == 0 {
		t.Fatalf("no field slot under %q's help: %q", copy.BrowserCustomDir, tt.Texts())
	}
	if h := rows[len(rows)-1] - rows[0] + 1; h < 27 || h > 29 {
		t.Errorf("the field slot's height = %v, want the reference's 28u", h)
	}
	// The slot is a recessed plane of its own, not the row's surface: its
	// fill differs from the card the rows sit on.
	if delta := channelDelta(card, sample(tt, 300, float32(rows[1]))); delta < 40 {
		t.Errorf("the field slot is not a distinct plane: card %v (delta %d)", card, delta)
	}
}
