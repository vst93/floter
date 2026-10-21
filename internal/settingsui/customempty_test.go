package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// The custom-shortcuts empty state is the reference's SettingsEmpty
// (`ShortcutsPage.tsx:240-252`): the 22px Keyboard mark, the state in
// `--text-secondary`, the grey hint underneath, and the "Add the first one"
// text action — a `.settings-reset` with a leading ＋ mark, never a filled
// button. The old port drew a muted line, a `text` glyph and a `ui.Button`, so
// the mark, the ink and the way out had all drifted.
func TestTheCustomShortcutsEmptyStateIsTheReferenceEmptyRegion(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})

	// The region resolves its own colors from the theme the app draws in;
	// capture the same tokens from that context rather than a second one.
	var secondary, muted ui.Color
	var icon float32
	tt := ui.NewTester(func(c *ui.Context) {
		tokens := a.tokens(c)
		secondary, muted = tokens.TextSecondary, tokens.Theme.TextMuted
		icon = c.Theme().Space(5.5) // the reference's 22px Keyboard mark
		a.View(c)
	}, 720, 1000)
	tt.Frame()
	if err := tt.Click(en.PageShortcuts); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()

	title, ok := tt.Find(en.ShortcutsNone)
	if !ok {
		t.Fatalf("the custom-shortcuts empty title is missing: %q", tt.Texts())
	}
	if !tt.HasText(en.ShortcutsEmptyHint) {
		t.Errorf("the empty hint is missing: %q", tt.Texts())
	}
	if tt.HasText("No custom shortcuts yet.") {
		t.Errorf("the old self-made empty sentence is still drawn: %q", tt.Texts())
	}

	// The title is `--text-secondary`, the reference primitive's own step —
	// measurably apart from the `--text-muted` the old line wore.
	bg, _ := modalColor(tt, title)
	ink := inkColor(tt, title)
	if want := overSurface(secondary, bg); !colorClose(ink, want, 14) {
		t.Errorf("title ink = %v, want the reference's --text-secondary over the card (%v)", ink, want)
	}
	if old := overSurface(muted, bg); colorClose(ink, old, 14) {
		t.Errorf("title ink = %v wears the old muted step, not --text-secondary", ink)
	}

	// The mark is the 22px Keyboard glyph above the title, drawn from the
	// shared vocabulary (`ShortcutsPage.tsx:241`). The region's own 24u top
	// padding keeps that band clear of the group above it, so the only ink in
	// the 34u above the title is the icon.
	midX := title.X + title.W/2
	top, bottom := inkBand(tt, midX, title.Y-34, title.Y-1, bg, 24)
	if top < 0 {
		t.Fatalf("no glyph ink above the empty title: the 22u Keyboard mark is missing")
	}
	if h := bottom - top + 1; h < icon*0.5 || h > icon+2 {
		t.Errorf("glyph ink height = %v, want the reference's 22u mark (%v)", h, icon)
	}

	// The way out is the reference's text action with its ＋ mark: accent ink
	// in the slot just left of the label, on a transparent plane. The old
	// filled button painted its control fill across the whole box, so its
	// leading inset would differ from the card.
	action, ok := tt.Find(en.ShortcutsAddFirst)
	if !ok {
		t.Fatalf("the empty region's way out is missing: %q", tt.Texts())
	}
	midY := action.Y + action.H/2
	mark := 256
	for x := int(action.X) - 20; x < int(action.X)-1; x++ {
		p := sample(tt, float32(x), midY)
		if m := int(p.R); m < mark {
			mark = m
		}
		if m := int(p.G); m < mark {
			mark = m
		}
		if m := int(p.B); m < mark {
			mark = m
		}
	}
	if mark >= 200 {
		t.Errorf("no accent ＋ mark left of the empty action (min channel %d)", mark)
	}
	if plane := sample(tt, action.X-1, midY); channelDelta(plane, bg) > 2 {
		t.Errorf("the empty action is a filled pill: its plane %v differs from the card %v", plane, bg)
	}

	// The action still opens the recorder, as the reference's own empty-state
	// action did.
	if err := tt.Click(en.ShortcutsAddFirst); err != nil {
		t.Fatalf("click add-first: %v", err)
	}
	tt.Frame()
	if !tt.HasText(en.ShortcutRecording) {
		t.Errorf("the recorder did not open: %q", tt.Texts())
	}
}

// The shared empty region's list floor is 92u (`settings.css:641`), and the
// custom-shortcuts group is a list placeholder — it does not override it the
// way the Sessions page body does. The glyph is the reference's 22px Keyboard
// mark (`ShortcutsPage.tsx:241`).
func TestTheCustomShortcutsEmptyRegionHasTheListFloor(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})

	var ref ui.Handle
	var box ui.Rect
	var floor, icon float32
	tt := ui.NewTester(func(c *ui.Context) {
		floor, icon = c.Theme().Space(23), c.Theme().Space(5.5)
		a.emptyState(c, "keyboard", en.ShortcutsNone, en.ShortcutsEmptyHint, 0, 0, nil).Bind(&ref)
		box = c.Resolve(ref).Bounds()
	}, 720, 300)
	tt.Frame()

	if box.H < floor {
		t.Errorf("empty region height = %v, want the list floor 92u (%v)", box.H, floor)
	}

	title, ok := tt.Find(en.ShortcutsNone)
	if !ok {
		t.Fatalf("the empty title is missing: %q", tt.Texts())
	}
	midX := box.X + box.W/2
	bg, _ := modalColor(tt, title)
	top, bottom := inkBand(tt, midX, title.Y-34, title.Y-1, bg, 24)
	if top < 0 {
		t.Fatalf("no glyph ink above the empty title: the 22u Keyboard mark is missing")
	}
	if h := bottom - top + 1; h < icon*0.5 || h > icon+2 {
		t.Errorf("glyph ink height = %v, want the reference's 22u mark (%v)", h, icon)
	}
}

// Recording on a saved row edits that row: the reference passed the row's own
// id into `capture(row.id, next)` (`ShortcutsPage.tsx:361-371`), so a re-record
// replaced the row's key instead of filling the row being made.
func TestACustomRowsRecorderEditsItsOwnRow(t *testing.T) {
	entries := []settings.CustomShortcut{{Key: "Cmd+Shift+P", Action: "plugin:clipboard"}}
	var committed [][]settings.CustomShortcut
	a := New(newStore(t), Actions{
		CustomShortcuts: func() []settings.CustomShortcut { return entries },
		SetCustomShortcuts: func(list []settings.CustomShortcut) []CustomShortcutRejection {
			committed = append(committed, list)
			entries = list
			return nil
		},
	})
	a.customRecording, a.customRecordRow = true, 0
	a.captureCustomKey(i18n.For("en").Settings, "Ctrl+Shift+Q")

	if a.customKey != "" {
		t.Errorf("the saved row's key landed in the draft row: %q", a.customKey)
	}
	if len(committed) != 1 || committed[0][0].Key != "Ctrl+Shift+Q" {
		t.Fatalf("committed = %+v", committed)
	}
	// The action travelled with the key rather than being reset by the edit.
	if committed[0][0].Action != "plugin:clipboard" {
		t.Errorf("the row's action = %q", committed[0][0].Action)
	}
}

// A key another custom row already holds is refused where it was recorded and
// the sentence hangs under that row, as the reference's `localRejection` did
// (`ShortcutsPage.tsx:361-371,267-274`).
func TestADuplicateCustomKeyIsRefusedOnItsRow(t *testing.T) {
	en := i18n.For("en").Settings
	entries := []settings.CustomShortcut{
		{Key: "Cmd+Shift+P", Action: "plugin:clipboard"},
		{Key: "Cmd+Shift+O", Action: "plugin:browser"},
	}
	var committed int
	a := New(newStore(t), Actions{
		CustomShortcuts: func() []settings.CustomShortcut { return entries },
		SetCustomShortcuts: func(list []settings.CustomShortcut) []CustomShortcutRejection {
			committed++
			entries = list
			return nil
		},
	})
	a.customRecording, a.customRecordRow = true, 1
	a.captureCustomKey(en, "Cmd+Shift+P")

	if committed != 0 {
		t.Errorf("a duplicate key reached the shell (%d commits)", committed)
	}
	if a.customWarning(en, "Cmd+Shift+O", 1) != en.ShortcutsConflict {
		t.Errorf("the row kept no warning: %q", a.customWarning(en, "Cmd+Shift+O", 1))
	}
	if got := a.customWarning(en, "Cmd+Shift+P", 0); got != "" {
		t.Errorf("the warning landed on the wrong row: %q", got)
	}
}
