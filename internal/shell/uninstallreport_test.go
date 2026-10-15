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

// The plain removal and the disable write report back the same way the
// componentized one does. The reference's removal bar and its disable band both
// awaited the mutation and only dismissed themselves once it landed
// (`src/ExtensionsPanel.tsx:1274-1283,1296-1304`); the shell is the side that
// knows when the work finished, so it owns the callback and hands the outcome
// back on the main thread.
func TestPlainRemovalReportsBackToTheBar(t *testing.T) {
	paths := integrationFixture(t)
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})

	done := make(chan error, 1)
	a.Settings.Actions.UninstallIntegration("io.github.vst93.v", "V Tools", settingsui.RemovalPackage, func(err error) { done <- err })
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a clean removal reported %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the removal never reported back")
	}
	if _, err := os.Stat(filepath.Join(paths.Extensions, "io.github.vst93.v")); !os.IsNotExist(err) {
		t.Errorf("the package was not removed: %v", err)
	}

	// An unknown id fails the write and the error reaches the band, so it can
	// stay open with the failure on it.
	a.Settings.Actions.UninstallIntegration("nope", "Ghost", settingsui.RemovalPackage, func(err error) { done <- err })
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("an unknown integration reported success")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the failed removal never reported back")
	}
}

// The disable band's answer comes back the same way: the reference awaited
// `extensions_disable` and only then cleared `disableTarget`
// (`src/ExtensionsPanel.tsx:1274-1283`). A missing id is the failure the band
// has to show.
func TestDisableReportsBackToTheBand(t *testing.T) {
	paths := integrationFixture(t)
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})

	done := make(chan error, 1)
	a.Settings.Actions.SetIntegrationEnabled("io.github.vst93.v", false, func(err error) { done <- err })
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a clean disable reported %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the disable never reported back")
	}
	repo, err2 := extensions.LoadRepository(paths.RepositoryFile)
	if err2 != nil {
		t.Fatalf("the repository could not be read back: %v", err2)
	}
	if got := repo.Extensions["io.github.vst93.v"]; got.Enabled {
		t.Errorf("the disable did not write the flag: %+v", got)
	}

	a.Settings.Actions.SetIntegrationEnabled("nope", false, func(err error) { done <- err })
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("an unknown integration reported success")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the failed disable never reported back")
	}
}
