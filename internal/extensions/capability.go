package extensions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
		cmd.Dir = dir
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
