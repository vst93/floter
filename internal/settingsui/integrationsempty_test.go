package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The Integrations page's empty list is the reference's shared empty region,
// not a bare muted line: `ExtensionsPanel.tsx:3177-3191` routes both its
// first-load and its empty placeholders through `SettingsEmpty`, and the empty
// arm (`:2462`) draws `<Package size={20} strokeWidth={2}/>` over
// `settings.extensions.emptyInstalled`. The primitive puts the title in
// `--text-secondary` on the 92u list floor (`settings.css:633-676`).
//
// The old port drew one `TextMuted` line with no mark and no floor, so this
// pins the page's own wiring: the reference's word, the Package mark, and the
// title's own ink.
func TestTheIntegrationsEmptyStateIsTheReferenceEmptyRegion(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration { return nil }

	// The region resolves its own colors from the theme the app draws in;
	// capture the same tokens from that context rather than a second one. The
	// old line wore the framework theme's `TextMuted`; the reference primitive
	// wears the palette's `TextSecondary` (`theme.Tokens`).
	var secondary, muted ui.Color
	var iconSize float32
	tt := ui.NewTester(func(c *ui.Context) {
		tokens := a.tokens(c)
		secondary, muted = tokens.TextSecondary, tokens.Theme.TextMuted
		iconSize = c.Theme().Space(5) // the reference's 20px Package mark
		a.View(c)
	}, 720, 760)
	tt.Frame()
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	title, ok := tt.Find(en.IntegrationsEmpty)
	if !ok {
		t.Fatalf("the integrations empty state is missing: %q", tt.Texts())
	}
	if tt.HasText("No integrations are installed.") {
		t.Errorf("the old self-made empty sentence is still drawn: %q", tt.Texts())
	}

	// The mark is the Package glyph above the title. The region's own 24u top
	// padding keeps that band clear of the section above it, so the only ink
	// in the 34u above the title is the icon.
	midX := title.X + title.W/2
	bg, _ := modalColor(tt, title)
	top, bottom := inkBand(tt, midX, title.Y-34, title.Y-1, bg, 24)
	if top < 0 {
		t.Fatalf("no glyph ink above the empty title: the 20u Package mark is missing")
	}
	if h := bottom - top + 1; h < iconSize*0.5 || h > iconSize+2 {
		t.Errorf("glyph ink height = %v, want the reference's 20u mark (%v)", h, iconSize)
	}

	// The title is `--text-secondary`, the reference primitive's own step —
	// measurably apart from the `--text-muted` the old line wore.
	ink := inkColor(tt, title)
	want := overSurface(secondary, bg)
	if !colorClose(ink, want, 14) {
		t.Errorf("title ink = %v, want the reference's --text-secondary over the card (%v)", ink, want)
	}
	if old := overSurface(muted, bg); colorClose(ink, old, 14) {
		t.Errorf("title ink = %v wears the old muted step, not --text-secondary", ink)
	}
}

// The shared empty region's list floor is 92u (`settings.css:641`), and the
// integrations list is a list placeholder — it does not override it the way
// the Sessions page body does. The glyph is the caller's 20px Package mark.
func TestTheIntegrationsEmptyRegionHasTheListFloor(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})

	var ref ui.Handle
	var box ui.Rect
	var floor, icon float32
	tt := ui.NewTester(func(c *ui.Context) {
		floor, icon = c.Theme().Space(23), c.Theme().Space(5)
		a.emptyState(c, "package", en.IntegrationsEmpty, "", icon, 0, nil).Bind(&ref)
		box = c.Resolve(ref).Bounds()
	}, 720, 300)
	tt.Frame()

	if box.H < floor {
		t.Errorf("empty region height = %v, want the list floor 92u (%v)", box.H, floor)
	}
	if box.H > floor+1 {
		t.Errorf("empty region height = %v, want the 92u floor, not a taller box", box.H)
	}

	title, ok := tt.Find(en.IntegrationsEmpty)
	if !ok {
		t.Fatalf("the empty title is missing: %q", tt.Texts())
	}
	midX := box.X + box.W/2
	bg, _ := modalColor(tt, title)
	top, bottom := inkBand(tt, midX, title.Y-34, title.Y-1, bg, 24)
	if top < 0 {
		t.Fatalf("no glyph ink above the empty title: the 20u Package mark is missing")
	}
	if h := bottom - top + 1; h < icon*0.5 || h > icon+2 {
		t.Errorf("glyph ink height = %v, want the reference's 20u mark (%v)", h, icon)
	}
}

// overSurface is one straight-alpha color drawn over an opaque one, the way the
// renderer paints a token text color onto its surface.
func overSurface(over, base ui.Color) ui.Color {
	a := float32(over.A) / 255
	mix := func(o, b uint8) uint8 {
		return uint8(float32(o)*a + float32(b)*(1-a) + 0.5)
	}
	return ui.Color{R: mix(over.R, base.R), G: mix(over.G, base.G), B: mix(over.B, base.B), A: 255}
}
