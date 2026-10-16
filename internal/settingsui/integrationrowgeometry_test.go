package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// The integration card is not a settings card. The reference draws each
// integration as one `.extension-row`: a 66px min-height with 8px of padding
// (`extensions.css:15-16,733-744`), so the card's title row is a 66u floor and
// every row under it — Permissions, Freshness, the command list, the action
// cluster — shares the same 8u inset. The old port drew the whole card with
// the settings geometry (a 44u floor and a 14u inset), so the title's chip sat
// on a different column from the sub-rows' labels and the card's separators.
//
// This renders a card through `cardGeometry` — the mechanism `integrationRow`
// uses — with the title row and one sub-row, and measures all three: the title
// row's own box height, the chip's left edge, and the separator's left edge,
// against the sub-row's label.
func TestTheIntegrationCardWearsTheExtensionRowGeometry(t *testing.T) {
	a := New(newStore(t), Actions{})

	var chipRef ui.Handle
	var chipBox ui.Rect
	var inset, floor float32
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		inset, floor = c.Theme().Space(2), c.Theme().Space(16.5)
		a.cardGeometry(c, inset, floor, c.Theme().Space(1), "", func() {
			a.cardRow(c, ui.Center,
				func() { a.extensionIcon(c, false).Bind(&chipRef) },
				rowFace{},
				func() {},
				func() { ui.Text(c, "V Tools") })
			a.row(c, "Permissions", "", func() {})
		})
		chipBox = c.Resolve(chipRef).Bounds()
	}, 480, 240)
	tt.Frame()

	// The title row is the card's first row: its box runs from the card's 1px
	// border down to the separator the Permissions row draws above itself.
	// Read the separator in a column inside its span but clear of both rows'
	// content.
	sepY := separatorY(tt, 440, int(floor)+15)
	titleHeight := sepY - 1
	if absFloat(titleHeight-floor) > 1 {
		t.Errorf("the title row is %v tall, want the reference extension row's 66u floor (%v)",
			titleHeight, floor)
	}

	// The chip leads the title row at the card's border plus the extension
	// row's 8u padding; the sub-row's label and the card's separator start on
	// that same column.
	perm := mustFind(t, tt, "Permissions")
	if want := 1 + inset; absFloat(chipBox.X-want) > 1 {
		t.Errorf("the title chip starts at x=%v, want the card's 1px border plus the extension row's 8u padding (%v)",
			chipBox.X, want)
	}
	if absFloat(chipBox.X-perm.X) > 1 {
		t.Errorf("the title chip starts at x=%v but the Permissions label at x=%v; the card's rows must share one inset",
			chipBox.X, perm.X)
	}
	if sepX := separatorLeft(tt, int(sepY), 440); absFloat(sepX-chipBox.X) > 1 {
		t.Errorf("the card's separator starts at x=%v, off the label column at x=%v", sepX, chipBox.X)
	}
}

// separatorY is the y of the card's separating hairline in the column at x,
// searched between y=2 and y1: the darkest pixel in that column (the hairline
// over the card's plane).
func separatorY(tt *ui.Tester, x, y1 int) float32 {
	img := tt.Image()
	best, bestY := 1<<30, 0
	for y := 2; y <= y1 && y < img.Rect.Dy(); y++ {
		p := imagePixel(img, x, y)
		if sum := int(p.R) + int(p.G) + int(p.B); sum < best {
			best, bestY = sum, y
		}
	}
	return float32(bestY)
}

// separatorLeft is the x where the hairline at row y begins: the first pixel
// that differs from the card's own plane (sampled a few rows above the
// hairline).
func separatorLeft(tt *ui.Tester, y, planeX int) float32 {
	img := tt.Image()
	plane := imagePixel(img, planeX, y-3)
	for x := 3; x < img.Rect.Dx(); x++ {
		if channelDelta(imagePixel(img, x, y), plane) > 6 {
			return float32(x)
		}
	}
	return 0
}

// The geometry reaches the live page, not just `cardGeometry`: the Integrations
// page's integration card wears the 8u extension inset while a base-plugin card
// beside it keeps the settings card's 14u, so the two label columns sit 6u
// apart. A card that fell back to the settings geometry would put both labels
// on one column and the delta at zero.
func TestTheIntegrationPageWearsTheExtensionInset(t *testing.T) {
	en := i18n.For("en").Settings
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Publisher: "vst", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Permissions: []string{"filesystem"},
	}, Actions{OpenPluginConfig: func(string) {}, SetPluginEnabled: func(string, bool) {}})
	tt := integrationPage(t, a, 720, 1600)

	perm := mustFind(t, tt, en.IntegrationsPermissions)
	base := mustFind(t, tt, en.BaseClipboard)
	// The settings card's 14u inset minus the extension card's 8u is 6u.
	tokens := theme.For(settings.Default(), false)
	want := tokens.Theme.Space(3.5) - tokens.Theme.Space(2)
	if absFloat((base.X-perm.X)-want) > 1.5 {
		t.Errorf("the base-plugin label sits %v right of the integration label, want the 6u between the 14u and 8u insets (%v)",
			base.X-perm.X, want)
	}
}

// The 66u floor reaches the page too: an integration with nothing but a name
// still draws a title row at the extension row's own height, not the bare
// content's and not the settings card's 44u. The row's box runs from the card's
// content top down to the separator the row under it draws.
func TestTheIntegrationTitleRowKeepsTheExtensionFloor(t *testing.T) {
	en := i18n.For("en").Settings
	a := oneIntegrationApp(Integration{ID: "x", Name: "V Tools", RuntimeAvailable: true}, Actions{})
	tt := integrationPage(t, a, 720, 1000)
	appr := mustFind(t, tt, en.ApprovalRecord)

	// Read the card's plane inside the approval row, then walk down a column
	// clear of every row's content (the card's right edge): the first
	// plane-coloured pixel is the card's content top, the next pixel that
	// leaves the plane is the separator above the approval row.
	plane := sample(tt, 670, appr.Y+appr.H/2)
	head := mustFind(t, tt, "Installed")
	top, sep := -1, -1
	for y := int(head.Y + head.H + 2); y < int(appr.Y); y++ {
		p := sample(tt, 670, float32(y))
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
		t.Fatalf("could not find the card's title row (top %d, separator %d)", top, sep)
	}
	titleHeight := float32(sep - top)
	// The content alone is ~55u-ish at the shipped scale; the 66u floor lifts
	// it. Assert the floor's own value with a hair of slack for the card's
	// border and the row's rounding.
	if want := theme.For(settings.Default(), false).Theme.Space(16.5); titleHeight < want-4 || titleHeight > want+8 {
		t.Errorf("the title row is %v tall, want the reference extension row's 66u floor (%v)",
			titleHeight, want)
	}
}

// The extension row's own line gap reaches the card's label column: the
// reference's `.extension-row__main` spaces a row's title, meta and
// description 4px apart (`extensions.css:879-880`), where the settings row
// uses 1px (`.settings-row__main { gap: 1px }`, `settings.css:528-531`). This
// renders the same two lines through `cardGeometry` with the extension gap and
// with the default, and reads the distance between them.
func TestTheIntegrationCardGivesItsLinesTheExtensionGap(t *testing.T) {
	a := New(newStore(t), Actions{})

	measure := func(lineGap float32) float32 {
		var first, second ui.Handle
		var firstBox, secondBox ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.cardGeometry(c, c.Theme().Space(2), c.Theme().Space(16.5), lineGap, "", func() {
				a.cardRow(c, ui.Center, nil, rowFace{}, func() {}, func() {
					ui.Text(c, "V Tools").Bind(&first)
					ui.Text(c, "the meta line").Bind(&second)
				})
			})
			firstBox = c.Resolve(first).Bounds()
			secondBox = c.Resolve(second).Bounds()
		}, 480, 200)
		tt.Frame()
		return secondBox.Y - (firstBox.Y + firstBox.H)
	}

	tokens := theme.For(settings.Default(), false)
	extensionGap, settingsGap := measure(tokens.Theme.Space(1)), measure(0)
	if absFloat(extensionGap-tokens.Theme.Space(1)) > 1 {
		t.Errorf("the card's line gap is %v, want the extension row's 4u (%v)", extensionGap, tokens.Theme.Space(1))
	}
	if absFloat(settingsGap-1) > 1 {
		t.Errorf("the settings card's line gap is %v, want its own 1px", settingsGap)
	}
	if extensionGap-settingsGap < 1.5 {
		t.Errorf("the integration card's line gap (%v) does not open past the settings card's (%v)",
			extensionGap, settingsGap)
	}
}

// The title line's own gap is the extension row's too: `.extension-row__title`
// spaces the name and the version 8px apart (`extensions.css:892-894`), not the
// 6u the port used.
func TestTheIntegrationTitleSpacesTheNameAndVersionByTheExtensionGap(t *testing.T) {
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3", RuntimeAvailable: true,
	}, Actions{})
	tt := integrationPage(t, a, 720, 1000)

	name := mustFind(t, tt, "V Tools")
	version := mustFind(t, tt, "v1.2.3")
	want := theme.For(settings.Default(), false).Theme.Space(2)
	if got := version.X - (name.X + name.W); absFloat(got-want) > 1.5 {
		t.Errorf("the name-to-version gap is %v, want the extension row's 8u (%v)", got, want)
	}
}
