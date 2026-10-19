//go:build !windows

package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/settingsui"
)

// The drift re-probe's visibility, end to end: the reference's R7-7 made a
// silent background re-probe visible exactly once per real change
// (`src-tauri/src/extensions/install.rs:843-975`, `src/ExtensionsPanel.tsx:2078-2132`).
// These tests pin the shell's wiring — which integrations are checked, the
// fail-soft contract, the once-per-session gate, and the toast's own sentence
// and action — while `internal/extensions` owns the pure decision.

// driftAppFixture installs a generated custom integration whose executable
// reports `version`, with `recorded` as the version the repository holds. The
// package shape is the connect flow's own (`<data>/<id>/integration`).
func driftAppFixture(t *testing.T, version, recorded string) (extensions.Paths, string) {
	t.Helper()
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	const id = "dev.floter.drifttool"
	pkg := filepath.Join(paths.Extensions, id, "integration")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.drifttool", "name": "Drift Tool",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "system", "executableNames": ["drifttool.sh"]},
	  "provider": {"type": "static-descriptor", "descriptor": "provider-description.json"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(pkg, "drifttool.sh")
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "drifttool ` + version + `"
  exit 0
fi
if [ "$1" = "--help" ]; then
  echo "drifttool - Gadgets"
  echo "Available Plugins"
  echo "📦 one 1.0"
  echo "📦 two 1.0"
  exit 0
fi
exit 3
`
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// The descriptor starts with only the root command, and the freshness
	// sidecar beside it records that scan: the state a connected integration is
	// really in after its connect-time derivation (one command, no earlier
	// count). A later drift has a previous count to report only because that
	// first scan wrote one.
	if err := os.WriteFile(filepath.Join(pkg, "provider-description.json"), []byte(`{
	  "protocolVersion": "1.0",
	  "provider": {"id": "dev.floter.drifttool", "name": "Drift Tool", "version": "1.0.0"},
	  "commands": [{"id": "drifttool", "name": "Drift Tool", "description": "root",
	    "execution": {"program": "self", "argsPrefix": [], "mode": "pty"}}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "help-probe.json"), []byte(`{
	  "probedAt": 1,
	  "rootArgumentCount": 0,
	  "subcommandCount": 0,
	  "commandCount": 1
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	repository := `{
	  "schemaVersion": 1,
	  "extensions": {"` + id + `": {"id": "` + id + `", "name": "Drift Tool",
	    "publisherId": "local-user", "publisherName": "Local user",
	    "distributionSource": "local", "runtimeOwnership": "system",
	    "providerKind": "static-descriptor", "state": "enabled", "enabled": true,
	    "manifestPath": "` + jsonEscaped(filepath.Join(pkg, "floter.extension.json")) + `",
	    "executablePath": "` + jsonEscaped(executable) + `",
	    "toolVersion": "` + recorded + `",
	    "channel": "stable", "installedAt": 1, "updatedAt": 1}}
	}`
	if err := os.WriteFile(paths.RepositoryFile, []byte(repository), 0o644); err != nil {
		t.Fatal(err)
	}
	return paths, id
}

// driftApp builds the app with the extension state loaded, but *without*
// kicking the drift loop: a test drives the checks itself. Calling
// `RefreshIntegrations` here would race the test against the background probe
// the listing spawns.
func driftApp(t *testing.T, paths extensions.Paths) *App {
	t.Helper()
	a := New(Options{
		Store: settings.NewStore(settings.Default()),
		Paths: paths,
	})
	a.Integrations.Refresh(context.Background())
	a.onMain(func() { a.Launcher.SetCommands(a.launcherCommands()) })
	return a
}

// driftToast finds the queued drift notice, empty when none is waiting.
func driftToast(a *App) ui.Toast {
	a.noticeMu.Lock()
	defer a.noticeMu.Unlock()
	for _, toast := range a.notices {
		if toast.ID == driftNoticeID {
			return toast
		}
	}
	return ui.Toast{}
}

// A moved version raises exactly one success toast, whose sentence is the
// reference's `settings.extensions.reprobeNotice` with the signed delta, and
// whose action is `settings.extensions.viewDetails`.
func TestTheDriftReprobeAnnouncesAChangedCommandList(t *testing.T) {
	paths, id := driftAppFixture(t, "2.0.0", "1.0.0")
	a := driftApp(t, paths)
	if !a.reprobeDriftedIntegration(id) {
		t.Fatal("a moved version did not re-probe")
	}
	toast := driftToast(a)
	if toast.Title == "" {
		t.Fatal("a changed command list raised no toast")
	}
	copy := i18n.For("en").Settings
	// 1.0.0's descriptor listed one command; the re-probe's help derivation
	// lists the root plus two subcommands = 3, so the delta is +2.
	want := copy.ReprobeNotice("Drift Tool", copy.ReprobeDeltaIncrease(2))
	if toast.Title != want {
		t.Errorf("the toast reads %q, want %q", toast.Title, want)
	}
	if toast.Type != "success" {
		t.Errorf("the toast's kind = %q, want success", toast.Type)
	}
	if toast.Action != copy.ViewDetails {
		t.Errorf("the toast's action = %q, want %q", toast.Action, copy.ViewDetails)
	}
	if toast.Timeout != successNotice {
		t.Errorf("the toast's window = %v, want %v", toast.Timeout, successNotice)
	}
	// The recorded version moved, so a second check is a no-op.
	if a.reprobeDriftedIntegration(id) {
		t.Error("the same drift re-probed twice")
	}
}

// The gate is the once-per-session rule: the same change announced twice stays
// silent the second time, and the no-op frames that came before it never burned
// the slot.
func TestTheDriftNoticeIsAnnouncedOnlyOnce(t *testing.T) {
	paths, id := driftAppFixture(t, "2.0.0", "1.0.0")
	a := driftApp(t, paths)
	if !a.reprobeDriftedIntegration(id) {
		t.Fatal("the first check did not re-probe")
	}
	first := driftToast(a)
	if first.Title == "" {
		t.Fatal("the first change raised no toast")
	}
	// Clear the queue (as a frame would) and re-offer the same frame by hand:
	// the gate must refuse it.
	a.noticeMu.Lock()
	a.notices = nil
	a.noticeMu.Unlock()
	frame := extensions.ReprobeNoticeFrame{
		ExtensionID:          id,
		ToolVersion:          "2.0.0",
		PreviousCommandCount: intPointer(1),
		CommandCount:         intPointer(3),
	}
	decision := extensions.DecideDriftNotice(frame, "Drift Tool", a.driftNoticeGate())
	if decision.Announce != nil {
		t.Error("the same change was announced twice")
	}
	// A genuinely different change still speaks: another version, another
	// magnitude.
	other := extensions.ReprobeNoticeFrame{
		ExtensionID:          id,
		ToolVersion:          "3.0.0",
		PreviousCommandCount: intPointer(3),
		CommandCount:         intPointer(5),
	}
	if extensions.DecideDriftNotice(other, "Drift Tool", a.driftNoticeGate()).Announce == nil {
		t.Error("a different change was silenced by the gate")
	}
}

// A tool that reports no version never probes, so no toast is raised and the
// repository's recorded version is left alone.
func TestTheDriftReprobeStaysSilentWithoutAVersion(t *testing.T) {
	paths, id := driftAppFixture(t, "noversion", "1.0.0")
	// The version probe answers with a word, not a semver.
	a := driftApp(t, paths)
	if a.reprobeDriftedIntegration(id) {
		t.Error("a tool that reports no version re-probed")
	}
	if toast := driftToast(a); toast.Title != "" {
		t.Errorf("an unmoved list raised %q", toast.Title)
	}
}

// A failed re-probe is soft: the row is untouched, no toast is raised, and the
// previous descriptor stays.
func TestTheDriftReprobeFailsSoft(t *testing.T) {
	paths, id := driftAppFixture(t, "2.0.0", "1.0.0")
	// Replace the executable with one that answers its version but nothing
	// usable for a derivation, so the re-probe errors.
	executable := filepath.Join(paths.Extensions, id, "integration", "drifttool.sh")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo drifttool 2.0.0; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := driftApp(t, paths)
	if a.reprobeDriftedIntegration(id) {
		t.Error("a re-probe that derived nothing reported a change")
	}
	if toast := driftToast(a); toast.Title != "" {
		t.Errorf("a failed re-probe raised %q", toast.Title)
	}
	// The recorded version did not move, so a later listing retries.
	integration, ok := a.Integrations.Inventory().WithID(id)
	if !ok {
		t.Fatalf("the integration vanished from the inventory: %s", id)
	}
	if integration.Entry.State == "broken" {
		t.Error("a failed re-probe marked the entry broken")
	}
}

// The in-flight gate keeps a listing storm from stacking probes: the first
// claim wins and every claim until the release is refused.
func TestTheDriftReprobeLoopDoesNotStack(t *testing.T) {
	a := newApp(t)
	if !a.startDriftReprobe() {
		t.Fatal("the first claim was refused")
	}
	if a.startDriftReprobe() {
		t.Error("a second probe was allowed while the first was in flight")
	}
	a.finishDriftReprobe()
	if !a.startDriftReprobe() {
		t.Error("the claim was not released")
	}
}

// The announcement reaches the screen: the toast viewport draws the drift
// toast's sentence and its action chip, and the chip opens the integrations
// page.
func TestTheDriftToastIsDrawnWithItsAction(t *testing.T) {
	paths, id := driftAppFixture(t, "2.0.0", "1.0.0")
	a := driftApp(t, paths)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()

	if !a.reprobeDriftedIntegration(id) {
		t.Fatal("the check did not re-probe")
	}
	tt.Frame()
	copy := i18n.For("en").Settings
	sentence := copy.ReprobeNotice("Drift Tool", copy.ReprobeDeltaIncrease(2))
	if !tt.HasText(sentence) {
		t.Fatalf("the toast's sentence is not drawn: %q", tt.Texts())
	}
	if !tt.HasText(copy.ViewDetails) {
		t.Fatalf("the toast's action is not drawn: %q", tt.Texts())
	}
	if err := tt.Click(copy.ViewDetails); err != nil {
		t.Fatalf("press the action: %v", err)
	}
	tt.Frame()
	if a.Surf != SurfaceSettings {
		t.Errorf("the action left the surface at %v, want settings", a.Surf)
	}
	if !strings.Contains(strings.ToLower(settingsui.PageName(a.Settings.Page)), "integration") {
		t.Errorf("the action landed on page %q, want integrations", settingsui.PageName(a.Settings.Page))
	}
}

// A listing driven through RefreshIntegrations reaches the same toast, on the
// goroutine the listing spawns.
func TestRefreshIntegrationsRaisesTheDriftToast(t *testing.T) {
	paths, _ := driftAppFixture(t, "2.0.0", "1.0.0")
	a := driftApp(t, paths)
	// The listing itself: the toast is raised by the goroutine the listing
	// spawns, so the test waits for it rather than driving the check by hand.
	a.RefreshIntegrations(context.Background())
	deadline := time.Now().Add(5 * time.Second)
	for {
		if driftToast(a).Title != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a listing never raised the drift toast")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The claim is released when the loop ends, so a later listing can run
	// again rather than being refused by a stuck flag.
	deadline = time.Now().Add(5 * time.Second)
	for a.driftLoopRunning() {
		if time.Now().After(deadline) {
			t.Fatal("the drift loop never released its claim")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
