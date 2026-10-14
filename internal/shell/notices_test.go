package shell

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
)

// The refusal gate coalesces one automatic failure per key: the first raise
// passes, the repeats inside the window do not, and the same failure coming
// back after the window is news again. Driven on an injected clock, so a
// burst of attempts needs no wall-clock wait.
func TestRefusalGateCoalescesPerKey(t *testing.T) {
	now := time.Unix(0, 0)
	gate := newRefusalGate(refusalWindow, func() time.Time { return now })
	if !gate.allow("a") {
		t.Fatal("the first raise was refused")
	}
	if gate.allow("a") {
		t.Fatal("a repeat inside the window passed")
	}
	if !gate.allow("b") {
		t.Fatal("another key was refused")
	}
	now = now.Add(refusalWindow - time.Millisecond)
	if gate.allow("a") {
		t.Fatal("a repeat just inside the window passed")
	}
	now = now.Add(time.Millisecond)
	if !gate.allow("a") {
		t.Fatal("a raise after the window was refused")
	}
}

// countTexts is how many labels in a frame contain want: a toast shows one, and
// the point of the window is that a second refusal does not show a second one.
func countTexts(texts []string, want string) int {
	count := 0
	for _, text := range texts {
		if strings.Contains(text, want) {
			count++
		}
	}
	return count
}

// pendingNotices is how many toasts are waiting for a frame.
func pendingNotices(a *App) int {
	a.noticeMu.Lock()
	defer a.noticeMu.Unlock()
	return len(a.notices)
}

// The toast's tone and mark follow the kind its raiser tagged it with: the old
// build's `.app-toast--error` and `--warning` share the amber band (its own note
// on the CSS says the warning "shares the error band's amber label"), the
// `--success` one wears the accent, and each kind leads with its own icon.
func TestToastTonesAndMarksFollowTheirKind(t *testing.T) {
	theme := ui.DarkTheme()
	cases := []struct {
		kind string
		mark string
		want ui.Color
	}{
		{"error", "alert", theme.Warning},
		{"warning", "triangle-alert", theme.Warning},
		{"success", "check", theme.Accent},
		{"", "", theme.Text},
		{"other", "", theme.Text},
	}
	for _, tc := range cases {
		if got := toastTone(theme, tc.kind); got != tc.want {
			t.Errorf("toastTone(%q) = %v, want %v", tc.kind, got, tc.want)
		}
		if got := toastMark(tc.kind); got != tc.mark {
			t.Errorf("toastMark(%q) = %q, want %q", tc.kind, got, tc.mark)
		}
		// A mark must be one the shared vocabulary can actually draw: a name that
		// resolves to nothing would silently fall back to the alert circle.
		if tc.mark != "" {
			if _, ok := launcher.Glyph(tc.mark); !ok {
				t.Errorf("the %q mark %q is not in the glyph set", tc.kind, tc.mark)
			}
		}
	}
}

// The app's own viewport, not the framework's theme one: a raised notice draws
// its sentence, its kind's mark and the dismiss control the old toast had — and
// the control really closes it.
func TestTheToastIsDrawnByTheAppsOwnViewport(t *testing.T) {
	a := newApp(t)
	copy := i18n.For("en").Settings
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()

	a.warnNotice("Toolchain missing")
	tt.Frame()
	if !tt.HasText("Toolchain missing") {
		t.Fatalf("the notice is not drawn: %q", tt.Texts())
	}
	// The dismiss control is the app's own, labeled from the old
	// `settings.extensions.dismissNotice`.
	if !tt.HasText(copy.DismissNotice) {
		t.Fatalf("the toast has no dismiss control: %q", tt.Texts())
	}
	if err := tt.Click(copy.DismissNotice); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	tt.Frame()
	if tt.HasText("Toolchain missing") {
		t.Errorf("the dismiss control did not close the toast: %q", tt.Texts())
	}
}

// A toast carrying an action draws it as the old `.app-toast__action` chip, and
// pressing it runs the caller's closure and closes the toast — a retry must not
// leave the toast behind.
func TestTheToastActionRunsAndClosesTheToast(t *testing.T) {
	a := newApp(t)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()

	ran := 0
	a.noticeMu.Lock()
	a.notices = append(a.notices, ui.Toast{
		ID:       "floter-notice",
		Title:    "Plugin page failed",
		Type:     "error",
		Action:   "Retry",
		OnAction: func() { ran++ },
		Timeout:  refusalNotice,
	})
	a.noticeMu.Unlock()

	tt.Frame()
	if !tt.HasText("Plugin page failed") || !tt.HasText("Retry") {
		t.Fatalf("the toast or its action is not drawn: %q", tt.Texts())
	}
	if err := tt.Click("Retry"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	tt.Frame()
	if ran != 1 {
		t.Errorf("the action ran %d times, want 1", ran)
	}
	if tt.HasText("Plugin page failed") {
		t.Errorf("pressing the action left the toast behind: %q", tt.Texts())
	}
}

// The stack's geometry is the old build's per surface: it hangs from the
// top-right on a full-height page (settings, terminal, a plugin page) and is
// pinned to the bottom-left on the collapsed launcher, whose thin window no
// top placement clears.
func TestTheToastStackAnchorsPerSurface(t *testing.T) {
	a := newApp(t)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()

	a.warnNotice("Where am I")
	tt.Frame()
	rect, ok := tt.Find("Where am I")
	if !ok {
		t.Fatalf("the notice is not drawn: %q", tt.Texts())
	}
	if rect.X+rect.W/2 > 360 || rect.Y+rect.H/2 < 260 {
		t.Fatalf("the launcher notice is not bottom-left: %+v", rect)
	}

	a.Open(SurfaceSettings)
	tt.Frame()
	rect, ok = tt.Find("Where am I")
	if !ok {
		t.Fatalf("the notice did not survive the surface change: %q", tt.Texts())
	}
	if rect.X+rect.W/2 < 360 || rect.Y+rect.H/2 > 260 {
		t.Fatalf("the settings notice is not top-right: %+v", rect)
	}
}

// The collapsed launcher shows only its newest toast — earlier ones stay in
// state but drop out of layout, so the thin window never reserves a second row
// — while a full-height page keeps the whole stack.
func TestTheLauncherToastStackKeepsOnlyTheNewest(t *testing.T) {
	a := newApp(t)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()
	a.noticeMu.Lock()
	a.notices = append(a.notices,
		ui.Toast{ID: "older", Title: "older notice", Timeout: refusalNotice},
		ui.Toast{ID: "newer", Title: "newer notice", Timeout: refusalNotice})
	a.noticeMu.Unlock()
	tt.Frame()
	if tt.HasText("older notice") {
		t.Errorf("the launcher kept a toast behind the newest: %q", tt.Texts())
	}
	if !tt.HasText("newer notice") {
		t.Errorf("the newest toast is not drawn: %q", tt.Texts())
	}

	a.Open(SurfaceSettings)
	tt.Frame()
	if !tt.HasText("older notice") || !tt.HasText("newer notice") {
		t.Errorf("a full-height page dropped a queued toast: %q", tt.Texts())
	}
}

// A refused link says so, where the user is, and once per window: the old
// build raised one refusal toast through its 30s failure deduper, and the
// sentence is the one its `settings.deepLinkRejected` carried.
func TestRefusedLinkSaysSoOnce(t *testing.T) {
	a := newApp(t)
	copy := i18n.For("en").Settings
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()

	// A connect with no manifest at all is refused before the page is touched.
	a.HandleURL("floter://connect")
	if got := pendingNotices(a); got != 1 {
		t.Fatalf("a refusal queued %d notices, want 1", got)
	}
	tt.Frame()
	if !tt.HasText(copy.DeepLinkRejected) {
		t.Fatalf("the refusal is not drawn: %q", tt.Texts())
	}
	if a.Surf != SurfaceLauncher {
		t.Errorf("a refused connect moved to surface %v", a.Surf)
	}

	// The same refusal inside the window is not raised again: one bad link, one
	// sentence. The window, not a de-duplicated id, is what keeps it to one.
	a.HandleURL("floter://register")
	if got := pendingNotices(a); got != 0 {
		t.Errorf("a repeat refusal queued %d notices, want 0", got)
	}
	tt.Frame()
	if got := countTexts(tt.Texts(), copy.DeepLinkRejected); got != 1 {
		t.Errorf("refusal labels = %d, want 1 (%q)", got, tt.Texts())
	}

	// A malformed URL is the same class of refusal, and shares the window.
	a.HandleURL("floter://%zz")
	if got := pendingNotices(a); got != 0 {
		t.Errorf("a malformed URL inside the window queued %d notices", got)
	}
	tt.Frame()
	if got := countTexts(tt.Texts(), copy.DeepLinkRejected); got != 1 {
		t.Errorf("after a malformed URL, refusal labels = %d, want 1 (%q)", got, tt.Texts())
	}
}

// The things the old build stayed quiet about stay quiet here: a scheme this
// app does not handle, and an action it does not know. A refusal toast for a
// stranger's bug is noise the user cannot act on.
func TestUnhandledLinksStayQuiet(t *testing.T) {
	a := newApp(t)
	copy := i18n.For("en").Settings
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()

	a.HandleURL("https://example.com/")
	a.HandleURL("floter://bogus")
	tt.Frame()
	if tt.HasText(copy.DeepLinkRejected) {
		t.Errorf("an unhandled link raised a refusal: %q", tt.Texts())
	}
}

// A connect whose manifest is not there is refused after the review starts,
// off the main thread: the user still gets the sentence, on the surface the
// link opened.
func TestConnectThatFailedSaysSo(t *testing.T) {
	a := newApp(t)
	copy := i18n.For("en").Settings
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()

	a.HandleURL("floter://connect?manifest=" + filepath.Join(t.TempDir(), "missing.json"))
	deadline := time.Now().Add(5 * time.Second)
	for pendingNotices(a) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the refusal never arrived")
		}
		time.Sleep(5 * time.Millisecond)
	}
	tt.Frame()
	if !tt.HasText(copy.DeepLinkRejected) {
		t.Errorf("the refusal is not drawn: %q", tt.Texts())
	}
}

// The advisory notice is the old build's warning toast: it shows where the
// user is — the extension form raises it from the settings page, not from the
// launcher's docked row — and it lives the warning window, one step shorter
// than the error's because it says what will not happen yet rather than what
// failed.
func TestWarningNoticeShowsWhereTheUserIs(t *testing.T) {
	a := newApp(t)
	sentence := i18n.For("en").Settings.LocalToolRuntimeSaveWarning("Python")
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()
	a.Open(SurfaceSettings)
	tt.Frame()

	a.warnNotice(sentence)
	a.noticeMu.Lock()
	queued := a.notices
	a.noticeMu.Unlock()
	if len(queued) != 1 || queued[0].Timeout != warningNotice {
		t.Fatalf("the warning queued %+v, want one with the warning window %v", queued, warningNotice)
	}
	tt.Frame()
	if !tt.HasText(sentence) {
		t.Fatalf("the warning is not drawn: %q", tt.Texts())
	}
	if a.Surf != SurfaceSettings {
		t.Errorf("a warning moved to surface %v", a.Surf)
	}
	// Nothing to say is not a toast.
	a.warnNotice("")
	if got := pendingNotices(a); got != 0 {
		t.Errorf("an empty warning queued %d notices", got)
	}
}
