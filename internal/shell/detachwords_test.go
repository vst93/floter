package shell

import (
	"testing"

	"floter/internal/i18n"
)

// The detached window's vocabulary and its task-bar title are the old build's,
// character for character. The React surface set the OS title to
// `${windowTitle} · floter` (`DetachedPluginApp.tsx:120-124`) and drew the
// dictionary's own words: the idle line, the fallback name, the re-run control,
// and the three body states (`launcher.externalRunning` / `externalFailed` /
// `externalEmpty`, `i18n.ts:37-39,965-967`).
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
		running  string
		failed   string
		empty    string
	}{
		{"en", "No command pinned yet", "floter plugin", "Rerun", "Running\u2026", "The command could not run", "The command produced no output"},
		{"zh", "尚未固定任何命令", "floter 插件", "重新执行", "正在执行…", "命令无法执行", "命令没有输出"},
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
		if copy.OutputWindowRunning != tc.running {
			t.Errorf("%s running = %q, want %q", tc.lang, copy.OutputWindowRunning, tc.running)
		}
		if copy.OutputWindowFailed != tc.failed {
			t.Errorf("%s failed = %q, want %q", tc.lang, copy.OutputWindowFailed, tc.failed)
		}
		if copy.OutputWindowEmpty != tc.empty {
			t.Errorf("%s empty = %q, want %q", tc.lang, copy.OutputWindowEmpty, tc.empty)
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
