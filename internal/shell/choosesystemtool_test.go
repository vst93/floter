package shell

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// writeCandidate writes a runnable fixture a candidate can point at.
func writeCandidate(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// twoCandidates stubs the search path with two programs for `v`, the state the
// reference's chooser exists for (`ExtensionsPanel.tsx:2623-2648`).
func twoCandidates(t *testing.T) (first, second string) {
	t.Helper()
	first = filepath.Join(t.TempDir(), "one", "v")
	second = filepath.Join(t.TempDir(), "two", "v")
	writeCandidate(t, first)
	writeCandidate(t, second)
	restore := extensions.StubToolCandidates(func(names ...string) []extensions.ExecutableCandidate {
		for _, name := range names {
			if name == "v" {
				return []extensions.ExecutableCandidate{
					{Name: "v", Path: first, Sources: []string{"path"}},
					{Name: "v", Path: second, Sources: []string{"path"}},
				}
			}
		}
		return nil
	})
	t.Cleanup(restore)
	return first, second
}

// The chooser names the tool and lists every candidate with its path and the
// source it was found through — the reference's `chooseSystemTool` title,
// `chooseSystemToolHint` and per-candidate rows.
func TestChooseSystemToolTextNamesEveryCandidate(t *testing.T) {
	copy := i18n.For("en").Settings
	title, detail := chooseSystemToolText("V Tools", []extensions.ExecutableCandidate{
		{Name: "v", Path: "/opt/homebrew/bin/v", Sources: []string{"path"}},
		{Name: "v", Path: "/usr/local/bin/v", Sources: []string{"path"}},
	}, copy)
	if title != "Choose a runtime for V Tools" {
		t.Errorf("title = %q", title)
	}
	for _, want := range []string{
		copy.ChooseSystemToolHint,
		"\u2022 v \u2014 /opt/homebrew/bin/v (path)",
		"\u2022 v \u2014 /usr/local/bin/v (path)",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail is missing %q:\n%s", want, detail)
		}
	}
}

// A manifest whose declared names match more than one program asks which one
// before it connects, and the user's pick is the program the integration binds
// to (`ExtensionsPanel.tsx:1393-1401`). One match never asks.
func TestRecommendedConnectAsksWhichExecutableToBind(t *testing.T) {
	_, second := twoCandidates(t)

	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	asked := 0
	chosen := ""
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		// The permission review is not what this test is about.
		ReviewRecommended: func(extensions.Prepared) bool { return true },
		ChooseSystemTool: func(name string, candidates []extensions.ExecutableCandidate) (string, bool) {
			asked++
			if name != "V Tools" {
				t.Errorf("chooser named %q, want the tool", name)
			}
			if len(candidates) != 2 {
				t.Errorf("chooser saw %d candidates, want two", len(candidates))
			}
			chosen = candidates[1].Path
			return chosen, true
		},
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})

	a.connectRecommended("io.github.vst93.v")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if integration, ok := a.Integrations.Inventory().WithID("io.github.vst93.v"); ok {
			if integration.Entry.ExecutablePath != chosen {
				t.Fatalf("bound to %q, want the chosen %q", integration.Entry.ExecutablePath, chosen)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the connect never landed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if chosen != second {
		t.Fatalf("the chooser's second candidate was %q, want %q", chosen, second)
	}
	if asked != 1 {
		t.Errorf("the chooser was shown %d times, want once", asked)
	}
}

// Cancelling the chooser connects nothing: the permission review is not even
// reached, so the machine is untouched.
func TestRecommendedConnectCancelledByTheChooserInstallsNothing(t *testing.T) {
	twoCandidates(t)

	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	reviewed := 0
	a := New(Options{
		Store:                settings.NewStore(settings.Default()),
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		ReviewRecommended:    func(extensions.Prepared) bool { reviewed++; return true },
		ChooseSystemTool:     func(string, []extensions.ExecutableCandidate) (string, bool) { return "", false },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})

	a.connectRecommended("io.github.vst93.v")
	time.Sleep(300 * time.Millisecond)
	if list := a.recommendedTools(); len(list) != 1 || list[0].Installed {
		t.Fatalf("a cancelled chooser landed: %+v", list)
	}
	if reviewed != 0 {
		t.Errorf("the review ran %d times after a cancelled chooser, want none", reviewed)
	}
	if _, err := os.Stat(filepath.Join(paths.Extensions, "io.github.vst93.v", "floter.extension.json")); err == nil {
		t.Error("a cancelled chooser wrote the package")
	}
}

// A reconnect whose program could not be found asks which of the matching
// programs to bind to, and binds to the pick — the reference's `reconnectSystem`
// arm of the same chooser (`ExtensionsPanel.tsx:1975-1988`).
func TestReconnectAsksWhichExecutableToBind(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "systool")
	paths, id := reconnectFixture(t, gone)
	first := filepath.Join(t.TempDir(), "one", "systool")
	second := filepath.Join(t.TempDir(), "two", "systool")
	writeCandidate(t, first)
	writeCandidate(t, second)
	restore := extensions.StubToolCandidates(func(names ...string) []extensions.ExecutableCandidate {
		for _, name := range names {
			if name == "systool" {
				return []extensions.ExecutableCandidate{
					{Name: "systool", Path: first, Sources: []string{"path"}},
					{Name: "systool", Path: second, Sources: []string{"path"}},
				}
			}
		}
		return nil
	})
	defer restore()

	log := &noticeLog{}
	a := reconnectApp(t, paths, log)
	a.chooseTool = func(name string, candidates []extensions.ExecutableCandidate) (string, bool) {
		if name != "Moved Tool" {
			t.Errorf("chooser named %q, want the tool", name)
		}
		return candidates[1].Path, true
	}
	a.reconnectIntegration(id, nil)

	want := i18n.For("en").Notifications.IntegrationReconnected("Moved Tool")
	if got := waitForNotice(t, log); got != want {
		t.Errorf("notice = %q, want %q", got, want)
	}
	integration, ok := a.Integrations.Inventory().WithID(id)
	if !ok || integration.Entry.ExecutablePath != second {
		t.Fatalf("entry = %+v, want the program rebound to the pick %s", integration.Entry, second)
	}
}
