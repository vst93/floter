package extensions

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// HostOSVersion reads the running OS's version in the shape a manifest's
// `platformOverrides[].minimumOsVersion` compares against, mirroring the old
// host's `current_os_version`: macOS's `sw_vers -productVersion` ("14.5"),
// Linux's `uname -r` ("6.8.0-45-generic"), Windows's kernel version
// ("10.0.19045"). An empty string means it could not be read, which is "no
// basis to refuse" — the same latitude ValidateCompatibility gives an unnamed
// build.
func HostOSVersion() string {
	switch runtime.GOOS {
	case "darwin":
		return commandVersion("sw_vers", "-productVersion")
	case "linux":
		return commandVersion("uname", "-r")
	case "windows":
		// `ver` prints "Microsoft Windows [Version 10.0.19045.4046]"; the
		// numeric reader takes the dotted run that follows the first digit.
		return commandVersion("cmd", "/c", "ver")
	default:
		return ""
	}
}

// commandVersion runs a program and returns its trimmed stdout, or an empty
// string when it cannot be run or fails — a version we cannot read, never a
// guessed one.
//
// The command goes through the same constructor as every other extension
// process, so it is in its own process group and the group is swept when it
// exits. No timeout is added: the reference's system-version read has none
// either, and this is a read-only `sw_vers`/`uname -r`/`ver` call.
func commandVersion(program string, args ...string) string {
	cmd := newExtensionCommand(context.Background(), program, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := runExtensionCommand(cmd); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// ValidateMinimumOSVersion refuses a manifest whose matched platform override
// demands a newer OS than the host runs, at the same install-time seam as
// ValidatePlatform (the old host's `install.rs` resolved and called
// `validate_minimum_os_version` before touching the disk). It is deliberately
// not checked on load/refresh: an already-installed package does not break
// because the host moved.
//
// A blank floor is no constraint. A floor we cannot read as a version is a
// refusal: the manifest asked for something, and we cannot honour it. A host
// version we cannot read is not a refusal.
func (m Manifest) ValidateMinimumOSVersion(target Platform, hostVersion string) error {
	minimum := strings.TrimSpace(m.MinimumOSVersion(target))
	if minimum == "" {
		return nil
	}
	required, ok := parseOSVersion(minimum)
	if !ok {
		return fmt.Errorf("extensions: %s declares an invalid minimum OS version %q", m.ID, minimum)
	}
	current, ok := parseOSVersion(strings.TrimSpace(hostVersion))
	if !ok {
		return nil
	}
	if current.less(required) {
		return fmt.Errorf("extensions: %s requires %s %s or newer; host is %s", m.ID, target.OS, minimum, strings.TrimSpace(hostVersion))
	}
	return nil
}

// osVersion is the numeric shape a floor and an OS version string are compared
// as.
type osVersion struct{ major, minor, patch uint64 }

func (v osVersion) less(other osVersion) bool {
	if v.major != other.major {
		return v.major < other.major
	}
	if v.minor != other.minor {
		return v.minor < other.minor
	}
	return v.patch < other.patch
}

// parseOSVersion reads the leading dotted numeric run that follows the first
// digit, dropping trailers — the old host's `numeric_version`: "6.8.0-45-generic"
// is 6.8.0, "Microsoft Windows [Version 10.0.19045.4046]" is 10.0.19045. Missing
// minor/patch components read as zero, so "14.5" equals "14.5.0". A string with
// no digit, or a component that overflows, is unreadable.
func parseOSVersion(text string) (osVersion, bool) {
	start := strings.IndexFunc(text, func(character rune) bool {
		return character >= '0' && character <= '9'
	})
	if start < 0 {
		return osVersion{}, false
	}
	var numeric strings.Builder
	for _, character := range text[start:] {
		if (character >= '0' && character <= '9') || character == '.' {
			numeric.WriteRune(character)
			continue
		}
		break
	}
	var components []uint64
	for _, component := range strings.Split(numeric.String(), ".") {
		if component == "" {
			continue
		}
		value, err := strconv.ParseUint(component, 10, 64)
		if err != nil {
			return osVersion{}, false
		}
		components = append(components, value)
	}
	if len(components) == 0 {
		return osVersion{}, false
	}
	version := osVersion{major: components[0]}
	if len(components) > 1 {
		version.minor = components[1]
	}
	if len(components) > 2 {
		version.patch = components[2]
	}
	return version, true
}
