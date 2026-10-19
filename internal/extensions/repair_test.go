//go:build !windows

package extensions

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// systoolAt writes a real program on a temporary path and points the search
// path at it, so a fixture's runtime resolves the way an installed one does.
func systoolAt(t *testing.T) string {
	t.Helper()
	here := filepath.Join(t.TempDir(), "systool")
	writeFile(t, here, "#!/bin/sh\n")
	if err := os.Chmod(here, 0o755); err != nil {
		t.Fatal(err)
	}
	stubToolAt(t, here)
	return here
}

// repairFixture is a system integration whose package is on disk; the mutate
// hook shapes the one fact each case is about.
func repairFixture(t *testing.T, mutate func(*Entry)) (Paths, string) {
	t.Helper()
	return systemFixture(t, "dev.floter.recheck", mutate)
}

// A re-check's three facts, at the package's own bytes: a package that is not
// there, a manifest that is not the approved one, and a package that verifies.
// The first two are reports, not repairs — nothing a re-check can do brings a
// missing file back or re-approves changed bytes — and the third leaves the
// record alone and reports "verified".
func TestTheRecheckVerifiesThePackagesOwnBytes(t *testing.T) {
	t.Run("the package directory is gone", func(t *testing.T) {
		paths, dir := repairFixture(t, nil)
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		report, err := Repair(paths, "dev.floter.recheck")
		if !errors.Is(err, ErrPackageMissing) {
			t.Fatalf("err = %v, want ErrPackageMissing", err)
		}
		if report.Repaired {
			t.Errorf("Repaired = true for a missing package, want false")
		}
	})

	t.Run("the manifest is not the approved one", func(t *testing.T) {
		approved := "sha256-0000000000000000000000000000000000000000000000000000000000000000"
		paths, _ := repairFixture(t, func(entry *Entry) {
			entry.ApprovedManifestDigest = &approved
		})
		report, err := Repair(paths, "dev.floter.recheck")
		if !errors.Is(err, ErrManifestChanged) {
			t.Fatalf("err = %v, want ErrManifestChanged", err)
		}
		if report.Repaired {
			t.Errorf("Repaired = true for a changed manifest, want false")
		}
		// The record is left as it was: a changed manifest is a prompt to
		// re-approve, not a state a repair may overwrite.
		if got := entryOf(t, paths, "dev.floter.recheck"); got.ApprovedManifestDigest == nil ||
			*got.ApprovedManifestDigest != approved {
			t.Errorf("the approval was touched: %+v", got.ApprovedManifestDigest)
		}
	})

	t.Run("the package verifies", func(t *testing.T) {
		here := systoolAt(t)
		paths, _ := repairFixture(t, func(entry *Entry) {
			entry.ExecutablePath = here
		})
		report, err := Repair(paths, "dev.floter.recheck")
		if err != nil {
			t.Fatalf("Repair: %v", err)
		}
		if report.Repaired {
			t.Errorf("Repaired = true for a verified package, want false")
		}
		if report.ManifestPath == "" {
			t.Error("ManifestPath is empty")
		}
	})
}

// The record can drift while the package on disk is intact: a remembered
// manifest path that no longer resolves. The fallback already read the
// manifest by the id, so the re-check verifies and rewrites the stale record —
// which is the one thing it repaired, so it reports "repaired".
func TestTheRecheckRewritesAStaleManifestPath(t *testing.T) {
	here := systoolAt(t)
	paths, dir := repairFixture(t, func(entry *Entry) {
		entry.ExecutablePath = here
		entry.ManifestPath = filepath.Join(t.TempDir(), "moved-away", manifestFileName)
	})
	report, err := Repair(paths, "dev.floter.recheck")
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if !report.Repaired {
		t.Error("Repaired = false for a stale manifest path, want true")
	}
	want := filepath.Join(dir, manifestFileName)
	if report.ManifestPath != want {
		t.Errorf("ManifestPath = %q, want %q", report.ManifestPath, want)
	}
	if got := entryOf(t, paths, "dev.floter.recheck").ManifestPath; got != want {
		t.Errorf("the repository still remembers %q, want %q", got, want)
	}
}

// A broken state a passing verify contradicts is dropped, as the old host's
// `clear_broken` did. That is a verification result rather than a repair, so
// the notice stays "re-checked: healthy" — the row's health row still shows
// the probe report the failure left behind.
func TestTheRecheckClearsABrokenStateItContradicts(t *testing.T) {
	code := "probe_failed"
	detail := "lifecycle probe \"ping\" failed"
	here := systoolAt(t)
	paths, _ := repairFixture(t, func(entry *Entry) {
		entry.ExecutablePath = here
		entry.State = "broken"
		entry.LastErrorCode = &code
		entry.LastErrorDetail = &detail
		entry.BrokenReason = &detail
	})
	report, err := Repair(paths, "dev.floter.recheck")
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if report.Repaired {
		t.Error("Repaired = true for a cleared broken state, want false (it is a verify result)")
	}
	got := entryOf(t, paths, "dev.floter.recheck")
	if got.State != "enabled" {
		t.Errorf("State = %q, want enabled", got.State)
	}
	if got.LastErrorCode != nil || got.BrokenReason != nil {
		t.Errorf("the failure record survived: %+v", got)
	}
}

// A system runtime whose program moved is the other thing a re-check repairs,
// by falling through to the same search-path reconnect the row's Reconnect
// button offers (the old host's `repair` did exactly this after a failed
// verify). The entry is rebound and the notice says "repaired".
func TestTheRecheckRebindsASystemRuntimeThatMoved(t *testing.T) {
	here := systoolAt(t)
	gone := filepath.Join(t.TempDir(), "systool")
	paths, _ := repairFixture(t, func(entry *Entry) {
		entry.ExecutablePath = gone
	})
	report, err := Repair(paths, "dev.floter.recheck")
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if !report.Repaired {
		t.Error("Repaired = false for a rebound runtime, want true")
	}
	if got := entryOf(t, paths, "dev.floter.recheck").ExecutablePath; got != here {
		t.Errorf("ExecutablePath = %q, want the program rebound to %s", got, here)
	}
}

// A runtime floter does not own has nowhere on the search path to look, so the
// re-check reports rather than pretending it can rebind: a bundled row whose
// runtime vanished is not repairable here.
func TestTheRecheckRefusesToRebindARuntimeItDoesNotOwn(t *testing.T) {
	paths, _ := systemFixture(t, "dev.floter.bundled", func(entry *Entry) {
		entry.RuntimeOwnership = "bundled"
		entry.ExecutablePath = filepath.Join(t.TempDir(), "gone")
	})
	report, err := Repair(paths, "dev.floter.bundled")
	if err == nil {
		t.Fatal("Repair succeeded for a runtime it does not own")
	}
	if report.Repaired {
		t.Errorf("Repaired = true for an unowned runtime, want false")
	}
}
