package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The Integrations panel's group header is the reference's own
// `.extensions-section-title` (`extensions.css:419-446`), not the settings
// cards' `.settings-section__label` the shared `section` draws: a body-step,
// muted, 620 label with the section's count in the neutral `.extension-status`
// chip beside it (`:448-472`, min-width 25u, an 18u box, the caption step at
// 600 in `--text-secondary`). The old port reused `section`, so the panel's
// headings came out one step larger in `--text-strong` and the count chip was
// missing entirely.
func TestExtensionSectionTitleWearsTheReferenceRegister(t *testing.T) {
	a := New(newStore(t), Actions{})

	var heading, secondary, strong ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		tokens := a.tokens(c)
		heading = c.Theme().TextMuted
		secondary, strong = tokens.TextSecondary, tokens.TextStrong
		a.extensionSection(c, "Base plugins", 137, func() {})
	}, 600, 200)
	tt.Frame()

	// The title is `--text-muted`, measurably apart from the strong step the
	// other pages' section labels use.
	title, ok := tt.Find("Base plugins")
	if !ok {
		t.Fatalf("the section title is missing: %q", tt.Texts())
	}
	bg, _ := modalColor(tt, title)
	titleInk := inkColor(tt, title)
	if want := overSurface(heading, bg); !colorClose(titleInk, want, 14) {
		t.Errorf("title ink = %v, want the reference's --text-muted over the page (%v)", titleInk, want)
	}
	if old := overSurface(strong, bg); colorClose(titleInk, old, 14) {
		t.Errorf("title ink = %v still wears .settings-section__label's --text-strong", titleInk)
	}

	// The count rides its own chip, in `--text-secondary`. The 10px glyph is
	// small enough that antialiasing keeps its darkest pixel off the full
	// token, so the check is which of the two candidates it is nearer — the
	// neutral chip step, not `.settings-section__title`'s strong one.
	chip, ok := tt.Find("137")
	if !ok {
		t.Fatalf("the section count chip is missing: %q", tt.Texts())
	}
	cbg, _ := modalColor(tt, chip)
	chipInk := inkColor(tt, chip)
	wantChip := overSurface(secondary, cbg)
	strongChip := overSurface(strong, cbg)
	if colorDistance(chipInk, wantChip) > colorDistance(chipInk, strongChip) {
		t.Errorf("count ink = %v is nearer --text-strong (%v) than --text-secondary (%v)", chipInk, strongChip, wantChip)
	}
	if colorDistance(chipInk, wantChip) > 30*30*3 {
		t.Errorf("count ink = %v is not the reference's --text-secondary over the chip (%v)", chipInk, wantChip)
	}

	// A negative count drops the chip: a group with a title and nothing to
	// count draws no number.
	tt2 := ui.NewTester(func(c *ui.Context) {
		a.extensionSection(c, "Detected", -1, func() {})
	}, 400, 120)
	tt2.Frame()
	if tt2.HasText("-1") {
		t.Errorf("a negative count was drawn: %q", tt2.Texts())
	}
}

// The Base plugins group's explanation is the reference's
// `settings-section__hint` line *under the title and above the list*
// (`ExtensionsPanel.tsx:2435-2436`), not the settings cards' footnote after
// the cards. The old port drew it at the end of the section, below every
// plugin card.
func TestBasePluginHintRidesAboveTheList(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{OpenPluginConfig: func(string) {}})
	a.Integrations = func() []Integration { return nil }
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	hint, ok := tt.Find(en.BasePluginsHint)
	if !ok {
		t.Fatalf("the base-plugins hint is missing: %q", tt.Texts())
	}
	first, ok := tt.Find(en.BaseClipboard)
	if !ok {
		t.Fatalf("the first base plugin is missing: %q", tt.Texts())
	}
	if hint.Y >= first.Y {
		t.Errorf("the base-plugins hint (Y=%v) is not above the list (Y=%v)", hint.Y, first.Y)
	}
}

// The Detected group's permanent disclosure is the reference's
// `.extensions-section-hint` (`ExtensionsPanel.tsx:2542-2544`;
// `extensions.css:711-720`): a caption-sized, tertiary sentence under the
// title and above the rows. The old port pushed it to the end of the section.
func TestDetectedHintRidesAboveTheList(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{
		Detected: func() []DetectedTool {
			return []DetectedTool{{Name: "ripgrep", Path: "/usr/bin/rg", RuntimeAvailable: true}}
		},
		ConnectDetected: func(DetectedTool) {},
	})
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	hint, ok := tt.Find(en.DetectedHint)
	if !ok {
		t.Fatalf("the detected hint is missing: %q", tt.Texts())
	}
	row, ok := tt.Find("ripgrep")
	if !ok {
		t.Fatalf("the detected row is missing: %q", tt.Texts())
	}
	if hint.Y >= row.Y {
		t.Errorf("the detected hint (Y=%v) is not above the list (Y=%v)", hint.Y, row.Y)
	}
}

// The panel's action button is the reference's `.extensions-action-button`
// (`extensions.css:1313-1338`): the 30u control height, the body step at 600,
// and a 999px corner. The old port used the framework's flat `ui.Button`
// (about 28.5u with the 9u control radius), so every authoring action stood a
// step short and squared off.
func TestTheActionButtonsWearTheReferencePill(t *testing.T) {
	a := New(newStore(t), Actions{})

	var ref ui.Handle
	var box ui.Rect
	var height, pad float32
	tt := ui.NewTester(func(c *ui.Context) {
		height, pad = c.Theme().Space(7.5), c.Theme().Space(2.5)
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			a.actionButton(c, "Connect").Bind(&ref)
		})
		box = c.Resolve(ref).Bounds()
	}, 400, 200)
	tt.Frame()

	if box.H < height-0.5 || box.H > height+0.5 {
		t.Errorf("action button height = %v, want the reference's 30u (%v)", box.H, height)
	}
	label, ok := tt.Find("Connect")
	if !ok {
		t.Fatalf("the action label is missing: %q", tt.Texts())
	}
	// `.extensions-action-button { padding: 0 10px }`: the words keep a 10u
	// side inset inside the pill.
	if got := label.X - box.X; absFloat(got-pad) > 1 {
		t.Errorf("leading inset = %v, want the reference's 10u (%v)", got, pad)
	}
}
