package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// The extension panel is not a settings page. The reference's
// `extensions.css` is written in literal px — `--extensions-row-height: 66px`,
// `--extensions-icon-size: 30px`, `.extension-row { padding: 8px }`
// (`extensions.css:14-16,733-744`) — so its boxes measure the same at every
// interface step, where `settings.css` writes `--u` and the settings rows
// follow the step. The port drew the integration card through `t.Space(n)`
// (4n·scale), so at the shipped `small` (0.9) the whole card was 10% smaller
// than the reference's and grew again at `large` (1.1).
//
// This renders one integration card through the mechanism `integrationRow`
// uses — `extensionMetrics` around `cardGeometry` — at the two shipped steps
// and reads the package chip's edge and the title row's box. Both must be the
// reference's own numbers at both steps; a settings card rendered beside them
// must still follow the step, or the extension fix would have been a global
// freeze.
func TestTheExtensionCardKeepsItsMetricsAtEveryInterfaceStep(t *testing.T) {
	type measured struct {
		chip      float32
		rowHeight float32
		settings  float32
	}
	measure := func(step string) measured {
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.UIScale = step }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})

		var chipRef ui.Handle
		var chipBox ui.Rect
		var extFloor, extY1, setY1 float32
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.extensionMetrics(func() {
				t := a.cardTheme(c)
				extFloor = t.Space(16.5)
				extY1 = extFloor + 40
				a.cardGeometry(c, t.Space(2), extFloor, t.Space(1), "", func() {
					a.cardRow(c, ui.Center,
						func() { a.extensionIcon(c, false).Bind(&chipRef) },
						rowFace{}, func() {}, func() { ui.Text(c, "V Tools") })
					a.row(c, "Permissions", "", func() {})
				})
			})
			chipBox = c.Resolve(chipRef).Bounds()
			// A settings card beside it: the same one-row-plus-separator
			// shape, drawn with the settings geometry.
			setY1 = c.Theme().Space(11) + 40
			a.card(c, func() {
				a.row(c, "Theme", "", func() {})
				a.row(c, "Language", "", func() {})
			})
		}, 480, 400)
		tt.Frame()

		// The extension card is the top half of the frame; the settings card
		// starts below it. Read each separator in the same column.
		extSep := separatorY(tt, 440, int(extY1))
		return measured{
			chip:      chipBox.W,
			rowHeight: extSep - 1,
			settings:  settingsRowHeight(tt, 440, int(extY1)+20, int(setY1)+int(extY1)+20),
		}
	}

	small, large := measure("small"), measure("large")

	// The chip is the reference's 30px at both steps.
	if absFloat(small.chip-30) > 1 || absFloat(large.chip-30) > 1 {
		t.Errorf("the package chip is %vpx at small and %vpx at large, want the reference's literal 30px at both",
			small.chip, large.chip)
	}
	if absFloat(small.chip-large.chip) > 1 {
		t.Errorf("the chip changes with the interface step: %vpx at small, %vpx at large",
			small.chip, large.chip)
	}
	// The title row is the reference's 66px floor at both steps.
	if absFloat(small.rowHeight-66) > 2 || absFloat(large.rowHeight-66) > 2 {
		t.Errorf("the extension title row is %vpx at small and %vpx at large, want the reference's literal 66px at both",
			small.rowHeight, large.rowHeight)
	}
	if absFloat(small.rowHeight-large.rowHeight) > 1 {
		t.Errorf("the extension row height changes with the interface step: %vpx at small, %vpx at large",
			small.rowHeight, large.rowHeight)
	}
	// The settings row does follow the step: this is the contrast that proves
	// the extension fix is scoped, not a global freeze.
	if large.settings-small.settings < 3 {
		t.Errorf("the settings row is %vpx at small and %vpx at large; it must keep following the interface step",
			small.settings, large.settings)
	}
}

// settingsRowHeight reads the first settings row's box height from the frame:
// the distance from the card's content top to the separator the second row
// draws above itself. It walks the column at x, skipping the extension card
// above, until it finds the settings card's plane and then the hairline.
func settingsRowHeight(tt *ui.Tester, x, from, to int) float32 {
	img := tt.Image()
	// The settings card's plane is sampled a little below its first row.
	plane := imagePixel(img, x, from+8)
	top, sep := -1, -1
	for y := from; y < to && y < img.Rect.Dy(); y++ {
		p := imagePixel(img, x, y)
		if top < 0 {
			if channelDelta(p, plane) <= 4 {
				top = y
			}
			continue
		}
		if channelDelta(p, plane) > 12 {
			sep = y
			break
		}
	}
	if top < 0 || sep < 0 {
		return 0
	}
	return float32(sep - top)
}

// The audit a Permissions row unfolds is drawn on the card's content column,
// so it must carry the row's own inset: its tier labels ("Floter enforces" /
// "Declared, not blocked") belong on the same column as the Permissions label
// above them, not flush against the card's edge. The reference's
// `.extension-permission-list` pads its own list (`extensions.css:1497-1503`);
// the port drew the block with no horizontal padding, so the labels sat a whole
// inset (8px) left of the row they explain.
func TestThePermissionAuditAlignsWithTheRowLabels(t *testing.T) {
	en := i18n.For("en").Settings
	for _, step := range []string{"small", "large"} {
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.UIScale = step }); err != nil {
			t.Fatal(err)
		}
		a := oneIntegrationApp(Integration{
			ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
			Enabled: true, Running: true, RuntimeAvailable: true,
			Permissions: []string{"filesystem", "network"},
			Enforced:    map[string]bool{"filesystem": true},
		}, Actions{})
		a.Store = store
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, 720, 1600)
		tt.Frame()
		if err := tt.Click("Integrations"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()

		perm := mustFind(t, tt, en.IntegrationsPermissions)
		if err := tt.Click(en.IntegrationsPermissions); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		tier := mustFind(t, tt, en.PermissionsEnforcedLabel)
		if delta := perm.X - tier.X; absFloat(delta) > 1 {
			t.Errorf("[%s] the tier label starts at x=%v but the Permissions label at x=%v (delta %v); the audit must sit on the row's own column",
				step, tier.X, perm.X, delta)
		}
	}
}

// The run form's own frame is a `below` block of the Run row, so its heading
// ("Run parameters") and the card its inputs live in must start on the row
// labels' column, not flush against the card's edge — the reference's
// `.extension-run-params` box sits inside the row's own padding
// (`extensions.css:1224-1236`). This opens the live run form and reads the
// heading's x against the Permissions label's.
func TestTheRunFormStartsOnTheRowLabelColumn(t *testing.T) {
	en := i18n.For("en").Settings
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Permissions: []string{"filesystem"},
		Enforced:    map[string]bool{"filesystem": true},
		Params:      []Param{{ID: "target", Label: "Target", Kind: "text"}},
	}, Actions{RunWithParams: func(string, map[string]string) *extensions.ParamFailure { return nil }})
	a.runOpen = map[string]bool{"dev.floter.tool": true}
	tt := integrationPage(t, a, 720, 1800)

	perm := mustFind(t, tt, en.IntegrationsPermissions)
	header := mustFind(t, tt, en.RunValues)
	if delta := header.X - perm.X; absFloat(delta) > 1 {
		t.Errorf("the run form's heading starts at x=%v but the row labels at x=%v (delta %v); the form must carry the row's inset",
			header.X, perm.X, delta)
	}
}

// The audit block's own left edge must have moved off the card's frame, not
// merely stopped short of it: the old port drew the labels at the card's inner
// edge, so the tier's x was the card's left plus one border. This pins the
// movement itself, so a future "fix" that zeroes the padding is caught even if
// the row labels move too.
func TestThePermissionAuditMovesOffTheCardEdge(t *testing.T) {
	en := i18n.For("en").Settings
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Permissions: []string{"filesystem"},
		Enforced:    map[string]bool{"filesystem": true},
	}, Actions{})
	tt := integrationPage(t, a, 720, 1400)

	// The card's left edge is the darkest vertical hairline left of the row
	// label; read it before opening the audit.
	perm := mustFind(t, tt, en.IntegrationsPermissions)
	cardX := -1
	img := tt.Image()
	for x := int(perm.X) - 1; x > 0; x-- {
		p := imagePixel(img, x, int(perm.Y+perm.H/2))
		q := imagePixel(img, x-1, int(perm.Y+perm.H/2))
		if channelDelta(p, q) > 8 && p.R < 245 {
			cardX = x
			break
		}
	}
	if cardX < 0 {
		t.Fatalf("could not find the card's left edge left of the Permissions label at x=%v", perm.X)
	}

	if err := tt.Click(en.IntegrationsPermissions); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tier := mustFind(t, tt, en.PermissionsEnforcedLabel)
	if moved := tier.X - float32(cardX); moved < 7 {
		t.Errorf("the tier label starts only %vpx right of the card's edge (x=%v, edge x=%v); the audit must carry the row's inset",
			moved, tier.X, cardX)
	}
}
