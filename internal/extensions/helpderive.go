// Help and version derivation for connected tools: the connect-time `--help`
// probes, the parser that turns their output into root arguments and
// subcommands, and the semver extractor.
//
// This file once also carried a capability-report probe suite
// (`ProbeCapabilities` and friends). It was removed: the original never
// populated a report — its reprobe path passed `Default::default()` — so the
// whole path was dead on both sides.
package extensions

import (
	"bytes"
	"context"
	"os"
	"strings"
	"time"
)

// helpDeriveTimeout bounds the connect-time `--help` probe. The probe
// itself already has its own timeout; this outer bound keeps connects snappy
// even when a binary hangs before the inner timeout fires.
const helpDeriveTimeout = 3 * time.Second

// ProbeHelpText runs `program args…` (any exit code counts), preferring
// stdout and falling back to stderr when stdout is empty. It reports ""
// when nothing readable came back in time.
func ProbeHelpText(ctx context.Context, binding RuntimeBinding, integration Integration, args ...string) string {
	probeCtx, cancel := context.WithTimeout(ctx, helpDeriveTimeout)
	defer cancel()
	argv := append(append([]string{}, binding.Args...), args...)
	cmd := newProbeCommand(probeCtx, binding.Program, argv...)
	cmd.Stdin = nil
	// The working directory only helps a tool that reads its own package
	// files; a directory that is not there would fail the exec outright
	// (chdir before fork), so it is set only when it exists.
	if dir := integration.PackageDir(); dir != "" {
		if info, statErr := os.Stat(dir); statErr == nil && info.IsDir() {
			cmd.Dir = dir
		}
	}
	cmd.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := runExtensionCommand(cmd)
	if probeCtx.Err() != nil {
		return ""
	}
	_ = runErr
	text := stdout.String()
	if strings.TrimSpace(text) == "" {
		text = stderr.String()
	}
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return text
}

// ProbeDerive runs the connect-time help derivation: one `--help` probe
// for the root option hints, and — when the output turns out to be a
// subcommand listing — one probe per listed subcommand (up to the probe
// budget) so its real flags can be suggested. Best-effort by contract: any
// failure degrades silently and never blocks a connection.
func ProbeDerive(ctx context.Context, integration Integration) HelpDerivation {
	binding, resolveErr := ResolveRuntime(integration)
	if resolveErr != nil {
		// Best-effort by contract: a tool whose runtime does not resolve
		// right now derives nothing, and the connection still proceeds.
		return HelpDerivation{}
	}
	rootText := ProbeHelpText(ctx, binding, integration, "--help")
	if rootText == "" {
		return HelpDerivation{}
	}
	derivation := HelpDerivation{RootArguments: DeriveArguments(rootText)}
	candidates := DeriveSubcommands(rootText)
	if len(candidates) > maxSubcommandProbes {
		candidates = candidates[:maxSubcommandProbes]
	}
	for _, candidate := range candidates {
		candidate := candidate
		text := ProbeHelpText(ctx, binding, integration, candidate.Name, "--help")
		if strings.TrimSpace(text) == "" {
			// Some tools only answer the short form (v plugins do).
			text = ProbeHelpText(ctx, binding, integration, candidate.Name, "-h")
		}
		if strings.TrimSpace(text) != "" {
			candidate.Arguments = DeriveArguments(text)
		}
		derivation.Subcommands = append(derivation.Subcommands, candidate)
	}
	return derivation
}

// SemverFromVersionOutput extracts the first semver-shaped token from a
// `--version` output. It tolerates a leading `v`, surrounding punctuation,
// multiple lines, and noise words (`git version 2.42.0`, `rg 14.1.0`,
// `v1.2`). Garbage in, empty out: when no token parses as a real semver the
// result is "" — this never guesses or fabricates a version.
func SemverFromVersionOutput(output string) string {
	for _, field := range strings.Fields(output) {
		if version, ok := semverFromVersionToken(field); ok {
			return version
		}
	}
	return ""
}

// semverFromVersionToken reads one token as a semver: punctuation stripped,
// a leading `v` dropped, the digits onward taken, two-component versions
// completed to a patch of zero. A token whose core does not read
// major.minor[.patch] with numeric parts is not a version.
func semverFromVersionToken(token string) (string, bool) {
	token = strings.Trim(token, "()[]{};,:'\"`")
	token = strings.TrimPrefix(strings.TrimPrefix(token, "v"), "V")
	start := strings.IndexFunc(token, func(char rune) bool { return char >= '0' && char <= '9' })
	if start < 0 {
		return "", false
	}
	end := start
	for _, char := range token[start:] {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') || char == '.' || char == '-' || char == '+' {
			end++
			continue
		}
		break
	}
	candidate := token[start:end]
	core := candidate
	suffix := ""
	if index := strings.IndexAny(candidate, "-+"); index >= 0 {
		core, suffix = candidate[:index], candidate[index:]
	}
	parts := strings.Split(core, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return "", false
	}
	major, minor := parts[0], parts[1]
	if !allDigits(major) || !allDigits(minor) {
		return "", false
	}
	patch := "0"
	if len(parts) == 3 {
		if parts[2] == "" || !allDigits(parts[2]) {
			return "", false
		}
		patch = parts[2]
	}
	version := major + "." + minor + "." + patch + suffix
	// The version has to parse as the installer's own semver, which also
	// bounds the prerelease and build syntax.
	if _, err := ParseSemver(version); err != nil {
		return "", false
	}
	return version, true
}

// allDigits reports whether a string is one or more ASCII digits.
func allDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, char := range text {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
