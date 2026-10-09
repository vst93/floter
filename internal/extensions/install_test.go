package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// packageFixture writes a package directory with a manifest whose runtime is
// an executable inside it.
func packageFixture(t *testing.T, id, version string, extraManifest string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's runtime is a shell script")
	}
	dir := t.TempDir()
	pkg := filepath.Join(dir, id)
	writeFile(t, filepath.Join(pkg, manifestFileName), `{
  "schemaVersion": "2.0", "id": "`+id+`", "name": "Fixture", "version": "`+version+`",
  "publisher": {"id": "vst93", "name": "vst"},
  "compatibility": {"floter": ">=0.3.0", "providerProtocol": "^1.0"},
  "distribution": {"type": "local"},
  "runtime": {"type": "script", "language": "shell", "path": "fixture-tool"},
  "provider": {"type": "executable", "argsPrefix": []}`+extraManifest+`
}`)
	tool := filepath.Join(pkg, "fixture-tool")
	writeFile(t, tool, "#!/bin/sh\necho fixture\n")
	if err := os.Chmod(tool, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(pkg, "template.conf"), "configured=yes\n")
	return pkg
}

func TestInstallLocal(t *testing.T) {
	paths := FromRoot(t.TempDir())
	pkg := packageFixture(t, "dev.floter.fixture", "1.0.0", `, "lifecycle": {"configurationTemplates": [{"source": "template.conf", "target": "template.conf"}]}`)

	entry, err := InstallLocal(paths, pkg)
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID != "dev.floter.fixture" || entry.PackageVersion != "1.0.0" {
		t.Errorf("entry = %+v", entry)
	}
	if entry.DistributionSource != "local" || entry.RuntimeOwnership != "bundled" || entry.ProviderKind != "executable" {
		t.Errorf("entry = %+v", entry)
	}
	if !entry.Enabled || entry.State != "enabled" {
		t.Errorf("a fresh install is not enabled: %+v", entry)
	}
	if entry.InstalledAt == 0 || entry.UpdatedAt == 0 {
		t.Errorf("timestamps = %d/%d", entry.InstalledAt, entry.UpdatedAt)
	}

	// The package is in place, the record points at it, and the template
	// landed in the data directory.
	installed := filepath.Join(paths.Extensions, "dev.floter.fixture")
	if _, err := os.Stat(filepath.Join(installed, manifestFileName)); err != nil {
		t.Errorf("the package is not installed: %v", err)
	}
	if entry.ManifestPath != filepath.Join(installed, manifestFileName) {
		t.Errorf("manifestPath = %q", entry.ManifestPath)
	}
	// A script runtime: the interpreter is the program, the package is the
	// runtime root, and the script itself is an argument.
	if filepath.Base(entry.ExecutablePath) != "sh" {
		t.Errorf("executablePath = %q, want the interpreter", entry.ExecutablePath)
	}
	if entry.RuntimeRoot == nil || *entry.RuntimeRoot != installed {
		t.Errorf("runtimeRoot = %v, want %q", entry.RuntimeRoot, installed)
	}
	if data, err := os.ReadFile(filepath.Join(paths.Data, "dev.floter.fixture", "template.conf")); err != nil || string(data) != "configured=yes\n" {
		t.Errorf("template = %q, %v", data, err)
	}
	// No staging or backup litter is left behind.
	leftovers, err := os.ReadDir(paths.Extensions)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range leftovers {
		if dir.Name() != "dev.floter.fixture" {
			t.Errorf("leftover %q", dir.Name())
		}
	}

	// The inventory now sees it.
	inventory := LoadInventory(paths)
	if len(inventory.Integrations) != 1 || inventory.Integrations[0].Name != "Fixture" {
		t.Errorf("inventory = %+v", inventory.Integrations)
	}
}

func TestInstallLocalRejectsBadPackages(t *testing.T) {
	paths := FromRoot(t.TempDir())

	if _, err := InstallLocal(paths, filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("a missing package installed")
	}

	empty := t.TempDir()
	if _, err := InstallLocal(paths, empty); err == nil {
		t.Error("a package with no manifest installed")
	}

	bad := filepath.Join(t.TempDir(), "bad")
	writeFile(t, filepath.Join(bad, manifestFileName), `{"id": "../escape", "name": "Bad"}`)
	if _, err := InstallLocal(paths, bad); err == nil {
		t.Error("a package with an unsafe id installed")
	}

	// A runtime that cannot be found is refused before anything is written.
	missingRuntime := filepath.Join(t.TempDir(), "noruntime")
	writeFile(t, filepath.Join(missingRuntime, manifestFileName), `{
  "schemaVersion": "2.0", "id": "dev.floter.noruntime", "name": "No Runtime",
  "runtime": {"type": "system", "executableNames": ["definitely-not-installed-xyz"]},
  "provider": {"type": "executable", "argsPrefix": []}
}`)
	if _, err := InstallLocal(paths, missingRuntime); err == nil {
		t.Error("a package whose runtime is missing installed")
	}
	if _, err := os.Stat(filepath.Join(paths.Extensions, "dev.floter.noruntime")); !os.IsNotExist(err) {
		t.Error("a refused install left a package behind")
	}
}

func TestInstallLocalUpdateKeepsTheAuditTrail(t *testing.T) {
	paths := FromRoot(t.TempDir())
	pkg := packageFixture(t, "dev.floter.fixture", "1.0.0", "")
	if _, err := InstallLocal(paths, pkg); err != nil {
		t.Fatal(err)
	}

	// An approval and a stale error marker the update must not drop.
	digest := "sha256-abc"
	if err := SetToolVersion(paths, "dev.floter.fixture", "0.0.9"); err != nil {
		t.Fatal(err)
	}
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		t.Fatal(err)
	}
	entry := repo.Extensions["dev.floter.fixture"]
	entry.ApprovedPermissions = []string{"filesystem-read"}
	entry.ApprovedManifestDigest = &digest
	entry.Channel = "beta"
	entry.LastErrorCode = pointer("probe-failed")
	entry.State = "broken"
	entry.BrokenReason = pointer("the tool vanished")
	repo.Extensions["dev.floter.fixture"] = entry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		t.Fatal(err)
	}

	// The new package lives elsewhere, which is how an update arrives.
	updated := packageFixture(t, "dev.floter.fixture", "2.0.0", "")
	if _, err := InstallLocal(paths, updated); err != nil {
		t.Fatal(err)
	}

	repo, err = LoadRepository(paths.RepositoryFile)
	if err != nil {
		t.Fatal(err)
	}
	entry = repo.Extensions["dev.floter.fixture"]
	if entry.PackageVersion != "2.0.0" || entry.CurrentVersion != "2.0.0" {
		t.Errorf("version = %q/%q", entry.PackageVersion, entry.CurrentVersion)
	}
	if entry.PreviousVersion == nil || *entry.PreviousVersion != "1.0.0" {
		t.Errorf("previousVersion = %v", entry.PreviousVersion)
	}
	if len(entry.ApprovedPermissions) != 1 || entry.ApprovedManifestDigest == nil {
		t.Errorf("the approval was dropped: %+v", entry)
	}
	if entry.Channel != "beta" {
		t.Errorf("channel = %q", entry.Channel)
	}
	if entry.State != "broken" || entry.LastErrorCode == nil {
		t.Errorf("the broken state was dropped: %+v", entry)
	}
	if entry.ToolVersion != nil && *entry.ToolVersion != "0.0.9" {
		t.Errorf("tool version = %v", entry.ToolVersion)
	}
	if entry.InstalledAt > entry.UpdatedAt {
		t.Errorf("timestamps = %d > %d", entry.InstalledAt, entry.UpdatedAt)
	}
}

func TestUninstall(t *testing.T) {
	paths := FromRoot(t.TempDir())
	pkg := packageFixture(t, "dev.floter.fixture", "1.0.0", "")
	if _, err := InstallLocal(paths, pkg); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(paths.Data, "dev.floter.fixture")
	writeFile(t, filepath.Join(dataDir, "state.json"), "{}")

	// Keeping the data is the default.
	if err := Uninstall(paths, "dev.floter.fixture", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.Extensions, "dev.floter.fixture")); !os.IsNotExist(err) {
		t.Error("the package survived")
	}
	if _, err := os.Stat(dataDir); err != nil {
		t.Errorf("the data was removed without being asked: %v", err)
	}
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.Extensions) != 0 {
		t.Errorf("the record survived: %+v", repo.Extensions)
	}
	if err := Uninstall(paths, "dev.floter.fixture", false); !errors.Is(err, ErrNoIntegration) {
		t.Errorf("uninstalling twice = %v", err)
	}
	if err := Uninstall(paths, "../escape", true); err == nil {
		t.Error("an unsafe id was uninstalled")
	}

	// A second install, removed with its data.
	if _, err := InstallLocal(paths, pkg); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(paths, "dev.floter.fixture", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Error("the data was not removed when asked")
	}
}

func TestManifestDigest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, manifestFileName)
	writeFile(t, path, exampleManifest)
	got, err := ManifestDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len("sha256-")+64 || got[:7] != "sha256-" {
		t.Errorf("digest = %q", got)
	}
	// The same bytes give the same digest; different bytes do not.
	again, _ := ManifestDigest(path)
	if again != got {
		t.Errorf("digest changed: %q vs %q", got, again)
	}
	writeFile(t, path, exampleManifest+"\n")
	other, _ := ManifestDigest(path)
	if other == got {
		t.Error("a changed manifest kept its digest")
	}
}

func TestValidID(t *testing.T) {
	for _, id := range []string{"io.github.vst93.v", "local.git", "a.b", "dev.floter.static"} {
		if err := validID(id); err != nil {
			t.Errorf("validID(%q) = %v", id, err)
		}
	}
	for _, id := range []string{"", "..", "../x", "/abs", ".hidden", "a/b", "UPPER", "a..b/.."} {
		if err := validID(id); err == nil {
			t.Errorf("validID(%q) accepted", id)
		}
	}
}

func pointer[T any](value T) *T { return &value }

func TestInstallLocalWithASystemRuntime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's interpreter is sh")
	}
	paths := FromRoot(t.TempDir())
	dir := t.TempDir()
	pkg := filepath.Join(dir, "dev.floter.system")
	writeFile(t, filepath.Join(pkg, manifestFileName), `{
  "schemaVersion": "2.0", "id": "dev.floter.system", "name": "System Tool", "version": "3.1.4",
  "runtime": {"type": "system", "executableNames": ["sh"]},
  "provider": {"type": "static-descriptor", "descriptor": "description.json", "argsPrefix": []}
}`)
	writeFile(t, filepath.Join(pkg, "description.json"), `{
  "protocolVersion": "1.0",
  "provider": {"id": "dev.floter.system", "name": "System Tool", "version": "3.1.4"},
  "commands": []
}`)

	entry, err := InstallLocal(paths, pkg)
	if err != nil {
		t.Fatal(err)
	}
	if entry.RuntimeOwnership != "system" || entry.ProviderKind != "static-descriptor" {
		t.Errorf("entry = %+v", entry)
	}
	if entry.RuntimeRoot != nil {
		t.Errorf("a system runtime has a root: %v", entry.RuntimeRoot)
	}
	if entry.ExecutablePath == "" || filepath.Base(entry.ExecutablePath) != "sh" {
		t.Errorf("executablePath = %q", entry.ExecutablePath)
	}
	// The static descriptor resolves from the installed copy.
	integration := Integration{Entry: entry, Paths: paths, Manifest: Manifest{
		ID:       entry.ID,
		Provider: Provider{Type: "static-descriptor", Descriptor: "description.json"},
		Runtime:  Runtime{Type: "system", ExecutableNames: []string{"sh"}},
	}}
	description, err := Describe(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	if description.Provider.Version != "3.1.4" {
		t.Errorf("provider = %+v", description.Provider)
	}
}
