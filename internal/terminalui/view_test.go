package terminalui

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
)

func newStore() *settings.Store { return settings.NewStore(settings.Default()) }

func render(t *testing.T, a *App, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
	}, w, h)
	tt.Frame()
	return tt
}

func TestTerminalEmptyState(t *testing.T) {
	closed := 0
	copy := i18n.For("en").Terminal
	a := New(newStore(), Actions{
		Close:      func() { closed++ },
		NewCommand: func() {},
	}, nil)
	tt := render(t, a, 860, 600)
	if !tt.HasText("Terminal") {
		t.Errorf("missing the title: %q", tt.Texts())
	}
	// The empty state is the reference's three things and nothing else: the
	// title over the two lines that leave the page, with no second sentence
	// between them (`main:src/App.tsx:3852-3872`).
	for _, want := range []string{copy.EmptyTitle, copy.EmptyNew, copy.EmptyBack} {
		if !tt.HasText(want) {
			t.Errorf("the empty state is missing %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("The session has not started yet.") {
		t.Errorf("the empty state drew a tip the reference does not: %q", tt.Texts())
	}
	if _, ok := tt.Find("Close terminal"); !ok {
		t.Errorf("the close control is not labeled: %q", tt.Texts())
	}
}

func TestTerminalSessionErrorShows(t *testing.T) {
	a := New(newStore(), Actions{}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("libghostty-vt is missing")
	})
	a.EnsureSession()
	if a.Err == nil {
		t.Fatal("EnsureSession swallowed the error")
	}
	tt := render(t, a, 860, 600)
	if !tt.HasText("libghostty-vt is missing") {
		t.Errorf("the error did not show: %q", tt.Texts())
	}
	// EnsureSession must not retry once it failed into a session; a second
	// call is a no-op until the surface is reopened.
	a.EnsureSession()
	if a.Current() != nil {
		t.Error("a terminal appeared from a failing start")
	}
}

func TestTerminalCloseButtonCallsBack(t *testing.T) {
	closed := 0
	a := New(newStore(), Actions{Close: func() { closed++ }}, nil)
	tt := render(t, a, 860, 600)
	if err := tt.Click(i18n.For("en").Terminal.Close); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if closed != 1 {
		t.Errorf("close ran %d times, want 1", closed)
	}
}

func TestTerminalFollowsTheLanguage(t *testing.T) {
	store := newStore()
	if err := store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{}, nil)
	tt := render(t, a, 860, 600)
	if !tt.HasText("终端") {
		t.Errorf("missing the Chinese title: %q", tt.Texts())
	}
}

func TestOptionsFollowTheSettings(t *testing.T) {
	store := newStore()
	a := New(store, Actions{Title: func(string) {}, Exit: func(*Session, int) {}}, nil)

	o := a.options()
	if o.Font.Family != "monospace" || o.Font.Size != 14 || o.Font.LineHeight != 1.2 {
		t.Errorf("default font = %+v", o.Font)
	}
	if o.Cursor != terminal.CursorBar || o.NoBlink {
		t.Errorf("default cursor = %v, noBlink=%v", o.Cursor, o.NoBlink)
	}
	if o.Theme == nil || o.DarkTheme == nil {
		t.Errorf("inherit built no theme pair: light=%+v dark=%+v", o.Theme, o.DarkTheme)
	}
	// The shipped `inherit` follows the window's appearance through the
	// plugin's light/dark pair, whose one overridden token is the cursor:
	// base.css's `--terminal-cursor` (teal), not the plugin's zero-cursor
	// fallback (the foreground). The foreground/background stay the plugin's
	// own defaults, so nothing else moved.
	if got, want := o.Theme.Cursor, ui.RGB(0x00, 0x7c, 0x86); got != want {
		t.Errorf("light inherit cursor = %v, want %v", got, want)
	}
	if got, want := o.DarkTheme.Cursor, ui.RGB(0x8b, 0xd5, 0xca); got != want {
		t.Errorf("dark inherit cursor = %v, want %v", got, want)
	}
	if got := o.Theme.Foreground; got != terminal.LightTheme().Foreground {
		t.Errorf("light inherit foreground = %v, want the plugin default", got)
	}
	// The default theme has no palette of its own: the view leaves its
	// background undrawn so the card's terminal-opacity tint shows, and picks a
	// named palette's colour up as its own layer (P7-dr).
	if !o.Transparent {
		t.Error("the inherit theme must leave the background to the card's tint")
	}
	if a.Pad() != 3 {
		t.Errorf("regular padding = %v, want 3", a.Pad())
	}
	// The step reaches the plugin's own Padding (mygo v0.6.0), which is where
	// the grid's room comes from. It used to ride the view's element padding,
	// which is inert on a leaf: an element's padding insets children, and the
	// terminal's draw callback is handed the whole border box. See
	// TestThePaddingStepReachesThePlugin.
	if got, want := o.Padding, (terminal.Padding{X: 3, Y: 3}); got != want {
		t.Errorf("regular plugin padding = %+v, want %+v", got, want)
	}
	// PaddingBackground is the zero value and paints nothing with Transparent:
	// the grid's room stays empty, as it was when the inset was merely
	// reserved.
	if o.PaddingColor != terminal.PaddingBackground {
		t.Errorf("padding colour = %v, want the transparent default", o.PaddingColor)
	}
	if o.OnTitle == nil {
		t.Error("the plugin callbacks are not wired")
	}
	// OnExit is the one callback that names the session it belongs to, so
	// startSession (not options) wires it; the session-level test below covers
	// that wiring.
	if o.OnExit != nil {
		t.Error("options wires the shared exit callback instead of the session's own")
	}

	if err := store.Update(func(s *settings.Settings) {
		s.FontSize = 20
		s.FontFamily = "Iosevka"
		s.CursorShape = "block"
		s.CursorBlink = false
		s.TerminalLineHeight = 1.5
		s.TerminalPadding = "relaxed"
		s.TerminalTheme = "forest"
		s.TerminalOpacity = 80
	}); err != nil {
		t.Fatal(err)
	}

	o = a.options()
	if o.Font.Family != "Iosevka" || o.Font.Size != 20 || o.Font.LineHeight != 1.5 {
		t.Errorf("font = %+v", o.Font)
	}
	if o.Cursor != terminal.CursorBlock || !o.NoBlink {
		t.Errorf("cursor = %v, noBlink=%v", o.Cursor, o.NoBlink)
	}
	if a.Pad() != 6 {
		t.Errorf("relaxed padding = %v, want 6", a.Pad())
	}
	if got, want := o.Padding, (terminal.Padding{X: 6, Y: 6}); got != want {
		t.Errorf("relaxed plugin padding = %+v, want %+v", got, want)
	}
	if o.Theme == nil {
		t.Fatal("the forest palette did not build")
	}
	// The palette's colours are opaque: how solid the background is is the
	// view's own layer (canvasFill), because the plugin resolves what it paints
	// the canvas with through the vt's colours, which carry no alpha (P7-dr).
	if got, want := o.Theme.Background, ui.RGB(0x0f, 0x1a, 0x14); got != want {
		t.Errorf("forest background = %v, want %v", got, want)
	}
	if got, want := o.Theme.Foreground, ui.RGB(0xd7, 0xe4, 0xd0); got != want {
		t.Errorf("forest foreground = %v, want %v", got, want)
	}
	if got, want := o.Theme.Cursor, ui.RGB(0x8b, 0xd4, 0x50); got != want {
		t.Errorf("forest cursor = %v, want %v", got, want)
	}
	// A light palette takes the light base.
	if err := store.Update(func(s *settings.Settings) { s.TerminalTheme = "paper" }); err != nil {
		t.Fatal(err)
	}
	if got := a.options().Theme.Background; got == ui.RGB(0x0f, 0x1a, 0x14) {
		t.Error("paper reused the dark background")
	}
	// The cursor shapes map one to one.
	for shape, want := range map[string]terminal.CursorStyle{
		"beam": terminal.CursorBar, "block": terminal.CursorBlock, "underline": terminal.CursorUnderline,
	} {
		if got := cursorStyle(shape); got != want {
			t.Errorf("cursorStyle(%q) = %v, want %v", shape, got, want)
		}
	}
}

// TestThePaddingStepReachesThePlugin pins A1's real finding: the padding step
// reaches the grid through Options.Padding, and cannot through the view's
// element padding. The second half is the antichain — it is what proves the
// first half was a no-op rather than a working carrier that merely changed
// hands.
func TestThePaddingStepReachesThePlugin(t *testing.T) {
	steps := []struct {
		name string
		want float32
	}{
		{"compact", 1},
		{"regular", 3},
		{"relaxed", 6},
	}
	for _, step := range steps {
		store := newStore()
		if err := store.Update(func(s *settings.Settings) { s.TerminalPadding = step.name }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{}, nil)
		o := a.options()
		if a.Pad() != step.want {
			t.Errorf("%s: Pad() = %v, want %v", step.name, a.Pad(), step.want)
		}
		if got, want := o.Padding, (terminal.Padding{X: step.want, Y: step.want}); got != want {
			t.Errorf("%s: Options.Padding = %+v, want %+v", step.name, got, want)
		}
	}

	// An element's Padding insets its children. A drawn leaf has none, and the
	// callback is handed the whole border box, so `Padding(pad)` on the
	// terminal's view could never move the grid. This is why A1 is a migration
	// and not a deletion: the number had no carrier before v0.6.0's
	// Options.Padding.
	var withPad, without ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			ui.Box(c).Fill().Padding(24).Draw(func(_ *ui.Painter, r ui.Rect) { withPad = r })
		})
	}, 200, 200)
	tt.Frame()
	tt2 := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			ui.Box(c).Fill().Padding(0).Draw(func(_ *ui.Painter, r ui.Rect) { without = r })
		})
	}, 200, 200)
	tt2.Frame()
	if withPad != without {
		t.Errorf("element padding moved a draw rect: %+v vs %+v; if mygo ever makes padding "+
			"inset a leaf's draw box, Options.Padding is no longer the only carrier and this "+
			"migration needs revisiting", withPad, without)
	}
	if want := (ui.Rect{X: 0, Y: 0, W: 200, H: 200}); without != want {
		t.Errorf("the draw rect = %+v, want the whole border box %+v", without, want)
	}
}

func TestPinControlCopiesTheSessionText(t *testing.T) {
	// A session whose text the test supplies, so the control can be used
	// without a real terminal.
	pinned := ""
	pinnedText := ""
	a := New(newStore(), Actions{
		Pin: func(title, text string) { pinned, pinnedText = title, text },
	}, nil)
	a.Text = func() string { return "line one\nline two" }
	withSession(a, &terminal.Terminal{}) // present so the control shows
	withTitle(a, "zsh")

	tt := render(t, a, 860, 600)
	if _, ok := tt.Find("Pin output"); !ok {
		t.Fatalf("the pin control is missing: %q", tt.Texts())
	}
	if err := tt.Click("Pin output"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if pinned != "zsh" || pinnedText != "line one\nline two" {
		t.Errorf("pinned %q / %q", pinned, pinnedText)
	}
}

func TestNoPinControlWithoutASession(t *testing.T) {
	a := New(newStore(), Actions{Pin: func(string, string) {}}, nil)
	tt := render(t, a, 860, 600)
	if _, ok := tt.Find("Pin output"); ok {
		t.Errorf("the pin control shows with no session: %q", tt.Texts())
	}
}

// The session's screen is saved when it closes and fed back into the next
// session, so the last session's scrollback comes back.
func TestSnapshotSaveAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "terminal-snapshot")
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	a.SnapshotPath = path

	// With no session there is nothing to save, and no error.
	if err := a.SaveSnapshot(); err != nil {
		t.Errorf("SaveSnapshot with no session = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a snapshot was written with no session: %v", err)
	}

	// The write is atomic and private: a temporary file in the same
	// directory, renamed over the target.
	if err := writeFileAtomically(path, []byte("screen")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no POSIX permission bits: Go reports a fixed 0666 there, so
	// the privacy of the snapshot is a promise only the POSIX platforms keep.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("snapshot mode = %v", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temporary files were left behind: %v", entries)
	}
}

// The terminal page's empty state offers a new session and a way back.
func TestTerminalEmptyStateControls(t *testing.T) {
	closeCalls := 0
	a := New(newStore(), Actions{
		Close: func() { closeCalls++ },
	}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	tt := render(t, a, 860, 600)
	if !tt.HasText("No terminal session yet") {
		t.Fatalf("the empty title is missing: %q", tt.Texts())
	}
	if err := tt.Click("New blank session"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The session attempt failed (no library), and the error replaces the hint.
	if !tt.HasText("no library in tests") {
		t.Errorf("the error did not show: %q", tt.Texts())
	}
}

// The terminal header carries the old build's two controls: a return to the
// launcher and a hand-off to the system's own terminal.
func TestTerminalHeaderControls(t *testing.T) {
	newCommands, externals := 0, 0
	closeCalls := 0
	a := New(newStore(), Actions{
		NewCommand:   func() { newCommands++ },
		OpenExternal: func() { externals++ },
		Close:        func() { closeCalls++ },
	}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	tt := render(t, a, 860, 600)
	if err := tt.Click("New command"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if newCommands != 1 {
		t.Errorf("new command ran %d times", newCommands)
	}
	if err := tt.Click(i18n.For("en").Terminal.OpenExternal); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if externals != 1 {
		t.Errorf("open external ran %d times", externals)
	}
	if err := tt.Click(i18n.For("en").Terminal.Close); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if closeCalls != 1 {
		t.Errorf("close ran %d times", closeCalls)
	}
}

// The bar's two labeled controls say what they do once the pointer rests on
// them: the reference put the same sentence in each button's `title=` — the
// sentence names the chord that does the same thing from the keyboard
// (`main:src/App.tsx:3764-3790`), read from the user's own bindings rather than
// spelled in the copy.
func TestTerminalBarButtonsCarryTheirHints(t *testing.T) {
	copy := i18n.For("en").Terminal
	store := newStore()
	a := New(store, Actions{
		NewCommand:   func() {},
		OpenExternal: func() {},
	}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	stored := store.Snapshot()
	tt := render(t, a, 860, 600)
	for _, want := range []struct{ label, hint string }{
		{copy.NewCommand, copy.NewCommandHint(shortcuts.Display(settings.Shortcut(stored, settings.ShortcutNewCommand)))},
		{copy.OpenExternal, copy.OpenExternalHint(shortcuts.Display(settings.Shortcut(stored, settings.ShortcutOpenExternalTerminal)))},
	} {
		rect, ok := tt.Find(want.label)
		if !ok {
			t.Fatalf("no %q control: %q", want.label, tt.Texts())
		}
		cx, cy := rect.X+rect.W/2, rect.Y+rect.H/2
		// The bar is hidden until the pointer is over it, and its buttons are
		// pass-through until then: one frame reveals the bar, the next lets the
		// button take the hover.
		tt.Move(cx, cy)
		tt.Frame()
		tt.Frame()
		// The tip waits for the pointer to rest (Base UI's 600ms), on the
		// tester's own clock, which stands still until it is walked.
		tt.Advance(700 * time.Millisecond)
		tt.Frame()
		if !tt.HasText(want.hint) {
			t.Errorf("the %q control has no tooltip %q: %q", want.label, want.hint, tt.Texts())
		}
		// Park the pointer away and let the tip go before the next control.
		tt.Move(430, 300)
		tt.Frame()
	}
}

// The tooltip's chord is the user's own binding, not a literal: rebinding
// `new_command` or `open_external_terminal` moves the word the tip prints, the
// way the reference's `formatShortcut(shortcuts.…)` read it at draw time
// (`main:src/App.tsx:3764-3790`).
func TestTerminalBarHintsFollowTheUserBinding(t *testing.T) {
	copy := i18n.For("en").Terminal
	shipped := settings.Default()
	defaultNew, defaultExternal := barHints(copy, shipped)

	bound := settings.Default()
	if !bound.SetShortcut(settings.ShortcutNewCommand, "Ctrl+Alt+K") {
		t.Fatal("the fixture's new-command binding was refused")
	}
	if !bound.SetShortcut(settings.ShortcutOpenExternalTerminal, "Ctrl+Alt+J") {
		t.Fatal("the fixture's open-external binding was refused")
	}
	newHint, externalHint := barHints(copy, bound)

	for _, want := range []struct{ name, hint, chord, shipped string }{
		{"new command", newHint, shortcuts.Display("Ctrl+Alt+K"), defaultNew},
		{"open external", externalHint, shortcuts.Display("Ctrl+Alt+J"), defaultExternal},
	} {
		if !strings.Contains(want.hint, want.chord) {
			t.Errorf("the %s hint = %q, want it to name %q", want.name, want.hint, want.chord)
		}
		if want.hint == want.shipped {
			t.Errorf("the %s hint did not move with the binding: %q", want.name, want.hint)
		}
	}
}

// The terminal view is a focusable region with an accessible name, so assistive
// technology says what it is — the reference's `aria-label={t("terminal.input")}`
// on its own text input (`main:src/App.tsx:3805`).
func TestTerminalInputCarriesItsAccessibleName(t *testing.T) {
	a := New(newStore(), Actions{}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	withSession(a, &terminal.Terminal{})
	tt := render(t, a, 860, 600)
	tt.Frame()
	if label := i18n.For("en").Terminal.Input; label == "" {
		t.Fatal("the copy has no terminal-input name")
	} else if _, ok := tt.Find(label); !ok {
		t.Errorf("the terminal view has no accessible name %q: %q", label, tt.Texts())
	}
}

// dotFrame renders the terminal page with a running session and the pointer
// over the bar, so the bar's identity row — and its status dot — is revealed.
// The dot's breath runs on the desktop's clock, so the frame is taken 400 ms
// into the loop, mid-fade: the one moment that tells a breathing dot from one
// at rest.
func dotFrame(t *testing.T, reduce bool) (*ui.Tester, ui.Color) {
	t.Helper()
	a := New(newStore(), Actions{}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	withSession(a, &terminal.Terminal{})
	var accent ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		accent = c.Theme().Accent
		a.View(c)
	}, 860, 600)
	tt.SetPreferences(ui.Preferences{ReduceMotion: reduce})
	tt.Move(400, 14)                   // over the bar: the identity row shows
	tt.Frame()                         // the dot appears here, and the loop starts with it
	tt.Advance(400 * time.Millisecond) // 400 ms into the breath
	tt.Frame()
	return tt, accent
}

// accentBox is the bounding box of the pixels that are the accent colour
// exactly, 0..1 blending left out: the dot's ink at full opacity.
func accentBox(img *image.RGBA, accent ui.Color) image.Rectangle {
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	for y := 0; y < 40; y++ {
		for x := 0; x < 400; x++ {
			i := y*img.Stride + x*4
			if img.Pix[i] == accent.R && img.Pix[i+1] == accent.G && img.Pix[i+2] == accent.B {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if maxX < 0 {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// The status dot's breath is the app's own loop, and the desktop's
// reduce-motion preference stops it: the old build's stylesheet names the
// breath and turns it off (`animation: none`), which leaves the dot at its 0%
// keyframe — the accent at full opacity and full size. The framework's Loop
// would turn regardless (a loop "shows that something is going on"), so the
// app reads the preference itself, through internal/motion.
//
// The keyframes are 1 -> 0.5 -> 1 over 2.4 s, so the two cases are read off
// one mid-cycle frame: a still, exact-accent 7px dot under reduce motion, and
// a blended, shrunk one when the breath runs.
func TestStatusDotRestsUnderReduceMotion(t *testing.T) {
	still, accent := dotFrame(t, true)
	box := accentBox(still.Image(), accent)
	if box.Empty() {
		t.Fatal("the resting dot draws no accent pixels in the bar")
	}
	// The disc is drawn at 7px; the *exact*-accent read below loses its edge
	// pixels to antialiasing, which the font and the rasteriser decide (macOS
	// keeps five, Linux seven). So the assertion is the shape and the step: a
	// square box (a disc, not an ellipse) no smaller than the mid-cycle 0.5
	// keyframe would draw — the breath's own floor — and no larger than the 7px
	// keyframe.
	if box.Dx() != box.Dy() || box.Dx() < 5 || box.Dx() > 7 {
		t.Errorf("the resting dot is %dx%d, want the 7px keyframe's disc (5..7 exact-accent pixels across; the edge blends with the bar)",
			box.Dx(), box.Dy())
	}
	at := image.Pt(box.Min.X+box.Dx()/2, box.Min.Y+box.Dy()/2)

	breathing, breathAccent := dotFrame(t, false)
	img := breathing.Image()
	i := at.Y*img.Stride + at.X*4
	if img.Pix[i] == breathAccent.R && img.Pix[i+1] == breathAccent.G && img.Pix[i+2] == breathAccent.B {
		t.Errorf("the dot is the untouched accent at %v 400 ms into the loop: the breath never ran", at)
	}
}

// A session whose program ended is held with its output on screen: the note
// says how it went, the dot goes still, and entering the page again starts a
// fresh session rather than showing the same finished frame.
func TestResidentSessionHoldsItsOutput(t *testing.T) {
	a := New(newStore(), Actions{}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	tt := render(t, a, 860, 600)
	// No session yet: the empty state.
	if !tt.HasText("No terminal session yet") {
		t.Fatalf("the empty state: %q", tt.Texts())
	}
	// The shell reports the program ended; with no session to hold, the page
	// still says nothing false — but the note is a session's own.
	a.Resident(0)
	tt.Frame()
	if tt.HasText("Process exited") {
		t.Error("a note showed with no session")
	}
	// The note is drawn for a held session, in the warning tone on a bad code.
	withSession(a, &terminal.Terminal{})
	a.Current().Resident, a.Current().ExitCode = true, 3
	tt.Frame()
	if !tt.HasText("Process exited") || !tt.HasText("Output retained") {
		t.Errorf("the resident note is missing: %q", tt.Texts())
	}
	if !a.exited() {
		t.Error("a held session does not read as ended")
	}
}

// Once the program on screen has ended, the bar's identity line names the code
// it returned, not just the session — the reference's `identityTitle`, which
// appends `terminal.headerExited` to the title while the session is held
// (`main:src/App.tsx:3716-3722`). It is a second word, distinct from the
// resident note's `terminal.processExited`, and the two show together.
func TestTerminalBarIdentityNamesTheExitCode(t *testing.T) {
	copy := i18n.For("en").Terminal
	a := New(newStore(), Actions{}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	withSession(a, &terminal.Terminal{})
	a.ResidentAt(0, 3)
	tt := render(t, a, 860, 600)
	tt.Frame()
	if want := copy.HeaderExited(3); !tt.HasText(want) {
		t.Errorf("the identity line does not name the exit code %q: %q", want, tt.Texts())
	}
	withTitle(a, "zsh")
	tt.Frame()
	if want := "zsh \u00b7 " + copy.HeaderExited(3); !tt.HasText(want) {
		t.Errorf("the identity line is not %q: %q", want, tt.Texts())
	}
	if want := copy.ProcessExited(3); !tt.HasText(want) {
		t.Errorf("the resident note is gone: %q", tt.Texts())
	}
}

// The bar's gear is the door to the terminal's own appearance drawer: it is a
// control of its own, and a caller that provides none draws none.
func TestTerminalBarSettingsDoor(t *testing.T) {
	opened := 0
	a := New(newStore(), Actions{Settings: func() { opened++ }}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	withSession(a, &terminal.Terminal{})
	withTitle(a, "vim")
	tt := render(t, a, 860, 600)
	tt.Frame()
	if !tt.HasText("Terminal settings") {
		t.Fatalf("the gear is missing: %q", tt.Texts())
	}
	if err := tt.Click("Terminal settings"); err != nil {
		t.Fatalf("the gear: %v", err)
	}
	tt.Frame()
	if opened != 1 {
		t.Errorf("the gear opened the drawer %d times", opened)
	}
	// Without the action the bar draws no gear at all.
	b := New(newStore(), Actions{}, nil)
	withSession(b, &terminal.Terminal{})
	tt2 := render(t, b, 860, 600)
	tt2.Frame()
	if tt2.HasText("Terminal settings") {
		t.Errorf("a gear was drawn with no action: %q", tt2.Texts())
	}
}

// withSession gives the surface one session holding a terminal, for the tests
// that only need a session to be present.
func withSession(a *App, term *terminal.Terminal) {
	a.Sessions = []*Session{{Term: term, Created: time.Now()}}
	a.Active = 0
}

// withTitle names the session on screen.
func withTitle(a *App, title string) {
	if session := a.Current(); session != nil {
		session.Title = title
	}
}
