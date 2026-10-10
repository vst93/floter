package extensions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const exampleManifest = `{
  "schemaVersion": "2.0",
  "id": "io.github.vst93.v",
  "name": "V Tools",
  "description": "Developer tools running in the terminal",
  "homepage": "https://github.com/vst93/v",
  "publisher": { "id": "vst93", "name": "vst" },
  "compatibility": { "floter": ">=0.3.0", "providerProtocol": "^1.0" },
  "distribution": { "type": "local" },
  "runtime": { "type": "system", "executableNames": ["v"], "versionArgs": ["--version"] },
  "provider": { "type": "executable", "argsPrefix": ["--floter"], "describeTimeoutMs": 5000, "completeTimeoutMs": 800 },
  "permissions": ["filesystem-read", "filesystem-write", "network-fetch", "process-spawn", "clipboard-read", "clipboard-write", "environment"]
}`

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, manifestFileName)
	writeFile(t, path, exampleManifest)

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "io.github.vst93.v" || m.Name != "V Tools" {
		t.Errorf("identity = %q / %q", m.ID, m.Name)
	}
	if m.Publisher.ID != "vst93" || m.Publisher.Name != "vst" {
		t.Errorf("publisher = %+v", m.Publisher)
	}
	if m.Compatibility.ProviderProtocol != "^1.0" {
		t.Errorf("protocol = %q", m.Compatibility.ProviderProtocol)
	}
	if m.Runtime.Type != "system" || !reflect.DeepEqual(m.Runtime.ExecutableNames, []string{"v"}) {
		t.Errorf("runtime = %+v", m.Runtime)
	}
	if m.Provider.Type != "executable" || !reflect.DeepEqual(m.Provider.ArgsPrefix, []string{"--floter"}) {
		t.Errorf("provider = %+v", m.Provider)
	}
	if m.DescribeTimeout() != 5000 || m.CompleteTimeout() != 800 {
		t.Errorf("timeouts = %d / %d", m.DescribeTimeout(), m.CompleteTimeout())
	}
	if len(m.Permissions) != 7 {
		t.Errorf("permissions = %v", m.Permissions)
	}

	// The shipped protocol defaults fill a manifest that names none.
	bare := filepath.Join(dir, "bare.json")
	writeFile(t, bare, `{"id": "a.b", "name": "Bare", "provider": {"type": "static-descriptor"}}`)
	m, err = LoadManifest(bare)
	if err != nil {
		t.Fatal(err)
	}
	if m.DescribeTimeout() != DefaultDescribeTimeoutMs || m.CompleteTimeout() != DefaultCompleteTimeoutMs {
		t.Errorf("default timeouts = %d / %d", m.DescribeTimeout(), m.CompleteTimeout())
	}

	if _, err := LoadManifest(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("a missing manifest loaded")
	}
	bad := filepath.Join(dir, "bad.json")
	writeFile(t, bad, "{not json")
	if _, err := LoadManifest(bad); err == nil {
		t.Error("a malformed manifest loaded")
	}
	anonymous := filepath.Join(dir, "anonymous.json")
	writeFile(t, anonymous, `{"schemaVersion": "2.0"}`)
	if _, err := LoadManifest(anonymous); err == nil {
		t.Error("a manifest with no id loaded")
	}
}

// TestRepositoryRoundTripKeepsUnknownKeys is the compatibility guarantee: a
// write from this build must not drop the fields an older build recorded.
func TestRepositoryRoundTripKeepsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, repositoryFile)
	original := `{
  "schemaVersion": 1,
  "extensions": {
    "io.github.vst93.v": {
      "id": "io.github.vst93.v",
      "name": "V Tools",
      "publisherId": "vst93",
      "publisherName": "vst",
      "distributionSource": "local",
      "runtimeOwnership": "system",
      "providerKind": "executable",
      "state": "enabled",
      "enabled": true,
      "packageVersion": "0.0.12",
      "currentVersion": "0.0.12",
      "manifestPath": "/tmp/v/floter.extension.json",
      "executablePath": "/usr/local/bin/v",
      "installedAt": 1700000000,
      "updatedAt": 1700000000,
      "channel": "stable",
      "runtimeIntegrity": "sha256-abc",
      "previousIntegrity": "sha256-old",
      "signatureVerified": true,
      "officialVerified": false,
      "probeReport": {"ok": true, "probes": []},
      "enabledBeforeBroken": null
    }
  }
}`
	writeFile(t, path, original)

	repo, err := LoadRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := repo.Extensions["io.github.vst93.v"]
	if !ok {
		t.Fatal("the entry did not load")
	}
	if entry.Name != "V Tools" || !entry.Enabled || entry.State != "enabled" {
		t.Errorf("entry = %+v", entry)
	}
	// probeReport is typed now (a HealthReport, the old build's field), so it
	// is asserted below rather than as a carried-through key.
	extra := entry.Extra()
	for _, key := range []string{"runtimeIntegrity", "previousIntegrity", "signatureVerified", "enabledBeforeBroken"} {
		if _, ok := extra[key]; !ok {
			t.Errorf("unknown key %q was not carried through: %v", key, extra)
		}
	}

	// Disable it and write the file back: everything else must survive.
	if _, ok := repo.SetEnabled("io.github.vst93.v", false); !ok {
		t.Fatal("SetEnabled did not find the entry")
	}
	if err := SaveRepository(path, repo); err != nil {
		t.Fatal(err)
	}
	back, err := LoadRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	entry = back.Extensions["io.github.vst93.v"]
	if entry.Enabled || entry.State != "disabled" {
		t.Errorf("enabled/state = %v/%q", entry.Enabled, entry.State)
	}
	if entry.PackageVersion != "0.0.12" || entry.ExecutablePath != "/usr/local/bin/v" {
		t.Errorf("known fields drifted: %+v", entry)
	}
	// The typed report survived the write: an unknown-shaped probeReport from
	// an older build parses into an empty one, and the write keeps it.
	if entry.ProbeReport == nil {
		t.Fatal("the stored probe report was dropped")
	}
	extra = entry.Extra()
	for _, key := range []string{"runtimeIntegrity", "previousIntegrity", "signatureVerified"} {
		if _, ok := extra[key]; !ok {
			t.Errorf("key %q was dropped by the write: %v", key, extra)
		}
	}
	var raw map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["schemaVersion"] != float64(1) {
		t.Errorf("schemaVersion = %v", raw["schemaVersion"])
	}
}

func TestRepositoryLoadsTolerantly(t *testing.T) {
	dir := t.TempDir()

	// A missing file is an empty repository, not an error.
	repo, err := LoadRepository(filepath.Join(dir, "none.json"))
	if err != nil || len(repo.Extensions) != 0 || repo.SchemaVersion != RepositorySchemaVersion {
		t.Errorf("missing file: %+v, %v", repo, err)
	}

	// An unknown schema is refused rather than guessed at.
	path := filepath.Join(dir, repositoryFile)
	writeFile(t, path, `{"schemaVersion": 99, "extensions": {}}`)
	if _, err := LoadRepository(path); err == nil {
		t.Error("an unknown schema loaded")
	}

	// Malformed JSON is an error too, with an empty repository to keep going.
	writeFile(t, path, "{not json")
	if _, err := LoadRepository(path); err == nil {
		t.Error("malformed JSON loaded")
	}
}

func TestSetEnabledKeepsBrokenState(t *testing.T) {
	repo := NewRepository()
	repo.Extensions["a.broken"] = Entry{ID: "a.broken", Name: "Broken", State: "broken"}
	entry, ok := repo.SetEnabled("a.broken", true)
	if !ok {
		t.Fatal("the entry was not found")
	}
	if entry.State != "broken" {
		t.Errorf("state = %q, want broken to survive enabling", entry.State)
	}
	if !entry.Enabled {
		t.Error("enabling did not set the flag")
	}
	if _, ok := repo.SetEnabled("nope", true); ok {
		t.Error("an unknown id was reported found")
	}
}

func TestInventoryJoinsRepositoryAndManifests(t *testing.T) {
	dir := t.TempDir()
	paths := FromRoot(dir)
	writeFile(t, filepath.Join(paths.Extensions, "io.github.vst93.v", manifestFileName), exampleManifest)
	// A package on disk with no repository entry.
	writeFile(t, filepath.Join(paths.Extensions, "orphan.pkg", manifestFileName), `{"id":"orphan.pkg","name":"Orphan"}`)
	// A recorded entry whose package is gone.
	writeFile(t, filepath.Join(dir, repositoryFile), `{
  "schemaVersion": 1,
  "extensions": {
    "io.github.vst93.v": {
      "id": "io.github.vst93.v", "name": "V Tools", "state": "enabled", "enabled": true,
      "packageVersion": "0.0.12", "toolVersion": "0.0.12",
      "manifestPath": "", "executablePath": "/usr/local/bin/v", "channel": "stable",
      "installedAt": 1, "updatedAt": 1, "publisherName": "vst"
    },
    "gone.tool": {
      "id": "gone.tool", "name": "Gone", "state": "enabled", "enabled": true,
      "packageVersion": "1.0.0", "manifestPath": "", "executablePath": "", "channel": "stable",
      "installedAt": 1, "updatedAt": 1
    }
  }
}`)

	inventory := LoadInventory(paths)
	if inventory.RepositoryErr != nil {
		t.Fatal(inventory.RepositoryErr)
	}
	if len(inventory.Integrations) != 2 {
		t.Fatalf("integrations = %+v", inventory.Integrations)
	}
	if got := inventory.Integrations[0]; got.Name != "Gone" || got.ManifestErr == nil {
		t.Errorf("the missing package did not report a manifest error: %+v", got)
	}
	v, ok := inventory.WithID("io.github.vst93.v")
	if !ok {
		t.Fatal("the manifest-backed integration is missing")
	}
	if v.ManifestErr != nil {
		t.Fatalf("manifest: %v", v.ManifestErr)
	}
	if v.Name != "V Tools" || v.Description == "" || v.Publisher != "vst" {
		t.Errorf("integration = %+v", v)
	}
	if !v.Running() || v.Broken() {
		t.Errorf("running/broken = %v/%v", v.Running(), v.Broken())
	}
	if got := inventory.Running(); len(got) != 1 || got[0].Entry.ID != "io.github.vst93.v" {
		t.Errorf("running = %+v", got)
	}
	if !reflect.DeepEqual(inventory.Orphans, []string{"orphan.pkg"}) {
		t.Errorf("orphans = %v", inventory.Orphans)
	}
}

func TestInventoryReportsABrokenRepository(t *testing.T) {
	dir := t.TempDir()
	paths := FromRoot(dir)
	writeFile(t, paths.RepositoryFile, `{"schemaVersion": 99}`)
	inventory := LoadInventory(paths)
	if inventory.RepositoryErr == nil {
		t.Error("a bad repository was not reported")
	}
	if len(inventory.Integrations) != 0 {
		t.Errorf("integrations = %+v", inventory.Integrations)
	}
}

func TestPathsFromRoot(t *testing.T) {
	paths := FromRoot("/tmp/floter")
	if paths.Extensions != "/tmp/floter/extensions" || paths.Data != "/tmp/floter/extension-data" {
		t.Errorf("paths = %+v", paths)
	}
	if paths.RepositoryFile != "/tmp/floter/extension-repository.json" {
		t.Errorf("repository = %q", paths.RepositoryFile)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{paths.Extensions, paths.Data, paths.Cache} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("%s was not created: %v", dir, err)
		}
	}
}
