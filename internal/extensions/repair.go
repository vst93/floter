package extensions

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrPackageMissing is what a re-check reports when the installed package's own
// files are not on disk: the directory the entry names, or the manifest inside
// it. Nothing short of a reinstall brings those back, so a re-check reports
// rather than repairs.
var ErrPackageMissing = errors.New("extensions: the installed package is missing")

// ErrManifestChanged is what a re-check reports when the manifest on disk is
// not the one the approval covers. The old host's `verify_installed` treated
// that as a broken package and left the record alone: the approval has to be
// given again, which is not something a repair may do on its own.
var ErrManifestChanged = errors.New("extensions: the manifest on disk is not the approved one")

// RepairReport is what a re-check did — the old host's
// `ExtensionRepairReport`, minus the fields only its Rust side read.
type RepairReport struct {
	// Repaired is true when the record had to be rewritten: a stale
	// `manifestPath`, or a system runtime rebound to where its program is now.
	// False means the package verified as it stood, which the row words as
	// "re-check complete: healthy".
	Repaired bool
	// ManifestPath is where the entry's manifest resolves after the re-check.
	ManifestPath string
}

// Repair verifies an installed package and repairs the one class of drift a
// re-check can fix on its own — the old host's `extensions_repair`, the Wrench
// a broken row shows (`src/ExtensionsPanel.tsx:1255-1267`).
//
// The verification is the package's own bytes, never a probe: the directory and
// the manifest are on disk, the manifest is the approved one
// (`ManifestDigest` against `ApprovedManifestDigest`), and the runtime still
// resolves. Probes answer a different question ("is the tool healthy?") and are
// what the row's Check button runs; a re-check is about whether what is
// installed is what the record claims.
//
// The repairs are the two drifts the record can carry:
//
//   - a remembered `manifestPath` that no longer resolves while the manifest
//     sits where the id names it (the fallback `manifestPathFor` already read),
//     which is rewritten;
//   - a system runtime whose program moved, which is re-derived through the
//     same search-path reconnect the row's Reconnect button offers — the old
//     host's `repair` fell through to `reconnect_system_locked` after a failed
//     verify for exactly this reason.
//
// A package that verifies also clears a stale broken state, as the old host's
// `clear_broken` did: the row was marked broken by a failure that is no longer
// true. That is a verification result, not a repair, so it leaves `Repaired`
// false.
func Repair(paths Paths, id string) (RepairReport, error) {
	inventory := LoadInventory(paths)
	integration, ok := inventory.WithID(id)
	if !ok {
		return RepairReport{}, ErrNoIntegration
	}

	// The manifest's own path is resolved the way the inventory read it: the
	// recorded path when it still exists, else the package directory named by
	// the id. A stale record therefore still verifies — rewriting it is the
	// repair below, not a failure.
	manifestPath := integration.ManifestPath()
	if problem := verifyPackage(integration, manifestPath); problem != nil {
		return RepairReport{ManifestPath: manifestPath}, problem
	}

	// The runtime is the other half of the verify. A program that is not where
	// the record says is repairable for a system runtime (the search path is
	// asked again) and a dead end for one floter does not own.
	if _, err := ResolveRuntime(integration); err != nil {
		report, rerr := reconnectIfSystemOwned(paths, integration)
		switch {
		case rerr == nil && report.Reconnected:
			return RepairReport{Repaired: true, ManifestPath: manifestPath}, nil
		case rerr != nil:
			return RepairReport{ManifestPath: manifestPath}, rerr
		}
		return RepairReport{ManifestPath: manifestPath}, err
	}

	// The package and its runtime verify. What is left is the record's own
	// drift: a remembered manifest path that no longer resolves, and a broken
	// state the passing verify contradicts.
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		return RepairReport{ManifestPath: manifestPath}, err
	}
	entry, ok := repo.Extensions[id]
	if !ok {
		return RepairReport{ManifestPath: manifestPath}, ErrNoIntegration
	}
	repaired := false
	if entry.ManifestPath != "" && entry.ManifestPath != manifestPath {
		entry.ManifestPath = manifestPath
		repaired = true
	}
	// A broken state the passing verify contradicts is dropped, but that is a
	// verification result rather than a repair, so it does not set `Repaired`.
	cleared := clearBroken(&entry)
	if repaired {
		entry.UpdatedAt = uint64(time.Now().Unix())
	}
	if repaired || cleared {
		repo.Extensions[id] = entry
		if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
			return RepairReport{ManifestPath: manifestPath}, err
		}
	}
	return RepairReport{Repaired: repaired, ManifestPath: manifestPath}, nil
}

// verifyPackage checks the package's own bytes: the directory and the manifest
// are there, and the manifest is the one the approval covers. A missing
// approved digest is *unknown*, not stale — the old data the approval helpers
// already treat that way.
func verifyPackage(integration Integration, manifestPath string) error {
	dir := integration.PackageDir()
	if dir == "" {
		return fmt.Errorf("%w: %s", ErrPackageMissing, integration.Entry.ID)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrPackageMissing, dir)
	}
	if info, err := os.Stat(manifestPath); err != nil || info.IsDir() {
		return fmt.Errorf("%w: %s", ErrPackageMissing, manifestPath)
	}
	digest, err := ManifestDigest(manifestPath)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrPackageMissing, manifestPath)
	}
	approved := ""
	if integration.Entry.ApprovedManifestDigest != nil {
		approved = *integration.Entry.ApprovedManifestDigest
	}
	if ApprovalIsStale(approved, digest) {
		return fmt.Errorf("%w: %s", ErrManifestChanged, integration.Entry.ID)
	}
	return nil
}

// reconnectIfSystemOwned re-derives a system runtime's binding from the search
// path, the old host's `reconnect_system_locked` fallback. Any other ownership
// reports `ErrNotSystemOwned`: a bundled or script runtime has nothing on the
// search path to look for.
func reconnectIfSystemOwned(paths Paths, integration Integration) (ReconnectReport, error) {
	if integration.Entry.RuntimeOwnership != RuntimeOwnershipSystem {
		return ReconnectReport{}, fmt.Errorf("%w: %s", ErrNotSystemOwned, integration.Entry.ID)
	}
	return Reconnect(paths, integration.Entry.ID)
}

// clearBroken drops the recorded failure from an entry whose verify passed and
// hands the row back the state its own switch says, the old host's
// `clear_broken`. It reports whether there was a broken state to drop.
func clearBroken(entry *Entry) bool {
	if entry.State != "broken" {
		return false
	}
	if entry.Enabled {
		entry.State = "enabled"
	} else {
		entry.State = "disabled"
	}
	entry.LastErrorCode = nil
	entry.LastErrorDetail = nil
	entry.LastErrorAt = nil
	entry.BrokenReason = nil
	return true
}
