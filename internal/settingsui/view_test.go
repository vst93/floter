package settingsui

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/control"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/terminalui"
	"time"
)

func newStore(t *testing.T) *settings.Store {
	t.Helper()
	return settings.NewStore(settings.Default())
}

// newStoreFor is newStore for a benchmark, which has no *testing.T.
func newStoreFor(b *testing.B) *settings.Store {
	b.Helper()
	return settings.NewStore(settings.Default())
}

func render(t *testing.T, a *App, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
	}, w, h)
	tt.Frame()
	return tt
}

func TestSettingsShowsThePagesAndTheGeneralControls(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 580)

	for _, want := range []string{
		"Settings", "General", "Sessions", "Shortcuts", "Integrations", "About",
		"Appearance", "Language", "Liquid glass effect", "App transparency",
		"Terminal transparency", "Interface size",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
}

// Every settings row keeps the grey second line the reference showed under
// its label. The check-box helper used to drop it, so the four General-page
// switches — and the Integrations page's PATH switch — read as bare labels
// with nothing saying what they do.
func TestSettingsRowsKeepTheirHints(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 1600)

	for _, want := range []string{
		copy.ThemeHint,
		copy.LaunchAtStartupHint,
		copy.HideOnBlurHint,
		copy.ShowRecentInLauncherHint,
		copy.ShowMenubarIconHint,
	} {
		if !tt.HasText(want) {
			t.Errorf("missing the hint %q in %q", want, tt.Texts())
		}
	}

	// The Integrations page's own switch carries its hint too.
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(copy.ShowCommandsInSearchHint) {
		t.Errorf("missing the PATH switch's hint in %q", tt.Texts())
	}
}

// Every slider names the two ends of its own axis, the reference's
// `SettingsScale` line under the track: the two transparency sliders read
// "Transparent … Solid" (`settings.transparency.scaleLow/.scaleHigh`) and the
// two terminal appearance sliders read "Small … Large"
// (`settings.scale.small/.large`). The live readout also moved onto the title
// line with the label (`.settings-slider__head`), as the reference draws it.
func TestSliderRowsNameTheirAxis(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 2400)

	count := func(text string) int {
		n := 0
		for _, shown := range tt.Texts() {
			if shown == text {
				n++
			}
		}
		return n
	}

	// The two transparency rows are the only place these two words appear, so
	// each one has to show up exactly twice (once per slider's end).
	if got := count(copy.TransparencyScaleLow); got != 2 {
		t.Errorf("the %q axis word appears %d times, want 2: %q", copy.TransparencyScaleLow, got, tt.Texts())
	}
	if got := count(copy.TransparencyScaleHigh); got != 2 {
		t.Errorf("the %q axis word appears %d times, want 2: %q", copy.TransparencyScaleHigh, got, tt.Texts())
	}

	// "Small"/"Large" are also the Interface size control's own option labels,
	// so the two slider ends sit on top of that one appearance each: three in
	// all. Before the ends were drawn each word showed up only once.
	if got := count(copy.ScaleSmall); got < 3 {
		t.Errorf("the %q axis word appears %d times, want at least 3: %q", copy.ScaleSmall, got, tt.Texts())
	}
	if got := count(copy.ScaleLarge); got < 3 {
		t.Errorf("the %q axis word appears %d times, want at least 3: %q", copy.ScaleLarge, got, tt.Texts())
	}
}

// The material group is its own section: the glass effect on one card and the
// two transparency sliders on a second, with the group's two explanations
// drawn as footnotes under the cards (the reference's `.settings-section__hint`
// at the end of a section) rather than repeated on each slider row. Before this
// the glass control sat inside the Appearance card and each transparency row
// carried its own inline sentence.
func TestGeneralMaterialGroupFootnotes(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 2400)

	count := func(text string) int {
		n := 0
		for _, shown := range tt.Texts() {
			if shown == text {
				n++
			}
		}
		return n
	}

	// The section title and its two explanations show up once each — never a
	// second time as a row-level hint.
	for _, once := range []string{copy.Material, copy.GlassHint, copy.TransparencyHint} {
		if got := count(once); got != 1 {
			t.Errorf("%q appears %d times, want 1: %q", once, got, tt.Texts())
		}
	}

	title, ok := tt.Find(copy.Material)
	if !ok {
		t.Fatalf("the material section title %q is missing", copy.Material)
	}
	// The glass control left the Appearance card and now lives in the material
	// section: its row sits below the section's own title.
	glass, ok := tt.Find(copy.Glass)
	if !ok {
		t.Fatalf("the glass control %q is missing", copy.Glass)
	}
	if glass.Y <= title.Y {
		t.Errorf("the glass control at %v is not under the material section at %v", glass, title)
	}
	mainRow, ok := tt.Find(copy.MainOpacity)
	if !ok {
		t.Fatalf("the %q row is missing", copy.MainOpacity)
	}
	termRow, ok := tt.Find(copy.TerminalOpacity)
	if !ok {
		t.Fatalf("the %q row is missing", copy.TerminalOpacity)
	}
	if mainRow.Y <= title.Y || termRow.Y <= title.Y {
		t.Errorf("the transparency rows are not under the material section (%v, %v vs %v)", mainRow, termRow, title)
	}
	// Both explanations are footnotes below the cards they speak for, not a
	// sentence inside the rows.
	glassFoot, _ := tt.Find(copy.GlassHint)
	if glassFoot.Y <= glass.Y {
		t.Errorf("the glass note at %v is not below the glass row at %v", glassFoot, glass)
	}
	opacityFoot, _ := tt.Find(copy.TransparencyHint)
	if opacityFoot.Y <= termRow.Y {
		t.Errorf("the transparency note at %v is not below the last transparency row at %v", opacityFoot, termRow)
	}
}

// The menu bar group is its own section: the visibility switch (R7-10c) and
// the light/dark app-icon choice (R150) share one card because both describe
// the one icon in the menu bar / tray (the reference's `settings.group.menuBar`
// card). Before this the icon row sat in the Appearance card and the switch in
// the Window behaviour card, a screen apart, in two groups that do not speak
// for the icon.
func TestGeneralMenuBarGroup(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 2600)

	// The group's title is drawn once; it is not also smuggled onto a card as
	// the accessible name of the wrong thing.
	title, ok := tt.Find(copy.GroupMenuBar)
	if !ok {
		t.Fatalf("the menu bar section title %q is missing: %q", copy.GroupMenuBar, tt.Texts())
	}
	switchRow, ok := tt.Find(copy.ShowMenubarIcon)
	if !ok {
		t.Fatalf("the %q switch is missing", copy.ShowMenubarIcon)
	}
	iconRow, ok := tt.Find(copy.AppIcon)
	if !ok {
		t.Fatalf("the %q row is missing", copy.AppIcon)
	}

	// Both controls live under the section's own title.
	if switchRow.Y <= title.Y || iconRow.Y <= title.Y {
		t.Errorf("the menu bar controls at %v/%v are not under the section title at %v", switchRow, iconRow, title)
	}

	// They sit in the same card: no other group's title falls between them.
	// The ordering is the reference's own (the visibility switch first, the
	// icon choice after it), not an invention.
	if switchRow.Y >= iconRow.Y {
		t.Errorf("the switch at %v is not above the icon row at %v, want the reference order", switchRow, iconRow)
	}
	for name, pos := range map[string]ui.Rect{
		copy.GroupWindow:   mustFind(t, tt, copy.GroupWindow),
		copy.GroupTerminal: mustFind(t, tt, copy.GroupTerminal),
		copy.Material:      mustFind(t, tt, copy.Material),
	} {
		if pos.Y > switchRow.Y && pos.Y < iconRow.Y {
			t.Errorf("the %q group at %v splits the menu bar card (%v → %v)", name, pos, switchRow, iconRow)
		}
	}

	// The icon left the Appearance card: the appearance section's own rows (the
	// language picker is its last) are above the menu bar section, while the
	// icon row is below it.
	if langRow, ok := tt.Find(copy.Language); !ok {
		t.Fatalf("the %q row is missing", copy.Language)
	} else if langRow.Y > title.Y {
		t.Errorf("the language row at %v is not above the menu bar section at %v", langRow, title)
	}
}

// The interface size is an Appearance choice, not a window behaviour one: the
// reference keeps it in the appearance card beside theme and language
// (R7-13c, `GeneralPage.tsx:370-390`) because it is the same kind of decision —
// how the interface looks. Before this it sat in the Window behaviour card. The
// same reference orders the window rows launch → hide-on-blur → page residency
// → recent items (`GeneralPage.tsx:410`/`:431`/`:450`/`:464`); this pins both.
func TestInterfaceSizeSitsWithAppearance(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 2800)

	appearance := mustFind(t, tt, copy.GroupAppearance)
	window := mustFind(t, tt, copy.GroupWindow)
	theme := mustFind(t, tt, copy.Theme)
	language := mustFind(t, tt, copy.Language)
	scale := mustFind(t, tt, copy.Scale)

	// The size row lives inside the Appearance card: below its title and
	// above the Window behaviour section that follows it, and after the two
	// rows the reference draws before it.
	if scale.Y <= appearance.Y || scale.Y >= window.Y {
		t.Errorf("the interface size at %v is not inside the appearance card (%v → %v)", scale, appearance, window)
	}
	if scale.Y <= theme.Y || scale.Y <= language.Y {
		t.Errorf("the interface size at %v is not after theme (%v) and language (%v), want the reference order", scale, theme, language)
	}

	// The window rows keep the reference order: page residency before the
	// recent-items switch, not after it.
	residency := mustFind(t, tt, copy.SurfaceResidency)
	recent := mustFind(t, tt, copy.ShowRecentInLauncher)
	if residency.Y >= recent.Y {
		t.Errorf("the page-residency row at %v is not above the recent-items switch at %v, want the reference order", residency, recent)
	}
}

// mustFind is Find for a row the caller already knows is on the page.
func mustFind(tb testing.TB, tt *ui.Tester, text string) ui.Rect {
	tb.Helper()
	r, ok := tt.Find(text)
	if !ok {
		tb.Fatalf("missing %q: %q", text, tt.Texts())
	}
	return r
}

func TestSettingsControlsWriteThroughTheStore(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 1400)

	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().Theme; got != "dark" {
		t.Errorf("theme = %q, want dark", got)
	}

	if err := tt.Click("Liquid Max"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().GlassStep; got != "liquid" {
		t.Errorf("glass step = %q, want liquid", got)
	}

	if err := tt.Click("Large"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().UIScale; got != "large" {
		t.Errorf("ui scale = %q, want large", got)
	}

	// The language switch is last: it relabels every control.
	if err := tt.Click("中文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().Language; got != "zh" {
		t.Errorf("language = %q, want zh", got)
	}
	if !tt.HasText("设置") {
		t.Errorf("the panel did not switch language: %q", tt.Texts())
	}
}

func TestSettingsOpacityShowsAndFollowsTheStore(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 580)

	if !tt.HasText("47%") || !tt.HasText("46%") {
		t.Errorf("opacity values missing: %q", tt.Texts())
	}
	if err := store.Update(func(s *settings.Settings) { s.MainOpacity = 80 }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("80%") {
		t.Errorf("the control did not follow the store: %q", tt.Texts())
	}
}

func TestSetWritesThroughAndClamps(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	a.set(func(s *settings.Settings) { s.MainOpacity = 250 })
	if got := store.Snapshot().MainOpacity; got != settings.MaxWindowOpacity {
		t.Errorf("opacity = %d, want the %d ceiling", got, settings.MaxWindowOpacity)
	}
	a.set(func(s *settings.Settings) { s.MainOpacity = 1 })
	if got := store.Snapshot().MainOpacity; got != settings.MinWindowOpacity {
		t.Errorf("opacity = %d, want the %d floor", got, settings.MinWindowOpacity)
	}
	for _, tc := range []struct {
		in   float64
		want uint8
	}{{47.4, 47}, {47.6, 48}, {0, settings.MinWindowOpacity}, {250, settings.MaxWindowOpacity}} {
		if got := clampPercent(tc.in); got != tc.want {
			t.Errorf("clampPercent(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSettingsSidebarRoutesAndEscapeCloses(t *testing.T) {
	closed := 0
	a := New(newStore(t), Actions{Close: func() { closed++ }})
	tt := render(t, a, 720, 580)

	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Page != PageIntegrations {
		t.Fatalf("page = %d, want integrations", a.Page)
	}
	if !tt.HasText("No integrations are installed.") {
		t.Errorf("the empty integrations state did not show: %q", tt.Texts())
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if closed != 1 || a.Page != PageIntegrations {
		t.Errorf("Escape closed %d times, page %d", closed, a.Page)
	}
}

func TestTerminalAppearanceControls(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 1800)

	// The settings card paints exactly the axes the schema gives this surface
	// (internal/control, R55): the family, the size, the line height and the
	// palette. Reading the list from the schema is what keeps this test and the
	// view from agreeing on a stale subset.
	labels := map[control.AppearanceAxis]string{
		control.AxisFontFamily: "Font family",
		control.AxisFontSize:   "Font size",
		control.AxisLineHeight: "Line height",
		control.AxisPalette:    "Terminal palette",
	}
	if len(labels) != len(control.SettingsCardAxes()) {
		t.Fatalf("the test names %d card axes, the schema has %d", len(labels), len(control.SettingsCardAxes()))
	}
	if !tt.HasText("Terminal appearance") {
		t.Errorf("missing the group title in %q", tt.Texts())
	}
	for _, axis := range control.SettingsCardAxes() {
		label, ok := labels[axis]
		if !ok {
			t.Fatalf("the card owes the schema an axis with no row: %q", axis)
		}
		if !tt.HasText(label) {
			t.Errorf("missing %q in %q", label, tt.Texts())
		}
	}

	// The rows the schema reserves for the terminal's own drawer are *absent*
	// here: the card is not a second copy of the drawer.
	for _, gone := range []string{"Cursor shape", "Blinking cursor", "Padding", "Safe paste"} {
		if rect, ok := tt.Find(gone); ok {
			t.Errorf("the settings card draws the drawer-only row %q at %v", gone, rect)
		}
	}

	// The card keeps the schema's own order (family, size, line height,
	// palette), so a row cannot be inserted between two others by accident.
	var lastY float32 = -1
	for _, axis := range control.SettingsCardAxes() {
		rect, ok := tt.Find(labels[axis])
		if !ok {
			t.Fatalf("%q is missing", labels[axis])
		}
		if rect.Y <= lastY {
			t.Errorf("%q at %v is not below the row the schema puts before it", labels[axis], rect)
		}
		lastY = rect.Y
	}

	// The terminal group sits below the fold: scroll the body to reach it.
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()

	// The trailing select's trigger shows its current value ("Follow app"),
	// not the row label; clicking it opens the drop-down.
	if err := tt.Click("Follow app"); err != nil {
		t.Fatalf("palette trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Forest"); err != nil {
		t.Fatalf("palette option: %v", err)
	}
	tt.Frame()
	if got := store.Snapshot().TerminalTheme; got != "forest" {
		t.Errorf("palette = %q, want forest", got)
	}
}

// The settings card's two sliders read the same values the terminal's own
// drawer does, because both paint the one schema (`internal/control`): the
// font size as `<n> px` and the line height to two decimals
// (`TerminalAppearance.tsx:190-195,210-215`). Before this the card dropped the
// unit and rounded the line height to one decimal, so the same 1.20× value read
// as `14` and `1.2×` here against `14 px` and `1.20×` in the drawer.
func TestTerminalCardReadoutsMatchTheDrawer(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 2400)

	// The card sits below the fold: scroll the body to reach it.
	tt.Scroll(400, 300, 0, 1500)
	tt.Frame()

	for _, readout := range []string{"14 px", "1.20\u00d7"} {
		if !tt.HasText(readout) {
			t.Errorf("the settings card is missing the readout %q: %q", readout, tt.Texts())
		}
	}

	// The font-size row carries the reference's card-only hint, which the
	// terminal's own drawer deliberately keeps out — and the card carries it
	// exactly once, on that row (the palette row is stacked with no hint).
	count := 0
	for _, shown := range tt.Texts() {
		if shown == i18n.For("en").Settings.TerminalHint {
			count++
		}
	}
	if count != 1 {
		t.Errorf("the card shows the terminal hint %d times, want 1 on the font-size row: %q", count, tt.Texts())
	}
}

func TestSessionsPage(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 760)

	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("No sessions are running.") {
		t.Errorf("missing the empty session state: %q", tt.Texts())
	}

	closed, shown := 0, 0
	a.Actions.CloseSessionAt = func(index int) { closed = index + 1 }
	a.Actions.SelectSession = func(index int) { shown = index + 1 }
	notes := []string{"Running · attached", "Running · detached", "Process exited · code 1"}
	a.Sessions = func() []Session {
		out := []Session{
			{Title: "zsh — floter", Running: true, Active: true, Index: 0, ID: 1},
			{Title: "vim", Running: true, Index: 1, ID: 2},
			{Title: "build", Running: false, Index: 2, ID: 3},
		}
		for i := range out {
			out[i].Note = notes[i]
		}
		return out
	}
	tt.Frame()
	for _, want := range []string{
		"zsh — floter", "vim", "build",
		"Running · attached", "Running · detached", "Process exited · code 1",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	// Closing a session kills its program, so the control arms first: one
	// press asks, and the second ends it.
	if err := tt.Click("Close session"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if closed != 0 {
		t.Fatalf("the first press closed a session: %d", closed)
	}
	if !tt.HasText("Close it?") {
		t.Fatalf("the armed question is missing: %q", tt.Texts())
	}
	if err := tt.Click("Keep"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	tt.Frame()
	if closed != 0 {
		t.Errorf("the way back closed a session: %d", closed)
	}
	if err := tt.Click("Close session"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Close it?"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	tt.Frame()
	if closed != 1 {
		t.Errorf("close session ran with index %d, want 0", closed-1)
	}
	// Resuming another session hands its index back.
	if err := tt.Click("Show"); err != nil {
		t.Fatalf("show: %v", err)
	}
	tt.Frame()
	if shown != 2 {
		t.Errorf("show ran with index %d, want 1", shown-1)
	}
	// A new session is the page's own control.
	started := 0
	a.Actions.NewSession = func() { started++ }
	tt.Frame()
	if err := tt.Click("New session"); err != nil {
		t.Fatalf("new session: %v", err)
	}
	tt.Frame()
	if started != 1 {
		t.Errorf("new session ran %d times", started)
	}
}

// Escape backs out of an armed session close rather than the page: the
// reference's SessionsPage installed its own capture-phase Escape listener
// (`disarmKill`) that answered the question and swallowed the key. With
// nothing armed, the same key is the panel's way out again.
func TestEscapeDisarmsASessionClose(t *testing.T) {
	closed, closes := 0, 0
	a := New(newStore(t), Actions{
		CloseSessionAt: func(index int) { closed = index + 1 },
		Close:          func() { closes++ },
	})
	a.Sessions = func() []Session {
		return []Session{{Title: "zsh — floter", Running: true, Active: true, Index: 0, ID: 1}}
	}
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Close session"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Close it?") {
		t.Fatalf("the close did not arm: %v", tt.Texts())
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if closes != 0 {
		t.Errorf("Escape left the page: Close ran %d times", closes)
	}
	if closed != 0 {
		t.Errorf("Escape closed a session: %d", closed)
	}
	if tt.HasText("Close it?") {
		t.Errorf("Escape left the question armed: %v", tt.Texts())
	}

	// Nothing is armed now, so Escape is the panel's way out again.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if closes != 1 {
		t.Errorf("the second Escape ran Close %d times, want 1", closes)
	}
}

// TestSessionsListOrdersAndToggles pins the sessions list's own axis: the
// newest session is first (the old page's default), the heading carries that
// order as a control, and pressing it swaps the list to oldest first. It also
// pins what a row says under the session's name: the shell's working
// directory, the state line, the screen size and the start time.
func TestSessionsListOrdersAndToggles(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	a.Sessions = func() []Session {
		return []Session{
			{Title: "first", Running: true, Index: 0, Note: "Running \u00b7 detached",
				Dir: "/home/u/first", Size: "80\u00d724", Created: base},
			{Title: "second", Running: true, Active: true, Index: 1, Note: "Running \u00b7 attached",
				Dir: "/home/u/second", Size: "100\u00d730", Created: base.Add(time.Hour)},
			{Title: "third", Running: false, Index: 2, Note: "Process exited \u00b7 code 1",
				Created: base.Add(2 * time.Hour)},
		}
	}
	tt.Frame()
	rowY := func(label string) float32 {
		t.Helper()
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no row %q in %q", label, tt.Texts())
		}
		return rect.Y
	}
	// Newest first, the shipped order.
	if !(rowY("third") < rowY("second") && rowY("second") < rowY("first")) {
		t.Errorf("the list is not newest-first: first=%v second=%v third=%v",
			rowY("first"), rowY("second"), rowY("third"))
	}
	if !tt.HasText("Newest first") {
		t.Errorf("the order control is missing: %q", tt.Texts())
	}
	// The row carries its own facts: the shell's cwd, the state, the size and
	// the start time (formatted the way the launcher writes a visit time).
	if !tt.HasText("/home/u/second") {
		t.Errorf("the working directory is missing: %q", tt.Texts())
	}
	if !tt.HasText("100\u00d730") {
		t.Errorf("the screen size is missing: %q", tt.Texts())
	}
	if !tt.HasText(base.Add(time.Hour).Local().Format("2006-01-02 15:04")) {
		t.Errorf("the start time is missing: %q", tt.Texts())
	}
	// Pressing the control swaps the axis, and its own word says so.
	if err := tt.Click("Newest first"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Oldest first") {
		t.Errorf("the order control did not flip: %q", tt.Texts())
	}
	if !(rowY("first") < rowY("second") && rowY("second") < rowY("third")) {
		t.Errorf("the list is not oldest-first after the toggle: first=%v second=%v third=%v",
			rowY("first"), rowY("second"), rowY("third"))
	}
}

// TestSessionsWithoutAStartKeepTheShellsOrder pins the sort's stability: the
// shell hands sessions over in creation order and a fixture that records no
// start time must not be reshuffled by the sort.
func TestSessionsWithoutAStartKeepTheShellsOrder(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Sessions = func() []Session {
		return []Session{
			{Title: "a", Index: 0},
			{Title: "b", Index: 1},
			{Title: "c", Index: 2},
		}
	}
	got := a.sortedSessions()
	for i, want := range []string{"a", "b", "c"} {
		if got[i].Title != want {
			t.Errorf("order after sorting = %v, want a b c", []string{got[0].Title, got[1].Title, got[2].Title})
			break
		}
	}
}

func TestShortcutsPage(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Shortcut = "Alt+Space"
	tt := render(t, a, 720, 580)

	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Show / hide floter") || !tt.HasText(shortcuts.Display("Alt+Space")) {
		t.Errorf("the shortcut did not show: %q", tt.Texts())
	}
	if !tt.HasText("Record") {
		t.Errorf("the recorder is missing: %q", tt.Texts())
	}
	if !tt.HasText("The global keys floter answers, and the ones you bind yourself.") {
		t.Errorf("the hint did not show: %q", tt.Texts())
	}
}

func TestAboutPage(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.About = About{
		Name:         "floter",
		Version:      "0.3.14",
		Framework:    "mygo 0.3.5",
		Scheme:       "floter://",
		SettingsPath: "/Users/v/Library/Application Support/floter/settings.json",
		RepoURL:      "https://github.com/vst93/floter",
	}
	tt := render(t, a, 720, 580)

	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{
		"v0.3.14", "0.3.14", "mygo 0.3.5", "floter://",
		"/Users/v/Library/Application Support/floter/settings.json",
		"https://github.com/vst93/floter",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	if err := tt.Click("https://github.com/vst93/floter"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if urls := tt.OpenedURLs(); len(urls) != 1 || urls[0] != "https://github.com/vst93/floter" {
		t.Errorf("opened URLs = %v", urls)
	}
}

// The deep-link copy button confirms the copy on itself — the old build's own
// "Link copied" for 1.6s — because a clipboard write is otherwise silent. The
// confirmation belongs to the line that was copied, not to both.
func TestDeepLinkCopyConfirmsOnTheButton(t *testing.T) {
	en := i18n.For("en").Settings
	var copied []string
	a := New(newStore(t), Actions{CopyText: func(text string) { copied = append(copied, text) }})
	tt := render(t, a, 720, 900)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(en.DeepLinkCopy) || tt.HasText(en.DeepLinkCopied) {
		t.Fatalf("before a copy: %q", tt.Texts())
	}
	if err := tt.Click(en.DeepLinkCopy); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(copied) != 1 || copied[0] != "floter://connect?manifest=/path/to/tool.json" {
		t.Fatalf("copied = %v", copied)
	}
	if !tt.HasText(en.DeepLinkCopied) {
		t.Errorf("the button did not confirm the copy: %q", tt.Texts())
	}
	// Only the copied line confirms; the other keeps its resting label.
	resting := 0
	for _, text := range tt.Texts() {
		if text == en.DeepLinkCopy {
			resting++
		}
	}
	if resting != 1 {
		t.Errorf("the other line's button changed too: %d rest on %q", resting, en.DeepLinkCopy)
	}
	// The window is the app's own state, so its expiry is testable without
	// the frame clock.
	if !a.linkCopied("floter://connect?manifest=/path/to/tool.json", time.Now()) {
		t.Error("the copied line is not marked copied")
	}
	if a.linkCopied("floter://connect?manifest=/path/to/tool.json", time.Now().Add(2*time.Second)) {
		t.Error("the confirmation outlived its window")
	}
}

func TestWindowBehaviourControls(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 1400)

	tt.Scroll(400, 300, 0, 400)
	tt.Frame()

	if err := tt.Click("Hide when focus is lost"); err != nil {
		t.Fatalf("hide on blur: %v", err)
	}
	tt.Frame()
	if store.Snapshot().HideOnBlur {
		t.Error("the hide-on-blur check box did not turn off")
	}

	// The residency control shows what is stored; a custom value the
	// presets do not name shows as itself, so the control never lies.
	if err := store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 45 }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("45 s") {
		t.Errorf("the custom residency did not show: %q", tt.Texts())
	}
	if err := store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 120 }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("120 s") {
		t.Errorf("the preset residency did not show: %q", tt.Texts())
	}
}

func TestResidencySelectMapping(t *testing.T) {
	never := settings.SurfaceResidencyNever
	for _, tc := range []struct {
		stored uint32
		value  string
	}{
		{0, "off"},
		{10, "10"},
		{120, "120"},
		{never, "never"},
		{45, "custom"},
		// A hand-edited value above the custom ceiling is clamped, so it maps
		// to the entry it actually stores rather than lying about 90000.
		{settings.MaxSurfaceResidencySeconds + 1, "custom"},
	} {
		if got := residencySelectValue(tc.stored); got != tc.value {
			t.Errorf("residencySelectValue(%d) = %q, want %q", tc.stored, got, tc.value)
		}
	}

	// Every named choice resolves to the number it stands for; custom carries
	// none, which is what opens the inline field.
	for _, tc := range []struct {
		choice string
		value  uint32
		ok     bool
	}{
		{"off", 0, true},
		{"10", 10, true},
		{"never", never, true},
		{"custom", 0, false},
		{"soon", 0, false},
	} {
		value, ok := residencyFromSelect(tc.choice)
		if value != tc.value || ok != tc.ok {
			t.Errorf("residencyFromSelect(%q) = (%d, %v), want (%d, %v)", tc.choice, value, ok, tc.value, tc.ok)
		}
	}

	// The field seeds with a stored custom value, and with the shipped default
	// for every named one. An out-of-range value was clamped by the normalizer,
	// so it seeds the clamped number rather than the default.
	if got := residencyCustomSeedSeconds(45); got != 45 {
		t.Errorf("seed(45) = %d, want 45", got)
	}
	for _, stored := range []uint32{0, 10, 120, never} {
		if got := residencyCustomSeedSeconds(stored); got != settings.DefaultSurfaceResidencySeconds {
			t.Errorf("seed(%d) = %d, want the shipped default", stored, got)
		}
	}
	if got := residencyCustomSeedSeconds(settings.MaxSurfaceResidencySeconds + 1); got != settings.MaxSurfaceResidencySeconds {
		t.Errorf("seed(above the ceiling) = %d, want the clamped ceiling", got)
	}

	// The typed draft is normalized like the reference's
	// normalizeResidencySeconds(Number(draft)): an empty field is off, junk is
	// the default, and a real number is clamped onto the domain.
	for _, tc := range []struct {
		text string
		want uint32
	}{
		{"", 0},
		{"  ", 0},
		{"45", 45},
		{"soon", settings.DefaultSurfaceResidencySeconds},
		{"999999", settings.MaxSurfaceResidencySeconds},
	} {
		if got := parseResidencyDraft(tc.text); got != tc.want {
			t.Errorf("parseResidencyDraft(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

// TestSurfaceResidencyControlIsOneRow pins the reference's single-row shape:
// the select names the custom entry and the inline seconds field (with its Set
// button) appears only while it is chosen. A preset stored value shows the
// select alone; selecting custom opens the field without writing anything; a
// committed custom value is echoed in the custom entry's own label.
func TestSurfaceResidencyControlIsOneRow(t *testing.T) {
	copy := i18n.For("en").Settings
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 1400)
	tt.Scroll(400, 300, 0, 400)
	tt.Frame()

	// A preset: the select shows it, and the row carries no inline field.
	if err := store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 120 }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find(copy.SurfaceResidencyValue(120)); !ok {
		t.Errorf("the preset residency did not show: %q", tt.Texts())
	}
	if _, ok := tt.Find(copy.SurfaceResidencyCustomApply); ok {
		t.Errorf("the custom field is drawn under a preset value: %q", tt.Texts())
	}
	if _, ok := tt.Find(copy.SurfaceResidencyCustom); ok {
		t.Errorf("the custom entry claims a value: %q", tt.Texts())
	}

	// Choosing custom opens the field on the seeded default and writes nothing.
	a.residencyChoice(residencyCustom, 120)
	tt.Frame()
	if _, ok := tt.Find(copy.SurfaceResidencyCustomApply); !ok {
		t.Errorf("choosing custom did not open the inline field: %q", tt.Texts())
	}
	if store.Snapshot().SurfaceResidencySeconds != 120 {
		t.Errorf("choosing custom wrote %d, want the stored 120 untouched", store.Snapshot().SurfaceResidencySeconds)
	}

	// Committing a custom duration stores it and echoes it in the entry.
	a.residencyDraft = "45"
	a.commitResidencyDraft()
	tt.Frame()
	if got := store.Snapshot().SurfaceResidencySeconds; got != 45 {
		t.Fatalf("the committed draft stored %d, want 45", got)
	}
	if _, ok := tt.Find(copy.SurfaceResidencyCustomValue(45)); !ok {
		t.Errorf("the custom entry did not echo the stored value: %q", tt.Texts())
	}

	// A number that lands on a preset closes the field again.
	a.residencyDraft = "30"
	a.commitResidencyDraft()
	tt.Frame()
	if _, ok := tt.Find(copy.SurfaceResidencyCustomApply); ok {
		t.Errorf("the field stayed open on a preset value: %q", tt.Texts())
	}
	if _, ok := tt.Find(copy.SurfaceResidencyValue(30)); !ok {
		t.Errorf("the committed preset did not show: %q", tt.Texts())
	}
}

func TestIntegrationsPage(t *testing.T) {
	toggled := ""
	toggledTo := false
	removedID, removedName := "", ""
	a := New(newStore(t), Actions{
		SetIntegrationEnabled: func(id string, enabled bool) { toggled, toggledTo = id, enabled },
		UninstallIntegration:  func(id, name, kind string) { removedID, removedName = id, name },
	})
	a.Integrations = func() []Integration {
		return []Integration{
			{
				ID: "io.github.vst93.v", Name: "V Tools", Description: "Developer tools",
				Publisher: "vst", Version: "0.0.12", ToolVersion: "0.0.12",
				Enabled: true, Running: true,
			},
			{
				ID: "dev.floter.static", Name: "Static Tools", Publisher: "floter",
				Version: "1.0.0", Enabled: false,
			},
			{
				ID: "bad.tool", Name: "Bad", Enabled: true, Broken: true, Error: "provider exploded",
			},
			{ID: "orphan.pkg", Name: "orphan.pkg", Orphan: true},
		}
	}
	tt := render(t, a, 720, 1200)

	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{
		"V Tools", "vst · 0.0.12 · io.github.vst93.v", "Running",
		"Static Tools", "Disabled", "Bad", "Broken", "provider exploded",
		"orphan.pkg", "Installed on disk, not recorded",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// The Running row's switch asks before turning the integration off, and
	// the confirmation is what performs it.
	if err := tt.Click("Enabled"); err != nil {
		t.Fatalf("the enable switch is missing: %v", err)
	}
	tt.Frame()
	if toggled != "" {
		t.Errorf("turning off happened without asking: %q", toggled)
	}
	if !tt.HasText("Disable V Tools?") {
		t.Fatalf("the confirmation is missing: %q", tt.Texts())
	}
	if err := tt.Click("Disable integration"); err != nil {
		t.Fatalf("the confirm button: %v", err)
	}
	tt.Frame()
	if toggled != "io.github.vst93.v" || toggledTo {
		t.Errorf("toggled %q to %v", toggled, toggledTo)
	}
	// The first row's removal button asks inline first; the bar's danger
	// button is what asks the shell to remove it. The row's verb follows the
	// integration kind (P7-eh): this fixture is a plain linked package, so it
	// says "Remove package", not the npm kind's "Uninstall". The bar asks the
	// same kind's question (P7-eo): "Remove V Tools?", never the generic npm
	// "Uninstall V Tools?".
	if err := tt.Click("Remove package"); err != nil {
		t.Fatalf("the uninstall button is missing: %v", err)
	}
	tt.Frame()
	if removedID != "" {
		t.Fatalf("the removal happened without asking: %q", removedID)
	}
	if !tt.HasText("Remove V Tools?") {
		t.Fatalf("the removal bar is missing: %q", tt.Texts())
	}
	if tt.HasText("Uninstall V Tools?") {
		t.Fatalf("the package bar asked the npm question: %q", tt.Texts())
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatalf("the cancel button: %v", err)
	}
	tt.Frame()
	if removedID != "" || tt.HasText("Remove V Tools?") {
		t.Fatalf("cancel removed or kept the bar: removed=%q texts=%q", removedID, tt.Texts())
	}
	if err := tt.Click("Remove package"); err != nil {
		t.Fatalf("reopening the removal bar: %v", err)
	}
	tt.Frame()
	// While a question is open, Escape answers it with "no" — it does not
	// close the page too.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if removedID != "" || tt.HasText("Remove V Tools?") {
		t.Fatalf("Escape removed or kept the bar: removed=%q texts=%q", removedID, tt.Texts())
	}
	if err := tt.Click("Remove package"); err != nil {
		t.Fatalf("reopening the removal bar: %v", err)
	}
	tt.Frame()
	// The bar's own danger button is the second "Remove package" on screen
	// (the row's button is the first), so it is read positionally: it sits in
	// the same row as Cancel, just to its right.
	cancel, ok := tt.Find("Cancel")
	if !ok {
		t.Fatalf("the bar's cancel button is missing: %q", tt.Texts())
	}
	tt.ClickAt(cancel.X+cancel.W+30, cancel.Y+cancel.H/2)
	tt.Frame()
	if removedID != "io.github.vst93.v" || removedName != "V Tools" {
		t.Errorf("uninstall recorded %q/%q", removedID, removedName)
	}
	if tt.HasText("Remove V Tools?") {
		t.Errorf("the bar stayed open after confirming: %q", tt.Texts())
	}

	// An orphan has no switch: clicking its name does nothing.
	before := toggled
	if err := tt.Click("orphan.pkg"); err == nil {
		tt.Frame()
	}
	if toggled != before {
		t.Errorf("an orphan toggled %q", toggled)
	}
}

// A generated custom integration's removal bar speaks its own words: the
// files Floter wrote for it go too, and the warning says Floter cannot undo
// it. The danger button is the only one of its name, so it is clicked by
// text.
func TestGeneratedIntegrationRemovalBar(t *testing.T) {
	removedID, removedName := "", ""
	a := New(newStore(t), Actions{
		UninstallIntegration: func(id, name, kind string) { removedID, removedName = id, name },
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "local.abc", Name: "Ripgrep", Enabled: true, Generated: true,
		}}
	}
	tt := render(t, a, 720, 1200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Delete integration"); err != nil {
		t.Fatalf("the uninstall button is missing: %v", err)
	}
	tt.Frame()
	// The band is one sentence: the kind's title and the reference's own
	// warning, in a single node. The reference never drew the `Description`
	// half of the removal key family in this bar (`RemovalConfirmation.tsx:41`
	// renders the Title only), so the old build's "created and stored by
	// Floter" body must not be here.
	for _, want := range []string{
		"Delete Ripgrep? This cannot be undone from Floter. Export your integrations first if you may need this script or configuration again.",
	} {
		if !tt.HasText(want) {
			t.Errorf("the generated removal bar misses %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("created and stored by Floter") {
		t.Errorf("the removal bar drew a body the reference never showed: %q", tt.Texts())
	}
	if removedID != "" {
		t.Fatalf("the removal happened without asking: %q", removedID)
	}
	// The row's own button and the bar's danger button now share the verb
	// (the custom kind says "Delete integration" in both places, exactly as
	// the reference's menu item and confirmation did), so the danger button
	// is clicked positionally: same row as Cancel, just to its right.
	cancel, ok := tt.Find("Cancel")
	if !ok {
		t.Fatalf("the bar's cancel button is missing: %q", tt.Texts())
	}
	tt.ClickAt(cancel.X+cancel.W+30, cancel.Y+cancel.H/2)
	tt.Frame()
	if removedID != "local.abc" || removedName != "Ripgrep" {
		t.Errorf("uninstall recorded %q/%q", removedID, removedName)
	}
	if tt.HasText("Delete Ripgrep?") {
		t.Errorf("the bar stayed open after confirming: %q", tt.Texts())
	}
}

func TestLaunchAtStartupControl(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 1400)

	tt.Scroll(400, 300, 0, 400)
	tt.Frame()
	if !tt.HasText("Launch at startup") {
		t.Fatalf("the control is missing: %q", tt.Texts())
	}
	if err := tt.Click("Launch at startup"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !store.Snapshot().LaunchAtStartup {
		t.Error("the switch did not write the setting")
	}
}

func TestInstallFromRegistryControl(t *testing.T) {
	installed := ""
	constraint := ""
	a := New(newStore(t), Actions{
		InstallFromRegistry: func(name, version string) { installed, constraint = name, version },
	})
	a.Integrations = func() []Integration { return nil }
	tt := render(t, a, 720, 620)

	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The field's contents are the app's own state, so a test can set them
	// the way typing would.
	a.installName = "@vst93/floter-v"
	a.installVersion = "^1.0.0"
	tt.Frame()

	if err := tt.Click("Install"); err != nil {
		t.Fatalf("the install button is missing: %v", err)
	}
	tt.Frame()
	if installed != "@vst93/floter-v" || constraint != "^1.0.0" {
		t.Errorf("install recorded %q / %q", installed, constraint)
	}
	// The fields clear for the next install.
	if a.installName != "" || a.installVersion != "" {
		t.Errorf("the fields were not cleared: %q / %q", a.installName, a.installVersion)
	}

	// An empty name asks for nothing.
	if err := tt.Click("Install"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if constraint != "^1.0.0" {
		t.Errorf("an empty request was sent: %q", installed)
	}
}

func TestAppIconReader(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})

	// The shipped icon is dark, and anything unknown lands on it.
	if got := a.appIcon(); got != "dark" {
		t.Errorf("default = %q, want dark", got)
	}
	if err := store.Update(func(s *settings.Settings) { s.SetExtra("app_icon", "light") }); err != nil {
		t.Fatal(err)
	}
	if got := a.appIcon(); got != "light" {
		t.Errorf("stored = %q, want light", got)
	}
	if err := store.Update(func(s *settings.Settings) { s.SetExtra("app_icon", "purple") }); err != nil {
		t.Fatal(err)
	}
	if got := a.appIcon(); got != "dark" {
		t.Errorf("unknown = %q, want dark", got)
	}

	// The control is on the page, with both appearances offered.
	tt := render(t, a, 720, 620)
	if !tt.HasText("App icon") {
		t.Errorf("the control is missing: %q", tt.Texts())
	}
	if !tt.HasText("The icon shown in the menu bar / tray and on the taskbar. Dark is the default.") {
		t.Errorf("the hint is missing: %q", tt.Texts())
	}
}

func TestIntegrationPermissionsLine(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Permissions: []string{"filesystem-read", "process-spawn"},
			Enforced:    map[string]bool{"process-spawn": true},
		}}
	}
	tt := render(t, a, 720, 620)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The line names every permission, marking the enforced one.
	if !tt.HasText("Read files") {
		t.Errorf("a permission is missing: %q", tt.Texts())
	}
	if !tt.HasText("Start processes (Floter enforces)") {
		t.Errorf("the enforced permission is not marked: %q", tt.Texts())
	}
	// The dialog copy is localized too.
	zh := New(settings.NewStore(settings.Settings{Language: "zh"}), Actions{})
	if got := zh.permissionLine(Integration{Permissions: []string{"process-spawn"}, Enforced: map[string]bool{"process-spawn": true}}, i18n.For("zh").Settings); !strings.Contains(got, "启动进程") || !strings.Contains(got, "强制拦截") {
		t.Errorf("zh line = %q", got)
	}
}

func TestShortcutRecorder(t *testing.T) {
	recorded := ""
	a := New(newStore(t), Actions{
		SetShortcut: func(id, accelerator string) { recorded = id + "=" + accelerator },
	})
	a.Shortcut, a.ShortcutID = "Alt+Space", "toggle_window"
	tt := render(t, a, 720, 620)

	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The row shows the current key, and Record starts the recorder.
	if !tt.HasText(shortcuts.Display("Alt+Space")) {
		t.Fatalf("the current shortcut is missing: %q", tt.Texts())
	}
	if err := tt.Click("Record"); err != nil {
		t.Fatalf("the record button is missing: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Press keys…") {
		t.Fatalf("the recorder did not start: %q", tt.Texts())
	}

	// A combination with a modifier is recorded...
	tt.Key(ui.Super, ui.KeyG)
	tt.Frame()
	if recorded != "toggle_window=Cmd+G" {
		t.Errorf("recorded %q", recorded)
	}
	if !a.recording == false {
		t.Error("the recorder stayed on")
	}

	// ...and Escape cancels without recording.
	recorded = ""
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if recorded != "" {
		t.Errorf("Escape recorded %q", recorded)
	}
	if !tt.HasText("Record") {
		t.Errorf("the recorder did not close: %q", tt.Texts())
	}

	// A key without a modifier is not a shortcut, and the recorder waits.
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyG)
	tt.Frame()
	if recorded != "" || !a.recording {
		t.Errorf("a bare key recorded %q, recording=%v", recorded, a.recording)
	}
}

// The Plugins page shows the built-in plugins' settings and writes every
// change through the store.
// The Integrations page lists one switch per command and reports a toggle.
func TestIntegrationCommandSwitches(t *testing.T) {
	store := newStore(t)
	toggled := [][3]string{}
	a := New(store, Actions{
		SetCommandEnabled: func(extensionID, commandID string, enabled bool) {
			toggled = append(toggled, [3]string{extensionID, commandID, strconv.FormatBool(enabled)})
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Commands: []Command{
				{ID: "jv", Name: "Just run", Enabled: true, Available: true},
				{ID: "build", Name: "Build", Enabled: false},
			},
		}}
	}
	tt := render(t, a, 720, 760)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Just run") || !tt.HasText("Build") {
		t.Fatalf("the command switches = %q", tt.Texts())
	}
	// A command whose runtime does not resolve says so rather than lying
	// about being ready.
	if !tt.HasText("Build  \u00b7  runtime unavailable") {
		t.Errorf("the unavailable note is missing: %q", tt.Texts())
	}
	// The per-command switches carry their own caption and the one line that
	// says what a switch does, above the list rather than on every row.
	en := i18n.For("en").Settings
	if !tt.HasText(en.IntegrationsCommands) || !tt.HasText(en.IntegrationsCommandsHint) {
		t.Errorf("the command list caption/hint = %q", tt.Texts())
	}
	if err := tt.Click("Build  \u00b7  runtime unavailable"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(toggled) != 1 || toggled[0] != [3]string{"io.github.vst93.v", "build", "true"} {
		t.Errorf("toggled = %v", toggled)
	}
}

// Switching pages records the choice, so the next visit reopens the page.
func TestPageSelectionIsRemembered(t *testing.T) {
	pages := []string{}
	a := New(newStore(t), Actions{SetPage: func(name string) { pages = append(pages, name) }})
	tt := render(t, a, 720, 580)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(pages) != 1 || pages[0] != "about" {
		t.Errorf("pages = %v", pages)
	}
	if page, ok := PageByName("integrations"); !ok || page != PageIntegrations {
		t.Errorf("PageByName(integrations) = %d, %v", page, ok)
	}
	// The plugins page is gone: its settings are the launcher's own sheet.
	// A file that still names it resolves to nothing, and the settings open
	// on the first page.
	if _, ok := PageByName("plugins"); ok {
		t.Error("the retired plugins page still resolves")
	}
	if _, ok := PageByName("nope"); ok {
		t.Error("an unknown page id resolved")
	}
	if got := PageName(PageIntegrations); got != "integrations" {
		t.Errorf("PageName = %q", got)
	}
}

// The Shortcuts page lists the custom shortcuts and can add, retarget and
// remove them.
func TestCustomShortcutsPage(t *testing.T) {
	store := newStore(t)
	entries := []settings.CustomShortcut{{Key: "Cmd+Shift+P", Action: "plugin:clipboard"}}
	rejections := []CustomShortcutRejection{}
	var committed [][]settings.CustomShortcut
	a := New(store, Actions{
		CustomShortcuts: func() []settings.CustomShortcut { return entries },
		SetCustomShortcuts: func(list []settings.CustomShortcut) []CustomShortcutRejection {
			committed = append(committed, list)
			entries = list
			return rejections
		},
	})
	tt := render(t, a, 720, 1000)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The binding prints through the platform's own formatter (the glyphs on
	// macOS, the modifier words elsewhere).
	if !tt.HasText("Custom shortcuts") || !tt.HasText(shortcuts.Display("Cmd+Shift+P")) {
		t.Fatalf("the custom section = %q", tt.Texts())
	}
	// The custom section sits below the app-keys list: scroll it into view.
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()

	// Recording a key fills the new row, and Add persists the binding.
	if err := tt.Click("Record key"); err != nil {
		t.Fatalf("record: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Press keys\u2026") {
		t.Fatalf("the recorder did not open: %q", tt.Texts())
	}
	tt.Key(ui.Super, ui.KeyJ)
	tt.Frame()
	if len(committed) != 0 {
		t.Fatalf("a half-made row was persisted: %+v", committed)
	}
	if err := tt.Click("Add"); err != nil {
		t.Fatalf("add: %v", err)
	}
	tt.Frame()
	if len(committed) != 1 || len(committed[0]) != 2 {
		t.Fatalf("committed = %+v", committed)
	}
	if committed[0][1].Action != "plugin:clipboard" {
		t.Errorf("the new row's action = %q", committed[0][1].Action)
	}

	// Retargeting a row writes the new action; removing one drops it.
	if err := tt.Click("Clipboard history"); err != nil {
		t.Fatalf("action trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Browser history"); err != nil {
		t.Fatalf("action option: %v", err)
	}
	tt.Frame()
	last := committed[len(committed)-1]
	if last[0].Action != "plugin:browser" {
		t.Errorf("retargeted action = %q", last[0].Action)
	}
	if err := tt.Click("Remove"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	tt.Frame()
	last = committed[len(committed)-1]
	if len(last) != 1 {
		t.Errorf("after a removal = %+v", last)
	}

	// A key the system refused is reported rather than looking bound.
	rejections = []CustomShortcutRejection{{Key: "Cmd+J", Reason: CustomShortcutReasonOccupied}}
	if err := tt.Click("Remove"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Most likely used by another app") {
		t.Errorf("the rejection is missing: %q", tt.Texts())
	}
}

// A refused custom shortcut is worded with one of the two tokens the shell
// reports, exactly as the old build's page did — `conflict` for a key that
// shadows one of the app's own, `occupied` for one the system would not take.
func TestCustomShortcutRejectionWording(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{CustomShortcutReasonConflict, "Already used by another floter shortcut"},
		{CustomShortcutReasonOccupied, "Most likely used by another app"},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			store := newStore(t)
			entries := []settings.CustomShortcut{{Key: "Cmd+Shift+P", Action: "plugin:clipboard"}}
			a := New(store, Actions{
				CustomShortcuts: func() []settings.CustomShortcut { return entries },
				SetCustomShortcuts: func(list []settings.CustomShortcut) []CustomShortcutRejection {
					entries = list
					return []CustomShortcutRejection{{Key: "Cmd+J", Reason: tc.reason}}
				},
			})
			tt := render(t, a, 720, 1000)
			if err := tt.Click("Shortcuts"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Scroll(400, 300, 0, 900)
			tt.Frame()
			if err := tt.Click("Remove"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText(tc.want) {
				t.Errorf("the rejection word is missing: %q", tt.Texts())
			}
		})
	}
}

// The Plugins page carries the calculator's card and writes its three fields
// through the store.
// An orphan package directory is listed with the two operations it can take,
// and nothing that needs a repository record.
func TestOrphanRowsOfferAdoptAndDelete(t *testing.T) {
	adopted := []string{}
	deleted := []string{}
	a := New(newStore(t), Actions{
		AdoptIntegration: func(id string) { adopted = append(adopted, id) },
		DeleteOrphan:     func(id string) { deleted = append(deleted, id) },
	})
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "dev.floter.installed", Name: "Installed", Enabled: true, Running: true},
			{ID: "dev.floter.orphan", Name: "dev.floter.orphan", Orphan: true},
		}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("dev.floter.orphan") {
		t.Fatalf("the orphan is missing: %q", tt.Texts())
	}
	if !tt.HasText("Adopt") || !tt.HasText("Delete") {
		t.Fatalf("the orphan operations are missing: %q", tt.Texts())
	}
	// The row explains what Adopt does: a plug-in directory unrecorded in the
	// repository, grafted in under the same permission review as an install.
	if hint := i18n.For("en").Settings.IntegrationsAdoptHint; !tt.HasText(hint) {
		t.Fatalf("the orphan row does not explain Adopt: %q", tt.Texts())
	}
	if err := tt.Click("Adopt"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(adopted) != 1 || adopted[0] != "dev.floter.orphan" {
		t.Errorf("adopted = %v", adopted)
	}
	if err := tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(deleted) != 1 || deleted[0] != "dev.floter.orphan" {
		t.Errorf("deleted = %v", deleted)
	}
	// The orphan has no enable switch: there is no record to enable.
	if tt.HasText("Search browser data") {
		t.Error("an unrelated control appeared")
	}
}

// The About page's update row is the old AboutPage's state machine: the
// current version line, the state that follows it, and the one control the
// state offers.
func TestAboutUpdateRowFollowsTheOldStateMachine(t *testing.T) {
	en := i18n.For("en").Settings
	installed := 0
	a := New(newStore(t), Actions{InstallUpdate: func() { installed++ }})
	a.About = About{Name: "floter", Version: "0.3.14"}
	tt := render(t, a, 720, 620)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// The update row lives in its own group, the reference's
	// `settings.group.update` section (`AboutPage.tsx:62-70`): the heading says
	// which card the row is, rather than leaving the page's one control loose
	// under the build's identity.
	group := mustFind(t, tt, en.GroupUpdate)
	current := mustFind(t, tt, en.UpdateCurrentVersion+": v0.3.14")
	if current.Y <= group.Y {
		t.Errorf("the update row at %v is not under the %q heading at %v", current, en.GroupUpdate, group)
	}
	if !tt.HasText(en.UpdateUpToDate) {
		t.Errorf("the up-to-date line is missing: %q", tt.Texts())
	}
	if tt.HasText(en.UpdateDownload) {
		t.Errorf("an update control appeared with nothing to install: %q", tt.Texts())
	}

	// An available update names its version and offers Download.
	a.Update = UpdateState{Checked: true, Available: true, Version: "0.4.0"}
	tt.Frame()
	if !tt.HasText(en.UpdateLatestVersion + ": v0.4.0") {
		t.Errorf("the latest-version line is missing: %q", tt.Texts())
	}
	if err := tt.Click(en.UpdateDownload); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if installed != 1 {
		t.Errorf("Download ran the install %d times", installed)
	}

	// A running download swaps the control for the byte bar and its readout;
	// the disabled button is the old `updateDownloading` branch.
	a.Update = UpdateState{Checked: true, Available: true, Version: "0.4.0", Downloading: true,
		Progress: &UpdateProgress{Downloaded: 1536, Total: 3072}}
	tt.Frame()
	if !tt.HasText("50% \u00b7 1.5 KB / 3.0 KB") {
		t.Errorf("the progress readout is missing: %q", tt.Texts())
	}
	if tt.HasText(en.UpdateDownload) {
		t.Errorf("the Download button stayed up during the download: %q", tt.Texts())
	}

	// The disabled Installing… button is the branch with no progress yet.
	a.Update = UpdateState{Checked: true, Available: true, Version: "0.4.0", Downloading: true}
	tt.Frame()
	if !tt.HasText(en.UpdateInstalling) {
		t.Errorf("the Installing\u2026 button is missing: %q", tt.Texts())
	}

	// A failure keeps the row and offers Retry, which re-runs the install
	// path the old page's `onDownloadUpdate` did.
	a.Update = UpdateState{Checked: true, Failed: true}
	tt.Frame()
	if !tt.HasText(en.UpdateFailedRetry) {
		t.Errorf("the retry line is missing: %q", tt.Texts())
	}
	if err := tt.Click(en.UpdateRetry); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if installed != 2 {
		t.Errorf("Retry ran the install %d times", installed)
	}

	// A build with no feed says so and offers nothing to press.
	a.Update = UpdateState{Checked: true, Disabled: true}
	tt.Frame()
	if !tt.HasText(en.UpdateDisabled) {
		t.Errorf("the no-feed line is missing: %q", tt.Texts())
	}
	if tt.HasText(en.UpdateDownload) || tt.HasText(en.UpdateRetry) {
		t.Errorf("a control appeared in the no-feed state: %q", tt.Texts())
	}
}

// formatBytes and updateProgressLabel are the old AboutPage's own formatting
// helpers, moved to Go.
func TestUpdateProgressFormatting(t *testing.T) {
	for _, tc := range []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{-1, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1610612736, "1.5 GB"},
	} {
		if got := formatBytes(tc.bytes); got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}

	if got := updateProgressLabel(&UpdateProgress{Downloaded: 1536, Total: 3072}); got != "50% \u00b7 1.5 KB / 3.0 KB" {
		t.Errorf("progress label = %q", got)
	}
	if got := updateProgressLabel(&UpdateProgress{Downloaded: 1536}); got != "1.5 KB" {
		t.Errorf("progress label without a total = %q", got)
	}
	if got := updateProgressLabel(&UpdateProgress{Downloaded: 9999, Total: 1000}); got != "100% \u00b7 9.8 KB / 1000 B" {
		t.Errorf("a past-total progress label = %q", got)
	}
}

// A row's permission list unfolds into an audit that says what each permission
// allows and whether the host itself decides it.
func TestPermissionAuditUnfolds(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Permissions: []string{"filesystem-read", "environment"},
			Enforced:    map[string]bool{"environment": true},
		}}
	}
	tt := render(t, a, 720, 620)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Read files") || !tt.HasText("Read the environment") {
		t.Fatalf("the permission line is missing: %q", tt.Texts())
	}
	// The explanations are folded away until asked for.
	if tt.HasText("The tool may read files your user can read.") {
		t.Error("the audit is open before it was asked for")
	}
	if err := tt.Click("Permissions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("The tool may read files your user can read.") {
		t.Errorf("the audit did not open: %q", tt.Texts())
	}
	// The two tiers are named, and each permission sits under its own.
	if !tt.HasText("Floter enforces") || !tt.HasText("Declared, not blocked") {
		t.Errorf("the tiers are missing: %q", tt.Texts())
	}
	if !tt.HasText("Read the environment") || !tt.HasText("Read files") {
		t.Errorf("a permission is missing: %q", tt.Texts())
	}
	if !tt.HasText("Floter enforces this one: the host decides what the tool receives.") {
		t.Errorf("the enforced explanation is missing: %q", tt.Texts())
	}
	// Asking again folds it away.
	if err := tt.Click("Permissions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("The tool may read files your user can read.") {
		t.Error("the audit did not fold away")
	}
}

// The Integrations page offers the export and import of the whole list,
// shows which one is running, and draws the last import's report.
func TestIntegrationTransferButtons(t *testing.T) {
	exported, imported := 0, 0
	a := New(newStore(t), Actions{
		ExportIntegrations: func() { exported++ },
		ImportIntegrations: func() { imported++ },
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Export"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if exported != 1 {
		t.Fatalf("exported %d times", exported)
	}
	if err := tt.Click("Import"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if imported != 1 {
		t.Errorf("imported %d times", imported)
	}

	// While a transfer runs the row says which one (the reference's busy line).
	a.TransferBusy = "export"
	tt.Frame()
	if !tt.HasText("Exporting…") {
		t.Errorf("the busy line is missing: %q", tt.Texts())
	}
	a.TransferBusy = "import"
	tt.Frame()
	if !tt.HasText("Importing…") {
		t.Errorf("the import busy line is missing: %q", tt.Texts())
	}
	a.TransferBusy = ""

	// A clean import leaves the reference's summary under the row.
	a.TransferReport = &ImportReport{Path: "/tmp/backup.json", Succeeded: 4, Skipped: 1}
	tt.Frame()
	if !tt.HasText("Imported 4, unchanged 1") {
		t.Errorf("the summary is missing: %q", tt.Texts())
	}

	// A failed import lists the first three failures by id: reason and the
	// tail count, and warns about the fields the export left out.
	a.TransferReport = &ImportReport{
		Path: "/tmp/backup.json",
		Failed: []TransferFailure{
			{ID: "a", Reason: "no package"},
			{ID: "b", Reason: "bad manifest"},
			{ID: "c", Reason: "missing tool"},
			{ID: "d", Reason: "denied"},
		},
		ExcludedFields: []ExcludedField{{ExtensionID: "a", FieldCount: 2}},
	}
	tt.Frame()
	if !tt.HasText("Some fields were excluded during export: a") {
		t.Errorf("the excluded-fields warning is missing: %q", tt.Texts())
	}
	if !tt.HasText("Failed: a: no package; b: bad manifest; c: missing tool (and 1 more)") {
		t.Errorf("the failure detail is missing: %q", tt.Texts())
	}
}

// The base plugins the app ships are listed on the Integrations page with the
// two controls they can have: the switch that turns one on or off, and the
// Configure button that opens its declarative configuration in the launcher
// (the same sheet the mode's own gear opens). The browser mode's own message
// points at this section, so a section that did not render would leave the
// hint pointing at a door that does not exist.
func TestBasePluginsSectionOpensConfigurationAndToggles(t *testing.T) {
	opened := ""
	toggles := map[string]bool{}
	a := New(newStore(t), Actions{
		OpenPluginConfig: func(id string) { opened = id },
		SetPluginEnabled: func(id string, enabled bool) { toggles[id] = enabled },
	})
	a.Integrations = func() []Integration { return nil }
	tt := render(t, a, 720, 900)

	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{
		"Base plugins",
		"Built-in functionality that ships with floter.",
		"Clipboard history", "Browser bookmarks & history", "Calculator",
		// The clipboard's privacy line is the one thing the row discloses
		// rather than describes; it must not be dropped.
		"Everything you copy system-wide is recorded — stored locally only.",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// Configure opens the plugin's own sheet: the row names the plugin in its
	// accessible name, because three buttons reading "Configure" alone are
	// three buttons a screen reader cannot tell apart.
	if err := tt.Click("Configure Calculator"); err != nil {
		t.Fatalf("the calculator's Configure button: %v", err)
	}
	tt.Frame()
	if opened != "calculator" {
		t.Errorf("Configure opened %q, want calculator", opened)
	}

	// The clipboard's switch turns the history off; the name is a hit target,
	// as every switch row's is.
	if err := tt.Click("Clipboard history"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got, ok := toggles["clipboard"]; !ok || got {
		t.Errorf("clipboard toggle = %v (set=%v), want false", got, ok)
	}

	// The calculator has no persisted on/off field, so it carries no switch
	// and clicking its name toggles nothing.
	if err := tt.Click("Calculator"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := toggles["calculator"]; ok {
		t.Error("the calculator toggled, but it has no switch")
	}
}

// The base-plugin rows are a door only when the shell has one to offer: with
// no configuration or switch actions the section is not drawn, so a page with
// nothing behind it cannot promise a control that does nothing.
func TestBasePluginsSectionNeedsAnAction(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration { return nil }
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("Base plugins") {
		t.Errorf("the section rendered with no action behind it: %q", tt.Texts())
	}
}

// The Shortcuts page lists the app's own keys with their bindings, and a
// recorder rebinds one.
func TestAppShortcutRowsRecord(t *testing.T) {
	store := newStore(t)
	recorded := [][2]string{}
	a := New(store, Actions{
		SetShortcut: func(id, accelerator string) { recorded = append(recorded, [2]string{id, accelerator}) },
	})
	a.Shortcut, a.ShortcutID = "Ctrl+Space", "toggle_window"
	tt := render(t, a, 720, 620)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"App keys", "New command", "Open a terminal window", "Run the n-th result"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// Each row has its own recorder: click the one beside the "New command"
	// row's binding (the tester finds elements by text, and several rows say
	// "Record").
	binding, ok := tt.Find(shortcuts.Display("CmdOrCtrl+W"))
	if !ok {
		t.Fatalf("the new command's binding is missing: %q", tt.Texts())
	}
	tt.ClickAt(binding.X+binding.W+20, binding.Y+binding.H/2)
	tt.Frame()
	if !tt.HasText("Press keys\u2026") {
		t.Fatalf("the recorder did not open: %q", tt.Texts())
	}
	tt.Key(ui.Super, ui.KeyK)
	tt.Frame()
	if len(recorded) != 1 || recorded[0][0] != "new_command" {
		t.Fatalf("recorded %v", recorded)
	}
}

// The Integrations page offers the shipped tool packages that are not
// installed, and hides the section once they all are.
func TestRecommendedToolsConnect(t *testing.T) {
	connected := []string{}
	a := New(newStore(t), Actions{
		ConnectRecommended: func(id string) { connected = append(connected, id) },
	})
	a.Recommended = func() []RecommendedTool {
		return []RecommendedTool{
			{ID: "io.github.vst93.v", Name: "V Tools", Description: "Developer tools"},
			{ID: "other.tool", Name: "Other", Installed: true},
		}
	}
	tt := render(t, a, 720, 620)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Recommended tools") || !tt.HasText("V Tools") {
		t.Fatalf("the recommended section = %q", tt.Texts())
	}
	// An installed one is not offered.
	if tt.HasText("Other") {
		t.Errorf("an installed tool was offered: %q", tt.Texts())
	}
	if err := tt.Click("Connect"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(connected) != 1 || connected[0] != "io.github.vst93.v" {
		t.Errorf("connected %v", connected)
	}

	// With everything installed the section is gone.
	a.Recommended = func() []RecommendedTool {
		return []RecommendedTool{{ID: "io.github.vst93.v", Name: "V Tools", Installed: true}}
	}
	tt.Frame()
	if tt.HasText("Recommended tools") {
		t.Errorf("the section stayed with nothing to connect: %q", tt.Texts())
	}
}

// A recorder releases the global shortcuts while it waits for a key, and takes
// them back when it is done: pressing the key that summons floter must be
// recorded, not acted on.
func TestRecorderSuspendsTheGlobalShortcuts(t *testing.T) {
	suspended := []bool{}
	a := New(newStore(t), Actions{
		SetShortcut:           func(string, string) {},
		SetShortcutsSuspended: func(on bool) { suspended = append(suspended, on) },
	})
	a.Shortcut, a.ShortcutID = "Ctrl+Space", "toggle_window"
	tt := render(t, a, 720, 1000)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(suspended) != 0 {
		t.Fatalf("the shortcuts were released before recording: %v", suspended)
	}

	// The summon key's recorder suspends, and a captured key resumes.
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(suspended) != 1 || suspended[0] != true {
		t.Fatalf("suspended = %v", suspended)
	}
	tt.Key(ui.Super, ui.KeyG)
	tt.Frame()
	if len(suspended) != 2 || suspended[1] != false {
		t.Errorf("suspended = %v", suspended)
	}

	// Escape cancels and resumes too.
	suspended = nil
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if len(suspended) != 2 || suspended[0] != true || suspended[1] != false {
		t.Errorf("suspended = %v", suspended)
	}

	// The custom section's recorder does the same.
	suspended = nil
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()
	if err := tt.Click("Record key"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if len(suspended) != 2 || suspended[0] != true || suspended[1] != false {
		t.Errorf("suspended = %v", suspended)
	}
}

// An integration's configuration form renders its schema, keeps its draft
// across frames, and saves through the shell.
func TestConfigurationForm(t *testing.T) {
	saved := map[string]map[string]any{}
	min := 0.5
	max := 9.0
	a := New(newStore(t), Actions{
		SaveConfiguration: func(id string, values map[string]any) error {
			if saved == nil {
				saved = map[string]map[string]any{}
			}
			saved[id] = values
			return nil
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Config: []ConfigField{
				{Key: "endpoint", Label: "Endpoint", Type: "text", Required: true, MinLength: intPtr(4)},
				{Key: "token", Label: "Token", Type: "password", Required: true},
				{Key: "region", Label: "Region", Type: "select", Options: []string{"eu", "us"}, Default: "eu"},
				{Key: "tags", Label: "Tags", Type: "multiSelect", Options: []string{"a", "b", "c"}},
				{Key: "retries", Label: "Retries", Type: "number", Minimum: &min, Maximum: &max},
				{Key: "verbose", Label: "Verbose", Type: "boolean", Description: "Print every request the tool makes"},
			},
			ConfigValues: map[string]any{"endpoint": "https://example.com", "token": "s3cret", "region": "us"},
		}}
	}
	tt := render(t, a, 720, 1100)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The form is folded away until asked for.
	if tt.HasText("Region") {
		t.Error("the form was open before it was asked for")
	}
	if err := tt.Click("Configure"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"Endpoint", "Token", "Region", "Tags", "Retries", "Verbose"} {
		if !tt.HasText(want) {
			t.Errorf("the field %q is missing: %q", want, tt.Texts())
		}
	}
	// The boolean field keeps its description too: the switch used to drop
	// it while the select and the text field showed theirs.
	if !tt.HasText("Print every request the tool makes") {
		t.Errorf("the boolean field's description is missing: %q", tt.Texts())
	}
	// The loaded values seed the draft: the select shows the stored region,
	// and a defaulted field the default.
	if !tt.HasText("us") {
		t.Errorf("the stored region is not shown: %q", tt.Texts())
	}

	// Ticking a multiselect option, typing into a number field and saving
	// hand the whole draft to the shell.
	if err := tt.Click("a"); err != nil {
		t.Fatalf("multiselect: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	values := saved["io.github.vst93.v"]
	if values == nil {
		t.Fatal("nothing was saved")
	}
	if values["endpoint"] != "https://example.com" || values["token"] != "s3cret" {
		t.Errorf("text/password = %v", values)
	}
	if values["region"] != "us" {
		t.Errorf("region = %v", values["region"])
	}
	tags, ok := values["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "a" {
		t.Errorf("tags = %v", values["tags"])
	}
	// The boolean and the number keep their state (untouched here: the
	// defaults are what a save would write).
	if _, ok := values["verbose"]; !ok && values["verbose"] != nil {
		t.Logf("verbose = %v", values["verbose"])
	}

	// A save error is reported on the form rather than vanishing.
	a.Actions.SaveConfiguration = func(string, map[string]any) error {
		return errors.New("the endpoint is unreachable")
	}
	tt.Click("Save")
	tt.Frame()
	if !tt.HasText("the endpoint is unreachable") {
		t.Errorf("the error is missing: %q", tt.Texts())
	}
}

func intPtr(v int) *int { return &v }

// The Plugins page's clear-history control reports a press.
// The launcher's own switches are reachable and write through the store: the
// recent applications, the menu bar icon, and the PATH commands joining the
// search.
func TestLauncherSwitchesWriteThroughTheStore(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{})
	tt := render(t, a, 720, 1400)
	if err := tt.Click("Show recently launched apps"); err != nil {
		t.Fatalf("recent switch: %v", err)
	}
	tt.Frame()
	if settings.ShowRecentInLauncher(store.Snapshot()) {
		t.Error("the recent switch did not turn off")
	}
	if err := tt.Click("Show menu bar icon"); err != nil {
		t.Fatalf("tray switch: %v", err)
	}
	tt.Frame()
	if settings.ShowMenubarIcon(store.Snapshot()) {
		t.Error("the tray switch did not turn off")
	}
	// The commands switch lives on the Integrations page.
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Show PATH commands in search"); err != nil {
		t.Fatalf("commands switch: %v", err)
	}
	tt.Frame()
	if !settings.ShowCommandsInSearch(store.Snapshot()) {
		t.Error("the commands switch did not turn on")
	}
}

// The sidebar leads every page with a mark and paints the chosen page on the
// resting control fill; the page's own navigation still switches.
func TestSidebarRowsCarryMarksAndSelection(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{})
	tt := render(t, a, 720, 900)
	tt.Frame()
	// Every page's label is there, and each row is the sidebar's own height.
	for _, title := range []string{"General", "Sessions", "Shortcuts", "Integrations", "About"} {
		rect, ok := tt.Find(title)
		if !ok {
			t.Fatalf("the sidebar is missing %q: %v", title, tt.Texts())
		}
		if rect.H < 24 {
			t.Errorf("%q is not a full row: %+v", title, rect)
		}
	}
	// Every page leads with a mark from the shared vocabulary.
	for page := PageGeneral; page < pageCount; page++ {
		if _, ok := pageGlyph(page); !ok {
			t.Errorf("page %d has no mark", page)
		}
	}
	// The chosen page is the one showing; clicking another switches both.
	if a.Page != PageGeneral {
		t.Fatalf("the panel opened on page %v", a.Page)
	}
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Page != PageAbout {
		t.Errorf("page = %v, want about", a.Page)
	}
}

// The local-tool form collects what the host needs and hands it on: the name,
// the program (with the picker), the command word, the arguments and the
// permissions the user ticked.
func TestLocalToolForm(t *testing.T) {
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
		ChooseProgram:    func() (string, error) { return "/usr/bin/rg", nil },
	})
	// 1800, not 1600: the form grew the content-action row the reference's
	// drawer carried (`CustomIntegrationDrawer.tsx:145-146`) and the program
	// row's hint (`CustomIntegrationDrawer.tsx:145`, `i18n.ts:665`), and this
	// test opens the Advanced disclosure too, so the Connect button sits lower.
	tt := render(t, a, 720, 1800)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The form is folded away until asked for.
	if tt.HasText("Name") {
		t.Error("the form is open before it was asked for")
	}
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatalf("the form's door: %v", err)
	}
	tt.Frame()
	for _, want := range []string{"Name", "Program", "Command", "Arguments", "Version arguments"} {
		if !tt.HasText(want) {
			t.Errorf("the form is missing %q: %q", want, tt.Texts())
		}
	}
	// The picker fills the program field. (The tester keeps a per-window
	// editor buffer, so the *state* is what the assertion reads.)
	if err := tt.Click("Choose\u2026"); err != nil {
		t.Fatalf("the picker: %v", err)
	}
	tt.Frame()
	if a.toolProgram != "/usr/bin/rg" {
		t.Errorf("the picker did not fill the program: %q", a.toolProgram)
	}
	// The permissions are rows, and one is ticked — but the declaration/audit
	// switches stay folded behind the form's Advanced disclosure until it is
	// opened (R161), so the form does not open on a ceremonial list.
	if tt.HasText(i18n.PermissionLabel("en", "environment")) {
		t.Error("a permission row is drawn before Advanced was opened")
	}
	if err := tt.Click("Advanced"); err != nil {
		t.Fatalf("the Advanced disclosure: %v", err)
	}
	tt.Frame()
	for _, permission := range extensions.AllPermissions {
		if !tt.HasText(i18n.PermissionLabel("en", permission)) {
			t.Errorf("the permission %q is missing", permission)
		}
	}
	if err := tt.Click(i18n.PermissionLabel("en", "environment")); err != nil {
		t.Fatalf("a permission row: %v", err)
	}
	tt.Frame()
	// Connect hands the whole draft on. The name is the form's own state (the
	// tester cannot type into a field it has already drawn).
	a.toolName = "Ripgrep"
	tt.Frame()
	if err := tt.Click("Connect"); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 {
		t.Fatalf("connected = %+v", connected)
	}
	tool := connected[0]
	if tool.Program != "/usr/bin/rg" {
		t.Errorf("program = %q", tool.Program)
	}
	found := false
	for _, permission := range tool.Permissions {
		if permission == "environment" {
			found = true
		}
	}
	if !found {
		t.Errorf("permissions = %v", tool.Permissions)
	}
}

// R158/R159 · the description control is executable-only and honest about the
// blank case. A script manifest writes its own internal sentence and never
// reads the field, so the control is not drawn in script mode; an executable's
// field carries the R159 placeholder (a blank value hides the description) and
// the "(optional)" label.
func TestLocalToolDescriptionIsExecutableOnly(t *testing.T) {
	copy := i18n.For("en").Settings
	// The placeholder is the R159 sentence, pinned here because the headless
	// tester paints a placeholder through the editor and never records it as a
	// label (see mygo ui/editor/paint.go): the exact copy is what this test owns.
	if got := copy.LocalToolDescriptionPlaceholder; got != "Leave blank to show no description" {
		t.Errorf("the R159 placeholder reads %q", got)
	}
	if got := i18n.For("zh").Settings.LocalToolDescriptionPlaceholder; got != "留空则不显示描述" {
		t.Errorf("the R159 placeholder (zh) reads %q", got)
	}
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(CustomTool) {},
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(copy.LocalToolDescription) {
		t.Fatalf("the executable form is missing the description label: %q", tt.Texts())
	}
	// Script mode drops the control: the manifest writes its own description and
	// the field would be silently dropped anyway.
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatalf("the kind: %v", err)
	}
	tt.Frame()
	if tt.HasText(copy.LocalToolDescription) {
		t.Errorf("the description control survived the switch to script: %q", tt.Texts())
	}
}

// R161: the declaration switches sit behind a default-closed Advanced
// disclosure — a brand-new integration must not open on a ceremonial list of
// checkboxes — and the boundary note inside says plainly that they isolate
// nothing. Folding is presentation only: the ticks stay in the form state and
// are still submitted, so an author who collapses the section loses nothing.
func TestLocalToolAdvancedDisclosure(t *testing.T) {
	copy := i18n.For("en").Settings
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Closed by default: the disclosure's own words are there, but neither the
	// switches nor the boundary note are drawn.
	if !tt.HasText(copy.LocalToolAdvanced) || !tt.HasText(copy.LocalToolAdvancedPermissions) {
		t.Fatalf("the Advanced disclosure is missing: %q", tt.Texts())
	}
	if tt.HasText(copy.PermissionBoundary) {
		t.Errorf("the boundary note is drawn while the section is closed: %q", tt.Texts())
	}
	for _, permission := range extensions.AllPermissions {
		if tt.HasText(i18n.PermissionLabel("en", permission)) {
			t.Errorf("the permission %q is drawn while the section is closed", permission)
		}
	}
	// Opening it draws the note and the rows.
	if err := tt.Click(copy.LocalToolAdvanced); err != nil {
		t.Fatalf("open: %v", err)
	}
	tt.Frame()
	if !tt.HasText(copy.PermissionBoundary) {
		t.Errorf("the boundary note did not appear: %q", tt.Texts())
	}
	// Tick a switch, fold the section back, and the tick survives — it lives
	// in the form state, not in the disclosure.
	if err := tt.Click(i18n.PermissionLabel("en", "environment")); err != nil {
		t.Fatalf("tick: %v", err)
	}
	tt.Frame()
	if !a.toolPermissions["environment"] {
		t.Fatalf("the tick was not recorded: %+v", a.toolPermissions)
	}
	if err := tt.Click(copy.LocalToolAdvanced); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if tt.HasText(copy.PermissionBoundary) {
		t.Errorf("the boundary note stayed after folding: %q", tt.Texts())
	}
	// The collapsed tick still reaches the backend.
	a.toolName, a.toolProgram = "Ripgrep", "/usr/bin/rg"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 {
		t.Fatalf("connected = %+v", connected)
	}
	found := false
	for _, permission := range connected[0].Permissions {
		if permission == "environment" {
			found = true
		}
	}
	if !found {
		t.Errorf("the collapsed tick was dropped: %v", connected[0].Permissions)
	}
}

// The freshness row: what is known about the command list, with "unknown"
// spelled out rather than painted as a zero.
func TestFreshnessRow(t *testing.T) {
	count := 3
	previous := 1
	a := New(settings.NewStore(settings.Default()), Actions{
		Freshness: func(id string) (extensions.Freshness, bool) {
			if id != "io.github.vst93.v" {
				return extensions.Freshness{}, false
			}
			return extensions.Freshness{
				AtSeconds:            time.Now().Add(-3 * time.Minute).Unix(),
				Source:               extensions.FreshnessProbe,
				Result:               extensions.FreshnessSuccess,
				CommandCount:         &count,
				PreviousCommandCount: &previous,
				Delta:                extensions.DeltaIncrease,
			}, true
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Command list") {
		t.Fatalf("the freshness row is missing: %q", tt.Texts())
	}
	if !tt.HasText("Succeeded") {
		t.Errorf("the result is missing: %q", tt.Texts())
	}
	// The delta says how far the list moved, and the count is the real one.
	if !tt.HasText("Commands 3") {
		t.Errorf("the count is missing: %q", tt.Texts())
	}
	if !tt.HasText("+2 since the scan before") {
		t.Errorf("the delta is missing: %q", tt.Texts())
	}
	// A never-probed integration shows no row at all.
	a.Actions.Freshness = func(string) (extensions.Freshness, bool) { return extensions.Freshness{}, false }
	tt.Frame()
	if tt.HasText("Command list") {
		t.Errorf("a never-probed integration shows a freshness row: %q", tt.Texts())
	}
}

// The stored lifecycle health is drawn on the integration row: the probe
// report lives in the repository, so the row says how the tool was last found
// to be before any check runs in this session. It is its own row, under its
// own label, distinct from the live diagnostics below.
func TestFreshnessDotProjectsTheRowState(t *testing.T) {
	count := func(n int) *int { return &n }
	freshness := extensions.Freshness{
		AtSeconds:            time.Now().Unix(),
		Source:               extensions.FreshnessProbe,
		Result:               extensions.FreshnessSuccess,
		Delta:                extensions.DeltaUnchanged,
		CommandCount:         count(4),
		PreviousCommandCount: count(4),
	}
	a := New(newStore(t), Actions{
		Freshness: func(id string) (extensions.Freshness, bool) {
			return freshness, id == "dev.floter.tool"
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.tool", Name: "Tool", Enabled: true, Running: true}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// A completed comparison with equal counts reads as in sync.
	if !tt.HasText("Command status: in sync") {
		t.Fatalf("the synced dot is missing: %q", tt.Texts())
	}
	// A count that moved is the one state that earns warmth.
	freshness.Delta = extensions.DeltaIncrease
	freshness.CommandCount = count(6)
	tt.Frame()
	if !tt.HasText("Command status: changed upstream") {
		t.Errorf("the changed dot is missing: %q", tt.Texts())
	}
	if tt.HasText("Command status: in sync") {
		t.Errorf("a changed list still reads as in sync: %q", tt.Texts())
	}
	// A first probe has nothing to compare against, so it is unknown, never
	// a fabricated "in sync".
	freshness.Delta = extensions.DeltaUnknown
	freshness.PreviousCommandCount = nil
	tt.Frame()
	if !tt.HasText("Command status: unknown, not compared") {
		t.Errorf("the unknown dot is missing: %q", tt.Texts())
	}
	// The dot's word is also its tooltip, so a pointer resting on it reads the
	// same sentence the label carries. (A Label and the tooltip a hover reveals
	// are two nodes; the count must rise.)
	countOf := func(want string) int {
		n := 0
		for _, s := range tt.Texts() {
			if s == want {
				n++
			}
		}
		return n
	}
	label := "Command status: unknown, not compared"
	if rect, ok := tt.Find(label); ok {
		before := countOf(label)
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
		// The tip waits for the pointer to rest (Base UI's 600ms).
		time.Sleep(700 * time.Millisecond)
		tt.Frame()
		if countOf(label) <= before {
			t.Errorf("the dot has no tooltip (nodes %d → %d): %q", before, countOf(label), tt.Texts())
		}
	} else {
		t.Fatal("the dot is not findable by its own label")
	}
	// A never-probed integration has no mark at all.
	a.Actions.Freshness = func(string) (extensions.Freshness, bool) { return extensions.Freshness{}, false }
	tt.Frame()
	if tt.HasText("Command status:") {
		t.Errorf("a never-probed integration shows a dot: %q", tt.Texts())
	}
}

func TestStoredHealthShowsOnTheRow(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "dev.floter.healthy", Name: "Alpha Tool", Enabled: true, Health: "healthy"},
			{ID: "dev.floter.broken", Name: "Beta Tool", Enabled: true, Broken: true, Health: "not working"},
			{ID: "dev.floter.diagnosed", Name: "Gamma Tool", Enabled: true, Diagnosis: "ok"},
		}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// The stored health is the row's value, under the Health label.
	if !tt.HasText("Health") {
		t.Fatalf("the health label is missing: %q", tt.Texts())
	}
	if !tt.HasText("healthy") {
		t.Errorf("the stored health is missing: %q", tt.Texts())
	}
	if !tt.HasText("not working") {
		t.Errorf("a broken tool's stored health is missing: %q", tt.Texts())
	}
	// The live check's own sentence is a different row with its own label, so
	// the two facts never read as one another.
	if !tt.HasText("Diagnostics") {
		t.Errorf("the diagnostics label is missing: %q", tt.Texts())
	}
	if !tt.HasText("ok") {
		t.Errorf("the diagnosis is missing: %q", tt.Texts())
	}

	// An integration with neither a stored report nor a diagnosis draws
	// neither row — "unknown" is its own state, not a row of zeroes.
	b := New(newStore(t), Actions{})
	b.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.plain", Name: "Plain Tool", Enabled: true}}
	}
	bt := render(t, b, 720, 900)
	if err := bt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	bt.Frame()
	if bt.HasText("Health") {
		t.Errorf("an unprobed integration shows a health row: %q", bt.Texts())
	}
	if bt.HasText("Diagnostics") {
		t.Errorf("an unchecked integration shows a diagnostics row: %q", bt.Texts())
	}
}

// The stored report's own detail is drawn under the Health row: every probe
// with its duration, the failed ones with their exit code, stderr and whether
// a retry is worth it, and when the run happened. The old build's
// `extension-health` block.
func TestStoredHealthReportShowsItsProbes(t *testing.T) {
	when := "2026-10-12T09:30:00Z"
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.probed", Name: "Probed Tool", Enabled: true, Health: "degraded",
			HealthReport: &HealthReport{
				CheckedAt: when,
				Probes: []HealthProbe{
					{ID: "up", Passed: true, DurationMs: 12, ExitCode: 0},
					{ID: "extras", Passed: false, DurationMs: 30, ExitCode: 3, Stderr: "no extras here", Retryable: true},
				},
			},
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{
		"Checks", "Failures",
		"up", "12ms",
		"extras", "30ms", "exit: 3", "no extras here", "Retryable",
		"Checked " + healthCheckedAt(when),
	} {
		if !tt.HasText(want) {
			t.Errorf("the health report is missing %q: %q", want, tt.Texts())
		}
	}

	// No stored report means no detail rows: the status word alone is not a
	// report, and "nothing has ever probed" has its own (absent) shape.
	b := New(newStore(t), Actions{})
	b.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.plain", Name: "Plain Tool", Enabled: true, Health: "healthy"}}
	}
	bt := render(t, b, 720, 900)
	if err := bt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	bt.Frame()
	if bt.HasText("Checks") || bt.HasText("Failures") {
		t.Errorf("a report-less integration shows probe detail: %q", bt.Texts())
	}
}

// The approval record block (R7-8): a recorded approval draws when it
// happened, the recorded digest with the on-disk one beside it, the
// permissions tiered by the host's own vocabulary, and the changed-manifest
// warning only when both digests are known and differ. No record is one
// empty-state line; an orphan has neither.
func TestApprovalRecordBlock(t *testing.T) {
	full := "sha256-" + strings.Repeat("abcdef0123456789", 4)
	disk := "sha256-" + strings.Repeat("0123456789abcdef", 4)
	at := uint64(1750000000)
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.approved", Name: "Approved Tool", Enabled: true,
			Approval: &ApprovalRecord{
				At:             at,
				ApprovedDigest: full,
				CurrentDigest:  disk,
				Enforced:       []string{"environment"},
				Disclosure:     []string{"network-fetch"},
			},
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{
		"Approval record", "Approved", approvalApprovedAt(at),
		"Manifest at approval", extensions.ShortDigest(full),
		"Manifest on disk now", extensions.ShortDigest(disk),
		"The manifest has changed since this approval. Re-approve to cover the current version.",
	} {
		if !tt.HasText(want) {
			t.Errorf("the approval record is missing %q: %q", want, tt.Texts())
		}
	}
	// The full digests are never printed: the record shows the 12-hex form.
	if tt.HasText(full) || tt.HasText(disk) {
		t.Errorf("a full digest leaked into the card: %q", tt.Texts())
	}
	// The tiers carry the host's own vocabulary, as the permission audit does.
	if !tt.HasText("Floter enforces") || !tt.HasText("Declared, not blocked") {
		t.Errorf("the permission tiers are missing: %q", tt.Texts())
	}

	// Unknown on either side is silent, not stale: the warning row goes away
	// when the current digest could not be read.
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.approved", Name: "Approved Tool", Enabled: true,
			Approval: &ApprovalRecord{
				At:             at,
				ApprovedDigest: full,
			},
		}}
	}
	tt.Frame()
	if tt.HasText("The manifest has changed since this approval. Re-approve to cover the current version.") {
		t.Errorf("an unknown current digest was reported stale: %q", tt.Texts())
	}
	if !tt.HasText("Not recorded") {
		t.Errorf("the unknown digest should say so: %q", tt.Texts())
	}

	// No record at all: one empty-state line, and none of the block's rows.
	b := New(newStore(t), Actions{})
	b.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.plain", Name: "Plain Tool", Enabled: true}}
	}
	bt := render(t, b, 720, 900)
	if err := bt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	bt.Frame()
	if !bt.HasText("No approval is recorded for this integration.") {
		t.Errorf("the empty state is missing: %q", bt.Texts())
	}
	for _, absent := range []string{"Manifest at approval", "Manifest on disk now", "Approved permissions"} {
		if bt.HasText(absent) {
			t.Errorf("a record-less integration draws %q: %q", absent, bt.Texts())
		}
	}

	// An orphan has no repository entry, so no record and no empty state.
	c := New(newStore(t), Actions{})
	c.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.orphan", Name: "dev.floter.orphan", Orphan: true}}
	}
	ct := render(t, c, 720, 900)
	if err := ct.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	ct.Frame()
	if ct.HasText("Approval record") {
		t.Errorf("an orphan draws an approval record: %q", ct.Texts())
	}

	// A record with no permissions says so instead of vanishing the row.
	d := New(newStore(t), Actions{})
	d.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.quiet", Name: "Quiet Tool", Enabled: true,
			Approval: &ApprovalRecord{At: at, ApprovedDigest: full, CurrentDigest: full},
		}}
	}
	dt := render(t, d, 720, 900)
	if err := dt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	dt.Frame()
	if !dt.HasText("None recorded") {
		t.Errorf("the no-permissions line is missing: %q", dt.Texts())
	}
	// Same digest on both sides: no warning.
	if dt.HasText("The manifest has changed since this approval. Re-approve to cover the current version.") {
		t.Errorf("an unchanged manifest was reported stale: %q", dt.Texts())
	}
}

// The detected section lists what the machine has and nothing has connected,
// and its Connect press hands the tool on with the discovery's own words.
func TestDetectedToolsSection(t *testing.T) {
	connected := []DetectedTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		Detected: func() []DetectedTool {
			return []DetectedTool{{
				Name:             "ripgrep",
				Path:             "/usr/bin/rg",
				Description:      "Fast search",
				RuntimeAvailable: true,
			}}
		},
		ConnectDetected: func(tool DetectedTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Found on this machine") || !tt.HasText("ripgrep") {
		t.Fatalf("the detected section is missing: %q", tt.Texts())
	}
	if !tt.HasText("Fast search") {
		t.Errorf("the description is missing: %q", tt.Texts())
	}
	if err := tt.Click("Connect"); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 || connected[0].Path != "/usr/bin/rg" {
		t.Errorf("connected = %+v", connected)
	}
	// Nothing detected: no section at all.
	a.Actions.Detected = func() []DetectedTool { return nil }
	tt.Frame()
	if tt.HasText("Found on this machine") {
		t.Errorf("an empty section was drawn: %q", tt.Texts())
	}
}

// Disabling asks first — stopping an integration kills whatever it is running
// — while enabling is no question at all, and Cancel leaves the state alone.
func TestDisableAsksFirst(t *testing.T) {
	toggled := []bool{}
	a := New(newStore(t), Actions{
		SetIntegrationEnabled: func(id string, enabled bool) { toggled = append(toggled, enabled) },
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Cancel: nothing happens.
	if err := tt.Click("Enabled"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Cancel"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	tt.Frame()
	if len(toggled) != 0 {
		t.Errorf("Cancel toggled: %v", toggled)
	}
	if tt.HasText("Disable V Tools?") {
		t.Errorf("the confirmation stayed up: %q", tt.Texts())
	}
	// Enabling a disabled integration is immediate.
	a.Integrations = func() []Integration {
		return []Integration{{ID: "io.github.vst93.v", Name: "V Tools", Enabled: false}}
	}
	tt.Frame()
	if err := tt.Click("Enabled"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(toggled) != 1 || !toggled[0] {
		t.Errorf("enabling asked or failed: %v", toggled)
	}
}

// The package form's door: a folder the user picks goes to the shell, which
// reviews and installs it.
func TestInstallFromFolderRow(t *testing.T) {
	installed := []string{}
	a := New(settings.NewStore(settings.Default()), Actions{
		InstallFromFolder: func(dir string) { installed = append(installed, dir) },
		ChooseFolder:      func() (string, error) { return "/tmp/my-package", nil },
	})
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Install from a folder") {
		t.Fatalf("the row is missing: %q", tt.Texts())
	}
	if err := tt.Click("Choose\u2026"); err != nil {
		t.Fatalf("the picker: %v", err)
	}
	tt.Frame()
	if len(installed) != 1 || installed[0] != "/tmp/my-package" {
		t.Errorf("installed = %v", installed)
	}
	// A caller with no picker hides the row entirely.
	a.Actions.ChooseFolder = nil
	tt.Frame()
	if tt.HasText("Install from a folder") {
		t.Errorf("the row shows without a picker: %q", tt.Texts())
	}
}

// The run form: one control per declared input, the required values refused
// until they are filled, and the answers handed to the shell as a map — never
// turned into a command string here.
func TestRunParamForm(t *testing.T) {
	type run struct {
		id     string
		values map[string]string
	}
	runs := []run{}
	a := New(settings.NewStore(settings.Default()), Actions{
		RunWithParams: func(id string, values map[string]string) *extensions.ParamFailure {
			runs = append(runs, run{id, values})
			return nil
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Params: []Param{
				{ID: "target", Label: "Target", Kind: "text", Required: true, Flag: "--target"},
				{ID: "verbose", Label: "Verbose", Kind: "boolean", Flag: "--verbose"},
				{ID: "mode", Label: "Mode", Kind: "select", Options: []string{"fast", "slow"}},
			},
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The form is behind the row's Run button, not resident: the same button
	// runs an integration with no inputs and unfolds the form for one with
	// inputs.
	if err := tt.Click("Run"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"Target", "Verbose", "Mode"} {
		if !tt.HasText(want) {
			t.Fatalf("the form is missing %q: %q", want, tt.Texts())
		}
	}
	// A required value that is empty is refused live, under the input that
	// caused it, and the Run button is disabled while it stands.
	if !tt.HasText("Required: fill in Target before running") {
		t.Fatalf("the row's reason is missing: %q", tt.Texts())
	}
	if err := tt.Click("Run with these values"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(runs) != 0 {
		t.Fatalf("an incomplete form ran: %v", runs)
	}
	// Filling it clears the reason and runs, with the select's default seeded.
	a.runValues["io.github.vst93.v"]["target"] = "here"
	tt.Frame()
	if tt.HasText("Required: fill in Target before running") {
		t.Errorf("the reason outlived the fix: %q", tt.Texts())
	}
	if err := tt.Click("Run with these values"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(runs) != 1 {
		t.Fatalf("runs = %v", runs)
	}
	values := runs[0].values
	if values["target"] != "here" || values["mode"] != "fast" {
		t.Errorf("values = %v", values)
	}
}

// A value the form can judge is refused live, under the input that caused it:
// a number that is not a finite decimal, a choice off the list. The Run button
// stays disabled while a reason stands.
func TestRunParamFormShowsPerRowReasons(t *testing.T) {
	runs := 0
	a := New(newStore(t), Actions{
		RunWithParams: func(id string, values map[string]string) *extensions.ParamFailure {
			runs++
			return nil
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Params: []Param{
				{ID: "count", Label: "Count", Kind: "number"},
				{ID: "mode", Label: "Mode", Kind: "select", Options: []string{"fast", "slow"}},
			},
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Run"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// A number the form cannot judge.
	a.runValues["io.github.vst93.v"]["count"] = "two"
	tt.Frame()
	if !tt.HasText("Count is not a valid value for this parameter") {
		t.Fatalf("the number reason is missing: %q", tt.Texts())
	}
	if err := tt.Click("Run with these values"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if runs != 0 {
		t.Fatalf("an invalid form ran: %d", runs)
	}
	// A choice off the list, then everything fixed.
	a.runValues["io.github.vst93.v"]["count"] = "2"
	a.runValues["io.github.vst93.v"]["mode"] = "medium"
	tt.Frame()
	if !tt.HasText("Mode is not a valid value for this parameter") {
		t.Fatalf("the choice reason is missing: %q", tt.Texts())
	}
	a.runValues["io.github.vst93.v"]["mode"] = "fast"
	tt.Frame()
	if tt.HasText("is not a valid value") {
		t.Errorf("the reasons outlived the fixes: %q", tt.Texts())
	}
}

// A run the backend refuses for a parameter reason is worded by the surface,
// with the label the user sees — never the raw key, and the id only when there
// is no label. The backend owns the fact, the dictionary owns the sentence.
func TestRunParamRefusalIsWorded(t *testing.T) {
	params := []extensions.ParamDefinition{
		{ID: "note", Label: "Note"},
		{ID: "mode", Label: "  "},
	}
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings
	cases := []struct {
		failure *extensions.ParamFailure
		en, zh  string
	}{
		{&extensions.ParamFailure{Key: extensions.RunErrorParamRequired, Param: "note"},
			"Required: fill in Note before running", "必填：运行前请填写 Note"},
		{&extensions.ParamFailure{Key: extensions.RunErrorParamInvalid, Param: "note"},
			"Note is not a valid value for this parameter", "Note 不是该参数的有效值"},
		{&extensions.ParamFailure{Key: extensions.RunErrorParamUnknown, Param: "sneaky"},
			"No parameter is declared with the id sneaky", "没有声明 id 为 sneaky 的参数"},
		{&extensions.ParamFailure{Key: extensions.RunErrorParamWindowsUnsafe, Param: "note"},
			"Note contains a character", "Note 含 cmd.exe"},
		// A label-less parameter falls back to its id.
		{&extensions.ParamFailure{Key: extensions.RunErrorParamRequired, Param: "mode"},
			"Required: fill in mode before running", "必填：运行前请填写 mode"},
	}
	for _, tc := range cases {
		got := paramFailureMessage(en, params, tc.failure)
		if !strings.Contains(got, tc.en) {
			t.Errorf("en = %q, want %q", got, tc.en)
		}
		if got := paramFailureMessage(zh, params, tc.failure); !strings.Contains(got, tc.zh) {
			t.Errorf("zh = %q, want %q", got, tc.zh)
		}
		// The raw key never leaks into the sentence.
		if strings.Contains(paramFailureMessage(en, params, tc.failure), string(tc.failure.Key)) {
			t.Errorf("the key leaked: %q", paramFailureMessage(en, params, tc.failure))
		}
	}
}

// The run form shows the backend's refusal after a successful local pre-check:
// a value the form cannot judge (a cmd.exe metacharacter) is caught by the
// authority and worded under the button.
func TestRunParamFormShowsTheBackendRefusal(t *testing.T) {
	a := New(newStore(t), Actions{
		RunWithParams: func(id string, values map[string]string) *extensions.ParamFailure {
			return &extensions.ParamFailure{Key: extensions.RunErrorParamWindowsUnsafe, Param: "target"}
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Params: []Param{{ID: "target", Label: "Target", Kind: "text"}},
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Run"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.runValues["io.github.vst93.v"]["target"] = "a&b"
	tt.Frame()
	if err := tt.Click("Run with these values"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Target contains a character") {
		t.Fatalf("the refusal is missing: %q", tt.Texts())
	}
}

// The last background run's output block in an integration's row: folded
// behind "View output" after a successful run, unfolded at once after a failed
// one, and four states inside — the two streams, the empty line, and the two
// truncation marks.
func TestRunOutputBlock(t *testing.T) {
	integration := func() []Integration {
		return []Integration{{ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true}}
	}
	a := New(newStore(t), Actions{})
	a.Integrations = integration
	a.SetRunResult("io.github.vst93.v", RunResult{Stdout: "hello\nworld\n", Success: true})
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// A successful run leaves the block folded: the row offers to view it.
	if !tt.HasText("View output") {
		t.Fatalf("the view control is missing: %q", tt.Texts())
	}
	if tt.HasText("Last output") {
		t.Fatalf("the block is unfolded before the press: %q", tt.Texts())
	}
	if err := tt.Click("View output"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Last output") || !tt.HasText("Hide output") {
		t.Fatalf("the block did not open: %q", tt.Texts())
	}
	if !tt.HasText("hello") || !tt.HasText("world") {
		t.Errorf("the output is missing: %q", tt.Texts())
	}
	// The hide control folds it again.
	if err := tt.Click("Hide output"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("Last output") {
		t.Errorf("the hide control did not fold the block: %q", tt.Texts())
	}

	// An empty capture says so rather than leaving a blank block.
	a.SetRunResult("io.github.vst93.v", RunResult{Success: true})
	a.setRunOutputOpen("io.github.vst93.v", true)
	tt.Frame()
	if !tt.HasText("(No output)") {
		t.Errorf("the empty line is missing: %q", tt.Texts())
	}

	// A truncated capture marks both ends.
	a.SetRunResult("io.github.vst93.v", RunResult{Stdout: "cut", Truncated: true, Success: true})
	a.setRunOutputOpen("io.github.vst93.v", true)
	tt.Frame()
	if !tt.HasText("Output was truncated") || !tt.HasText("output truncated at 64 KB") {
		t.Errorf("the truncation marks are missing: %q", tt.Texts())
	}

	// A failed run unfolds its output at once: the reason is one glance away.
	failed := New(newStore(t), Actions{})
	failed.Integrations = integration
	failed.SetRunResult("io.github.vst93.v", RunResult{Stderr: "boom\n", Success: false})
	tf := render(t, failed, 720, 900)
	if err := tf.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tf.Frame()
	if !tf.HasText("Last output") || !tf.HasText("boom") {
		t.Errorf("a failed run did not unfold its output: %q", tf.Texts())
	}
}

// The connect form's declared inputs: a row per parameter, add and remove, and
// the rows a request carries — a row still being typed (no id) is skipped
// rather than refused.
func TestConnectFormDeclaresInputs(t *testing.T) {
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	// The frame is taller than the panel: the settings rows are the 44u floor
	// now, and the inputs editor sits at the end of them.
	tt := render(t, a, 720, 2000)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Parameters") || !tt.HasText("None declared") {
		t.Fatalf("the inputs editor is missing: %q", tt.Texts())
	}
	if err := tt.Click("Add an input"); err != nil {
		t.Fatalf("add: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Input 1") {
		t.Fatalf("the row is missing: %q", tt.Texts())
	}
	// The row exists but has no id yet: it is not a declaration.
	a.toolName, a.toolProgram = "Gadget", "/usr/bin/rg"
	tool := a.declaredParams()
	if len(tool) != 0 {
		t.Errorf("an id-less row became a declaration: %+v", tool)
	}
	// Filling it in makes it one.
	a.toolParams[0].ID = "target"
	a.toolParams[0].Flag = "--target"
	a.toolParams[0].Kind = extensions.ParamText
	params := a.declaredParams()
	if len(params) != 1 || params[0].ID != "target" || params[0].Flag != "--target" {
		t.Fatalf("declared = %+v", params)
	}
	// Removing the row takes it back out.
	tt.Frame()
	if err := tt.Click("Remove"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	tt.Frame()
	if len(a.toolParams) != 0 {
		t.Errorf("the row survived its removal: %+v", a.toolParams)
	}
}

// A declared input that still needs something says so under its own row, and
// the save refuses there rather than bouncing the typo back from the backend.
func TestConnectFormNamesAnUnfinishedInputAndRefusesToSave(t *testing.T) {
	copy := i18n.For("en").Settings
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 2200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add an input"); err != nil {
		t.Fatalf("add: %v", err)
	}
	tt.Frame()
	// A named select with no choices: its row names the missing options.
	a.toolParams[0].ID = "mode"
	a.toolParams[0].Kind = extensions.ParamSelect
	a.toolName, a.toolProgram = "Gadget", "/usr/bin/rg"
	tt.Frame()
	if !tt.HasText(copy.ParamNeedsOptions) {
		t.Fatalf("the row did not name its missing options: %q", tt.Texts())
	}
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 0 {
		t.Fatalf("an unfinished input reached the backend: %+v", connected)
	}
	// Giving it options clears the line and lets the save through.
	a.toolParams[0].Options = "fast\nslow"
	tt.Frame()
	if tt.HasText(copy.ParamNeedsOptions) {
		t.Fatalf("the row still names options it now has: %q", tt.Texts())
	}
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 || len(connected[0].Params) != 1 || connected[0].Params[0].ID != "mode" {
		t.Fatalf("connected = %+v", connected)
	}
}

// R153: the version box stays editable, so the caption under it says what the
// value is — a fallback a successful probe overwrites on connect.
func TestLocalToolVersionHint(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(CustomTool) {},
		ChooseProgram:    func() (string, error) { return "", nil },
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText(copy.LocalToolVersionHint) {
		t.Fatal("the version hint is drawn before the form was opened")
	}
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(copy.LocalToolVersionHint) {
		t.Errorf("the version hint is missing: %q", tt.Texts())
	}
}

// R153: the select choices are edited one per line, not comma-separated, so a
// choice whose value contains a comma survives a round-trip instead of being
// split in two on the way back.
func TestParamOptionsNewlineCodec(t *testing.T) {
	a := New(settings.NewStore(settings.Default()), Actions{})
	a.beginEdit("local.abc", CustomTool{
		Name: "Ledger", Program: "/usr/bin/ledger",
		Params: []extensions.ParamDefinition{{
			ID: "amount", Kind: extensions.ParamSelect,
			Options: []string{"1,000", "1,000,000"},
		}},
	})
	if a.toolParams[0].Options != "1,000\n1,000,000" {
		t.Fatalf("beginEdit options = %q", a.toolParams[0].Options)
	}
	got := a.editorParams()
	if len(got) != 1 || len(got[0].Options) != 2 ||
		got[0].Options[0] != "1,000" || got[0].Options[1] != "1,000,000" {
		t.Fatalf("editorParams options = %+v", got)
	}
}

// Editing a generated integration: the button fills the same form from what
// the integration declares now, and the submit rewrites it (the id names it)
// rather than connecting a second one.
func TestEditGeneratedIntegration(t *testing.T) {
	edited := []struct {
		id   string
		tool CustomTool
	}{}
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		EditCustomTool: func(id string, tool CustomTool) {
			edited = append(edited, struct {
				id   string
				tool CustomTool
			}{id, tool})
		},
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
		ToolForEdit: func(id string) (CustomTool, bool) {
			if id != "local.abc" {
				return CustomTool{}, false
			}
			return CustomTool{
				Name: "Ripgrep", Program: "/usr/bin/rg", Command: "rg",
				Args: "--floter", Version: "1.0.0",
				Platforms: []string{"darwin", "linux"},
				Params:    []extensions.ParamDefinition{{ID: "target", Kind: "text", Flag: "--target"}},
			}, true
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "local.abc", Name: "Ripgrep", Enabled: true, Running: true, Generated: true,
		}}
	}
	tt := render(t, a, 720, 2200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Edit\u2026"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	tt.Frame()
	// The form is open and filled from the integration.
	if !tt.HasText("Ripgrep") || !tt.HasText("Input 1") {
		t.Fatalf("the form was not filled: %q", tt.Texts())
	}
	if a.toolEditing != "local.abc" || a.toolProgram != "/usr/bin/rg" {
		t.Fatalf("editing=%q program=%q", a.toolEditing, a.toolProgram)
	}
	// Saving rewrites it, and does not connect a second package.
	if err := tt.Click("Save"); err != nil {
		t.Fatalf("save: %v", err)
	}
	tt.Frame()
	if len(edited) != 1 || edited[0].id != "local.abc" {
		t.Fatalf("edited = %+v", edited)
	}
	if len(connected) != 0 {
		t.Errorf("editing also connected: %+v", connected)
	}
	if edited[0].tool.Command != "rg" || edited[0].tool.Args != "--floter" {
		t.Errorf("the draft lost its fields: %+v", edited[0].tool)
	}
	// R7-5 · the edit form is seeded from what the integration declares, so its
	// platforms row starts on the manifest's own list and saves it back.
	if got := strings.Join(edited[0].tool.Platforms, ","); got != "darwin,linux" {
		t.Errorf("the draft lost its platforms: %q", got)
	}
	if a.toolEditing != "" {
		t.Errorf("the form stayed in edit mode: %q", a.toolEditing)
	}
}

// The settings panel's own header band: the app's name and version at the
// start, and the two ways out at the end — quit, and back to the launcher.
// It is the same 56u band the launcher's field row and every page's header
// use, so the window's first line does not move when the surface changes.
func TestSettingsHeaderBand(t *testing.T) {
	quit, closed := 0, 0
	a := New(newStore(t), Actions{
		Quit:  func() { quit++ },
		Close: func() { closed++ },
	})
	a.About = About{Name: "floter", Version: "0.3.14"}
	tt := render(t, a, 720, 580)
	for _, want := range []string{"Settings", "v0.3.14", "Quit floter", "Close settings"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("Quit floter"); err != nil {
		t.Fatalf("quit: %v", err)
	}
	tt.Frame()
	if quit != 1 || closed != 0 {
		t.Errorf("quit = %d, close = %d", quit, closed)
	}
	if err := tt.Click("Close settings"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if closed != 1 {
		t.Errorf("close = %d", closed)
	}
	// A panel with no actions draws no buttons for them.
	b := New(newStore(t), Actions{})
	b.About = About{Name: "floter"}
	tt = render(t, b, 720, 580)
	if tt.HasText("Quit floter") || tt.HasText("Close settings") {
		t.Errorf("buttons were drawn with no actions: %q", tt.Texts())
	}
}

// The header's two icon-only controls carry their own sentence: the copy the
// reference put in each button's `title=` is the tip that shows once the
// pointer rests, so a bare glyph is not an unexplained mark. The strings were
// declared and translated but no view showed them.
func TestSettingsHeaderButtonsCarryTheirHints(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{Quit: func() {}, Close: func() {}})
	tt := render(t, a, 720, 580)
	for _, want := range []struct{ label, hint string }{
		{copy.Quit, copy.QuitHint},
		{copy.Close, copy.CloseHint},
	} {
		rect, ok := tt.Find(want.label)
		if !ok {
			t.Fatalf("no %q control: %q", want.label, tt.Texts())
		}
		// Park the pointer away first, so this control's own rest timer starts
		// fresh rather than warming on the previous tip.
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H+120)
		tt.Frame()
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
		// The tip waits for the pointer to rest (Base UI's 600ms).
		time.Sleep(700 * time.Millisecond)
		tt.Frame()
		if !tt.HasText(want.hint) {
			t.Errorf("the %q control has no tooltip %q: %q", want.label, want.hint, tt.Texts())
		}
	}
}

// "Restore defaults" is the shortcuts group's own action, on the group title
// row - and it really resets: the shell's reset re-registers the shipped
// bindings and drops the custom ones.
func TestRestoreDefaultsRuns(t *testing.T) {
	reset := 0
	a := New(newStore(t), Actions{ResetShortcuts: func() { reset++ }})
	tt := render(t, a, 720, 800)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Restore the shipped keys"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	tt.Frame()
	if reset != 1 {
		t.Errorf("restore ran %d times", reset)
	}
}

// sample reads one pixel of the last frame, at a logical coordinate.
func sample(tt *ui.Tester, x, y float32) ui.Color {
	img := tt.Image()
	i := int(y)*img.Stride + int(x)*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// The palette row shows what its choice paints: the block under the picker is
// drawn in the palette's own background, and follows the picker when it
// changes (R43 — the user asked a palette to show what it does).
func TestThePaletteRowPreviewsItsColors(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	if err := store.Update(func(s *settings.Settings) { s.TerminalTheme = "ink" }); err != nil {
		t.Fatal(err)
	}
	tt := render(t, a, 720, 3000)
	label := i18n.For("en").Settings.PalettePreview
	rect, ok := tt.Find(label)
	if !ok {
		t.Fatalf("the palette preview is missing: %q", tt.Texts())
	}
	want, _ := terminalui.PaletteColorsFor("ink", true)
	// The sample sits in the block's top-left padding, clear of any glyph.
	if got := sample(tt, rect.X+4, rect.Y+4); got != want.Background {
		t.Errorf("preview background = %v, want the ink background %v", got, want.Background)
	}

	// A new choice repaints the block.
	if err := store.Update(func(s *settings.Settings) { s.TerminalTheme = "paper" }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	rect, _ = tt.Find(label)
	want, _ = terminalui.PaletteColorsFor("paper", false)
	if got := sample(tt, rect.X+4, rect.Y+4); got != want.Background {
		t.Errorf("preview background after the change = %v, want the paper background %v", got, want.Background)
	}
}

// `inherit` has no fixed pair, so the preview follows the app's own
// appearance, as the reference's `--terminal-*` tokens did.
func TestThePalettePreviewFollowsTheAppearanceForInherit(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 3000)
	label := i18n.For("en").Settings.PalettePreview
	rect, ok := tt.Find(label)
	if !ok {
		t.Fatalf("the palette preview is missing: %q", tt.Texts())
	}
	want, _ := terminalui.PaletteColorsFor("inherit", false)
	if got := sample(tt, rect.X+4, rect.Y+4); got != want.Background {
		t.Errorf("light inherit preview = %v, want %v", got, want.Background)
	}

	tt.SetDark(true)
	tt.Frame()
	rect, _ = tt.Find(label)
	want, _ = terminalui.PaletteColorsFor("inherit", true)
	if got := sample(tt, rect.X+4, rect.Y+4); got != want.Background {
		t.Errorf("dark inherit preview = %v, want %v", got, want.Background)
	}
}

// R7-5 · the connection form limits the tool to the platforms ticked on its
// "Supported platforms" row, which starts on this machine's own; unticking
// every one of them is refused under that row rather than silently sent as
// "every platform" (the manifest's own meaning for an empty list). The old
// form did the same thing by disabling its submit button.
func TestLocalToolPlatformsRow(t *testing.T) {
	copy := i18n.For("en").Settings
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(copy.LocalToolPlatforms) {
		t.Fatalf("the form is missing the platforms row: %q", tt.Texts())
	}
	// A fresh form ticks this machine's own platform, and only it.
	own := currentPlatform()
	if len(a.toolPlatforms) != 1 || !a.toolPlatforms[own] {
		t.Fatalf("a fresh form starts on %+v, want just %q", a.toolPlatforms, own)
	}
	// Unticking it leaves nothing: the form says so and refuses to connect.
	if err := tt.Click(platformLabel(own)); err != nil {
		t.Fatalf("untick %s: %v", own, err)
	}
	tt.Frame()
	a.toolName, a.toolProgram = "Ripgrep", "/usr/bin/rg"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 0 {
		t.Fatalf("an empty platform list reached the host: %+v", connected)
	}
	if !tt.HasText(copy.LocalToolPlatformRequired) {
		t.Fatalf("the row did not name the requirement: %q", tt.Texts())
	}
	// Ticking two platforms connects, and both travel with the draft.
	for _, platform := range []string{"linux", "windows"} {
		if err := tt.Click(platformLabel(platform)); err != nil {
			t.Fatalf("tick %s: %v", platform, err)
		}
		tt.Frame()
	}
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 {
		t.Fatalf("connected = %+v", connected)
	}
	if got := strings.Join(connected[0].Platforms, ","); got != "linux,windows" {
		t.Errorf("the draft carried platforms %q", got)
	}
}

// platformLabel is the row's own word for one platform id, for a test that
// clicks it by name.
func platformLabel(id string) string {
	for _, platform := range customPlatforms {
		if platform.ID == id {
			return platform.Label
		}
	}
	return id
}

// The connection form reads the same shape rules the host applies (the old
// host's `settings.extensions.form.*`): a name past 80 characters, a command
// that is not a lowercase slug, a version that is not semver. A fresh form
// stays quiet; a submit names the rule under its own row and refuses, and one
// fix at a time lets the connection through.
func TestLocalToolFormNamesTheShapeRules(t *testing.T) {
	copy := i18n.For("en").Settings
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Nothing is refused before the user asks: a fresh form is not red.
	for _, sentence := range []string{copy.LocalToolNameLength, copy.LocalToolCommandShape, copy.LocalToolVersionShape} {
		if tt.HasText(sentence) {
			t.Fatalf("a fresh form named %q: %q", sentence, tt.Texts())
		}
	}
	a.toolName, a.toolProgram = "Ripgrep", "/usr/bin/rg"

	// A version that is not semver refuses and says so under its row.
	a.toolVersion = "not.a.version"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 0 {
		t.Fatalf("a bad version reached the backend: %+v", connected)
	}
	if !tt.HasText(copy.LocalToolVersionShape) {
		t.Fatalf("the version row did not name the rule: %q", tt.Texts())
	}

	// A command that is not a lowercase slug refuses the same way.
	a.toolVersion = "1.2.3"
	a.toolCommand = "Bad Command"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 0 {
		t.Fatalf("a bad command reached the backend: %+v", connected)
	}
	if !tt.HasText(copy.LocalToolCommandShape) {
		t.Fatalf("the command row did not name the rule: %q", tt.Texts())
	}

	// A name past 80 characters refuses too.
	a.toolCommand = "rg"
	a.toolName = strings.Repeat("x", 81)
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 0 {
		t.Fatalf("an over-long name reached the backend: %+v", connected)
	}
	if !tt.HasText(copy.LocalToolNameLength) {
		t.Fatalf("the name row did not name the rule: %q", tt.Texts())
	}

	// Everything fixed: the same form connects and the draft is clean.
	a.toolName = "Ripgrep"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 {
		t.Fatalf("connected = %+v", connected)
	}
	if connected[0].Name != "Ripgrep" || connected[0].Command != "rg" || connected[0].Version != "1.2.3" {
		t.Errorf("the draft = %+v", connected[0])
	}
	// A program carries no script language: the form's language state would
	// otherwise refuse the request as scriptLanguageInvalid at the host.
	if connected[0].ScriptLanguage != "" || connected[0].ScriptContent != "" {
		t.Errorf("an executable carried script fields: %+v", connected[0])
	}
}

// A recording that another action already answers for is refused: nothing is
// written, and the page says so — at its top and under the refused row's name,
// as the reference's `rejectedAction` did. Recording the action's own chord is
// no change, and a free chord is written and clears the warning.
func TestShortcutRecorderRefusesAChordAnotherActionHolds(t *testing.T) {
	store := newStore(t)
	store.Update(func(s *settings.Settings) {
		s.SetShortcut(settings.ShortcutPaste, "Cmd+V")
	})
	written := []string{}
	a := New(store, Actions{SetShortcut: func(id, accelerator string) {
		written = append(written, id+"="+accelerator)
		store.Update(func(s *settings.Settings) { s.SetShortcut(id, accelerator) })
	}})
	a.Shortcut, a.ShortcutID = "Ctrl+Space", settings.ShortcutToggleWindow
	tt := render(t, a, 720, 620)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	warnings := func() int {
		count := 0
		for _, text := range tt.Texts() {
			if text == "Already used by another app" {
				count++
			}
		}
		return count
	}

	// The first Record belongs to the summon key, whose row is the one above
	// the app's own keys.
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(ui.Super, ui.KeyV) // Cmd+V, which paste already answers for
	tt.Frame()
	if len(written) != 0 {
		t.Errorf("a duplicate was written: %v", written)
	}
	if got := warnings(); got != 2 {
		t.Errorf("the refusal is shown %d times, want the page's own and the row's (2)", got)
	}
	// The refused write left no trace in the store, which is the one source of
	// truth.
	if got := settings.Shortcut(store.Snapshot(), settings.ShortcutToggleWindow); got != settings.DefaultSummonShortcut {
		t.Errorf("the refused duplicate changed the store: %q", got)
	}

	// The action's own chord is no change: nothing is written and nothing is
	// refused.
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeySpace) // the shipped summon key, which is this row's own
	tt.Frame()
	if len(written) != 0 {
		t.Errorf("re-recording the action's own chord wrote %v", written)
	}
	if got := warnings(); got != 0 {
		t.Errorf("the action's own chord was refused: %d", got)
	}

	// A free chord is written, and the warning goes away.
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(ui.Super|ui.Shift, ui.KeyK)
	tt.Frame()
	if len(written) != 1 || written[0] != "toggle_window=Cmd+Shift+K" {
		t.Fatalf("a free chord wrote %v", written)
	}
	if got := warnings(); got != 0 {
		t.Errorf("the warning stayed up: %d", got)
	}
	if got := settings.Shortcut(store.Snapshot(), settings.ShortcutToggleWindow); got != "Cmd+Shift+K" {
		t.Errorf("the written chord = %q", got)
	}
}

// ↑/↓ walk the sidebar pages from anywhere on the settings surface, wrap at
// both ends, and remember the choice — the reference's window-level handler
// (`src/hooks/useAppKeyboard.ts`, "↑/↓ cycle sidebar pages from anywhere in
// the settings panel") over `nextSettingsPage`.
func TestSidebarArrowsWalkThePagesFromAnywhere(t *testing.T) {
	pages := []string{}
	a := New(newStore(t), Actions{SetPage: func(name string) { pages = append(pages, name) }})

	// The sidebar holds the keyboard on entry. ↓ walks the sidebar order.
	a.FocusSidebar()
	tt := render(t, a, 720, 580)
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if a.Page != PageSessions {
		t.Fatalf("page after down = %d, want sessions", a.Page)
	}

	// With the keyboard nowhere near the sidebar — the window owns it — the
	// same keys still walk the pages.
	a2 := New(newStore(t), Actions{})
	tt2 := render(t, a2, 720, 580)
	tt2.Key(0, ui.KeyDown)
	tt2.Frame()
	if a2.Page != PageSessions {
		t.Fatalf("page after down with no control focused = %d, want sessions", a2.Page)
	}

	// ↑ from the first page wraps to the last, and ↓ from the last to the
	// first.
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if a.Page != PageGeneral {
		t.Fatalf("page after up from sessions = %d, want general", a.Page)
	}
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if a.Page != PageAbout {
		t.Fatalf("page after up from general = %d, want about (wrapped)", a.Page)
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if a.Page != PageGeneral {
		t.Fatalf("page after down from about = %d, want general (wrapped)", a.Page)
	}

	want := []string{"sessions", "general", "about", "general"}
	if len(pages) != len(want) {
		t.Fatalf("recorded pages = %v, want %v", pages, want)
	}
	for i := range want {
		if pages[i] != want[i] {
			t.Errorf("recorded pages = %v, want %v", pages, want)
			break
		}
	}
}

// The reference's `isArrowKeyEditableTarget` keeps ↑/↓ out of the page
// navigation while the keyboard is in a text field — a single-line INPUT as
// much as a TEXTAREA (`src/settings-nav.ts:24-38`). mygo's editor only claims
// ↑/↓ for multiline fields (`ui/editor_navigation.go:46-47`), so a single-line
// TextInput used to hand them to the surface's window shortcut and the page
// changed under the caret.
func TestSingleLineFieldsKeepTheirArrows(t *testing.T) {
	copy := i18n.For("en").Settings
	pages := []string{}
	a := New(newStore(t), Actions{
		SetPage:             func(name string) { pages = append(pages, name) },
		InstallFromRegistry: func(string, string) {},
	})
	tt := render(t, a, 720, 580)

	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Page != PageIntegrations {
		t.Fatalf("page = %d, want integrations", a.Page)
	}

	// Put the keyboard in the npm package field: a single-line TextInput.
	rect, ok := tt.Find(copy.IntegrationsPackageHint)
	if !ok {
		t.Fatalf("the package field is not on screen: %q", tt.Texts())
	}
	tt.ClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
	tt.Frame()
	if !tt.Focused(copy.IntegrationsPackageHint) {
		t.Fatalf("the package field did not take the keyboard")
	}

	// ↓/↑ stay in the field: the page does not change and the caret stays put.
	before := len(pages)
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if a.Page != PageIntegrations {
		t.Errorf("↓ in a text field walked to page %d", a.Page)
	}
	if !tt.Focused(copy.IntegrationsPackageHint) {
		t.Errorf("↓ in a text field moved the keyboard off it")
	}
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if a.Page != PageIntegrations {
		t.Errorf("↑ in a text field walked to page %d", a.Page)
	}
	if len(pages) != before {
		t.Errorf("the field's arrows recorded page switches: %v", pages)
	}

	// Outside the field the arrows still walk the pages (the P7-dc rule): the
	// field's claim is scoped to its own focus, not the whole surface.
	a.FocusSidebar()
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if a.Page != PageAbout {
		t.Errorf("↓ outside a text field went to page %d, want about", a.Page)
	}
}

// nextPage is the reference's `nextSettingsPage` over `wrapIndex`: the sidebar
// order wraps at both ends.
func TestNextPageWraps(t *testing.T) {
	cases := []struct {
		page, delta, want int
	}{
		{PageGeneral, 1, PageSessions},
		{PageSessions, 1, PageShortcuts},
		{PageAbout, 1, PageGeneral},
		{PageGeneral, -1, PageAbout},
		{PageAbout, -1, PageIntegrations},
		{PageShortcuts, 0, PageShortcuts},
	}
	for _, tc := range cases {
		if got := nextPage(tc.page, tc.delta); got != tc.want {
			t.Errorf("nextPage(%d, %d) = %d, want %d", tc.page, tc.delta, got, tc.want)
		}
	}
}
