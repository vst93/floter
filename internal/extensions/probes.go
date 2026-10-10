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

// The manifest's lifecycle probes: health checks the tool declares, which the
// host runs through the same runtime binding its commands use. A probe's
// arguments are complete tool arguments — independent of the provider protocol
// prefix — and an empty lifecycle declaration executes nothing, including for
// a v1 manifest.
//
// The report is persisted on the repository entry under `probeReport`, the
// field the old build wrote, and a **required** probe's failure marks the
// integration broken: a tool whose own health check says it cannot run is not
// usable, whatever the repository's enabled flag says.

// HealthSchemaVersion is the probe report's schema version.
const HealthSchemaVersion = 1

// The health statuses, as the old build's vocabulary spelled them.
const (
	HealthUnknown   = "unknown"
	HealthHealthy   = "healthy"
	HealthDegraded  = "degraded"
	HealthUnhealthy = "unhealthy"
)

// probeOutputCap is the cap on one probe's captured stderr.
const probeOutputCap = 64 << 10

// ProbeRecord is one probe's outcome.
type ProbeRecord struct {
	ID     string `json:"probeId"`
	Passed bool   `json:"passed"`
	// DurationMs is how long the probe took.
	DurationMs uint64 `json:"durationMs"`
	// ExitCode is -1 when the probe never ran to an exit.
	ExitCode int    `json:"exitCode"`
	Stderr   string `json:"stderr,omitempty"`
}

// HealthReport is the aggregated lifecycle health, persisted on the
// repository entry.
type HealthReport struct {
	SchemaVersion int           `json:"schemaVersion"`
	Status        string        `json:"status"`
	CheckedAt     string        `json:"checkedAt"`
	Probes        []ProbeRecord `json:"probes,omitempty"`
}

// finalize determines overall health: a required probe's failure is
// unhealthy, an optional one's is degraded, and nothing run proves nothing.
func (r *HealthReport) finalize(schema []Probe) {
	failed := map[string]bool{}
	for _, record := range r.Probes {
		if !record.Passed {
			failed[record.ID] = true
		}
	}
	requiredFailed, optionalFailed := false, false
	for _, probe := range schema {
		if failed[probe.ID] {
			if probe.Required {
				requiredFailed = true
			} else {
				optionalFailed = true
			}
		}
	}
	switch {
	case requiredFailed:
		r.Status = HealthUnhealthy
	case optionalFailed:
		r.Status = HealthDegraded
	case len(r.Probes) == 0:
		r.Status = HealthUnknown
	default:
		r.Status = HealthHealthy
	}
}

// ErrNoIntegration is the same error the other entry operations report.
//
// ErrUnknownAction is not defined here; the report's errors are plain errors.

// ErrNoProbes is what a reprobe reports for an integration whose manifest
// declares none: it proves nothing and must not erase an earlier report.
var ErrNoProbes = errors.New("extensions: the integration declares no lifecycle probes")

// Reprobe runs an integration's declared probes and records the report on its
// repository entry, marking it broken when a required probe failed.
//
// The manifest's identity is checked first: a package that has been swapped
// under the repository must not be probed as the integration it replaced.
func Reprobe(ctx context.Context, paths Paths, id string) (HealthReport, error) {
	inventory := LoadInventory(paths)
	if inventory.RepositoryErr != nil {
		return HealthReport{}, inventory.RepositoryErr
	}
	integration, ok := inventory.WithID(id)
	if !ok {
		return HealthReport{}, ErrNoIntegration
	}
	if integration.ManifestErr != nil {
		return HealthReport{}, fmt.Errorf("extensions: %s: %w", id, integration.ManifestErr)
	}
	if integration.Manifest.ID != integration.Entry.ID {
		return HealthReport{}, fmt.Errorf("extensions: the installed manifest does not match %s", id)
	}
	probes := integration.Manifest.Lifecycle.Probes
	if len(probes) == 0 {
		return HealthReport{}, ErrNoProbes
	}
	binding, err := ResolveRuntime(integration)
	if err != nil {
		return HealthReport{}, err
	}

	report := HealthReport{
		SchemaVersion: HealthSchemaVersion,
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	for _, probe := range probes {
		record := runProbe(ctx, binding, integration, probe)
		report.Probes = append(report.Probes, record)
	}
	report.finalize(probes)

	if err := recordHealthReport(paths, id, report, probes); err != nil {
		return report, err
	}
	return report, nil
}

// runProbe runs one probe and reads its outcome. The arguments are the tool's
// own; the timeout is the probe's, with a floor so a zero cannot hang the host.
func runProbe(ctx context.Context, binding RuntimeBinding, integration Integration, probe Probe) ProbeRecord {
	timeout := time.Duration(probe.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// The probe's arguments are the tool's own: the runtime's leading
	// arguments, then the probe's, in the integration's configured environment
	// (its own process environment otherwise), with the standard input closed
	// and both streams captured.
	argv := append(append([]string{}, binding.Args...), probe.Args...)
	cmd := exec.CommandContext(probeCtx, binding.Program, argv...)
	cmd.Stdin = nil
	if dir := integration.PackageDir(); dir != "" {
		cmd.Dir = dir
	}
	cmd.WaitDelay = time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	started := time.Now()
	runErr := cmd.Run()
	duration := time.Since(started)

	record := ProbeRecord{
		ID:         probe.ID,
		DurationMs: uint64(duration.Milliseconds()),
		ExitCode:   -1,
		Stderr:     clip(stderr.Bytes()),
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			record.ExitCode = exitErr.ExitCode()
		}
	} else {
		record.Passed = true
		record.ExitCode = 0
	}
	return record
}

// recordHealthReport writes the report onto the repository entry, marking the
// integration broken when a required probe failed and clearing that when a
// later reprobe passes.
func recordHealthReport(paths Paths, id string, report HealthReport, schema []Probe) error {
	if report.Status == HealthUnknown {
		return nil
	}
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		return err
	}
	entry, ok := repo.Extensions[id]
	if !ok {
		return ErrNoIntegration
	}
	entry.ProbeReport = &report
	entry.UpdatedAt = uint64(time.Now().Unix())

	requiredFailed := false
	for _, probe := range schema {
		if !probe.Required {
			continue
		}
		for _, record := range report.Probes {
			if record.ID == probe.ID && !record.Passed {
				requiredFailed = true
			}
		}
	}
	detail := ""
	if requiredFailed {
		var messages []string
		for _, record := range report.Probes {
			if record.Passed {
				continue
			}
			messages = append(messages, fmt.Sprintf("lifecycle probe %q failed (exit code %d): %s",
				record.ID, record.ExitCode, strings.TrimSpace(record.Stderr)))
		}
		detail = strings.Join(messages, "; ")
	}
	if requiredFailed {
		entry.State = "broken"
		code := "probe_failed"
		entry.LastErrorCode = &code
		entry.LastErrorDetail = &detail
		now := uint64(time.Now().Unix())
		entry.LastErrorAt = &now
		entry.BrokenReason = &detail
	} else {
		entry.LastErrorCode = nil
		entry.LastErrorDetail = nil
		entry.BrokenReason = nil
	}
	repo.Extensions[id] = entry
	return SaveRepository(paths.RepositoryFile, repo)
}
