package settingsui

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// The reference's extension row leads with a package chip: every row draws
// `.extension-row__open` as a `[icon | main]` grid (`extensions.css:858-869`),
// the chip being `.extension-row__icon` — a 30u square with a 1px `--icon-edge`
// hairline, the `--ext-radius` corner and the `--icon-surface` plane
// (`:871-889`) — carrying lucide's `Package` at 17u
// (`src/extensions/ExtensionRow.tsx:179-181`). The old port went straight from
// the card's edge into the title. This renders one `cardRow` with the chip as
// its lead and reads both boxes: the chip is the reference's 30u square, the
// title clears it by the reference's 12u gap, and the title's own origin has
// moved right by the whole chip plus that gap.
func TestTheExtensionRowLeadsWithThePackageChip(t *testing.T) {
	a := New(newStore(t), Actions{})

	var chipRef, titleRef ui.Handle
	var chipBox, titleBox ui.Rect
	var side, gap float32
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.cardRows = 0 // the row is the card's first, as on a live card
		side, gap = c.Theme().Space(7.5), c.Theme().Space(3)
		a.cardRow(c, ui.Center,
			func() { a.extensionIcon(c, false).Bind(&chipRef) },
			rowFace{},
			func() {},
			func() { ui.Text(c, "V Tools").Bind(&titleRef) })
		chipBox = c.Resolve(chipRef).Bounds()
		titleBox = c.Resolve(titleRef).Bounds()
	}, 480, 120)
	tt.Frame()

	if absFloat(chipBox.W-side) > 0.5 || absFloat(chipBox.H-side) > 0.5 {
		t.Errorf("the package chip is %vx%v, want the reference's 30u square (%v)",
			chipBox.W, chipBox.H, side)
	}
	if got := titleBox.X - (chipBox.X + chipBox.W); absFloat(got-gap) > 1 {
		t.Errorf("the chip-to-title gap is %v, want the reference's 12u (%v)", got, gap)
	}
	if got := titleBox.X - chipBox.X; got < side+gap-1 {
		t.Errorf("the title starts only %v right of the chip, want the chip's 30u plus the 12u gap (%v)",
			got, side+gap)
	}
	if chipBox.X <= 0 {
		t.Errorf("the chip sits at x=%v; it must lead the row, inside its leading rule", chipBox.X)
	}
}

// The chip is a real chip, not a blank square: its edge is the `--icon-edge`
// hairline and its face the `--icon-surface` plane. A detected row wears the
// same box as a ghost — the reference's dashed edge, no plane and
// `--text-tertiary` ink (`extensions.css:833-837`).
func TestThePackageChipIsSolidAndTheDetectedOneIsAGhost(t *testing.T) {
	a := New(newStore(t), Actions{})

	draw := func(detected bool) (*ui.Tester, ui.Rect) {
		t.Helper()
		var chipRef ui.Handle
		var chipBox ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.extensionIcon(c, detected).Bind(&chipRef)
			chipBox = c.Resolve(chipRef).Bounds()
		}, 200, 100)
		tt.Frame()
		return tt, chipBox
	}

	solid, solidBox := draw(false)
	ghost, ghostBox := draw(true)

	if solidBox.W == 0 || ghostBox.W == 0 {
		t.Fatalf("the chips have no box: solid=%v ghost=%v", solidBox, ghostBox)
	}
	// Both chips are the same box.
	if absFloat(solidBox.W-ghostBox.W) > 0.5 || absFloat(solidBox.H-ghostBox.H) > 0.5 {
		t.Fatalf("the solid chip %v and the ghost %v are different boxes", solidBox, ghostBox)
	}

	// The solid chip draws a continuous top edge; the ghost's is dashed, so it
	// has gaps. Count the ink runs along each top edge.
	plane := sample(solid, 150, 80)
	solidRuns := inkRuns(solid, solidBox.Y+0.5, solidBox.X, solidBox.X+solidBox.W, plane)
	ghostRuns := inkRuns(ghost, ghostBox.Y+0.5, ghostBox.X, ghostBox.X+ghostBox.W, plane)
	if solidRuns != 1 {
		t.Errorf("the solid chip's top edge breaks into %d runs, want a continuous hairline", solidRuns)
	}
	if ghostRuns < 2 {
		t.Errorf("the detected chip's top edge is one run (%d); the reference draws it dashed", ghostRuns)
	}

	// The solid chip has a face; the ghost does not. Read a point inside the
	// chip but clear of the 17u glyph (the glyph spans the middle ~17u).
	insideY := solidBox.Y + solidBox.H/2
	solidInside := sample(solid, solidBox.X+2.5, insideY)
	ghostInside := sample(ghost, ghostBox.X+2.5, insideY)
	if channelDelta(solidInside, plane) < 4 {
		t.Errorf("the solid chip's face %v matches the plane outside it; it must carry --icon-surface",
			solidInside)
	}
	if delta := channelDelta(ghostInside, plane); delta > 4 {
		t.Errorf("the detected chip has a face (%v, delta %d from the plane); the reference draws none",
			ghostInside, delta)
	}
}

// inkRuns counts the contiguous runs of ink along a horizontal scanline: a
// solid 1px hairline is one run, a dashed one is several. It reads the frame
// once — `Tester.Image` converts the whole bitmap — and indexes it directly.
func inkRuns(tt *ui.Tester, y, x0, x1 float32, plane ui.Color) int {
	img := tt.Image()
	runs, ink := 0, false
	for x := int(x0); x < int(x1); x++ {
		d := channelDelta(imagePixel(img, x, int(y)), plane)
		switch {
		case d > 12 && !ink:
			runs++
			ink = true
		case d <= 12:
			ink = false
		}
	}
	return runs
}

// imagePixel reads one pixel out of a tester frame.
func imagePixel(img *image.RGBA, x, y int) ui.Color {
	if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
		return ui.Color{}
	}
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// The reference lights a 1px accent rule on the row's leading edge on hover
// (`.extension-row::before` + `:hover::before { background: var(--accent) }`,
// `extensions.css:747-771`) over the ladder's hover fill. The old port only
// swapped the fill in. This hovers a live row and reads the rule's own pixel at
// the row's left edge.
func TestTheRowLeadingRuleLightsOnHover(t *testing.T) {
	a := New(newStore(t), Actions{})

	var labelRef ui.Handle
	var labelBox ui.Rect
	var inset float32
	var accent ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.cardRows = 0 // the row is the card's first, as on a live card
		inset, accent = c.Theme().Space(3.5), c.Theme().Accent
		a.cardRow(c, ui.Center, nil, rowFace{}, func() {}, func() {
			ui.Text(c, "V Tools").Bind(&labelRef)
		})
		labelBox = c.Resolve(labelRef).Bounds()
	}, 480, 120)
	tt.Frame()

	// The row's left edge is the label's inset back: the 1px rule plus the
	// content's own `inset-1` padding add up to `inset`.
	rowX := labelBox.X - inset
	midY := labelBox.Y + labelBox.H/2
	restRule := sample(tt, rowX+0.5, midY)
	restNext := sample(tt, rowX+1.5, midY)

	tt.Move(labelBox.X+labelBox.W/2, midY)
	tt.Frame()
	hoverRule := sample(tt, rowX+0.5, midY)
	hoverNext := sample(tt, rowX+1.5, midY)

	if channelDelta(restRule, restNext) > 6 {
		t.Errorf("the rule is lit at rest (%v vs the plane %v); the reference draws it only on hover",
			restRule, restNext)
	}
	if channelDelta(hoverRule, hoverNext) < 30 {
		t.Errorf("hover did not light the leading rule: rule %v, plane %v", hoverRule, hoverNext)
	}
	// The rule is the accent, not just any lift.
	if delta := channelDelta(hoverRule, accent); delta > 60 {
		t.Errorf("the lit rule is %v, want the accent %v", hoverRule, accent)
	}
	if channelDelta(hoverNext, restNext) < 4 {
		t.Errorf("hover did not lift the row's fill: %v → %v", restNext, hoverNext)
	}
}

// The row a `floter://register` link named is the row the link **chose**, so it
// wears the contract's neutral selection rung (`RowSelected`) and nothing else:
// no keyline, no leading rule. A broken row wears `.extension-row--broken`
// (`extensions.css:843-853`): the warm raised pane, the warning keyline and the
// warning rule. This pins the state-to-face mapping the integration row asks
// for, so a later change cannot quietly drop one of the marks.
//
// P8-c 重定基线（§6.3 第 2 条 / §5），**S**（结构真变了）：旧映射是
// `RaisedQuiet` + `AccentEdge` + `accent` bar 三处都给 register 行。合约 §4.3
// 末句说选中行在列表里一次都不带 accent，§5 只给 accent 留侧栏图标/主 CTA/焦点环
// /控件状态，所以这三处全换成中性：面 `RowSelected`，无环，无前导规则条。
func TestTheIntegrationRowFaceFollowsItsState(t *testing.T) {
	a := New(newStore(t), Actions{})

	var faces struct{ plain, register, broken rowFace }
	var tokens theme.Tokens
	var warning ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = a.tokens(c)
		c.SetTheme(tokens.Theme)
		warning = c.Theme().Warning
		faces.plain = a.integrationRowFace(c, Integration{})
		faces.register = a.integrationRowFace(c, Integration{Highlighted: true})
		faces.broken = a.integrationRowFace(c, Integration{Broken: true})
	}, 100, 100)
	tt.Frame()

	if faces.plain.Bar.A != 0 || faces.plain.FaceSet || faces.plain.Inset.A != 0 {
		t.Errorf("a plain row wears %+v, want no rule, no face and no keyline", faces.plain)
	}
	if !faces.register.FaceSet || faces.register.Face != tokens.RowSelected {
		t.Errorf("the register row's face is %v (set %v), want the neutral RowSelected %v",
			faces.register.Face, faces.register.FaceSet, tokens.RowSelected)
	}
	if faces.register.Inset.A != 0 {
		t.Errorf("the register row's keyline is %v, want none (accent belongs to the leading mark alone)",
			faces.register.Inset)
	}
	if faces.register.Bar.A != 0 {
		t.Errorf("the register row's leading rule is %v, want none (§4.3: a selected list row carries no accent)",
			faces.register.Bar)
	}
	if !faces.broken.FaceSet || faces.broken.Face != tokens.RaisedWarm {
		t.Errorf("the broken row's face is %v (set %v), want the warm pane %v",
			faces.broken.Face, faces.broken.FaceSet, tokens.RaisedWarm)
	}
	if faces.broken.Inset != warning {
		t.Errorf("the broken row's keyline is %v, want --text-warning %v", faces.broken.Inset, warning)
	}
	if faces.broken.Bar != warning {
		t.Errorf("the broken row's rule is %v, want --text-warning %v", faces.broken.Bar, warning)
	}
}

// The warm pane is a token of its own now (base.css's `--glass-raised-warm`, an
// alias of `--system-icon-surface`), so a broken row's fill and a normal row's
// fill read apart on the live page rather than only in the status word.
func TestTheBrokenRowWearsTheWarmPane(t *testing.T) {
	a := oneIntegrationApp(Integration{
		ID: "ok", Name: "Normal Tool", Enabled: true, Running: true,
	}, Actions{})
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "ok", Name: "Normal Tool", Enabled: true, Running: true, RuntimeAvailable: true},
			{ID: "bad", Name: "Broken Tool", Enabled: true, Broken: true, RuntimeAvailable: false},
		}
	}
	tt := integrationPage(t, a, 720, 1400)

	normal := mustFind(t, tt, "Normal Tool")
	broken := mustFind(t, tt, "Broken Tool")

	// A point inside each row's plane, just left of its title (past the
	// package chip, which ends 12u before the title).
	normalPlane := sample(tt, normal.X-2, normal.Y+normal.H/2)
	brokenPlane := sample(tt, broken.X-2, broken.Y+broken.H/2)
	if delta := channelDelta(normalPlane, brokenPlane); delta < 8 {
		t.Errorf("the broken row's plane %v matches the normal row's %v (delta %d); the reference warms it",
			brokenPlane, normalPlane, delta)
	}
}

// The face a state asks for is painted, not just computed: the register row's
// neutral selected pane, and the broken row's warm pane with its warning
// keyline and rule, all land on the frame at the row's own edges. A face that
// only lived in the mapping would leave the live row on the card's plane.
//
// P8-c 重定基线（§6.3 第 2 条）：register 行的环与前导规则条被合约收掉（S）。
// 所以两行不再共用同一套三段断言：broken 行仍要求「环与前导规则条都要站得
// 出面」，register 行反过来要求「面站得出来，而环与前导规则条都 **在**」——
// 这就是 §5 的禁止侧。
func TestTheRowFacePaintsItsPaneAndKeyline(t *testing.T) {
	a := New(newStore(t), Actions{})

	type measure struct {
		pane, key, bar ui.Color
	}
	read := func(face func(c *ui.Context) rowFace) measure {
		t.Helper()
		var labelRef ui.Handle
		var labelBox ui.Rect
		var inset float32
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.cardRows = 0 // the row is the card's first
			inset = c.Theme().Space(3.5)
			a.cardRow(c, ui.Center, nil, face(c), func() {}, func() {
				ui.Text(c, "V Tools").Bind(&labelRef)
			})
			labelBox = c.Resolve(labelRef).Bounds()
		}, 480, 120)
		tt.Frame()
		rowX := labelBox.X - inset
		midY := labelBox.Y + labelBox.H/2
		return measure{
			pane: sample(tt, rowX+6, midY),
			key:  sample(tt, rowX+6, 0.5), // the 1px keyline on the top edge
			bar:  sample(tt, rowX+0.5, midY),
		}
	}

	register := read(func(c *ui.Context) rowFace {
		return a.integrationRowFace(c, Integration{Highlighted: true})
	})
	broken := read(func(c *ui.Context) rowFace {
		return a.integrationRowFace(c, Integration{Broken: true})
	})

	// The register row's *pane* stands off the card (`RowSelected` over it),
	// and neither the keyline nor the leading rule draws at all.
	if channelDelta(register.pane, register.key) > 3 {
		t.Errorf("the register row still wears a keyline: top edge %v vs its pane %v", register.key, register.pane)
	}
	if channelDelta(register.bar, register.pane) > 3 {
		t.Errorf("the register row still draws a leading rule: %v vs its pane %v", register.bar, register.pane)
	}

	// The broken row keeps all three marks.
	if channelDelta(broken.key, broken.pane) < 12 {
		t.Errorf("the broken row's keyline %v does not stand off its pane %v; the row has no inner edge",
			broken.key, broken.pane)
	}
	if channelDelta(broken.bar, broken.pane) < 20 {
		t.Errorf("the broken row's leading rule %v does not stand off its pane %v", broken.bar, broken.pane)
	}

	// The two faces are different panes, not one shared warm-or-quiet lift.
	if delta := channelDelta(register.pane, broken.pane); delta < 8 {
		t.Errorf("the register pane %v and the broken pane %v are the same (delta %d)",
			register.pane, broken.pane, delta)
	}
}

// The live rows lead with the chip on both lists: the connected list through
// `integrationRow` and the Detected list through the chip-bearing row helper,
// so a tool the machine has reads like a row that is merely not connected yet.
// The section group's own box is the card's edge, so the title's distance from
// it is the chip plus its gap; the band where the chip must sit is checked for
// the mark's ink, which a row without a chip leaves on the card's plane.
func TestTheLiveIntegrationRowsLeadWithTheChip(t *testing.T) {
	en := i18n.For("en").Settings
	a := oneIntegrationApp(Integration{
		ID: "ok", Name: "Connected Tool", Enabled: true, Running: true, RuntimeAvailable: true,
	}, Actions{
		Detected: func() []DetectedTool {
			return []DetectedTool{{Name: "Ghost Tool", Path: "/usr/bin/ghost", RuntimeAvailable: true}}
		},
		ConnectDetected: func(DetectedTool) {},
	})
	tt := integrationPage(t, a, 720, 1600)

	connectedGroup := mustFind(t, tt, sectionLabel(en.IntegrationsInstalled))
	detectedGroup := mustFind(t, tt, sectionLabel(en.DetectedTitle))
	connected := mustFind(t, tt, "Connected Tool")
	ghost := mustFind(t, tt, "Ghost Tool")

	for _, tc := range []struct {
		name  string
		group ui.Rect
		title ui.Rect
	}{
		{"connected", connectedGroup, connected},
		{"detected", detectedGroup, ghost},
	} {
		// The title clears the card's edge by the chip's 30u plus the 12u gap
		// (and the row's own inset), never by the inset alone.
		if got := tc.title.X - tc.group.X; got < 42 {
			t.Errorf("the %s row's title starts %v from the card edge, want the 30u chip plus the 12u gap",
				tc.name, got)
		}
		// And the chip's own band carries the mark's ink: the title distance
		// alone would also pass on a row that merely reserved the space.
		band := ui.Rect{X: tc.group.X + 12, Y: tc.title.Y - 30, W: 36, H: 70}
		if got := inkCount(tt, band); got < 20 {
			t.Errorf("the %s row's chip band %v has only %d ink pixels; the chip is not drawn",
				tc.name, band, got)
		}
	}
}

// inkCount is how many pixels in a box differ from the box's own most common
// colour by more than a hairline: the ink a mark lays down on its plane. It
// reads the frame once and indexes it directly.
func inkCount(tt *ui.Tester, r ui.Rect) int {
	img := tt.Image()
	plane := panelSurface(tt, r)
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if channelDelta(imagePixel(img, x, y), plane) > 25 {
				n++
			}
		}
	}
	return n
}
