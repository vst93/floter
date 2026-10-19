package shell

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// noticeLog captures the system notifications a background task raises, which
// is the only place a completion is observable without a window.
type noticeLog struct {
	mu    sync.Mutex
	items []string
}

func (l *noticeLog) add(_, body string) {
	l.mu.Lock()
	l.items = append(l.items, body)
	l.mu.Unlock()
}

func (l *noticeLog) first() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.items) == 0 {
		return ""
	}
	return l.items[0]
}

// reconnectFixture records a system-owned integration bound to a program at
// `bound` — the state a row is in after the tool it was connected to was
// reinstalled somewhere else.
func reconnectFixture(t *testing.T, bound string) (extensions.Paths, string) {
	t.Helper()
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(paths.Extensions, "dev.floter.moved")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(`{
  "schemaVersion": "2.0", "id": "dev.floter.moved", "name": "Moved Tool",
  "distribution": {"type": "local"},
  "runtime": {"type": "system", "executableNames": ["systool"]},
  "provider": {"type": "executable", "argsPrefix": []}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
  "schemaVersion": 1,
  "extensions": {
    "dev.floter.moved": {
      "id": "dev.floter.moved", "name": "Moved Tool", "state": "enabled", "enabled": true,
      "distributionSource": "local", "runtimeOwnership": "system",
      "packageVersion": "1.0.0", "manifestPath": "",
      "executablePath": "`+jsonEscaped(bound)+`",
      "channel": "stable", "installedAt": 1, "updatedAt": 1
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return paths, "dev.floter.moved"
}

func reconnectApp(t *testing.T, paths extensions.Paths, log *noticeLog) *App {
	t.Helper()
	a := New(Options{
		Store: settings.NewStore(settings.Default()),
		Paths: paths,
		// The machine's own state is never touched by a test.
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) {
			return nil, errors.New("no terminal library in tests")
		},
		OpenAtLogin:    func() bool { return false },
		SetOpenAtLogin: func(bool) error { return nil },
		Notify:         log.add,
	})
	a.RefreshIntegrations(context.Background())
	return a
}

// waitForNotice waits for the first notification a background task raised.
func waitForNotice(t *testing.T, log *noticeLog) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if got := log.first(); got != "" {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatal("no notification ever arrived")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A system integration whose program moved had no way back from the panel: the
// row said the runtime was unavailable and nothing on it could re-derive the
// path. The reference had `extensions_reconnect_system` behind its Reconnect
// button (`src/extensions/ExtensionRow.tsx:345-370`); this walks the route end
// to end — the page projects the row as needing the repair, the action rebinds
// the entry to where the program is now, and the page stops offering it.
func TestTheReconnectActionRebindsAMovedProgram(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "systool")
	paths, id := reconnectFixture(t, gone)
	here := filepath.Join(t.TempDir(), "systool")
	if err := os.WriteFile(here, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := extensions.StubToolCandidates(func(names ...string) []extensions.ExecutableCandidate {
		for _, name := range names {
			if name == "systool" {
				return []extensions.ExecutableCandidate{{Name: name, Path: here, Sources: []string{"path"}}}
			}
		}
		return nil
	})
	defer restore()

	log := &noticeLog{}
	a := reconnectApp(t, paths, log)
	// The row the page draws before the repair: the program is not there, and
	// the row knows it — that is what puts the button on it.
	before := a.integrationList()
	if len(before) != 1 || before[0].RuntimeAvailable {
		t.Fatalf("rows = %+v, want one with an unavailable runtime", before)
	}
	if before[0].Homepage != "" {
		t.Fatalf("homepage = %q, want none", before[0].Homepage)
	}

	a.reconnectIntegration(id, nil)

	// The notice is the reference's "repaired" arm: the record changed, so the
	// sentence says the row was reconnected rather than that it was checked.
	want := i18n.For("en").Notifications.IntegrationReconnected("Moved Tool")
	if got := waitForNotice(t, log); got != want {
		t.Errorf("notice = %q, want %q", got, want)
	}
	integration, ok := a.Integrations.Inventory().WithID(id)
	if !ok || integration.Entry.ExecutablePath != here {
		t.Fatalf("entry = %+v, want the program rebound to %s", integration.Entry, here)
	}
	// And after it the row is healthy again: the page offers no repair it no
	// longer needs.
	after := a.integrationList()
	if len(after) != 1 || !after[0].RuntimeAvailable {
		t.Errorf("rows = %+v, want the runtime available again", after)
	}
}

// A reconnect that cannot find the program says so and leaves the row the way
// it was: nothing on this machine answers to the manifest's name, so there is
// no new path to write and no silent success to report.
func TestAReconnectWithNothingToBindToReportsIt(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "systool")
	paths, id := reconnectFixture(t, gone)
	// Nothing on the search path answers to the manifest's name, so the
	// candidate list is empty and the reconnect falls back to the by-name
	// lookup — which the stub below also reports as missing.
	restoreCandidates := extensions.StubToolCandidates(func(...string) []extensions.ExecutableCandidate { return nil })
	defer restoreCandidates()
	restore := extensions.StubToolLookup(func(names ...string) (string, bool) { return "", false })
	defer restore()

	log := &noticeLog{}
	a := reconnectApp(t, paths, log)
	a.reconnectIntegration(id, nil)

	want := i18n.For("en").Notifications.IntegrationCheckFailed("Moved Tool")
	if got := waitForNotice(t, log); got != want {
		t.Errorf("notice = %q, want %q", got, want)
	}
	integration, ok := a.Integrations.Inventory().WithID(id)
	if !ok {
		t.Fatal("the integration disappeared")
	}
	if integration.Entry.ExecutablePath != gone {
		t.Errorf("ExecutablePath = %q, want the stale %q kept", integration.Entry.ExecutablePath, gone)
	}
	// The row still needs the repair, which is the point of recording why.
	if row := a.integrationList(); len(row) != 1 || row[0].RuntimeAvailable {
		t.Errorf("rows = %+v, want the runtime still unavailable", row)
	}
}

// The row's other repair arm hands the project's own page to the system's
// browser: a runtime floter does not own has nowhere on the search path to
// look, which is exactly the reference's `installTool` arm.
func TestTheRepairOpensTheProjectPageItWasGiven(t *testing.T) {
	paths, id := reconnectFixture(t, filepath.Join(t.TempDir(), "systool"))
	log := &noticeLog{}
	a := reconnectApp(t, paths, log)
	var opened []string
	a.openURLCommand = func(browserID, url string) error { opened = append(opened, browserID+"|"+url); return nil }

	a.openHomepage("  https://example.com/tool  ")
	if len(opened) != 1 || opened[0] != "|https://example.com/tool" {
		t.Errorf("opened = %v, want the trimmed page in the default browser", opened)
	}
	// A blank page opens nothing at all: there is no page to hand over, and a
	// browser opened on an empty address is worse than nothing.
	a.openHomepage("   ")
	if len(opened) != 1 {
		t.Errorf("opened = %v, want a blank page to open nothing", opened)
	}
	if _, ok := a.Integrations.Inventory().WithID(id); !ok {
		t.Error("the fixture's integration vanished")
	}
}

// The re-check's two answers are different notices, exactly as the reference
// split them (`src/ExtensionsPanel.tsx:1255-1267`): a record that had to be
// rewritten says "repaired", and a package that verified as it stood says
// "re-check complete: healthy". A repair that always said one of the two would
// make the other unreachable, which is what this walks end to end.
func TestTheRepairNoticesSplitRepairedFromRechecked(t *testing.T) {
	program := func(t *testing.T) string {
		t.Helper()
		here := filepath.Join(t.TempDir(), "systool")
		if err := os.WriteFile(here, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return here
	}

	t.Run("a package that verifies says it was re-checked", func(t *testing.T) {
		paths, id := reconnectFixture(t, program(t))
		log := &noticeLog{}
		a := reconnectApp(t, paths, log)
		a.repairIntegration(id, nil)

		want := i18n.For("en").Notifications.IntegrationRechecked("Moved Tool")
		if got := waitForNotice(t, log); got != want {
			t.Errorf("notice = %q, want %q", got, want)
		}
		if entry, ok := a.Integrations.Inventory().WithID(id); !ok || entry.Entry.State != "enabled" {
			t.Errorf("entry = %+v, want the verified row left enabled", entry.Entry)
		}
	})

	t.Run("a record that drifted says it was repaired", func(t *testing.T) {
		paths, id := reconnectFixture(t, program(t))
		repo, err := extensions.LoadRepository(paths.RepositoryFile)
		if err != nil {
			t.Fatal(err)
		}
		entry := repo.Extensions[id]
		entry.ManifestPath = filepath.Join(t.TempDir(), "moved-away", "floter.extension.json")
		repo.Extensions[id] = entry
		if err := extensions.SaveRepository(paths.RepositoryFile, repo); err != nil {
			t.Fatal(err)
		}

		log := &noticeLog{}
		a := reconnectApp(t, paths, log)
		a.repairIntegration(id, nil)

		want := i18n.For("en").Notifications.IntegrationRepaired("Moved Tool")
		if got := waitForNotice(t, log); got != want {
			t.Errorf("notice = %q, want %q", got, want)
		}
		// The stale path was rewritten to where the manifest resolves: the
		// package directory the id names.
		after, ok := a.Integrations.Inventory().WithID(id)
		if !ok {
			t.Fatal("the integration vanished")
		}
		if after.Entry.ManifestPath == entry.ManifestPath {
			t.Errorf("ManifestPath = %q, want the stale path rewritten", after.Entry.ManifestPath)
		}
	})
}

// The panel holds the row's button in its in-flight face until the shell
// answers, so the shell reports the outcome back on the same main-thread hop as
// the notification — success and failure alike, the reference's `runMutation`
// `finally` (`src/ExtensionsPanel.tsx:1190-1210`). A shell that forgot the
// callback would leave the button spinning forever.
func TestTheRepairReportsBackToTheRow(t *testing.T) {
	t.Run("a repair reports success", func(t *testing.T) {
		here := filepath.Join(t.TempDir(), "systool")
		if err := os.WriteFile(here, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		paths, id := reconnectFixture(t, here)
		log := &noticeLog{}
		a := reconnectApp(t, paths, log)

		done := make(chan error, 1)
		a.repairIntegration(id, func(err error) { done <- err })
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("a verified package reported %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the row was never told the repair finished")
		}
	})

	t.Run("a reconnect reports its failure", func(t *testing.T) {
		gone := filepath.Join(t.TempDir(), "systool")
		paths, id := reconnectFixture(t, gone)
		restoreCandidates := extensions.StubToolCandidates(func(...string) []extensions.ExecutableCandidate { return nil })
		defer restoreCandidates()
		restore := extensions.StubToolLookup(func(...string) (string, bool) { return "", false })
		defer restore()

		log := &noticeLog{}
		a := reconnectApp(t, paths, log)

		done := make(chan error, 1)
		a.reconnectIntegration(id, func(err error) { done <- err })
		select {
		case err := <-done:
			if err == nil {
				t.Error("a reconnect with nothing to bind to reported success")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the row was never told the reconnect finished")
		}
	})
}
