package shell

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// The local package review always carries the manifest summary, whether or
// not the package declares anything: the reference reviews a local package
// unconditionally (ExtensionsPanel.tsx:1430-1440, LocalInstallDialog.tsx).
func TestLocalInstallReviewCarriesTheManifestSummary(t *testing.T) {
	copy := i18n.For("en").Settings
	prepared := extensions.Prepared{
		Manifest: extensions.Manifest{
			Name:      "Ripgrep",
			Platforms: []string{"darwin-arm64", "linux-x86_64-gnu"},
			Runtime:   extensions.Runtime{Type: "system"},
		},
	}
	title, detail := localInstallReviewText(prepared, copy, "en")
	if title != "Connect Ripgrep?" {
		t.Errorf("title = %q", title)
	}
	for _, want := range []string{
		"Source: Local package",
		"Runtime: system",
		"Supported platforms: darwin-arm64, linux-x86_64-gnu",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail is missing %q:\n%s", want, detail)
		}
	}
	// A package that declares nothing gets no permission lines and no
	// boundary note.
	if strings.Contains(detail, copy.InstallReviewBoundary) {
		t.Errorf("a package that declares nothing carried the boundary note:\n%s", detail)
	}
	if strings.Contains(detail, "\u2022 ") {
		t.Errorf("a package that declares nothing carried a permission line:\n%s", detail)
	}
	// An empty platform list says so rather than showing nothing.
	prepared.Manifest.Platforms = nil
	if _, detail := localInstallReviewText(prepared, copy, "en"); !strings.Contains(detail, "Supported platforms: Not available") {
		t.Errorf("an empty platform list = %q", detail)
	}
}

// The declared permissions ride the review with their enforcement sentence,
// and the boundary note follows them.
func TestLocalInstallReviewListsDeclaredPermissions(t *testing.T) {
	copy := i18n.For("en").Settings
	prepared := extensions.Prepared{
		Manifest: extensions.Manifest{
			Name:      "Gated",
			Platforms: []string{"linux-x86_64-gnu"},
			Runtime:   extensions.Runtime{Type: "script"},
		},
		Approval: extensions.PermissionApproval{
			Declared: []string{extensions.PermissionEnvironment, extensions.PermissionNetworkFetch},
		},
	}
	_, detail := localInstallReviewText(prepared, copy, "en")
	enforced := "\u2022 " + i18n.PermissionLabel("en", extensions.PermissionEnvironment) + " \u2014 " + copy.PermissionsEnforced
	if !strings.Contains(detail, enforced) {
		t.Errorf("the enforced permission is missing:\n%s", detail)
	}
	declared := "\u2022 " + i18n.PermissionLabel("en", extensions.PermissionNetworkFetch) + " \u2014 " + copy.PermissionsDeclared
	if !strings.Contains(detail, declared) {
		t.Errorf("the disclosed permission is missing:\n%s", detail)
	}
	if !strings.Contains(detail, copy.InstallReviewBoundary) {
		t.Errorf("the boundary note is missing:\n%s", detail)
	}
}

// writeLocalPackage writes a local package directory whose manifest declares
// the given permissions JSON fragment (e.g. `"permissions": ["environment"]`,
// or empty for none).
func writeLocalPackage(t *testing.T, dir, permissions string) {
	t.Helper()
	manifest := `{
  "schemaVersion": "2.0", "id": "dev.floter.reviewed", "name": "Reviewed",
  "distribution": {"type": "local"},
  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
  "provider": {"type": "static-descriptor", "descriptor": "description.json", "argsPrefix": []}` + permissions + `
}`
	if err := os.WriteFile(filepath.Join(dir, "floter.extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tool.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	descriptor := `{"protocolVersion":"1.0","provider":{"id":"dev.floter.reviewed","name":"Reviewed","version":"1.0.0"},"commands":[]}`
	if err := os.WriteFile(filepath.Join(dir, "description.json"), []byte(descriptor), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The folder install goes through the review first — and the review happens
// even when the package declares nothing, which is the reference's own rule:
// `reviewLocalManifest` runs on every folder pick and only its confirm reaches
// `extensions_install` (ExtensionsPanel.tsx:1430-1440,1550-1560).
func TestInstallFromFolderReviewsBeforeCommitting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's runtime is a shell script")
	}
	pkg := filepath.Join(t.TempDir(), "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLocalPackage(t, pkg, "")

	var reviewed []extensions.Prepared
	answer := false
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
		ReviewInstall: func(prepared extensions.Prepared) bool {
			reviewed = append(reviewed, prepared)
			return answer
		},
	})

	a.Settings.Actions.InstallFromFolder(pkg)
	deadline := time.Now().Add(10 * time.Second)
	for len(reviewed) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the review was never shown")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// The package declares nothing, so the old code would have skipped
	// straight to the install; the review must still be the door.
	if reviewed[0].Approval.NeedsApproval() {
		t.Errorf("the fixture declares permissions: %+v", reviewed[0].Approval)
	}
	if reviewed[0].Manifest.Name != "Reviewed" {
		t.Errorf("reviewed manifest = %+v", reviewed[0].Manifest)
	}
	// Declining installs nothing.
	time.Sleep(200 * time.Millisecond)
	if inventory := a.Integrations.Inventory(); len(inventory.Integrations) != 0 {
		t.Fatalf("a declined review landed: %+v", inventory.Integrations)
	}

	// Confirming installs it.
	answer = true
	a.Settings.Actions.InstallFromFolder(pkg)
	for {
		if integration, ok := a.Integrations.Inventory().WithID("dev.floter.reviewed"); ok && integration.Name == "Reviewed" {
			break
		}
		if time.Now().After(deadline.Add(20 * time.Second)) {
			t.Fatalf("the confirmed install did not land: %+v", a.Integrations.Inventory().Integrations)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(reviewed) != 2 {
		t.Errorf("review count = %d, want one per folder pick", len(reviewed))
	}
}
