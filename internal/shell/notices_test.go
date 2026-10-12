package shell

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
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
