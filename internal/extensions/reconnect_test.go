package extensions

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// systemFixture records a system-owned integration in a config root: a
// manifest whose runtime is a program looked up on the search path, and a
// repository entry the test shapes through `mutate`. Nothing here runs the
// provider — a reconnect resolves the runtime and writes the record, no more.
func systemFixture(t *testing.T, id string, mutate func(*Entry)) (Paths, string) {
	t.Helper()
	paths := FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(paths.Extensions, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, manifestFileName), `{
  "schemaVersion": "2.0", "id": "`+id+`", "name": "System Tool",
  "distribution": {"type": "local"},
  "runtime": {"type": "system", "executableNames": ["systool"]},
  "provider": {"type": "executable", "argsPrefix": []}
}`)

	entry := Entry{
		ID:                 id,
		Name:               "System Tool",
		DistributionSource: "local",
		RuntimeOwnership:   RuntimeOwnershipSystem,
		State:              "enabled",
		Enabled:            true,
		ManifestPath:       filepath.Join(dir, manifestFileName),
		InstalledAt:        1,
		UpdatedAt:          1,
	}
	if mutate != nil {
		mutate(&entry)
	}
	repo := NewRepository()
	repo.Extensions[id] = entry
	if err := SaveRepository(paths.RepositoryFile, repo); err != nil {
		t.Fatal(err)
	}
	return paths, dir
}

// stubToolAt points the search-path lookup at one program, for the whole
// reconnect.
func stubToolAt(t *testing.T, path string) {
	t.Helper()
	restore := StubToolLookup(func(names ...string) (string, bool) {
		for _, name := range names {
			if name == "systool" {
				return path, true
			}
		}
		return "", false
	})
	t.Cleanup(restore)
}

func entryOf(t *testing.T, paths Paths, id string) Entry {
	t.Helper()
	integration, ok := LoadInventory(paths).WithID(id)
	if !ok {
		t.Fatalf("no entry for %s", id)
	}
	return integration.Entry
}

// A reconnect is the answer to "the program moved": the entry was bound to a
// path that is no longer there, the search path knows where the program went,
// and the record follows it. This is the whole point of the button — the row
// that says "Executable is no longer available at …" must be able to look
// again.
func TestReconnectFollowsTheProgramToItsNewPath(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "systool")
	paths, _ := systemFixture(t, "dev.floter.sys", func(entry *Entry) {
		entry.ExecutablePath = gone
	})
	moved := filepath.Join(t.TempDir(), "bin", "systool")
	writeFile(t, moved, "#!/bin/sh\n")
	stubToolAt(t, moved)

	report, err := Reconnect(paths, "dev.floter.sys")
	if err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	if !report.Reconnected || report.Path != moved {
		t.Fatalf("report = %+v, want reconnected at %s", report, moved)
	}
	entry := entryOf(t, paths, "dev.floter.sys")
	if entry.ExecutablePath != moved {
		t.Errorf("ExecutablePath = %q, want %q", entry.ExecutablePath, moved)
	}
	if entry.UpdatedAt == 1 {
		t.Error("the rewrite did not stamp updatedAt")
	}
}

// A row the last check marked broken because its program vanished is exactly
// the row the repair is for: the recorded binding failure is cleared and the
// row gets its own switch's state back, so the launcher's commands come back.
func TestReconnectClearsTheBindingFailureItWasBoundTo(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "systool")
	missing := BindingMissingDetail(gone)
	paths, _ := systemFixture(t, "dev.floter.sys", func(entry *Entry) {
		entry.ExecutablePath = gone
		entry.State = "broken"
		entry.LastErrorCode = strptr(BindingCodeMissing)
		entry.LastErrorDetail = &missing
		entry.BrokenReason = &missing
		entry.LastErrorAt = u64ptr(2)
	})
	moved := filepath.Join(t.TempDir(), "systool")
	writeFile(t, moved, "#!/bin/sh\n")
	stubToolAt(t, moved)

	report, err := Reconnect(paths, "dev.floter.sys")
	if err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	if !report.Reconnected {
		t.Error("clearing a recorded failure is a change to the record")
	}
	entry := entryOf(t, paths, "dev.floter.sys")
	if entry.LastErrorCode != nil || entry.LastErrorDetail != nil || entry.BrokenReason != nil || entry.LastErrorAt != nil {
		t.Errorf("the binding failure survived: %+v", entry)
	}
	if entry.State != "enabled" {
		t.Errorf("State = %q, want enabled", entry.State)
	}
}

// The state handed back is the row's own switch, not always "enabled": a
// disabled integration that is repaired stays off.
func TestReconnectLeavesADisabledRowDisabled(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "systool")
	detail := BindingMissingDetail(gone)
	paths, _ := systemFixture(t, "dev.floter.sys", func(entry *Entry) {
		entry.ExecutablePath = gone
		entry.State = "broken"
		entry.Enabled = false
		entry.LastErrorCode = strptr(BindingCodeMissing)
		entry.LastErrorDetail = &detail
	})
	stubToolAt(t, filepath.Join(t.TempDir(), "systool"))

	if _, err := Reconnect(paths, "dev.floter.sys"); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	if entry := entryOf(t, paths, "dev.floter.sys"); entry.State != "disabled" {
		t.Errorf("State = %q, want disabled", entry.State)
	}
}

// Nothing on the search path: the reconnect fails and *says how*, by recording
// the same keyed failure a check records, so the row words it in the user's
// language rather than going grey. The stale path stays on the entry — a
// failed reconnect must not throw away the only fact known about where the
// program used to be.
func TestReconnectRecordsTheRuntimeItCouldNotFind(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "systool")
	paths, _ := systemFixture(t, "dev.floter.sys", func(entry *Entry) {
		entry.ExecutablePath = gone
	})
	restore := StubToolLookup(func(names ...string) (string, bool) { return "", false })
	defer restore()

	report, err := Reconnect(paths, "dev.floter.sys")
	if err == nil {
		t.Fatal("a reconnect with nothing to bind to succeeded")
	}
	if report.Reconnected || report.Path != "" {
		t.Errorf("report = %+v, want an empty one", report)
	}
	var failure *BindingFailure
	if !errors.As(err, &failure) || failure.Code != BindingCodeRuntimeUnavailable {
		t.Fatalf("err = %v, want a %s binding failure", err, BindingCodeRuntimeUnavailable)
	}
	entry := entryOf(t, paths, "dev.floter.sys")
	if entry.ExecutablePath != gone {
		t.Errorf("ExecutablePath = %q, want the stale %q", entry.ExecutablePath, gone)
	}
	if entry.State != "broken" {
		t.Errorf("State = %q, want broken", entry.State)
	}
	if entry.LastErrorCode == nil || *entry.LastErrorCode != BindingCodeRuntimeUnavailable {
		t.Errorf("LastErrorCode = %v, want %s", entry.LastErrorCode, BindingCodeRuntimeUnavailable)
	}
	if entry.LastErrorDetail == nil {
		t.Fatal("no keyed detail to render the reason from")
	}
	parsed, ok := ParseBindingDetail(*entry.LastErrorDetail)
	if !ok || parsed.Code != BindingCodeRuntimeUnavailable {
		t.Errorf("detail = %q, want a keyed %s", *entry.LastErrorDetail, BindingCodeRuntimeUnavailable)
	}
	if len(parsed.Names) != 1 || parsed.Names[0] != "systool" {
		t.Errorf("detail names = %v, want the names that were looked for", parsed.Names)
	}
}

// A reconnect that finds the program exactly where the entry already said it
// was changed nothing, and the record must not be rewritten for it: the notice
// is the reference's "already working" arm, not a repair.
func TestReconnectThatFindsNothingNewLeavesTheEntryAlone(t *testing.T) {
	here := filepath.Join(t.TempDir(), "systool")
	writeFile(t, here, "#!/bin/sh\n")
	paths, _ := systemFixture(t, "dev.floter.sys", func(entry *Entry) {
		entry.ExecutablePath = here
	})
	stubToolAt(t, here)

	report, err := Reconnect(paths, "dev.floter.sys")
	if err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	if report.Reconnected || report.Path != here {
		t.Errorf("report = %+v, want an unchanged binding at %s", report, here)
	}
	if entry := entryOf(t, paths, "dev.floter.sys"); entry.UpdatedAt != 1 {
		t.Errorf("UpdatedAt = %d, want the untouched 1", entry.UpdatedAt)
	}
}

// Only a system row's binding is floter's to re-derive: a bundled package
// carries its runtime, so there is nothing on the search path to look for. The
// reference's reconnect rejected every other ownership the same way.
func TestReconnectRefusesARuntimeFloterDoesNotOwn(t *testing.T) {
	here := filepath.Join(t.TempDir(), "bundled.sh")
	writeFile(t, here, "#!/bin/sh\n")
	paths, _ := systemFixture(t, "dev.floter.bundled", func(entry *Entry) {
		entry.RuntimeOwnership = "bundled"
		entry.ExecutablePath = here
	})
	stubToolAt(t, filepath.Join(t.TempDir(), "systool"))

	_, err := Reconnect(paths, "dev.floter.bundled")
	if !errors.Is(err, ErrNotSystemOwned) {
		t.Fatalf("err = %v, want ErrNotSystemOwned", err)
	}
	if entry := entryOf(t, paths, "dev.floter.bundled"); entry.ExecutablePath != here || entry.UpdatedAt != 1 {
		t.Errorf("a refused reconnect touched the entry: %+v", entry)
	}
	if _, err := Reconnect(paths, "dev.floter.nothing"); !errors.Is(err, ErrNoIntegration) {
		t.Errorf("err = %v, want ErrNoIntegration for an unknown id", err)
	}
}

// A failed *probe* is a different fact about the same entry, and a reconnect
// must not erase it: the row is broken because the tool's own health check
// said so, and that reason has to survive the path being re-derived.
func TestReconnectKeepsAProbeFailureItDidNotCause(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "systool")
	reason := `lifecycle probe "version" failed (exit code 1): boom`
	paths, _ := systemFixture(t, "dev.floter.sys", func(entry *Entry) {
		entry.ExecutablePath = gone
		entry.State = "broken"
		entry.LastErrorCode = strptr("probe_failed")
		entry.LastErrorDetail = &reason
		entry.BrokenReason = &reason
	})
	moved := filepath.Join(t.TempDir(), "systool")
	writeFile(t, moved, "#!/bin/sh\n")
	stubToolAt(t, moved)

	if _, err := Reconnect(paths, "dev.floter.sys"); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	entry := entryOf(t, paths, "dev.floter.sys")
	if entry.ExecutablePath != moved {
		t.Errorf("ExecutablePath = %q, want %q", entry.ExecutablePath, moved)
	}
	if entry.LastErrorCode == nil || *entry.LastErrorCode != "probe_failed" {
		t.Errorf("LastErrorCode = %v, want probe_failed to survive", entry.LastErrorCode)
	}
	if entry.BrokenReason == nil || *entry.BrokenReason != reason {
		t.Errorf("BrokenReason = %v, want the probe's own sentence", entry.BrokenReason)
	}
	if entry.State != "broken" {
		t.Errorf("State = %q, want the probe failure to keep it broken", entry.State)
	}
}

func strptr(value string) *string { return &value }

func u64ptr(value uint64) *uint64 { return &value }
