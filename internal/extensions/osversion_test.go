package extensions

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// parseOSVersion reads the same leading dotted run the old host's
// numeric_version did: after the first digit, trailers dropped, missing
// component as zero, overflow unreadable.
func TestParseOSVersion(t *testing.T) {
	cases := []struct {
		text string
		want osVersion
		ok   bool
	}{
		{"14.5", osVersion{14, 5, 0}, true},
		{"14.5.1", osVersion{14, 5, 1}, true},
		{"6.8.0-45-generic", osVersion{6, 8, 0}, true},
		{"Microsoft Windows [Version 10.0.19045.4046]", osVersion{10, 0, 19045}, true},
		{"  v0.4.7  ", osVersion{0, 4, 7}, true},
		{"10..19045", osVersion{10, 19045, 0}, true},
		{"", osVersion{}, false},
		{"beta", osVersion{}, false},
		{"18446744073709551616", osVersion{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			got, ok := parseOSVersion(tc.text)
			if ok != tc.ok {
				t.Fatalf("parseOSVersion(%q) ok = %v, want %v", tc.text, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("parseOSVersion(%q) = %+v, want %+v", tc.text, got, tc.want)
			}
		})
	}
}

// The floor comes from the last matched override that names one: the walk is
// most specific first, so an OS-wide floor wins, and an override without a
// floor does not erase an earlier one.
func TestMinimumOSVersionFollowsTheOverrideOrder(t *testing.T) {
	linux := Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}
	manifest := Manifest{
		PlatformOverrides: map[string]PlatformOverride{
			"linux-x86_64-gnu": {MinimumOSVersion: "5.0"},
			"linux-any":        {MinimumOSVersion: "6.8"},
		},
	}
	if got := manifest.MinimumOSVersion(linux); got != "6.8" {
		t.Errorf("an OS-wide floor should win: got %q", got)
	}

	manifest.PlatformOverrides = map[string]PlatformOverride{
		"linux-x86_64-gnu": {MinimumOSVersion: "5.0"},
		"linux-any":        {ProviderArgsPrefix: []string{"--linux"}},
	}
	if got := manifest.MinimumOSVersion(linux); got != "5.0" {
		t.Errorf("an override without a floor should keep the earlier one: got %q", got)
	}

	if got := (Manifest{}).MinimumOSVersion(linux); got != "" {
		t.Errorf("a manifest without overrides has no floor: %q", got)
	}
}

// The install-time gate: below the floor refuses, at or above passes, an
// unreadable host is no refusal, an unreadable floor is a refusal, and a
// target with no matching override is never checked.
func TestValidateMinimumOSVersion(t *testing.T) {
	linux := Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}
	cases := []struct {
		name      string
		overrides map[string]PlatformOverride
		target    Platform
		host      string
		wantErr   bool
	}{
		{"below the floor", map[string]PlatformOverride{"linux-any": {MinimumOSVersion: "6.8"}}, linux, "5.15.0", true},
		{"equal to the floor", map[string]PlatformOverride{"linux-any": {MinimumOSVersion: "6.8"}}, linux, "6.8.0", false},
		{"above the floor", map[string]PlatformOverride{"linux-any": {MinimumOSVersion: "6.8"}}, linux, "6.8.1-arch1", false},
		{"missing patch reads as zero", map[string]PlatformOverride{"darwin-any": {MinimumOSVersion: "14.5"}}, Platform{OS: "darwin"}, "14.5.0", false},
		{"no declaration", nil, linux, "0.1", false},
		{"another platform's floor is not this host's", map[string]PlatformOverride{"darwin-any": {MinimumOSVersion: "99"}}, linux, "5.15", false},
		{"an unreadable host is no refusal", map[string]PlatformOverride{"linux-any": {MinimumOSVersion: "99"}}, linux, "", false},
		{"an unreadable floor is a refusal", map[string]PlatformOverride{"linux-any": {MinimumOSVersion: "soon"}}, linux, "5.15", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := Manifest{ID: "dev.floter.fixture", PlatformOverrides: tc.overrides}
			err := manifest.ValidateMinimumOSVersion(tc.target, tc.host)
			if tc.wantErr != (err != nil) {
				t.Fatalf("ValidateMinimumOSVersion(%+v, %q) = %v, wantErr %v", tc.target, tc.host, err, tc.wantErr)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), manifest.ID) {
				t.Errorf("the refusal does not name the extension: %v", err)
			}
		})
	}
}

// The install path refuses a package whose override demands a newer OS than
// the host runs, before it touches the disk — the old host's
// `install.rs::validate_minimum_os_version`.
func TestPrepareRefusesAnOsBelowTheOverrideFloor(t *testing.T) {
	paths := FromRoot(t.TempDir())
	paths.Platform = Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}
	paths.OSVersion = "5.15.0-1-generic"
	pkg := osFloorFixture(t, "dev.floter.old-kernel", `{"linux-any": {"minimumOsVersion": "6.8"}}`)

	_, err := PrepareLocal(paths, pkg)
	if err == nil {
		t.Fatal("PrepareLocal accepted a package requiring linux 6.8 on a 5.15 host")
	}
	if !strings.Contains(err.Error(), "requires linux 6.8") {
		t.Errorf("error = %v", err)
	}
	if entries, err := os.ReadDir(paths.Extensions); err == nil && len(entries) != 0 {
		t.Errorf("the refused install left files behind: %v", entries)
	}
}

// The regression guard: a host that meets the floor installs normally, so the
// check does not refuse everything.
func TestAMatchingOsVersionLetsACompatibleManifestThrough(t *testing.T) {
	paths := FromRoot(t.TempDir())
	paths.Platform = Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}
	paths.OSVersion = "6.8.0-45-generic"
	pkg := osFloorFixture(t, "dev.floter.new-kernel", `{"linux-any": {"minimumOsVersion": "6.8"}}`)

	entry, err := InstallLocal(paths, pkg)
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID != "dev.floter.new-kernel" {
		t.Errorf("entry = %+v", entry)
	}
}

// osFloorFixture writes a local package whose manifest carries the given
// `platformOverrides` block, verbatim.
func osFloorFixture(t *testing.T, id, overrides string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's runtime is a shell script")
	}
	dir := t.TempDir()
	pkg := filepath.Join(dir, id)
	writeFile(t, filepath.Join(pkg, manifestFileName), `{
  "schemaVersion": "2.0", "id": "`+id+`", "name": "Floor", "version": "1.0.0",
  "publisher": {"id": "vst93", "name": "vst"},
  "distribution": {"type": "local"},
  "runtime": {"type": "script", "language": "shell", "path": "floor-tool"},
  "provider": {"type": "executable", "argsPrefix": []},
  "platformOverrides": `+overrides+`
}`)
	tool := filepath.Join(pkg, "floor-tool")
	writeFile(t, tool, "#!/bin/sh\necho floor\n")
	if err := os.Chmod(tool, 0o755); err != nil {
		t.Fatal(err)
	}
	return pkg
}
