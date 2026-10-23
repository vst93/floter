package shell

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// writeLinkedPackage stages a local package directory the `connect` link can
// review. The manifest is the caller's, so a test can choose one that clears
// PrepareLocal and fails (or lands) at Commit.
func writeLinkedPackage(t *testing.T, paths extensions.Paths, id, manifest string) string {
	t.Helper()
	pkg := filepath.Join(paths.Extensions, id)
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return pkg
}

// A `floter://connect` link that clears the review but fails while installing
// says so: the link opened the integrations page and asked for approval, so a
// silent dead end is the one thing the user cannot interpret. This is the same
// refusal the link's two earlier failures (no manifest, unreadable manifest)
// already raise, through the same window.
//
// The package here is structurally valid and passes PrepareLocal; it names a
// system runtime whose program is not on the machine, so the failure happens
// inside Commit — after the review, which is exactly the path that used to
// only write a log line.
func TestTheLinkedConnectThatFailsToCommitSaysSo(t *testing.T) {
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	// PrepareLocal never resolves the runtime; install does (see
	// extensions.ResolveRuntime), so an absent program is a Commit-time
	// failure, not a review-time one.
	pkg := writeLinkedPackage(t, paths, "dev.floter.ghost", `{
	  "schemaVersion": "2.0", "id": "dev.floter.ghost", "name": "Ghost",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "system", "executableNames": ["floter-test-no-such-program"]},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]}
	}`)

	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:                store,
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		ConfirmPermissions:   func(extensions.PermissionApproval) bool { return true },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})

	a.HandleURL("floter://connect?manifest=" + pkg)
	deadline := time.Now().Add(5 * time.Second)
	for pendingNotices(a) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the refusal never arrived: a Commit failure stayed silent")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, ok := a.Integrations.Inventory().WithID("dev.floter.ghost"); ok {
		t.Fatal("a failed link installed the integration")
	}
}

// A link whose install lands reports it through the app's one completion path,
// exactly as the UI's one-press connect does (connectLocalTool): the panel is
// hidden in a headless test, so the result arrives as a system notification.
// A link that installs and says nothing leaves the user guessing whether the
// click did anything at all.
func TestTheLinkedConnectThatLandsSaysSo(t *testing.T) {
	requirePOSIXShell(t)
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := writeLinkedPackage(t, paths, "dev.floter.linked", `{
	  "schemaVersion": "2.0", "id": "dev.floter.linked", "name": "Linked",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]},
	  "permissions": ["environment"]
	}`)
	if err := os.WriteFile(filepath.Join(pkg, "tool.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var notified []string
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:                store,
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		ConfirmPermissions:   func(extensions.PermissionApproval) bool { return true },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
		Notify:               func(title, body string) { notified = append(notified, title+"|"+body) },
	})

	a.HandleURL("floter://connect?manifest=" + pkg)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, ok := a.Integrations.Inventory().WithID("dev.floter.linked"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the link never connected the integration")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The completion runs after RefreshIntegrations on the same goroutine; give
	// the notification seam the same moment the install got.
	deadline = time.Now().Add(5 * time.Second)
	for len(notified) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("a landed link said nothing: a silent install leaves the user guessing")
		}
		time.Sleep(5 * time.Millisecond)
	}
	want := i18n.NotificationsFor("en").IntegrationInstalled("Linked")
	if !strings.Contains(notified[0], want) {
		t.Errorf("completion = %q, want it to carry %q", notified[0], want)
	}
	if len(notified) != 1 {
		t.Errorf("a landed link raised %d notifications, want 1: %q", len(notified), notified)
	}
}

// A declined permission review is the user's decision, not a failure: the link
// must not answer it with "that link could not be opened". The install does not
// happen (already pinned by TestDeepLinkConnectAndRegister) and nothing is said.
func TestTheLinkedConnectThatIsDeclinedStaysQuiet(t *testing.T) {
	requirePOSIXShell(t)
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := writeLinkedPackage(t, paths, "dev.floter.declined", `{
	  "schemaVersion": "2.0", "id": "dev.floter.declined", "name": "Declined",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]},
	  "permissions": ["environment"]
	}`)
	if err := os.WriteFile(filepath.Join(pkg, "tool.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	asked := 0
	var notified []string
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:                store,
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		ConfirmPermissions:   func(extensions.PermissionApproval) bool { asked++; return false },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
		Notify:               func(title, body string) { notified = append(notified, title+"|"+body) },
	})

	a.HandleURL("floter://connect?manifest=" + pkg)
	deadline := time.Now().Add(5 * time.Second)
	for asked == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the review never ran")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// Let Commit run and answer the decline before asserting the silence.
	time.Sleep(100 * time.Millisecond)
	if _, ok := a.Integrations.Inventory().WithID("dev.floter.declined"); ok {
		t.Fatal("a declined link installed the integration")
	}
	if got := pendingNotices(a); got != 0 {
		t.Errorf("a declined review raised %d notices, want 0: a decision is not a failure", got)
	}
	if len(notified) != 0 {
		t.Errorf("a declined review raised %d notifications, want 0: %q", len(notified), notified)
	}
}
