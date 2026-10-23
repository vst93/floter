package settingsui

import (
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P9-k: the recorder's refusal line used to be reachable only by floter's own
// conflicts. A chord the system would not take (another app holds it) now lands
// on the same sentence, because the shell reports the refusal back instead of
// leaving it in the log.
func TestTheRecorderSaysWhenTheSystemRefusedTheChord(t *testing.T) {
	store := newStore(t)
	refused := ""
	a := New(store, Actions{
		SetShortcut: func(id, accelerator string) {
			store.Update(func(s *settings.Settings) { s.SetShortcut(id, accelerator) })
		},
		ShortcutRefused: func(id string) bool { return id == refused },
	})
	en := i18n.For("en").Settings

	refused = settings.ShortcutToggleWindow
	a.recordShortcut(settings.ShortcutToggleWindow, "Cmd+Shift+G")
	if got := a.shortcutWarning(settings.ShortcutToggleWindow, en); got != en.ShortcutRejected {
		t.Fatalf("warning = %q, want %q", got, en.ShortcutRejected)
	}
	if a.rejectedShortcut != settings.ShortcutToggleWindow {
		t.Errorf("rejectedShortcut = %q", a.rejectedShortcut)
	}

	// The system takes the next one: the line goes away with it.
	refused = ""
	a.recordShortcut(settings.ShortcutToggleWindow, "Cmd+Shift+H")
	if got := a.shortcutWarning(settings.ShortcutToggleWindow, en); got != "" {
		t.Fatalf("warning = %q after a chord that took", got)
	}
}
