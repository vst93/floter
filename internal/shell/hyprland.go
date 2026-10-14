package shell

import (
	"runtime"

	"floter/internal/settings"
	"floter/internal/spawn"
)

// Hyprland (Wayland) tiles a new toplevel, so the launcher comes up as a
// full-width tiled window rather than the centred floating panel it is
// everywhere else. Tauri's own geometry calls cannot change that — the
// compositor owns tiling — so the one app-side lever is `hyprctl dispatch`,
// which converts the active window to a floating one. The reveal runs these
// dispatches after the window is shown and focused, so the launcher is the
// active window, and the panel keeps its normal size and position.
//
// Everything here is best-effort: a machine without `hyprctl` on PATH, a
// compositor that renamed its socket, or a focus race all degrade to the
// pre-Hyprland behaviour rather than breaking the reveal. Only Hyprland is
// special-cased; other tiling compositors (sway, niri, …) have different
// models and are deliberately left alone.

// hyprlandFloatingDispatches are the dispatches that float and centre the
// active window, in order. `setfloating` is a *set* (not a toggle), so a
// reveal that lands on an already floating window is a no-op rather than a
// flip back to tiling; `togglefloating` would do the opposite and must not be
// used.
var hyprlandFloatingDispatches = [][]string{
	{"dispatch", "setfloating"},
	{"dispatch", "centerwindow"},
}

// ensureHyprlandFloating floats and centres the active window when the current
// session is Hyprland, reporting whether it ran. A no-op on every other
// platform and compositor.
func (a *App) ensureHyprlandFloating() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	detected := settings.HyprlandSession()
	runHyprlandFloating(detected, spawn.Program)
	return detected
}

// runHyprlandFloating runs the dispatches through `run` in order, or nothing
// when the session is not Hyprland. `run` is injected so the order and the
// set-not-toggle rule are a unit test rather than a live compositor.
func runHyprlandFloating(detected bool, run func(string, ...string) error) {
	if !detected {
		return
	}
	for _, args := range hyprlandFloatingDispatches {
		_ = run("hyprctl", args...)
	}
}
