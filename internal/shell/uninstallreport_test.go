package shell

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/settings"
	"floter/internal/settingsui"
)

// A componentized removal reports back to the dialog that asked. The reference
// awaited the mutation and closed the dialog only when it landed, and told the
// user through the app's notice stack either way
// (`src/ExtensionsPanel.tsx:1314-1341`). The shell is the only side that knows
// when the work finishes, so it owns the completion callback and hands it the
// outcome on the main thread.
func TestComponentizedUninstallReportsBackToTheDialog(t *testing.T) {
	paths := integrationFixture(t)
	notified := [][2]string{}
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		Notify:      func(title, body string) { notified = append(notified, [2]string{title, body}) },
	})

	done := make(chan error, 1)
	a.Settings.Actions.UninstallComponents("io.github.vst93.v", "V Tools", settingsui.RemovalNPM,
		settingsui.UninstallComponents{RemoveHostConfig: true, RemoveToolData: true, RemoveArtifacts: true},
		func(err error) { done <- err })
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a clean removal reported %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the removal never reported back")
	}
	if len(notified) != 1 || !strings.Contains(notified[0][1], "V Tools") {
		t.Errorf("the success did not raise the removal notice: %v", notified)
	}
	if _, err := os.Stat(filepath.Join(paths.Extensions, "io.github.vst93.v")); !os.IsNotExist(err) {
		t.Errorf("the package was not removed: %v", err)
	}

	// A failure reports the error back — so the dialog can stay open — and
	// still raises the failure sentence the user needs to learn why.
	notified = nil
	a.Settings.Actions.UninstallComponents("nope", "Ghost", settingsui.RemovalNPM,
		settingsui.UninstallComponents{}, func(err error) { done <- err })
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("an unknown integration reported success")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the failed removal never reported back")
	}
	if len(notified) != 1 || !strings.Contains(notified[0][1], "Ghost") {
		t.Errorf("the failure did not raise the removal-failed notice: %v", notified)
	}
}
