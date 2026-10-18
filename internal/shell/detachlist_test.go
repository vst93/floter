package shell

import (
	"image/color"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
)

// detachedTokens is the theme's resolved tokens for the shell's stored
// settings: the palette the detached and pinned windows draw with (their own
// Contexts resolve the same way, see `detachedView`).
func (a *App) detachedTokens() theme.Tokens {
	return launcher.Resolve(a.Store.Snapshot(), true, ui.Preferences{}).Tokens
}

// The detached window's list arm draws the reference's row plates
// (`DetachedPluginApp.tsx:341-350`, `.plugin-window__row`,
// `main:src/styles/launcher.css:1792-1818`) instead of printing the list
// protocol's raw JSON.
//
// This is the surface's most visible defect. `DetachedPluginApp` resolved the
// list emission into rows before R95, and the port drew `OutputView.Text`
// unconditionally — which for a list is the *serialized protocol*
// (`{"output":[{"id":…}]}`), not what the plugin meant to say. A command that
// pinned its list out therefore showed its own wire format.
func TestTheDetachedWindowDrawsTheListArmAsRows(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	run := extensions.CapturedRun{Stdout: `{"output":[` +
		`{"id":"a","title":"Alpha","subtitle":"one"},` +
		`{"id":"b","title":"Beta"}]}`}
	view := a.Launcher.OutputFor(run, nil, nil)
	if !view.IsList || len(view.Rows) != 2 {
		t.Fatalf("the emission did not parse as a list: IsList=%v rows=%d", view.IsList, len(view.Rows))
	}
	detached := &detachedOutput{title: "jv", ran: true, view: view}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()

	for _, want := range []string{"Alpha", "one", "Beta"} {
		if !tt.HasText(want) {
			t.Errorf("the list arm did not draw %q: %q", want, tt.Texts())
		}
	}
	// The protocol itself is never on screen: that is the bug this pins.
	for _, text := range tt.Texts() {
		if strings.Contains(text, `"id"`) || strings.Contains(text, "output") {
			t.Errorf("the list arm printed its own wire format: %q", text)
		}
	}

	// A status row is information *about* the list, not an entry in it, so it
	// wears the muted status line rather than a plate (the same split the
	// launcher's own list makes).
	statusView := a.Launcher.OutputFor(
		extensions.CapturedRun{Stdout: `[{"id":"s","title":"Nothing here","kind":"status"}]`}, nil, nil)
	detached = &detachedOutput{title: "jv", ran: true, view: statusView}
	tt = ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	status, ok := tt.Find("Nothing here")
	if !ok {
		t.Fatalf("the status row was not drawn: %q", tt.Texts())
	}
	// A plate is a row's own box: its text sits inside the plate's 10u padding,
	// so a row's title starts one step right of a status line's — the status
	// line has no plate and starts on the body's own 14u inset
	// (`.plugin-window__status` vs `.plugin-window__row`,
	// `launcher.css:1792-1818`).
	rowView := a.Launcher.OutputFor(extensions.CapturedRun{Stdout: `[{"id":"r","title":"Alpha"}]`}, nil, nil)
	detached = &detachedOutput{title: "jv", ran: true, view: rowView}
	tt = ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	row, ok := tt.Find("Alpha")
	if !ok {
		t.Fatalf("the row was not drawn: %q", tt.Texts())
	}
	if row.X <= status.X {
		t.Errorf("a row's title is not inset inside its plate: row x=%.1f, status x=%.1f", row.X, status.X)
	}
}

// The pinned-text window is the reference's *text* arm of the same
// plugin-window surface (`DetachedPluginApp.tsx:308-350`): it wears the bar —
// the title on the left, the Close key on the right, and no Rerun, because a
// snapshot has no command to run — over the same text body. The port drew it
// barless, so a pinned clip had no in-app way to close it.
func TestThePinnedWindowWearsTheReferenceBar(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	copy := i18n.For("en").Launcher
	state := &pinned{title: "Clip 1", text: "some copied text"}
	tt := ui.NewTester(func(c *ui.Context) { a.pinnedView(c, state) }, 720, 400)
	tt.Frame()

	if !tt.HasText("Clip 1") {
		t.Errorf("the bar's title is missing: %q", tt.Texts())
	}
	if !tt.HasText("some copied text") {
		t.Errorf("the pinned text is missing: %q", tt.Texts())
	}
	// The title is the bar's, not the body's: it sits above the text, and its
	// box is the caption register — one step under the body's.
	titleRect, ok := tt.Find("Clip 1")
	if !ok {
		t.Fatal("the title was not drawn")
	}
	bodyRect, ok := tt.Find("some copied text")
	if !ok {
		t.Fatal("the body text was not drawn")
	}
	if titleRect.Y >= bodyRect.Y {
		t.Errorf("the title is not above the body: title y=%.0f, body y=%.0f", titleRect.Y, bodyRect.Y)
	}
	if titleRect.H >= bodyRect.H {
		t.Errorf("the bar title is not the quieter register: title box %.1f, body box %.1f", titleRect.H, bodyRect.H)
	}
	// The bar's Close key: its face is the X icon, so the word is only the
	// accessible name — found by the label, drawn as a square mark.
	closeRect, ok := tt.Find(copy.OutputWindowClose)
	if !ok {
		t.Fatalf("the Close key is missing: %q", tt.Texts())
	}
	if ratio := shellInkRatio(t, tt, closeRect); ratio >= 1.7 {
		t.Errorf("the Close key draws a word, not the X (ratio %.2f)", ratio)
	}
	// A snapshot has no command to re-run, so the reference draws no Rerun key.
	if tt.HasText(copy.OutputWindowRerun) {
		t.Errorf("a pinned text offered a re-run: %q", tt.Texts())
	}
}

// The detached window's body is the reference's own: `.plugin-window__body` is
// a 14u padding and nothing else (`launcher.css:1776-1781`) — no card frame, no
// border, no second inset. The port wrapped the output in a bordered, rounded
// pane, so the text floated in a box the design does not draw.
func TestTheDetachedBodyDrawsNoCardFrame(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	detached := &detachedOutput{title: "jv", ran: true, view: launcher.OutputView{Text: "x"}}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	text, ok := tt.Find("x")
	if !ok {
		t.Fatalf("the body was not drawn: %q", tt.Texts())
	}
	// The body's own inset is the reference's 14u (`launcher.css:1780`): 12.6
	// pixels at the shipped step, with no glyph bearing (the text starts on the
	// content box's edge). A card frame would put the text on 14u *plus* a
	// second padding (~26), and a 1px border would pull it in to ~8.
	// `.plugin-window__body { padding: calc(var(--u) * 14) }` is the block's own
	// inset: 14u = 3.5 `Space` steps at the interface step the frame is drawn at.
	spacing := float64(a.Launcher.Spacing)
	if spacing <= 0 {
		spacing = 4
	}
	want := 3.5 * spacing
	if diff := math.Abs(float64(text.X) - want); diff > 2 {
		t.Errorf("the body text starts at x=%.1f, want the reference's 14u inset %.1f (no frame, no second inset)", text.X, want)
	}
}

// launcherColor reads a frame pixel as the ui.Color the tokens use.
func launcherColor(c color.RGBA) ui.Color {
	return ui.Color{R: c.R, G: c.G, B: c.B, A: c.A}
}

// R95 · the detached window's list arm pages exactly as the launcher's does.
//
// The reference read the shared protocol rule and the shared page size in both
// windows (`DetachedPluginApp.tsx:109-127, 248-270` — `pagePluginEmission`,
// `PLUGIN_INITIAL_PAGES`, the scroll trigger against
// `PLUGIN_LOAD_MORE_THRESHOLD`), and the port wired the trigger into the
// launcher's own output view only. A plugin that emitted more than the first
// window therefore had rows in its detached window that nothing could reach:
// the list was silently truncated at two pages.
func TestTheDetachedListPagesLikeTheLauncher(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	run := extensions.CapturedRun{Stdout: "[" + detachRows(25) + "]"}
	view := a.Launcher.OutputFor(run, nil, nil)
	if want := extensions.PageSize * extensions.InitialPages; len(view.Rows) != want {
		t.Fatalf("the first window holds %d rows, want %d", len(view.Rows), want)
	}
	detached := &detachedOutput{title: "jv", ran: true, view: view}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	if tt.HasText("Row 25") {
		t.Fatalf("a row beyond the loaded window is on screen: %q", tt.Texts())
	}

	// Scrolled to the end, the body asks for the next page and draws it.
	detached.scroll.Y = math.MaxFloat32
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Row 25") {
		t.Fatalf("the next page never came: %q", tt.Texts())
	}
	if detached.view.Page != nil {
		t.Errorf("the complete list still claims a page: %+v", detached.view.Page)
	}
}

// A plugin that pages itself is asked with the cursor it named — in its own
// window, not only in the panel — and a page that does not speak the protocol
// ends the list instead of being asked for forever.
func TestTheDetachedSelfPagingListReplaysItsCursor(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	entry := &extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "paged", Name: "Paged"},
	}
	// Twenty rows: more than the window can show, so its end is not on screen
	// and the trigger has not fired yet (the reference's own first-page size).
	first := `{"output":[` + detachRows(20) + `],"page":{"cursor":"c1","hasMore":true}}`
	view := a.Launcher.OutputFor(extensions.CapturedRun{Stdout: first}, nil, entry)
	if !view.Declared || view.Page == nil || !view.Page.HasMore {
		t.Fatalf("the self-paging emission was not read as one: %+v", view.Page)
	}
	var asks int
	var gotCursor string
	a.Launcher.Actions.LoadOutputPage = func(_ extensions.CommandEntry, _ []string, cursor string, done func(extensions.CapturedRun, error)) {
		asks++
		gotCursor = cursor
		done(extensions.CapturedRun{Stdout: `{"output":[{"id":"last","title":"Last"}],"page":{"cursor":null,"hasMore":false}}`}, nil)
	}
	detached := &detachedOutput{title: "jv", ran: true, view: view}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	if asks != 0 {
		t.Fatalf("an unscrolled body asked for a page: %d", asks)
	}
	detached.scroll.Y = math.MaxFloat32
	tt.Frame()
	if asks != 1 || gotCursor != "c1" {
		t.Fatalf("the cursor was not replayed: asked %d times with %q", asks, gotCursor)
	}
	tt.Frame()
	if !tt.HasText("Last") {
		t.Fatalf("the fetched page was not drawn: %q", tt.Texts())
	}
	// The plugin said it was done: the trigger stops asking.
	tt.Frame()
	if asks != 1 {
		t.Errorf("a finished list was asked again: %d", asks)
	}
}

// detachRows is n list-protocol rows, numbered from one.
func detachRows(n int) string {
	rows := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		rows = append(rows, `{"id":"r`+strconv.Itoa(i)+`","title":"Row `+strconv.Itoa(i)+`"}`)
	}
	return strings.Join(rows, ",")
}
