package extensions

import (
	"context"
	"os"
)

// The silent half of the drift re-probe: a generated custom integration's
// command list is derived once, at connect time, from the executable's own
// `--help`. When the upstream tool ships a new version its subcommands and
// flags may have moved and floter has no way to notice. This routine closes
// that gap — when the version recorded at connect time no longer matches what
// the executable reports right now, the command list is re-derived (the same
// atomic replace and help-probe sidecar the manual re-probe uses) and the
// recorded version moves with it. The reference is
// `src-tauri/src/extensions/install.rs:843-975` `reprobe_on_tool_version_change`.
//
// Best-effort by contract, mirroring that routine: a missing executable or a
// failed probe is reported to the caller, the previous descriptor stays
// valid, the entry is never marked broken, and a listing never fails because
// of it. Publisher-shipped descriptors are excluded by construction
// (`isGeneratedIntegration` requires the local publisher and the generated
// package directory), so upstream release content is never overwritten.
//
// The visible half — one toast per real change — is `driftnotice.go`; this
// file decides *whether to probe*, never what to say.

// DriftReprobe is what one version-drift check found and did.
type DriftReprobe struct {
	// ReprobeRan reports whether the version moved and the command list was
	// re-derived. False means there was nothing to compare, nothing moved, or
	// the integration was never eligible.
	ReprobeRan bool
	// ToolVersion is the version the executable reports now; "" when it
	// reports none, which is never a drift.
	ToolVersion string
	// Report is the re-probe's own report, meaningful only when ReprobeRan.
	Report ReprobeReport
}

// CheckToolVersionDrift compares a generated custom integration's recorded
// tool version with the one its executable reports now, and re-probes the
// command list when they differ. An ineligible integration, a tool that
// reports no version, and a version that has not moved are all
// `DriftReprobe{}` with a nil error: nothing happened, so there is nothing to
// announce.
func CheckToolVersionDrift(ctx context.Context, paths Paths, id string) (DriftReprobe, error) {
	integration, ok := LoadInventory(paths).WithID(id)
	if !ok {
		return DriftReprobe{}, nil
	}
	if !driftReprobeEligible(integration) {
		return DriftReprobe{}, nil
	}
	executable := integration.Entry.ExecutablePath
	if executable == "" {
		return DriftReprobe{}, nil
	}
	if _, err := os.Stat(executable); err != nil {
		// The tool is gone; the binding/reconnect path owns that state. A
		// missing executable is not a command drift.
		return DriftReprobe{}, nil
	}
	current := probeVersion(ctx, paths, id)
	if current == "" {
		// No version output (or no `versionArgs`): there is nothing to
		// compare against, so never probe on this basis.
		return DriftReprobe{}, nil
	}
	if recorded := integration.Entry.ToolVersion; recorded != nil && *recorded == current {
		return DriftReprobe{}, nil
	}
	report, err := ReprobeCommands(ctx, paths, id)
	if err != nil {
		// Fail soft: keep the old descriptor, never break the entry, never
		// block the list. A later listing retries once the tool answers.
		return DriftReprobe{}, err
	}
	// The commands are already rebuilt, so a failure to record the version is
	// not a failure of the re-probe: it only means the next listing probes
	// the same drift again.
	recordErr := SetToolVersion(paths, id, current)
	return DriftReprobe{ReprobeRan: true, ToolVersion: current, Report: report}, recordErr
}

// driftReprobeEligible is what a drift re-probe may touch: an enabled,
// floter-generated custom integration on a system runtime whose command list
// comes from a static descriptor (never a script). Publisher descriptors and
// anything not generated here are excluded — `isGeneratedIntegration`
// already requires the static descriptor kind and the local publisher.
func driftReprobeEligible(integration Integration) bool {
	if !integration.Entry.Enabled {
		return false
	}
	if integration.Entry.RuntimeOwnership != RuntimeOwnershipSystem {
		return false
	}
	if !isGeneratedIntegration(integration) {
		return false
	}
	return integration.Manifest.Runtime.Type != "script"
}
