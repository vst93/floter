package launcher

import (
	"runtime"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/extensions"
	"floter/internal/settings"
)

// The launcher's window geometry. The width and the collapsed height are the
// old shell's contract — `INPUT_WINDOW_WIDTH` and `INPUT_WINDOW_HEIGHT` in
// src-tauri/src/lib.rs — and the collapsed height is a scale-1 measurement
// multiplied by the interface-size factor, exactly as
// `scaled_input_window_height` did.
const (
	// InputWindowWidth is the launcher's fixed width, in DIPs.
	InputWindowWidth = 720

	// The collapsed launcher's height at scale 1, per platform: 78 on Linux
	// (a 58px card in the shell's 10px gutters), 72 on Windows (a 56px card
	// in the 4px + 12px gutters) and 58 elsewhere (the card alone).
	inputWindowHeightLinux   = 78
	inputWindowHeightWindows = 72
	inputWindowHeightDefault = 58

	// ResultsAreaHeight is the room under the input row for the result list:
	// about four rows at the shipped interface size. The old shell grew the
	// window to a measured band as results arrived; P1 opens at the collapsed
	// row plus this fixed band so typing never resizes the window.
	ResultsAreaHeight = 240
)

// InputWindowHeight is the collapsed launcher window's height at scale 1.
func InputWindowHeight() float64 {
	switch runtime.GOOS {
	case "windows":
		return inputWindowHeightWindows
	case "linux":
		return inputWindowHeightLinux
	default:
		return inputWindowHeightDefault
	}
}

// WindowHeight is the height the launcher opens at: the collapsed input
// window's height at the settings' interface step, plus the result band. The
// step scales the input row exactly as the old native path scaled its
// fallback height.
func WindowHeight(uiScale string) float64 {
	return InputWindowHeight()*settings.UIScaleFactor(uiScale) + ResultsAreaHeight
}

// Actions are what a result can do, supplied by the shell: the launcher does
// not know how surfaces are opened or how an application launches, only
// that the user asked for one.
type Actions struct {
	// OpenSettings, OpenTerminal and Quit run the built-in commands.
	OpenSettings func()
	OpenTerminal func()
	Quit         func()
	// Dismiss is Escape with an empty query: hide the launcher window, as
	// the old shell did.
	Dismiss func()
	// Copy puts text on the clipboard, for the calculator row.
	Copy func(text string)
	// OpenApp launches an installed application.
	OpenApp func(app apps.App)
	// RunCommand runs an extension's command.
	RunCommand func(entry extensions.CommandEntry)
}

// App is the launcher surface's state: the settings store it reads, the
// query the field edits, the chosen result, and the handles the framework
// needs to keep the field focused across builds.
type App struct {
	Store   *settings.Store
	Actions Actions

	Query    string
	Selected int

	// Apps is the installed applications the shell scanned, and Commands the
	// enabled extensions' commands; both join the results once the user
	// types.
	Apps     []apps.App
	Commands []extensions.CommandEntry

	// Scroll keeps the result list's place; Search is the field's identity,
	// for the focus the launcher keeps on it while the surface shows.
	List   ui.ListState
	Search ui.Handle

	// chosenRow tracks the row the keyboard last moved to, so the list is
	// scrolled only when the choice changed.
	chosenRow int

	// toast is a message to show on the next frame, set by an action.
	toast string
}

// SetApps replaces the scanned applications.
func (a *App) SetApps(found []apps.App) { a.Apps = found }

// SetCommands replaces the extensions' commands.
func (a *App) SetCommands(found []extensions.CommandEntry) { a.Commands = found }

// New builds the launcher state over a settings store and the shell's
// actions.
func New(store *settings.Store, actions Actions) *App {
	return &App{Store: store, Actions: actions}
}

// settings is the current normalized settings.
func (a *App) settings() settings.Settings { return a.Store.Snapshot() }

// FocusSearch asks for the keyboard focus on the query field. The launcher
// calls it every frame: the field owns the keyboard while the surface shows,
// exactly as the old collapsed shell reasserted focus.
func (a *App) FocusSearch() { a.Search.Focus() }

// View builds the launcher: a search field pinned over the result list,
// which scrolls under it behind a soft scroll edge, as the old shell's
// results did. The shell draws the window chrome and the glass panel; this
// is the content inside it.
func (a *App) View(c *ui.Context) {
	copy := StringsFor(a.settings().Language)
	t := c.Theme()

	// The field owns the keyboard while the launcher shows, exactly as the
	// old collapsed shell reasserted focus on every reveal.
	a.FocusSearch()

	results := a.Results()
	a.clampSelection(len(results))

	// The field row floats over the list; the list's top padding is the
	// row plus the edge, so a row scrolls under the field rather than to
	// its edge.
	fieldRow := t.Space(7)
	edge := fieldRow + t.Space(2)

	ui.Box(c).Fill().Children(func() {
		if len(results) == 0 {
			ui.Column(c).FillWidth().Padding(t.Space(3)).Center().Children(func() {
				ui.Text(c, copy.NoResults).FontSize(t.FontSize).TextColor(t.TextMuted)
			})
		} else {
			// The list builds only the rows in view, so a catalog of
			// hundreds costs a frame like a catalog of three. The choice is
			// painted by the row itself (see row), not by the list's own
			// selection. Padding puts the first row below the field, and
			// scrolls with the content, so rows pass under the field.
			ui.List(c.Key("launcher.results"), &a.List, len(results), func(i int) {
				a.row(c, results[i], i)
			}).Fill().Padding(edge, 0, 0, 0).Label(copy.ResultsLabel)
		}

		// The rows fade into the panel under the field. PassThrough lets
		// the pointer reach a row the strip covers.
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(edge).PassThrough().
			Material(glass.ScrollEdge{Background: t.Background})

		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(fieldRow).
			AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			field := ui.TextInput(c.Key("launcher.search"), &a.Query).
				Bind(&a.Search).
				Label(copy.Label).
				Placeholder(copy.Placeholder).
				Grow(1)
			if field.Changed() {
				a.Selected = 0
			}
			if field.Submitted() {
				a.activate(results)
			}
			if a.Query != "" {
				if ui.Button(c, "✕").Label(copy.Clear).Clicked() {
					a.Query = ""
					a.Selected = 0
					a.FocusSearch()
				}
			} else {
				ui.Text(c, copy.Hint).FontSize(t.FontSize).TextColor(t.TextMuted)
			}
		})
	})

	// Keep the chosen row in view when the keyboard moved the choice (the
	// list leaves the scrolling to the app when it has no selection).
	if a.Selected != a.chosenRow {
		a.chosenRow = a.Selected
		a.List.ScrollIntoView(a.Selected)
	}

	// An action that asks for feedback (the calculator's copy) shows it
	// once, on the frame after it ran.
	if a.toast != "" {
		c.Toast(a.toast)
		a.toast = ""
	}

	// The field keeps the focus, so the list's arrows are read here: a
	// single-line text input leaves plain Up and Down to shortcuts.
	if c.Shortcut(0, ui.KeyDown) {
		a.move(1, len(results))
	}
	if c.Shortcut(0, ui.KeyUp) {
		a.move(-1, len(results))
	}
	if c.Shortcut(0, ui.KeyEscape) {
		switch {
		case a.Query != "":
			a.Query = ""
			a.Selected = 0
		case a.Actions.Dismiss != nil:
			a.Actions.Dismiss()
		}
	}
}

// row builds one result: title, detail and an optional shortcut. The chosen
// row wears the accent as a tint, as the old launcher's rows did.
func (a *App) row(c *ui.Context, item Item, i int) {
	t := c.Theme()
	row := ui.Row(c).FillWidth().Focusable().Padding(t.Space(1.5), t.Space(2)).Radius(t.Radius).Gap(t.Space(2))
	if i == a.Selected {
		row.Background(t.Accent.Alpha(0.14))
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, item.Title).FontSize(t.FontSize).TextColor(t.Text)
			if item.Detail != "" {
				ui.Text(c, item.Detail).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			}
		})
		if item.Shortcut != "" {
			ui.Text(c, item.Shortcut).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
	})
	if row.Clicked() {
		a.Selected = i
		a.activate(a.Results())
	}
}

// activate runs the result at the current selection.
func (a *App) activate(results []Item) {
	if a.Selected < 0 || a.Selected >= len(results) {
		return
	}
	if run := results[a.Selected].Run; run != nil {
		run()
	}
}

// move moves the selection by delta, wrapping around the list.
func (a *App) move(delta, n int) {
	a.Selected = NextIndex(a.Selected, delta, n)
}

func (a *App) clampSelection(n int) {
	a.Selected = ClampIndex(a.Selected, n)
}
