package shell

import (
	"testing"

	"floter/internal/i18n"
	"floter/internal/settingsui"
)

// The removal completion is the kind's own sentence, not one generic word.
// The reference had four (`src/ExtensionsPanel.tsx:1306-1308,1336-1339`): a
// generated custom integration is deleted, an npm package is uninstalled, a
// system-owned built-in is disconnected, and a linked package is removed from
// the list. The settings page's confirmation bar already spoke the same kind,
// so the notification must not answer with a sentence the user did not
// confirm.
func TestIntegrationRemovalNoticeFollowsTheKind(t *testing.T) {
	en := i18n.For("en").Notifications
	for _, tc := range []struct {
		kind string
		want string
	}{
		{settingsui.RemovalCustom, "Deleted custom integration V Tools"},
		{settingsui.RemovalNPM, "Uninstalled V Tools"},
		{settingsui.RemovalSystem, "Disconnected V Tools; the external files and system tool were kept"},
		{settingsui.RemovalPackage, "Removed extension package V Tools from the list; external files were kept"},
	} {
		if got := integrationRemovalNotice(en, tc.kind, "V Tools"); got != tc.want {
			t.Errorf("kind %q = %q, want %q", tc.kind, got, tc.want)
		}
	}
	// An unknown kind is a package by default: `removalKind` has no other
	// answer, and a hand-written caller should still say what was removed.
	if got := integrationRemovalNotice(en, "something-new", "V Tools"); got != en.IntegrationPackageRemoved("V Tools") {
		t.Errorf("unknown kind = %q, want the package sentence", got)
	}
}
