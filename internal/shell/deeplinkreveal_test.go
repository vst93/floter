package shell

import (
	"testing"
	"time"
)

// P9-m: a refused link brings the panel forward even when its sentence is
// suppressed. The old build's 30s failure deduper coalesced the *toast*; the
// reveal (`focus_main`) was never part of that window. Before P9-m the gate sat
// in front of `Show`, so a second bad link inside 30s did nothing at all — the
// hidden panel stayed hidden and the user read "the click did nothing". The
// reveal now runs before the gate; only the sentence is coalesced.
func TestARepeatRefusalStillRevealsThePanel(t *testing.T) {
	a := newApp(t)
	clock := time.Unix(1700000000, 0)
	a.now = func() time.Time { return clock }
	reveals := 0
	a.showWindow = func() { reveals++ }

	// The first refusal reveals and raises the sentence.
	a.HandleURL("floter://connect")
	if reveals != 1 {
		t.Fatalf("the first refusal revealed %d times, want 1", reveals)
	}
	if got := pendingNotices(a); got != 1 {
		t.Fatalf("the first refusal queued %d notices, want 1", got)
	}

	// A second refusal inside the window still reveals; only the sentence is
	// coalesced. The first toast is still pending (no frame drew it), so an
	// unchanged count of 1 proves no second one was queued.
	clock = clock.Add(time.Second)
	a.HandleURL("floter://register")
	if reveals != 2 {
		t.Errorf("a repeat refusal inside the window revealed %d times, want 2", reveals)
	}
	if got := pendingNotices(a); got != 1 {
		t.Errorf("a repeat refusal queued a second sentence: %d pending, want 1", got)
	}

	// Once the window elapses the sentence is raised again — the reveal count
	// keeps climbing and a second toast is queued.
	clock = clock.Add(refusalWindow)
	a.HandleURL("floter://connect")
	if reveals != 3 {
		t.Errorf("a refusal after the window revealed %d times, want 3", reveals)
	}
	if got := pendingNotices(a); got != 2 {
		t.Errorf("a refusal after the window queued %d notices, want 2", got)
	}
}

// P9-m: a malformed URL is refused where the user is, not by kicking them to
// the launcher. The refusal is about the link; the page the user was reading
// stays. Before P9-m this path opened the launcher first, so a bad external
// trigger silently replaced the settings page (or the terminal) under them.
func TestAMalformedURLDoesNotSwitchSurface(t *testing.T) {
	a := newApp(t)
	a.Open(SurfaceSettings)
	if a.Surf != SurfaceSettings {
		t.Fatalf("setup: surf = %v, want settings", a.Surf)
	}
	reveals := 0
	a.showWindow = func() { reveals++ }

	a.HandleURL("floter://%zz")
	if a.Surf != SurfaceSettings {
		t.Errorf("a malformed URL moved to surface %v, want settings", a.Surf)
	}
	if reveals != 1 {
		t.Errorf("a malformed URL revealed %d times, want 1", reveals)
	}
	if got := pendingNotices(a); got != 1 {
		t.Errorf("a malformed URL queued %d notices, want 1", got)
	}
}

// P9-m: a scheme this app does not handle stays silent (the old build kept
// UnsupportedScheme quiet), but it is still a reason to come forward — without
// changing the surface. Before P9-m it opened the launcher: the silent half was
// right, the page-kick half was wrong, and a stranger's bug moved the user.
func TestAForeignSchemeRevealsSilentlyWithoutSwitching(t *testing.T) {
	a := newApp(t)
	a.Open(SurfaceSettings)
	if a.Surf != SurfaceSettings {
		t.Fatalf("setup: surf = %v, want settings", a.Surf)
	}
	reveals := 0
	a.showWindow = func() { reveals++ }

	a.HandleURL("https://example.com/")
	if a.Surf != SurfaceSettings {
		t.Errorf("a foreign scheme moved to surface %v, want settings", a.Surf)
	}
	if reveals != 1 {
		t.Errorf("a foreign scheme revealed %d times, want 1", reveals)
	}
	if got := pendingNotices(a); got != 0 {
		t.Errorf("a foreign scheme raised %d notices, want 0", got)
	}
}
