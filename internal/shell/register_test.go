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

// stubLookTool replaces the register link's name probe for a test, and returns
// through t.Cleanup. The probe is a variable for exactly this (`lookTool`,
// system.go) — the seam `internal/extensions` keeps for its own lookups
// (`runtimepath.go:70-76`): the real one reads the machine's search path, and
// `extensions.SearchPath` is computed once per process (`runtimepath.go:33-53`),
// so a test cannot choose it.
func stubLookTool(t *testing.T, lookup func(names ...string) (string, bool)) {
	t.Helper()
	previous := lookTool
	lookTool = lookup
	t.Cleanup(func() { lookTool = previous })
}

// stubOneTool answers the one name with the one path, and nothing else.
func stubOneTool(t *testing.T, name, path string) {
	t.Helper()
	stubLookTool(t, func(names ...string) (string, bool) {
		for _, asked := range names {
			if asked == name {
				return path, true
			}
		}
		return "", false
	})
}

// A link that names a program the probe finds connects it through the same
// one-press flow the Detected row uses, and the resulting card wears the
// link's mark: the reference drew the dot only on the named row
// (`src/extensions/ExtensionRow.tsx:196-205,290`).
//
// The link carries a **name**, not a path: the router refuses anything that is
// not a bare command name (`bareCommandName`, linkname.go), so the fixture is
// what the name probe answers with — the reference's register resolves through
// the discovery inventory the same way (`deep_link.rs:1136-1180`).
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
	stubOneTool(t, "rg", path)
	a.registerFromLink("rg")

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
	a.registerFromLink("rg")
	want := i18n.For("en").Settings.RegisterAlreadyConnected("rg")
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
	stubOneTool(t, "rg", path)
	a.registerFromLink("rg")
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

// The router is the gate, and there is no second door behind it: a caller that
// hands registerFromLink a path gets "not found", never a connect. The
// reference has no path branch at all — `validate_command` refuses one before
// the resolver runs (`deep_link.rs:364-390`) — while this build used to take an
// absolute path as it was, with no question about whether it was a program
// (`system.go:252-260`), so `floter://register?cmd=C:\notes.txt` connected a
// text file. The branch is gone; this pin is what keeps it gone.
func TestTheRegisterLinkDoesNotTakeAPath(t *testing.T) {
	a := newApp(t)
	stubOneTool(t, "rg", filepath.Join(t.TempDir(), "rg"))

	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.registerFromLink(path)

	if a.registerTarget != "" {
		t.Errorf("a path was marked as a register target: %q", a.registerTarget)
	}
	want := i18n.For("en").Settings.RegisterNotFound(path)
	if got := pendingNoticeText(a); got != want {
		t.Errorf("a path reads %q, want %q", got, want)
	}
}
