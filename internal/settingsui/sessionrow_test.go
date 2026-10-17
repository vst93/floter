package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/theme"
)

// The sessions page's row family was the one settings surface never compared
// against the reference field by field. `main:src/settings/SessionsPage.tsx`
// draws its list as the shared `SettingsRow` with four pieces of its own on
// top, and `main:src/styles/settings.css` names each of them:
//
//	.session-manager__row     three columns (glyph, text, actions), 56px floor
//	.settings-row__icon       26px chip, `--icon-surface`, `--radius-xs`
//	.session-manager__cwd     monospace, `--text-caption`
//	.session-manager__meta    caption step, tabular figures
//	.session-manager__icon-button   28px square, `--text-muted`, hover ladder
//	.session-manager__resume-dot     5u accent badge, top-right 3u/3u
//	.session-manager__kill-confirm   28px warm pill, warning edge+ink, Trash2 13
//
// The port had drifted on six of those: the trailing controls were *labelled*
// buttons ("Show", "Close session") with a second pair drawn for the question
// ("Close it?" / "Keep") where the reference draws two icon buttons and one
// expanding pill, the row led with a bare cell, and the meta line wore the
// body step. These pin the reference's shape.

// pixelAt is one pixel of the last frame as a theme colour, for comparing two
// boxes' planes.
func pixelAt(tt *ui.Tester, x, y int) ui.Color {
	img := tt.Image()
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// The trailing controls are the reference's two icon buttons, carrying the
// reference's own accessible words — `terminal.sessionResume` and
// `terminal.sessionKill` (`i18n.ts:180-181`), never the port's "Show" and
// "Close session" — and the question is the same control grown into the
// reference's `terminal.sessionKillArm` pill (`:182`), with no third button
// anywhere on the row.
func TestTheSessionTrailingControlsAreTheReferenceIconButtons(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{
		CloseSessionAt: func(int) {},
		SelectSession:  func(int) {},
	})
	a.Sessions = func() []Session {
		return []Session{{ID: 1, Title: "zsh", Running: true, Active: true, Index: 0}}
	}
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	for _, gone := range []string{"Show", "Close session", "Close it?", "Keep"} {
		if tt.HasText(gone) {
			t.Errorf("the port's own word %q is still drawn: %q", gone, tt.Texts())
		}
	}
	resume, ok := tt.Find(en.SessionsResume)
	if !ok {
		t.Fatalf("the resume control is missing: %q", tt.Texts())
	}
	bin, ok := tt.Find(en.SessionsKill)
	if !ok {
		t.Fatalf("the bin control is missing: %q", tt.Texts())
	}
	// Both are the reference's 28u squares (`settings.css:1299-1310`).
	if resume.W != 28 || resume.H != 28 || bin.W != 28 || bin.H != 28 {
		t.Errorf("the icon buttons are not 28u squares: resume %v, bin %v", resume, bin)
	}

	// The question replaces the bin in place, and is wider than a square.
	if err := tt.Click(en.SessionsKill); err != nil {
		t.Fatalf("arm: %v", err)
	}
	tt.Frame()
	ask, ok := tt.Find(en.SessionsKillArm)
	if !ok {
		t.Fatalf("the armed question is missing: %q", tt.Texts())
	}
	if ask.W <= 28 {
		t.Errorf("the question did not expand in place: %v", ask)
	}
	if tt.HasText("Keep") || tt.HasText("Close it?") {
		t.Errorf("a second pair of buttons came back: %q", tt.Texts())
	}
}

// The question pill wears the reference's warm plane: `.session-manager__kill-confirm`
// is `--glass-raised-warm` (an alias of `--system-icon-surface`) under a
// `--text-warning` edge (`settings.css:1330-1347`), so the pixels *inside* the
// pill differ from the card's own plane beside it.
func TestTheSessionKillQuestionWearsTheWarmPill(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{CloseSessionAt: func(int) {}})
	a.Sessions = func() []Session {
		return []Session{{ID: 1, Title: "zsh", Running: true, Active: true, Index: 0}}
	}
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(en.SessionsKill); err != nil {
		t.Fatalf("arm: %v", err)
	}
	tt.Frame()
	ask, ok := tt.Find(en.SessionsKillArm)
	if !ok {
		t.Fatalf("the armed question is missing: %q", tt.Texts())
	}
	y := int(ask.Y + ask.H/2)
	inside := pixelAt(tt, int(ask.X)+2, y)
	card := pixelAt(tt, int(ask.X)-6, y)
	if colorClose(inside, card, 3) {
		t.Errorf("the question wears the card's plane: %v vs the card %v", inside, card)
	}
}

// The row's leading cell is the shared settings-row chip carrying lucide's
// SquareTerminal, not the extension panel's package chip and not the bare
// prompt: the reference names both
// (`SessionsPage.tsx:186-187`, `.settings-row__icon`, `settings.css:511-518`).
func TestTheSessionRowLeadsWithTheSessionsMark(t *testing.T) {
	if _, ok := launcher.Glyph("square-terminal"); !ok {
		t.Fatalf("the shared vocabulary has no square-terminal mark")
	}
	if _, ok := launcher.Glyph("play"); !ok {
		t.Fatalf("the shared vocabulary has no play mark (the reference's Play button, `SessionsPage.tsx:212`)")
	}
	if _, ok := launcher.Glyph("trash"); !ok {
		t.Fatalf("the shared vocabulary has no trash mark")
	}

	a := New(newStore(t), Actions{})
	a.Sessions = func() []Session {
		return []Session{{ID: 1, Title: "zsh", Running: true, Active: true, Index: 0}}
	}
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	title, ok := tt.Find("zsh")
	if !ok {
		t.Fatalf("the row's title is missing")
	}
	// The chip's own mark paints ink to the left of the label, in a cell whose
	// plane is the chip's.
	y := int(title.Y + title.H/2)
	plane := pixelAt(tt, int(title.X)-36, y)
	ink := false
	for x := int(title.X) - 36; x < int(title.X); x++ {
		if colorDistance(pixelAt(tt, x, y), plane) > 40 {
			ink = true
			break
		}
	}
	if !ink {
		t.Errorf("the row leads with a bare cell: no mark left of the label")
	}
}

// The cwd line is a *value* and the meta line is a footnote: the reference sets
// the directory in the terminal's monospace stack at the caption step
// (`.session-manager__cwd { font-family: "SF Mono", …; font-size:
// var(--text-caption) }`, `settings.css:1272-1276`) while the meta line under it
// is the caption step alone (`.session-manager__meta`, `:1280-1286`). The row
// builder's own `Mono` flag is what selects that face, so it is what this pins:
// the directory is the one line in the block marked mono, and the meta line is
// never.
func TestTheSessionCwdLineIsTheBlocksOnlyMonoLine(t *testing.T) {
	session := Session{
		Title: "zsh", Running: true, Note: "Running · attached",
		Dir: "/tmp/some/where", Size: "80×24",
	}
	lines := sessionLines(session)
	if len(lines) != 2 {
		t.Fatalf("sessionLines = %+v, want the cwd and the meta line", lines)
	}
	if !lines[0].Mono {
		t.Errorf("the cwd line is not marked mono (the reference draws it in the terminal's own stack)")
	}
	if lines[0].Text != session.Dir {
		t.Errorf("the first line is not the directory: %+v", lines[0])
	}
	if lines[1].Mono {
		t.Errorf("the meta line was marked mono: %+v", lines[1])
	}
	if len(lines[1].Segments) != 2 {
		t.Errorf("the meta line is not the state+size line: %+v", lines[1])
	}

	// A session with no reported directory draws no cwd line at all — a bare
	// `~` is the reference's `session.cwd || "~"` fallback, not an empty string
	// this build has to invent.
	bare := sessionLines(Session{Title: "sh", Running: true, Note: "Running · attached"})
	if len(bare) != 1 || bare[0].Segments == nil {
		t.Errorf("a session with no directory did not drop the cwd line: %+v", bare)
	}
}

// The About page's update action is the reference's own accent pill:
// `.update-banner__button` (`settings.css:1542-1568`) — the `--accent` fill
// under the `--glass-raised-rim` top edge, a `999px` stadium, `6u/14u` of
// padding, the emphasis step at 600, `--accent-contrast` ink and *no* keyline
// at all. The port drew the framework's grey `ui.Button` there, so the one
// thing on the page the user presses wore the control ladder's resting plane.
func TestTheUpdateActionWearsTheReferenceAccentPill(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{InstallUpdate: func() {}})
	a.About.Version = "0.3.14"
	a.Update = UpdateState{Checked: true, Available: true, Version: "9.9.9"}
	a.selectPage(PageAbout)
	tt := render(t, a, 900, 700)

	button, ok := tt.Find(en.UpdateDownload)
	if !ok {
		t.Fatalf("the update control is missing: %q", tt.Texts())
	}
	// The pill's fill is the theme's accent, not the control ladder's grey:
	// read the pixel just inside its edge, clear of the word.
	tokens := theme.ForPreferences(a.Store.Snapshot(), false, ui.Preferences{})
	fill := pixelAt(tt, int(button.X)+2, int(button.Y+button.H/2))
	if colorDistance(fill, tokens.Theme.Accent) > colorDistance(fill, tokens.Theme.Surface) {
		t.Errorf("the update control's fill is not the accent pill: %v (accent %v, control %v)",
			fill, tokens.Theme.Accent, tokens.Theme.Surface)
	}
	// And it is a pill, not the framework control's rounded box: its height is
	// the reference's own 6u/14u box (28u at the default step), where the
	// framework's `ui.Button` drew its own `1.5u/3.5u` padding around a text
	// line.
	if button.H < 27 || button.H > 30 {
		t.Errorf("the pill's height = %v, want the reference's 28u box", button.H)
	}
}
