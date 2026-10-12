package shell

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
)

// A detached output window is the panel's opposite: it does not follow the
// panel's summon and hide, it keeps the command it was opened for, and its own
// header re-runs that command. The window itself is injected here — a real one
// needs a display — so the state the shell builds is what the test reads.
func TestDetachedOutputWindow(t *testing.T) {
	opened := []launcher.OutputView{}
	a := New(Options{
		Store: settings.NewStore(settings.Default()),
		OpenDetached: func(view launcher.OutputView) *mygo.Window {
			opened = append(opened, view)
			return nil
		},
	})
	entry := &extensions.CommandEntry{Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}}
	a.detachOutput(launcher.OutputView{
		Title:  "jv --floter",
		Text:   "{\"a\": 1}",
		Status: "ok",
		Entry:  entry,
	})
	if len(opened) != 1 {
		t.Fatalf("opened = %d windows", len(opened))
	}
	if len(a.detached) != 1 {
		t.Fatalf("the window is not held: %d", len(a.detached))
	}
	detached := a.detached[0]
	if detached.title != "jv --floter" {
		t.Errorf("title = %q", detached.title)
	}
	if detached.entry == nil || detached.entry.Command.ID != "jv" {
		t.Errorf("the window lost its command: %+v", detached.entry)
	}
	if detached.view.Text != "{\"a\": 1}" {
		t.Errorf("the window lost its text: %q", detached.view.Text)
	}

	// A run with no title of its own names the window with the fallback, so a
	// window is never nameless in the task bar.
	a.detachOutput(launcher.OutputView{Text: "x"})
	copy := i18n.For("en").Launcher
	if got := a.detached[1].title; got != copy.OutputWindowFallback {
		t.Errorf("fallback title = %q", got)
	}
	// Closing the app closes them: a window that outlived its app would be a
	// process nobody can reach.
	a.closeDetached()
	if len(a.detached) != 0 {
		t.Errorf("windows survived the quit: %d", len(a.detached))
	}
}

// R91 · a window belongs to its *content*, not to the press that opened it:
// pinning the same command again replaces the window already showing it (the
// R84-R90 rule, kept for the case it was written for) instead of opening a
// twin, while a *different* thing opens another window.
func TestDetachReusesTheWindowForTheSameContent(t *testing.T) {
	opened := []launcher.OutputView{}
	a := New(Options{
		Store: settings.NewStore(settings.Default()),
		OpenDetached: func(view launcher.OutputView) *mygo.Window {
			opened = append(opened, view)
			return nil
		},
	})
	entry := &extensions.CommandEntry{
		IntegrationID: "local.tool",
		Command:       extensions.Command{ID: "jv", Name: "JSON Viewer"},
	}

	// First pin: a window of its own.
	a.detachOutput(launcher.OutputView{Title: "jv --a", Text: "one", Entry: entry})
	if len(opened) != 1 || len(a.detached) != 1 {
		t.Fatalf("first pin opened %d windows, holds %d", len(opened), len(a.detached))
	}

	// The same command, different argv and a fresh run: the window it already
	// has, whose content is replaced — not a twin.
	staleGeneration := a.detached[0].generation
	a.detachOutput(launcher.OutputView{Title: "jv --b", Text: "two", Entry: entry})
	if len(opened) != 1 {
		t.Fatalf("pinning the same command opened a twin: %d windows", len(opened))
	}
	if len(a.detached) != 1 {
		t.Fatalf("the same content is held twice: %d", len(a.detached))
	}
	if got := a.detached[0]; got.title != "jv --b" || got.view.Text != "two" {
		t.Errorf("the window kept the old run: title %q text %q", got.title, got.view.Text)
	}
	// Replacing the content bumps the generation, so a run the window started
	// for the old pin cannot land on top of the new one.
	if a.detached[0].generation == staleGeneration {
		t.Errorf("replacing the content did not age the run in flight")
	}

	// A run with no command is keyed by its own title: the same title reuses,
	// a different one opens its own window.
	a.detachOutput(launcher.OutputView{Title: "Note", Text: "x"})
	a.detachOutput(launcher.OutputView{Title: "Note", Text: "y"})
	if len(opened) != 2 || len(a.detached) != 2 {
		t.Fatalf("the same note opened a twin: %d windows", len(opened))
	}
	a.detachOutput(launcher.OutputView{Title: "Other", Text: "z"})
	if len(opened) != 3 || len(a.detached) != 3 {
		t.Fatalf("a different note did not open its own window: %d", len(opened))
	}
	if got := a.detached[2]; got.title != "Other" || got.view.Text != "z" {
		t.Errorf("the third window is not the other note: title %q text %q", got.title, got.view.Text)
	}
}

// The content identity is the R91 `content_key`: a command by its integration
// and its own id (never its label or argv), a command-less run by its trimmed
// title. The NUL separator keeps the split unambiguous.
func TestDetachContentKey(t *testing.T) {
	jv := &extensions.CommandEntry{
		IntegrationID: "local.tool",
		Command:       extensions.Command{ID: "jv", Name: "JSON Viewer"},
	}
	// The command, not the label: a renamed title (a different argv) is the
	// same window.
	if detachContentKey(jv, "jv --a") != detachContentKey(jv, "renamed") {
		t.Errorf("the key followed the title, not the command")
	}
	// A different command is a different window.
	other := &extensions.CommandEntry{IntegrationID: "local.tool", Command: extensions.Command{ID: "other"}}
	if detachContentKey(jv, "x") == detachContentKey(other, "x") {
		t.Errorf("two commands share one key")
	}
	// The NUL separator disambiguates the integration/id split.
	abC := &extensions.CommandEntry{IntegrationID: "ab", Command: extensions.Command{ID: "c"}}
	aBc := &extensions.CommandEntry{IntegrationID: "a", Command: extensions.Command{ID: "bc"}}
	if detachContentKey(abC, "x") == detachContentKey(aBc, "x") {
		t.Errorf("ab + c collided with a + bc")
	}
	// A command-less run is its trimmed title.
	if got := detachContentKey(nil, " Note "); got != "Note" {
		t.Errorf("text key = %q, want %q", got, "Note")
	}
	// The remembered geometry's key is derived from the content and distinct
	// per content.
	if detachStateKey("a") == detachStateKey("b") {
		t.Errorf("two contents share one geometry key")
	}
	if detachStateKey("Note") != detachStateKey(detachContentKey(nil, "Note")) {
		t.Errorf("the geometry key does not follow the content key")
	}
}

// The newest run wins: a run started for content the user has since replaced
// does not overwrite the newer pin (R91's "newest run replaces the old one").
func TestAStaleDetachedRunDoesNotOverwriteNewerContent(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	detached := &detachedOutput{title: "jv", running: true}
	stale := detached.generation

	// The user pins the same command again while the old run is in flight: the
	// window's content is replaced and the in-flight run is now stale.
	detached.generation++
	detached.view = launcher.OutputView{Text: "new"}
	a.applyDetachedRun(detached, stale, launcher.OutputView{Text: "old"})
	if detached.view.Text != "new" {
		t.Errorf("the stale run overwrote the newer pin: %q", detached.view.Text)
	}

	// A run started for the content the window holds now still lands.
	a.applyDetachedRun(detached, detached.generation, launcher.OutputView{Text: "fresh"})
	if detached.view.Text != "fresh" {
		t.Errorf("the current run did not land: %q", detached.view.Text)
	}
	if detached.running {
		t.Errorf("a landed run left the window marked as running")
	}
}

func TestDetachedWindowBody(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	copy := i18n.For("en").Launcher
	detached := &detachedOutput{
		title: "jv --floter",
		view:  launcher.OutputView{Title: "jv --floter", Text: "{\"a\": 1}", Status: "ok"},
	}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	for _, want := range []string{"jv --floter", "{\"a\": 1}", copy.OutputWindowClose} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	// Nothing to re-run: the control is not drawn.
	if tt.HasText(copy.OutputWindowRerun) {
		t.Errorf("a window with no command offered a re-run: %q", tt.Texts())
	}
	// With a command behind it, the control is there, and before any run the
	// window says so rather than showing an empty box.
	detached.entry = &extensions.CommandEntry{Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}}
	detached.view = launcher.OutputView{}
	tt = ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	if !tt.HasText(copy.OutputWindowRerun) {
		t.Errorf("the re-run control is missing: %q", tt.Texts())
	}
	if !tt.HasText(copy.OutputWindowIdle) {
		t.Errorf("an unrun window does not say so: %q", tt.Texts())
	}
}
