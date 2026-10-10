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

// The window's own body draws its title, its text and the two controls that
// belong to it; the re-run control is offered only when there is a command
// behind the run.
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
