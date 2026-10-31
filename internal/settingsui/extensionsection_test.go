package settingsui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
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

// integrationPage renders the Integrations page over one app, the way the
// panel's own tests reach the list.
func integrationPage(t *testing.T, a *App, w, h int) *ui.Tester {
	t.Helper()
	tt := render(t, a, w, h)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("open Integrations: %v", err)
	}
	tt.Frame()
	return tt
}

// oneIntegrationApp is integrationPage's fixture: an app whose installed list
// is the one integration, with the actions a row needs wired.
func oneIntegrationApp(integration Integration, actions Actions) *App {
	a := New(settings.NewStore(settings.Default()), actions)
	a.Integrations = func() []Integration { return []Integration{integration} }
	return a
}

// The reference's title line is `name` then `v{currentVersion}` in one
// `.extension-row__title` flex row (`src/extensions/ExtensionRow.tsx:207-230`),
// not a version parked on its own identity row. The old port printed the name
// and description as one label and the version far below on the identity line.
func TestIntegrationTitleCarriesTheVersion(t *testing.T) {
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Publisher: "vst", Version: "1.2.3",
		Enabled: true, Running: true,
	}, Actions{})
	tt := integrationPage(t, a, 720, 900)

	name := mustFind(t, tt, "V Tools")
	version := mustFind(t, tt, "v1.2.3")
	// The version sits on the name's own line, immediately after the name.
	if version.Y < name.Y || version.Y > name.Y+name.H {
		t.Errorf("the version at %v is not on the title line at %v", version, name)
	}
	if version.X < name.X+name.W {
		t.Errorf("the version at x=%v is not after the name ending at x=%v", version.X, name.X+name.W)
	}
	// It stays above the meta line the version used to live on.
	identity := mustFind(t, tt, "vst · 1.2.3 · dev.floter.tool")
	if version.Y >= identity.Y {
		t.Errorf("the version at %v is not above the meta line at %v", version, identity)
	}
}

// The freshness dot is the reference's `.extension-row__dot` in the *title*
// line (`src/extensions/ExtensionRow.tsx:220-230`), trailing the name and
// version, not a control at the row's trailing edge. The old port drew it in
// the first row's control slot, beside the state word and the unavailable
// badge.
func TestFreshnessDotRidesTheTitle(t *testing.T) {
	en := i18n.For("en").Settings
	count := 4
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "Tool", Version: "9.9.9",
		Enabled: true, Running: true, RuntimeAvailable: false,
	}, Actions{
		Freshness: func(id string) (extensions.Freshness, bool) {
			if id != "dev.floter.tool" {
				return extensions.Freshness{}, false
			}
			return extensions.Freshness{
				AtSeconds:            time.Now().Unix(),
				Source:               extensions.FreshnessProbe,
				Result:               extensions.FreshnessSuccess,
				Delta:                extensions.DeltaUnchanged,
				CommandCount:         &count,
				PreviousCommandCount: &count,
			}, true
		},
	})
	tt := integrationPage(t, a, 720, 900)

	name := mustFind(t, tt, "Tool")
	version := mustFind(t, tt, "v9.9.9")
	dot := mustFind(t, tt, en.FreshnessDot(en.FreshnessDotSynced))
	// Same title line as the name, after the version.
	if dot.Y < name.Y || dot.Y > name.Y+name.H {
		t.Errorf("the dot at %v is not on the title line at %v", dot, name)
	}
	if dot.X < version.X+version.W {
		t.Errorf("the dot at x=%v is not after the version ending at x=%v", dot.X, version.X+version.W)
	}
	// It is not in the control slot: the unavailable badge keeps that end of
	// the row, and the dot stays to its left.
	badge := mustFind(t, tt, en.RuntimeUnavailable)
	if dot.X >= badge.X {
		t.Errorf("the dot at x=%v is in the control slot (badge x=%v), not the title", dot.X, badge.X)
	}
}

// The reference's `.extension-row__meta` is one line under the title carrying
// the package identity, the kind, the runtime source and the state word
// (`src/extensions/ExtensionRow.tsx:255-282`). The old port scattered identity,
// kind and state across separate rows (and drew the state in the control slot).
func TestIntegrationMetaLineCombinesTheFacts(t *testing.T) {
	en := i18n.For("en").Settings
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "Tool", Publisher: "acme", Version: "1.0.0",
		DistributionSource: "npm", RuntimeSource: "managed",
		Enabled: true, Running: true, RuntimeAvailable: true,
	}, Actions{})
	tt := integrationPage(t, a, 720, 900)

	name := mustFind(t, tt, "Tool")
	// P13-af 重定基线（结构）：四个 `·` 曾经是行里的独立 flex 子节点，现在是
	// **它引入的那条事实自己的节点**里的第一个字形（参考的 `::before`，
	// `extensions.css:979-984`）——否则行一折行，dot 就留在上一行的末尾。
	// 所以后三条事实的节点文本各自带前导 `· `，第一条不带。
	meta := []ui.Rect{
		mustFind(t, tt, "acme · 1.0.0 · dev.floter.tool"),
		mustFind(t, tt, "· "+en.IntegrationsKindNPM),
		mustFind(t, tt, "· "+en.IntegrationsRuntimeManaged),
		mustFind(t, tt, "· "+en.IntegrationsRunning),
	}
	for _, r := range meta {
		if r.Y < name.Y+name.H {
			t.Errorf("the meta fact at %v is not under the title ending at %v", r, name.Y+name.H)
		}
	}
	// All four share the one meta line: their boxes overlap vertically, even
	// though each register's line box is a different height.
	sameLine := func(a, b ui.Rect) bool {
		return a.Y < b.Y+b.H && b.Y < a.Y+a.H
	}
	for _, r := range meta[1:] {
		if !sameLine(meta[0], r) {
			t.Errorf("the meta fact at %v is not on the identity fact's line at %v", r, meta[0])
		}
	}
	// The reference's order: identity, kind, runtime source, state.
	for i := 1; i < len(meta); i++ {
		if meta[i].X <= meta[i-1].X {
			t.Errorf("meta fact %d at x=%v is not after fact %d at x=%v", i, meta[i].X, i-1, meta[i-1].X)
		}
	}
}

// The toggle is the reference's `.extension-row__toggle-slot`, at the far end
// of the actions *after* every action button (`src/extensions/ExtensionRow.tsx:437-458`).
// The old port drew it first, inside the row's control slot, before the run,
// repair and removal doors.
func TestIntegrationToggleTrailsTheActions(t *testing.T) {
	en := i18n.For("en").Settings
	a := oneIntegrationApp(Integration{
		ID: "pkg", Name: "V Tools", DistributionSource: "local",
		Enabled: true, Running: true, RuntimeAvailable: true,
	}, Actions{
		UninstallIntegration:  func(string, string, string, func(error)) {},
		SetIntegrationEnabled: func(string, bool, func(error)) {},
	})
	tt := integrationPage(t, a, 720, 900)

	remove := mustFind(t, tt, en.IntegrationsRemovePackage)
	toggle := mustFind(t, tt, en.IntegrationsEnable)
	if toggle.X <= remove.X {
		t.Errorf("the toggle at x=%v does not trail the removal button at x=%v", toggle.X, remove.X)
	}
	// Same actions row: the two are centred on one another (the switch is a
	// touch taller than the pill, so the check is the vertical centres).
	if absFloat((toggle.Y+toggle.H/2)-(remove.Y+remove.H/2)) > remove.H/2 {
		t.Errorf("the toggle at %v is not on the actions row at %v", toggle, remove)
	}
}
