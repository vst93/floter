package launcher

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
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
	// Entry is the command the output came from, when it came from one: the
	// window that can be opened for it re-runs the same command.
	Entry *extensions.CommandEntry
}

// Copy is the launcher's copy in the stored language, for a caller outside
// the launcher (the shell's notification wording).
func (a *App) Copy() i18n.Launcher { return a.copy() }

// RunStatus is the output view's one-line summary, for a caller that reports
// the same run elsewhere (a notification).
func (a *App) RunStatus(run extensions.CapturedRun, err error) string {
	copy := a.copy()
	if err != nil && !run.Success && !run.TimedOut {
		return err.Error()
	}
	return a.runStatus(copy, run)
}

// showOutput puts a captured run's output in front of the user: as a list when
// the command printed the list protocol, as text otherwise.
func (a *App) showOutput(run extensions.CapturedRun, err error, entry *extensions.CommandEntry) {
	view := a.OutputFor(run, err, entry)
	a.output = &view
	a.Selected, a.chosenRow = 0, -1
}

// OutputFor builds the view of one captured run: its text (or the list
// protocol's rows), its status, and the command it came from. The launcher
// shows it in its own band; a caller outside can show the same view in a
// window of its own (see the shell's detached window).
func (a *App) OutputFor(run extensions.CapturedRun, err error, entry *extensions.CommandEntry) OutputView {
	copy := a.copy()
	view := OutputView{
		Entry:      entry,
		Title:      strings.Join(run.Command, " "),
		Text:       run.Text(),
		Status:     a.runStatus(copy, run),
		FontFamily: a.settings().FontFamily,
		Selected:   -1,
	}
	// A list is read from standard output alone: standard error beside it is
	// a warning, not part of the protocol.
	if rows, ok := extensions.ParseRows(run.Stdout); ok {
		view.Rows, view.IsList = rows, true
		view.Selected = firstRunnable(rows)
	}
	if !view.IsList {
		if err != nil && view.Text == "" {
			view.Text = err.Error()
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
		parts = append(parts, fmt.Sprintf("%.1fs", run.Duration.Seconds()))
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
			ui.List(c.Key("launcher.output.list"), &a.OutputList, len(view.Rows), func(i int) {
				a.outputRow(c, view.Rows[i], i)
			}).Fill().Padding(edge+header, t.Space(1), 0, 0).Label(view.Title)
		} else {
			ui.Scroll(c.Key("launcher.output")).TrackScroll(&a.Output).Fill().
				Padding(edge+header, t.Space(2), t.Space(1), t.Space(2)).Children(func() {
				ui.Text(c, view.Text).Font(view.FontFamily).FontSize(t.FontSize).
					TextColor(t.Text).Selectable()
			})
		}
		ui.Column(c).Absolute().Top(edge).Left(t.Space(2)).Right(t.Space(2)).Children(func() {
			ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, view.Title).FontSize(t.FontSize).TextColor(t.Text).Ellipsis("\u2026").SingleLine().Grow(1)
				// The output can be shown in a window of its own: the old
				// build's own pin, on the surface that holds the text.
				if a.Actions.PinText != nil {
					if ui.Button(c, "\u2197").Label(copy.HistoryPin).Clicked() {
						a.Actions.PinText(view.Title, view.Text)
					}
				}
				// And a command's page can be detached into a window that
				// stays put: the old build's second window, which never hides
				// on blur and re-runs the command from its own controls.
				if a.Actions.DetachOutput != nil && view.Entry != nil {
					if ui.Button(c, "\u29c9").Label(copy.OutputDetach).Clicked() {
						a.Actions.DetachOutput(*view)
					}
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
