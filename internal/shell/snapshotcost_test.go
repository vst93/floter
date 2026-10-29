package shell

import (
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/settingsui"
	"floter/internal/terminalui"
)

// P11-cq: the shell's frame reads one settings value, and its per-session
// closure reads none at all.
//
// The rule P11-co wrote for the other three surfaces — "a frame copies the
// settings once, however many rows read them" — had never reached the shell.
// `Store.Snapshot` is a deep copy of the whole value, the carried-through keys
// included, and the shell was taking one per read: `View`'s own appearance
// resolve, `toastViewport`'s tokens and words, `appShortcuts`' bindings, and —
// the one that grew with the list — `Settings.Sessions`, which asked
// `terminalui.LabelOf` and `terminalui.NoteOf` per session and each of those
// snapshotted on its own. Measured with sixteen sessions (the page's own
// `maxSessions` ceiling): 36 snapshots for one Sessions frame, of which 32 were
// the closure, i.e. two per session.
//
// The pins below read `Store.Snapshots()` — the store's own copy counter — and
// not a memo's `Builds()`. That is the lesson P11-co paid for: `Builds()`
// stays green when someone routes a read around the memo, and the whole point
// of this round is the reads that were doing exactly that.

// sessionsPage is a shell showing the sessions page, with n sessions in it.
func sessionsPage(t *testing.T, n int) (*App, *settings.Store) {
	t.Helper()
	store := settings.NewStore(settings.Default())
	a := New(Options{Store: store})
	for i := 0; i < n; i++ {
		a.Terminal.Sessions = append(a.Terminal.Sessions, &terminalui.Session{
			Term: &terminal.Terminal{}, Created: time.Now(),
		})
	}
	a.Terminal.Active = 0
	a.Surf = SurfaceSettings
	a.Settings.Page = settingsui.PageSessions
	return a, store
}

// settledFrame draws one frame twice and reports the snapshots the second one
// took: the first frame is the memo's miss (it builds the value), and the rule
// is about every frame after it. A frame that never settles is its own bug,
// which is why the helper draws rather than reading a counter off the store
// before the frame has run.
func settledFrame(t *testing.T, a *App, store *settings.Store) uint64 {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
	tt.Frame()
	before := store.Snapshots()
	tt.Frame()
	return store.Snapshots() - before
}

// The Sessions page's frame takes no settings copy at all once the memo holds
// the value: every reader in it goes through the memo, and the per-session
// closure asks the terminal surface's memo rather than the store. Sixteen
// sessions — the ceiling `trimSessions` enforces — is the case that used to
// cost the most, so it is the one pinned.
func TestASessionsFrameTakesNoSettingsCopy(t *testing.T) {
	a, store := sessionsPage(t, 16)
	if got := settledFrame(t, a, store); got != 0 {
		t.Fatalf("a 16-session frame took %d settings copies, want 0: "+
			"the sessions closure reads LabelOf/NoteOf per session, and each of "+
			"those used to snapshot the store on its own (P11-cq)", got)
	}
}

// The number must not grow with the list: one session and sixteen cost the
// same. A rule that only holds at one size is a rule about that size.
func TestTheSettingsCopiesDoNotGrowWithTheSessionList(t *testing.T) {
	_, store := sessionsPage(t, 0)
	a0, _ := sessionsPage(t, 0)
	small := settledFrame(t, a0, store)

	a16, store16 := sessionsPage(t, 16)
	large := settledFrame(t, a16, store16)
	if small != large {
		t.Fatalf("0 sessions cost %d copies and 16 cost %d: the per-session "+
			"closure still snapshots per session (P11-cq)", small, large)
	}
}

// `Settings.Sessions()` is the closure on its own, called outside any frame:
// it answers every session's four facts and takes no copy of the settings.
func TestTheSessionsClosureTakesNoSettingsCopy(t *testing.T) {
	a, store := sessionsPage(t, 16)
	_ = a.Settings.Sessions() // warm both memos
	before := store.Snapshots()
	_ = a.Settings.Sessions()
	if got := store.Snapshots() - before; got != 0 {
		t.Fatalf("one Settings.Sessions() call with 16 sessions took %d settings "+
			"copies, want 0 (P11-cq)", got)
	}
}

// Every surface's frame is the same rule, because the memo is one field on the
// shell: the launcher, the terminal, and each settings page. The Integrations
// page is the one that also rebuilds its list per frame, so it is where a
// read that bypassed the memo would hide.
func TestEverySurfacesFrameTakesNoSettingsCopy(t *testing.T) {
	for _, surface := range []struct {
		name string
		surf Surface
		page int
	}{
		{"launcher", SurfaceLauncher, settingsui.PageGeneral},
		{"terminal", SurfaceTerminal, settingsui.PageGeneral},
		{"settings/general", SurfaceSettings, settingsui.PageGeneral},
		{"settings/sessions", SurfaceSettings, settingsui.PageSessions},
		{"settings/integrations", SurfaceSettings, settingsui.PageIntegrations},
		{"settings/about", SurfaceSettings, settingsui.PageAbout},
	} {
		t.Run(surface.name, func(t *testing.T) {
			a, store := sessionsPage(t, 16)
			a.Settings.Integrations = func() []settingsui.Integration {
				out := make([]settingsui.Integration, 0, 20)
				for i := 0; i < 20; i++ {
					out = append(out, settingsui.Integration{
						ID: "ext", Name: "Integration", Enabled: true, Running: true,
					})
				}
				return out
			}
			a.Surf, a.Settings.Page = surface.surf, surface.page
			if got := settledFrame(t, a, store); got != 0 {
				t.Fatalf("%s took %d settings copies on a settled frame, want 0 (P11-cq)", surface.name, got)
			}
		})
	}
}

// The sessions page's own text is unchanged by the round: the round moves
// where the settings are read from, never what they hold. A frame that read
// the wrong language, or lost the ` · exit N` suffix, would show here.
func TestTheSessionsPageStillSaysWhatItSaid(t *testing.T) {
	a, _ := sessionsPage(t, 0)
	a.Terminal.Sessions = []*terminalui.Session{
		{Title: "zsh", Term: &terminal.Terminal{}, Created: time.Now()},
		{Resident: true, ExitCode: 3, Created: time.Now()},
	}
	a.Terminal.Active = 0
	sessions := a.Settings.Sessions()
	if len(sessions) != 2 {
		t.Fatalf("sessions = %+v", sessions)
	}
	if sessions[0].Title != "zsh" {
		t.Errorf("the named session's title = %q, want %q", sessions[0].Title, "zsh")
	}
	if sessions[1].Note != "Exited \u00b7 exit 3" {
		t.Errorf("the ended session's note = %q, want %q", sessions[1].Note, "Exited \u00b7 exit 3")
	}
}
