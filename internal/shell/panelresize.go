package shell

import "strings"

// A Wayland session cannot lock a panel's size the way the other platforms can.
//
// On X11, Windows and macOS "the user may not resize this" and "the app may
// resize this" are two different things: a non-resizable window still accepts
// the program's own `SetSize`, which is what the panel lives on — the launcher
// is sized from its content on every keystroke (the window's height walk) and
// the settings panel resizes itself as it opens. GTK on Wayland has one lever
// for both: `gtk_window_set_resizable(false)` pins the window's min and max
// size hints to the size it happens to carry, and the compositor then clamps
// *every* resize into that box — the app's own included. A locked panel on
// GNOME's or KDE's Wayland is therefore a window whose height never follows its
// content again: the result list grows underneath a fixed surface, every page
// switch keeps the previous page's height, and the leftover transparent surface
// keeps answering clicks the user aimed at the desktop beside the card.
//
// The reference meets the same wall and answers it the same way
// (`main:src-tauri/src/lib.rs:504-546`, `set_panel_resizable`): on Wayland a
// lock request is answered with "stay unlocked". Nothing is given up — an
// undecorated Wayland toplevel has no frame the compositor lets the user grab,
// so the flag buys no user-resizability there in the first place.
//
// The wlroots compositors (Hyprland, sway, niri) take the other branch: mygo
// builds the panel as a `wlr-layer-shell` surface there (`initLayer`), and a
// layer surface is sized by the app alone, so the flag is inert. The plain
// toplevel — GNOME and KDE on Wayland, where `gtk-layer-shell` is not
// available — is exactly the case that needs this, which is why the rule is
// keyed on the session and not on the compositor.
//
// The lock is what the *launcher* and the *settings* panel ask for; the
// terminal asks for the opposite on every platform (`resize` turns it on per
// surface). This rule can only ever add the unlock, never take it away.

// waylandSessionEnv reports whether the environment describes a Wayland
// session, through the reference's two signals: a Wayland socket, or a session
// type of `wayland` (compared case-insensitively, as `lib.rs:505-506` does).
// `goos` and `getenv` are parameters so the rule is a unit test rather than a
// live desktop.
func waylandSessionEnv(goos string, getenv func(string) string) bool {
	if goos != "linux" {
		return false
	}
	if getenv("WAYLAND_DISPLAY") != "" {
		return true
	}
	return strings.EqualFold(getenv("XDG_SESSION_TYPE"), "wayland")
}

// panelResizable answers what the window's user-resize flag is allowed to be:
// the surface's own answer everywhere, and "yes" on a Wayland session, where a
// "no" would clamp the app's own resizes too (see the file comment). The
// terminal's `true` comes back unchanged on every platform.
func panelResizable(want bool, goos string, getenv func(string) string) bool {
	if waylandSessionEnv(goos, getenv) {
		return true
	}
	return want
}
