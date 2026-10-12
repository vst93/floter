package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PlatformOverride is one entry of a manifest's `platformOverrides`: what
// changes when the same package runs on one platform target. It mirrors the
// old host's `PlatformOverride` (schema `platformOverrides`, pattern
// `darwin-aarch64`, `linux-x86_64-gnu`, …).
type PlatformOverride struct {
	// ProviderArgsPrefix replaces the provider's own argsPrefix on this
	// target.
	ProviderArgsPrefix []string `json:"providerArgsPrefix"`
	// Environment is merged over the provider's own environment; a key here
	// wins.
	Environment map[string]string `json:"environment"`
	// MinimumOSVersion is the oldest OS the target may run. It is parsed and
	// carried here; enforcing it needs an OS version the Go host does not yet
	// collect (see the docs' 具体待办).
	MinimumOSVersion string `json:"minimumOsVersion"`
}

// Platform is the device a manifest is resolved for. The names are the
// canonical ones the schema's `platforms` and `platformOverrides` spell:
// `darwin`/`linux`/`windows`, `aarch64`/`x86_64`/`armv7`, and, on Linux, the
// libc (`gnu`/`musl`/`unknown`) plus the arm ABI (`eabi`/`eabihf`/`unknown`).
type Platform struct {
	OS   string
	Arch string
	Libc string
	Abi  string
}

// CurrentPlatform is the platform this process runs on, in the same names the
// manifest uses. Go reports the OS and architecture directly; the Linux libc
// and arm ABI are read from the loader the system ships.
func CurrentPlatform() Platform {
	platform := Platform{
		OS:   canonicalOS(runtime.GOOS),
		Arch: canonicalArch(runtime.GOARCH),
	}
	if platform.OS == "linux" {
		platform.Libc = detectLibc()
		if platform.Arch == "armv7" {
			platform.Abi = detectArmABI()
		}
	}
	return platform
}

// canonicalOS maps Go's GOOS names to the manifest's.
func canonicalOS(goos string) string {
	switch goos {
	case "macos", "darwin":
		return "darwin"
	case "windows":
		return "windows"
	case "linux":
		return "linux"
	default:
		return goos
	}
}

// canonicalArch maps Go's GOARCH names to the manifest's, accepting the
// spellings the old host's `PlatformArch::parse` accepted.
func canonicalArch(goarch string) string {
	switch strings.ToLower(strings.TrimSpace(goarch)) {
	case "aarch64", "arm64":
		return "aarch64"
	case "x86_64", "amd64", "x64":
		return "x86_64"
	case "armv7", "armv7l", "armhf", "arm":
		return "armv7"
	default:
		return strings.ToLower(strings.TrimSpace(goarch))
	}
}

// detectLibc names the Linux C library: the musl loader when one is present,
// glibc when the system carries its loader, and `unknown` when neither can be
// seen (a musl rootfs that names its loader differently, say). It never
// guesses gnu for a system it cannot read.
func detectLibc() string {
	for _, dir := range []string{"/lib", "/usr/lib", "/lib64", "/usr/lib64"} {
		if matches, _ := filepath.Glob(filepath.Join(dir, "ld-musl-*.so.1")); len(matches) > 0 {
			return "musl"
		}
	}
	for _, pattern := range []string{"/lib/*-linux-gnu", "/lib64/ld-linux-*.so.*", "/lib/ld-linux-*.so.*"} {
		if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
			return "gnu"
		}
	}
	return "unknown"
}

// detectArmABI reads the arm hard-float marker; a kernel that names the
// calling convention reports eabihf, and the soft-float default is eabi.
func detectArmABI() string {
	if _, err := os.Stat("/lib/ld-linux-armhf.so.3"); err == nil {
		return "eabihf"
	}
	if _, err := os.Stat("/lib/ld-linux.so.3"); err == nil {
		return "eabi"
	}
	return "unknown"
}

// Identifier is the fully qualified target, `<os>-<arch>` plus, on Linux, the
// libc and (for armv7) the ABI — the key `platformOverrides` may use.
func (p Platform) Identifier() string {
	identifier := p.OS + "-" + p.Arch
	if p.OS != "linux" {
		return identifier
	}
	libc := p.Libc
	if libc == "" {
		libc = "unknown"
	}
	identifier += "-" + libc
	if p.Arch == "armv7" {
		abi := p.Abi
		if abi == "" {
			abi = "unknown"
		}
		identifier += "-" + abi
	}
	return identifier
}

// OSIdentifier is the OS-wide target (`darwin-any`, `linux-any`, …). A
// manifest may override it and every arch of that OS picks it up.
func (p Platform) OSIdentifier() string { return p.OS + "-any" }

// Identifiers are the keys a manifest's override may name, most specific
// first: the resolver applies them in order, so the OS-wide entry wins over
// the arch-specific one, as the old host's `override_identifiers` did.
func (p Platform) Identifiers() []string {
	return []string{p.Identifier(), p.OSIdentifier()}
}

// ValidatePlatform refuses a manifest that does not support the target. An
// empty `platforms` list is "no constraint" — the same reading the old host
// gave it.
func (m Manifest) ValidatePlatform(target Platform) error {
	if len(m.Platforms) == 0 {
		return nil
	}
	for _, supported := range m.Platforms {
		if strings.EqualFold(strings.TrimSpace(supported), target.OS) {
			return nil
		}
	}
	return fmt.Errorf("extensions: %s does not support %s", m.ID, target.OS)
}

// EffectiveProvider is the provider as it runs on one platform: the manifest's
// own provider with the matching `platformOverrides` applied — argsPrefix
// replaced, environment merged (the override wins).
func (m Manifest) EffectiveProvider(target Platform) Provider {
	provider := m.Provider
	for _, identifier := range target.Identifiers() {
		override, ok := m.PlatformOverrides[identifier]
		if !ok {
			continue
		}
		if override.ProviderArgsPrefix != nil {
			provider.ArgsPrefix = append([]string{}, override.ProviderArgsPrefix...)
		}
		if len(override.Environment) > 0 {
			merged := make(map[string]string, len(provider.Environment)+len(override.Environment))
			for key, value := range provider.Environment {
				merged[key] = value
			}
			for key, value := range override.Environment {
				merged[key] = value
			}
			provider.Environment = merged
		}
	}
	return provider
}

// Target is the platform an inventory under these paths is resolved for: the
// injected one when a test sets it, else the host.
func (p Paths) Target() Platform {
	if p.Platform.OS == "" {
		return CurrentPlatform()
	}
	return p.Platform
}
