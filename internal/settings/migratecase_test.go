package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// The migration copies a legacy settings file into place only when the legacy
// file is not the destination. That comparison used to be `strings.EqualFold`,
// which folds case on every platform — so on Linux, where `Settings.json` and
// `settings.json` are two files, the migration decided the source *was* the
// destination, found "no legacy", and silently left the user on defaults.
//
// The answer now comes from `pathid.Same`, i.e. from the filesystem. Both
// answers are asserted, so the test says the same thing on a case-sensitive
// volume (Linux) and on a case-insensitive one (macOS, by default).
func TestTheMigrationDoesNotConfuseTwoCaseSpellings(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "Settings.json")
	if err := os.WriteFile(source, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "settings.json")

	targetInfo, targetErr := os.Stat(target)
	sourceInfo, sourceErr := os.Stat(source)
	if sourceErr != nil {
		t.Fatal(sourceErr)
	}
	oneFile := targetErr == nil && os.SameFile(targetInfo, sourceInfo)

	report, err := MigrateLegacy(target, []string{source})
	if err != nil {
		t.Fatalf("MigrateLegacy: %v", err)
	}

	if oneFile {
		// The volume folded the two spellings: the "legacy" file is the target
		// itself, which holds user data, so the migration must leave it alone.
		if report.Migrated || report.Reason != "settings-exist" {
			t.Fatalf("on a case-insensitive volume the migration reported %+v, want settings-exist", report)
		}
		return
	}

	// Two files: the copy has to happen.
	if !report.Migrated || report.Source != source {
		t.Fatalf("the legacy file %q was not migrated: %+v", source, report)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read migrated settings: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("the migrated settings file is empty")
	}
}
