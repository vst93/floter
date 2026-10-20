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

// The connect review names the tool the way the reference's
// `confirmConnectRecommended` does, shows the program it would run, and splits
// the manifest's own permissions into the two tiers with each tier's sentence
// and each permission's explanation — the reference's `permissionsRequired`
// label, `PermissionTierList` and `permissionReviewBoundary`
// (ExtensionsPanel.tsx:2594-2620).
func TestRecommendedInstallReviewNamesTheToolAndTiersItsPermissions(t *testing.T) {
	copy := i18n.For("en").Settings
	prepared := extensions.Prepared{
		Manifest:       extensions.Manifest{Name: "V Tools"},
		ExecutablePath: "/usr/local/bin/v",
		Approval: extensions.PermissionApproval{
			Declared: []string{
				extensions.PermissionProcessSpawn,
				extensions.PermissionEnvironment,
				extensions.PermissionNetworkFetch,
			},
		},
	}
	title, detail := recommendedInstallReviewText(prepared, copy, "en")
	if title != "Connect the recommended tool V Tools to Floter?" {
		t.Errorf("title = %q", title)
	}
	for _, want := range []string{
		"/usr/local/bin/v",
		copy.PermissionsRequired,
		copy.PermissionsEnforcedLabel,
		copy.PermissionsEnforcedHint,
		copy.PermissionsDisclosureLabel,
		copy.PermissionsDisclosureHint,
		copy.InstallReviewBoundary,
		"\u2022 " + i18n.PermissionLabel("en", extensions.PermissionProcessSpawn) + " \u2014 " + i18n.PermissionDescription("en", extensions.PermissionProcessSpawn),
		"\u2022 " + i18n.PermissionLabel("en", extensions.PermissionNetworkFetch) + " \u2014 " + i18n.PermissionDescription("en", extensions.PermissionNetworkFetch),
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail is missing %q:\n%s", want, detail)
		}
	}
	// The host-enforced tier comes first: the split is the message, so the
	// order is part of the contract, not incidental.
	if enforced, disclosure := strings.Index(detail, copy.PermissionsEnforcedLabel), strings.Index(detail, copy.PermissionsDisclosureLabel); enforced < 0 || disclosure < 0 || enforced > disclosure {
		t.Errorf("the tiers are out of order (enforced=%d, disclosure=%d):\n%s", enforced, disclosure, detail)
	}
	// A resolved program is only shown when the connect flow already knows it.
	prepared.ExecutablePath = ""
	if _, detail := recommendedInstallReviewText(prepared, copy, "en"); strings.Contains(detail, "/usr/local/bin/v") {
		t.Errorf("an unresolved program leaked into the review:\n%s", detail)
	}
}

// The connect flow asks for the manifest's permissions first and only connects
// once the user confirms the review: declining leaves the machine untouched,
// confirming installs the package (ExtensionsPanel.tsx:1349-1400).
func TestRecommendedConnectReviewsBeforeCommitting(t *testing.T) {
	requirePOSIXShell(t)
	// The shipped package runs the V toolchain; a fixture stands in for it, so
	// the test does not depend on the machine having V installed.
	restore := extensions.StubToolLookup(func(names ...string) (string, bool) {
		path := filepath.Join(t.TempDir(), "tool")
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path, true
	})
	defer restore()

	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	answer := false
	reviewed := 0
	a := New(Options{
		Store:                settings.NewStore(settings.Default()),
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		ReviewRecommended:    func(extensions.Prepared) bool { reviewed++; return answer },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})

	a.connectRecommended("io.github.vst93.v")
	deadline := time.Now().Add(10 * time.Second)
	for reviewed == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the review was never shown")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Declining installs nothing.
	time.Sleep(200 * time.Millisecond)
	if list := a.recommendedTools(); len(list) != 1 || list[0].Installed {
		t.Fatalf("a declined review landed: %+v", list)
	}
	if _, err := os.Stat(filepath.Join(paths.Extensions, "io.github.vst93.v", "floter.extension.json")); err == nil {
		t.Error("a declined review wrote the package")
	}

	// Confirming installs it.
	answer = true
	a.connectRecommended("io.github.vst93.v")
	for {
		if list := a.recommendedTools(); len(list) == 1 && list[0].Installed {
			break
		}
		if time.Now().After(deadline.Add(20 * time.Second)) {
			t.Fatalf("the confirmed connect did not land: %+v", a.recommendedTools())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if reviewed != 2 {
		t.Errorf("review count = %d, want one per connect", reviewed)
	}
}
