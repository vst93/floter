package shell

import "testing"

// The reference's Wayland rule (`main:src-tauri/src/lib.rs:504-546`): on a
// Wayland session a "lock the panel's size" request is answered with "stay
// unlocked", because GTK implements the lock by pinning the window's min and
// max size hints to its current size and the compositor then clamps every
// resize — the app's own included. The launcher is sized from its content on
// every keystroke, so a locked panel there is a window whose height never
// follows the list again.
//
// `env` is the pair of variables the reference reads, so this drives the rule
// without a live desktop. Mutation check: drop the Wayland branch from
// `panelResizable` and every `want: true` row comes back false (the launcher
// and the settings panel would be built locked on GNOME's and KDE's Wayland).
func TestAWaylandSessionKeepsThePanelUnlocked(t *testing.T) {
	wayland := map[string]string{"WAYLAND_DISPLAY": "wayland-1"}
	waylandType := map[string]string{"XDG_SESSION_TYPE": "wayland"}
	waylandTypeScreaming := map[string]string{"XDG_SESSION_TYPE": "Wayland"}
	x11 := map[string]string{"XDG_SESSION_TYPE": "x11", "DISPLAY": ":0"}
	tty := map[string]string{}

	for _, tc := range []struct {
		name  string
		goos  string
		env   map[string]string
		want  bool
		notes string
	}{
		{"linux wayland socket", "linux", wayland, true, "the reference's first signal"},
		{"linux wayland session type", "linux", waylandType, true, "the reference's second signal"},
		{"linux wayland session type, other casing", "linux", waylandTypeScreaming, true, "EqualFold, as lib.rs:506"},
		{"linux x11", "linux", x11, false, "the lock is honoured there"},
		{"linux with no session at all", "linux", tty, false, "a console is not Wayland"},
		{"windows", "windows", wayland, false, "the env is not a session on Windows"},
		{"darwin", "darwin", wayland, false, "and not one on macOS"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string { return tc.env[key] }
			if got := panelResizable(false, tc.goos, getenv); got != tc.want {
				t.Errorf("panelResizable(false, %s, %v) = %v, want %v (%s)",
					tc.goos, tc.env, got, tc.want, tc.notes)
			}
		})
	}
}

// The rule can only ever add the unlock: the terminal asks for a resizable
// window on every platform, and it must keep it — Wayland session or not.
func TestTheTerminalKeepsItsResizeEverywhere(t *testing.T) {
	for _, tc := range []struct {
		goos string
		env  map[string]string
	}{
		{"linux", map[string]string{"WAYLAND_DISPLAY": "wayland-1"}},
		{"linux", map[string]string{"XDG_SESSION_TYPE": "x11"}},
		{"windows", nil},
		{"darwin", nil},
	} {
		getenv := func(key string) string { return tc.env[key] }
		if !panelResizable(true, tc.goos, getenv) {
			t.Errorf("panelResizable(true, %s, %v) = false; the terminal must stay resizable", tc.goos, tc.env)
		}
	}
}

// The session test is the one the two callers share, so it has to read the two
// signals the same way the reference does: a socket that is set but empty is
// not a session, and a session type of `x11` is not one either.
func TestTheSessionTestReadsTheReferencesTwoSignals(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"a socket", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, true},
		{"an empty socket", map[string]string{"WAYLAND_DISPLAY": ""}, false},
		{"a session type", map[string]string{"XDG_SESSION_TYPE": "wayland"}, true},
		{"an X11 session", map[string]string{"XDG_SESSION_TYPE": "x11"}, false},
		{"a tty session", map[string]string{"XDG_SESSION_TYPE": "tty"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string { return tc.env[key] }
			if got := waylandSessionEnv("linux", getenv); got != tc.want {
				t.Errorf("waylandSessionEnv(%v) = %v, want %v", tc.env, got, tc.want)
			}
			// Only Linux has sessions of this shape.
			if waylandSessionEnv("windows", getenv) {
				t.Errorf("waylandSessionEnv read a Wayland session on Windows (%v)", tc.env)
			}
		})
	}
}
