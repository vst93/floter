package terminalui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P11-bt · **一帧只解析一份调色板** —— 终端页。
//
// The settings surface got this memo in P11-bs: `theme.ForPreferences` normalizes
// the settings, resolves the appearance, builds a whole `ui.Theme` and lays the
// palette over it, and that surface asked for it once per row. The terminal
// surface asks from fewer places but on a hotter frame — it redraws on every byte
// the program writes — and it had the same shape: `tokensOf` (thirteen call
// sites), plus two notes that called `theme.ForPreferences` directly, each with
// its own `Store.Snapshot`.
//
// The arms below are the properties the memo has to have: one build per frame;
// a hit is the palette a cold reader builds from the same inputs (so no pixel
// moved); and a change to the settings, or to the appearance, builds again — a
// preview slider cannot be served a stale palette.
func TestOnePalettePerTerminalFrame(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{Settings: func() {}, Close: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, "floter — zsh")
	tt := render(t, a, 800, 600)

	// `render` drew one frame; every frame after it draws from the same memo.
	if got := a.tokenMemo.Builds(); got != 1 {
		t.Fatalf("the first frame built %d palettes, want 1", got)
	}
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if got := a.tokenMemo.Builds(); got != 1 {
		t.Fatalf("three more frames left the memo at %d builds, want 1", got)
	}

	// A hit is the value the miss would have built: a reader with a cold memo,
	// given the same context, builds the same palette.
	warm, cold := framePalettes(t, a, New(store, Actions{}, nil))
	// Compared field by field, not with %v on the whole value: `Tokens.Theme` is
	// a pointer to the theme the frame was given, and its address differs.
	palette := func(t theme.Tokens) string {
		return fmt.Sprintf("%v|%v|%v|%v|%v|%v|%v|%v|%v|%v",
			t.Appearance, t.Scale, t.Theme.Background, t.Control, t.ControlEdge,
			t.InputStroke, t.TextSecondary, t.HairlineSoft, t.FieldFill, t.RowSelected)
	}
	if palette(warm) != palette(cold) {
		t.Errorf("the memo served a palette the frame would not have built")
	}

	// The key is exact: a change to the stored settings is a new version, and
	// the next frame builds again rather than serving the old palette.
	if err := store.Update(func(s *settings.Settings) { s.HideOnBlur = !s.HideOnBlur }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.tokenMemo.Builds(); got != 2 {
		t.Fatalf("a settings change left the memo at %d builds, want 2", got)
	}

	// The same for the appearance: dark and light are two different palettes.
	tt.SetDark(true)
	tt.Frame()
	if got := a.tokenMemo.Builds(); got != 3 {
		t.Fatalf("the appearance change left the memo at %d builds, want 3", got)
	}
}

// framePalettes draws one frame of a and asks both readers for their palette
// inside that build, the way the drawing code asks.
func framePalettes(t *testing.T, a, cold *App) (warm, fresh theme.Tokens) {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
		warm = tokensOf(a, c)
		// A reader with a cold memo, on the same context: what a miss builds.
		fresh = tokensOf(cold, c)
	}, 800, 600)
	tt.Frame()
	return warm, fresh
}

// BenchmarkTerminalFrame measures one frame of the terminal surface: the panel,
// the canvas and the bar over a held session, the frame that redraws on every
// byte the program writes.
func BenchmarkTerminalFrame(b *testing.B) {
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{Settings: func() {}, Close: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, "floter — zsh")
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 800, 600)
	tt.Frame()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Frame()
	}
}

// A swapped store is a new palette. Both stores begin at version zero, so the
// version alone cannot tell them apart: the reader notices the swap by the
// pointer and forgets what it built, exactly as the settings surface's memo
// does (P11-bs). Without that, a surface handed a new store would keep drawing
// the old one's colours.
func TestASwappedStoreIsANewPalette(t *testing.T) {
	darkSettings := settings.Default()
	darkSettings.Theme = "dark"
	light := settings.NewStore(settings.Default())
	dark := settings.NewStore(darkSettings)

	a := New(light, Actions{Settings: func() {}, Close: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, "floter — zsh")
	tt := render(t, a, 800, 600)
	if got := a.tokenMemo.Builds(); got != 1 {
		t.Fatalf("the first frame built %d palettes, want 1", got)
	}

	a.Store = dark
	tt.Frame()
	// The swap forgets what the memo built (`Reset`), so the frame after it
	// builds the new store's palette — one build, not the old store's hit.
	if got := a.tokenMemo.Builds(); got != 1 {
		t.Fatalf("a swapped store left the memo at %d builds, want 1", got)
	}
	warm, cold := framePalettes(t, a, New(dark, Actions{}, nil))
	palette := func(t theme.Tokens) string {
		return fmt.Sprintf("%v|%v|%v", t.Appearance, t.Theme.Background, t.Control)
	}
	if palette(warm) != palette(cold) {
		t.Errorf("the swapped store served the old store's palette: %s, want %s",
			palette(warm), palette(cold))
	}
}
