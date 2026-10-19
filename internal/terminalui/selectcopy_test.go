package terminalui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/settings"
)

// TestSelectCopyReachesThePlugin covers the `terminal_select_copy` setting,
// which shipped with no reader at all: the settings page recorded it and the
// terminal surface never asked, so the switch did nothing.
//
// The reference answered it on every mount and on every change
// (`main:src/App.tsx:875` → the renderer's `selectCopy`; a finished selection
// was copied as it ended). The plugin has the same two stops, so the setting
// maps onto them directly.
func TestSelectCopyReachesThePlugin(t *testing.T) {
	cases := []struct {
		name string
		on   bool
		want terminal.CopyOnSelect
	}{
		{"a finished selection lands on the clipboard", true, terminal.CopyOnSelectClipboard},
		{"the selection waits for copy_selection", false, terminal.CopyOnSelectNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := settings.NewStore(settings.Default())
			if err := store.Update(func(s *settings.Settings) { s.TerminalSelectCopy = tc.on }); err != nil {
				t.Fatal(err)
			}
			newTerminal, options := captureTerminal(t)
			a := New(store, Actions{}, newTerminal)
			a.EnsureSession()
			if a.Err != nil {
				t.Fatalf("the session did not start: %v", a.Err)
			}
			if len(*options) != 1 {
				t.Fatalf("the plugin was built %d times, want 1", len(*options))
			}
			if got := (*options)[0].CopyOnSelect; got != tc.want {
				t.Errorf("CopyOnSelect = %v, want %v", got, tc.want)
			}

			// A change made while the terminal is open: Refresh rebuilds the
			// options from the store and hands them to the plugin's own setters,
			// SetCopyOnSelect among them.
			if err := store.Update(func(s *settings.Settings) { s.TerminalSelectCopy = !tc.on }); err != nil {
				t.Fatal(err)
			}
			a.Refresh()
			want := terminal.CopyOnSelectClipboard
			if tc.on {
				want = terminal.CopyOnSelectNone
			}
			if got := a.options().CopyOnSelect; got != want {
				t.Errorf("after the change CopyOnSelect = %v, want %v", got, want)
			}
		})
	}
}
