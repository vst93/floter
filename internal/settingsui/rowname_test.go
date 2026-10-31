package settingsui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// sessionsPageWith renders the sessions page at width with one session whose
// name is title, the way a shell names it: a program-chosen string, not a
// sentence this build wrote.
func sessionsPageWith(t *testing.T, title string, width int) *ui.Tester {
	t.Helper()
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{SelectSession: func(int) {}, CloseSessionAt: func(int) {}})
	created := time.Date(2024, 1, 2, 15, 4, 0, 0, time.Local)
	a.Sessions = func() []Session {
		return []Session{{
			ID: 1, Title: title, Running: true, Index: 0,
			Note: "Running · attached", Size: "80×24", Created: created,
			Dir: "/home/vst/workspace/floter/internal/settingsui",
		}}
	}
	a.selectPage(PageSessions)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, width, 600)
	tt.SetDark(true)
	tt.Frame()
	tt.Frame()
	return tt
}

// The session row's name is a *value*, so it truncates instead of wrapping:
// `.session-manager__row .settings-row__label` is `white-space: nowrap` with
// `text-overflow: ellipsis` in the reference (`main:src/styles/settings.css:
// 1249-1260`), under the comment that a daemon-chosen name "has to truncate
// rather than wrap, or one row would grow and break the list's rhythm".
//
// Measured before the pin existed: at 460u a long name wrapped to four lines
// (18u → 72u) and pushed that one row's Play/Trash pair down with it — the
// card's rhythm broke on exactly the session whose name was most informative.
func TestASessionNameIsOneLineWhateverItsLength(t *testing.T) {
	const long = "vst@build-host: ~/workspace/floter/internal/settingsui (mygo-rewrite)"
	for _, width := range []int{860, 560, 460} {
		width := width
		short := sessionsPageWith(t, "zsh", width)
		shortName, ok := short.Find("zsh")
		if !ok {
			t.Fatalf("%du: the short session name is not drawn: %q", width, short.Texts())
		}
		tall := sessionsPageWith(t, long, width)
		longName, ok := tall.Find(long)
		if !ok {
			t.Fatalf("%du: the long session name is not drawn: %q", width, tall.Texts())
		}
		if longName.H != shortName.H {
			t.Errorf("%du: a long session name is %.1fu tall, a short one %.1fu — "+
				"the name wrapped and grew the row", width, longName.H, shortName.H)
		}
		if longName.Y != shortName.Y {
			t.Errorf("%du: the name starts at y=%.1f with a long title and y=%.1f with a short one", width, longName.Y, shortName.Y)
		}
		// The grey block under the name is what the row's height is made of, so
		// the row kept its height exactly when that block did not move.
		const dir = "/home/vst/workspace/floter/internal/settingsui"
		shortDir, okShort := short.Find(dir)
		longDir, okLong := tall.Find(dir)
		if !okShort || !okLong {
			t.Fatalf("%du: the working directory is not drawn (short %v, long %v)", width, okShort, okLong)
		}
		if shortDir.Y != longDir.Y {
			t.Errorf("%du: the directory line sits at y=%.1f under a long name and y=%.1f under a short one",
				width, longDir.Y, shortDir.Y)
		}
	}
}

// Cutting a name in half is only acceptable because the cut is in the paint:
// the element still holds every character, and resting the pointer on it hands
// the whole string back. The reference does not do this — its sessions page is
// not the only place a session exists — but this build's is, so a truncated
// name would otherwise be unreadable everywhere.
func TestATruncatedSessionNameStillReadsInFull(t *testing.T) {
	const long = "vst@build-host: ~/workspace/floter/internal/settingsui (mygo-rewrite)"
	tt := sessionsPageWith(t, long, 460)
	if !tt.HasText(long) {
		t.Fatalf("the name was cut in the data, not just in the paint: %q", tt.Texts())
	}
	name, ok := tt.Find(long)
	if !ok {
		t.Fatalf("the long session name is not drawn: %q", tt.Texts())
	}
	if got := countOfText(tt, long); got != 1 {
		t.Fatalf("the name is drawn %d times before the pointer rests, want 1 (the row itself)", got)
	}
	// The tip waits for the pointer to rest (600ms), and the tester's clock is
	// its own, so the wait is Advance's, not the wall clock's.
	tt.Move(name.X+name.W/2, name.Y+name.H/2)
	tt.Advance(700 * time.Millisecond)
	tt.Frame()
	if got := countOfText(tt, long); got != 2 {
		t.Errorf("resting the pointer on a truncated name drew it %d times, want 2 "+
			"(the row and its tip): %q", got, tt.Texts())
	}
}

// countOfText counts the nodes showing exactly want — a label and the tooltip
// a hover reveals are two, so an increase is the tip appearing.
func countOfText(tt *ui.Tester, want string) int {
	n := 0
	for _, s := range tt.Texts() {
		if s == want {
			n++
		}
	}
	return n
}

// integrationsPageWith renders the integrations page at width with one
// integration whose package name is name.
func integrationsPageWith(t *testing.T, name string, width int) *ui.Tester {
	t.Helper()
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "ext", Name: name, Version: "1.2.3", Enabled: true, Running: true,
		}}
	}
	a.selectPage(PageIntegrations)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, width, 760)
	tt.SetDark(true)
	tt.Frame()
	tt.Frame()
	return tt
}

// The integrations row's title is the reference's `.extension-row__title`, and
// its name is `strong` with `white-space: nowrap` and `text-overflow: ellipsis`
// (`main:src/styles/extensions.css:895-907`) — a package name is a value like a
// session's, so it takes one line rather than pushing the version and the
// freshness mark out of the row. Measured before the pin: at 460u a long name
// wrapped to two lines and the description under it moved down with it.
func TestAnIntegrationNameIsOneLineWhateverItsLength(t *testing.T) {
	const long = "com.vst93.floter.extensions.markdown-preview-themes"
	for _, width := range []int{860, 560, 460} {
		width := width
		short := integrationsPageWith(t, "V Tools", width)
		shortName, ok := short.Find("V Tools")
		if !ok {
			t.Fatalf("%du: the short integration name is not drawn: %q", width, short.Texts())
		}
		tall := integrationsPageWith(t, long, width)
		longName, ok := tall.Find(long)
		if !ok {
			t.Fatalf("%du: the long integration name is not drawn: %q", width, tall.Texts())
		}
		if longName.H != shortName.H {
			t.Errorf("%du: a long integration name is %.1fu tall, a short one %.1fu — "+
				"the name wrapped", width, longName.H, shortName.H)
		}
		// The version chip rides the same line as the name (`.extension-row__
		// title span { flex: 0 0 auto }`): a name that grew past its column
		// would push it off the row.
		version, ok := tall.Find("v1.2.3")
		if !ok {
			t.Fatalf("%du: the version chip left the row when the name grew: %q", width, tall.Texts())
		}
		if version.Y+version.H > longName.Y+longName.H+1 {
			t.Errorf("%du: the version chip (y=%.1f..%.1f) fell below the name line (y=%.1f..%.1f)",
				width, version.Y, version.Y+version.H, longName.Y, longName.Y+longName.H)
		}
	}
}
