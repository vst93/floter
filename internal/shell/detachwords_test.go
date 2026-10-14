package shell

import (
	"testing"

	"floter/internal/i18n"
)

// The detached window's vocabulary and its task-bar title are the old build's,
// character for character. The React surface set the OS title to
// `${windowTitle} · floter` (`DetachedPluginApp.tsx:120-124`) and drew three
// words from the dictionary (`i18n.ts:46-49,969-976`): the idle line, the
// fallback name, and the re-run control.
//
// This test exists because these strings are exactly the kind of detail a
// rewrite paraphrases without noticing — before this round the Go build said
// "Nothing has run yet" / "floter command" / "Re-run" where the reference says
// "No command pinned yet" / "floter plugin" / "Rerun".
func TestDetachedWindowWordsMatchTheOldDictionary(t *testing.T) {
	cases := []struct {
		lang     string
		idle     string
		fallback string
		rerun    string
	}{
		{"en", "No command pinned yet", "floter plugin", "Rerun"},
		{"zh", "尚未固定任何命令", "floter 插件", "重新执行"},
	}
	for _, tc := range cases {
		copy := i18n.For(tc.lang).Launcher
		if copy.OutputWindowIdle != tc.idle {
			t.Errorf("%s idle = %q, want %q", tc.lang, copy.OutputWindowIdle, tc.idle)
		}
		if copy.OutputWindowFallback != tc.fallback {
			t.Errorf("%s fallback = %q, want %q", tc.lang, copy.OutputWindowFallback, tc.fallback)
		}
		if copy.OutputWindowRerun != tc.rerun {
			t.Errorf("%s rerun = %q, want %q", tc.lang, copy.OutputWindowRerun, tc.rerun)
		}
	}
}

// The window-manager title wears the app tail; the header inside the window
// keeps the bare content label. A pinned window is therefore identifiable in
// the task bar, exactly as the reference made it.
func TestDetachedWindowTitleCarriesTheAppTail(t *testing.T) {
	if got := detachWindowTitle("jv --floter"); got != "jv --floter \u00b7 floter" {
		t.Errorf("window title = %q, want %q", got, "jv --floter \u00b7 floter")
	}
	// The bare label is untouched: the tail is a title-only decoration.
	if got := detachContentKey(nil, "Note"); got != "Note" {
		t.Errorf("content key = %q", got)
	}
}
