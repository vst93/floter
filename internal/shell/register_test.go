package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// pendingNoticeText is the sentence of the notice waiting for the next frame,
// empty when none is queued.
func pendingNoticeText(a *App) string {
	a.noticeMu.Lock()
	defer a.noticeMu.Unlock()
	if len(a.notices) == 0 {
		return ""
	}
	return a.notices[0].Title
}

// A `floter://register` link that names nothing resolvable says so instead of
// connecting half a package: the reference's `registerMiss` rendered
// `settings.extensions.registerNotFound` rather than leaving the user with a
// page that looks like nothing happened
// (`src/ExtensionsPanel.tsx:1516-1528,2516-2528`).
func TestTheRegisterLinkSaysWhenNoToolMatches(t *testing.T) {
	a := newApp(t)
	const name = "no-such-program-anywhere-12345"
	a.registerFromLink(name)

	if a.registerTarget != "" {
		t.Errorf("a link that matched nothing set a target: %q", a.registerTarget)
	}
	want := i18n.For("en").Settings.RegisterNotFound(name)
	if got := pendingNoticeText(a); got != want {
		t.Errorf("the miss reads %q, want %q", got, want)
	}
}

// A link that names a program this machine has connects it through the same
// one-press flow the Detected row uses, and the resulting card wears the
// link's mark: the reference drew the dot only on the named row
// (`src/extensions/ExtensionRow.tsx:196-205,290`).
func TestTheRegisterLinkMarksTheProgramItNamed(t *testing.T) {
	requirePOSIXShell(t)
	a := New(Options{
		Store: settings.NewStore(settings.Default()),
		Paths: extensions.FromRoot(t.TempDir()),
		// The connect reviews permissions; a test answers without a dialog.
		ConfirmPermissions: func(extensions.PermissionApproval) bool { return true },
	})

	path := filepath.Join(t.TempDir(), "rg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.registerFromLink(path)

	if a.registerTarget != path {
		t.Fatalf("registerTarget = %q, want %q", a.registerTarget, path)
	}
	if got := pendingNoticeText(a); got != "" {
		t.Errorf("a matched link raised a notice: %q", got)
	}

	// The connect runs off this thread; the card appears after it lands.
	deadline := time.Now().Add(5 * time.Second)
	var marked bool
	for time.Now().Before(deadline) {
		for _, integration := range a.integrationList() {
			if integration.Highlighted {
				marked = true
			}
		}
		if marked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !marked {
		t.Fatalf("no card wore the link mark after the connect (target %q)", a.registerTarget)
	}

	// Registering the same program again is not a second connect: the
	// reference said "already connected" rather than "not found"
	// (`src/ExtensionsPanel.tsx:1516-1524`).
	a.registerFromLink(path)
	want := i18n.For("en").Settings.RegisterAlreadyConnected(path)
	if got := pendingNoticeText(a); got != want {
		t.Errorf("the second link reads %q, want %q", got, want)
	}
}

// The mark is the link's moment, not a mode: it clears on its own (the
// reference's `useTimedReset(registerTarget, …)`,
// `src/ExtensionsPanel.tsx:1546`).
func TestTheRegisterMarkIsAMoment(t *testing.T) {
	requirePOSIXShell(t)
	a := newApp(t)
	a.registerHighlight = 20 * time.Millisecond

	path := filepath.Join(t.TempDir(), "rg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.registerFromLink(path)
	if a.registerTarget != path {
		t.Fatalf("registerTarget = %q, want %q", a.registerTarget, path)
	}

	deadline := time.Now().Add(2 * time.Second)
	for a.registerTarget != "" && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if a.registerTarget != "" {
		t.Errorf("the mark did not clear: %q", a.registerTarget)
	}
}

// sameExecutable treats a symlinked pair as one program and never matches an
// empty path: the register link may hand back the search-path link while the
// inventory records the resolved binary.
func TestSameExecutableResolvesSymlinks(t *testing.T) {
	requirePOSIXShell(t)
	dir := t.TempDir()
	real := filepath.Join(dir, "rg")
	if err := os.WriteFile(real, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "rg-link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	other := filepath.Join(dir, "fd")
	if err := os.WriteFile(other, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		left, right string
		want        bool
	}{
		{real, real, true},
		{link, real, true},
		{real, link, true},
		{real, other, false},
		{"", real, false},
		{real, "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		if got := sameExecutable(tc.left, tc.right); got != tc.want {
			t.Errorf("sameExecutable(%q, %q) = %v, want %v", tc.left, tc.right, got, tc.want)
		}
	}
}
