package launcher

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
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

	// EmptyResultsAreaHeight is what P0 adds under the input row: the blank
	// result area the P1 list will fill, with the empty-state hint in it.
	// The old shell expanded to a measured height when results arrived; P0
	// opens at the collapsed row plus this fixed area so the layout the
	// round asked for is visible without a window manager that resizes.
	EmptyResultsAreaHeight = 140

	// shellGutter is the shell's padding around the panel (base.css's
	// platform blocks; Linux's is 10 DIPs).
	shellGutter = 10
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

// WindowHeight is the height P0 opens the launcher at: the collapsed input
// window's height at the settings' interface step, plus the empty result
// area. The step scales the input row exactly as the old native path scaled
// its fallback height.
func WindowHeight(uiScale string) float64 {
	return InputWindowHeight()*settings.UIScaleFactor(uiScale) + EmptyResultsAreaHeight
}

// App is the launcher's state: the settings it was opened with and the
// query the field edits.
type App struct {
	Settings settings.Settings
	Query    string
}

// New builds the launcher state from a settings value.
func New(s settings.Settings) *App {
	return &App{Settings: settings.Normalize(s)}
}

// View builds the launcher shell: the search field over the empty result
// area and its hint, on the glass panel. It is the window's content.
func (a *App) View(c *ui.Context) {
	systemDark := c.Theme().Dark
	surface := Resolve(a.Settings, systemDark)

	// Explicit settings win over the desktop's appearance; `auto` leaves the
	// frame's own system-following theme in place.
	switch surface.Theme {
	case ThemeLight:
		c.SetTheme(ui.LightTheme())
	case ThemeDark:
		c.SetTheme(ui.DarkTheme())
	}
	t := c.Theme()

	// The window is transparent: only the panel paints, so the desktop (and
	// the glass that reads it) shows through the shell's gutter.
	c.Root().Background(ui.Transparent)

	ui.Column(c).Fill().Padding(shellGutter).Children(func() {
		panel := ui.Column(c).Fill().Radius(t.Space(3)).Padding(t.Space(3))
		if material := glassmap.Material(surface.Glass, t); material != nil {
			panel.Material(material)
		} else {
			// The `off` stop: a plain, near-solid face and no material.
			panel.Background(t.Surface)
		}
		panel.Children(func() {
			ui.TextInput(c, &a.Query).
				Placeholder(surface.Strings.Placeholder).
				Label(surface.Strings.Label).
				Focus()
			// The empty result area: blank until P1 fills it, with the hint
			// the round asked for.
			ui.Column(c).FillWidth().Grow(1).Center().Children(func() {
				ui.Text(c, surface.Strings.Hint).
					FontSize(t.FontSize).
					TextColor(t.TextMuted)
			})
		})
	})
}
