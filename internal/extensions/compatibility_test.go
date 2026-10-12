package extensions

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// compatibilityFixture writes a package whose manifest carries the given
// compatibility block, verbatim.
func compatibilityFixture(t *testing.T, id, compatibility string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's runtime is a shell script")
	}
	dir := t.TempDir()
	pkg := filepath.Join(dir, id)
	writeFile(t, filepath.Join(pkg, manifestFileName), `{
  "schemaVersion": "2.0", "id": "`+id+`", "name": "Compat", "version": "1.0.0",
  "publisher": {"id": "vst93", "name": "vst"},
  `+compatibility+`,
  "distribution": {"type": "local"},
  "runtime": {"type": "script", "language": "shell", "path": "compat-tool"},
  "provider": {"type": "executable", "argsPrefix": []}
}`)
	tool := filepath.Join(pkg, "compat-tool")
	writeFile(t, tool, "#!/bin/sh\necho compat\n")
	if err := os.Chmod(tool, 0o755); err != nil {
		t.Fatal(err)
	}
	return pkg
}

// TestValidateCompatibility pins the two ranges the old host checked at
// install: the Floter version and the provider protocol. A blank range is no
// constraint; a host that cannot name itself checks nothing at all.
func TestValidateCompatibility(t *testing.T) {
	cases := []struct {
		name     string
		host     string
		floter   string
		protocol string
		wantErr  bool
	}{
		{"in range", "0.3.14", ">=0.3.0", "^1.0", false},
		{"above the range", "0.3.14", ">=0.4.0", "^1.0", true},
		{"below the range", "0.3.14", "^0.2.0", "^1.0", true},
		{"intersection", "0.3.14", ">=0.3.0 <0.4.0", "^1.0", false},
		{"alternatives", "0.3.14", "^0.2.0 || ^0.3.0", "^1.0", false},
		{"protocol too new", "0.3.14", ">=0.3.0", "^2.0", true},
		{"protocol range", "0.3.14", ">=0.3.0", ">=1.0.0", false},
		{"no ranges", "0.3.14", "", "", false},
		{"no host version", "", ">=99.0.0", "^9.0", false},
		{"unparseable host", "dev", ">=99.0.0", "^9.0", false},
		{"preview host matches its release", "0.4.0-preview", ">=0.4.0", "^1.0", false},
		{"unparseable range never matches", "0.3.14", "not-a-range", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := Manifest{ID: "dev.floter.fixture"}
			manifest.Compatibility = Compatibility{Floter: tc.floter, ProviderProtocol: tc.protocol}
			err := manifest.ValidateCompatibility(tc.host)
			if tc.wantErr != (err != nil) {
				t.Fatalf("ValidateCompatibility(%q, floter=%q, protocol=%q) = %v, wantErr %v",
					tc.host, tc.floter, tc.protocol, err, tc.wantErr)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), manifest.ID) {
				t.Errorf("error does not name the extension: %v", err)
			}
		})
	}
}

// TestPrepareRefusesAManifestBuiltForAnotherRelease is the install-time
// guard: a package that names a future Floter never reaches the disk.
func TestPrepareRefusesAManifestBuiltForAnotherRelease(t *testing.T) {
	paths := FromRoot(t.TempDir())
	paths.HostVersion = "0.3.14"
	pkg := compatibilityFixture(t, "dev.floter.future", `"compatibility": {"floter": ">=0.4.0", "providerProtocol": "^1.0"}`)

	_, err := PrepareLocal(paths, pkg)
	if err == nil {
		t.Fatal("PrepareLocal accepted a manifest requiring Floter >=0.4.0")
	}
	if !strings.Contains(err.Error(), "requires Floter >=0.4.0") {
		t.Errorf("error = %v", err)
	}
	// The staging step happens after the check, so nothing was written.
	if entries, err := os.ReadDir(paths.Extensions); err == nil && len(entries) != 0 {
		t.Errorf("the refused install left files behind: %v", entries)
	}
}

// TestInstallLocalRefusesAnIncompatibleProtocol covers the second range: a
// package speaking another provider protocol is refused too.
func TestInstallLocalRefusesAnIncompatibleProtocol(t *testing.T) {
	paths := FromRoot(t.TempDir())
	paths.HostVersion = "0.3.14"
	pkg := compatibilityFixture(t, "dev.floter.proto", `"compatibility": {"floter": ">=0.3.0", "providerProtocol": "^2.0"}`)

	_, err := InstallLocal(paths, pkg)
	if err == nil {
		t.Fatal("InstallLocal accepted a manifest requiring provider protocol ^2.0")
	}
	if !strings.Contains(err.Error(), "provider protocol") {
		t.Errorf("error = %v", err)
	}
}

// TestAHostVersionLetsACompatibleManifestThrough is the regression guard:
// setting the host version must not refuse a package that fits it.
func TestAHostVersionLetsACompatibleManifestThrough(t *testing.T) {
	paths := FromRoot(t.TempDir())
	paths.HostVersion = "0.3.14"
	pkg := compatibilityFixture(t, "dev.floter.ok", `"compatibility": {"floter": ">=0.3.0", "providerProtocol": "^1.0"}`)

	entry, err := InstallLocal(paths, pkg)
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID != "dev.floter.ok" {
		t.Errorf("entry = %+v", entry)
	}
}
