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

// OutputView is one captured run, ready to show.
type OutputView struct {
	// Title is the command line that ran.
	Title string
	// Text is what it wrote (standard output, with standard error appended
	// when there is any).
	Text string
	// Status is the one-line summary: the exit code, the duration, and
	// whether the output was cut or the run timed out.
	Status string
	// FontFamily is the family the text is drawn in: the terminal's own, so
	// a command's output reads the way it would in the terminal.
	FontFamily string
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

// showOutput puts a captured run's output in front of the user.
func (a *App) showOutput(run extensions.CapturedRun, err error) {
	copy := a.copy()
	view := &OutputView{
		Title:      strings.Join(run.Command, " "),
		Text:       run.Text(),
		Status:     a.runStatus(copy, run),
		FontFamily: a.settings().FontFamily,
	}
	if err != nil && view.Text == "" {
		view.Text = err.Error()
	}
	if view.Text == "" {
		view.Text = copy.OutputEmpty
	}
	a.output = view
	a.Selected, a.chosenRow = 0, -1
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
// status line pinned under the field, and the text scrolling under it.
func (a *App) outputBody(c *ui.Context, copy i18n.Launcher, edge float32) {
	view := a.output
	t := c.Theme()
	header := t.Space(4)
	ui.Column(c).Fill().Children(func() {
		ui.Scroll(c.Key("launcher.output")).TrackScroll(&a.Output).Fill().
			Padding(edge+header, t.Space(2), t.Space(1), t.Space(2)).Children(func() {
			ui.Text(c, view.Text).Font(view.FontFamily).FontSize(t.FontSize).
				TextColor(t.Text).Selectable()
		})
		ui.Column(c).Absolute().Top(edge).Left(t.Space(2)).Right(t.Space(2)).Children(func() {
			ui.Text(c, view.Title).FontSize(t.FontSize).TextColor(t.Text).Ellipsis("\u2026").SingleLine()
			ui.Text(c, view.Status+"  \u00b7  "+copy.OutputHint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		})
	})
}
