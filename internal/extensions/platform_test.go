//go:build !windows

package extensions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ValidatePlatform answers the one question the schema's `platforms` asks: may
// this host run the package? An empty list is no constraint.
func TestManifestPlatformsGateTheHost(t *testing.T) {
	linux := Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}
	cases := []struct {
		name      string
		platforms []string
		target    Platform
		refused   bool
	}{
		{"no list is no constraint", nil, linux, false},
		{"an empty list is no constraint", []string{}, linux, false},
		{"the host is named", []string{"linux"}, linux, false},
		{"one of several names is the host", []string{"darwin", "linux"}, linux, false},
		{"case does not matter", []string{"Linux"}, linux, false},
		{"another OS is refused", []string{"darwin"}, linux, true},
		{"windows is refused on darwin", []string{"windows"}, Platform{OS: "darwin", Arch: "aarch64"}, true},
		{"linux is refused on windows", []string{"linux"}, Platform{OS: "windows", Arch: "x86_64"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := Manifest{ID: "dev.floter.tool", Platforms: tc.platforms}
			err := manifest.ValidatePlatform(tc.target)
			if tc.refused && err == nil {
				t.Fatalf("platforms %v with target %s was accepted", tc.platforms, tc.target.OS)
			}
			if !tc.refused && err != nil {
				t.Fatalf("platforms %v with target %s was refused: %v", tc.platforms, tc.target.OS, err)
			}
			if err != nil && !strings.Contains(err.Error(), "does not support") {
				t.Errorf("the refusal does not say what it is: %v", err)
			}
		})
	}
}

// The identifiers an override may name, most specific first: the full target
// then the OS-wide one, exactly the old host's `override_identifiers`.
func TestPlatformIdentifiers(t *testing.T) {
	cases := []struct {
		platform Platform
		want     []string
	}{
		{
			Platform{OS: "darwin", Arch: "aarch64"},
			[]string{"darwin-aarch64", "darwin-any"},
		},
		{
			Platform{OS: "windows", Arch: "x86_64"},
			[]string{"windows-x86_64", "windows-any"},
		},
		{
			Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"},
			[]string{"linux-x86_64-gnu", "linux-any"},
		},
		{
			// Missing libc/abi read as `unknown`, never dropped.
			Platform{OS: "linux", Arch: "x86_64"},
			[]string{"linux-x86_64-unknown", "linux-any"},
		},
		{
			Platform{OS: "linux", Arch: "armv7", Libc: "musl", Abi: "eabihf"},
			[]string{"linux-armv7-musl-eabihf", "linux-any"},
		},
	}
	for _, tc := range cases {
		got := tc.platform.Identifiers()
		if len(got) != len(tc.want) {
			t.Fatalf("%+v identifiers = %v, want %v", tc.platform, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%+v identifiers[%d] = %q, want %q", tc.platform, i, got[i], tc.want[i])
			}
		}
	}
}

// A platform override replaces the argsPrefix and merges the environment, and
// the OS-wide entry is applied after the full target so it wins.
func TestPlatformOverridesResolveToTheProvider(t *testing.T) {
	manifest := Manifest{
		Provider: Provider{
			ArgsPrefix:  []string{"--base"},
			Environment: map[string]string{"KEEP": "1", "REPLACE": "base"},
		},
		PlatformOverrides: map[string]PlatformOverride{
			"linux-x86_64-gnu": {
				ProviderArgsPrefix: []string{"--arch"},
				Environment:        map[string]string{"REPLACE": "arch", "ARCH_ONLY": "yes"},
			},
			"linux-any": {
				ProviderArgsPrefix: []string{"--linux"},
				Environment:        map[string]string{"REPLACE": "linux", "OS_ONLY": "yes"},
			},
			"darwin-any": {
				ProviderArgsPrefix: []string{"--darwin"},
			},
		},
	}
	provider := manifest.EffectiveProvider(Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"})
	if got := strings.Join(provider.ArgsPrefix, " "); got != "--linux" {
		t.Errorf("argsPrefix = %q, want the OS-wide --linux (applied last)", got)
	}
	if provider.Environment["KEEP"] != "1" {
		t.Errorf("an untouched key was lost: %v", provider.Environment)
	}
	if provider.Environment["REPLACE"] != "linux" {
		t.Errorf("REPLACE = %q, want the OS-wide linux", provider.Environment["REPLACE"])
	}
	if provider.Environment["ARCH_ONLY"] != "yes" || provider.Environment["OS_ONLY"] != "yes" {
		t.Errorf("an override key was lost: %v", provider.Environment)
	}
	// The manifest's own provider is not mutated by resolving.
	if len(manifest.Provider.ArgsPrefix) != 1 || manifest.Provider.ArgsPrefix[0] != "--base" {
		t.Errorf("the manifest provider was mutated: %v", manifest.Provider.ArgsPrefix)
	}
	if manifest.Provider.Environment["REPLACE"] != "base" {
		t.Errorf("the manifest environment was mutated: %v", manifest.Provider.Environment)
	}

	// A platform with no matching override keeps the base provider.
	untouched := manifest.EffectiveProvider(Platform{OS: "windows", Arch: "x86_64"})
	if strings.Join(untouched.ArgsPrefix, " ") != "--base" || untouched.Environment["REPLACE"] != "base" {
		t.Errorf("windows took an override: %+v", untouched)
	}
}

// The provider protocol reads the resolved provider: a manifest's override
// reaches the argv and the environment the describe call uses.
func TestProtocolArgsReadThePlatformOverride(t *testing.T) {
	manifest := Manifest{
		Provider: Provider{
			ArgsPrefix:  []string{"--base"},
			Environment: map[string]string{"FLOTER_PLATFORM": "base"},
		},
	}
	args := ProtocolArgs(RuntimeBinding{Program: "tool"}, manifest, OpDescribe)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--base") {
		t.Fatalf("the base argsPrefix is missing: %v", args)
	}
	// The override is applied for the host this test runs on; on Linux a
	// matching `linux-any` must be visible.
	if CurrentPlatform().OS == "linux" {
		manifest.PlatformOverrides = map[string]PlatformOverride{
			"linux-any": {
				ProviderArgsPrefix: []string{"--linux"},
				Environment:        map[string]string{"FLOTER_PLATFORM": "linux"},
			},
		}
		args = ProtocolArgs(RuntimeBinding{Program: "tool"}, manifest, OpDescribe)
		if strings.Join(args, " ") != "--linux describe --protocol 1.0" {
			t.Errorf("resolved argv = %v", args)
		}
		env := providerEnv(manifest)
		if !containsEnv(env, "FLOTER_PLATFORM=linux") || containsEnv(env, "FLOTER_PLATFORM=base") {
			t.Errorf("resolved env has the wrong FLOTER_PLATFORM: %v", env)
		}
	}
}

func containsEnv(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}

// An installed package that names another platform stops being runnable and
// says why; the injected target makes the test independent of the CI host.
func TestInventoryRefusesAnUnsupportedPlatform(t *testing.T) {
	paths, _ := inventoryWithPlatforms(t, `"platforms": ["linux"]`)

	paths.Platform = Platform{OS: "darwin", Arch: "aarch64"}
	integration, ok := LoadInventory(paths).WithID("dev.floter.tool")
	if !ok {
		t.Fatal("the integration vanished instead of being marked")
	}
	if integration.ManifestErr == nil {
		t.Fatal("a darwin-only package is runnable on darwin? no: it names linux")
	}
	if integration.Running() {
		t.Error("an unsupported package reports as running")
	}
	if !strings.Contains(integration.ManifestErr.Error(), "does not support darwin") {
		t.Errorf("the reason does not say what it is: %v", integration.ManifestErr)
	}

	// The same package on the platform it names runs.
	paths.Platform = Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}
	integration, _ = LoadInventory(paths).WithID("dev.floter.tool")
	if integration.ManifestErr != nil {
		t.Fatalf("a supported package was refused: %v", integration.ManifestErr)
	}
	if !integration.Running() {
		t.Error("a supported package does not report as running")
	}
}

// A manifest with no `platforms` is unchanged: the constraint is opt-in.
func TestInventoryKeepsAnUnconstrainedManifest(t *testing.T) {
	paths, _ := inventoryWithPlatforms(t, "")
	paths.Platform = Platform{OS: "windows", Arch: "x86_64"}
	integration, ok := LoadInventory(paths).WithID("dev.floter.tool")
	if !ok || integration.ManifestErr != nil {
		t.Fatalf("an unconstrained manifest was refused: ok=%v err=%v", ok, integration.ManifestErr)
	}
}

// The install path refuses a package built for another OS before it touches
// the disk, as the old host's `resolve` did in `install`.
func TestInstallRefusesAnUnsupportedPlatform(t *testing.T) {
	paths, pkg := inventoryWithPlatforms(t, `"platforms": ["darwin"]`)
	paths.Platform = Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}
	_, err := PrepareLocal(paths, pkg)
	if err == nil {
		t.Fatal("installing a darwin-only package on linux was accepted")
	}
	if !strings.Contains(err.Error(), "does not support linux") {
		t.Errorf("the refusal does not say what it is: %v", err)
	}
}

// inventoryWithPlatforms writes a minimal installed integration whose
// manifest carries the given extra fields (empty for none).
func inventoryWithPlatforms(t *testing.T, extra string) (Paths, string) {
	t.Helper()
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(paths.Extensions, "dev.floter.tool", "integration")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
	  "schemaVersion": "2.0", "id": "dev.floter.tool", "name": "Tool",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "system", "executableNames": ["tool"]},
	  "provider": {"type": "static-descriptor", "descriptor": "provider-description.json"}`
	if extra != "" {
		manifest += "," + extra
	}
	manifest += `
	}`
	if err := os.WriteFile(filepath.Join(pkg, manifestFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "provider-description.json"), []byte(`{
	  "protocolVersion": "1.0",
	  "provider": {"id": "dev.floter.tool", "name": "Tool", "version": "1.0.0"},
	  "commands": [{"id": "tool", "name": "Tool", "description": "root",
	    "execution": {"program": "self", "argsPrefix": [], "mode": "pty"}}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	repository := `{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.tool": {"id": "dev.floter.tool", "name": "Tool",
	    "publisherId": "local-user", "providerKind": "static-descriptor",
	    "state": "enabled", "enabled": true,
	    "manifestPath": "` + jsonEscaped(filepath.Join(pkg, manifestFileName)) + `",
	    "installedAt": 1, "updatedAt": 1}}
	}`
	if err := os.WriteFile(paths.RepositoryFile, []byte(repository), 0o644); err != nil {
		t.Fatal(err)
	}
	return paths, pkg
}
