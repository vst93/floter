package extensions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// The capability probes for an integration whose manifest declares them:
// each probe passes a set of arguments to the tool and checks the exit code
// and the captured stdout against expectations. The report aggregates the
// results into a version string and the set of supported features — what the
// settings page shows beside the tool's name.

// The two structural probes every report carries.
const (
	ProbeVersion = "version"
	ProbeHelp    = "help"
)

// UnknownVersion is the version string a report carries when the version
// probe did not pass.
const UnknownVersion = "unknown"

// probeTimeout is how long one probe may run before it is killed.
const probeTimeout = 5 * time.Second

// CapabilityProbe is one capability check against a tool.
type CapabilityProbe struct {
	// ID identifies the probe in the report.
	ID string
	// Args are passed to the tool.
	Args []string
	// ExpectExitCode, when set, is the exit code that confirms the
	// capability (some tools exit non-zero from --help).
	ExpectExitCode *int
	// ExpectOutput, when set, is a substring that must appear in stdout.
	ExpectOutput string
}

// ProbeResult is one probe's outcome.
type ProbeResult struct {
	Probe  CapabilityProbe
	Passed bool
	// Stdout is the captured standard output (capped).
	Stdout string
	// Reason describes a failure, for the report's limitations.
	Reason string
}

// CapabilityReport aggregates probe results into what the settings page
// shows.
type CapabilityReport struct {
	// Version is the version the tool reported, or UnknownVersion.
	Version string
	// SupportedFeatures lists every probe id that passed.
	SupportedFeatures []string
	// Limitations describes each failed probe with its arguments and reason.
	Limitations []string
}

// ProbeCapabilities runs the version, help and custom probes against an
// integration and produces the report. The version probe's first non-empty
// output line becomes the version; a probe passes when its expected exit code
// matches (zero by default) and its expected output appears in stdout.
func ProbeCapabilities(ctx context.Context, integration Integration) (CapabilityReport, error) {
	binding, err := ResolveRuntime(integration)
	if err != nil {
		return CapabilityReport{}, err
	}
	probes := []CapabilityProbe{
		{ID: ProbeVersion, Args: []string{"--version"}, ExpectExitCode: capIntPtr(0)},
		{ID: ProbeHelp, Args: []string{"--help"}, ExpectExitCode: capIntPtr(0)},
	}
	probes = append(probes, capabilityProbes(integration)...)

	report := CapabilityReport{Version: UnknownVersion}
	for _, probe := range probes {
		result := probe.Run(ctx, binding, integration)
		if result.Passed {
			report.SupportedFeatures = append(report.SupportedFeatures, result.Probe.ID)
			if result.Probe.ID == ProbeVersion {
				report.Version = firstLine(result.Stdout)
			}
			continue
		}
		invocation := result.Probe.ID
		if len(result.Probe.Args) > 0 {
			invocation += " " + strings.Join(result.Probe.Args, " ")
		}
		if result.Reason != "" {
			report.Limitations = append(report.Limitations, invocation+": "+result.Reason)
		} else {
			report.Limitations = append(report.Limitations, invocation+": failed")
		}
	}
	return report, nil
}

// capabilityProbes reads the manifest's declared capability probes, for an
// integration that declares any beyond the two structural ones.
func capabilityProbes(integration Integration) []CapabilityProbe {
	var probes []CapabilityProbe
	for _, probe := range integration.Manifest.Lifecycle.Probes {
		probes = append(probes, CapabilityProbe{
			ID:   probe.ID,
			Args: append([]string{}, probe.Args...),
		})
	}
	return probes
}

// Run executes one capability probe and reads its outcome.
func (p CapabilityProbe) Run(ctx context.Context, binding RuntimeBinding, integration Integration) ProbeResult {
	result := ProbeResult{Probe: p}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	argv := append(append([]string{}, binding.Args...), p.Args...)
	cmd := exec.CommandContext(probeCtx, binding.Program, argv...)
	cmd.Stdin = nil
	if dir := integration.PackageDir(); dir != "" {
		if info, statErr := os.Stat(dir); statErr == nil && info.IsDir() {
			cmd.Dir = dir
		}
	}
	cmd.WaitDelay = time.Second
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	started := time.Now()
	runErr := cmd.Run()
	_ = started

	result.Stdout = clip(stdout.Bytes())
	if probeCtx.Err() != nil {
		result.Reason = fmt.Sprintf("timed out after %s", probeTimeout)
		return result
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code := exitErr.ExitCode()
			result.Reason = fmt.Sprintf("exit code %d", code)
			if p.ExpectExitCode != nil && code == *p.ExpectExitCode {
				// The exit code matched; only the output check can fail now.
				result.Reason = ""
			}
		} else {
			result.Reason = fmt.Sprintf("could not run the tool: %v", runErr)
			return result
		}
	} else if p.ExpectExitCode == nil {
		// No expectation and a zero exit: the probe passes.
	}

	if p.ExpectExitCode != nil {
		exitCode := 0
		if runErr != nil {
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			}
		}
		if exitCode != *p.ExpectExitCode {
			result.Reason = fmt.Sprintf("exit code %d", exitCode)
			return result
		}
	}
	if p.ExpectOutput != "" && !strings.Contains(result.Stdout, p.ExpectOutput) {
		result.Reason = "the expected output did not appear"
		return result
	}
	result.Passed = true
	return result
}

// firstLine is the first non-empty line of a block of text, trimmed.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func capIntPtr(v int) *int { return &v }

// helpDeriveTimeout bounds the connect-time `--help` probe. The probe
// itself already has its own timeout; this outer bound keeps connects snappy
// even when a binary hangs before the inner timeout fires.
const helpDeriveTimeout = 3 * time.Second

// subcommandProbeTimeout bounds one subcommand's second-level help probe.
const subcommandProbeTimeout = 4 * time.Second

// ProbeHelpText runs `program args…` (any exit code counts), preferring
// stdout and falling back to stderr when stdout is empty. It reports ""
// when nothing readable came back in time.
func ProbeHelpText(ctx context.Context, binding RuntimeBinding, integration Integration, args ...string) string {
	probeCtx, cancel := context.WithTimeout(ctx, helpDeriveTimeout)
	defer cancel()
	argv := append(append([]string{}, binding.Args...), args...)
	cmd := exec.CommandContext(probeCtx, binding.Program, argv...)
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
	runErr := cmd.Run()
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
