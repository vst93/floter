package recommended

import (
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
)

// The shipped package materializes into a directory the install pipeline can
// take: a manifest and a descriptor, nothing else.
func TestMaterialize(t *testing.T) {
	tool, ok := WithID("io.github.vst93.v")
	if !ok {
		t.Fatal("the shipped package is not in the table")
	}
	dir := t.TempDir()
	if err := tool.Materialize(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	if !names["floter.extension.json"] || !names["provider-description.json"] {
		t.Fatalf("the package = %v", names)
	}
	// The manifest parses and is the one the table names.
	manifest, err := extensions.LoadManifest(filepath.Join(dir, "floter.extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != tool.ID || manifest.Name != tool.Name {
		t.Errorf("manifest = %+v", manifest)
	}
	if manifest.Provider.Descriptor != "provider-description.json" {
		t.Errorf("descriptor = %q", manifest.Provider.Descriptor)
	}

	if _, ok := WithID("nope"); ok {
		t.Error("an unknown id resolved")
	}
	dir2 := t.TempDir()
	missing := Tool{ID: "x", Dir: "missing"}
	if err := missing.Materialize(dir2); err == nil {
		t.Error("a missing package materialized")
	}
}

// Preparing a shipped package stages it for the ordinary install: the
// manifest is read and the permissions it declares are what the user is asked
// about.
func TestPrepare(t *testing.T) {
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	prepared, cleanup, err := Prepare(paths, "io.github.vst93.v")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if prepared.Manifest.ID != "io.github.vst93.v" {
		t.Errorf("prepared = %+v", prepared.Manifest)
	}
	if len(prepared.Approval.Declared) == 0 {
		t.Error("the package declares no permissions")
	}
	// Nothing is installed until it is committed.
	if _, ok := extensions.LoadInventory(paths).WithID("io.github.vst93.v"); ok {
		t.Error("preparing installed the package")
	}
	if _, _, err := Prepare(paths, "nope"); err == nil {
		t.Error("an unknown id prepared")
	}
}
