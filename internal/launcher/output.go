package launcher

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/ansi"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/theme"
)

// The captured-output view: what a command whose manifest sends its output to
// the background said, shown in the launcher rather than in a terminal.
//
// The old build's plugin text view had a three-line floor, a ten-row ceiling
// and scrolled inside it; this is the same shape, and the two keys are the
// old ones: Enter copies the text, Escape closes the view.

// OutputView is one captured run, ready to show: its rows when the output is
// the list protocol, its text otherwise.
type OutputView struct {
	// Title is the command line that ran.
	Title string
	// Text is what it wrote (standard output, with standard error appended
	// when there is any).
	Text string
	// Rows are the list protocol's rows, when the output is a list; a nil
	// slice with IsList false means the output is text.
	Rows []extensions.Row
	// IsList reports whether the output is a list (which may be empty).
	IsList bool
	// Selected is the chosen row in a list, -1 while none is.
	Selected int
	// Status is the one-line summary: the exit code, the duration, and
	// whether the output was cut or the run timed out.
	Status string
	// FontFamily is the family the text is drawn in: the terminal's own, so
	// a command's output reads the way it would in the terminal.
	FontFamily string
	// Spans are the text's runs with the styles its own ANSI sequences asked
	// for: a CLI writes colour and weight for a terminal, and the text form
	// reads them rather than printing the bytes (see internal/ansi). Text
	// holds the same characters with every sequence removed, for copying.
	Spans []ansi.Span
	// Entry is the command the output came from, when it came from one: the
	// window that can be opened for it re-runs the same command.
	Entry *extensions.CommandEntry
	// Args are the arguments the run was given, kept so a later page of a
	// self-paging list re-runs the same command.
	Args []string
	// AllRows is the full set of rows the emission carried, before any
	// in-memory windowing; Rows is the window actually drawn.
	AllRows []extensions.Row
	// Page is the list's pagination state: the plugin's own block when it
	// pages itself, the window's block when the launcher pages in memory, and
	// nil for a list that is complete in one emission.
	Page *extensions.Page
	// PagesLoaded is how many viewport pages the in-memory window holds.
	PagesLoaded int
	// Declared is true when the plugin pages itself: its cursor is replayed
	// rather than the window being grown.
	Declared bool
	// LoadingMore is true while a self-paging plugin's next page is in flight.
	LoadingMore bool
}

// Copy is the launcher's copy in the stored language, for a caller outside
// the launcher (the shell's notification wording).
func (a *App) Copy() i18n.Launcher { return a.copy() }

// RunStatus is the output view's one-line summary, for a caller that reports
// the same run elsewhere (a notification).
func (a *App) RunStatus(run extensions.CapturedRun, err error) string {
	copy := a.copy()
	if err != nil && !run.Success && !run.TimedOut {
		return a.failureReason(err)
	}
	return a.runStatus(copy, run)
}

// failureReason is a failed run's one-line reason. A keyed failure (a stable
// key plus a JSON payload) is localised; so is a `run_param_*` refusal, which
// carries the parameter id instead of a payload. Anything else keeps its own
// words.
func (a *App) failureReason(err error) string {
	if err == nil {
		return ""
	}
	copy := i18n.For(a.settings().Language).Settings
	if failure, ok := extensions.ParseRunFailure(err.Error()); ok {
		return runErrorMessage(copy, failure)
	}
	if failure, ok := extensions.ParseParamFailure(err.Error()); ok {
		return paramRunMessage(copy, failure)
	}
	return err.Error()
}

// showOutput puts a captured run's output in front of the user: as a list when
// the command printed the list protocol, as text otherwise.
func (a *App) showOutput(run extensions.CapturedRun, err error, entry *extensions.CommandEntry, args []string) {
	view := a.OutputFor(run, err, entry)
	view.Args = args
	a.output = &view
	a.Selected, a.chosenRow = 0, -1
}

// OutputFor builds the view of one captured run: its text (or the list
// protocol's rows), its status, and the command it came from. The launcher
// shows it in its own band; a caller outside can show the same view in a
// window of its own (see the shell's detached window).
func (a *App) OutputFor(run extensions.CapturedRun, err error, entry *extensions.CommandEntry) OutputView {
	copy := a.copy()
	// The text is read as a terminal reads it: the styles it asked for are
	// kept as spans, and the characters without them are what a copy takes.
	raw := run.Text()
	view := OutputView{
		Entry:      entry,
		Title:      strings.Join(run.Command, " "),
		Text:       ansi.Plain(raw),
		Spans:      ansi.Parse(raw),
		Status:     a.runStatus(copy, run),
		FontFamily: a.settings().FontFamily,
		Selected:   -1,
	}
	// A list is read from standard output alone: standard error beside it is
	// a warning, not part of the protocol.
	if emission, ok := extensions.ParseEmission(run.Stdout); ok {
		view.Rows, view.IsList = emission.Rows, true
		view.AllRows = emission.Rows
		view.PagesLoaded = extensions.InitialPages
		view.Declared = emission.Page != nil
		view.Page = emission.Page
		if !view.Declared {
			// A plugin that hands over its whole list is windowed in memory,
			// one viewport page at a time: a complete list gets no block at all,
			// so a short list draws no footer and every row.
			rows, page := extensions.WindowRows(emission.Rows, view.PagesLoaded)
			view.Rows = rows
			if page.HasMore {
				view.Page = &page
			} else {
				view.Page = nil
			}
		}
		view.Selected = firstRunnable(view.Rows)
	}
	if !view.IsList {
		if err != nil && view.Text == "" {
			view.Text = a.failureReason(err)
		}
		if view.Text == "" {
			view.Text = copy.OutputEmpty
		}
	}
	return view
}

// firstRunnable is the first row Enter could run, -1 when none is.
func firstRunnable(rows []extensions.Row) int {
	for i, row := range rows {
		if row.Runnable() {
			return i
		}
	}
	return -1
}

// loadMoreOutputPage asks for the next page of an output list: a self-paging
// plugin is re-run with the cursor from its last emission, and an in-memory
// window simply grows by one page. A complete list, or a fetch already in
// flight, is left alone.
func (a *App) loadMoreOutputPage() {
	view := a.output
	if view == nil || !view.IsList || view.Page == nil || !view.Page.HasMore || view.LoadingMore {
		return
	}
	if view.Declared {
		if view.Entry == nil || a.Actions.LoadOutputPage == nil {
			return
		}
		view.LoadingMore = true
		entry := *view.Entry
		args := append([]string{}, view.Args...)
		cursor := view.Page.Cursor
		a.Actions.LoadOutputPage(entry, args, cursor, func(run extensions.CapturedRun, err error) {
			if a.output != view {
				return // the view was closed or replaced while the page ran
			}
			view.LoadingMore = false
			emission, ok := extensions.ParseEmission(run.Stdout)
			if !ok {
				// The plugin stopped speaking the protocol: keep what we have
				// and stop asking, rather than looping on a page we cannot read.
				view.Page = &extensions.Page{}
				return
			}
			view.Rows = extensions.MergeRows(view.Rows, emission.Rows)
			view.AllRows = view.Rows
			if emission.Page != nil {
				view.Page = emission.Page
			} else {
				view.Page = &extensions.Page{}
			}
		})
		return
	}
	view.PagesLoaded++
	rows, page := extensions.WindowRows(view.AllRows, view.PagesLoaded)
	view.Rows = rows
	if page.HasMore {
		view.Page = &page
	} else {
		view.Page = nil
	}
}

// outputFooter draws the list's pagination line. The scroll trigger is the
// affordance, so "more" is the quiet state; a self-paging plugin's fetch in
// flight says so, and a list the plugin has finished says nothing.
func (a *App) outputFooter(c *ui.Context, copy i18n.Launcher, view *OutputView) {
	if view.Page == nil {
		return
	}
	t := c.Theme()
	text := copy.OutputMore
	switch {
	case view.LoadingMore:
		text = copy.OutputLoading
	case !view.Page.HasMore:
		text = copy.OutputEnd
	}
	ui.Row(c).FillWidth().Shrink(0).Padding(t.Space(1), t.Space(2)).Justify(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
	})
}

// moveOutputSelection moves the list's choice by one, skipping rows that
// cannot be run.
func (a *App) moveOutputSelection(delta int) {
	if a.output == nil || !a.output.IsList {
		return
	}
	rows := a.output.Rows
	index := a.output.Selected
	for i := 0; i < len(rows); i++ {
		index += delta
		if index < 0 {
			index = len(rows) - 1
		}
		if index >= len(rows) {
			index = 0
		}
		if rows[index].Runnable() {
			a.output.Selected = index
			a.OutputList.ScrollIntoView(index)
			return
		}
	}
}

// runOutputRow does what Enter does on the chosen row: open a page, copy text,
// or put it back in the field. A row with no action does nothing.
func (a *App) runOutputRow() {
	if a.output == nil || !a.output.IsList {
		return
	}
	rows := a.output.Rows
	if a.output.Selected < 0 || a.output.Selected >= len(rows) {
		return
	}
	row := rows[a.output.Selected]
	if !row.Runnable() {
		return
	}
	copy := a.copy()
	switch row.Action.Type {
	case "open":
		if a.Actions.OpenURL != nil {
			a.Actions.OpenURL("", row.Action.URL)
		}
		a.leaveOutput()
		a.Hide()
	case "copy":
		if a.Actions.Copy != nil {
			a.Actions.Copy(row.Action.Text)
			a.toast = copy.Copied
		}
	case "insert":
		// Leaving clears the field, so the text goes in after it.
		a.leaveOutput()
		a.Query = row.Action.Text
		a.pendingCaret = true
	}
}

// outputText is what Enter copies from the view: the text, or the chosen row's
// text in a list.
func (a *App) outputText() string {
	if a.output == nil {
		return ""
	}
	if !a.output.IsList {
		return a.output.Text
	}
	rows := a.output.Rows
	if a.output.Selected < 0 || a.output.Selected >= len(rows) {
		return ""
	}
	row := rows[a.output.Selected]
	if row.Action != nil && row.Action.Type != "open" {
		return row.Action.Text
	}
	if row.Subtitle != "" {
		return row.Title + " \u2014 " + row.Subtitle
	}
	return row.Title
}

// runStatus is the output view's summary line.
func (a *App) runStatus(copy i18n.Launcher, run extensions.CapturedRun) string {
	parts := []string{}
	switch {
	case run.TimedOut:
		parts = append(parts, copy.OutputTimedOut)
	case run.Success:
		parts = append(parts, copy.OutputSucceeded)
	default:
		parts = append(parts, fmt.Sprintf(copy.OutputFailed, run.ExitCode))
	}
	if run.Duration > 0 {
		parts = append(parts, extensions.FormatRunDuration(run.Duration.Seconds()*1000))
	}
	if run.Truncated {
		parts = append(parts, copy.OutputTruncated)
	}
	return strings.Join(parts, "  ·  ")
}

// leaveOutput closes the output view.
func (a *App) leaveOutput() {
	if a.output == nil {
		return
	}
	a.output = nil
	a.Query = ""
	a.Selected, a.chosenRow = 0, -1
}

// InOutputView reports whether a captured run's output is on screen.
func (a *App) InOutputView() bool { return a.output != nil }

// outputBody draws the captured output in place of the result list: the
// status line pinned under the field, and the text or the rows scrolling under
// it.
func (a *App) outputBody(c *ui.Context, copy i18n.Launcher, edge float32) {
	view := a.output
	t := c.Theme()
	header := t.Space(4)
	ui.Column(c).Fill().Children(func() {
		if view.IsList {
			ui.Column(c).Fill().Children(func() {
				ui.List(c.Key("launcher.output.list"), &a.OutputList, len(view.Rows), func(i int) {
					a.outputRow(c, view.Rows[i], i)
				}).Fill().Padding(edge+header, t.Space(2), 0, t.Space(2)).Label(view.Title)
				// The scroll trigger: reaching the end of what is loaded asks for
				// the next page, exactly as the reference's own list did.
				if a.OutputList.AtEnd() {
					a.loadMoreOutputPage()
				}
				a.outputFooter(c, copy, view)
			})
		} else {
			ui.Scroll(c.Key("launcher.output")).TrackScroll(&a.Output).Fill().
				Padding(edge+header, t.Space(3), t.Space(1), t.Space(3)).Children(func() {
				ui.RichText(c, a.outputSpans(c, view)...).Font(view.FontFamily).
					FontSize(t.FontSize).TextColor(t.Text).Selectable()
			})
		}
		ui.Column(c).Absolute().Top(edge).Left(t.Space(3)).Right(t.Space(3)).Children(func() {
			ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, view.Title).FontSize(t.FontSize).TextColor(t.Text).Ellipsis("\u2026").SingleLine().Grow(1)
				// The output's own controls: 28u transparent squares with a
				// mark, the reference's `.plugin-window__bar-button`, never
				// filled pills in the text surface's header.
				tokens := theme.For(a.settings(), t.Dark)
				control := func(mark, label, hint string, run func()) {
					if run == nil {
						return
					}
					button := ui.Row(c).Focusable().Shrink(0).Size(t.Space(7), t.Space(7)).
						Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
						Label(label).Tooltip(hint)
					if button.Hovered() {
						button.Background(tokens.ControlHover)
					}
					button.Children(func() {
						ui.Text(c, mark).FontSize(tokens.Emphasis).TextColor(t.TextMuted)
					})
					if button.Clicked() {
						run()
					}
				}
				// The output can be shown in a window of its own: the old
				// build's own pin, on the surface that holds the text.
				if a.Actions.PinText != nil {
					control("\u2197", copy.HistoryPin, copy.HistoryPin, func() { a.Actions.PinText(view.Title, view.Text) })
				}
				// And a command's page can be detached into a window that
				// stays put: the old build's second window, which never hides
				// on blur and re-runs the command from its own controls.
				if a.Actions.DetachOutput != nil && view.Entry != nil {
					control("\u29c9", copy.OutputDetach, copy.OutputDetachHint, func() { a.Actions.DetachOutput(*view) })
				}
			})
			ui.Text(c, view.Status+"  \u00b7  "+a.outputHint(copy, view)).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		})
	})
}

// outputHint is the key line: a list can be walked and run, text can only be
// copied.
func (a *App) outputHint(copy i18n.Launcher, view *OutputView) string {
	if view.IsList {
		return copy.OutputListHint
	}
	return copy.OutputHint
}

// outputRow draws one list row: its group heading when it starts a group, its
// icon, and its two lines. A status row is muted and cannot be chosen.
func (a *App) outputRow(c *ui.Context, row extensions.Row, i int) {
	t := c.Theme()
	if i > 0 && row.Group != "" && row.Group != a.output.Rows[i-1].Group {
		ui.Text(c, row.Group).FontSize(t.FontSize-1).TextColor(t.TextMuted).
			Padding(t.Space(1), t.Space(2), 0, t.Space(2))
	}
	style := ui.Row(c).Key(row.ID).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(t.Radius).Gap(t.Space(2))
	selected := i == a.output.Selected && row.Runnable()
	switch {
	case selected:
		// The list's own selection: a quiet accent tint with the emphasis in
		// the row's type, exactly as the launcher's rows draw it.
		style.Background(t.Accent.Alpha(0.085))
	case row.Status || row.Disabled:
		// A note is not a door: it stays muted and never highlights.
	default:
		if style.Hovered() {
			style.Background(t.SurfaceHover)
		}
	}
	style.Children(func() {
		dim := row.Status || row.Disabled
		plate := a.iconPlate(c, false)
		if dim {
			plate.Opacity(0.62)
		}
		plate.Children(func() {
			if glyph, ok := outputGlyph(row.Icon); ok {
				ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
			}
		})
		ui.Column(c).Grow(1).Children(func() {
			color := t.Text
			if dim {
				color = t.TextMuted
			}
			title := ui.Text(c, row.Title).FontSize(t.FontSize).TextColor(color)
			if selected {
				title.FontWeight(700)
			}
			if row.Subtitle != "" {
				subtitle := ui.Text(c, row.Subtitle).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				if selected {
					subtitle.TextColor(t.Text)
				}
			}
		})
	})
	if style.Clicked() && row.Runnable() {
		a.output.Selected = i
		a.runOutputRow()
	}
}

// outputSpans maps a command's own ANSI output to the spans the text element
// draws: the sixteen program colours of the current appearance, the weights
// and the decorations, with the surface's own colour for whatever the command
// left unsaid.
func (a *App) outputSpans(c *ui.Context, view *OutputView) []ui.Span {
	appearance := theme.For(a.settings(), c.Theme().Dark).Appearance
	base := c.Theme().Text
	out := make([]ui.Span, 0, len(view.Spans))
	for _, span := range view.Spans {
		if span.Text == "" {
			continue
		}
		style := span.Style
		fg := ansiColor(appearance, style.FG, base)
		bg := ansiColor(appearance, style.BG, ui.Transparent)
		if style.Inverse {
			fg, bg = ansiColor(appearance, style.BG, base), ansiColor(appearance, style.FG, c.Theme().Background)
		}
		if style.Dim && style.FG.Kind == ansi.Default {
			// Dim on the surface's own colour is a lighter one, since there
			// is no brightness to halve.
			fg = fg.Alpha(0.7)
		}
		weight := 400
		if style.Bold {
			weight = 700
		}
		out = append(out, ui.Span{
			Text:          span.Text,
			Size:          c.Theme().FontSize,
			Weight:        weight,
			Italic:        style.Italic,
			Color:         fg,
			Underline:     style.Underline,
			Strikethrough: style.Strike,
			Background:    bg,
		})
	}
	if len(out) == 0 {
		out = append(out, ui.Span{Text: view.Text, Size: c.Theme().FontSize, Color: base})
	}
	return out
}

// ansiColor resolves one parsed colour, falling back to the surface's own for
// a colour the command did not name.
func ansiColor(appearance theme.Appearance, color ansi.Color, fallback ui.Color) ui.Color {
	switch color.Kind {
	case ansi.Named:
		return theme.ANSIColor(appearance, 1, color.N, 0, 0, 0)
	case ansi.Indexed:
		if color.N < 16 {
			return theme.ANSIColor(appearance, 1, color.N, 0, 0, 0)
		}
		return theme.ANSIColor(appearance, 2, color.N, 0, 0, 0)
	case ansi.RGB:
		return theme.ANSIColor(appearance, 3, 0, color.R, color.G, color.B)
	}
	return fallback
}
