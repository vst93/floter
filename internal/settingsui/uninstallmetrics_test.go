package settingsui

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// uninstallHairline is the one-pixel edge the dialog's own pane and each of its
// cards draws, in the reference's own unit: it is the piece every gutter
// measured off a card's checkbox has to carry, because the checkbox sits inside
// the card's border as well as inside its padding.
const uninstallHairline = 1

// The dialog's own numbers, read from the frame it is drawn in rather than from
// a theme built beside the tester: `uninstallSubBody` and the ladder's registers
// are what the dialog writes, and the framework's text boxes are the only handle
// a frame gives a test on a register (it reports no font size).
type uninstallMetrics struct {
	space    float32
	body     float32
	subBody  float32
	emphasis float32
	title    float32
	tokens   theme.Tokens
}

func uninstallDialogMetrics(t *testing.T, a *App) uninstallMetrics {
	t.Helper()
	var m uninstallMetrics
	ui.NewTester(func(c *ui.Context) {
		tk := c.Theme()
		m.tokens = a.tokens(c)
		m.space = tk.Space(1)
		m.body = tk.FontSize
		m.subBody = uninstallSubBody(tk)
		m.emphasis = m.tokens.Emphasis
		m.title = m.tokens.Title
	}, 100, 100).Frame()
	if m.space <= 0 || m.body <= 0 {
		t.Fatal("the frame resolved no theme")
	}
	return m
}

// uninstallFrame is one frame of the componentized uninstall drawn on its own,
// with the dialog's own box. The dialog's blocks are laid out against that box,
// so reading them here is what makes the reference's gutters measurable at all;
// the page route reads the same numbers through a page of offsets.
type uninstallFrame struct {
	uninstallMetrics
	frame *ui.Tester
	box   ui.Rect
}

func drawUninstallDialog(t *testing.T, a *App, integration Integration, w, h int) uninstallFrame {
	t.Helper()
	copy := i18n.For("en").Settings
	var ref ui.Handle
	var box ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		a.uninstallDialog(c, copy, integration).Bind(&ref)
		box = c.Resolve(ref).Bounds()
	}, w, h)
	tt.Frame()
	tt.Frame()
	return uninstallFrame{uninstallMetrics: uninstallDialogMetrics(t, a), frame: tt, box: box}
}

func (f uninstallFrame) mustFind(t *testing.T, text string) ui.Rect {
	t.Helper()
	box, ok := f.frame.Find(text)
	if !ok {
		t.Fatalf("the dialog does not draw %q: %v", text, f.frame.Texts())
	}
	return box
}

// uninstallTestApp is the dialog's own fixture: one installed integration and an
// uninstall action, with nothing else on the page.
func uninstallTestApp() *App {
	a := newTestApp()
	a.Actions = Actions{UninstallComponents: func(string, string, string, UninstallComponents, func(error)) {}}
	a.Integrations = func() []Integration {
		return []Integration{{ID: "npm", Name: "NPM Tool", DistributionSource: "npm"}}
	}
	return a
}

// npmTool is the integration every dialog test draws.
func npmTool() Integration {
	return Integration{ID: "npm", Name: "NPM Tool", DistributionSource: "npm"}
}

// registerHeight lays one line of text at a size and reads the box's height —
// the only handle a frame gives on a register.
func registerHeight(t *testing.T, text string, size float32, weight int) float32 {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		line := ui.Text(c, text).FontSize(size)
		if weight > 0 {
			line.FontWeight(weight)
		}
	}, 400, 100)
	tt.Frame()
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the register probe did not draw %q", text)
	}
	return box.H
}

// hintHeight lays the dialog's hint out at a size and the pane's own content
// width, so a wrapped line's height can be compared with the dialog's.
func hintHeight(t *testing.T, text string, size, width float32) float32 {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(width).Children(func() {
			ui.Text(c, text).FontSize(size)
		})
	}, int(width)+40, 400)
	tt.Frame()
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the hint probe did not draw %q", text)
	}
	return box.H
}

// actionButtonWidth draws the dialog's own action pill on its own and reads its
// natural width: a row lays its children out at their content width, where the
// root's column would stretch the pill across the frame.
func actionButtonWidth(t *testing.T, a *App, marked bool) float32 {
	t.Helper()
	en := i18n.For("en").Settings
	icon := checkIcon
	if !marked {
		icon = nil
	}
	var ref ui.Handle
	var box ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			a.uninstallActionButton(c, en.IntegrationsUninstall, true, false, icon).Bind(&ref)
		})
		box = c.Resolve(ref).Bounds()
	}, 400, 100)
	tt.Frame()
	return box.W
}

// The dialog's own pane is the reference's `.extensions-dialog`
// (`ComponentizedUninstallDialog.css:17-34`): 17px of padding on every side —
// the title's own corner reads it — between a `--input-stroke` hairline and the
// `--glass-float` material, and for the uninstall variant the reference's own
// 420–480px width band (`.extensions-uninstall-dialog`, `:36-39`). The pane used
// to be a bare column with no width of its own, so it took whatever the page's
// row gave it.
func TestComponentizedUninstallDialogIsTheReferenceFloatingPane(t *testing.T) {
	en := i18n.For("en").Settings
	a := uninstallTestApp()
	integration := npmTool()

	wide := drawUninstallDialog(t, a, integration, 640, 900)
	wantInset := wide.space*4.25 + uninstallHairline
	title := wide.mustFind(t, en.IntegrationsUninstallTitle(integration.Name))
	if inset := title.X - wide.box.X; absFloat(inset-wantInset) > 0.6 {
		t.Errorf("the pane's left padding is %.1f, want %.1f (17px + the hairline)", inset, wantInset)
	}
	if inset := title.Y - wide.box.Y; absFloat(inset-wantInset) > 0.6 {
		t.Errorf("the pane's top padding is %.1f, want %.1f (17px + the hairline)", inset, wantInset)
	}
	// `max-width: 480px`: a window with room to spare gets the ceiling.
	if want := wide.space * 120; absFloat(wide.box.W-want) > 0.6 {
		t.Errorf("the pane is %.1f wide in a 640px window, want the reference's 480px ceiling (%.1f)", wide.box.W, want)
	}
	// `min-width: 420px`: a window without the room gets the floor, not the
	// window.
	narrow := drawUninstallDialog(t, a, integration, 400, 900)
	if want := narrow.space * 105; absFloat(narrow.box.W-want) > 0.6 {
		t.Errorf("the pane is %.1f wide in a 400px window, want the reference's 420px floor (%.1f)", narrow.box.W, want)
	}
}

// The dialog's own gutters are the reference's two numbers, not one even gap:
// the hint hangs 6px under the title (`.extensions-dialog-hint { margin: 6px 0 0 }`,
// `:46-51`), the component list 16px under both
// (`.extensions-uninstall-components { margin: 16px 0 }`, `:53-58`), the cards
// 8px apart (`gap: 8px`), and each card carries its own 12px padding and 4px
// row gap (`.extensions-uninstall-component`, `:60-73`). The dialog used to draw
// all four blocks on one 4px gap, which is neither of the reference's numbers.
//
// Each gutter is read off a box the frame reports: a card's checkbox is its
// first child at that 12px padding, so the room from a hint's bottom edge to the
// next checkbox is the reference's gutter plus the padding and the card's
// hairline.
func TestComponentizedUninstallDialogSpacing(t *testing.T) {
	en := i18n.For("en").Settings
	a := uninstallTestApp()
	integration := npmTool()
	f := drawUninstallDialog(t, a, integration, 640, 900)
	space := f.space
	cardInset := space*3 + uninstallHairline

	title := f.mustFind(t, en.IntegrationsUninstallTitle(integration.Name))
	hint := f.mustFind(t, en.IntegrationsUninstallDescription)
	if gap := hint.Y - (title.Y + title.H); absFloat(gap-space*1.5) > 0.6 {
		t.Errorf("the hint sits %.1f under the title, want the reference's 6px (%.1f)", gap, space*1.5)
	}

	program := f.mustFind(t, en.UninstallProgram)
	if gap := program.Y - (hint.Y + hint.H); absFloat(gap-(space*4+cardInset)) > 0.6 {
		t.Errorf("the component list starts %.1f under the hint, want its 16px margin plus the card's own inset (%.1f)",
			gap, space*4+cardInset)
	}
	programHint := f.mustFind(t, en.UninstallProgramHint)
	host := f.mustFind(t, en.UninstallHostConfig)
	if gap := host.Y - (programHint.Y + programHint.H); absFloat(gap-(space*2+2*cardInset)) > 0.6 {
		t.Errorf("one card's edge to the next is %.1f, want the reference's 8px between them (%.1f)",
			gap, space*2+2*cardInset)
	}
	artifactsHint := f.mustFind(t, en.UninstallArtifactsHint)
	cancel := f.mustFind(t, en.PermissionsCancel)
	if gap := cancel.Y - (artifactsHint.Y + artifactsHint.H); absFloat(gap-(space*4+cardInset)) > 0.6 {
		t.Errorf("the actions sit %.1f under the list, want its 16px margin plus the card's own inset (%.1f)",
			gap, space*4+cardInset)
	}

	// Inside a card: the label's own line, the reference's 4px row gap, then the
	// hint. The checkbox is the card's first child, so the room from it to the
	// hint is that line plus that gap — and the line is the label at the ladder's
	// emphasis step, which is what pins the label's register.
	labelLine := registerHeight(t, en.UninstallHostConfig, f.emphasis, 600)
	if got, want := f.mustFind(t, en.UninstallHostConfigHint).Y-host.Y, labelLine+space; absFloat(got-want) > 0.6 {
		t.Errorf("the label and its hint are %.1f apart, want the label's own line at the emphasis step plus the reference's 4px gap (%.1f)",
			got, want)
	}
}

// The two actions are the reference's `.extensions-action-button`
// (`extensions.css:1313-1385`): the row is `justify-content: flex-end; gap: 7px`
// (`ComponentizedUninstallDialog.css:74-78`), so the pair ends flush at the
// pane's content edge seven units apart, and each pill is the family's 30px
// control height with the mark leading the word — the framework's own `Button`
// is left-aligned, pads 14px a side and carries its mark after the label.
func TestComponentizedUninstallActionsAreTheReferencePill(t *testing.T) {
	en := i18n.For("en").Settings
	a := uninstallTestApp()
	integration := npmTool()
	f := drawUninstallDialog(t, a, integration, 640, 900)
	space := f.space

	cancel := f.mustFind(t, en.PermissionsCancel)
	confirm := f.mustFind(t, en.IntegrationsUninstall)
	if gap := confirm.X - (cancel.X + cancel.W); absFloat(gap-space*1.75) > 0.6 {
		t.Errorf("the two actions are %.1f apart, want the reference's 7px (%.1f)", gap, space*1.75)
	}
	contentRight := f.box.X + f.box.W - uninstallHairline - space*4.25
	if gap := contentRight - (confirm.X + confirm.W); absFloat(gap) > 0.6 {
		t.Errorf("the actions end %.1f short of the pane's content edge, want the reference's flex-end (flush)", gap)
	}
	if want := space * 7.5; absFloat(cancel.H-want) > 0.6 {
		t.Errorf("the pill is %.1f tall, want the family's 30px control height (%.1f)", cancel.H, want)
	}
	// `<Check size={14} strokeWidth={2} />` leads the word, and the family's
	// `--extensions-inline-gap` (5px) sits between them: the same pill with its
	// mark is exactly that much wider.
	plain, marked := actionButtonWidth(t, a, false), actionButtonWidth(t, a, true)
	if got, want := marked-plain, space*3.5+space*1.25; absFloat(got-want) > 0.6 {
		t.Errorf("the confirm's mark adds %.1f to the pill, want the reference's 14px mark and 5px gap (%.1f)", got, want)
	}
}

// The dialog's text is the reference's registers, not offsets off the body size:
// the title is `--text-title` (13px) in `--text-strong`
// (`.extensions-dialog-title`, `:41-45`) and both hints are the 10.5px step
// between caption and body that `.extensions-dialog-hint` and
// `.extensions-component-hint` write (`:46-51,113-118`). The component label's
// `--text-emphasis` (12px) is pinned by the spacing test's label-line relation.
// A frame reports no font size, so each register is read as the height of the
// same line laid out at that size — the two differ, which is what makes the
// reading a test rather than a restatement.
func TestComponentizedUninstallTypeRegisters(t *testing.T) {
	en := i18n.For("en").Settings
	a := uninstallTestApp()
	integration := npmTool()
	f := drawUninstallDialog(t, a, integration, 640, 900)

	titleText := en.IntegrationsUninstallTitle(integration.Name)
	title := f.mustFind(t, titleText)
	titleLine := registerHeight(t, titleText, f.title, 0)
	bodyLine := registerHeight(t, titleText, f.body, 0)
	if absFloat(title.H-titleLine) > 0.6 {
		t.Errorf("the title's line is %.1f tall, want the ladder's title step (%.1f)", title.H, titleLine)
	}
	if absFloat(titleLine-bodyLine) < 0.6 {
		t.Fatalf("the title and body steps lay out at the same height (%.1f): the probe cannot tell them apart", titleLine)
	}

	hintText := en.IntegrationsUninstallDescription
	contentWidth := f.box.W - 2*uninstallHairline - 2*f.space*4.25
	hint := f.mustFind(t, hintText)
	subBodyLines := hintHeight(t, hintText, f.subBody, contentWidth)
	bodyLines := hintHeight(t, hintText, f.body, contentWidth)
	if absFloat(hint.H-subBodyLines) > 0.6 {
		t.Errorf("the hint's block is %.1f tall, want the same line at the 10.5px sub-body step (%.1f)", hint.H, subBodyLines)
	}
	if absFloat(hint.H-bodyLines) < 0.6 {
		t.Errorf("the hint is still drawn at the body step (both blocks are %.1f tall)", hint.H)
	}
}

// The card answers the pointer the way the reference's own rule does
// (`ComponentizedUninstallDialog.css:60-96`): the disabled program row is the
// inert one — its card and its box both wear the not-allowed cursor, and the
// data rows wear neither — while the hover and press rungs are colours alone,
// so a pointer over a card may not move a single box.
func TestComponentizedUninstallCardAnswersThePointer(t *testing.T) {
	en := i18n.For("en").Settings
	a := uninstallTestApp()
	integration := npmTool()
	f := drawUninstallDialog(t, a, integration, 640, 900)

	program := f.mustFind(t, en.UninstallProgram)
	host := f.mustFind(t, en.UninstallHostConfig)
	// The card's own body, clear of the checkbox that leads it.
	const cardBodyX = 300
	for _, probe := range []struct {
		name string
		x, y float32
		want ui.Cursor
	}{
		{"the disabled row's box", program.X + program.W/2, program.Y + program.H/2, ui.CursorNotAllowed},
		{"the disabled row's card", cardBodyX, program.Y + program.H/2, ui.CursorNotAllowed},
		{"a data row's box", host.X + host.W/2, host.Y + host.H/2, ui.CursorDefault},
		{"a data row's card", cardBodyX, host.Y + host.H/2, ui.CursorDefault},
	} {
		f.frame.Move(probe.x, probe.y)
		f.frame.Frame()
		f.frame.Frame()
		if got := f.frame.Cursor(); got != probe.want {
			t.Errorf("the cursor over %s is %v, want %v", probe.name, got, probe.want)
		}
	}

	// The rungs are colours, never geometry: the same boxes come back with the
	// pointer resting on a card.
	before := f.mustFind(t, en.UninstallHostConfigHint)
	f.frame.Move(cardBodyX, host.Y+host.H/2)
	f.frame.Frame()
	f.frame.Frame()
	if after := f.mustFind(t, en.UninstallHostConfigHint); before != after {
		t.Errorf("a pointer over a card moved it: %+v -> %+v", before, after)
	}
	if !f.frame.HasText(en.UninstallArtifactsHint) {
		t.Errorf("the dialog lost a row under the pointer: %v", f.frame.Texts())
	}
}

// The card's own `--elev-0` is the raised lens the resting pane carries
// (`ComponentizedUninstallDialog.css:60-73`), and the press rule sets
// `box-shadow: none` as it sinks (`:74-77`). The lens is top-weighted — the
// lit rim and sheen over the bottom shade — so it is read as the top-minus-
// bottom spread of the card's own scanlines, a metric the press fill's colour
// cancels out of. A pointer held down is the one rung that shows the flat
// press fill alone.
func TestComponentizedUninstallCardDropsItsShadowOnPress(t *testing.T) {
	a := uninstallTestApp()
	on := true
	part := uninstallPart{Label: "Host configuration", Hint: "Remove the host configuration.", Value: &on}
	var ref ui.Handle
	var card ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(ui.DarkTheme())
		ui.Column(c).Fill().Background(ui.RGB(24, 25, 29)).Padding(40).Children(func() {
			a.uninstallPartRow(c, part).Bind(&ref)
		})
		card = c.Resolve(ref).Bounds()
	}, 500, 300)
	tt.Frame()
	tt.Frame()
	if card.W == 0 || card.H == 0 {
		t.Fatalf("the card did not lay out: %+v", card)
	}
	// The top and bottom scanlines sit inside the card's border and clear of
	// its rounded corners and its label, so only the glass changes between
	// them.
	spread := func() float64 {
		x0, x1 := int(card.X+card.W*0.3), int(card.X+card.W*0.7)
		top := rowMeanRGB(tt, int(card.Y)+3, x0, x1)
		bottom := rowMeanRGB(tt, int(card.Y+card.H)-3, x0, x1)
		return top - bottom
	}
	rest := spread()
	tt.Press(card.X+card.W/2, card.Y+card.H/2)
	tt.Frame()
	tt.Frame()
	pressed := spread()
	if rest <= 0.5 {
		t.Errorf("the resting card has no top-weighted lens to remove: spread %.2f", rest)
	}
	if math.Abs(pressed) >= rest {
		t.Errorf("the press did not drop the card's shadow: rest spread %.2f, press spread %.2f", rest, pressed)
	}
}
