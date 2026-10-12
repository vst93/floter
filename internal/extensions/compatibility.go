package extensions

import (
	"fmt"
	"strings"
)

// ValidateCompatibility checks a manifest's declared ranges against this
// host, the way the old host's `ExtensionManifest::validate_compatibility`
// did at every install and pre-install review. Two ranges are read:
//
//   - `compatibility.floter`: the Floter versions the package runs under.
//   - `compatibility.providerProtocol`: the provider protocol it speaks.
//
// A blank range is no constraint, and a host version that cannot be parsed
// (a build running with no version, or a hand-built one) is left unchecked:
// a build that cannot name itself has no basis to refuse a package. The old
// host rejected an unparseable requirement; here it simply never matches,
// which is the same refusal with a smaller blast radius.
func (m Manifest) ValidateCompatibility(hostVersion string) error {
	host, ok := parseHostVersion(hostVersion)
	if !ok {
		return nil
	}
	if requirement := strings.TrimSpace(m.Compatibility.Floter); requirement != "" {
		if !Satisfies(host, requirement) {
			return fmt.Errorf("extensions: %s requires Floter %s, host is %s", m.ID, requirement, strings.TrimSpace(hostVersion))
		}
	}
	if requirement := strings.TrimSpace(m.Compatibility.ProviderProtocol); requirement != "" {
		protocol, err := ParseSemver(ProtocolVersion)
		if err == nil && !Satisfies(protocol, requirement) {
			return fmt.Errorf("extensions: %s does not support provider protocol %s", m.ID, ProtocolVersion)
		}
	}
	return nil
}

// parseHostVersion reads the running build's version for a compatibility
// check. A preview build ("0.4.0-preview") matches as its release line
// ("0.4.0"), as the old host did, so preview builds still load packages
// targeting the same release.
func parseHostVersion(text string) (Semver, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Semver{}, false
	}
	host, err := ParseSemver(text)
	if err != nil {
		return Semver{}, false
	}
	host.Prerelease, host.Build, host.Original = "", "", ""
	return host, true
}
