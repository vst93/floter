// Package settingsui draws the settings surface: a sidebar of pages beside
// a body.
//
// The General page's controls bind to the settings store, which writes
// every change back to the same JSON file while preserving the keys this
// build does not own (see internal/settings); the terminal group drives the
// terminal surface's appearance (see internal/terminalui). The other pages
// route but say they arrive later.
package settingsui

import (
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"floter/internal/control"
	"floter/internal/extensions"
	"floter/internal/glassmap"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/scriptlang"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/terminalui"
	"floter/internal/theme"
)

// The settings pages, in sidebar order.
//
// The built-in plugins have no page: their configuration is the launcher's own
// sheet, rendered from the schema a plugin declares (see `PluginSheet`). The
// old build retired its plugin pages for the same reason in R33 — 「配置页面通用
// 化：点击设置之后，弹出一个基于通用规则的配置页面，而不是一个新的完全独立的
// 页面」 — and a page here would be a second form of the same settings, free to
// drift from the first.
const (
	PageGeneral = iota
	PageSessions
	PageShortcuts
	PageIntegrations
	PageAbout

	pageCount
)

// pageNames are the ids the `last_settings_page` setting stores, in the same
// order as the pages: reopening the settings surface returns to the page the
// user last looked at. A file that still names `plugins` (a build that had the
// page) resolves to the first page, since `PageByName` refuses what it does not
// know.
var pageNames = [pageCount]string{"general", "sessions", "shortcuts", "integrations", "about"}

// PageByName resolves a stored page id, reporting whether it is one this build
// knows.
func PageByName(name string) (int, bool) {
	for i, page := range pageNames {
		if page == name {
			return i, true
		}
	}
	return PageGeneral, false
}

// IntegrationsPage is the page index for a caller outside the package.
func IntegrationsPage() int { return PageIntegrations }

// PageName is the stored id of a page index.
func PageName(page int) string {
	if page < 0 || page >= pageCount {
		return pageNames[PageGeneral]
	}
	return pageNames[page]
}

// Actions are the shell's callbacks.
type Actions struct {
	// Close leaves the settings surface (Escape, the header's close button),
	// and Quit leaves the application.
	Close func()
	Quit  func()
	// CloseSession ends the running terminal session, from the Sessions
	// page.
	CloseSession func()
	// SelectSession shows the session at an index, and CloseSessionAt ends
	// it: the sessions list is a list of the terminal's own sessions, not a
	// single one, so a click has to name which.
	SelectSession  func(index int)
	CloseSessionAt func(index int)
	// NewSession starts another terminal session.
	NewSession func()
	// SetIntegrationEnabled turns an installed extension on or off.
	//
	// done reports the write back to the confirmation band that asked, on the
	// main thread: a nil error closes it (the reference's
	// `setDisableTarget(null)` after `await runMutation`), and a non-nil error
	// leaves the band open with its answer pill usable again, exactly as the
	// reference's `runMutation` `finally` did (`src/ExtensionsPanel.tsx:1274-1283`).
	// It may be nil for the enable direction, which asks nothing.
	SetIntegrationEnabled func(id string, enabled bool, done func(error))
	// UninstallIntegration removes an installed extension. The kind is the
	// same `removalKind` the confirmation bar spoke in, so the shell's
	// completion notification can be the kind's own word rather than one
	// generic "removed" (`src/ExtensionsPanel.tsx:1286-1340`).
	//
	// done reports the removal back to the band that asked, on the main thread:
	// a nil error closes it (the reference's `setRemovalTarget(null)` after
	// `await runMutation`), and a non-nil error leaves it open with its answer
	// pill usable again (`src/ExtensionsPanel.tsx:1296-1304`). It may be nil, in
	// which case the band closes at once.
	UninstallIntegration func(id string, name string, kind string, done func(error))
	// UninstallComponents removes the parts of an installed extension the
	// checkboxes name. Nil falls back to UninstallIntegration. The kind is
	// passed for the same reason as above: custom is "deleted", npm is
	// "uninstalled".
	//
	// done reports the removal back to the dialog that asked, on the main
	// thread: a nil error closes it (the reference's
	// `setUninstallDialogTarget(null)` after `await`), and a non-nil error
	// leaves it open with its answer buttons usable again, exactly as the
	// reference's `runMutation` `finally` did (`src/ExtensionsPanel.tsx:1314-1341`).
	// It may be nil, in which case the caller closes the dialog at once.
	UninstallComponents func(id string, name string, kind string, components UninstallComponents, done func(error))
	// DiagnoseIntegration asks an integration's provider to check itself.
	DiagnoseIntegration func(id string)
	// RepairIntegration re-checks an installed package and repairs it when the
	// record drifted from what is on disk: the old host's `extensions_repair`,
	// behind the Wrench a broken row shows. Nil hides the button.
	//
	// done reports the re-check back to the row that asked, on the main thread:
	// the row's button leaves its in-flight face on the answer, whatever it
	// was, exactly as the reference's `runMutation` cleared `busy` in `finally`
	// (`src/ExtensionsPanel.tsx:1190-1210`). It may be nil, in which case the
	// row settles itself.
	RepairIntegration func(id string, done func(error))
	// ReconnectIntegration re-derives a system integration's runtime binding
	// from the search path: the old host's `extensions_reconnect_system`, and
	// the only repair a row whose program moved can offer. Nil hides the
	// button. done is the same in-flight hand-off as RepairIntegration's.
	ReconnectIntegration func(id string, done func(error))
	// OpenHomepage sends the user to an integration's own page — the
	// reference's `installTool` arm for a row whose runtime floter does not
	// own and cannot look for. Nil hides that button.
	OpenHomepage func(url string)
	// ReprobeCommands re-runs the connect-time help derivation of a
	// generated custom integration and rebuilds its command list. Nil
	// hides the button.
	ReprobeCommands func(id string)
	// ReprobeHealth re-runs an integration's stored lifecycle probes and
	// writes the new report back to the repository: the reference's
	// `extensions_reprobe` (`ExtensionsPanel.tsx:1139-1155`, `handleReprobe`),
	// which the Health block's own refresh button calls
	// (`:2908-2918`). Nil hides that button, exactly as the reference draws
	// it only when a report is on screen.
	ReprobeHealth func(id string)
	// SetShortcut records a new accelerator for an action id.
	SetShortcut func(id, accelerator string)
	// InstallFromRegistry installs a package from the npm registry.
	InstallFromRegistry func(name, constraint string)
	// SetCommandEnabled turns one of an integration's commands on or off: a
	// command only appears in the launcher while its switch is on.
	SetCommandEnabled func(extensionID, commandID string, enabled bool)
	// SetCommandAlias records one command's alias (an empty alias removes
	// the entry) and re-hands the launcher its command list.
	SetCommandAlias func(extensionID, commandID, alias string)
	// AdoptIntegration grafts an orphan package into the repository, and
	// DeleteOrphan removes an orphan package directory.
	AdoptIntegration func(id string)
	DeleteOrphan     func(id string)
	// InstallUpdate downloads and installs the update the last check found,
	// re-checking first the way the old build's `downloadAndInstallUpdate`
	// did. Nil hides the update row's control.
	InstallUpdate func()
	// ExportIntegrations writes the installed integrations to a file the user
	// picks, and ImportIntegrations applies one they pick.
	ExportIntegrations func()
	ImportIntegrations func()
	// RunWithParams runs an integration's command with the answers to its
	// declared inputs; the host turns them into argv and refuses an illegal
	// combination with a `run_param_*` key, which the form words under itself.
	// Nil hides the form.
	RunWithParams func(id string, values map[string]string) *extensions.ParamFailure
	// OpenConfiguration runs a tool-owned configuration's `openCommand` argv:
	// the reference's `onOpenCommand(plan, …)` arm for a `tool` owner, which
	// spawns the provider's own editor (`ExtensionsPanel.tsx:2982`). Nil
	// leaves the card's "Open configuration" button disabled.
	OpenConfiguration func(id string, argv []string)
	// InstallFromFolder installs a package directory the user picks, and
	// ChooseFolder asks them for one. Nil hides the row.
	InstallFromFolder func(dir string)
	ChooseFolder      func() (string, error)
	// Detected lists the programs the machine has that are not yet connected,
	// and ConnectDetected connects the one the user picked. Nil hides the
	// section.
	Detected        func() []DetectedTool
	ConnectDetected func(tool DetectedTool)
	// InstallDetected opens a terminal on the install command of a detected
	// tool that has a recipe but is not runnable yet — the reference's
	// `installInTerminal` arm, which hands the command to the user's own shell
	// (`src/ExtensionsPanel.tsx:2562-2574`). Nil leaves that button disabled.
	InstallDetected func(command string)
	// Freshness reports what is known about one integration's command list
	// (the help-probe sidecar and the health report); nil hides the row.
	Freshness func(id string) (extensions.Freshness, bool)
	// CopyText puts text on the clipboard, for the About page's deep links.
	CopyText func(text string)
	// ExportScript writes the local-tool form's script to a file the user
	// picks: the reference's `extensions_custom_export_script`, behind the
	// "Export script" button beside "Copy script"
	// (`CustomIntegrationDrawer.tsx:145`, `ExtensionsPanel.tsx:1951-1964`).
	// name is the file name the save dialog offers (the integration's id, the
	// word "script", and the language's extension); the host owns the dialog,
	// the write, and the success toast. Nil hides the button.
	ExportScript func(name, content string)
	// EditCustomTool rewrites a generated integration from the same form the
	// connect flow uses (the id names it, so its identity is kept), and
	// ScriptRuntime answers what this machine has for a script language: the
	// interpreter on PATH and its version, or nothing. Nil means the form
	// cannot say, and shows the toolchain's name alone.
	ScriptRuntime func(language string) scriptlang.Check
	// Invalidate asks for another frame, from whatever goroutine the probe
	// finished on: mygo allows it from any goroutine, and the shell wires it to
	// the window. Nil just means no redraw is requested, so a headless test
	// drives its own frames.
	Invalidate func()
	// NotifyWarning reports a warning the form cannot draw for itself: the old
	// panel's `onNotify("warning", …)`, which reached the app's own notice
	// stack rather than the form — the message is about the machine, not about
	// a row the user is editing. The integration form raises one when it saves
	// a script whose toolchain this machine does not have; the save goes
	// through either way, so it is advice, never a refusal. Nil drops the
	// message.
	NotifyWarning func(text string)
	// NotifySuccess reports a confirmation the panel cannot draw for itself:
	// the old panel's `onNotify("success", …)`. The About page's deep-link
	// copy raises one — the reference's `DeepLinkRow.onCopied` reached the
	// app's toast stack, and a clipboard write is otherwise silent. Nil drops
	// the message.
	NotifySuccess func(text string)
	// ToolForEdit hands the form what that integration declares now. Both nil
	// hides the button.
	EditCustomTool func(id string, tool CustomTool)
	ToolForEdit    func(id string) (CustomTool, bool)
	// ConnectRecommended installs one of the shipped tool packages, and
	// ConnectLocalTool connects a program already on this machine.
	ConnectRecommended func(id string)
	ConnectLocalTool   func(tool CustomTool)
	// ChooseProgram asks the user to point at a program; nil hides the picker.
	ChooseProgram func() (string, error)
	// SaveConfiguration validates and stores an integration's configuration
	// values, moving its password fields into the secrets file. `done` reports
	// the write back on the main thread, whatever it did — the reference's
	// `runMutation` cleared `busy` in `finally`
	// (`ExtensionsPanel.tsx:1993-2007`) — so the Save's in-flight face is
	// swapped for the reference's `configSaving` word until the answer lands.
	// It is called exactly once, synchronously or from a goroutine.
	SaveConfiguration func(id string, values map[string]any, done func(error))
	// ExportConfiguration asks for a path and writes one integration's
	// host-owned configuration there, as the reference's
	// `extensions_config_export` did (`ExtensionsPanel.tsx:2036-2048`): the
	// toolbar's export icon-button. Values are the form's current draft, so a
	// user can export an unsaved edit. The host owns the document, the dialog
	// and the write, so the schema it validates against is the real one, never
	// a copy the form made. `done` is the same in-flight hand-off as
	// SaveConfiguration's; nil hides the button.
	ExportConfiguration func(id string, values map[string]any, done func(error))
	// CopyConfiguration puts one integration's host-owned configuration on the
	// clipboard as the reference's `extensions_config_copy` did
	// (`ExtensionsPanel.tsx:2019-2031`): the toolbar's copy icon-button. Like
	// `ExportConfiguration`, the host builds the document from the real schema
	// and raises the success notice; `done` is the same in-flight hand-off. Nil
	// turns the button into a no-op.
	CopyConfiguration func(id string, values map[string]any, done func(error))
	// ClearClipboardHistory drops every clipboard entry that is not a
	// favourite.
	ClearClipboardHistory func()
	// OpenPluginConfig opens one built-in plugin's declarative configuration
	// in the launcher — the same sheet the mode's own gear opens. Nil hides
	// the Configure buttons on the base-plugin rows.
	OpenPluginConfig func(id string)
	// SetPluginEnabled turns one built-in plugin's own switch on or off (the
	// clipboard history and the browser search; the calculator has none).
	// Nil hides the switches.
	SetPluginEnabled func(id string, enabled bool)
	// ResetShortcuts restores the shipped bindings, and
	// SetShortcutsSuspended releases the global shortcuts while a recorder
	// waits for a key (so pressing the current one is recorded rather than
	// acted on).
	ResetShortcuts        func()
	SetShortcutsSuspended func(suspended bool)
	// CustomShortcuts reports the user-defined global shortcuts, and
	// SetCustomShortcuts persists a new list and registers it, returning the
	// keys the system refused.
	CustomShortcuts    func() []settings.CustomShortcut
	SetCustomShortcuts func([]settings.CustomShortcut) []CustomShortcutRejection
	// SetPage records the page the user switched to (the
	// `last_settings_page` setting).
	SetPage func(name string)
}

// Integration is one installed extension, as the Integrations page lists it.
type Integration struct {
	ID          string
	Name        string
	Description string
	Publisher   string
	Version     string
	ToolVersion string

	Enabled bool
	// Running means it contributes commands; Broken that its runtime is
	// unusable; Error why its provider could not be asked.
	Running bool
	Broken  bool
	Error   string
	// FailureCode and FailureDetail are the failure the repository recorded
	// on the entry (R11): the code is a dictionary key and the detail is
	// usually a keyed payload. They stay raw so a language switch re-renders
	// the reason, and `failureReason` owns the words.
	FailureCode   string
	FailureDetail string
	// Orphan is a package directory no repository entry names.
	Orphan bool
	// Permissions are the ids the package declares, and Enforced which of
	// them the host itself decides.
	Permissions []string
	Enforced    map[string]bool
	// Diagnosis is what the last health check said, empty when none ran.
	Diagnosis string
	// DiagnosisFailed marks a check that reported a problem.
	DiagnosisFailed bool
	// DiagnosisChecks is what the last check looked at, in the provider's own
	// order: each check's id, the provider's status word for it and its
	// sentence. The reference's `.extension-diagnostics` grid prints one row
	// per check, so the status a single `Diagnosis` string could only
	// summarise is readable fact by fact
	// (`main:src/ExtensionsPanel.tsx:2895-2901`).
	DiagnosisChecks []DiagnosisCheck
	// DiagnosisStatus is the provider's own overall verdict for that check
	// (`diagnose.status`, `ExtensionsPanel.tsx:2896`), which the grid's summary
	// line prints in the provider's vocabulary. Empty when the provider named
	// none.
	DiagnosisStatus string
	// Health is the stored lifecycle probe report's status, empty when the
	// manifest declares no probes. It survives across runs (the repository
	// keeps it), so a row shows how the tool was last found to be.
	Health string
	// HealthStatus is the **raw** status word `Health` was printed from
	// (`extensions.HealthHealthy` &c.), which is what the chip's own face is
	// keyed on: the reference writes the modifier class straight from the
	// report (`className={`extension-health__status
	// extension-health__status--${healthReport.status}`}`,
	// `main:src/ExtensionsPanel.tsx:2924`), never from the translated word.
	// Reading the localized word back meant a word-table change — or a status
	// the four-word ladder does not name — silently dropped the chip to the
	// transparent `unknown` face. Empty when the row carries no report.
	HealthStatus string
	// HealthReport is that same stored report's detail: every probe with its
	// duration, the failed ones with their exit code and stderr, and when the
	// run happened. Nil when nothing has ever probed the integration.
	HealthReport *HealthReport
	// Approval is the recorded permission approval, as the card draws it: what
	// was approved, when, and against which manifest — with the on-disk
	// digest beside it, so "what I approved" and "what is here now" are two
	// readable facts (the old host's R7-8/R7-8b block). Nil when no approval
	// has ever been recorded.
	Approval *ApprovalRecord
	// Commands are the commands the integration declares, each with the
	// switch state the launcher reads.
	Commands []Command
	// Config is the schema the integration declares for its host-owned
	// configuration, and ConfigValues the values this machine holds (a
	// password's real value, which the form hides as it is typed). Both are
	// empty when the integration declares none.
	Config       []ConfigField
	ConfigValues map[string]any
	// ConfigOwner is the provider's own word for who holds the values: the
	// reference's `configuration.descriptor.owner` (`ExtensionsPanel.tsx:2975`).
	// Only a `host`-owned configuration is the form's to draw — a tool's own
	// configuration is opened where the tool keeps it — so a `tool` owner
	// hides the form's section (see configOwnerIsHost).
	ConfigOwner string
	// ConfigOpenCommand is the tool-owned configuration's open command: the
	// provider's `openCommand` argv, carried so the card can offer the
	// reference's "Open configuration" button
	// (`ExtensionsPanel.tsx:2977-2986`). Empty when there is nothing to open,
	// in which case the button is drawn disabled — the reference's
	// `disabled={!configuration.openPlan}`.
	ConfigOpenCommand []string
	// ConfigVersion is the schema generation the stored values were written
	// against, carried so a copied or exported configuration names it (the
	// reference's `ConfigurationExportDocument.config_version`,
	// `src-tauri/src/extensions/config.rs:88-93`). Zero when nothing is known.
	ConfigVersion int
	// Generated marks an integration the host itself created from a local
	// tool's own help: only these can be re-probed, so only these get the
	// button.
	Generated bool
	// DistributionSource and RuntimeOwnership are the repository's own kind
	// fields (`npm`/`local`/`built-in`, and `bundled`/`system`). Together with
	// Generated they are what the card's removal verb is derived from, exactly
	// as the reference's `removalKind` read them
	// (`src/extensions/ExtensionRow.tsx:109-114`).
	DistributionSource string
	RuntimeOwnership   string
	// RuntimeSource is where the runtime lives, the reference's own
	// `runtimeSource` projection (`managed`/`system`/`bundled`,
	// `src-tauri/src/commands/extensions.rs:540-550`): a package that bundles
	// its runtime is `bundled`, an executable or descriptor runtime is
	// `managed` when Floter owns it and `system` when the machine does. It is
	// the meta line's third fact, beside the package identity and the kind.
	RuntimeSource string
	// Highlighted marks the integration a `floter://register` link named, so
	// its card wears the link's mark
	// (`src/extensions/ExtensionRow.tsx:196-205,290`). It is a status mark,
	// not a control: nothing about the card's actions changes.
	Highlighted bool
	// RuntimeAvailable says the runtime the entry was bound to still
	// resolves: the program is on disk, or the interpreter a script needs is
	// installed. A row whose runtime is gone offers the reference's repair
	// action instead of a run it could not perform
	// (`src/extensions/ExtensionRow.tsx:345-370`).
	RuntimeAvailable bool
	// Homepage is the project's own page when the manifest names one. It is
	// where the repair sends a user when floter has no runtime of its own to
	// look for, the reference's `installTool` arm.
	Homepage string
	// Params are the declared inputs a run needs: the card renders a form for
	// them, and the answers become argv.
	Params []Param
	// OutputMode is the manifest's declared route for a run: `terminal` (a
	// live session on the terminal page) or `background` (a captured run
	// that reports through a notice). The run form states it up front, as the
	// reference's `RunParamForm` did with `extension.output`
	// (`src/extensions/ExtensionRow.tsx:465-467`).
	OutputMode string
}

// DiagnosisCheck is one thing a provider's last health check looked at, as
// the `.extension-diagnostics` grid draws it: the provider's own status word
// (never translated — it is the vocabulary the provider answered in) and its
// message.
type DiagnosisCheck struct {
	ID      string
	Status  string
	Message string
}

// HealthReport is the stored lifecycle probe report's detail, as the card
// draws it: when the run happened and what each probe did.
type HealthReport struct {
	// CheckedAt is the run's RFC 3339 timestamp, as the repository stored it.
	CheckedAt string
	Probes    []HealthProbe
}

// ApprovalRecord is the repository's stored approval, projected for the
// card: when it happened, the permission ids it covered (tiered by the same
// host-enforced vocabulary the permission audit uses), and the manifest
// digest it is bound to — beside the digest of the manifest on disk now, so
// a post-approval change is two rows the user can read, not a boolean.
type ApprovalRecord struct {
	// At is the approval's Unix seconds, as the repository recorded them.
	At uint64
	// ApprovedDigest is the manifest digest the approval is bound to
	// (`sha256-…`), empty when the record predates the field.
	ApprovedDigest string
	// CurrentDigest is the digest of the manifest on disk now, empty when it
	// could not be read. "Unknown" stays distinguishable from "changed".
	CurrentDigest string
	// Permissions are the approved ids, split into the two tiers by the
	// host's own classifier (an unknown permission is disclosure, never
	// enforced: over-claiming enforcement is the failure mode that would
	// make the UI lie). Each group preserves the record's own order.
	Enforced   []string
	Disclosure []string
}

// HealthProbe is one probe record of a stored report.
type HealthProbe struct {
	ID         string
	Passed     bool
	DurationMs uint64
	// ExitCode is -1 when the probe never ran to an exit.
	ExitCode int
	Stderr   string
	// Retryable is true for an optional probe: a failed required probe is a
	// broken tool, not a transient one. It is projected from the manifest's
	// own `required`, so it never has to be persisted twice.
	Retryable bool
}

// Param is one declared input, as the form renders it.
type Param struct {
	ID          string
	Label       string
	Kind        string // text, number, boolean, select, path
	Default     string
	Required    bool
	Placeholder string
	Options     []string
	Flag        string
}

// ConfigField is one field of an integration's host-owned configuration, as
// the settings form renders it.
type ConfigField struct {
	Key         string
	Label       string
	Type        string // text, password, path, select, multiSelect, boolean, number
	Description string
	Required    bool
	Default     any
	Options     []string
	Minimum     *float64
	Maximum     *float64
	MinLength   *int
	MaxLength   *int
}

// Command is one of an integration's commands, as the switch list shows it.
type Command struct {
	ID          string
	Name        string
	Description string
	// Aliases are the descriptor's own declared aliases, which together with
	// the id and the name form the summon vocabulary the panel prints — the
	// reference's `pluginCommandTriggers(command)` reads exactly these three
	// sources (`plugins/external.ts:86-97`), so the words the panel shows are
	// the words that enter the mode.
	Aliases []string
	// Enabled is whether the command may be summoned from the launcher.
	Enabled bool
	// Declared is whether the provider's descriptor declared this command, as
	// opposed to one the host synthesized (the tool-owned configuration
	// editor). The reference prints the summon words only for a command its
	// registry knows (`ExtensionsPanel.tsx:2824-2829`), and the synthesized
	// command is never in it (`main:src-tauri/src/extensions/catalog.rs:427-449`).
	Declared bool
	// Available is whether the integration's runtime resolves right now: a
	// command whose runtime is missing is still listed, and says so.
	Available bool
	// Alias is what the user types to summon this command. Taken marks an
	// alias another command claimed first — the conflict policy is first
	// come first served, so the field keeps the user's text but the row says
	// it is not live.
	Alias string
	Taken bool
}

// UninstallComponents says which parts of an installed integration to remove.
// The program always goes; the three data categories are the user's choice.
type UninstallComponents struct {
	// RemoveHostConfig removes the host-owned configuration (config.json and
	// the secrets generations).
	RemoveHostConfig bool
	// RemoveToolData removes the tool's own data (sessions, completions,
	// health).
	RemoveToolData bool
	// RemoveArtifacts removes generated artifacts (logs, caches,
	// user-generated files).
	RemoveArtifacts bool
}

// Session is one running session, as the Sessions page shows it.
type Session struct {
	// ID is the session's stable identity, the key the close control arms on:
	// the sessions list can shrink while a question is up, and the armed
	// session has to stay the same one (the reference's `killArmedId`),
	// `src/settings/SessionsPage.tsx:54`). Zero never arms.
	ID      uint64
	Title   string
	Running bool
	// Active marks the session on screen: the one the terminal surface is
	// showing, and the one "close" would end without asking which.
	Active bool
	// Index is the session's own position, which a click hands back.
	Index int
	// Note is the session's state line: how the program went, when it did.
	Note string
	// Dir is the working directory the shell reported (empty when it has not
	// said), and Size the screen's size in cells, as "80×24".
	Dir  string
	Size string
	// Created is when the session started: the list's own order axis, newest
	// first by default as the old page listed them.
	Created time.Time
}

// About is what the About page shows: the identity of the build and where
// its files live.
type About struct {
	Name         string
	Version      string
	Framework    string
	Scheme       string
	SettingsPath string
	RepoURL      string
}

// UpdateProgress is the bytes a running update download has and the total it
// expects. Total is 0 when the feed did not report one, which is the state the
// old build's progress label drew as a bare downloaded count.
type UpdateProgress struct {
	Downloaded int64
	Total      int64
}

// UpdateState is the About page's update row, as the last check and the running
// download left it: the old build's `updateInfo` / `updateDownloading` /
// `updateProgress` / `updateFailed` quartet, plus the no-feed state this build
// can be in.
type UpdateState struct {
	// Checked is true once a check has answered (found, up to date, or
	// disabled); a failed check is also an answer and sets Failed.
	Checked bool
	// Available is true when the check found a newer version, named by
	// Version.
	Available bool
	Version   string
	// Downloading is true from the moment the download starts until it
	// finishes or fails; Progress is set while the bytes are moving.
	Downloading bool
	Failed      bool
	// Disabled is true when the build has no update feed at all.
	Disabled bool
	Progress *UpdateProgress
}

// App is the settings surface's state.
type App struct {
	Store   *settings.Store
	Actions Actions
	About   About

	// Now is the panel's time source, injectable so the save-failure banner's
	// window can be exercised without waiting for it; nil means the wall
	// clock.
	Now func() time.Time

	// cardRows counts the rows the card being built has drawn, so each row
	// after the first draws the rule above itself. It is the build's own
	// cursor, reset by card and never read outside it.
	cardRows int

	// cardRowInset, cardTitleFloor and cardLineGap are the row geometry the
	// card being built wears when it is not the settings default: zero means
	// the settings card's own 14u inset, 44u floor and 1px line gap. The
	// integration card is the one caller that sets them (cardGeometry),
	// because it draws the reference's `.extension-row` — an 8u padding, a 66u
	// floor and a 4px line gap (`extensions.css:15-16,733-744,879-880`) — so
	// its title row and every row under it line up on the extension row's own
	// text column. `card` resets them to the default, so a card nested inside
	// the integration card (the run form, the config fields) keeps the
	// settings geometry.
	cardRowInset   float32
	cardTitleFloor float32
	cardLineGap    float32

	// cardUnit is the metric unit the surface being drawn uses when its
	// reference CSS is *not* written in `--u`: zero keeps the theme's own
	// `Space` (4·scale), and a positive value is the literal unit the surface
	// pins instead (4 for the extension panel, whose `extensions.css` is
	// written in literal px — a 66px row, a 30px chip, an 8px padding — and so
	// does not follow the interface step). It is set for the duration of a
	// draw by extensionMetrics and read through cardSpace/cardTheme, so a
	// helper that sizes its own metrics follows the surface it is drawn on.
	cardUnit float32

	// cardNoRules suppresses the row separator for the duration of a draw.
	// The reference's `.extension-run-params` is a grid whose rows are spaced
	// 6px apart and never ruled (`extensions.css:1224-1233` — only `gap`, no
	// hairline), so the run form must not read as a settings card's ruled rows.
	// runParamsBox sets it while its rows are drawn and restores it after; zero
	// means the ordinary settings behaviour, where every row but the first
	// draws the rule.
	cardNoRules bool

	// disableConfirm marks the integrations whose switch was turned off and
	// is waiting for the user's answer. removeConfirm marks the ones whose
	// destructive button was pressed and is likewise waiting — the old
	// panel's removal bar (`RemovalConfirmation`), asked inline rather than
	// through a native dialog. runValues holds the run form's
	// answers per integration and runError its last refusal. runOpen is
	// which integrations have their run form unfolded: the row's Run
	// button is the disclosure (the reference's `runFormOpen`), so an
	// integration that declares inputs does not wear the form until it is
	// asked for. runOutputs holds each integration's last background run and
	// outputOpen which of those blocks are unfolded (the old panel's
	// `runOutputs` / `outputOpen`), with outputScroll the fixed-height
	// scroller's offset.
	disableConfirm map[string]bool
	removeConfirm  map[string]bool
	// disableConfirmAt and removeConfirmAt are when each question was asked:
	// the reference armed both bars with `useTimedReset(removalTarget, …)` /
	// `useTimedReset(disableTarget, …)` and its default 3000 ms window
	// (`src/hooks/useTimedReset.ts:3`, `src/ExtensionsPanel.tsx:864-865`), so an
	// ask left unanswered withdraws instead of staying armed on a later glance.
	disableConfirmAt map[string]time.Time
	removeConfirmAt  map[string]time.Time
	// disableBusy and removeBusy mark the bands whose answer is in flight, so
	// the confirm pill holds the reference's disabled face (`disabled={busy}`,
	// `opacity: 0.4`, no hover/press) until the shell reports back; the error
	// maps carry the failure that left a band open, exactly as the uninstall
	// dialog's busy/error pair does (`ComponentizedUninstallDialog.tsx:126`).
	disableBusy  map[string]bool
	disableError map[string]string
	removeBusy   map[string]bool
	removeError  map[string]string
	// repairBusy marks the integrations whose re-check or reconnect is in
	// flight, so the row's own button wears the reference's turning LoaderCircle
	// and its `disabled={busy}` face until the shell reports back
	// (`src/extensions/ExtensionRow.tsx:313-330,345-372`).
	repairBusy map[string]bool
	// toggleBusy marks the integrations whose enable write is in flight, so
	// the row's switch swaps its thumb for the reference's turning LoaderCircle
	// and holds its `disabled` face until the shell reports back
	// (`src/extensions/ExtensionRow.tsx:443-455`). A disable write rides
	// disableBusy: its answer comes from the confirmation band, not the switch.
	toggleBusy   map[string]bool
	runValues    map[string]map[string]string
	runError     string
	runOpen      map[string]bool
	runOutputs   map[string]RunResult
	outputOpen   map[string]bool
	outputScroll map[string]*ui.ScrollState

	// The local-tool form's draft: what the user has typed, and the
	// permissions they have ticked. It lives on the panel so a keystroke
	// survives the rebuild, and it is cleared once the tool is connected.
	toolOpen bool
	// toolAdvanced is whether the form's declaration/audit switches are
	// unfolded: they sit behind a default-closed Advanced disclosure (R161),
	// so a brand-new integration does not open on a list of permission rows.
	toolAdvanced bool
	// toolMode is the kind of tool the form is building: "" (a program on
	// this machine) or "script".
	toolMode string
	// toolLanguage and toolScript are a script's own draft.
	toolLanguage    string
	toolScript      string
	toolName        string
	toolProgram     string
	toolCommand     string
	toolArgs        string
	toolVersionArgs string
	toolVersion     string
	toolDescription string
	toolOutput      string
	// toolPlatforms is which operating systems the connection is limited to,
	// keyed by the platform ids the manifest carries (see customPlatforms).
	// Nil means the form has not touched the row yet, so it starts on the
	// current platform alone (R7-5).
	toolPlatforms   map[string]bool
	toolPermissions map[string]bool
	// toolParams is the declared inputs the form is editing, one per row.
	toolParams []toolParam
	// scriptRuntime is the script form's own probe state, one entry per
	// language: the old panel's `runtimeChecks` mirror. A probe runs off the
	// render thread, so the muted line can say "checking" instead of blocking
	// the frame on a `--version` subprocess; `scriptRuntimeMu` guards the map
	// because the probe lands on its own goroutine. `scriptRuntimeEpoch` is the
	// old drawer's `runtimeChecksEpoch`: a form that closed and reopened in
	// between must not inherit the previous session's verdict, so a stale
	// result is dropped rather than cached.
	scriptRuntime      map[string]*runtimeProbe
	scriptRuntimeMu    sync.Mutex
	scriptRuntimeEpoch int
	// scriptEditor is the form's script box, kept across frames so its buffer,
	// undo history and caret survive the rebuild; nil until a script form is
	// first drawn. See scripteditor.go.
	scriptEditor *scriptEditor
	// toolEditing is the integration the form is rewriting, empty when it is
	// connecting something new.
	toolEditing string
	// toolIssue is whether the form has been submitted and refused: the rows
	// name what they still need only after the user asked, so a form that just
	// opened is not already covered in red.
	toolIssue bool

	// armedSession is the session whose close control has been pressed once,
	// named by its stable ID, and armedSessionAt when it was asked: the second
	// press ends the session's program, and the question answers itself with
	// "keep" after sessionCloseWindow. A zero ID is "nothing armed" and never
	// matches a real session (the reference's `killArmedId` was a
	// `string | null`, `src/settings/SessionsPage.tsx:54`); the two fields are
	// only ever set together, so the clock drives the expiry and the ID drives
	// the match.
	armedSession   uint64
	armedSessionAt time.Time
	// sessionsSortOldest is the sessions list's order: false (the shipped
	// state) lists the newest session first, as the old page's own default,
	// and the heading's control flips it.
	sessionsSortOldest bool

	// residencyInit, residencyCustomOpen and residencyDraft are the
	// page-residency control's own state: whether the inline custom field has
	// been seeded yet, whether it is open, and what it drafts. residencyInit
	// seeds the other two once from the stored value on the first frame, so a
	// stored custom duration opens the field it came from — the reference's
	// `useState` initializers in `SurfaceResidencyControl`.
	residencyInit       bool
	residencyCustomOpen bool
	residencyDraft      string

	// saveFailed is set when one of the page's own controls could not write
	// its change back, with saveFailedAt the moment it failed: the banner over
	// the page says so and clears itself after a few seconds (see
	// savealert.go).
	saveFailed   bool
	saveFailedAt time.Time
	// saveAlertShown is whether the current banner has drawn its first frame,
	// and saveAlertProgress/saveAlertAnimating where its enter animation is:
	// see savealert.go.
	saveAlertShown     bool
	saveAlertProgress  float32
	saveAlertAnimating bool

	// entered marks that the panel's first frame has been drawn. The shell
	// asks for the sidebar's focus when it opens the surface, but on the very
	// first open the list has no state to take the request yet; View delivers
	// it once the list is built, and entered keeps that to once.
	entered bool
	// sidebarEntryFocus is the reference's `data-entry-focus` (R152): as the
	// panel opens the sidebar takes the keyboard, but the chosen pill already
	// marks the keyboard position, so the list's `:focus-visible` ring is
	// withheld until the first interaction. entryFocusArmed records that the
	// focus really landed, so a later loss of it can drop the mark.
	sidebarEntryFocus bool
	entryFocusArmed   bool

	// debouncing marks a slider preview or alias edit that has been shown but
	// not written yet, with debounceAt the moment the window closes;
	// pendingAliases holds the latest alias per command while it runs. See
	// debounce.go for the reference's 180ms rule.
	debouncing     bool
	debounceAt     time.Time
	pendingAliases map[string]aliasEdit

	// Sessions reports the running sessions; nil when the shell has none
	// (tests).
	Sessions func() []Session
	// Recommended reports the shipped tool packages and whether each is
	// installed; nil when the shell has none (tests).
	Recommended func() []RecommendedTool
	// Integrations reports the installed extensions; nil when the shell has
	// none (tests).
	Integrations func() []Integration
	// Shortcut is the global summon key the shell registered, and
	// ShortcutID the action it belongs to.
	Shortcut   string
	ShortcutID string
	// recording is set while the recorder waits for a key combination.
	recording bool

	// Page is the chosen page, an index into the sidebar.
	Page int

	// Update is the About page's update state: what the last check found,
	// whether a download is running, and the bytes it has so far.
	Update UpdateState
	// copiedLink is the deep-link value the About page's copy button last put
	// on the clipboard, and copiedLinkUntil when its confirmation expires. The
	// button reads "Link copied" while it holds (the old build's 1.6s), so a
	// press that landed reads differently from one that did not.
	copiedLink      string
	copiedLinkUntil time.Time
	// copiedContent is which of the local-tool form's content actions last put
	// something on the clipboard ("script", "command", "plan"), and
	// copiedContentUntil when that button's confirmation expires. Keying by the
	// action rather than by the copied text keeps two buttons that happen to
	// copy the same bytes from lighting up together (the reference's
	// `customContentOperation` plus one `showSuccess` per press).
	copiedContent      string
	copiedContentUntil time.Time
	// uninstallOpen is which integrations have the componentized uninstall
	// dialog open, and uninstallDraft its checkboxes.
	uninstallOpen  map[string]bool
	uninstallDraft map[string]UninstallComponents
	// uninstallBusy is which integrations have a componentized removal in
	// flight, so the dialog's two pills hold the reference's disabled face
	// (`ComponentizedUninstallDialog.tsx:126,142`) until the answer lands, and
	// uninstallError the last failure on a dialog that stayed open.
	uninstallBusy  map[string]bool
	uninstallError map[string]string
	// auditOpen is which integrations have their permission audit unfolded,
	// configOpen which configuration forms are open, configDrafts their form
	// state, and configError the last save's error.
	auditOpen    map[string]bool
	configOpen   map[string]bool
	configDrafts map[string]*configDraft
	configError  map[string]string
	// configOperations is which integrations have a copy or export in flight
	// ("copy" or "export"), configSaveBusy which have a save in flight, and
	// configDiscard which have asked to fold a dirty form away. The
	// reference's one `configOperation` item disabled both the toolbar's
	// icon-buttons (`disabled={Boolean(configOperation)}`,
	// `ExtensionsPanel.tsx:2994,2997`) — a copy and an export cannot run at
	// once — while `busy.kind === "save"` swapped the Save's own word
	// (`ExtensionsPanel.tsx:3020`: `t("settings.extensions.saving")`).
	configOperations map[string]string
	configSaveBusy   map[string]bool
	configDiscard    map[string]bool
	// configDiscardAt is when each fold-away question was asked: the reference
	// armed it with `useTimedReset(detailsDiscardArmed, …)` and its 3000 ms
	// default (`ExtensionsPanel.tsx:866,2056-2060`), so an ask left unanswered
	// withdraws.
	configDiscardAt map[string]time.Time
	// configFieldErrors is the sentence shown at one field of one integration,
	// keyed by the integration and the field (see configFieldErrorKey): the
	// reference left the schema's `min`/`max`/`minLength`/`maxLength` bounds to
	// the native control, which drew its own validity state *at the field*
	// (`ExtensionsPanel.tsx:3251-3256`), so the hint belongs there rather than
	// only on the form's own bottom line.
	configFieldErrors map[string]string
	// configListScrolls keeps each multiselect list box's own offset, keyed by
	// the integration and the field, so a long option list scrolls inside the
	// reference's 66px viewport where the user left it.
	configListScrolls map[string]*ui.ScrollState
	// shortcutsSuspended is the last state reported to the shell, so the
	// recorder releases the global keys once and takes them back once.
	shortcutsSuspended bool
	// TransferBusy is "export" or "import" while a transfer is in flight, and
	// empty otherwise: the reference's `syncOperation`, which disables the two
	// buttons and shows the busy line while one runs.
	TransferBusy string
	// TransferReport is what the last import did; nil until one runs, and
	// cleared when the next transfer starts (the reference's `importReport`).
	TransferReport *ImportReport

	// Sidebar holds the page list's identity, its choice and its focus.
	Sidebar ui.ListState
	// Body keeps the form's scroll offset across frames.
	Body ui.ScrollState
	// PluginScroll keeps the plugin configuration sheet's offset, so a long
	// schema scrolls where the user left it.
	PluginScroll ui.ScrollState
	// pluginNumberDrafts holds a number field's in-progress text while its box
	// has the keyboard, so a half-typed or emptied value is not overwritten by
	// the stored one mid-edit; an entry is dropped when the box loses focus and
	// the field re-reads its value (the framework's `ui.NumberInput` did the
	// same with its own local draft).
	pluginNumberDrafts map[string]string

	// installName and installVersion are the npm install field's contents.
	installName    string
	installVersion string

	// The custom-shortcut row being made: whether it is on screen at all (the
	// reference's Add put it there), the recorded key, the chosen action and,
	// for a command line, its text. customRecordRow names the row the recorder
	// is editing: an index into the stored list, or -1 for the row being made.
	customDraftOpen bool
	customRecording bool
	customRecordRow int
	customKey       string
	customAction    string
	customCommand   string
	// customDuplicate is a key this section refused on the spot (another custom
	// row already holds it), and customRejections are the keys the shell could
	// not register with the system. Both hang under the row that holds the key.
	customDuplicate    string
	customDuplicateRow int
	customRejections   []CustomShortcutRejection
	// customEdits holds a saved row's command line while it is being typed in
	// place, keyed by the binding. The reference kept the same draft on the row
	// (`row.command`) and committed it on Enter/blur (`ShortcutsPage.tsx:204-216`);
	// the port's fields commit on Enter, so the text lives here between frames
	// and is dropped when the row's action is rewritten or the row is removed.
	customEdits map[string]string

	// rejectedShortcut is the action whose last recording the page refused —
	// the reference's `rejectedAction`. It carries the warning at the top of
	// the page and under that row's name until the next recording starts.
	rejectedShortcut string
}

// CustomShortcutRejection is a key the system would not bind.
type CustomShortcutRejection struct {
	Key    string
	Reason string
}

// The reasons a custom global shortcut is refused, as the shell reports them
// and the settings page words them. The old host had exactly these two tokens
// (`src-tauri/src/commands/config.rs:1691,1707`): a key that shadows one of the
// app's own shortcuts is a `conflict`, and one the system will not take (almost
// always another application already owns it) is `occupied`.
const (
	// CustomShortcutReasonConflict: the key is already one of the app's own.
	CustomShortcutReasonConflict = "conflict"
	// CustomShortcutReasonOccupied: the system refused the key.
	CustomShortcutReasonOccupied = "occupied"
)

// New builds the settings surface over a store.
func New(store *settings.Store, actions Actions) *App {
	a := &App{Store: store, Actions: actions, customRecordRow: -1}
	// The sidebar is a click-only list: it is *not* given a cursor, so the
	// framework's own list navigation never claims ↑/↓. The reference's page
	// keys are the window's, not the sidebar's — they walk the pages from
	// anywhere on the surface, wrap at both ends, and remember the choice
	// (see `stepPage`). Giving the list a cursor here would put its
	// non-wrapping, non-persisting navigation in front of that handler
	// whenever the sidebar held the keyboard.
	a.Sidebar.Label = func(row int) string { return a.page(row).Title }
	return a
}

// FocusSidebar asks for the keyboard focus on the page list, and arms the
// entry-focus mark: the shell calls it when the settings surface opens, so the
// sidebar is where the keyboard starts (the reference's
// `focusCurrentSettingsSidebar`).
func (a *App) FocusSidebar() {
	a.Sidebar.Handle.Focus()
	a.sidebarEntryFocus = true
	a.entryFocusArmed = false
}

// SelectedPage is the chosen page.
func (a *App) SelectedPage() int { return a.Page }

// SidebarEntryFocus reports whether the sidebar's entry-focus mark is up — the
// reference's `data-entry-focus` (R152) — for the tests.
func (a *App) SidebarEntryFocus() bool { return a.sidebarEntryFocus }

// page describes one sidebar entry in the current language.
type page struct {
	Title string
	Hint  string
}

// page returns the i-th page's copy.
func (a *App) page(i int) page {
	c := i18n.For(a.Store.Snapshot().Language).Settings
	switch i {
	case PageSessions:
		return page{c.PageSessions, c.PageSessionsHint}
	case PageShortcuts:
		return page{c.PageShortcuts, c.PageShortcutsHint}
	case PageIntegrations:
		return page{c.PageIntegrations, c.PageIntegrationsHint}
	case PageAbout:
		return page{c.PageAbout, c.PageAboutHint}
	default:
		return page{c.PageGeneral, c.PageGeneralHint}
	}
}

// syncShortcutSuspension tells the shell whether a recorder is waiting for a
// key. While one is, the global shortcuts are released: pressing the key that
// currently summons floter must be recorded, not acted on.
func (a *App) syncShortcutSuspension() {
	if a.Actions.SetShortcutsSuspended == nil {
		return
	}
	suspended := a.recording || a.customRecording
	if suspended == a.shortcutsSuspended {
		return
	}
	a.shortcutsSuspended = suspended
	a.Actions.SetShortcutsSuspended(suspended)
}

// View builds the settings surface: the page list beside the body.
func (a *App) View(c *ui.Context) {
	defer a.syncShortcutSuspension()
	copy := i18n.For(a.Store.Snapshot().Language).Settings
	t := c.Theme()

	// A write the page's own controls could not commit is announced for a few
	// seconds; while one is up the frame keeps coming so the deadline is
	// reached on a still page.
	a.expireSaveFailed(c)
	// A slider drag or alias edit is shown at once but written once the pause
	// passes; the frame keeps coming until the window closes.
	a.expireSettingsWrites(c)
	// A session's two-step close also carries its own clock: the reference's
	// `killArmTimer` reverted the button after three seconds, so an ask left
	// unanswered does not stay armed on a later glance.
	a.expireSessionClose(c)
	// The two inline confirmations carry the same clock: the reference armed
	// their targets with `useTimedReset` and its own three-second default
	// (`src/ExtensionsPanel.tsx:864-865`).
	a.expireConfirmations(c)

	// The panel's very first frame puts the keyboard on the sidebar. The
	// shell's own `FocusSidebar` runs when the surface opens, before the list
	// has ever been built, so on the first open the request has nowhere to
	// land; delivering it here, once the list takes state, is the reference's
	// `focusCurrentSettingsSidebar` (`src/App.tsx`).
	if !a.entered {
		a.entered = true
		a.FocusSidebar()
	}

	// While a recorder waits, Escape belongs to it: a shortcut of the view is
	// handled before the focused element's input, so both recorders have to
	// yield or their cancel key would close the panel instead. And while any
	// confirmation or armed session close is up, Escape answers that question —
	// it must not fall through to Close and leave the page as well (the
	// reference's own `stopImmediatePropagation`: the removal/disable bars and
	// the sessions page's kill arm each installed a capture-phase Escape
	// listener that cancelled the question and swallowed the key).
	if !a.recording && !a.customRecording && c.Shortcut(0, ui.KeyEscape) {
		switch {
		case a.anyConfirmOpen():
			a.cancelConfirmations()
		case a.anySessionArmed():
			a.disarmSessionClose()
		case a.Actions.Close != nil:
			a.Actions.Close()
		}
	}

	// ↑/↓ walk the sidebar from anywhere on the surface — a header button, a
	// control, or the sidebar itself — wrapping at both ends, as the
	// reference's window-level handler did (`useAppKeyboard.ts`: "↑/↓ cycle
	// sidebar pages from anywhere in the settings panel"). An editor keeps its
	// own arrows (its `wants` runs before the window's shortcuts) and so do
	// the page's select/range controls (their element-scoped shortcuts sit in
	// the focus chain), which is the reference's `isArrowKeyEditableTarget`
	// rule. While a recorder waits for a key, the arrows do not navigate.
	if !a.recording && !a.customRecording {
		switch {
		case c.Shortcut(0, ui.KeyDown):
			a.stepPage(1)
		case c.Shortcut(0, ui.KeyUp):
			a.stepPage(-1)
		}
	}

	// The panel is the reference's own: a 56u header band carrying the app's
	// name and its version, the hairline under it, and below them the two
	// columns — a 148px sidebar and the content. The header band is the same
	// one the launcher's field row and every page's header use (R37): it names
	// `launcher.FieldRowUnits`, so the two surfaces' first band is one height
	// and switching does not move the window's top edge. The 10u gap and the
	// 16u side padding are the reference's `.settings-card__header`.
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Height(t.Space(launcher.FieldRowUnits)).Padding(0, t.Space(4)).
			Gap(t.Space(2.5)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, copy.Title).FontSize(t.FontSize + 2).FontWeight(600).
				TextColor(a.tokens(c).TextStrong)
			if version := strings.TrimSpace(a.About.Version); version != "" {
				ui.Text(c, "v"+version).FontSize(t.FontSize).TextColor(t.TextMuted)
			}
			// The header's own two controls, at its end: the way out of the
			// application and the way back to the launcher, as the old panel's
			// `.settings-card__actions` carried them.
			ui.Box(c).Grow(1)
			// The header's own two controls, at its end: the way out of the
			// application and the way back to the launcher. They are the
			// reference's `.toolbar-button` — 28u transparent squares with a
			// 16u mark — never filled pills in the header band.
			headerButton := func(glyphName, label, hint string, run func()) {
				if run == nil {
					return
				}
				tokens := a.tokens(c)
				button := ui.Row(c).Focusable().Shrink(0).Size(t.Space(7), t.Space(7)).
					Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).Label(label).
					Tooltip(hint)
				if button.Hovered() {
					button.Background(tokens.ControlHover)
					// The header's icon button lights the resting lens rung under
					// the pointer (`.toolbar-button:hover`); it is transparent at
					// rest, like the reference's own.
					tokens.Lens(button)
				}
				button.Children(func() {
					if glyph, ok := launcher.Glyph(glyphName); ok {
						ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(t.TextMuted)
					}
				})
				if button.Clicked() {
					run()
				}
			}
			headerButton("power", copy.Quit, copy.QuitHint, a.Actions.Quit)
			headerButton("close", copy.Close, copy.CloseHint, a.Actions.Close)
		})
		// The header/body separator is the contract's one weak hairline
		// (`docs/raycast-design.md` §6.1): a 1px `HairlineSoft` rule, full width,
		// with no fade — the same line the card's rows draw between themselves.
		a.hairlineRule(c, false)
		// The content pane carries its own fill (`.settings-card__body`'s
		// `--surface-sunken`): the panel is a reading surface, more solid than
		// the launcher, and the sidebar sits on the same one so the two read
		// as one pane.
		//
		// The row *grows* into the space the header leaves instead of asking
		// for 100% of the card: a `Fill` height resolves against the card, so
		// the row kept the header's 57u below the window — the body scroller
		// then believed it had that much more viewport than the user can see,
		// and the last rows of every page sat below the card's bottom edge and
		// could not be scrolled to (「设置页面的列表，无法滚动到底部」).
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Gap(t.Space(3)).Children(func() {
			ui.Column(c).Width(148).Padding(t.Space(3), t.Space(2)).Gap(t.Space(0.75)).
				Background(a.tokens(c).Sunken).Children(func() {
				list := ui.List(c.Key("settings.sidebar"), &a.Sidebar, pageCount, func(i int) {
					a.sidebarRow(c, i)
				}).Grow(1).Label(copy.Title)
				// The entry-focus mark withholds the list's own ring: the chosen
				// pill already shows where the keyboard is, and the reference's
				// `data-entry-focus` kept `:focus-visible` off the sidebar until
				// the first interaction (R152). Once the keyboard leaves the
				// sidebar — a Tab, a click on a control — the mark goes and the
				// ring is the framework's again.
				list.FocusRing(!a.sidebarEntryFocus)
				switch {
				case a.sidebarEntryFocus && a.Sidebar.Focused(c):
					a.entryFocusArmed = true
				case a.entryFocusArmed:
					a.sidebarEntryFocus, a.entryFocusArmed = false, false
				}
			})
			// The sidebar's column separator is the same weak hairline, drawn
			// vertically (same rule, §6.1).
			a.hairlineRule(c, true)
			ui.Column(c).Grow(1).Background(a.tokens(c).Sunken).Children(func() {
				a.body(c, copy)
			})
		})
	})
}

// permissionTier draws one tier of the permission audit: its label and the
// one-line explanation of what the tier means, then a row per permission. An
// enforced permission's row is a plane with the accent edge — a *check* the
// host owns — while a declared one is the plain neutral pane: an FYI, not a
// promise.
func (a *App) permissionTier(c *ui.Context, label, hint string, enforced bool, language string, permissions []string) {
	if len(permissions) == 0 {
		return
	}
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		ui.Text(c, label).FontSize(t.FontSize - 1).FontWeight(650).TextColor(tokens.TextStrong)
		if hint != "" {
			ui.Text(c, hint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
		a.card(c, func() {
			for _, permission := range permissions {
				permission := permission
				a.row(c, i18n.PermissionLabel(language, permission), i18n.PermissionDescription(language, permission), func() {
					mark := "\u25cb"
					if enforced {
						mark = "\u25cf"
					}
					color := t.TextMuted
					if enforced {
						color = t.Accent
					}
					ui.Icon(c, mustGlyph("alert")).Size(t.Space(4), t.Space(4)).TextColor(color)
					ui.Text(c, mark).FontSize(t.FontSize - 3).TextColor(color)
				})
			}
		})
	})
}

// mustGlyph is the shared glyph vocabulary with a hard failure: the names here
// are program constants, so a missing one is a bug rather than a runtime
// condition.
func mustGlyph(name string) *ui.SVG {
	glyph, ok := launcher.Glyph(name)
	if !ok {
		glyph, _ = launcher.Glyph("alert")
	}
	return glyph
}

// sidebarRow builds one page entry: a leading mark, the page's name at weight
// 580, and the chosen page on the row ladder's neutral chosen fill — the
// sidebar's own selection, since the list is not asked to paint one.
func (a *App) sidebarRow(c *ui.Context, i int) {
	t := c.Theme()
	tokens := a.tokens(c)
	selected := i == a.Page
	row := ui.Row(c).FillWidth().MinHeight(t.Space(8.5)).
		Padding(0, t.Space(2)).Gap(t.Space(2)).
		Radius(tokens.RadiusSM).AlignItems(ui.Center).
		Transition(ui.ElementTransition{Colors: true, Duration: 120 * time.Millisecond})
	if selected {
		// P8-b 重定基线：旧 `--glass-raised-quiet` + `--elev-0` 透镜 → 合约
		// §4.3/§6.1 的**中性**选中档 `RowSelected`（0.085/0.06 的白/黑），无镜片。
		//
		// Raycast 的侧栏选中和它的结果列表是同一档：一块不透明的抬起面会
		// 读成第二个材质，而选中档只是「比悬停再深一格」的中性填充，镜片
		// 属于旧 glass 语言（§1 的低光泽上限）。accent 仍只留在前导图标上
		// （§5 第 1 条）。
		row.Background(tokens.RowSelected)
	} else if row.Hovered() {
		// The pointer lights the ladder's hover rung, the same neutral family
		// one step under the chosen page.
		row.Background(tokens.RowHover)
	}
	// The reference puts the row's ink on one of three rungs
	// (`settings.css:137,152-156,166-170`): `--text-muted` at rest,
	// `--text-primary` under the pointer, and `--text-primary` on the chosen
	// page. The old port kept the resting muted ink for hover too and used
	// `--text-strong` for the chosen page, so a pointed-at row never lit and
	// the current one was a hair dimmer than the reference's own.
	color := t.TextMuted
	if selected || row.Hovered() {
		color = t.Text
	}
	row.Children(func() {
		if glyph, ok := pageGlyph(i); ok {
			// The reference draws this mark at a *literal* 15, not in units
			// (`App.tsx:2925`'s `<Icon size={15} strokeWidth={2}/>`), so it
			// does not follow the interface-size step: the sidebar's five
			// marks sit at 15px under every step, while every other icon in
			// this app is `--u`-scaled. A `Space(4)` here tracked the step and
			// left the mark at 14.4 even at the shipped default.
			icon := ui.Icon(c, glyph).Size(sidebarGlyphSize, sidebarGlyphSize).TextColor(color)
			if selected {
				icon.TextColor(t.Accent)
			}
		}
		ui.Text(c, a.page(i).Title).FontSize(tokens.Emphasis).FontWeight(580).
			TextColor(color).Ellipsis("\u2026").SingleLine()
	})
	if row.Clicked() {
		// A click is the first interaction, so the entry-focus mark goes.
		a.sidebarEntryFocus, a.entryFocusArmed = false, false
		a.selectPage(i)
	}
}

// sidebarGlyphSize is the sidebar's leading mark size in DIPs, and it is a
// constant on purpose: the reference writes `<Icon size={15}/>` in the JSX
// (`main:src/App.tsx:2925`) so the five marks are 15px under every
// interface-size step, unlike every other icon in the app, which is sized in
// `--u`. It is the one place the port must *not* reach for the step.
const sidebarGlyphSize float32 = 15

// pageGlyph is the mark a settings page leads with: the exact lucide set the
// reference's sidebar named (`main:src/App.tsx:2903-2908`),
// `[general SlidersHorizontal, sessions SquareTerminal, shortcuts Keyboard,
// integrations Blocks, about Info]`, drawn through the launcher's shared
// vocabulary so a page and the thing it configures look alike. It is not the
// page's *content* glyph (a star for shortcuts, a file for integrations): the
// reference picks a shape per page, not the first mark to hand.
func pageGlyph(page int) (*ui.SVG, bool) {
	switch page {
	case PageGeneral:
		return launcher.Glyph("sliders-horizontal")
	case PageSessions:
		return launcher.Glyph("square-terminal")
	case PageShortcuts:
		return launcher.Glyph("keyboard")
	case PageIntegrations:
		return launcher.Glyph("blocks")
	case PageAbout:
		return launcher.Glyph("info")
	}
	return nil, false
}

// selectPage switches pages and remembers the choice, so the next visit to
// the settings surface reopens where the user left off.
func (a *App) selectPage(page int) {
	// The page's own debounced controls are written before it leaves: a drag
	// that is still inside its window must not be lost with the page.
	a.flushSettingsWrites()
	// The reference's kill arm lived in the SessionsPage component: leaving
	// the page unmounted it, so the question never survived a visit
	// elsewhere. Drop it where the page changes hands.
	if page != PageSessions {
		a.disarmSessionClose()
	}
	a.Page = page
	if a.Actions.SetPage != nil {
		a.Actions.SetPage(PageName(page))
	}
}

// stepPage walks the sidebar by `delta`, wrapping around its order, and puts
// the keyboard back on the sidebar — the reference's `nextSettingsPage` plus
// its `settingsSidebarButtons.current.get(next)?.focus()`.
func (a *App) stepPage(delta int) {
	a.selectPage(nextPage(a.Page, delta))
	// An arrow is the first interaction, so the entry-focus mark goes too: the
	// ring is the keyboard's own from here.
	a.sidebarEntryFocus, a.entryFocusArmed = false, false
	a.Sidebar.Handle.Focus()
}

// nextPage is the page `delta` steps away in the sidebar order, wrapping at
// both ends (the reference's `nextSettingsPage` over `wrapIndex`).
func nextPage(page, delta int) int {
	n := pageCount
	return ((page+delta)%n + n) % n
}

// body builds the chosen page: the page's own title and hint, then its
// sections, scrolling together under a soft scroll edge — the reference's
// `.settings-page__header` is part of the scrolling content, not a second
// pinned band (the card header above it is the only one).
func (a *App) body(c *ui.Context, copy i18n.Settings) {
	p := a.page(a.Page)
	t := c.Theme()

	// The scroller reserves the scroll edge's own height, so the resting state
	// sits below the band and only content that actually scrolls passes
	// through it.
	edge := t.Space(4)
	ui.Column(c).Fill().Children(func() {
		ui.Scroll(c.Key("settings.body")).TrackScroll(&a.Body).Fill().
			Padding(edge, 0, t.Space(6), 0).Children(func() {
			// The horizontal inset lives on the content column, not on the
			// scroller, so the scrollbar still hugs the panel's edge
			// (`.settings-content > *`).
			ui.Column(c).FillWidth().Padding(0, t.Space(4.5)).Gap(t.Space(5.5)).Children(func() {
				// The write-failure banner sits above the page's own header, so a
				// refused change is the first thing the user reads.
				a.saveAlert(c, copy)
				// The page's own registers: the title at the display step (650)
				// and the one-line hint at the body step (500), as
				// `.settings-page__title` / `__subtitle` had them.
				ui.Column(c).FillWidth().Gap(t.Space(0.75)).Children(func() {
					ui.Text(c, p.Title).FontSize(a.tokens(c).Display).FontWeight(650).TextColor(t.Text)
					if p.Hint != "" {
						ui.Text(c, p.Hint).FontSize(t.FontSize).FontWeight(500).TextColor(t.TextMuted)
					}
				})
				switch a.Page {
				case PageGeneral:
					a.general(c, copy)
				case PageSessions:
					a.sessions(c, copy)
				case PageIntegrations:
					a.integrations(c, copy)
				case PageShortcuts:
					a.shortcuts(c, copy)
				case PageAbout:
					a.about(c, copy)
				default:
					ui.Column(c).FillWidth().Padding(t.Space(4)).Center().Children(func() {
						ui.Text(c, copy.PagePlaceholder).FontSize(t.FontSize).TextColor(t.TextMuted)
					})
				}
			})
		})
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(edge).PassThrough().
			Material(glass.ScrollEdge{Background: t.Background})
	})
}

// sessions lists the running terminal sessions.
//
// The list is the sessions page's own, not the terminal surface's: it is
// ordered along its own axis — newest first, the old page's default — and the
// heading carries that order as a control, so a hand can flip it. A session's
// program runs in this process, so there is no refresh to offer: what the list
// says is what the surface holds, at the frame it was drawn.
func (a *App) sessions(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	sessions := a.sortedSessions()
	if len(sessions) == 0 {
		// The reference's shared empty region (`SessionsPage.tsx:142-167,
		// 218-244`): the mark, the state in one line and the grey why under
		// it, with the page-body floor (`.session-manager > .settings-empty`
		// is 160u, where the list placeholder is 92u). This build's session
		// list is in-process and synchronous, so only the resting arm of the
		// reference's three-way is reachable (see `sessionsEmptyFaceFor`); the
		// page's own New session control stays the way out.
		face := sessionsEmptyFaceFor(copy, false, false)
		a.emptyState(c, face.Icon, face.Title, face.Hint, 0, t.Space(40), func() {
			if a.Actions.NewSession != nil {
				if ui.Button(c, copy.SessionsNew).Clicked() {
					a.Actions.NewSession()
				}
			}
		})
		return
	}
	// The order control shows the order in force and swaps it on press, as
	// the old page's `session-manager__sort-toggle` did — and it leads with
	// that control's own mark, lucide's ArrowUpDown at 13px
	// (`SessionsPage.tsx:123-125`), so the two directions the list can run in
	// are on the control itself.
	order := copy.SessionsSortNewest
	if a.sessionsSortOldest {
		order = copy.SessionsSortOldest
	}
	a.pageColumn(c, func() {
		// A question whose session is gone has nothing left to answer: the
		// reference's page re-rendered its list against the broker each poll,
		// so a killed/ended session's row took its arm with it. Here the list
		// is this frame's, so an arm that names no row is dropped before it is
		// handed to another one (see sessionInList).
		if a.armedSession != 0 && !sessionInList(sessions, a.armedSession) {
			a.disarmSessionClose()
		}
		a.sectionWithControls(c, copy.PageSessions, copy.SessionsActive, func() {
			a.actionWithGlyph(c, "arrow-up-down", order, func() {
				a.sessionsSortOldest = !a.sessionsSortOldest
			})
		}, func() {
			a.card(c, func() {
				for _, session := range sessions {
					session := session
					// The old page's own row was the resume control: its
					// `onActivate` was wired only for a *resumable* session
					// — `resumable = !session.exited && !session.attached`
					// (`SessionsPage.tsx:170,197`) — so pressing anywhere on
					// a session that is running behind the one on screen
					// brings it back, while the Play button beside it is
					// the same action's explicit form. An exited session and
					// the one already on screen answer nothing.
					resumable := session.Running && !session.Active
					// The row says what the session is doing — attached (on
					// screen), running behind the one that is, or how the
					// program went — and, under it, where the shell is and
					// when the session started. Its leading mark is the
					// reference's own `.settings-row__icon` chip with lucide's
					// SquareTerminal (`SessionsPage.tsx:186-187`), which is the
					// shared settings-row slot and not the extension panel's
					// package chip.
					activated := a.rowAligned(c, ui.Center, func() { a.sessionChip(c) }, rowFace{},
						session.Title, sessionLines(session), "", func() {
							// The trailing controls are the reference's two 28u icon
							// buttons (`SessionsPage.tsx:203-232`): Play (disabled once
							// the session has exited, while it is on screen, or while
							// another row's action runs) and Trash2 (disabled for the
							// same busy window). They sit in a 3u-gap row
							// (`.session-manager__actions { gap: 3u }`, `:1333-1336`).
							ui.Row(c).Gap(t.Space(0.75)).AlignItems(ui.Center).Children(func() {
								// The reference disables a row's controls while *another*
								// row's action is in flight (`actionId !== null`,
								// `SessionsPage.tsx:204,226`), and this build's arm is
								// the one such state a session has.
								// The reference's own reservations for the two controls
								// (`SessionsPage.tsx:204-206,226`): the Play button is
								// disabled on an *exited* session (`disabled={session.exited
								// || busy || actionId !== null}`), and the bin is disabled
								// only while this build has an action in flight — which it
								// never does, since a resume and a close both call straight
								// through to the shell. Its accent resume dot marks the
								// running-but-detached row it is *for* (`:203`).
								if a.Actions.SelectSession != nil {
									a.sessionIconButton(c, "play", copy.SessionsResume,
										false, resumable, !session.Running,
										func() { a.Actions.SelectSession(session.Index) })
								}
								if a.Actions.CloseSessionAt == nil {
									return
								}
								// Closing a session kills its program, so the control
								// arms first — the reference's two-step in place, where
								// the icon button *expands into* the warm question pill
								// rather than drawing a second row of buttons
								// (`SessionsPage.tsx:216-229`).
								if a.armedSession == session.ID {
									a.sessionKillConfirm(c, copy.SessionsKillArm, false, func() {
										a.disarmSessionClose()
										a.Actions.CloseSessionAt(session.Index)
									})
									return
								}
								a.sessionIconButton(c, "trash", copy.SessionsKill,
									true, false, false, func() { a.armSessionClose(session.ID) })
							})
						})
					if activated && resumable && a.Actions.SelectSession != nil {
						a.Actions.SelectSession(session.Index)
					}
				}
			})
		})
		if a.Actions.NewSession != nil {
			if ui.Button(c, copy.SessionsNew).Clicked() {
				a.Actions.NewSession()
			}
		}
	})
}

// sortedSessions is the session list in the order the page's control asks for:
// newest first by default, oldest first once the user has flipped it. The sort
// is stable, so sessions with no recorded start (a test's fixtures) keep the
// order the shell handed over.
func (a *App) sortedSessions() []Session {
	sessions := a.runningSessions()
	if len(sessions) < 2 {
		return sessions
	}
	out := append([]Session(nil), sessions...)
	sort.SliceStable(out, func(i, j int) bool {
		if a.sessionsSortOldest {
			return out[i].Created.Before(out[j].Created)
		}
		return out[i].Created.After(out[j].Created)
	})
	return out
}

// sessionLines is the row's grey block: the shell's working directory (when
// it reported one) above the state line, the screen size and the start time —
// the old page's `.session-manager__cwd` and its `.session-manager__meta`
// line. The state word is a segment of its own so it can carry its colour:
// the accent while the session runs, the quiet grey once it has exited, as
// `.session-state` and `.session-state--exited` did. The directory is marked
// `Mono`, because the reference draws it in its monospace stack — a path is a
// value, `settings.css:1272-1276`.
func sessionLines(session Session) []rowLine {
	var lines []rowLine
	if session.Dir != "" {
		lines = append(lines, rowLine{Text: session.Dir, Mono: true})
	}
	if meta := sessionMeta(session); len(meta) > 0 {
		lines = append(lines, rowLine{Segments: meta})
	}
	return lines
}

// sessionMeta is the meta line's segments — the state word, the screen size
// and the start time — each its own segment so the state word can take a
// brighter ink while the session runs and the quiet grey once it has exited. It
// is empty when the session reported none of the three.
func sessionMeta(session Session) []rowSegment {
	var segments []rowSegment
	if session.Note != "" {
		role := rowSegmentTertiary
		if session.Running {
			role = rowSegmentStrong
		}
		segments = append(segments, rowSegment{Text: session.Note, Role: role})
	}
	if session.Size != "" {
		segments = append(segments, rowSegment{Text: session.Size, Role: rowSegmentTertiary})
	}
	if !session.Created.IsZero() {
		segments = append(segments, rowSegment{
			Text: session.Created.Local().Format("2006-01-02 15:04"),
			Role: rowSegmentTertiary,
		})
	}
	return segments
}

// integrations lists the installed extensions, each with its state and a
// switch. An orphan package directory is listed too, with the two operations
// it can take.
func (a *App) integrations(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	integrations := a.installedIntegrations()
	a.pageColumn(c, func() {
		ui.Text(c, copy.IntegrationsHint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		// The panel's own content is `extensions.css`, written in literal px, so
		// it does not follow the interface step the way the settings pages' `--u`
		// metrics do (the page column's own gap above still scales). Everything
		// drawn inside wears the extension unit.
		a.extensionMetrics(func() {
			a.basePluginSection(c, copy)
			a.localToolSection(c, copy)
			a.detectedSection(c, copy)
			a.recommendedRow(c, copy)
			a.transferRow(c, copy)
			a.installRow(c, copy)
			a.installFolderRow(c, copy)
			// The PATH commands' own switch sits with the integrations, as the old
			// panel had it: it decides whether the search offers them at all.
			a.card(c, func() {
				a.checkbox(c, copy.ShowCommandsInSearch, copy.ShowCommandsInSearchHint,
					settings.ShowCommandsInSearch(a.Store.Snapshot()),
					func(on bool) {
						a.set(func(s *settings.Settings) { s.SetShowCommandsInSearch(on) })
					})
			})
			if len(integrations) == 0 {
				// The reference's shared empty region (`ExtensionsPanel.tsx:2462,
				// 3177-3191`): the installed list's own `Package` mark at 20px, the
				// state in one line, on the 92u list floor. The reference's
				// `EmptyState` also carries a first-load spinner and a filtered-query
				// echo on the hint line; this build's list is in-process and has no
				// filter, so neither has a source and only the resting arm is drawn.
				a.emptyState(c, "package", copy.IntegrationsEmpty, "", a.cardSpace(t, 5), 0, nil)
				return
			}
			a.extensionSection(c, copy.IntegrationsInstalled, len(integrations), func() {
				for _, integration := range integrations {
					a.integrationRow(c, copy, integration)
				}
			})
		})
	})
}

// basePlugin is one built-in plugin as the Integrations page lists it: its id
// (the same vocabulary the launcher's gear and the plugin configuration use),
// what it is called and does, the extra privacy line the clipboard carries,
// and whether it has a persisted on/off switch at all.
type basePlugin struct {
	ID         string
	Name       string
	Hint       string
	Privacy    string
	Toggleable bool
	Enabled    bool
}

// basePlugins is the shipped list, in the order the old panel listed it. The
// calculator has no persisted on/off field, so it carries no switch rather
// than a dead one.
func (a *App) basePlugins(copy i18n.Settings) []basePlugin {
	s := a.Store.Snapshot()
	return []basePlugin{
		{
			ID: settings.CustomPluginClipboard, Name: copy.BaseClipboard, Hint: copy.BaseClipboardHint,
			Privacy: copy.BaseClipboardPrivacy, Toggleable: true, Enabled: settings.ClipboardOf(s).Enabled,
		},
		{
			ID: settings.CustomPluginBrowser, Name: copy.BaseBrowser, Hint: copy.BaseBrowserHint,
			Toggleable: true, Enabled: settings.BrowserPluginOf(s).Enabled,
		},
		{ID: settings.CustomPluginCalculator, Name: copy.BaseCalculator, Hint: copy.BaseCalculatorHint},
	}
}

// basePluginSection is the Integrations page's own list of the built-in
// plugins: one card per plugin, naming it, saying what it does, and carrying
// the two controls it can have — the switch that turns it on or off, and the
// Configure button that opens its declarative configuration in the launcher
// (the mode's own gear opens the same sheet). The browser mode's message
// ("Enable it in Settings > Integrations") is this section; without it the
// hint would point at a door that does not exist.
func (a *App) basePluginSection(c *ui.Context, copy i18n.Settings) {
	if a.Actions.OpenPluginConfig == nil && a.Actions.SetPluginEnabled == nil {
		return
	}
	plugins := a.basePlugins(copy)
	a.extensionSection(c, copy.BasePlugins, len(plugins), func() {
		a.extensionHint(c, copy.BasePluginsHint, false)
		for _, plugin := range plugins {
			plugin := plugin
			a.card(c, func() { a.basePluginCard(c, copy, plugin) })
		}
	})
}

// basePluginCard is one built-in plugin's own card — the reference's
// `.extensions-base-plugin` (`extensions.css:2550-2563`): a 66px **floor**, a
// 16px gap between its two columns, a `12px 14px` padding pair, the panel's
// `--ext-border` hairline and the card ladder's `--radius-md` on the
// `--glass-control` plane (the face `card` already paints, which is why this
// draws the reference's *interior* and not a card of its own).
//
// The interior is the reference's two columns, aligned to the top rather than
// centred (`align-items: flex-start`):
//
//	.extensions-base-plugin__main  name, description, optional privacy note,
//	                               a `flex column` with a 3px gap
//	.extensions-base-plugin__actions  the Configure pill (24px, emphasis
//	                               step) and the switch — never the row's own
//	                               trailing slot (`cardRow`'s 44u floor and
//	                               8px gap are the settings card's, not this
//	                               card's)
//
// The three text lines are the reference's own registers: the name is the
// **title** step (13px) at line-height 1.5 with no weight of its own, the
// description the **emphasis** step (12px) at 1.5 in `--text-secondary`, and
// the privacy note the **body** step (11px) in `--text-warning` with a 2px top
// padding — `.settings-privacy-hint`'s ink, worn by the one row that discloses
// rather than describes. The port had flattened all three into one settings
// row: the name at the emphasis step, the description one register too quiet
// (`--text-muted`) and single-line, the note weight-500 with no padding.
func (a *App) basePluginCard(c *ui.Context, copy i18n.Settings, plugin basePlugin) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	enabled, changed := plugin.Enabled, false
	labelClicked := false
	// `min-height: 66px` (`--extensions-row-height` is 66) at the extension
	// panel's literal unit, with the card's own `align-items: flex-start`.
	// The floor is what the reference's own note buys (`extensions.css:2540-2549`):
	// the card's arithmetic — two 12px paddings, two 1px borders, a 19.5px name
	// line, a 3px gap and one 18px description line — lands on ~66.5px, so a
	// plugin whose description is short (or absent) must not draw a card
	// shorter than the one that carries it. Without the floor the calculator's
	// one-line card settled ~3px under the clipboard's three-line card, and the
	// column of cards lost its shared edge.
	// 66px is a `border-box` floor in the reference (base.css's `*` reset), so
	// the two 1px borders are *inside* it: the row's own floor is the 64px
	// interior. `MinHeight(Space(16))` is 64 at the panel's literal unit.
	row := ui.Row(c).FillWidth().Gap(t.Space(4)).MinHeight(t.Space(16)).
		AlignItems(ui.Start).Padding(t.Space(3), t.Space(3.5))
	row.Children(func() {
		// The copy column: `min-width: 0`, a `flex column` with the
		// reference's 3px gap. Its name and description wrap, as the
		// reference's do (no `nowrap` here, unlike a settings row's sublabel).
		main := ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(0.75))
		main.Children(func() {
			ui.Text(c, plugin.Name).FontSize(tokens.Title).LineHeight(1.5).
				TextColor(tokens.TextStrong)
			if plugin.Hint != "" {
				ui.Text(c, plugin.Hint).FontSize(tokens.Emphasis).LineHeight(1.5).
					TextColor(tokens.TextSecondary).Wrap()
			}
			if plugin.Privacy != "" {
				// `.extensions-base-plugin__privacy` + `.settings-privacy-hint`:
				// the body step in the warning ink, its own `2px 2px 0` padding.
				ui.Text(c, plugin.Privacy).FontSize(tokens.Body).
					Padding(t.Space(0.5), t.Space(0.5), 0, t.Space(0.5)).
					TextColor(t.Warning).Wrap()
			}
		})
		if main.Clicked() {
			labelClicked = true
		}
		// The controls column: `flex: 0 0 auto`, centred, the reference's 10px
		// gap. The switch takes the reference's 2px optical nudge off the
		// card's top edge.
		actions := ui.Row(c).Shrink(0).Gap(t.Space(2.5)).AlignItems(ui.Center)
		actions.Children(func() {
			if a.Actions.OpenPluginConfig != nil {
				// The visible text is the reference's "Configure"; the
				// accessible name names the plugin, because three buttons
				// reading "Configure" alone are three buttons a screen
				// reader cannot tell apart. It is the card's own
				// one-step-down pill
				// (`.extensions-base-plugin__configure`,
				// `extensions.css:2610-2614`): a 24u height and the
				// emphasis step, so it never out-shouts the plugin name
				// beside it.
				configure := a.extensionActionButton(c, copy.PluginConfigure,
					t.Space(6), t.Space(3), tokens.Emphasis)
				configure.Label(copy.PluginConfigure + " " + plugin.Name)
				if configure.Clicked() {
					a.Actions.OpenPluginConfig(plugin.ID)
				}
			}
			if plugin.Toggleable && a.Actions.SetPluginEnabled != nil {
				on := enabled
				sw := a.switchControl(c, &on).Label(plugin.Name).Margin(t.Space(0.5), 0, 0, 0)
				if sw.Changed() {
					enabled, changed = on, true
				}
			}
		})
	})
	// The name is a hit target, as every switch row's is: pressing the words
	// toggles the control beside them. The reference wires only the switch
	// itself; the wider target is this build's own addition.
	if labelClicked && plugin.Toggleable && a.Actions.SetPluginEnabled != nil {
		enabled, changed = !enabled, true
	}
	if changed {
		a.Actions.SetPluginEnabled(plugin.ID, enabled)
	}
}

// RecommendedTool is one shipped package, as the page lists it.
type RecommendedTool struct {
	ID          string
	Name        string
	Description string
	// Installed is true when the inventory already has it.
	Installed bool
}

// recommendedRow lists the shipped tool packages that are not installed yet,
// each with the one press that installs it. Nothing is shown once everything
// shipped is installed.
func (a *App) recommendedRow(c *ui.Context, copy i18n.Settings) {
	if a.Actions.ConnectRecommended == nil || a.Recommended == nil {
		return
	}
	var pending []RecommendedTool
	for _, tool := range a.Recommended() {
		if !tool.Installed {
			pending = append(pending, tool)
		}
	}
	if len(pending) == 0 {
		return
	}
	a.section(c, copy.IntegrationsRecommended, copy.IntegrationsRecommendedHint, func() {
		a.card(c, func() {
			for _, tool := range pending {
				tool := tool
				a.row(c, tool.Name, tool.Description, func() {
					if a.actionButton(c, copy.IntegrationsConnect).Clicked() {
						a.Actions.ConnectRecommended(tool.ID)
					}
				})
			}
		})
	})
}

// transferRow is the export and import of the whole list: a backup of what is
// installed and how it is configured, and the way back in on another machine.
//
// The reference's panel (`src/ExtensionsPanel.tsx:1206-1240,2300-2350`) puts
// the pair behind one overflow menu, disables them while a transfer is in
// flight, shows that transfer's busy line beside them, and renders the last
// import's report under the row. This build keeps the two buttons in the row
// (its one authoring row) but follows the same three states: disabled while
// busy, one busy line, and the report block below.
func (a *App) transferRow(c *ui.Context, copy i18n.Settings) {
	t := a.cardTheme(c)
	busy := a.TransferBusy != ""
	ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).
		Padding(0, 0, t.Space(1), 0).Children(func() {
		if a.Actions.ExportIntegrations != nil {
			if a.actionButton(c, copy.IntegrationsExport).Disabled(busy).Clicked() {
				a.TransferReport = nil
				a.Actions.ExportIntegrations()
			}
		}
		if a.Actions.ImportIntegrations != nil {
			if a.actionButton(c, copy.IntegrationsImport).Disabled(busy).Clicked() {
				a.TransferReport = nil
				a.Actions.ImportIntegrations()
			}
		}
		switch a.TransferBusy {
		case "export":
			ui.Text(c, copy.IntegrationsExporting).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		case "import":
			ui.Text(c, copy.IntegrationsImporting).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
	})
	if a.TransferReport != nil {
		a.transferReport(c, copy)
	}
}

// installRow is the npm install field: a package name, an optional version,
// and the button that asks the shell to install it.
func (a *App) installRow(c *ui.Context, copy i18n.Settings) {
	if a.Actions.InstallFromRegistry == nil {
		return
	}
	t := a.cardTheme(c)
	name, version := a.installName, a.installVersion
	requested := false
	ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).
		Padding(0, 0, t.Space(1), 0).Children(func() {
		a.textInput(c, &name).Placeholder(copy.IntegrationsPackageHint).Label(copy.IntegrationsPackageHint).Grow(1)
		a.textInput(c, &version).Placeholder(copy.IntegrationsVersionHint).Width(160)
		if a.actionButton(c, copy.IntegrationsInstall).Clicked() {
			requested = true
		}
	})
	a.installName, a.installVersion = name, version
	if requested && strings.TrimSpace(name) != "" {
		a.Actions.InstallFromRegistry(strings.TrimSpace(name), strings.TrimSpace(version))
		a.installName, a.installVersion = "", ""
		a.Body.Y = 0
	}
}

// stateMark is an integration's own state, in the reference's language
// (`.extension-status`): a running integration is accent *text*, a broken one
// is a warm chip, and anything else is a quiet secondary mark. It is a state,
// not an action — the row's switch is the action — so no state spends an accent
// fill. (The running accent *text* is the reference's own reading of a live
// process and predates §5; it is a label colour, not one of the three accent
// surfaces, and is recorded in the P8-c list rather than changed here.)
//
// The broken chip is the reference's `.extension-status--broken`
// (`extensions.css:1020-1025`): the warm system plane (`tokens.RaisedWarm`, the
// amber surface — *not* the warning ink at a low alpha, which tinted the chip
// with the label's own hue) under a 1px hairline, at the `--ext-radius` corner.
//
// P8-b 重定基线：环 `t.Warning` → 合约 §6.1 第 8 条的 `ControlEdge`，字重
// 620 → 600。预热面（系统语义色）保留：它说的是「这条不工作」，而环是结构，
// 不是语义——合约下环只有两档（`HairlineSoft` 的结构线与 `ControlEdge` 的
// 控件边），warning 环那种「彩色描边」属于旧 glass 语言的彩色 rim。
func (a *App) stateMark(c *ui.Context, integration Integration, copy i18n.Settings) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	text := integration.state(copy)
	switch {
	case integration.Broken:
		chip := ui.Row(c).Padding(t.Space(0.5), t.Space(1.5)).Radius(tokens.RadiusSM).
			Background(tokens.RaisedWarm)
		chip.InsetShadow(0, 0, 0, 1, tokens.ControlEdge)
		chip.Children(func() {
			ui.Text(c, text).FontSize(t.FontSize - 1).FontWeight(600).TextColor(t.Warning)
		})
	case integration.Enabled:
		// P8-d 重定基线（§5 判据）：「运行中」曾用 `t.Accent`。它说的是「这条
		// 是活的」——一个持续状态，不是「哪个被选中 / 哪个是开的」。改
		// `TextStrong`；`default` 分支的 `TextSecondary` 就是另一档，两档之间
		// 靠明度分层（§5 新补的那句：中性档之间的差异不属本预算，靠明度分层
		// 是鼓励的做法），所以「运行中 vs 关」仍然分得开，只是不再用色相。
		ui.Text(c, text).FontSize(t.FontSize - 1).TextColor(tokens.TextStrong)
	default:
		ui.Text(c, text).FontSize(t.FontSize - 1).TextColor(tokens.TextSecondary)
	}
}

// runtimeUnavailableBadge is the reference's `.extension-status--broken`
// chip in the meta line (`src/extensions/ExtensionRow.tsx:258-272`): a warm
// chip with the warning keyline, reading "Tool unavailable" (or its
// localisation), with the specific cause beside it only when the list knows
// one. It is a state mark, not an action — the row's repair button (drawn
// separately) is the action, and this chip only says why the button is there.
//
// Its face is `stateMark`'s broken chip: the warm system plane
// (`tokens.RaisedWarm`) fill under the `ControlEdge` hairline at the
// `--ext-radius` corner — the same three facts the reference's one
// `.extension-status--broken` rule carried for both readers, minus the coloured
// ring the contract does not allow outside §5's three places (§6.1 第 8 条).
func (a *App) runtimeUnavailableBadge(c *ui.Context, copy i18n.Settings, integration Integration) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	label := copy.RuntimeUnavailable
	if code := integration.FailureCode; code != "" {
		if cause := copy.ErrorCodeLabel(code); cause != "" && cause != code {
			label += " · " + cause
		}
	}
	chip := ui.Row(c).Padding(t.Space(0.5), t.Space(1.5)).Radius(tokens.RadiusSM).
		Background(tokens.RaisedWarm)
	chip.InsetShadow(0, 0, 0, 1, tokens.ControlEdge)
	chip.Children(func() {
		ui.Text(c, label).FontSize(t.FontSize - 1).FontWeight(600).TextColor(t.Warning)
	})
	return chip
}

// integrationMeta is the reference's single `.extension-row__meta` line under
// the title (`src/extensions/ExtensionRow.tsx:255-282`): the package identity,
// the integration kind, the runtime source and the state word, in the caption
// register the reference drew them. The three plain facts share the tertiary
// ink and a `·` separator; the state word keeps its own colour (accent while
// running, a warm chip while broken, secondary while off) through stateMark,
// which is why it is a separate node rather than one joined string.
func (a *App) integrationMeta(c *ui.Context, copy i18n.Settings, integration Integration) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	facts := make([]string, 0, 3)
	if id := integration.identity(); id != "" {
		facts = append(facts, id)
	}
	if kind := integration.integrationKind(copy); kind != "" {
		facts = append(facts, kind)
	}
	if source := integration.runtimeSourceLabel(copy); source != "" {
		facts = append(facts, source)
	}
	sep := "·"
	ui.Row(c).FillWidth().Wrap().GapX(t.Space(2)).GapY(t.Space(1)).AlignItems(ui.Center).Children(func() {
		for i, fact := range facts {
			if i > 0 {
				ui.Text(c, sep).FontSize(tokens.Caption).TextColor(tokens.TextTertiary).Opacity(0.7)
			}
			ui.Text(c, fact).FontSize(tokens.Caption).FontWeight(500).TextColor(tokens.TextTertiary)
		}
		if len(facts) > 0 {
			ui.Text(c, sep).FontSize(tokens.Caption).TextColor(tokens.TextTertiary).Opacity(0.7)
		}
		a.stateMark(c, integration, copy)
	})
}

// healthDetail draws the stored probe report's own lines under the Health row
// (the old build's `extension-health` block): every probe the last run
// recorded with its duration, then the failed ones with the reason a retry
// decision needs, then when the run happened. The status word above stays
// the row's value; these lines are its detail.
func (a *App) healthDetail(c *ui.Context, copy i18n.Settings, report *HealthReport) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	// The block the reference draws under the Health row is a column with an
	// 8u gap (`ExtensionsPanel.tsx:2920-2972`, `extensions.css:2182`), and
	// each of its groups opens with the reference's own `__label`: a caption
	// at 620 on the secondary ink, one voice with the block labels around it
	// (`extensions.css:2191`) — never a ruled card row with a 44u floor and a
	// separator. The port drew each group that way, so the checks read as
	// four unrelated settings rows instead of one block's list.
	//
	// The reference's `capabilities` half (its `Features` / `Limitations`
	// groups) is deliberately not drawn: the original never populated it (its
	// reprobe path passed `Default::default()`, `probe_executor.rs:18`), so
	// both groups are dead on both sides — see the note in
	// `internal/extensions/helpderive.go`. Its own empty state is the same
	// dead end: the port sets `Health` and `HealthReport` together.
	block := ui.Column(c).FillWidth().Gap(t.Space(2))
	block.Children(func() {
		if len(report.Probes) > 0 {
			// The checks group keeps the reference's own label and then one
			// probe line each — mark, monospace id, trailing duration.
			ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
				ui.Text(c, copy.IntegrationsHealthChecks).FontSize(tokens.Caption).
					FontWeight(620).TextColor(tokens.TextSecondary)
				for _, probe := range report.Probes {
					probe := probe
					a.healthProbeRow(c, probe)
				}
			})
		}
		var failed []HealthProbe
		for _, probe := range report.Probes {
			if !probe.Passed {
				failed = append(failed, probe)
			}
		}
		if len(failed) > 0 {
			ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
				ui.Text(c, copy.IntegrationsHealthFailures).FontSize(tokens.Caption).
					FontWeight(620).TextColor(tokens.TextSecondary)
				for _, probe := range failed {
					probe := probe
					a.healthFailureCard(c, copy, probe)
				}
			})
		}
		if when := healthCheckedAt(report.CheckedAt); when != "" {
			ui.Text(c, copy.IntegrationsHealthCheckedAt(when)).FontSize(tokens.Caption).
				TextColor(t.TextMuted).Wrap()
		}
	})
}

// diagnosticsSection draws the drawer's Diagnostics block at the reference's
// own face (`main:src/ExtensionsPanel.tsx:2892-2902`,
// `main:src/styles/extensions.css:2207-2210,2324-2350`): a block label, the
// verdict word as a summary line, then one grid row per check — the check's
// own status word in a fixed 54px column and its message in the rest, each
// row closed by a 1px hairline. What it replaces was the whole check reduced
// to one ruled card row whose grey subline carried the summary and whose
// trailing slot carried the integration's own state word (a third fact that
// had nothing to do with the check).
func (a *App) diagnosticsSection(c *ui.Context, copy i18n.Settings, integration Integration) {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	// The block label: the drawer's `.extension-detail-block h4` — sentence case
	// at the secondary ink, caption 620 (`extensions.css:2008-2013`), one voice
	// with the metadata grid's own `dt` labels.
	ui.Row(c).FillWidth().Children(func() {
		ui.Text(c, copy.IntegrationsDiagnostics).FontSize(tokens.Caption).FontWeight(620).
			TextColor(tokens.TextSecondary)
	})
	if !integration.diagnosisRan() {
		// No check ran at all (the provider answered with nothing): the
		// reference's own empty line (`extensions.css:2054-2059`,
		// `--text-muted` body). A check that ran but reported no rows is *not*
		// this state — its verdict is the summary line on its own.
		ui.Text(c, copy.IntegrationsDiagnosticsUnavailable).FontSize(t.FontSize).
			TextColor(t.TextMuted).Wrap()
		return
	}
	// The verdict is the reference's plain summary span: `justify-self: start`,
	// a `2px 0` padding pair, no pane, no dot and **no weight of its own** —
	// the rule sets colour and `capitalize` alone, and the caption step
	// inherits the body's 400 (`extensions.css:2324-2332`). The port drew it
	// at 620, one step heavier than the reference's own summary. Its colour
	// follows the *status word it prints*, the same classification the check
	// rows below use (the reference ranged `--${diagnose.status}` over both,
	// `ExtensionsPanel.tsx:2896-2899`), so a verdict the finding flag does not
	// cover still wears its own state's ink.
	verdict := diagnosisVerdict(integration, copy)
	ui.Text(c, capitalizeWord(verdict)).FontSize(tokens.Caption).
		TextColor(statusInk(verdict, tokens, t, tokens.TextSecondary))
	// One row per check: the reference's `.extension-diagnostics > div` is a
	// `54px minmax(0,1fr)` grid with an 8px gap, a `8px 0` padding pair and a
	// 1px bottom hairline — dropped on the last row
	// (`extensions.css:2344-2350,2306-2308`).
	for i, check := range integration.DiagnosisChecks {
		a.diagnosisCheckRow(c, check, i == len(integration.DiagnosisChecks)-1)
	}
}

// diagnosisCheckRow is one `.extension-diagnostics > div`: the status word in
// the grid's own 54px column and the message in the rest, on the caption step.
func (a *App) diagnosisCheckRow(c *ui.Context, check DiagnosisCheck, last bool) {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	// The status word is the provider's *raw* vocabulary (`extensions.css:2341`
	// capitalises only the first letter, `capitalize`), so it is not translated:
	// it is the word the provider answered in.
	ui.Column(c).FillWidth().Children(func() {
		grid := ui.Grid(c).FillWidth().Padding(t.Space(2), 0).
			ColumnTracks(ui.Fixed(a.cardSpace(t, 13.5)), ui.Fr(1)).
			GapX(t.Space(2)).AlignItems(ui.Start)
		grid.Children(func() {
			ui.Text(c, capitalizeWord(check.Status)).FontSize(tokens.Caption).
				TextColor(statusInk(check.Status, tokens, t, tokens.TextTertiary)).ColumnStart(1)
			ui.Text(c, check.Message).FontSize(tokens.Caption).
				TextColor(t.TextMuted).Wrap().ColumnStart(2)
		})
		if !last {
			ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).Shrink(0)
		}
	})
}

// diagnosisVerdict is the summary line's word: the provider's own overall
// status when it gave one, and the integration's state word when the check
// itself said nothing (`Diagnosis` is that fallback's spelling).
func diagnosisVerdict(integration Integration, copy i18n.Settings) string {
	if status := strings.TrimSpace(integration.DiagnosisStatus); status != "" {
		return status
	}
	if word := strings.TrimSpace(integration.Diagnosis); word != "" {
		return word
	}
	return integration.state(copy)
}

// diagnosisRan reports whether any check reported anything at all: the
// provider's own verdict, a summary word this build derived, or a check row.
// None of the three is the reference's "Diagnostics are unavailable." state.
func (i Integration) diagnosisRan() bool {
	return strings.TrimSpace(i.DiagnosisStatus) != "" ||
		strings.TrimSpace(i.Diagnosis) != "" || len(i.DiagnosisChecks) > 0
}

// capitalizeWord applies the reference's `text-transform: capitalize` to one
// provider-supplied word (`extensions.css:2331,2341`): the first letter is
// uppercased and the rest is left exactly as the provider spelled it, so a
// vocabulary this build does not know still prints its own word.
func capitalizeWord(word string) string {
	trimmed := strings.TrimSpace(word)
	if trimmed == "" {
		return trimmed
	}
	first := []rune(trimmed)[0]
	return strings.ToUpper(string(first)) + string([]rune(trimmed)[1:])
}

// statusInk is one status word's colour, the reference's own split: `ok` and
// `healthy` take the strong ink, `error` and `broken` the warning, and every
// other word the caller's quiet register (`extensions.css:2334-2346`). The
// summary line and the check rows share this classifier because the reference
// ranged the *same* `--${diagnose.status}` / `--${check.status}` modifier over
// both; only the fallback differs — the summary sits on `--text-secondary`, a
// check row on the quieter `--text-tertiary` (`:2329` vs `:2360`).
//
// P8-d 重定基线（§5 判据）：`ok`/`healthy` 曾是 `t.Accent`。accent 只回答
// 「哪个被选中 / 哪个是开的」（§5 四类），一个「这条好」的**结果语义**不在
// 其中——它既不是控件状态，也不是主 CTA。所以这两个词降到 `TextStrong`（与
// 它右边那列消息的 `TextMuted` 仍差两档，读得出「结论 vs 说明」），而
// `error`/`broken` 保留 `t.Warning`：那是**语义色**，不是 accent（§5 第 3 条的
// 状态色说的是控件自己的选中/开启，语义色另有一支）。
func statusInk(status string, tokens theme.Tokens, t *ui.Theme, quiet ui.Color) ui.Color {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ok", "healthy":
		return tokens.TextStrong
	case "error", "broken":
		return t.Warning
	default:
		return quiet
	}
}

// healthDuration is one probe's wall time, as the old card printed it: the
// raw millisecond count, no unit conversion.
func healthDuration(ms uint64) string { return strconv.FormatUint(ms, 10) + "ms" }

// healthExitCode is a probe's exit code, or `-` for one that never ran to an
// exit (the host records -1). The old card's `exit: {code}` reads the same
// way whether the code came from a real exit or from a timeout.
func healthExitCode(code int) string {
	if code < 0 {
		return "-"
	}
	return strconv.Itoa(code)
}

// healthCheckedAt is the stored RFC 3339 timestamp as a readable local time.
// An empty or unparseable value is returned trimmed rather than swallowed: a
// timestamp this build cannot read is still what the repository holds.
func healthCheckedAt(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.Local().Format("2006-01-02 15:04")
	}
	return raw
}

// approvalDetail draws the recorded permission approval under the
// integration's own rows (the old host's R7-8 block): when it happened, the
// manifest digest it is bound to with the on-disk digest beside it — "what I
// approved" and "what is here now" as two readable rows rather than a
// boolean — the permissions it covered, tiered by the same host-enforced
// vocabulary the permission audit uses, and one warning line when the bytes
// on disk no longer match the approval. No record is one empty-state line,
// not a block of placeholders.
func (a *App) approvalDetail(c *ui.Context, copy i18n.Settings, record *ApprovalRecord) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	if record == nil {
		a.row(c, copy.ApprovalRecord, copy.ApprovalRecordNone, func() {})
		return
	}
	a.row(c, copy.ApprovalRecord, "", func() {})
	a.row(c, copy.ApprovalRecordApprovedAt, approvalApprovedAt(record.At), func() {})
	digest := copy.ApprovalRecordDigestUnknown
	if short := extensions.ShortDigest(record.ApprovedDigest); short != "" {
		digest = short
	}
	current := copy.ApprovalRecordDigestUnknown
	if short := extensions.ShortDigest(record.CurrentDigest); short != "" {
		current = short
	}
	a.row(c, copy.ApprovalRecordDigest, digest, func() {})
	a.row(c, copy.ApprovalRecordDigestCurrent, current, func() {})
	language := a.Store.Snapshot().Language
	stale := extensions.ApprovalIsStale(record.ApprovedDigest, record.CurrentDigest)
	// The changed-manifest warning is *reported*, never acted on: "the approval
	// no longer covers the bytes on disk" is a prompt to re-approve, not a
	// version state to restore. Unknown (either side missing) stays silent. It
	// rides under the permissions row as the reference's own
	// `.extension-approval-record__stale` banner (`extensions.css:2171-2179`),
	// never as a nameless ruled card row with a lone `!`.
	staleBelow := func() {
		if stale {
			a.approvalStaleBanner(c, copy)
		}
	}
	if len(record.Enforced) == 0 && len(record.Disclosure) == 0 {
		a.row(c, copy.ApprovalRecordPermissions, copy.ApprovalRecordPermissionsNone, func() {}, staleBelow)
	} else {
		// The reference draws the approved permissions as *pills* under the
		// label, never as one ruled row per id: an enforced id wears the
		// recognizable raised-quiet + accent-keyline face its twin in the audit
		// list has, a disclosure id the plain icon-surface face
		// (`.extension-approval-record__permission`, `extensions.css:2149-2168`).
		a.row(c, copy.ApprovalRecordPermissions, "", func() {}, func() {
			ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
				if len(record.Enforced) > 0 {
					ui.Text(c, copy.PermissionsEnforcedLabel).FontSize(tokens.Caption).
						FontWeight(620).TextColor(tokens.TextSecondary)
					a.approvalPermissionPills(c, language, record.Enforced, true)
				}
				if len(record.Disclosure) > 0 {
					ui.Text(c, copy.PermissionsDisclosureLabel).FontSize(tokens.Caption).
						FontWeight(620).TextColor(tokens.TextSecondary)
					a.approvalPermissionPills(c, language, record.Disclosure, false)
				}
			})
		}, staleBelow)
	}
}

// approvalApprovedAt is the record's Unix seconds as a readable local time
// (the old host's `toLocaleString`; the health report's own timestamp shape).
// A record without a time is the zero row's own absence: no line at all.
func approvalApprovedAt(at uint64) string {
	if at == 0 {
		return ""
	}
	return time.Unix(int64(at), 0).Local().Format("2006-01-02 15:04")
}

// integrationRowFace is the decoration the integration row's own state asks
// for. A broken row wears `.extension-row--broken`: the warm raised pane, the
// warning keyline and the warning rule, so an unusable integration reads at a
// glance instead of only in its status word. A row a `floter://register` link
// named is the row the link **chose**, so it wears the contract's neutral
// selection rung (`RowSelected`, docs/raycast-design.md §4.3) with no keyline
// and no leading rule. Every other row keeps the card's plane and an unlit
// rule (hover lights it in cardRow).
//
// P8-c 重定基线（§6.3 第 2 条 / §5）：命名行的 `RaisedQuiet` + `AccentEdge` +
// accent bar → `RowSelected`，accent 只留给前导图标。旧面是 base.css 的
// `.extension-row--register`（升起的静默面 + accent 内环 + accent 前导条），
// 三处都是 accent；而 §5 的判据是「accent 只能回答『哪个被选中』，不能回答
// 『这块面好看』」——这一行确实是被选的，但**选中行**在列表里从不拿 accent
// （§4.3 末句），所以选中本身用中性档，前导图标是唯一可带 accent 的位置。
func (a *App) integrationRowFace(c *ui.Context, integration Integration) rowFace {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	switch {
	case integration.Broken:
		return rowFace{Face: tokens.RaisedWarm, FaceSet: true, Inset: t.Warning, Bar: t.Warning}
	case integration.Highlighted:
		return rowFace{Face: tokens.RowSelected, FaceSet: true}
	}
	return rowFace{}
}

// integrationRow is one integration as a card: its identity and state, what
// it declares, what it can run (each command a row with its switch and its
// alias), and the row of doors — check, re-scan, uninstall.
func (a *App) integrationRow(c *ui.Context, copy i18n.Settings, integration Integration) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	// The row's own one-word freshness, read once and used twice: the mark
	// beside the name and the fuller row below it, projected from the same
	// value so the two can never disagree.
	freshness, hasFreshness := a.freshnessOf(integration)
	// The card wears the reference extension row's own geometry, not the
	// settings card's: `.extension-row` is a 66px row with 8px of padding and a
	// 4px line gap (`extensions.css:15-16,733-744,879-880`), so the title row is
	// a 66u floor and every row under it — Permissions, Freshness, the command
	// list, the action cluster — shares the same 8u inset and 4u line gap. That
	// is what makes the title's name, the sub-rows' labels and the card's
	// separators line up on one text column.
	a.cardGeometry(c, t.Space(2), t.Space(16.5), t.Space(1), "", func() {
		// The link's mark: a row the `floter://register` link named wears the
		// reference's own sentence under its description (the reference drew a
		// dot before the name; a native pane has no tooltip, so the word itself
		// is the mark). An unnamed row draws nothing.
		highlight := ""
		if integration.Highlighted {
			highlight = copy.RegisterHighlight
		}
		// The title row is the reference's own `.extension-row__title`: the
		// name, the version beside it and the freshness mark trailing both,
		// all in the label column, with the runtime-unavailable chip as the
		// row's only control. The old port printed the version on a separate
		// identity row and the freshness dot in the control slot.
		a.cardRow(c, ui.Center, func() { a.packageChip(c, false, integration.Highlighted) }, a.integrationRowFace(c, integration), func() {
			if !integration.RuntimeAvailable {
				a.runtimeUnavailableBadge(c, copy, integration)
			}
		}, func() {
			title := ui.Row(c).FillWidth().Wrap().GapX(t.Space(2)).GapY(t.Space(1)).AlignItems(ui.Center)
			title.Children(func() {
				ui.Text(c, integration.Name).FontSize(t.FontSize + 1).FontWeight(580).TextColor(tokens.TextStrong)
				if integration.Version != "" {
					ui.Text(c, "v"+integration.Version).FontSize(t.FontSize).FontWeight(500).TextColor(tokens.TextTertiary)
				}
				if hasFreshness {
					a.freshnessDot(c, copy, freshness)
				}
			})
			if integration.Description != "" {
				ui.Text(c, integration.Description).FontSize(t.FontSize).FontWeight(500).TextColor(t.TextMuted)
			}
			if highlight != "" {
				ui.Text(c, highlight).FontSize(t.FontSize).FontWeight(500).TextColor(t.Warning)
			}
			// The reference's single `.extension-row__meta` line replaces the
			// old separate identity row: the package identity, the kind, the
			// runtime source and the state word together under the title.
			a.integrationMeta(c, copy, integration)
		})

		if len(integration.Permissions) > 0 && !integration.Orphan {
			labelClicked := a.row(c, copy.IntegrationsPermissions, a.permissionLine(integration, copy), func() {
				if a.actionButton(c, copy.IntegrationsPermissions).Clicked() {
					a.toggleAudit(integration.ID)
				}
			}, func() { a.permissionAudit(c, copy, integration) })
			if labelClicked {
				a.toggleAudit(integration.ID)
			}
		}

		// The freshness of the command list: when it was last derived, how
		// that went, and how the list moved. Nothing is shown for an
		// integration nothing has ever probed — "unknown" is its own state,
		// not a row of zeroes.
		if hasFreshness {
			a.freshnessSection(c, copy, integration, freshness)
		}

		// The stored lifecycle health: the probe report lives in the repository,
		// so the row says how the tool was last found to be — before any check
		// runs in this session — and a check refreshes it. The status word is
		// the row's value; the live findings are the diagnostics row below.
		if integration.Health != "" {
			a.row(c, copy.IntegrationsHealth, "", func() {
				// The reference's heading carries the re-run control beside the
				// block's name (`.extension-detail-heading` is a
				// `space-between` flex row, `extensions.css:2015-2020`), and the
				// status chip opens the report under it. This build's row *is*
				// that heading, so the chip and the button share the slot: the
				// chip is the block's value and the button its one action.
				ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
					a.healthStatusChip(c, copy, integration.Health, integration.HealthStatus)
					// `healthReprobe` runs only while the stored report has a schema
					// the host can execute (`Reprobe` refuses with `ErrNoProbes`
					// otherwise), which is exactly the state the reference gates
					// the button on: `healthReport &&` (`:2909`).
					if a.Actions.ReprobeHealth != nil && integration.HealthReport != nil {
						label := copy.IntegrationsHealthReprobe
						if a.extensionIconButton(c, "refresh", label, a.rowBusy(integration.ID)).Clicked() {
							a.Actions.ReprobeHealth(integration.ID)
						}
					}
				})
			})
		}
		if integration.HealthReport != nil {
			a.healthDetail(c, copy, integration.HealthReport)
		}

		// The approval record: every installed integration has either one or
		// the empty state — an orphan has neither (no repository entry names
		// it, so there is nothing to record).
		if !integration.Orphan {
			a.approvalDetail(c, copy, integration.Approval)
		}

		if integration.diagnosisRan() {
			a.diagnosticsSection(c, copy, integration)
		}
		if reason := failureReason(copy, integration.FailureCode, integration.FailureDetail); reason != "" {
			a.row(c, copy.IntegrationsFailureDetail, "", func() {
				ui.Text(c, reason).FontSize(t.FontSize - 1).TextColor(t.Danger)
			})
		}
		if integration.Error != "" {
			a.row(c, copy.IntegrationsDiagnostics, integration.Error, func() {
				ui.Text(c, "\u25cf").FontSize(t.FontSize - 3).TextColor(t.Danger)
			})
		}

		if configOwnerIsHost(integration) && !integration.Orphan {
			// The row *is* the reference's configuration block title: the panel
			// drew `<h4>{t("settings.extensions.configuration")}</h4>` above the
			// form (`ExtensionsPanel.tsx:2971-2975`), and the block's own action
			// carried the verb. Labelling the row with the verb instead printed
			// "Configure" twice on one line and gave the block a different name
			// from its tool-owned sibling, which already leads with
			// `ConfigSection` (see configOpenToolRow).
			opened := a.row(c, copy.ConfigSection, "", func() {
				if a.actionButton(c, copy.ConfigOpen).Clicked() {
					a.toggleConfig(integration.ID)
				}
			}, func() { a.configFields(c, copy, integration) }, func() {
				// The discard bar is the reference's own sibling of the form
				// (`ExtensionsPanel.tsx:3029-3035`): it appears while the
				// fold-away question is armed and the draft is still dirty.
				if a.configDiscard[integration.ID] && a.configDirty(integration, a.draft(integration)) {
					a.configDiscardBar(c, copy, integration)
				}
			})
			if opened {
				a.toggleConfig(integration.ID)
			}
		} else if configOwnerIsTool(integration) && !integration.Orphan {
			// A tool-owned configuration has no host form: the reference drew an
			// "Open configuration" button instead, running the provider's own
			// `openCommand` (`ExtensionsPanel.tsx:2977-2986`). The button is
			// disabled when the provider declared no `openCommand`, exactly as
			// the reference's `disabled={!configuration.openPlan}` did.
			a.configOpenToolRow(c, copy, integration)
		}

		// Only the commands the provider's descriptor declared are *commands* to
		// this panel. The host synthesizes an editor command for a tool-owned
		// configuration, and the reference never lists it: its command list is
		// `externalCommands` filtered by extension, which is the provider
		// registry (`main:src-tauri/src/extensions/catalog.rs:427-449`), and a
		// tool-owned configuration is the "Configuration" block above with an
		// "Open configuration" action (`ExtensionsPanel.tsx:2971-2986`). The
		// synthesized row here carried a launcher switch and an alias field for
		// a command the launcher does not accept (see the shell's
		// `launcherCommands`), i.e. two controls that could not do anything.
		if commands := declaredCommands(integration.Commands); len(commands) > 0 && !integration.Orphan {
			// The per-command switches' caption and the one line that says
			// what a switch does, printed above the list rather than repeated
			// on every row (the old build's R39).
			a.sectionHeading(c, copy.IntegrationsCommands, nil)
			a.sectionHint(c, copy.IntegrationsCommandsHint)
			a.commandList(c, copy, integration.ID, commands)
		}

		if len(integration.Params) > 0 {
			a.runForm(c, copy, integration)
		}

		// The last background run's output rides the row it came from: the
		// panel run that took the background route leaves it here (see
		// shell.runWithParams).
		a.runOutput(c, copy, integration)

		if integration.Orphan {
			// An orphan is a package directory the repository does not name:
			// the row offers to graft it in or to remove it, and nothing
			// else — there is no record to enable, check or uninstall.
			a.row(c, copy.IntegrationsOrphan, copy.IntegrationsAdoptHint, func() {
				if a.Actions.AdoptIntegration != nil {
					if a.actionButton(c, copy.IntegrationsAdopt).Clicked() {
						a.Actions.AdoptIntegration(integration.ID)
					}
				}
				if a.Actions.DeleteOrphan != nil {
					if a.actionButton(c, copy.IntegrationsDeleteOrphan).Clicked() {
						a.Actions.DeleteOrphan(integration.ID)
					}
				}
			})
			return
		}

		enabled := integration.Enabled
		enableChanged := false
		// The actions row keeps no label of its own: the reference drew the
		// action cluster and the toggle beside the row's title, not under an
		// "Enabled" heading (`src/extensions/ExtensionRow.tsx:288-458`). The
		// switch's own accessible name is what a test and a screen reader read.
		// busy is the whole-row lock: while any operation on this integration is
		// in flight the reference disabled *every* control in the cluster and
		// the switch together (`const busy = Boolean(operation)` /
		// `disabled={busy}`, `src/extensions/ExtensionRow.tsx:131,301-419,447`),
		// not only the button that was pressed.
		busy := a.rowBusy(integration.ID)
		// rowAction draws one of the row's action pills, disabled while the row
		// is busy. The one button whose own work is in flight (the re-check or
		// the reconnect) is drawn by repairAction in its busy face instead.
		rowAction := func(label string) ui.Element {
			button := a.actionButton(c, label)
			if busy {
				button = button.Disabled(true)
			}
			return button
		}
		a.row(c, "", "", func() {
			// The row's Run button: on a runnable row it runs the integration —
			// straight away when it declares no inputs, or unfolding the inline
			// parameter form when it does (the same button is the disclosure, so
			// there is no second control to learn). A row that cannot run keeps
			// the button but disabled, with the reason in its tooltip, exactly as
			// the reference's `extensions-icon-button--row` did
			// (`src/extensions/ExtensionRow.tsx:381-399`).
			if a.Actions.RunWithParams != nil {
				runnable := integration.Running && !integration.Broken
				run := a.actionButton(c, copy.RunAction)
				if !runnable {
					run = run.Tooltip(copy.RunActionUnavailable).Disabled(true)
				}
				if busy {
					run = run.Disabled(true)
				}
				if run.Clicked() && runnable {
					if len(integration.Params) > 0 {
						if a.runOpen == nil {
							a.runOpen = map[string]bool{}
						}
						a.runOpen[integration.ID] = !a.runOpen[integration.ID]
						a.runError = ""
					} else {
						// No inputs: run at once. There is no form to print a
						// refusal under, and a run with no declared parameters
						// cannot raise a parameter failure, so the returned
						// failure is always nil here.
						a.Actions.RunWithParams(integration.ID, nil)
					}
				}
			}
			// A broken row's entry is the re-check, not a reconnect: the
			// reference drew the Wrench on a `broken` row and the reconnect
			// button on every other unavailable one
			// (`src/extensions/ExtensionRow.tsx:313-330,345-370`). A re-check
			// verifies the installed package and falls through to the search
			// path itself when the runtime is what moved, so a broken row
			// still has its way back.
			if integration.Broken {
				if a.Actions.RepairIntegration != nil {
					a.repairAction(c, integration.ID, copy.IntegrationsRecheck, copy.IntegrationsRecheck, a.Actions.RepairIntegration)
				}
			} else {
				switch integration.repairKind() {
				case repairReconnect:
					if a.Actions.ReconnectIntegration != nil {
						a.repairAction(c, integration.ID, integration.repairLabel(copy), integration.repairTooltip(copy), a.Actions.ReconnectIntegration)
					}
				case repairHomepage:
					if a.Actions.OpenHomepage != nil {
						if rowAction(integration.repairLabel(copy)).Tooltip(integration.repairTooltip(copy)).Clicked() {
							a.Actions.OpenHomepage(integration.Homepage)
						}
					}
				}
			}
			if a.Actions.DiagnoseIntegration != nil {
				if rowAction(copy.IntegrationsCheck).Clicked() {
					a.Actions.DiagnoseIntegration(integration.ID)
				}
			}
			// A generated custom integration's command list came from the
			// tool's own help at connect time; the button re-runs that
			// derivation, so a tool that shipped new subcommands comes back
			// with them without a disconnect.
			if integration.Generated && a.Actions.ReprobeCommands != nil {
				if rowAction(copy.IntegrationsReprobe).Clicked() {
					a.Actions.ReprobeCommands(integration.ID)
				}
			}
			if integration.Generated && a.Actions.EditCustomTool != nil && a.Actions.ToolForEdit != nil {
				if rowAction(copy.EditLabel).Clicked() {
					if tool, ok := a.Actions.ToolForEdit(integration.ID); ok {
						a.beginEdit(integration.ID, tool)
					}
				}
			}
			// Removal is one door for every kind: the row's button asks the
			// kind's own removal bar (`requestRemove` → `removeConfirmation`),
			// and the bar's confirm branches — npm and custom go on to the
			// componentized dialog, a system-owned built-in and a linked local
			// package run the plain removal, exactly as the reference's
			// `confirmRemoval` did (`src/ExtensionsPanel.tsx:1286-1304`).
			// Before this branch the two actions were wired in the shell and
			// some kinds skipped the bar, which handed npm and custom rows one
			// question fewer than the reference asked.
			if (a.Actions.UninstallIntegration != nil || a.Actions.UninstallComponents != nil) && !integration.Orphan {
				if rowAction(integration.removalLabel(copy)).Clicked() {
					a.requestRemove(integration.ID)
				}
			}
			// The componentized dialog, once the bar's confirm opened it,
			// floats over the row.
			a.uninstallComponentsRow(c, copy, integration)
			// The enable/disable toggle rides at the very end of the actions,
			// after the removal door, where the reference's separate
			// `.extension-row__toggle-slot` sat (`src/extensions/ExtensionRow.tsx:437-458`).
			// Its handling is unchanged: turning one *off* asks first because it
			// stops whatever is running; turning one on is no question at all.
			if a.Actions.SetIntegrationEnabled != nil {
				on := enabled
				switch {
				case a.toggleBusy[integration.ID] || a.disableBusy[integration.ID]:
					// The write is in flight: the reference swapped the switch's
					// thumb for a turning LoaderCircle and set `aria-busy` until
					// `runMutation`'s `finally` (`settings-switch--loading`,
					// `src/extensions/ExtensionRow.tsx:443-455`).
					a.busySwitchControl(c, &on, "extensions.toggle."+integration.ID).Label(copy.IntegrationsEnable)
				case busy || integration.Broken:
					// Another operation is in flight, or the row is broken: the
					// reference disabled the switch for both
					// (`disabled={busy || extension.state === "broken"}`).
					disabledSwitch(a.switchControl(c, &on)).Label(copy.IntegrationsEnable)
				default:
					if a.switchControl(c, &on).Label(copy.IntegrationsEnable).Changed() {
						if !on {
							a.requestDisable(integration.ID)
						} else {
							enabled, enableChanged = on, true
						}
					}
				}
			}
		})
		if enableChanged && a.Actions.SetIntegrationEnabled != nil {
			a.startToggle(integration.ID, enabled)
		}
		if a.disableConfirm[integration.ID] {
			a.disableConfirmation(c, copy, integration)
		}
		if a.removeConfirm[integration.ID] {
			a.removeConfirmation(c, copy, integration)
		}
	})
}

// configOwnerIsHost reports whether an integration's configuration is the
// form's to draw.
//
// The reference gated the whole form on `configuration.descriptor.owner ===
// "host"` (`ExtensionsPanel.tsx:2975`) and drew an "Open configuration"
// button for a `tool`-owned one; the schema on which `owner` is *required*
// (`docs/extensions/schemas/provider-description.schema.json`) leaves a
// provider that omits it invalid, so no owner and `host` are the same thing
// here. A `tool` owner is the one case that must not wear the form: its
// editor lives at the tool, and drawing a host form over it would offer to
// write values the tool owns.
func configOwnerIsHost(integration Integration) bool {
	return len(integration.Config) > 0 && !strings.EqualFold(integration.ConfigOwner, "tool")
}

// configOwnerIsTool reports whether an integration's configuration is the
// tool's own to open: the reference's `configuration.descriptor.owner ===
// "tool"` arm (`ExtensionsPanel.tsx:2977`). Such a card carries the "Open
// configuration" button rather than the host form. The gate is the owner
// alone, not the schema's presence — a `tool` owner declares no host schema,
// and its `openCommand` may be empty, in which case the button is drawn
// disabled (the reference's `disabled={!configuration.openPlan}`).
func configOwnerIsTool(integration Integration) bool {
	return strings.EqualFold(integration.ConfigOwner, "tool")
}

// toggleConfig folds or unfolds one integration's configuration form.
//
// Folding a form whose draft has unsaved edits is the reference's own
// two-step: the first press only *asks* (`if (configDirty) {
// setDetailsDiscardArmed(true); return }`, `ExtensionsPanel.tsx:2056-2060`)
// and the panel draws the discard bar under the form; the question is answered
// by keeping the form open or by discarding, which is what finally folds it.
// Opening a folded form is never a question.
func (a *App) toggleConfig(id string) {
	if a.configOpen == nil {
		a.configOpen = map[string]bool{}
	}
	if a.configOpen[id] {
		if integration, ok := a.integrationByID(id); ok && a.configDirty(integration, a.draft(integration)) {
			a.armConfigDiscard(id)
			return
		}
		a.closeConfig(id)
		return
	}
	a.configOpen[id] = true
	a.configError[id] = ""
	a.disarmConfigDiscard(id)
}

// integrationByID finds an integration in the list the panel is drawing.
func (a *App) integrationByID(id string) (Integration, bool) {
	for _, integration := range a.installedIntegrations() {
		if integration.ID == id {
			return integration, true
		}
	}
	return Integration{}, false
}

// armConfigDiscard asks before a dirty form is folded away. The reference armed
// its own question with `useTimedReset(detailsDiscardArmed, …)` and the same
// three-second window the removal and disable bars use
// (`ExtensionsPanel.tsx:866`), so an ask left unanswered withdraws instead of
// staying armed on a later glance.
func (a *App) armConfigDiscard(id string) {
	if a.configDiscard == nil {
		a.configDiscard = map[string]bool{}
	}
	if a.configDiscardAt == nil {
		a.configDiscardAt = map[string]time.Time{}
	}
	a.configDiscard[id] = true
	a.configDiscardAt[id] = a.now()
}

// disarmConfigDiscard answers the question with "keep": the form stays open,
// exactly as the reference's Cancel did (`setDetailsDiscardArmed(false)`).
func (a *App) disarmConfigDiscard(id string) {
	if a.configDiscard != nil {
		delete(a.configDiscard, id)
	}
	if a.configDiscardAt != nil {
		delete(a.configDiscardAt, id)
	}
}

// closeConfig folds one integration's form away and drops the draft: a form
// opened again reads the stored values, which is the reference's own effect on
// the selected integration changing (`ExtensionsPanel.tsx:1094-1096`).
func (a *App) closeConfig(id string) {
	a.disarmConfigDiscard(id)
	if a.configOpen != nil {
		delete(a.configOpen, id)
	}
	if a.configDrafts != nil {
		delete(a.configDrafts, id)
	}
	if a.configError != nil {
		delete(a.configError, id)
	}
	if a.configFieldErrors != nil {
		prefix := id + "\x00"
		for key := range a.configFieldErrors {
			if strings.HasPrefix(key, prefix) {
				delete(a.configFieldErrors, key)
			}
		}
	}
}

// discardConfigChanges is the discard bar's destructive answer: the edits are
// dropped and the form closes (the reference's `discardDetailsChanges`,
// `ExtensionsPanel.tsx:2135-2138`).
func (a *App) discardConfigChanges(id string) { a.closeConfig(id) }

// configOpenToolRow draws a tool-owned configuration's one outlet: the
// reference's `settings.extensions.configuration` block title and the
// "Open configuration" action that runs the provider's own `openCommand`
// (`ExtensionsPanel.tsx:2971-2986`). The action is the panel's ordinary
// action pill, carrying the reference's ExternalLink mark then the word, and
// it is disabled when the provider declared no `openCommand` — the
// reference's `disabled={!configuration.openPlan}`.
func (a *App) configOpenToolRow(c *ui.Context, copy i18n.Settings, integration Integration) {
	a.row(c, copy.ConfigSection, "", func() {
		openable := len(integration.ConfigOpenCommand) > 0 && a.Actions.OpenConfiguration != nil
		button := a.extensionGlyphActionButton(c, "link", copy.ConfigOpenConfiguration,
			a.cardTheme(c).Space(7.5), a.cardTheme(c).Space(3.75), a.tokens(c).Body)
		if !openable {
			button = button.Disabled(true).Opacity(0.4).Cursor(ui.CursorDefault)
		}
		if button.Clicked() && openable {
			a.Actions.OpenConfiguration(integration.ID, append([]string{}, integration.ConfigOpenCommand...))
		}
	})
}

// configDiscardBar draws the reference's `.extension-discard-bar`
// (`extensions.css:2455-2482`, `ExtensionsPanel.tsx:3029-3035`): the error
// notice with the question, then Cancel and the destructive Discard. It is
// drawn under the form, and only while the question is armed *and* the draft is
// still dirty — the reference's `detailsDiscardArmed && configDirty`.
func (a *App) configDiscardBar(c *ui.Context, copy i18n.Settings, integration Integration) {
	band := a.noticeBand(c, true, copy.ConfigDiscardConfirm, func(cancel *ui.Element) {
		*cancel = a.barButton(c, copy.ConfigCancel, false, false, nil, func() {
			a.disarmConfigDiscard(integration.ID)
		})
		a.barButton(c, copy.ConfigDiscard, true, false, nil, func() {
			a.discardConfigChanges(integration.ID)
		})
	})
	// The reference's own effect puts focus on Cancel, so the first Enter or
	// Escape answers "keep" without the pointer moving.
	band.Focus()
}

// toggleAudit folds or unfolds one integration's permission audit.
func (a *App) toggleAudit(id string) {
	if a.auditOpen == nil {
		a.auditOpen = map[string]bool{}
	}
	a.auditOpen[id] = !a.auditOpen[id]
}

// permissionAudit draws the per-permission explanation under a row: what each
// permission allows, and whether the host itself decides it. The list is folded
// away until the user asks for it, so a row stays a row.
func (a *App) permissionAudit(c *ui.Context, copy i18n.Settings, integration Integration) {
	t := a.cardTheme(c)
	language := a.Store.Snapshot().Language
	if a.actionButton(c, copy.IntegrationsPermissions).Clicked() {
		if a.auditOpen == nil {
			a.auditOpen = map[string]bool{}
		}
		a.auditOpen[integration.ID] = !a.auditOpen[integration.ID]
	}
	if !a.auditOpen[integration.ID] {
		return
	}
	// Two tiers, the host's own decision first: what floter refuses until it
	// is approved, then what the tool merely declares. The split is the
	// message — a user must be able to tell a check from an FYI.
	var enforced, disclosure []string
	for _, permission := range integration.Permissions {
		if integration.Enforced[permission] {
			enforced = append(enforced, permission)
			continue
		}
		disclosure = append(disclosure, permission)
	}
	// The audit is a `below` block of the Permissions row: it is drawn on the
	// card's content column, so it carries the row's own inset to put its tier
	// labels on the same column as the row labels above them. Without it the
	// labels sit flush against the card's edge, one whole inset left of the
	// labels they explain (the reference's `.extension-permission-list` pads
	// its own list, `extensions.css:1497-1503`).
	inset := a.rowInset(t)
	ui.Column(c).FillWidth().Gap(t.Space(3)).Padding(0, inset, t.Space(1), inset).Children(func() {
		a.permissionTier(c, copy.PermissionsEnforcedLabel, copy.PermissionsEnforcedHint, true, language, enforced)
		a.permissionTier(c, copy.PermissionsDisclosureLabel, copy.PermissionsDisclosureHint, false, language, disclosure)
	})
}

// commandList draws an integration's commands as the reference's
// `.extension-command-list` (`ExtensionsPanel.tsx:2777-2871`,
// `extensions.css:2206-2281`): one plate per command, each holding the
// identity line (name, description, the words that summon it and the launcher
// switch) above the alias editor at the plate's full width. The port had one
// flat settings row per command with the alias editor as its grey second line,
// which is neither the reference's two-line plate nor its bordered list.
func (a *App) commandList(c *ui.Context, copy i18n.Settings, extensionID string, commands []Command) {
	t := a.cardTheme(c)
	ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
		for _, command := range commands {
			command := command
			a.commandPlate(c, copy, extensionID, command)
		}
	})
}

// declaredCommands keeps the commands the provider's descriptor declared and
// drops the ones the host synthesized (a tool-owned configuration's editor).
// The reference's command list is the provider registry, which never holds the
// synthesized command; see commandList's caller.
func declaredCommands(commands []Command) []Command {
	out := make([]Command, 0, len(commands))
	for _, command := range commands {
		if command.Declared {
			out = append(out, command)
		}
	}
	return out
}

// commandPlate is one command's card in the reference's list
// (`.extension-command-list > .extension-command-list__row`): a
// `--hairline`-edged, `--glass-control`-filled plate with the `--ext-radius`
// corner and a 10px/12px padding pair. A command whose launcher switch is off
// is dimmed as a whole (opacity 0.62) so its state reads before the switch is
// found — a de-emphasis, never a disable (the alias editor and the switch stay
// live).
func (a *App) commandPlate(c *ui.Context, copy i18n.Settings, extensionID string, command Command) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	plate := ui.Column(c).FillWidth().Gap(t.Space(2)).
		Padding(t.Space(2.5), t.Space(3)).
		Radius(tokens.RadiusSM).Background(tokens.Control)
	// `.extension-command-list__row` is a hairline-edged plate; the edge is a
	// real 1px border here because the reference's own box is `border: 1px` and
	// the plate's box was never a pinned metric.
	plate.Border(1, tokens.Hairline)
	if !command.Enabled {
		plate.Opacity(0.62)
	}
	plate.Children(func() {
		a.commandIdentity(c, copy, extensionID, command)
		a.commandAliasEditor(c, copy, extensionID, command)
	})
}

// commandIdentity is the reference's `.extension-command-list__head`
// (`ExtensionsPanel.tsx:2815-2843`): the command's name, its description and
// the words that summon it on the left, the launcher switch on the right. The
// name is a `<code>` the reference set in the caption register at
// `--text-secondary` (`.extension-command-list code`,
// `extensions.css:2310-2316`); the description and the trigger line are the
// muted caption the list's `span` rule gave (`:2318-2322`).
func (a *App) commandIdentity(c *ui.Context, copy i18n.Settings, extensionID string, command Command) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	on := command.Enabled
	toggled := false
	name := command.Name
	if name == "" {
		name = command.ID
	}
	h := ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(t.Space(3))
	h.Children(func() {
		identity := ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(0.5))
		identity.Children(func() {
			ui.Text(c, name).FontSize(tokens.Caption).
				TextColor(tokens.TextSecondary).SingleLine().Ellipsis("\u2026")
			description := command.Description
			if description == "" {
				description = copy.IntegrationsNoDescription
			}
			ui.Text(c, description).FontSize(tokens.Caption).
				TextColor(t.TextMuted).SingleLine().Ellipsis("\u2026")
			// The summon words are the reference's own "Summon with: …" line,
			// printed only when the command is one the provider registry knows —
			// the same vocabulary the launcher's trigger parser reads. A command
			// the host synthesized (the tool-owned configuration editor) is not in
			// that registry, so it prints no line (`ExtensionsPanel.tsx:2824-2829`).
			if command.Declared {
				if words := commandTriggerWords(command); len(words) > 0 {
					ui.Text(c, copy.IntegrationsCommandsTriggers(strings.Join(words, ", "))).
						FontSize(tokens.Caption).TextColor(t.TextMuted).Opacity(0.85).
						SingleLine().Ellipsis("\u2026")
				}
			}
		})
		// The switch rides the identity line's own top (the reference's
		// `align-items: flex-start` + `margin-top: 2px`). Its own click is the
		// toggle — never the whole line's, or the label would toggle too and
		// the two would cancel.
		if a.switchControl(c, &on).Label(copy.IntegrationsCommandEnable(name)).
			Margin(t.Space(0.5), 0, 0, 0).Changed() {
			toggled = true
		}
	})
	if toggled && a.Actions.SetCommandEnabled != nil {
		a.Actions.SetCommandEnabled(extensionID, command.ID, on)
	}
}

// commandAliasEditor is the reference's `.extension-command-alias`
// (`ExtensionsPanel.tsx:2852-2868`): the caption label over the input, on the
// plate's full width. The input is the panel's own inline box (see
// `aliasField`), and the taken note is the muted caption line under it. It is
// a text field, so it rides the reference's 180ms debounce: one write per
// pause, not one per keystroke (see debounce.go).
func (a *App) commandAliasEditor(c *ui.Context, copy i18n.Settings, extensionID string, command Command) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	ui.Column(c).FillWidth().Gap(t.Space(0.5)).Children(func() {
		ui.Text(c, copy.IntegrationsCommandAlias).FontSize(tokens.Caption).TextColor(t.TextMuted)
		alias := command.Alias
		name := command.Name
		if name == "" {
			name = command.ID
		}
		if a.aliasField(c, &alias).Placeholder(copy.IntegrationsCommandAliasPlaceholder).
			Label(copy.IntegrationsCommandAliasFor(name)).FillWidth().Changed() &&
			a.Actions.SetCommandAlias != nil {
			a.debounceAlias(extensionID, command.ID, alias)
		}
		if command.Taken {
			ui.Text(c, copy.IntegrationsCommandAliasTaken).
				FontSize(tokens.Caption).TextColor(t.TextMuted)
		}
	})
}

// commandTriggerWords is a command's summon vocabulary: its id, its name and
// its declared aliases, each reduced to its first whitespace-separated token —
// the reference's `pluginCommandTriggers` (`plugins/external.ts:86-97`), which
// the launcher's trigger parser reads too, so the words the plate prints are
// exactly the words that enter the mode. A word that does not begin with an
// ASCII letter or digit is dropped: the trigger split the launcher applies
// needs a typeable first character, and the reference's own de-duplication
// would otherwise print a word nothing can summon.
func commandTriggerWords(command Command) []string {
	seen := map[string]bool{}
	words := make([]string, 0, 3)
	for _, source := range append([]string{command.ID, command.Name}, command.Aliases...) {
		word := firstTriggerWord(strings.TrimSpace(source))
		key := strings.ToLower(word)
		if word == "" || seen[key] {
			continue
		}
		seen[key] = true
		words = append(words, word)
	}
	return words
}

// firstTriggerWord is the first whitespace-separated token of a source word:
// the reference reduces every trigger source to it because the mode is entered
// by one word followed by a space (`plugins/external.ts:79-84`).
func firstTriggerWord(source string) string {
	if fields := strings.Fields(source); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// permissionLine names the package's permissions, marking the ones the host
// itself decides.
func (a *App) permissionLine(integration Integration, copy i18n.Settings) string {
	language := a.Store.Snapshot().Language
	parts := make([]string, 0, len(integration.Permissions))
	for _, permission := range integration.Permissions {
		label := i18n.PermissionLabel(language, permission)
		if integration.Enforced[permission] {
			label += " (" + copy.PermissionsEnforced + ")"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " · ")
}

// state is the integration's one-word state.
func (i Integration) state(copy i18n.Settings) string {
	switch {
	case i.Orphan:
		return copy.IntegrationsOrphan
	case i.Broken:
		return copy.IntegrationsBroken
	case i.Enabled:
		return copy.IntegrationsRunning
	default:
		return copy.IntegrationsOff
	}
}

// identity is the publisher and version line.
func (i Integration) identity() string {
	parts := []string{}
	if i.Publisher != "" {
		parts = append(parts, i.Publisher)
	}
	if i.Version != "" {
		parts = append(parts, i.Version)
	}
	if i.ToolVersion != "" && i.ToolVersion != i.Version {
		parts = append(parts, "tool "+i.ToolVersion)
	}
	if i.ID != "" {
		parts = append(parts, i.ID)
	}
	return strings.Join(parts, " · ")
}

// integrationKind is the integration's *type*, the second slot the reference's
// `.extension-row__meta` filled (`src/extensions/ExtensionRow.tsx:220-222`: the
// package identity, then `t(integrationKindKey(extension))`). It is a type, not
// the removal verb: the same card reads "System tool" and offers "Disconnect".
//
// The reference's `integrationKindKey` (`:77-94`) told seven kinds apart, but
// the `recommended` / `manifest` / `tool` branches all require `!connected`, and
// this build never draws a `DetectedTool` or `RecommendedTool` as an
// `Integration` — those live in their own sections with their own renderers. So
// for the connected cards this method is the four-way pick, walked in the
// reference's own order (`generatedCustom` first, so a custom integration that
// also came from npm is still custom).
func (i Integration) integrationKind(copy i18n.Settings) string {
	switch {
	case i.Generated:
		return copy.IntegrationsKindCustom
	case i.DistributionSource == "npm":
		return copy.IntegrationsKindNPM
	case i.DistributionSource == "built-in" && i.RuntimeOwnership == "system":
		return copy.IntegrationsKindSystem
	default:
		return copy.IntegrationsKindPackage
	}
}

// runtimeSourceLabel is where the runtime lives, the meta line's third slot
// (`src/extensions/ExtensionRow.tsx:233`). The word follows the reference's own
// `runtimeSource` projection; an unknown source (a fixture with no projection)
// prints nothing rather than inventing a home for the runtime.
func (i Integration) runtimeSourceLabel(copy i18n.Settings) string {
	switch i.RuntimeSource {
	case "managed":
		return copy.IntegrationsRuntimeManaged
	case "system":
		return copy.IntegrationsRuntimeSystem
	case "bundled":
		return copy.IntegrationsRuntimeBundled
	default:
		return ""
	}
}

// identityLine is the card's own second row: the package identity the reference
// printed, then the type word (`integrationKind`). Both are skipped when empty
// so an orphan's bare id row does not trail a separator, and the two are the
// reference's own two meta slots in its own order.
func (i Integration) identityLine(copy i18n.Settings) string {
	parts := []string{}
	if id := i.identity(); id != "" {
		parts = append(parts, id)
	}
	if kind := i.integrationKind(copy); kind != "" {
		parts = append(parts, kind)
	}
	return strings.Join(parts, " · ")
}

// The removal kinds the reference told apart (`src/extensions/ExtensionRow.tsx:109-114`).
// Exported so the shell can pick the kind's own removal notification after the
// settings page has passed the same fact through the action.
const (
	RemovalCustom  = "custom"
	RemovalNPM     = "npm"
	RemovalSystem  = "system"
	RemovalPackage = "package"
)

// removalKind is what this integration *is*, for the purpose of the removal
// verb: a generated custom integration is deleted, an npm package is
// uninstalled, a built-in whose runtime the system owns is disconnected (the
// package is not floter's to delete), and anything else is a linked local
// package that is only removed from the list. Ported line for line from the
// reference's `removalKind`; the order matters — a generated integration that
// also came from npm is still a custom one, exactly as the reference read it
// (`generatedCustom` first).
func (i Integration) removalKind() string {
	switch {
	case i.Generated:
		return RemovalCustom
	case i.DistributionSource == "npm":
		return RemovalNPM
	case i.DistributionSource == "built-in" && i.RuntimeOwnership == "system":
		return RemovalSystem
	default:
		return RemovalPackage
	}
}

// componentizedRemoval reports whether this kind's removal is the
// componentized dialog. The reference offered it to npm and custom only
// (`src/ExtensionsPanel.tsx:1290-1304`): those are the kinds whose files
// floter owns and whose host-config / tool-data / artifact categories are the
// user's to pick. A system-owned built-in and a linked local package have no
// data directory of floter's to delete, so they take the plain removal (the
// removal bar → `extensions_uninstall {removeData:false}`) and the reference
// never drew the three data checkboxes under them.
func (i Integration) componentizedRemoval() bool {
	switch i.removalKind() {
	case RemovalNPM, RemovalCustom:
		return true
	default:
		return false
	}
}

// removalLabel is the removal verb for this kind, the same four-way pick the
// reference made when it chose its menu item's text and icon
// (`src/extensions/ExtensionRow.tsx:409-418`): the custom kind reuses the
// `deleteCustom` string the confirmation already has, npm the plain
// uninstall.
func (i Integration) removalLabel(copy i18n.Settings) string {
	switch i.removalKind() {
	case RemovalCustom:
		return copy.RemoveDelete
	case RemovalNPM:
		return copy.IntegrationsUninstall
	case RemovalSystem:
		return copy.IntegrationsDisconnect
	default:
		return copy.IntegrationsRemovePackage
	}
}

// removalWords is the confirmation bar's question for this kind. The reference
// did not fall back to one generic sentence: `removalTextKey`
// (`src/ExtensionsPanel.tsx:2140-2151`) built one key per kind and
// `RemovalConfirmation.tsx:41` read the title through it, so the bar asks the
// same question the button promised — "Disconnect X?" under a Disconnect
// button, not "Uninstall X?".
//
// The second return is the `Description` half of that key family. The
// reference's bar never drew it — `:41` renders the Title only, and the
// `*Description` strings have no other reader in the original tree — so the
// band's whole question is `removalSentence`; the detail stays here as the
// per-kind mapping it is.
func (i Integration) removalWords(copy i18n.Settings) (title, detail string) {
	switch i.removalKind() {
	case RemovalCustom:
		return copy.RemoveCustomTitle(i.Name), copy.RemoveCustomDetail + " " + copy.RemoveCustomWarning
	case RemovalNPM:
		return copy.RemoveTitle(i.Name), copy.RemoveDetail
	case RemovalSystem:
		return copy.IntegrationsDisconnectTitle(i.Name), copy.IntegrationsDisconnectDetail
	default:
		return copy.IntegrationsRemovePackageTitle(i.Name), copy.IntegrationsRemovePackageDetail
	}
}

// removalSentence is the band's whole question, assembled the way the
// reference assembled it (`RemovalConfirmation.tsx:41`): the kind's title, and —
// for a generated custom integration only — the warning that Floter cannot undo
// the deletion. It is one line, not a title over a body: the reference's
// `*Description` strings were never drawn in the bar.
func (i Integration) removalSentence(copy i18n.Settings) string {
	title, _ := i.removalWords(copy)
	if i.Generated {
		return title + " " + copy.RemoveCustomWarning
	}
	return title
}

// The repair a connected row offers when its runtime no longer resolves. The
// reference drew one button for this state and picked its arm from the same
// three facts (`src/extensions/ExtensionRow.tsx:345-370`).
type repairKind int

const (
	// repairNone: the runtime resolves, or the row is broken for a reason a
	// check already reported — there is nothing to repair here.
	repairNone repairKind = iota
	// repairReconnect: the program belongs to the machine, so the search path
	// is asked again.
	repairReconnect
	// repairHomepage: floter does not own this runtime and has nowhere of its
	// own to look, so the row sends the user to the project's page.
	repairHomepage
)

// repairKind is which repair this row offers, or none. The condition is the
// reference's own: a *connected*, *not broken* row whose runtime does not
// resolve offered the button when its runtime belonged to the system, when it
// had a project page, or both. Managed and bundled rows with no page render
// nothing at all (the reference's own note on that arm), and a `broken` row
// shows the re-check instead (`src/extensions/ExtensionRow.tsx:313-330`).
func (i Integration) repairKind() repairKind {
	if i.Broken || i.RuntimeAvailable {
		return repairNone
	}
	if i.RuntimeOwnership == extensions.RuntimeOwnershipSystem {
		return repairReconnect
	}
	if i.Homepage != "" {
		return repairHomepage
	}
	return repairNone
}

// repairLabel is the button's word: the reference named the reconnect arm
// "Reconnect" whenever the runtime was the system's to look for (with or
// without a page, because reconnecting is what it does), and "Install tool
// first" for the arm that can only hand over a page.
func (i Integration) repairLabel(copy i18n.Settings) string {
	if i.repairKind() == repairHomepage {
		return copy.IntegrationsInstallTool
	}
	return copy.IntegrationsReconnect
}

// repairTooltip is the full sentence behind the button. The reference's
// `reconnectUnboundHint` says what to expect when the search path found
// nothing *and* there is no page either — the button re-runs the search, and
// the sentence says so rather than promising a tool it knows about.
func (i Integration) repairTooltip(copy i18n.Settings) string {
	if i.repairKind() == repairReconnect && i.Homepage == "" {
		return copy.IntegrationsReconnectUnboundHint
	}
	return i.repairLabel(copy)
}

// repairAction draws the row's re-check / reconnect entry and holds it in the
// in-flight face from the press until the shell calls back, so a press that
// has not landed is visible rather than assumed. The reference drew exactly
// this pair (`src/extensions/ExtensionRow.tsx:313-330,345-372`): one button
// whose icon became a turning LoaderCircle and whose `disabled={busy}` face
// stayed until `runMutation`'s `finally`.
func (a *App) repairAction(c *ui.Context, id, label, tooltip string, run func(string, func(error))) {
	if a.repairBusy[id] {
		a.busyActionButton(c, label, "extensions.repair."+id).Tooltip(tooltip)
		return
	}
	button := a.actionButton(c, label).Tooltip(tooltip)
	if a.rowBusyExceptRepair(id) {
		// Another operation on the row is in flight: the reference disabled
		// *every* control in the cluster for the duration, not only the one
		// that was pressed (`disabled={busy}`, `src/extensions/ExtensionRow.tsx:301-419`).
		button = button.Disabled(true)
	}
	if button.Clicked() {
		a.startRepair(id, run)
	}
}

// rowBusy reports whether any operation on an integration is in flight, so the
// row's whole action cluster and its switch can lock together. The reference
// kept one `operation` for the panel and disabled the row's controls from it
// (`const busy = Boolean(operation)` / `disabled={busy}`,
// `src/extensions/ExtensionRow.tsx:131,301-419,447`); here the in-flight marks
// live on the row's own maps, so the union is what the whole-row lock reads.
func (a *App) rowBusy(id string) bool {
	return a.repairBusy[id] || a.disableBusy[id] || a.removeBusy[id] ||
		a.uninstallBusy[id] || a.toggleBusy[id]
}

// rowBusyExceptRepair is rowBusy without the repair mark, for the re-check /
// reconnect entry itself: when its own work is in flight the button wears the
// busy pill instead of a plain disabled face, so it asks only about the row's
// *other* operations.
func (a *App) rowBusyExceptRepair(id string) bool {
	return a.disableBusy[id] || a.removeBusy[id] || a.uninstallBusy[id] ||
		a.toggleBusy[id]
}

// startRepair marks a row's re-check or reconnect as in flight and hands the
// shell the id to work on with the callback that settles the row.
func (a *App) startRepair(id string, run func(string, func(error))) {
	if a.repairBusy == nil {
		a.repairBusy = map[string]bool{}
	}
	a.repairBusy[id] = true
	run(id, func(_ error) { a.finishRepair(id) })
	if a.Actions.Invalidate != nil {
		a.Actions.Invalidate()
	}
}

// finishRepair settles a row after the shell reports a re-check or reconnect
// back on the main thread. The reference's `runMutation` cleared `busy` in
// `finally` whether the work succeeded or failed, and a failure reached the
// user through the app's own notice rather than through the row
// (`src/ExtensionsPanel.tsx:1190-1210,1255-1267`).
func (a *App) finishRepair(id string) {
	if a.repairBusy != nil {
		delete(a.repairBusy, id)
	}
	if a.Actions.Invalidate != nil {
		a.Actions.Invalidate()
	}
}

// startToggle marks an enable write as in flight and hands the shell the id to
// work on with the callback that settles the switch. The reference's
// `runMutation` set `operation` to the enable kind and cleared it in `finally`
// (`src/ExtensionsPanel.tsx:1190-1210`); a failure reaches the user through
// the shell's own notice path, so the switch only has to come back.
func (a *App) startToggle(id string, enabled bool) {
	if a.toggleBusy == nil {
		a.toggleBusy = map[string]bool{}
	}
	a.toggleBusy[id] = true
	a.Actions.SetIntegrationEnabled(id, enabled, func(_ error) { a.finishToggle(id) })
	if a.Actions.Invalidate != nil {
		a.Actions.Invalidate()
	}
}

// finishToggle settles an enable write after the shell reports back on the main
// thread: success and failure both return the switch to its resting face.
func (a *App) finishToggle(id string) {
	if a.toggleBusy != nil {
		delete(a.toggleBusy, id)
	}
	if a.Actions.Invalidate != nil {
		a.Actions.Invalidate()
	}
}

func (a *App) installedIntegrations() []Integration {
	if a.Integrations == nil {
		return nil
	}
	return a.Integrations()
}

// shortcuts lists the shortcuts the app answers, with the key each one
// currently holds and a recorder for the one the app can change.
func (a *App) shortcuts(c *ui.Context, copy i18n.Settings) {
	a.pageColumn(c, func() {
		// A refused recording is the first thing the user reads, above the
		// page's own groups (the old page's static save alert).
		a.shortcutRejectedAlert(c, copy)
		// The group title row carries the group's own action, as the reference's
		// pane did: "Restore defaults" beside the title, in the accent-text
		// idiom, and it really resets.
		a.sectionWithAction(c, copy.ShortcutsToggle, copy.ShortcutsHint,
			copy.ShortcutsReset, func() {
				if a.Actions.ResetShortcuts != nil {
					a.Actions.ResetShortcuts()
				}
			}, func() {
				a.card(c, func() {
					a.shortcutRow(c, copy)
				})
			})
		a.section(c, copy.ShortcutsApp, copy.ShortcutsAppHint, func() {
			a.card(c, func() {
				for _, action := range settings.ShortcutActions {
					if action == settings.ShortcutToggleWindow {
						continue // the summon key has its own card above
					}
					a.shortcutMapRow(c, copy, action)
				}
			})
		})
		a.customSection(c, copy)
	})
}

// shortcutMapRow is one of the app's own keys: its name, its binding, and a
// recorder that rebinds that action. A recording the page refused carries the
// reference's warning under the name.
func (a *App) shortcutMapRow(c *ui.Context, copy i18n.Settings, action string) {
	a.rowNote(c, copy.ShortcutNames[action], "", a.shortcutWarning(action, copy), func() {
		a.shortcutControl(c, copy, action, a.binding(action))
	})
}

// shortcutWarning is the reference's `settings.shortcut.rejected`, drawn in the
// row whose recording was refused (the page draws the same sentence at its top,
// as `src/settings/ShortcutsPage.tsx:382,404-406` did).
func (a *App) shortcutWarning(action string, copy i18n.Settings) string {
	if a.rejectedShortcut == action {
		return copy.ShortcutRejected
	}
	return ""
}

// binding is one action's accelerator, from the settings.
func (a *App) binding(action string) string {
	return settings.Shortcut(a.Store.Snapshot(), action)
}

// shortcutRow is one shortcut: its name, its keys, and the recorder.
func (a *App) shortcutRow(c *ui.Context, copy i18n.Settings) {
	a.rowNote(c, copy.ShortcutsToggle, "", a.shortcutWarning(settings.ShortcutToggleWindow, copy), func() {
		a.shortcutControl(c, copy, "", a.summonShortcut())
	})
}

// shortcutControl is a row's trailing slot: the recorder while the action is
// being recorded (capturing the next chord, Escape to give up), the current
// keys and the Record button otherwise.
func (a *App) shortcutControl(c *ui.Context, copy i18n.Settings, action, value string) {
	t := c.Theme()
	recording := a.recording && (action == "" || a.ShortcutID == action)
	if recording {
		// The armed recorder: the same control face as the resting one, plus
		// the accent ring and the accent prompt — the one state in this page
		// where an accent ring is allowed, because the user *is* aiming a key
		// at it (§6.1 第 10 条).
		//
		// P8-b 重定基线：`RaisedQuiet` 不透面 → `Control`（与静息同一档面），
		// 只多一圈 accent 环；一个不透明的抬起面会读成第二个材质（§1）。
		tokens := a.tokens(c)
		capture := ui.Box(c).Focusable().Padding(t.Space(1), t.Space(2)).Radius(tokens.RadiusSM).
			Background(tokens.Control).Border(1, t.Accent).Label(copy.ShortcutRecording)
		capture.Children(func() {
			ui.Text(c, copy.ShortcutRecording).FontSize(t.FontSize).TextColor(t.Accent)
		})
		capture.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputKeyDown {
				return false
			}
			// Escape gives up the recording, but only a bare Escape: a modified
			// Escape is a chord the user may want to record (the old build's
			// `if (event.key === "Escape" && bare) onCancel()`).
			if ev.Key == ui.KeyEscape && ev.Mods == 0 {
				a.recording = false
				return true
			}
			accelerator, ok := shortcuts.FromKey(ev.Mods, ev.Key)
			if !ok {
				return true // a key without a modifier is not a shortcut
			}
			a.recording = false
			id := action
			if id == "" {
				id = settings.ShortcutToggleWindow
			}
			a.recordShortcut(id, accelerator)
			return true
		})
		capture.Focus()
		return
	}
	// The reference's `.shortcut-recorder` (`settings.css:1198-1224`): a
	// resting control pane (82u × 27u, `--glass-control` fill, 1px
	// `--glass-control-edge` border, `--radius-sm` corner, `--elev-0` lens)
	// showing the current chord in the monospace stack
	// (`.shortcut-recorder__keys`, `:1238-1242`). The old port drew bare
	// text and a separate Record button, so the row's trailing slot had no
	// bordered control the reference put there.
	a.shortcutRecorder(c, copy.ShortcutRecord, shortcuts.Display(value), func() {
		a.recording, a.ShortcutID = true, action
		// Starting a recording clears the last refusal, as the reference's own
		// toggle did.
		a.rejectedShortcut = ""
	})
}

// shortcutRecorder is the recorder's resting control pane: an 82u-wide,
// 27u-tall control on the resting `Control` face under the `ControlEdge`
// hairline, in the secondary ink. Hover lifts the fill to `ControlHover` and
// the ink to `TextStrong`; press drops it to `ControlPress`. The recorded
// chord is set in the reference's monospace stack (`.shortcut-recorder__keys`),
// the one face where the reference itself breaks the app's proportional font.
// The button carries the reference's `aria-label`
// (`settings.shortcut.record`) and its focus-visible ring.
//
// P8-b 重定基线：旧 hover 把边抬到 `t.Accent` → 合约 §6.1 第 10 条「静息用
// `Control`，只有录制中才用 accent 环」。accent 在本页只有两处法定用途
// （侧栏前导图标、主 CTA，§5），一个悬停的静息控件不是第三处：悬停只抬
// **面**与**字**，不抬色相。
func (a *App) shortcutRecorder(c *ui.Context, aria, label string, clicked func()) {
	t := c.Theme()
	tokens := a.tokens(c)
	button := ui.Row(c).Focusable().Label(aria).MinWidth(t.Space(20.5)).MinHeight(t.Space(6.75)).
		Padding(0, t.Space(2.5)).Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
		Background(tokens.Control).Border(1, tokens.ControlEdge).FocusRing(true)
	ink := tokens.TextSecondary
	if button.Hovered() {
		button.Background(tokens.ControlHover)
		ink = tokens.TextStrong
	}
	if button.Pressed() {
		button.Background(tokens.ControlPress)
	}
	button.Children(func() {
		ui.Text(c, label).Font(deepLinkValueFont).FontSize(t.FontSize).TextColor(ink)
	})
	if button.Clicked() {
		clicked()
	}
}

// recordShortcut applies a chord the recorder captured. The map's own rule
// decides: another action already answering for the chord is refused and the
// page says so instead of writing it, re-recording an action's own chord is no
// change at all, and only a new binding reaches the shell (the old build's
// `capture`, `src/hooks/useShortcutCapture.ts:112-136`).
func (a *App) recordShortcut(action, accelerator string) {
	switch settings.ShortcutRecordingOf(a.Store.Snapshot(), action, accelerator) {
	case settings.ShortcutRecorded:
		a.rejectedShortcut = ""
		if a.Actions.SetShortcut != nil {
			a.Actions.SetShortcut(action, accelerator)
		}
	case settings.ShortcutRefused:
		a.rejectedShortcut = action
	default:
		a.rejectedShortcut = ""
	}
}

// updateRow is the About page's update row: the version line the old
// AboutPage drew (`Current version: v…`), the state that follows it, and the
// one control the state offers. The state machine is the old build's:
//
//	progress  → the byte bar and its readout
//	downloading → a disabled "Installing…" button
//	failed    → a "Retry" button
//	available → a "Download" button
//	otherwise → no control
//
// The build's no-feed state has no old-build counterpart; it keeps the row and
// says so in the sublabel rather than pretending an update was checked.
func (a *App) updateRow(c *ui.Context, copy i18n.Settings) {
	state := a.Update
	sublabel := copy.UpdateUpToDate
	switch {
	case state.Disabled:
		sublabel = copy.UpdateDisabled
	case state.Failed:
		sublabel = copy.UpdateFailedRetry
	case state.Available:
		sublabel = copy.UpdateLatestVersion + ": v" + state.Version
	}
	a.row(c, copy.UpdateCurrentVersion+": v"+a.About.Version, sublabel, func() {
		switch {
		case state.Progress != nil:
			a.updateProgress(c, state.Progress)
		case state.Downloading:
			a.updateButton(c, copy.UpdateInstalling, true, nil)
		case state.Failed:
			a.updateButton(c, copy.UpdateRetry, false, a.Actions.InstallUpdate)
		case state.Available:
			a.updateButton(c, copy.UpdateDownload, false, a.Actions.InstallUpdate)
		}
	})
}

// updateButton is the About page's one accent control: the accent fill with
// `--accent-contrast` ink, the lit top rim (`inset 0 1px 0`), the emphasis step
// at 600, and no keyline at all. Hover fades the whole button to 0.9; a disabled
// face is 0.6 with the pointer refused. It is the one thing on the page the user
// presses, so it is the page's whole accent budget (§5 第 2 条).
//
// P8-b 重定基线：`Radius(999)` 胶囊 → `RadiusSM`(8) 圆角矩形。accent 填充保留；
// 合约的 CTA 是一个圆角按钮，不是药丸（§6.1 第 9 条），半径从半径梯上取字面值，
// 不用 `Space()`——一个角不随 `--ui-scale` 走（§2）。
func (a *App) updateButton(c *ui.Context, label string, disabled bool, run func()) {
	tokens := a.tokens(c)
	t := c.Theme()
	button := ui.ButtonBase(c).Label(label).
		MinHeight(t.Space(7)).Padding(t.Space(1.5), t.Space(3.5)).Radius(tokens.RadiusSM).
		Background(t.Accent)
	// `box-shadow: inset 0 1px 0 var(--glass-raised-rim)`: the lit top rim is
	// what makes the accent pill read as a raised control.
	button.InsetShadow(0, 1, 0, 0, tokens.LensRim)
	switch {
	case disabled:
		button = button.Opacity(0.6).Cursor(ui.CursorDefault).Disabled(true)
	case button.Hovered():
		button = button.Opacity(0.9)
	}
	button.Children(func() {
		ui.Text(c, label).FontSize(tokens.Emphasis).FontWeight(600).TextColor(t.AccentText)
	})
	if button.Clicked() && !disabled && run != nil {
		run()
	}
}

// UpdateFraction is the share of an update download that is done, between 0
// and 1, and 0 while the feed has not named a total. The About page's own
// track and the system's progress surface (the taskbar button, the Dock icon,
// the launcher entry) read it from here, so the two cannot drift: one download
// has one fraction.
func UpdateFraction(p *UpdateProgress) float64 {
	if p == nil || p.Total <= 0 {
		return 0
	}
	return min(max(float64(p.Downloaded)/float64(p.Total), 0), 1)
}

// updateProgress is the old `.update-banner__progress`: a track with the
// accent bar over it and the tabular readout under it (`42% · 1.5 MB / 3.6 MB`,
// or just the downloaded count when the feed named no total).
func (a *App) updateProgress(c *ui.Context, progress *UpdateProgress) {
	t := c.Theme()
	tokens := a.tokens(c)
	track := t.Space(45)
	fraction := float32(UpdateFraction(progress))
	ui.Column(c).MinWidth(track).Shrink(0).Gap(t.Space(1.25)).Children(func() {
		// `.update-banner__progress-track` is a 4u groove with the ladder's
		// small corner and the track edge as an *inset* keyline
		// (`settings.css:1577-1583`), so the bar rides inside a 1px recess
		// rather than inside a border that grows the box.
		bar := ui.Row(c).Width(track).Height(t.Space(1)).Radius(tokens.RadiusXS).
			Background(tokens.Track)
		bar.InsetShadow(0, 0, 0, 1, tokens.TrackEdge)
		bar.Children(func() {
			if fraction > 0 {
				ui.Box(c).Width(track * fraction).Height(t.Space(1)).
					Radius(tokens.RadiusXS).Background(t.Accent)
			}
		})
		// `.update-banner__progress-label` is the caption step in
		// `--text-secondary` with tabular figures, right-aligned under the
		// track (`settings.css:1592-1599`).
		ui.Text(c, updateProgressLabel(progress)).FontSize(tokens.Caption).
			TextColor(tokens.TextSecondary).TextAlign(ui.End).SingleLine()
	})
}

// updateProgressLabel is the progress readout the old build formatted inline:
// `Math.round(percent)% · formatBytes(downloaded) / formatBytes(total)`, or
// `formatBytes(downloaded)` when there is no total.
func updateProgressLabel(progress *UpdateProgress) string {
	if progress.Total <= 0 {
		return formatBytes(progress.Downloaded)
	}
	percent := int(math.Round(float64(progress.Downloaded) * 100 / float64(progress.Total)))
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}
	return strconv.Itoa(percent) + "% \u00b7 " + formatBytes(progress.Downloaded) + " / " + formatBytes(progress.Total)
}

// formatBytes is the old AboutPage's `formatBytes`: B/KB/MB/GB, whole bytes,
// one decimal above that, and "0 B" for anything not positive.
func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB"}
	value := float64(bytes)
	index := 0
	for value >= 1024 && index < len(units)-1 {
		value /= 1024
		index++
	}
	if index == 0 {
		return strconv.FormatInt(int64(math.Round(value)), 10) + " B"
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + units[index]
}

// about shows the build's identity and where its files live.
func (a *App) about(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	about := a.About
	a.pageColumn(c, func() {
		// The build's identity is the panel's own header band (see View); the
		// page's rows carry the same facts in the card the update check acts
		// from.
		a.card(c, func() {
			a.keyValue(c, copy.AboutFramework, about.Framework)
			a.keyValue(c, copy.AboutScheme, about.Scheme)
			a.keyValue(c, copy.AboutSettingsFile, about.SettingsPath)
			if about.RepoURL != "" {
				a.row(c, copy.AboutProject, "", func() {
					ui.Link(c, about.RepoURL, about.RepoURL).FontSize(t.FontSize)
				})
			}
		})
		// The update row lives in its own group, the reference's
		// `settings.group.update` section with its card (`AboutPage.tsx:62-70`):
		// the one control the user presses gets a heading and a card of its own
		// rather than sitting loose beside the build's identity.
		a.section(c, copy.GroupUpdate, "", func() {
			a.card(c, func() {
				a.updateRow(c, copy)
			})
		})
		// The deep-link lines: a URL scheme is invisible when it works and
		// invisible when it does not, so the About page is the one place a
		// user can find out that floter answers `floter://` at all. Copying a
		// line is confirmed on the button itself (the old build's own 1.6s
		// "Link copied"), because a clipboard write is otherwise silent.
		a.sectionInline(c, copy.DeepLinkTitle, copy.DeepLinkHint, func() {
			a.card(c, func() {
				for _, link := range []string{deepLinkConnectExample, deepLinkRegisterExample} {
					link := link
					copied := a.linkCopied(link, c.Now())
					if copied {
						// Keep a frame coming at the moment the confirmation
						// expires, so the button returns to "Copy link" on its
						// own rather than at the next unrelated event.
						c.After(a.copiedLinkUntil.Sub(c.Now()))
					}
					label := copy.DeepLinkCopy
					if copied {
						label = copy.DeepLinkCopied
					}
					a.codeRow(c, link, func() {
						a.actionWithGlyph(c, linkCopyGlyph(copied), label, func() {
							if a.Actions.CopyText != nil {
								a.Actions.CopyText(link)
								a.copiedLink, a.copiedLinkUntil = link, c.Now().Add(linkCopiedDuration)
								if a.Actions.NotifySuccess != nil {
									a.Actions.NotifySuccess(copy.DeepLinkCopied)
								}
							}
						})
					})
				}
			})
		})
	})
}

// linkCopyGlyph is the mark on a deep-link copy action: lucide's Check while
// the copy's 1.6s confirmation holds, Copy otherwise — the two marks the
// reference's `DeepLinkRow` swapped (`DeepLinkRow.tsx:47-58`).
func linkCopyGlyph(copied bool) string {
	if copied {
		return "check"
	}
	return "copy"
}

// The two `floter://` forms the About page offers. The reference declared
// them once in `src/deep-link.ts` (`DEEP_LINK_EXAMPLE` /
// `DEEP_LINK_REGISTER_EXAMPLE`), and the page copied them; the pair lives here
// so the rendered value and the test that copies it cannot drift.
const (
	deepLinkConnectExample  = "floter://connect?manifest=/path/to/tool.json"
	deepLinkRegisterExample = "floter://register?cmd=rg"
)

// linkCopiedDuration is how long the deep-link copy button keeps its
// confirmation: the old build's own 1600ms.
const linkCopiedDuration = 1600 * time.Millisecond

// linkCopied reports whether the deep-link copy button for value should show
// its confirmation, clearing the state once its window has passed.
func (a *App) linkCopied(value string, now time.Time) bool {
	if a.copiedLink == "" || a.copiedLink != value {
		return false
	}
	if !now.Before(a.copiedLinkUntil) {
		a.copiedLink = ""
		return false
	}
	return true
}

// keyValue is one labeled line of read-only information: the label in a
// fixed column, the value beside it, selectable so it can be copied.
// keyValue is one About row: the label at the start, the selectable value at
// the end (wrapped so a long path still reads).
func (a *App) keyValue(c *ui.Context, label, value string) {
	t := c.Theme()
	if value == "" {
		return
	}
	a.row(c, label, "", func() {
		ui.Text(c, value).FontSize(t.FontSize).TextColor(t.TextMuted).Selectable()
	})
}

// runningSessions is what the Sessions page lists.
func (a *App) runningSessions() []Session {
	if a.Sessions == nil {
		return nil
	}
	return a.Sessions()
}

// summonShortcut is the global shortcut the shell registered, or a dash
// while it is unknown (a test build).
func (a *App) summonShortcut() string {
	if a.Shortcut == "" {
		return "-"
	}
	return a.Shortcut
}

// general is the General page: the settings the loader owns, each a real
// control writing straight through the store.
func (a *App) general(c *ui.Context, copy i18n.Settings) {
	s := a.Store.Snapshot()

	a.pageColumn(c, func() {
		a.section(c, copy.GroupAppearance, "", func() {
			a.card(c, func() {
				a.pick(c, copy.Theme, copy.ThemeHint, copy.Themes, s.Theme,
					func(id string) { a.set(func(s *settings.Settings) { s.Theme = id }) })
				a.pick(c, copy.Language, copy.LanguageHint, copy.Languages, s.Language,
					func(id string) { a.set(func(s *settings.Settings) { s.Language = id }) })
				// R7-13c · the interface size sits in Appearance next to
				// theme/language because it is the same kind of choice — how the
				// interface looks — not a window behaviour: it scales the whole
				// UI wherever the window is, and it keeps working when the window
				// never hides. It was in the Window behaviour card, a group it has
				// nothing in common with.
				a.pick(c, copy.Scale, copy.ScaleHint, copy.Scales, s.UIScale,
					func(id string) { a.set(func(s *settings.Settings) { s.UIScale = id }) })
			})
		})

		a.section(c, copy.GroupWindow, "", func() {
			a.card(c, func() {
				a.checkbox(c, copy.LaunchAtStartup, copy.LaunchAtStartupHint, s.LaunchAtStartup,
					func(on bool) { a.set(func(s *settings.Settings) { s.LaunchAtStartup = on }) })
				a.checkbox(c, copy.HideOnBlur, copy.HideOnBlurHint, s.HideOnBlur,
					func(on bool) { a.set(func(s *settings.Settings) { s.HideOnBlur = on }) })
				// R35/R41 · the page-residency window sits beside `hide_on_blur`
				// because the two are read together but answer different
				// questions: the switch decides whether the *window* disappears,
				// this one decides whether the *surface* survives it. The
				// reference draws it before the recent-items switch, as the
				// single row residencyRow builds (`SurfaceResidencyControl`).
				a.residencyRow(c, copy)
				a.checkbox(c, copy.ShowRecentInLauncher, copy.ShowRecentInLauncherHint, settings.ShowRecentInLauncher(s),
					func(on bool) { a.set(func(s *settings.Settings) { s.SetShowRecentInLauncher(on) }) })
			})
		})

		// The menu bar group: the two controls that describe the one icon the
		// user sees in the menu bar / tray — its visibility switch and the
		// light/dark choice for its image. They share a card because they are
		// one residency choice (`settings.group.menuBar`); splitting the icon
		// into Appearance left the two ends of the same decision a screen apart.
		a.section(c, copy.GroupMenuBar, "", func() {
			a.card(c, func() {
				a.checkbox(c, copy.ShowMenubarIcon, copy.ShowMenubarIconHint, settings.ShowMenubarIcon(s),
					func(on bool) { a.set(func(s *settings.Settings) { s.SetShowMenubarIcon(on) }) })
				a.pick(c, copy.AppIcon, copy.AppIconHint, copy.AppIcons, a.appIcon(),
					func(id string) {
						a.set(func(s *settings.Settings) { s.SetExtra("app_icon", id) })
					})
			})
		})

		a.section(c, copy.GroupTerminal, "", func() {
			a.card(c, func() {
				// The family is a picker of the faces this machine actually has
				// (detected from its font files), plus the current value so a
				// hand-set family is never lost.
				families := make([]i18n.Option, 0, len(terminalui.FontFamilyOptions(s.FontFamily)))
				for _, family := range terminalui.FontFamilyOptions(s.FontFamily) {
					families = append(families, i18n.Option{ID: family, Label: family})
				}
				// The card paints the axes the schema puts on this surface
				// (internal/control, R55): the family, the size, the line height
				// and the palette. The padding, cursor shape and blink rows live
				// in the terminal's own appearance drawer, so the page does not
				// grow a second copy of a control the terminal already owns.
				rows := map[control.AppearanceAxis]func(){
					control.AxisFontFamily: func() {
						a.choose(c, copy.FontFamily, "", families, s.FontFamily,
							func(v string) { a.set(func(s *settings.Settings) { s.FontFamily = v }) })
					},
					control.AxisFontSize: func() {
						// The readout is the reference's `<n> px`, and the row carries
						// the card-only hint (`TerminalAppearance.tsx:190-195`): the
						// settings card shows `sublabel: terminalAppearanceHint` on the
						// font-size row, while the terminal's own drawer keeps it out.
						a.slider(c, sliderRow{
							label: copy.FontSize, description: copy.TerminalHint,
							low: copy.ScaleSmall, high: copy.ScaleLarge,
							value: float64(s.FontSize), lo: settings.MinFontSize, hi: settings.MaxFontSize,
							format: func(v float64) string { return fmt.Sprintf("%d px", int(math.Round(v))) },
							apply: func(v float64) {
								a.previewSetting(func(s *settings.Settings) { s.FontSize = int(math.Round(v)) })
							},
						})
					},
					control.AxisLineHeight: func() {
						// `.toFixed(2)` — the step is 0.05, so one decimal dropped the
						// hundredths the reference shows (`TerminalAppearance.tsx:210-215`).
						a.slider(c, sliderRow{
							label: copy.LineHeight, low: copy.ScaleSmall, high: copy.ScaleLarge,
							value: s.TerminalLineHeight,
							lo:    settings.MinTerminalLineHeight, hi: settings.MaxTerminalLineHeight,
							format: func(v float64) string { return fmt.Sprintf("%.2f\u00d7", v) },
							apply: func(v float64) {
								a.previewSetting(func(s *settings.Settings) { s.TerminalLineHeight = v })
							},
						})
					},
					control.AxisPalette: func() {
						a.paletteRow(c, copy, s.TerminalTheme,
							func(id string) { a.set(func(s *settings.Settings) { s.TerminalTheme = id }) })
					},
				}
				for _, axis := range control.SettingsCardAxes() {
					if row, ok := rows[axis]; ok {
						row()
					}
				}
			})
		})

		// The material group is the reference's *last* General-page section
		// (`GeneralPage.tsx:558`, after the terminal appearance section at
		// `:533`): the glass effect on one card and the two transparency sliders
		// on a second, the group's two explanations drawn as footnotes under both
		// (the reference's `.settings-section__hint` at the end of the section)
		// rather than repeated on every row. It is also the reference's one
		// section spaced looser than the rest — `.settings-section--material`
		// (`settings.css:767-772`), which `sectionMaterial` keeps. The old port
		// drew it second, right under Appearance.
		a.sectionMaterial(c, copy.Material, []string{copy.GlassHint, copy.TransparencyHint}, func() {
			a.card(c, func() {
				a.glassPick(c, copy.Glass, "", copy.GlassSteps, s.GlassStep,
					func(id string) { a.set(func(s *settings.Settings) { s.GlassStep = id }) })
			})
			a.cardLabeled(c, copy.GroupTransparency, func() {
				// A window the system gives no material to has nothing for the
				// desktop to show through (shell.windowShowsMaterial paints the
				// card solid there), so both sliders are drawn disabled with the
				// reason rather than left as knobs that move nothing
				// (「滑块整条失效」).
				disabled := ""
				if !glassmap.TransparencyAvailable(runtime.GOOS, c.Vibrancy()) {
					disabled = copy.TransparencyUnavailable
				}
				a.slider(c, sliderRow{
					label: copy.MainOpacity, low: copy.TransparencyScaleLow, high: copy.TransparencyScaleHigh,
					value: float64(s.MainOpacity), lo: settings.MinWindowOpacity, hi: settings.MaxWindowOpacity,
					format:   func(v float64) string { return copy.Percent(clampPercent(v)) },
					apply:    func(v float64) { a.previewSetting(func(s *settings.Settings) { s.MainOpacity = clampPercent(v) }) },
					disabled: disabled,
				})
				a.slider(c, sliderRow{
					label: copy.TerminalOpacity, low: copy.TransparencyScaleLow, high: copy.TransparencyScaleHigh,
					value: float64(s.TerminalOpacity), lo: settings.MinWindowOpacity, hi: settings.MaxWindowOpacity,
					format: func(v float64) string { return copy.Percent(clampPercent(v)) },
					apply: func(v float64) {
						a.previewSetting(func(s *settings.Settings) { s.TerminalOpacity = clampPercent(v) })
					},
					disabled: disabled,
				})
				// The one readout the group carries: what the window *shows*,
				// once the glass material's own weight is counted. The slider is
				// the ask; this is the answer, and it differs where the material
				// has a floor of its own (glassmap.Spec.Composite).
				a.effectiveOpacity(c, copy, s)
			})
		})
	})
}

// pick builds one labeled segmented choice: options[i].Label stands for
// options[i].ID. The reference draws every segmented row stacked
// (`GeneralPage.tsx:340,356,382,516,568` and `TerminalAppearance.tsx:388`), so
// the choice sits on its own full-width line under the label rather than in a
// trailing slot.
func (a *App) pick(c *ui.Context, label, description string, options []i18n.Option, current string, apply func(string)) {
	a.pickWith(c, a.segmented, label, description, options, current, apply)
}

// glassPick is `pick` for the material picker alone. The reference draws that
// one track with `.glass-intensity__segments` (`main:src/styles/settings.css:
// 792-799`) — labels centered because a glass stop is a *choice*, where every
// other segmented row is a row of labels read left to right — so it routes
// through `SegmentedGlassFill` instead of the base form.
func (a *App) glassPick(c *ui.Context, label, description string, options []i18n.Option, current string, apply func(string)) {
	a.pickWith(c, a.glassSegmented, label, description, options, current, apply)
}

// pickWith is the one body `pick` and `glassPick` share; draw is the segmented
// form that carries the labels (base or glass).
func (a *App) pickWith(c *ui.Context, draw func(*ui.Context, []string, int, func(int)), label, description string, options []i18n.Option, current string, apply func(string)) {
	index := optionIndex(options, current)
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
	}
	chosen := ""
	a.stackedRow(c, label, description, func() {
		draw(c, labels, index, func(i int) {
			if i >= 0 && i < len(options) {
				chosen = options[i].ID
			}
		})
	})
	if chosen != "" {
		apply(chosen)
	}
}

// segmented is the reference's inline segmented control, drawn from the one
// shared implementation (`control.Segmented`) so the terminal's appearance
// drawer cannot drift into a second shape for the same choice. Its callers are
// the stacked rows (the settings picks and the plugin-sheet radio), where the
// reference's control spans the row, so it takes the full-width form.
func (a *App) segmented(c *ui.Context, labels []string, current int, apply func(int)) {
	control.SegmentedFill(c, a.tokens(c), labels, current, apply)
}

// glassSegmented is `segmented` for the material picker: the reference's one
// centered track (`.glass-intensity__segments`).
func (a *App) glassSegmented(c *ui.Context, labels []string, current int, apply func(int)) {
	control.SegmentedGlassFill(c, a.tokens(c), labels, current, apply)
}

// choose builds one labeled drop-down choice, for lists too long to line up
// as segments.
func (a *App) choose(c *ui.Context, label, description string, options []i18n.Option, current string, apply func(string)) {
	index := optionIndex(options, current)
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
	}
	selected := ""
	if index >= 0 && index < len(options) {
		selected = options[index].Label
	}
	chosen := ""
	a.row(c, label, description, func() {
		e := a.chooseSelect(c, &selected, labels, label)
		if e.Changed() {
			for _, option := range options {
				if option.Label == selected {
					chosen = option.ID
					break
				}
			}
		}
	})
	if chosen != "" {
		apply(chosen)
	}
}

// chooseSelect builds the drop-down `choose` draws: the framework's Select at
// the surface's own metric. The extension panel's `.extension-config-field
// input, select` share one literal box (`extensions.css:2402-2417`), so inside
// `extensionMetrics` the trigger is pinned to that box and does not follow the
// interface step; on the settings pages, where `cardUnit` is zero, it keeps the
// framework's `--u` metrics and scales, as the reference's own fields did.
//
// Both faces are the reference's *field* rung, not the framework's control
// rung: `.settings-select` (`settings.css:919-940`) and
// `.extension-config-field select` (`extensions.css:2402-2417`) both paint
// `--glass-field` under `--glass-field-shadow`, so the trigger wears
// `fieldFace` rather than the opaque `--glass-control` the framework's own
// Select draws.
func (a *App) chooseSelect(c *ui.Context, selected *string, labels []string, label string) ui.Element {
	e := ui.Select(c, selected, labels).Label(label).Width(220).Radius(999)
	if a.cardUnit > 0 {
		e = e.MinHeight(28).Padding(4, 8)
	}
	return a.fieldFace(c, e)
}

// paletteRow is the terminal palette row: the labeled picker, with a
// miniature terminal painted in the chosen palette's own colors under it
// (R43: the user asked a palette to show what it does). The preview is a block
// in the row, never an overlay, and `inherit` follows the app's appearance.
func (a *App) paletteRow(c *ui.Context, copy i18n.Settings, current string, apply func(string)) {
	options := copy.Palettes
	index := optionIndex(options, current)
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
	}
	selected := ""
	if index >= 0 && index < len(options) {
		selected = options[index].Label
	}
	chosen := ""
	// No sublabel: the reference's palette row is `stacked: true` with no
	// hint (the card-only `terminalAppearanceHint` belongs to the font-size
	// row alone, `TerminalAppearance.tsx:194,272-276`). Its control is one
	// full-width line, not a trailing slot: the reference's `.terminal-palette`
	// splits the select and the preview apart (`settings.css:871-878`).
	a.stackedRow(c, copy.Palette, "", func() {
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			e := a.fieldFace(c, ui.Select(c, &selected, labels).Label(copy.Palette).Width(220).Radius(999))
			if e.Changed() {
				for _, option := range options {
					if option.Label == selected {
						chosen = option.ID
						break
					}
				}
			}
			// The preview follows the picker in the same frame the choice is
			// made, so a click on the select repaints the block immediately.
			shown := current
			if chosen != "" {
				shown = chosen
			}
			a.palettePreview(c, copy, shown)
		})
	})
	if chosen != "" {
		apply(chosen)
	}
}

// palettePreview paints the miniature terminal: three lines of static text in
// the palette's own background and text, the middle line behind its selection
// tint and a block cursor at the end — the reference's
// `.terminal-palette-preview`, which costs no canvas and no PTY. It keeps the
// drawer's own fixed width so the picker and the block share one line.
func (a *App) palettePreview(c *ui.Context, copy i18n.Settings, name string) {
	terminalui.PalettePreview(c, a.tokens(c), name, copy.PalettePreview, c.Theme().Space(37))
}

// sliderRow is one range row's content: its words, its axis, its value, what a
// change does, and — when the row cannot be moved — the sentence that says why.
// It is a struct rather than nine positional arguments because the two
// transparency rows are the only ones that can be disabled, and a bare string
// in the middle of a call is the kind of argument a later reader silently
// shifts.
type sliderRow struct {
	label, description string
	// low and high are the axis' own words at the track's two ends; when both
	// are empty the scale line is not drawn rather than inventing an axis.
	low, high     string
	value, lo, hi float64
	format        func(float64) string
	apply         func(float64)
	// disabled, when set, is why the row cannot be moved: the track is drawn
	// disabled and the sentence is printed under it. It is what a window the
	// system gives no material to shows instead of a knob that moves nothing.
	disabled string
}

// slider builds one stacked row: the label and its live readout on the title
// line, the hint under them, the track at full width, and one word at each end
// of the track naming the axis — the reference's `.settings-slider__head` +
// `SettingsScale` shape (the macOS range row).
func (a *App) slider(c *ui.Context, row sliderRow) {
	tokens := a.tokens(c)
	t := c.Theme()
	inset := a.rowInset(t)
	v := row.value
	chosen := false
	first := a.cardRows == 0
	a.cardRows++
	ui.Column(c).FillWidth().Children(func() {
		if !first {
			ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).MarginX(inset).Shrink(0)
		}
		ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(2.25), inset).Children(func() {
			// The title line carries the value, pushed to the far end, as the
			// reference's `.settings-slider__head` does: the number is a readout,
			// not a second control, and a rolling counter is how the motion
			// system shows a value changing in place. Its label is the row's own
			// emphasis step (`.settings-row__label`), the readout the body step
			// (`.terminal-setting-control__value`).
			ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Text(c, row.label).FontSize(t.FontSize + 1).FontWeight(580).TextColor(tokens.TextStrong)
				ui.Text(c, row.format(v)).FontSize(t.FontSize).TextColor(tokens.TextStrong).
					ContentTransition(ui.ContentTransition{Numeric: true})
			})
			if row.description != "" {
				ui.Text(c, row.description).FontSize(t.FontSize).TextColor(t.TextMuted)
			}
			track := ui.Slider(c, &v, row.lo, row.hi).FillWidth()
			if row.disabled != "" {
				track.Disabled(true)
			}
			if track.Changed() {
				chosen = true
			}
			if row.low != "" || row.high != "" {
				// The axis' words are the ladder's caption step, not "body
				// minus two": at the shipped `small` step that offset was 7.9pt,
				// the smallest ink on any page, and the two words that say what
				// the track's ends mean were the hardest thing to read in the
				// row they belong to.
				ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Text(c, row.low).FontSize(tokens.Caption).FontWeight(500).TextColor(t.TextMuted)
					ui.Text(c, row.high).FontSize(tokens.Caption).FontWeight(500).TextColor(t.TextMuted)
				})
			}
			if row.disabled != "" {
				ui.Text(c, row.disabled).FontSize(tokens.Caption).FontWeight(500).TextColor(tokens.TextTertiary)
			}
		})
	})
	if chosen && row.disabled == "" {
		row.apply(v)
	}
}

// effectiveOpacity is the transparency group's one readout line: what the
// window actually shows once the glass material's own weight is counted
// (glassmap.Spec.Composite). It is the answer to the two sliders above it — the
// slider is the ask — and it is the number that made the coupling visible: the
// same 47% used to show three different ways across the three materials, and a
// window with no material at all used to show nothing.
func (a *App) effectiveOpacity(c *ui.Context, copy i18n.Settings, s settings.Settings) {
	tokens := a.tokens(c)
	t := c.Theme()
	inset := a.rowInset(t)
	spec := glassmap.SpecForPreferencesOS(runtime.GOOS, s.GlassStep, s.MainOpacity, c.Preferences().HighContrast)
	percent := uint8(math.Round(float64(spec.Composite()) * 100))
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Padding(t.Space(1), inset).Children(func() {
		ui.Text(c, copy.EffectiveOpacity).FontSize(tokens.Caption).FontWeight(500).TextColor(tokens.TextTertiary)
		ui.Text(c, copy.EffectiveOpacityValue(percent)).FontSize(tokens.Caption).FontWeight(560).
			TextColor(tokens.TextSecondary).ContentTransition(ui.ContentTransition{Numeric: true})
	})
}

// checkbox builds one self-labeling check box in the form's content column:
// the switch of the old panel, in the control this toolkit makes directly
// clickable by its own text. The description is the row's grey second line,
// as every other row kind draws it.
func (a *App) checkbox(c *ui.Context, label, description string, on bool, apply func(bool)) {
	value := on
	changed := false
	// The label is the row's own (the reference puts it at the start) and
	// the control is the switch at the end, so the check box here is the
	// switch alone.
	if a.row(c, label, description, func() {
		if a.switchControl(c, &value).Label(label).Changed() {
			changed = true
		}
	}) {
		// The row's own label is a hit target: clicking it toggles, as the
		// framework's Field does for its control.
		value = !value
		changed = true
	}
	if changed {
		apply(value)
	}
}

// text builds one labeled text field.
func (a *App) text(c *ui.Context, label, description, placeholder, value string, apply func(string)) {
	edit := value
	a.row(c, label, description, func() {
		e := a.textInput(c, &edit).Placeholder(placeholder).Label(label).Width(220)
		if e.Submitted() {
			apply(edit)
		}
	})
}

// appIcon is the stored app icon, normalized to the two the app ships (dark
// is the default).
func (a *App) appIcon() string {
	value, _ := a.Store.Snapshot().Extra()["app_icon"].(string)
	if value == "light" {
		return "light"
	}
	return "dark"
}

// set writes one settings change through the store. A refused write is not
// silent: the store rolls the change back to its last committed snapshot and
// the banner over the page says the file could not be written.
func (a *App) set(mutate func(*settings.Settings)) {
	// An immediate write also commits anything the debounce is holding, so the
	// two paths cannot disagree about which value is current.
	a.flushSettingsWrites()
	if err := a.Store.Update(mutate); err != nil {
		a.noteSaveFailed()
	}
}

func clampPercent(v float64) uint8 {
	n := uint8(math.Round(v))
	if n < settings.MinWindowOpacity {
		return settings.MinWindowOpacity
	}
	if n > settings.MaxWindowOpacity {
		return settings.MaxWindowOpacity
	}
	return n
}

// parseResidencyDraft reads a typed duration the way the reference's
// `normalizeResidencySeconds(Number(draft))` does: an empty field is zero
// (off), a non-numeric one falls back to the shipped default rather than
// switching the window off, and a real number is clamped onto the domain.
func parseResidencyDraft(text string) uint32 {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	n, err := strconv.ParseUint(trimmed, 10, 32)
	if err != nil {
		return settings.DefaultSurfaceResidencySeconds
	}
	return settings.NormalizeResidencySeconds(uint32(n))
}

// residencyPresetSeconds are the select's named steps, the reference's
// `RESIDENCY_PRESET_SECONDS` ladder: every one a value a person says out loud.
// Off and never are separate entries, not members of this list.
var residencyPresetSeconds = []uint32{10, 20, 30, 60, 120}

// residencyCustom is the select's own entry for a duration the presets do not
// name; its value lives in the inline field, committed by the user.
const residencyCustom = "custom"

// residencySelectValue is the select's value for a stored duration: "off", a
// preset's own number as text, "never", or "custom" for anything else — the
// reference's pure mapping (`residencySelectValue`). The select shows this,
// never a second copy of the state.
func residencySelectValue(seconds uint32) string {
	value := settings.NormalizeResidencySeconds(seconds)
	switch value {
	case 0:
		return "off"
	case settings.SurfaceResidencyNever:
		return "never"
	}
	for _, preset := range residencyPresetSeconds {
		if preset == value {
			return strconv.FormatUint(uint64(preset), 10)
		}
	}
	return residencyCustom
}

// residencyFromSelect is the number a choice stands for. ok is false for the
// custom entry (and any id the domain does not know): its value is the inline
// field's, so the caller opens the field rather than writing anything.
func residencyFromSelect(choice string) (uint32, bool) {
	switch choice {
	case "off":
		return 0, true
	case "never":
		return settings.SurfaceResidencyNever, true
	case residencyCustom:
		return 0, false
	}
	n, err := strconv.ParseUint(choice, 10, 32)
	if err != nil {
		return 0, false
	}
	return settings.NormalizeResidencySeconds(uint32(n)), true
}

// residencyCustomSeedSeconds is the number the inline field opens on: a stored
// custom duration seeds the field with itself, so opening "custom" on an
// existing one does not silently reset it; off, a preset and "never" seed the
// shipped default — the least surprising number to edit from
// (`residencyCustomSeedSeconds`). A hand-edited out-of-range value has already
// been clamped by the normalizer, and seeds that clamped number.
func residencyCustomSeedSeconds(seconds uint32) uint32 {
	value := settings.NormalizeResidencySeconds(seconds)
	if value == 0 || value == settings.SurfaceResidencyNever {
		return settings.DefaultSurfaceResidencySeconds
	}
	for _, preset := range residencyPresetSeconds {
		if preset == value {
			return settings.DefaultSurfaceResidencySeconds
		}
	}
	return value
}

// residencyCustomLabel is the custom entry's label: while a custom duration is
// stored the entry echoes it (`Custom (45 s)`), so the select never lies about
// the number in the field; otherwise it is the plain "Custom…".
func residencyCustomLabel(copy i18n.Settings, value uint32, selectValue string) string {
	if selectValue == residencyCustom {
		return copy.SurfaceResidencyCustomValue(value)
	}
	return copy.SurfaceResidencyCustom
}

// residencyRow paints the page-residency control as one row: the select names
// off, the presets, never and custom, and the inline seconds field (with its
// unit and Set button) appears beside it only while "custom" is chosen — the
// reference's `SurfaceResidencyControl` (`GeneralPage.tsx:157-250`). The row
// never grows a second line for the field.
func (a *App) residencyRow(c *ui.Context, copy i18n.Settings) {
	s := a.Store.Snapshot()
	value := settings.NormalizeResidencySeconds(s.SurfaceResidencySeconds)
	selectValue := residencySelectValue(value)

	// The field's own state. The first frame seeds it from what is stored, so
	// a stored custom duration opens the field it came from; afterwards a value
	// written anywhere else re-seeds the draft while the field is closed, and
	// the user's typing is the draft while it is open (the reference's `useState`
	// initializers plus its re-seeding effect).
	if !a.residencyInit {
		a.residencyInit = true
		a.residencyCustomOpen = selectValue == residencyCustom
	}
	if !a.residencyCustomOpen {
		a.residencyDraft = strconv.FormatUint(uint64(residencyCustomSeedSeconds(value)), 10)
	}

	labels := []string{copy.SurfaceResidencyOff}
	choices := []string{"off"}
	for _, preset := range residencyPresetSeconds {
		labels = append(labels, copy.SurfaceResidencyValue(preset))
		choices = append(choices, strconv.FormatUint(uint64(preset), 10))
	}
	labels = append(labels, copy.SurfaceResidencyNever, residencyCustomLabel(copy, value, selectValue))
	choices = append(choices, "never", residencyCustom)

	selected := labels[0]
	for i, choice := range choices {
		if choice == selectValue {
			selected = labels[i]
		}
	}

	t := c.Theme()
	a.row(c, copy.SurfaceResidency, copy.SurfaceResidencyHint, func() {
		e := a.fieldFace(c, ui.Select(c, &selected, labels).Label(copy.SurfaceResidency).Width(200).Radius(999))
		if e.Changed() {
			for i, label := range labels {
				if label == selected {
					a.residencyChoice(choices[i], value)
					break
				}
			}
		}
		if !a.residencyCustomOpen {
			return
		}
		input := a.textInput(c, &a.residencyDraft).Label(copy.SurfaceResidencyCustomLabel).
			Placeholder(copy.SurfaceResidencyCustomUnit).Width(90)
		ui.Text(c, copy.SurfaceResidencyCustomUnit).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		if input.Submitted() || ui.Button(c, copy.SurfaceResidencyCustomApply).Clicked() {
			a.commitResidencyDraft()
		}
	})
}

// residencyChoice applies one select choice: a concrete duration is written
// through and closes the field; "custom" opens the field on the seed and
// writes nothing until the user commits.
func (a *App) residencyChoice(choice string, stored uint32) {
	next, ok := residencyFromSelect(choice)
	if !ok {
		a.residencyDraft = strconv.FormatUint(uint64(residencyCustomSeedSeconds(stored)), 10)
		a.residencyCustomOpen = true
		return
	}
	a.residencyCustomOpen = false
	a.set(func(s *settings.Settings) { s.SurfaceResidencySeconds = next })
}

// commitResidencyDraft writes the inline field's duration: it normalizes the
// typed text, echoes the stored number back into the field, and closes the
// field when that number lands on a named entry — the reference's commitCustom.
func (a *App) commitResidencyDraft() {
	next := parseResidencyDraft(a.residencyDraft)
	a.residencyDraft = strconv.FormatUint(uint64(next), 10)
	a.residencyCustomOpen = residencySelectValue(next) == residencyCustom
	a.set(func(s *settings.Settings) { s.SurfaceResidencySeconds = next })
}

func optionIndex(options []i18n.Option, id string) int {
	for i, option := range options {
		if option.ID == id {
			return i
		}
	}
	return 0
}

// CustomTool is a local tool the user wants to connect: what the form
// collected. The host mints the id and derives the commands.
type CustomTool struct {
	Name string
	// Mode is "executable" for a program on this machine and "script" for a
	// provider the host writes from the editor below.
	Mode string
	// ScriptLanguage and ScriptContent describe a script tool: the language
	// id (see internal/scriptlang) and the script itself.
	ScriptLanguage string
	ScriptContent  string
	Program        string
	Command        string
	Args           string
	VersionArgs    string
	Version        string
	Description    string
	Output         string
	Permissions    []string
	// Platforms limits the connection to the operating systems it names; the
	// form always sends at least one.
	Platforms []string
	// Params are the inputs the connected tool declares.
	Params []extensions.ParamDefinition
}

// localToolSection is the "connect a program on this machine" form: the words
// that name it, the file itself, its arguments, and what it declares. The
// section is folded away until the user asks for it, so the page stays a list
// of what is installed.
func (a *App) localToolSection(c *ui.Context, copy i18n.Settings) {
	if a.Actions.ConnectLocalTool == nil {
		return
	}
	a.section(c, copy.LocalToolTitle, copy.LocalToolHint, func() {
		if a.actionButton(c, copy.LocalToolAdd).Clicked() {
			a.toolOpen = !a.toolOpen
			// Opening a fresh form clears the previous refusal, so the new one
			// starts quiet; closing keeps nothing either. It also drops the
			// cached toolchain verdicts: the user may have installed the
			// runtime since the last form.
			if a.toolOpen {
				a.clearScriptRuntimeCache()
			}
			a.toolIssue = false
		}
		if !a.toolOpen {
			return
		}
		if a.toolPermissions == nil {
			a.toolPermissions = map[string]bool{}
		}
		// A fresh form starts limited to this machine's own platform (R7-5).
		// An edit keeps whatever the integration declares, even nothing: the
		// row then says a platform is required rather than silently inventing
		// one the manifest does not name.
		if a.toolPlatforms == nil {
			a.toolPlatforms = map[string]bool{currentPlatform(): true}
		}
		name, program, command := a.toolName, a.toolProgram, a.toolCommand
		args, versionArgs, version := a.toolArgs, a.toolVersionArgs, a.toolVersion
		description := a.toolDescription
		// The kind of tool decides which rows the form shows: a program on
		// this machine, or a script the host writes from the editor below.
		mode := a.toolMode
		language := a.toolLanguage
		if language == "" {
			language = scriptlang.Languages[0].ID
		}
		script := a.toolScript
		if script == "" && mode == routeScript {
			script = scriptlang.Template(language)
		}
		// The rules the host will apply, read here first: a row that broke one
		// says so under itself rather than a banner bounced back from the
		// backend. The same function gates the submit, so the two cannot drift.
		issues := customIssues(a.customDraft(name, program, command, args, versionArgs, version, description, mode, language, script))
		a.card(c, func() {
			// R9-3 · the id is minted by the backend and immutable, so when
			// an existing integration is being edited the form names it in a
			// read-only row instead of offering an editable field. A fresh
			// connection has no id yet, so the row is absent.
			if a.toolEditing != "" {
				a.keyValue(c, copy.LocalToolId, a.toolEditing)
			}
			a.row(c, copy.LocalToolName, "", func() {
				a.extensionField(c, &name, customFieldMetrics).Label(copy.LocalToolName).Width(240).Grow(1).Placeholder("Ripgrep")
			}, func() {
				a.toolIssueUnder(c, copy, issues, extensions.CustomFieldName)
			})
			a.row(c, copy.LocalToolMode, "", func() {
				index := 0
				if mode == routeScript {
					index = 1
				}
				if a.segmentedControl(c, &index, copy.LocalToolModeProgram, copy.LocalToolModeScript).Changed() {
					if index == 0 {
						mode = ""
					} else {
						mode = routeScript
					}
				}
			}, func() {
				a.toolIssueUnder(c, copy, issues, extensions.CustomFieldMode)
			})
			if mode == routeScript {
				a.scriptRows(c, copy, &language, &script, issues)
			} else {
				a.row(c, copy.LocalToolProgram, copy.LocalToolProgramHint, func() {
					a.extensionField(c, &program, customFieldMetrics).Label(copy.LocalToolProgram).Width(240).Grow(1).
						Placeholder(copy.LocalToolProgramPlaceholder)
					if a.Actions.ChooseProgram != nil {
						if a.actionButton(c, copy.LocalToolChoose).Clicked() {
							if picked, err := a.Actions.ChooseProgram(); err == nil && picked != "" {
								program = picked
							}
						}
					}
				})
			}
			a.row(c, copy.LocalToolCommand, copy.LocalToolCommandHint, func() {
				a.extensionField(c, &command, customFieldMetrics).Label(copy.LocalToolCommand).Width(160).Placeholder("rg")
			}, func() {
				a.toolIssueUnder(c, copy, issues, extensions.CustomFieldCommand)
			})
			if mode != routeScript {
				a.row(c, copy.LocalToolArgs, copy.LocalToolArgsHint, func() {
					a.extensionField(c, &args, customFieldMetrics).Label(copy.LocalToolArgs).Width(220).Placeholder("--floter")
				})
				a.row(c, copy.LocalToolVersionArgs, copy.LocalToolVersionArgsHint, func() {
					a.extensionField(c, &versionArgs, customFieldMetrics).Label(copy.LocalToolVersionArgs).Width(160).Placeholder("--version")
				})
			}
			a.row(c, copy.LocalToolVersion, copy.LocalToolVersionHint, func() {
				a.extensionField(c, &version, customFieldMetrics).Label(copy.LocalToolVersion).Width(120).Placeholder("0.0.1")
			}, func() {
				a.toolIssueUnder(c, copy, issues, extensions.CustomFieldVersion)
			})
			// R158 · the description is executable-only: a script manifest writes
			// its own internal description and never reads this field, so an
			// editable control here would be silently dropped. The value stays in
			// a.toolDescription, so switching back to executable restores it. R159
			// · the placeholder says a blank field means no description, and the
			// backend stores the honest empty rather than a generated sentence.
			if mode != routeScript {
				a.row(c, copy.LocalToolDescription, "", func() {
					a.extensionField(c, &description, customFieldMetrics).Label(copy.LocalToolDescription).Width(240).Grow(1).
						Placeholder(copy.LocalToolDescriptionPlaceholder)
				})
			}
			// The content actions sit where the reference put them: after the
			// script box and the fields, before the output row
			// (`CustomIntegrationDrawer.tsx:145-146`).
			a.toolContentActions(c, copy, mode, command, program, args, script, language)
			a.row(c, copy.LocalToolOutput, "", func() {
				index := 0
				if a.toolOutput == routeBackground {
					index = 1
				}
				if a.segmentedControl(c, &index, copy.LocalToolTerminal, copy.LocalToolBackground).Changed() {
					if index == 0 {
						a.toolOutput = ""
					} else {
						a.toolOutput = routeBackground
					}
				}
			})
			// The connection is limited to the platforms ticked here, and at
			// least one has to stay ticked: an empty list means "every
			// platform", which is not something the user can ask for by
			// unticking everything by accident. The refusal is named under
			// the row (R7-5).
			ticked := make([]bool, len(customPlatforms))
			for i, platform := range customPlatforms {
				ticked[i] = a.toolPlatforms[platform.ID]
			}
			a.row(c, copy.LocalToolPlatforms, "", func() {
				ui.Row(c).FillWidth().Gap(c.Theme().Space(3)).Wrap().Children(func() {
					for i, platform := range customPlatforms {
						i, platform := i, platform
						checked := ticked[i]
						if ui.Checkbox(c, &checked, platform.Label).Changed() {
							if checked {
								a.toolPlatforms[platform.ID] = true
							} else {
								delete(a.toolPlatforms, platform.ID)
							}
						}
					}
				})
			}, func() {
				if len(a.toolPlatforms) == 0 {
					a.issueUnder(c, copy.LocalToolPlatformRequired)
				}
			})
			// The declaration and audit switches sit behind an "Advanced"
			// disclosure, closed by default: a brand-new integration should not
			// open on a ceremonial list of permission checkboxes, and the note
			// inside says plainly that they isolate nothing (R161). Nothing is
			// unmounted — the ticks live in a.toolPermissions and are read at
			// connect time — so folding changes nothing about what is
			// submitted, and an author who already declared permissions can
			// open the section and edit them.
			opened := a.row(c, copy.LocalToolAdvanced, copy.LocalToolAdvancedPermissions, func() {
				if a.actionButton(c, copy.LocalToolAdvanced).Clicked() {
					a.toolAdvanced = !a.toolAdvanced
				}
			}, func() { a.toolPermissionFields(c, copy) })
			if opened {
				a.toolAdvanced = !a.toolAdvanced
			}
		})
		a.toolParamsEditor(c, copy)
		action := copy.LocalToolConnect
		if a.toolEditing != "" {
			action = copy.LocalToolSave
		}
		requested := a.actionButton(c, action).Clicked()
		a.toolName, a.toolProgram, a.toolCommand = name, program, command
		a.toolArgs, a.toolVersionArgs, a.toolVersion = args, versionArgs, version
		a.toolDescription = description
		a.toolMode, a.toolLanguage, a.toolScript = mode, language, script
		tool := a.customDraft(name, program, command, args, versionArgs, version, description, mode, language, script)
		if requested {
			// A row that still needs something must not reach the backend: the
			// editor already prints the reason under that row, so the save
			// refuses here rather than bouncing back an error to re-explain it.
			if len(extensions.ParamRowIssues(a.editorParams())) > 0 {
				a.toolIssue = true
				return
			}
			// The same shape rules the host will apply, read before the host
			// sees them: name, command, version, platforms, mode, script.
			if len(customIssues(tool)) > 0 {
				a.toolIssue = true
				return
			}
			// The program row has no localization key of its own: it refuses
			// silently, the same way the old form disabled its submit button.
			if tool.Mode != routeScript && tool.Program == "" {
				return
			}
			// A missing toolchain does not block the save — the user may install
			// the runtime and come back — but it warns once, so the integration
			// is not silently dead on arrival. The old form awaited the probe
			// before saving so the sentence reflects the machine rather than a
			// race; `scriptRuntimeVerdict` waits for a probe still in flight (it
			// normally already landed) and stays quiet when no probe is wired.
			if tool.Mode == routeScript && a.Actions.NotifyWarning != nil {
				if check, ok := a.scriptRuntimeVerdict(tool.ScriptLanguage); ok &&
					scriptlang.StatusOf(tool.ScriptLanguage, check).State == scriptlang.Missing {
					a.Actions.NotifyWarning(copy.LocalToolRuntimeSaveWarning(scriptlang.Lookup(tool.ScriptLanguage).Label))
				}
			}
			if a.toolEditing != "" {
				a.Actions.EditCustomTool(a.toolEditing, tool)
			} else {
				a.Actions.ConnectLocalTool(tool)
			}
			a.toolOpen = false
			a.toolIssue = false
			a.endEdit()
		}
	})
}

// customDraft is the form as the draft connection it would submit: one place
// that decides what each field means, so the pre-check and the host's request
// cannot disagree. A non-script carries no language — the field belongs to the
// script runtime, and the form's language state outlives a switch back to the
// program kind (R7-5's old host refused exactly this).
func (a *App) customDraft(name, program, command, args, versionArgs, version, description, mode, language, script string) CustomTool {
	platforms := make([]string, 0, len(customPlatforms))
	for _, platform := range customPlatforms {
		if a.toolPlatforms[platform.ID] {
			platforms = append(platforms, platform.ID)
		}
	}
	permissions := []string{}
	for _, permission := range extensions.AllPermissions {
		if a.toolPermissions[permission] {
			permissions = append(permissions, permission)
		}
	}
	draft := CustomTool{
		Name:        strings.TrimSpace(name),
		Mode:        mode,
		Program:     strings.TrimSpace(program),
		Command:     strings.TrimSpace(command),
		Args:        strings.TrimSpace(args),
		VersionArgs: strings.TrimSpace(versionArgs),
		Version:     strings.TrimSpace(version),
		Description: strings.TrimSpace(description),
		Output:      a.toolOutput,
		Permissions: permissions,
		Platforms:   platforms,
		Params:      a.declaredParams(),
	}
	if mode == routeScript {
		draft.ScriptLanguage = language
		draft.ScriptContent = script
	}
	return draft
}

// customIssues is the backend's own rules, read on the draft as it would be
// submitted: the form shows the same sentences the host would refuse with.
func customIssues(tool CustomTool) []extensions.CustomIssue {
	return extensions.CustomIssues(extensions.CustomRequest{
		Name:           tool.Name,
		Mode:           tool.Mode,
		Command:        tool.Command,
		Version:        tool.Version,
		Platforms:      tool.Platforms,
		ScriptLanguage: tool.ScriptLanguage,
		ScriptContent:  tool.ScriptContent,
	})
}

// toolIssueUnder prints the form's own translation of the first issue that
// belongs to one row, once the user has asked to submit. A field with no issue
// draws nothing.
func (a *App) toolIssueUnder(c *ui.Context, copy i18n.Settings, issues []extensions.CustomIssue, field extensions.CustomField) {
	if !a.toolIssue {
		return
	}
	for _, issue := range issues {
		if issue.Field == field {
			a.issueUnder(c, customIssueMessage(copy, issue.Key))
			return
		}
	}
}

// toolContentActions draws the form's content actions, the reference's
// `.extension-custom-form__actions` row (`CustomIntegrationDrawer.tsx:145-146`):
// in script mode one "Copy script" and the "Export script" button that writes
// the same text to a file, in program mode "Copy command" and "Copy execution
// plan". Each copy button confirms on itself for 1.6s, because a clipboard
// write is otherwise silent. The value copied is read at press time from the
// form's current draft, so it is what the user sees now rather than what the
// form opened with.
func (a *App) toolContentActions(c *ui.Context, copy i18n.Settings, mode, command, program, args, script, language string) {
	t := a.cardTheme(c)
	a.actionRow(c, func() {
		ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
			if mode == routeScript {
				a.contentAction(c, "script", copy.LocalToolCopyScript, copy.LocalToolScriptCopied, script)
				a.exportScriptAction(c, copy, command, script, language)
				return
			}
			a.contentAction(c, "command", copy.LocalToolCopyCommand, copy.LocalToolCommandCopied, strings.TrimSpace(command))
			a.contentAction(c, "plan", copy.LocalToolCopyPlan, copy.LocalToolPlanCopied,
				executionPlanJSON(program, args))
		})
	})
}

// exportScriptAction draws the reference's "Export script" button: the second
// half of the script-mode action row, which writes the editor's text to a file
// the user picks (`ExtensionsPanel.tsx:1951-1964`). It is absent when the host
// wired no exporter, and inert while the script is blank — the reference
// mounted it always but the write would have produced an empty file.
func (a *App) exportScriptAction(c *ui.Context, copy i18n.Settings, command, script, language string) {
	if a.Actions.ExportScript == nil || strings.TrimSpace(script) == "" {
		return
	}
	if a.actionButton(c, copy.LocalToolExportScript).Clicked() {
		a.Actions.ExportScript(scriptExportName(a.toolEditing, command, language), script)
	}
}

// scriptExportName is the file name the save dialog offers for a script: the
// reference's `{id}-script.{extension}`, where the id is the integration being
// edited and, for a connection that has no id yet, the command word the user
// typed (`ExtensionsPanel.tsx:1955-1958`: `editingCustomId ?? customIntegration
// .command`). The extension comes from the language table, the same value the
// file filter shows.
func scriptExportName(id, command, language string) string {
	stem := strings.TrimSpace(id)
	if stem == "" {
		stem = strings.TrimSpace(command)
	}
	if stem == "" {
		stem = "floter-provider"
	}
	return stem + "-script." + scriptlang.Extension(language)
}

// contentAction draws one copy button: its label, or its confirmation while
// the window holds, and the press that puts action's value on the clipboard.
// An empty value copies nothing and confirms nothing, the same way the
// reference's buttons disabled on a missing value.
func (a *App) contentAction(c *ui.Context, action, label, done, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	copied := a.contentCopied(action, c.Now())
	if copied {
		c.After(a.copiedContentUntil.Sub(c.Now()))
	}
	if copied {
		label = done
	}
	if a.actionButton(c, label).Clicked() {
		if a.Actions.CopyText != nil {
			a.Actions.CopyText(value)
			a.copiedContent, a.copiedContentUntil = action, c.Now().Add(copiedContentDuration)
		}
	}
}

// copiedContentDuration is how long a content action's button keeps its
// confirmation: the old build's own 1600ms (`showSuccess` then reset), the same
// window the deep-link copy used.
const copiedContentDuration = 1600 * time.Millisecond

// contentCopied reports whether the content action should show its
// confirmation, clearing the state once its window has passed (the reference's
// `customContentOperation` reset in the action's `finally`).
func (a *App) contentCopied(action string, now time.Time) bool {
	if a.copiedContent == "" || a.copiedContent != action {
		return false
	}
	if !now.Before(a.copiedContentUntil) {
		a.copiedContent = ""
		return false
	}
	return true
}

// executionPlanJSON is the plan the reference's `copyExecutionPlan` put on the
// clipboard: the program, its default arguments, and the run mode, pretty-
// printed (`src/ExtensionsPanel.tsx:1947-1950`). It is a preview of what will
// run, never a shell string — the argv is built by the host from these same
// fields (`customRequest`), so the plan can only describe them.
func executionPlanJSON(program, args string) string {
	plan := struct {
		Program string   `json:"program"`
		Args    []string `json:"args"`
		Mode    string   `json:"mode"`
	}{Program: strings.TrimSpace(program), Args: splitPlanArgs(args), Mode: "pty"}
	encoded, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return ""
	}
	return string(encoded)
}

// splitPlanArgs is the plan's own read of the arguments field: the same
// whitespace split the host applies when it builds the argv, but an absent
// field reads as an empty list rather than a null one (the reference spread an
// always-present array into its plan).
func splitPlanArgs(args string) []string {
	fields := strings.Fields(args)
	if fields == nil {
		return []string{}
	}
	return fields
}

// customIssueMessage is the one sentence for one rule, in the stored language.
func customIssueMessage(copy i18n.Settings, key extensions.CustomIssueKey) string {
	switch key {
	case extensions.CustomIssueNameLength:
		return copy.LocalToolNameLength
	case extensions.CustomIssueCommandShape:
		return copy.LocalToolCommandShape
	case extensions.CustomIssueVersionShape:
		return copy.LocalToolVersionShape
	case extensions.CustomIssuePlatformRequired:
		return copy.LocalToolPlatformRequired
	case extensions.CustomIssueModeInvalid:
		return copy.LocalToolModeInvalid
	case extensions.CustomIssueScriptEmpty:
		return copy.LocalToolScriptEmpty
	case extensions.CustomIssueScriptLanguageInvalid:
		return copy.LocalToolScriptLanguageInvalid
	}
	return ""
}

// toolPermissionFields are the form's declaration switches and the boundary
// note above them, drawn only while the Advanced disclosure is open. The note
// is the honest half of the disclosure: the switches record intent and drive
// the audit, they do not sandbox anything — local code runs as the user either
// way (`settings.extensions.permissionBoundary`).
func (a *App) toolPermissionFields(c *ui.Context, copy i18n.Settings) {
	if !a.toolAdvanced {
		return
	}
	if a.toolPermissions == nil {
		a.toolPermissions = map[string]bool{}
	}
	a.row(c, "", copy.PermissionBoundary, func() {})
	// One row per permission, the host's own two marked as the checks they are
	// (see the permission tiers).
	for _, permission := range extensions.AllPermissions {
		permission := permission
		on := a.toolPermissions[permission]
		label := i18n.PermissionLabel(a.Store.Snapshot().Language, permission)
		labelClicked := a.row(c, label, "", func() {
			if a.switchControl(c, &on).Label(label).Changed() {
				a.toolPermissions[permission] = on
			}
		})
		if labelClicked {
			a.toolPermissions[permission] = !on
		}
	}
}

// scriptRows are the rows a script tool shows in place of the program row: the
// language it is written in (with the line saying whether this machine has its
// toolchain), and the editor the script is typed into.
func (a *App) scriptRows(c *ui.Context, copy i18n.Settings, language *string, script *string, issues []extensions.CustomIssue) {
	options := make([]i18n.Option, 0, len(scriptlang.Languages))
	for _, option := range scriptlang.Languages {
		options = append(options, i18n.Option{ID: option.ID, Label: option.Label})
	}
	a.choose(c, copy.LocalToolLanguage, copy.LocalToolLanguageHint, options, *language, func(id string) {
		// The editor owns the text now: switching language only names the new
		// one. The template swap for an untouched draft happens in the
		// editor's own sync (see scriptEditorView), against the buffer it
		// holds — including a switch away from a written script, which keeps
		// it as the old form did.
		*language = id
	})
	a.row(c, copy.LocalToolLanguage, "", func() {
		ui.Text(c, a.scriptRuntimeLine(*language)).FontSize(c.Theme().FontSize - 1).
			TextColor(c.Theme().TextMuted)
	})
	// The editor is a stacked row: a script wants the row's full width and
	// more than one line, so the label and the status sit above it. The box is
	// mygo's code editor (line numbers, syntax highlighting) rather than a bare
	// TextArea; scriptEditorView keeps its state across frames.
	a.stackedRow(c, copy.LocalToolScript, copy.LocalToolScriptHint, func() {
		a.scriptEditorView(c, copy, language, script)
		a.toolIssueUnder(c, copy, issues, extensions.CustomFieldScript)
	})
}

// copy is the settings surface's words in the stored language.
func (a *App) copy() i18n.Settings {
	return i18n.For(a.Store.Snapshot().Language).Settings
}

// runtimeProbe is one language's toolchain probe, off the render thread:
// `done` closes when the probe has landed, and `check` is the verdict it
// found. A probe that has not landed yet is the render's "checking" state —
// the old panel's `null` — never a verdict.
type runtimeProbe struct {
	done  chan struct{}
	check scriptlang.Check
}

// scriptRuntimeStart returns the probe for a language, starting one on a
// goroutine the first time the form asks for it. Probes are deduplicated: a
// redraw while one is in flight — or a save that arrives mid-flight — joins
// the request already running rather than firing a second PATH scan.
func (a *App) scriptRuntimeStart(language string) *runtimeProbe {
	a.scriptRuntimeMu.Lock()
	defer a.scriptRuntimeMu.Unlock()
	if a.scriptRuntime == nil {
		a.scriptRuntime = map[string]*runtimeProbe{}
	}
	if probe, ok := a.scriptRuntime[language]; ok {
		return probe
	}
	probe := &runtimeProbe{done: make(chan struct{})}
	a.scriptRuntime[language] = probe
	epoch := a.scriptRuntimeEpoch
	probeFn := a.Actions.ScriptRuntime
	invalidate := a.Actions.Invalidate
	go func() {
		check := probeFn(language)
		a.scriptRuntimeMu.Lock()
		// A form that closed and reopened while this ran must not inherit the
		// verdict: the machine may have changed (the drawer's `epoch` guard).
		if epoch == a.scriptRuntimeEpoch {
			probe.check = check
		}
		a.scriptRuntimeMu.Unlock()
		close(probe.done)
		if invalidate != nil {
			invalidate()
		}
	}()
	return probe
}

// scriptRuntimeCheck is the muted line's own read of the probe: a landed
// verdict, or nil while one is in flight (or before the form has asked). The
// second result is false when the host wired no probe at all, so the line
// falls back to the toolchain's name alone.
func (a *App) scriptRuntimeCheck(language string) (*scriptlang.Check, bool) {
	if a.Actions.ScriptRuntime == nil {
		return nil, false
	}
	probe := a.scriptRuntimeStart(language)
	select {
	case <-probe.done:
		check := probe.check
		return &check, true
	default:
		return nil, true
	}
}

// scriptRuntimeVerdict is the probe's verdict for a language, waiting for a
// probe that is still in flight. The old form awaited the probe before saving
// so the missing-toolchain warning reflects the machine rather than a race;
// the wait is short because the form opened a frame earlier and the probe has
// almost always landed. A stale (epoch-dropped) probe answers with the zero
// Check, which reads as "not asked" and so raises no warning.
func (a *App) scriptRuntimeVerdict(language string) (scriptlang.Check, bool) {
	if a.Actions.ScriptRuntime == nil {
		return scriptlang.Check{}, false
	}
	probe := a.scriptRuntimeStart(language)
	<-probe.done
	return probe.check, true
}

// clearScriptRuntimeCache drops the verdicts and bumps the epoch: a fresh form
// gets a fresh toolchain check, the same reset the old panel's epoch did. A
// user who installed the runtime since the last form is told to reopen it, so
// reopening is exactly when the cache must not survive — and any probe still
// in flight from the closed form is dropped rather than cached.
func (a *App) clearScriptRuntimeCache() {
	a.scriptRuntimeMu.Lock()
	a.scriptRuntimeEpoch++
	a.scriptRuntime = nil
	a.scriptRuntimeMu.Unlock()
}

// scriptRuntimeLine is the one muted line under the language picker: what this
// machine has for the language. An absent toolchain is deliberately not an
// error — a user may install Python and come back — so it is a sentence, never
// a red banner.
func (a *App) scriptRuntimeLine(language string) string {
	copy := a.copy()
	option := scriptlang.Lookup(language)
	check, wired := a.scriptRuntimeCheck(language)
	if !wired {
		// No probe wired is not a verdict: the toolchain's name is all the form
		// can honestly say.
		return option.Toolchain
	}
	if check == nil {
		// Asked, still in flight (or not asked a frame ago): the old panel's
		// `check === null`, painted as "checking", never as "missing".
		return copy.LocalToolRuntimeChecking
	}
	switch status := scriptlang.StatusOf(language, *check); status.State {
	case scriptlang.Available:
		line := copy.LocalToolRuntimeReady(status.Name, status.Version)
		if status.Compiled {
			line += "  ·  " + copy.LocalToolRuntimeCompiled
		}
		return line
	case scriptlang.Missing:
		return copy.LocalToolRuntimeMissing(strings.Join(status.Names, ", "))
	default:
		// A landed check with no observation cannot claim the toolchain is
		// missing: the toolchain's name is all we can honestly say.
		return option.Toolchain
	}
}

// isTemplate reports whether a script is still one of the shipped templates,
// so a language switch may replace it.
func isTemplate(script string) bool {
	for _, template := range scriptlang.Templates() {
		if script == template {
			return true
		}
	}
	return false
}

// routeBackground is the manifest's own spelling for a command whose output
// the launcher shows instead of the terminal, and routeScript the form's
// spelling for a tool the host writes rather than one it finds.
const (
	routeBackground = "background"
	routeScript     = "script"
)

// customPlatforms are the operating systems a connected tool can be limited
// to, in the order the old form lists them: the id is what the manifest stores
// and the runtime checks, the label is what the row shows.
var customPlatforms = []struct{ ID, Label string }{
	{"darwin", "macOS"},
	{"linux", "Linux"},
	{"windows", "Windows"},
}

// currentPlatform is the platform a fresh form ticks: this machine's own, with
// anything the manifest vocabulary does not name falling back to linux, as the
// old build's CURRENT_PLATFORM did.
func currentPlatform() string {
	for _, platform := range customPlatforms {
		if platform.ID == runtime.GOOS {
			return platform.ID
		}
	}
	return "linux"
}

// CurrentPlatform is the platform a connection request built without the form
// supports: this machine's own, in the manifest vocabulary. The form ticks it
// for a fresh tool, and the shell's detected/register flows — which skip the
// form — need the same default, or the request is refused with
// `platformRequired` before anything is written.
func CurrentPlatform() string { return currentPlatform() }

// freshnessOf reads what is known about an integration's command list, for
// the card's own row. Nothing is shown for an integration nothing has probed.
func (a *App) freshnessOf(integration Integration) (extensions.Freshness, bool) {
	freshness := a.Actions.Freshness
	if freshness == nil {
		return extensions.Freshness{}, false
	}
	value, ok := freshness(integration.ID)
	if !ok {
		return extensions.Freshness{}, false
	}
	return value, true
}

// freshnessState is the row's trailing word: how the last derivation went.
func freshnessState(copy i18n.Settings, freshness extensions.Freshness) string {
	switch freshness.Result {
	case extensions.FreshnessRunning:
		return copy.FreshnessResultRunning
	case extensions.FreshnessSuccess:
		return copy.FreshnessResultSuccess
	case extensions.FreshnessDegraded:
		return copy.FreshnessResultDegraded
	case extensions.FreshnessFailed:
		return copy.FreshnessResultFailed
	default:
		return copy.FreshnessResultUnknown
	}
}

// freshnessDot is the one-word mark beside an integration's name: the same
// projection the freshness row below uses, so a user scanning the cards sees a
// list move without reading every row's timeline. Only a change earns warmth;
// a synced list is a quiet secondary dot and an unknown one is muted.
func (a *App) freshnessDot(c *ui.Context, copy i18n.Settings, freshness extensions.Freshness) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	dot := freshness.DotState()
	color := t.TextMuted
	switch dot {
	case extensions.FreshnessDotSynced:
		color = tokens.TextSecondary
	case extensions.FreshnessDotChanged:
		color = t.Warning
	}
	// The mark is its own label: the tooltip and the accessible name are the
	// same sentence, and the state word inside it comes from a switch over the
	// same value the colour does.
	label := freshnessDotLabel(copy, dot)
	ui.Box(c).Size(t.Space(2), t.Space(2)).Radius(t.Space(1)).Shrink(0).
		Background(color).Label(label).Tooltip(label)
}

// freshnessDotLabel is the mark's whole sentence: the state's own word inside
// the shared "Command status: {state}" frame, so a state added to the
// vocabulary without a word cannot print an empty string.
func freshnessDotLabel(copy i18n.Settings, dot extensions.FreshnessDot) string {
	word := copy.FreshnessDotUnknown
	switch dot {
	case extensions.FreshnessDotSynced:
		word = copy.FreshnessDotSynced
	case extensions.FreshnessDotChanged:
		word = copy.FreshnessDotChanged
	}
	return copy.FreshnessDot(word)
}

// freshnessSection draws the drawer's freshness block at the reference's own
// faces (`ExtensionsPanel.tsx:3076-3176`, `extensions.css:1997-2119`): a block
// label, a two-cell metadata grid, the command-count line, and the secondary
// health-source note. What it replaces was one ruled card row whose grey
// subline carried every fact in the row's own register.
func (a *App) freshnessSection(c *ui.Context, copy i18n.Settings, integration Integration, freshness extensions.Freshness) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	// The section is the extension drawer's block label: the reference's
	// `.extension-detail-block h4` — sentence case at the *secondary* ink,
	// caption 620 (`extensions.css:2008-2013`). It is one voice with the
	// metadata grid's own `dt` labels one line below, not the uppercase 650 in
	// tertiary grey the port had reached for.
	ui.Row(c).FillWidth().Children(func() {
		ui.Text(c, copy.Freshness).FontSize(tokens.Caption).FontWeight(620).
			TextColor(tokens.TextSecondary)
	})
	// The when/result line: the reference's two-column `.extension-metadata` grid
	// (`extensions.css:2022-2050`) — a `dt` label at the *tertiary* caption step
	// and its `dd` value at the *body* step in `--text-secondary`, two cells to a
	// row with an 8u/16u gap pair. The port had flattened the pair into one
	// ruled card row whose grey subline carried both facts in the row's register.
	meta := ui.Row(c).FillWidth().GapX(t.Space(3)).GapY(0)
	meta.Children(func() {
		a.metadataCell(c, copy.FreshnessLastProbe, freshnessWhenValue(copy, freshness))
		a.metadataCell(c, copy.FreshnessResult, freshnessState(copy, freshness))
	})
	// The command-count line: a flex row, not a metadata cell — the reference's
	// `.extension-freshness__commands` (`extensions.css:2077-2111`) puts a
	// tertiary caption label and the count at the row's own *body* step side by
	// side with an 8u gap, then the muted `·` and the caption delta word; the
	// port had folded both facts into the subline's prose.
	count := copy.FreshnessCommandsUnknown
	if freshness.CommandCount != nil {
		count = strconv.Itoa(*freshness.CommandCount)
	}
	line := ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center)
	line.Children(func() {
		ui.Text(c, copy.FreshnessCommands).FontSize(tokens.Caption).
			TextColor(tokens.TextTertiary)
		ui.Text(c, count).FontSize(t.FontSize).FontWeight(600).
			TextColor(tokens.TextStrong)
		// The separator is the reference's own muted `·` between the count and the
		// delta word (`extensions.css:2096-2103`); the delta word itself steps up
		// one register — `--text-secondary` — only when the list actually moved,
		// because a real movement is information and "no change" is not
		// (`:2105-2111`).
		ui.Text(c, "\u00b7").FontSize(tokens.Caption).TextColor(t.TextMuted)
		ui.Text(c, freshnessDeltaWord(copy, freshness)).FontSize(tokens.Caption).
			TextColor(freshnessDeltaInk(freshness, tokens, t)).SingleLine()
	})
	// The health report is the source of the time, not of the count: the
	// reference states that once, as a *secondary* `.extension-detail-note` — one
	// muted caption line with no pane, no rule and no border
	// (`extensions.css:2006-2011,2114-2119`).
	if freshness.Source == extensions.FreshnessHealth {
		ui.Text(c, copy.FreshnessHealthSource).FontSize(tokens.Caption).
			TextColor(t.TextMuted).Wrap()
	}
}

// metadataCell is one `.extension-metadata` cell: its `dt` at the tertiary
// caption step and its `dd` at the body step in the secondary ink
// (`extensions.css:2033-2050`). The reference sized the value one step above
// its own label, which is what keeps the pair from reading as two rows of the
// same register.
func (a *App) metadataCell(c *ui.Context, label, value string) {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		ui.Text(c, label).FontSize(tokens.Caption).TextColor(tokens.TextTertiary)
		ui.Text(c, value).FontSize(t.FontSize).TextColor(tokens.TextSecondary).
			SingleLine()
	})
}

// freshnessWhenValue is the metadata grid's "last probe" value: how long ago
// the list was derived, or the reference's "Never probed" word when nothing
// ever did (`ExtensionsPanel.tsx:3136-3145`).
func freshnessWhenValue(copy i18n.Settings, freshness extensions.Freshness) string {
	if freshness.AtSeconds == 0 {
		return copy.FreshnessNever
	}
	return freshnessWhen(copy, time.Since(time.Unix(freshness.AtSeconds, 0)))
}

// freshnessDeltaWord is the reference's `.extension-freshness__delta-text`
// (`extensions.css:2100-2111`): the one sentence the count moved by, one per
// kind. The count rides the copy's own formatter, so a locale words it itself.
func freshnessDeltaWord(copy i18n.Settings, freshness extensions.Freshness) string {
	switch freshness.Delta {
	case extensions.DeltaIncrease:
		return copy.FreshnessDeltaIncrease(freshness.DeltaCount())
	case extensions.DeltaDecrease:
		return copy.FreshnessDeltaDecrease(freshness.DeltaCount())
	case extensions.DeltaUnchanged:
		return copy.FreshnessDeltaUnchanged
	default:
		return copy.FreshnessDeltaUnknown
	}
}

// freshnessDeltaInk is the delta word's register: `--text-secondary` once the
// list actually moved, `--text-muted` otherwise (`extensions.css:2096-2111`).
func freshnessDeltaInk(freshness extensions.Freshness, tokens theme.Tokens, t *ui.Theme) ui.Color {
	switch freshness.Delta {
	case extensions.DeltaIncrease, extensions.DeltaDecrease:
		return tokens.TextSecondary
	default:
		return t.TextMuted
	}
}

// freshnessDetail is the section's whole detail line, kept for the panel's own
// readers (the shell's tests and diagnostics). The section itself draws the
// facts in their own registers through freshnessSection.
func (a *App) freshnessDetail(copy i18n.Settings, freshness extensions.Freshness) string {
	parts := []string{}
	if freshness.AtSeconds > 0 {
		when := freshnessWhen(copy, time.Since(time.Unix(freshness.AtSeconds, 0)))
		source := copy.FreshnessProbeSource
		if freshness.Source == extensions.FreshnessHealth {
			source = copy.FreshnessHealthSource
		}
		parts = append(parts, when+"  \u00b7  "+source)
	} else {
		parts = append(parts, copy.FreshnessNever)
	}
	commands := copy.FreshnessCommandsUnknown
	if freshness.CommandCount != nil {
		commands = strconv.Itoa(*freshness.CommandCount)
	}
	parts = append(parts, copy.FreshnessCommands+" "+commands)
	switch freshness.Delta {
	case extensions.DeltaIncrease:
		parts = append(parts, copy.FreshnessDeltaIncrease(freshness.DeltaCount()))
	case extensions.DeltaDecrease:
		parts = append(parts, copy.FreshnessDeltaDecrease(freshness.DeltaCount()))
	case extensions.DeltaUnchanged:
		parts = append(parts, copy.FreshnessDeltaUnchanged)
	default:
		parts = append(parts, copy.FreshnessDeltaUnknown)
	}
	return strings.Join(parts, "  \u00b7  ")
}

// freshnessWhen renders how long ago a probe ran. Under 45 seconds it is the
// caller's own "just now" word — every locale words that differently enough
// that a table of our own would be worse than one key — and past it the
// localized relative time ("3 min. ago" / "3分钟前").
func freshnessWhen(copy i18n.Settings, age time.Duration) string {
	if age < 45*time.Second {
		return copy.FreshnessJustNow
	}
	value, unit := relativeAge(age)
	return copy.FreshnessRelative(value, unit)
}

// relativeAge picks the largest unit whose threshold the age is under and
// rounds to nearest, exactly the thresholds the old build's `relativeTime`
// used: minutes under an hour, hours under a day, then days, months and
// years. The rounding is what turns 50s into "1 min." and 25h into
// "yesterday".
func relativeAge(age time.Duration) (int, i18n.RelativeUnit) {
	seconds := age.Seconds()
	switch {
	case seconds < 3600:
		return int(math.Round(seconds / 60)), i18n.RelativeMinute
	case seconds < 86400:
		return int(math.Round(seconds / 3600)), i18n.RelativeHour
	case seconds < 2592000:
		return int(math.Round(seconds / 86400)), i18n.RelativeDay
	case seconds < 31536000:
		return int(math.Round(seconds / 2592000)), i18n.RelativeMonth
	default:
		return int(math.Round(seconds / 31536000)), i18n.RelativeYear
	}
}

// DetectedTool is one program the machine has that is not yet connected.
type DetectedTool struct {
	// Name is what the user calls it and Path where it lives.
	Name        string
	Path        string
	Description string
	// VersionArgs is how the tool reports its version, when the caller knows
	// better than the default.
	VersionArgs string
	// RuntimeAvailable reports whether the program is runnable right now: the
	// reference's `runtimeAvailable`, which decides between Connect and the two
	// install arms (`src/extensions/ExtensionRow.tsx:296-302`).
	RuntimeAvailable bool
	// InstallCommand is the recipe command that would install this tool with a
	// package manager the machine has, empty when the catalog does not know the
	// tool or has no recipe for this platform (`src/extensions/tool-rows.ts`'s
	// `installCommandForId`).
	InstallCommand string
}

// detectedSection lists the programs this machine has that nothing has
// connected yet, each with the one press that connects it: discovery is the
// first link of the same loop the install and invoke rows close, so a tool
// the machine already has does not need its path typed by hand.
func (a *App) detectedSection(c *ui.Context, copy i18n.Settings) {
	if a.Actions.Detected == nil || a.Actions.ConnectDetected == nil {
		return
	}
	tools := a.Actions.Detected()
	if len(tools) == 0 {
		return
	}
	a.extensionSection(c, copy.DetectedTitle, len(tools), func() {
		a.extensionHint(c, copy.DetectedHint, true)
		a.card(c, func() {
			for _, tool := range tools {
				tool := tool
				a.iconRow(c, true, tool.Name, tool.Description, func() {
					a.detectedAction(c, copy, tool)
				}, func() {
					a.row(c, tool.Path, "", func() {})
				})
			}
		})
	})
}

// detectedActionKind is which of the reference's three arms a detected row's
// one button takes: the program is runnable and connects, it is not runnable
// but a recipe exists and opens the install command in a terminal, or floter
// has nothing and the button says so disabled
// (`src/extensions/ExtensionRow.tsx:296-302`).
type detectedActionKind int

const (
	detectedConnect detectedActionKind = iota
	detectedInstallInTerminal
	detectedInstallFirst
)

// detectedActionFor picks the arm from the row's own facts. RuntimeAvailable
// wins: a runnable program is connected, never reinstalled.
func detectedActionFor(tool DetectedTool) (kind detectedActionKind, disabled bool) {
	switch {
	case tool.RuntimeAvailable:
		return detectedConnect, false
	case tool.InstallCommand != "":
		return detectedInstallInTerminal, false
	default:
		return detectedInstallFirst, true
	}
}

// detectedLabel words the arm: the reference put the same three words on the
// button's visible label and its accessible name.
func detectedLabel(copy i18n.Settings, kind detectedActionKind) string {
	switch kind {
	case detectedInstallInTerminal:
		return copy.IntegrationsInstallInTerminal
	case detectedInstallFirst:
		return copy.IntegrationsInstallTool
	default:
		return copy.IntegrationsConnect
	}
}

// detectedAction draws the row's button and runs the arm it chose. A row with
// nothing to offer still draws its word, disabled, rather than vanishing — the
// reference's disabled button was the only sign there was nothing to press.
func (a *App) detectedAction(c *ui.Context, copy i18n.Settings, tool DetectedTool) {
	kind, disabled := detectedActionFor(tool)
	if kind == detectedInstallInTerminal && a.Actions.InstallDetected == nil {
		disabled = true
	}
	button := a.actionButton(c, detectedLabel(copy, kind)).Disabled(disabled)
	if !button.Clicked() {
		return
	}
	switch kind {
	case detectedInstallInTerminal:
		a.Actions.InstallDetected(tool.InstallCommand)
	case detectedConnect:
		a.Actions.ConnectDetected(tool)
	}
}

// requestDisable opens the confirmation for turning an integration off.
func (a *App) requestDisable(id string) {
	if a.disableConfirm == nil {
		a.disableConfirm = map[string]bool{}
	}
	if a.disableConfirmAt == nil {
		a.disableConfirmAt = map[string]time.Time{}
	}
	a.disableConfirm[id] = true
	a.disableConfirmAt[id] = a.now()
}

// barButton is the compact inline button the reference drew inside a notice
// band (`.extension-discard-bar .extensions-action-button`,
// `src/styles/extensions.css:2473-2481`): the panel's 30px pill is overridden
// to a 24px ladder step — height 24px, padding 0 8px, caption font, the
// radius-xs corner — so the answers read as part of the band rather than as
// floating controls. Hover lifts the plane to the control-hover step; the
// hairline stays the neutral `ControlEdge` (P8-d: it used to rise to the accent
// keyline, which §5 forbids on hover). A disabled button keeps
// its resting face and ignores the press.
// barButton is the reference's inline action button inside a notice band
// (`extensions.css:1313-1385`): a 24px pill with 8px of side padding, a
// hairline border, the control plane and no fill — the same shape every
// discard bar's answers share. The danger variant is the ink alone
// (`--text-warning`), not a red plane: the reference's destructive action is
// quieter than the framework's filled Button. Hover lifts the plane to
// `--glass-control-hover`, and the ink
// moves to `--text-primary` for both the neutral and the danger button — the
// reference's own `:hover` rule applies to `--danger` too. The icon, when
// one is passed, leads the word: the reference's confirm buttons are
// `<Trash2 size={14} />` / `<Unplug size={14} />` / `<PowerOff size={14} />`
// before their label, so the mark a destructive confirm is recognized by
// comes before the word it confirms.
func (a *App) barButton(c *ui.Context, label string, danger, disabled bool, icon *ui.SVG, run func()) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	ink := tokens.TextStrong
	edge := tokens.ControlEdge
	plane := tokens.Control
	if danger {
		ink = t.Warning
	}
	button := ui.ButtonBase(c).
		Height(t.Space(6)).
		Padding(0, t.Space(2)).
		Radius(tokens.RadiusXS).
		Gap(t.Space(1.25)).
		Background(plane).Border(1, edge)
	if !disabled {
		if button.Hovered() {
			// P8-d 重定基线（§5 禁止侧「行 hover/按下」）：hover 边曾是
			// `tokens.AccentEdge`。这是同族的最后一处尾巴（`rows.go` 的
			// `extensionActionFace`、`uninstall.go` 的两个 pill 与本条是同一
			// 个 `.extensions-action-button` hover 规则）——accent 不能回答
			// 「指针在这条上」，边保持 `ControlEdge`，只抬面与墨。
			plane = tokens.ControlHover
			edge = tokens.ControlEdge
			ink = t.Text
			button = button.Background(plane).Border(1, edge)
		}
	}
	// `.extensions-action-button:active:not(:disabled) { transform: scale(0.97) }`
	// (`extensions.css:138-140`): the press sinks the pill a hair, over the
	// same `--dur-2` transform transition. The disabled pill keeps its resting
	// shape, exactly as the dialog's own action pill does.
	if !disabled && button.Pressed() {
		button.Scale(0.97, 0.97)
	}
	// `.extension-discard-bar .extensions-action-button` only overrides the
	// size, padding, radius and font; the rest is the shared
	// `.extensions-action-button` face (`extensions.css:1313-1385`): the
	// `--elev-0` lens, the 5px `--extensions-inline-gap`, 600 weight at
	// `line-height: 1`, and the `--dur-2` colour transition. The 24px button
	// was a flat tint without them.
	// `.extensions-action-button` transitions only its colours and its
	// `transform` (`extensions.css:1331`: `background-color`, `border-color`,
	// `color`, `transform`), never its place in the parent. The old port also
	// asked for a position transition, which made a newly built pill slide
	// from where its predecessor stood — the disable band's pills drifted up
	// under a sentence that had already settled, so a press aimed at the pill
	// could miss it mid-flight.
	button.Transition(ui.ElementTransition{Colors: true, Duration: 120 * time.Millisecond})
	tokens.Lens(button)
	button.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(t.Space(3.5), t.Space(3.5))
		}
		ui.Text(c, label).FontSize(tokens.Caption).FontWeight(600).TextColor(ink).LineHeight(1)
	})
	if disabled {
		// `:disabled { opacity: 0.4; cursor: default; pointer-events: none }`:
		// the framework's flag carries the pointer half, the flatness and the
		// cursor are the two it does not paint.
		button = button.Disabled(true).Opacity(0.4).Cursor(ui.CursorDefault)
	}
	if !disabled && button.Clicked() {
		run()
	}
	return button
}

// noticeBand is the reference's inline confirmation band — the one shape
// `.extensions-notice .extension-discard-bar` gave every question asked in
// place (`src/styles/extensions.css:146-172,2455-2458`): a leading alert mark,
// then a single sentence that takes the free space, then the answers on the
// same row. Both inline questions are this band, and only the ink differs: a
// removal is a loss, so it wears the error tint — `extensions-notice--error`
// keeps the same control pane but paints the ink and the keyline in
// `--text-warning`, and its danger button is warning-coloured
// (`.extensions-action-button--danger`). The band used to be a stacked card
// here — bold title over a muted body with the buttons below — which is not a
// shape the reference ever drew.
//
// The band's own spacing is the reference's: an 8px gap between the mark, the
// sentence and the answers (`.extensions-notice { gap: 8px }`), and a 1.45
// line-height on the sentence (`.extensions-notice { line-height: 1.45 }`) —
// the air a single line of body text carries in every notice the reference
// draws.
//
// It returns the Cancel button so the caller can focus it — the reference's
// own `useEffect` put focus on the Cancel button the moment the band appeared
// (`RemovalConfirmation.tsx:17-18`), so the first press of Enter or Escape
// answers the question without the pointer moving.
func (a *App) noticeBand(c *ui.Context, danger bool, sentence string, answers func(*ui.Element)) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	ink := tokens.TextSecondary
	if danger {
		ink = t.Warning
	}
	var cancel ui.Element
	band := ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).
		Padding(t.Space(2), t.Space(2.5)).Margin(0, t.Space(3)).
		Radius(tokens.RadiusSM).Background(tokens.Control).Border(1, tokens.ControlEdge)
	// `.extensions-notice--error` keeps the band's `--ext-border` and layers
	// the warning as its own inset keyline (`extensions.css:174-178`), rather
	// than recolouring the border: the error band is the neutral pane with an
	// amber ring, not a second edge.
	if danger {
		band.InsetShadow(0, 0, 0, 1, t.Warning)
	}
	band.Children(func() {
		ui.Icon(c, mustGlyph("alert")).Size(t.Space(3.75), t.Space(3.75)).
			TextColor(ink)
		// The question is one flowing sentence, not a bold title over a
		// muted body: the band speaks in one ink throughout.
		ui.Text(c, sentence).FontSize(t.FontSize).TextColor(ink).Grow(1).
			LineHeight(1.45)
		answers(&cancel)
	})
	return cancel
}

// disableConfirmation is the notice under an integration's switch: stopping it
// is a mark, not a delete — the files and data stay, and it can be turned back
// on whenever the user likes — but whatever it is running stops at once, so
// the question is asked rather than assumed. It is the reference's own
// horizontal band, not a stacked card (`DisableConfirmation.tsx:60-68`): a
// leading alert mark, then one sentence holding the title and the consequence
// together (`{disableTitle} {disableDescription}`), then Cancel and the Disable
// action on the same row.
//
// The confirm pill carries the reference's own `disabled={busy}`
// (`DisableConfirmation.tsx:67`) while its Cancel key never does (`:65`):
// pressing Disable marks the band in flight and the band stays up until the
// shell reports back, so a slow or failed write is visible rather than assumed,
// but the question can still be answered with "no" at any time. The reference
// awaited `runMutation` and only then cleared `disableTarget`
// (`src/ExtensionsPanel.tsx:1274-1283`); here the same hand-off is the `done`
// callback the shell calls on the main thread.
func (a *App) disableConfirmation(c *ui.Context, copy i18n.Settings, integration Integration) {
	id := integration.ID
	busy := a.disableBusy[id]
	sentence := copy.DisableTitle(integration.Name) + " " + copy.DisableDescription
	if message := a.disableError[id]; message != "" {
		// A failed write stays on the band, in the family's own error register:
		// the reference reached the user through `showError` while the band it
		// kept mounted said nothing (`src/ExtensionsPanel.tsx:1274-1283`).
		sentence = message
	}
	cancel := a.noticeBand(c, false, sentence, func(cancel *ui.Element) {
		*cancel = a.barButton(c, copy.IntegrationsCancel, false, false, nil, func() {
			a.disableConfirm[id] = false
		})
		a.barButton(c, copy.DisableConfirm, false, busy, mustGlyph("power"), func() {
			if a.Actions.SetIntegrationEnabled == nil {
				a.disableConfirm[id] = false
				return
			}
			a.markDisableBusy(id)
			a.Actions.SetIntegrationEnabled(id, false, func(err error) {
				a.finishDisableConfirmation(id, err)
			})
		})
	})
	// The reference's useEffect focuses the Cancel button on open, so the
	// first Enter or Escape answers the question without the pointer moving.
	// A zero-value Element's Focus is a no-op, so no nil check is needed.
	cancel.Focus()
}

// markDisableBusy records that a disable is in flight and drops the last
// failure, so the band's busy face speaks for the new attempt rather than the
// old one.
func (a *App) markDisableBusy(id string) {
	if a.disableBusy == nil {
		a.disableBusy = map[string]bool{}
	}
	if a.disableError != nil {
		delete(a.disableError, id)
	}
	a.disableBusy[id] = true
}

// finishDisableConfirmation settles the disable band after the shell reports
// back. A nil error is the reference's success arm: the band closes (the
// reference dropped `disableTarget` only after the await). A non-nil error
// keeps the band up with the failure on it and the answer pill usable again —
// the reference's `runMutation` cleared `busy` in `finally`, but only a
// successful write dismissed the band (`src/ExtensionsPanel.tsx:1274-1283`).
func (a *App) finishDisableConfirmation(id string, err error) {
	if a.disableBusy != nil {
		delete(a.disableBusy, id)
	}
	if err != nil {
		if a.disableError == nil {
			a.disableError = map[string]string{}
		}
		a.disableError[id] = err.Error()
	} else {
		if a.disableError != nil {
			delete(a.disableError, id)
		}
		if a.disableConfirm == nil {
			a.disableConfirm = map[string]bool{}
		}
		a.disableConfirm[id] = false
	}
	if a.Actions.Invalidate != nil {
		a.Actions.Invalidate()
	}
}

// requestRemove opens the confirmation for deleting an integration.
func (a *App) requestRemove(id string) {
	if a.removeConfirm == nil {
		a.removeConfirm = map[string]bool{}
	}
	if a.removeConfirmAt == nil {
		a.removeConfirmAt = map[string]time.Time{}
	}
	a.removeConfirm[id] = true
	a.removeConfirmAt[id] = a.now()
}

// confirmWindow is how long an inline confirmation waits for its answer: the
// reference armed both bars with `useTimedReset(removalTarget, …)` /
// `useTimedReset(disableTarget, …)` and its default timeout of 3000 ms
// (`src/hooks/useTimedReset.ts:3`, `src/ExtensionsPanel.tsx:864-865`). An ask
// one moment is not still armed on a later glance.
const confirmWindow = 3 * time.Second

// expireConfirmations withdraws any inline question that has waited longer than
// its window, and keeps the frames coming while one is live so the deadline is
// reached on a still page exactly as it is with a hand on it. A band whose
// answer is already in flight is left alone: the reference's timer was cleared
// on unmount, but its own `busy` face outlives a press that has not landed yet.
func (a *App) expireConfirmations(c *ui.Context) {
	now := a.now()
	live := false
	for id, at := range a.disableConfirmAt {
		if !a.disableConfirm[id] {
			delete(a.disableConfirmAt, id)
			continue
		}
		if now.Sub(at) > confirmWindow && !a.disableBusy[id] {
			a.disableConfirm[id] = false
			delete(a.disableConfirmAt, id)
			continue
		}
		live = true
	}
	for id, at := range a.removeConfirmAt {
		if !a.removeConfirm[id] {
			delete(a.removeConfirmAt, id)
			continue
		}
		if now.Sub(at) > confirmWindow && !a.removeBusy[id] {
			a.removeConfirm[id] = false
			delete(a.removeConfirmAt, id)
			continue
		}
		live = true
	}
	// The configuration form's fold-away question carries the same clock: the
	// reference armed it with the identical `useTimedReset` and 3000 ms default
	// (`ExtensionsPanel.tsx:866`).
	for id, at := range a.configDiscardAt {
		if !a.configDiscard[id] {
			delete(a.configDiscardAt, id)
			continue
		}
		if now.Sub(at) > confirmWindow {
			delete(a.configDiscard, id)
			delete(a.configDiscardAt, id)
			continue
		}
		live = true
	}
	if live && c != nil {
		c.AnimationFrame()
	}
}

// sessionCloseWindow is how long a session's close question waits for its
// answer: the reference armed it with `KILL_CONFIRM_TIMEOUT = 3000`
// (`src/settings/SessionsPage.tsx:10`) and its `armKill` timer cleared the
// question when the window passed. An ask one moment is not still armed on a
// later glance.
const sessionCloseWindow = 3 * time.Second

// anySessionArmed reports whether a session's two-step close is waiting for
// its answer, and disarmSessionClose answers it with "keep" — Escape's job
// there, as the reference's SessionsPage capture listener did (`disarmKill`):
// the key backs out of the question rather than the page.
func (a *App) anySessionArmed() bool {
	return a.armedSession != 0
}

// armSessionClose asks a session's close question and starts its clock. It is
// keyed on the session's stable ID, not its position, so a session that moved
// while the question was up is still the one the answer ends. The reference's
// `armKill` reset the pending timer on every arm, so arming one session drops
// any other (see `sessionCloseWindow`).
func (a *App) armSessionClose(id uint64) {
	a.armedSession, a.armedSessionAt = id, a.now()
}

// sessionInList reports whether any listed session carries the given identity:
// the row that owns an armed question must still be there to show it.
func sessionInList(sessions []Session, id uint64) bool {
	for i := range sessions {
		if sessions[i].ID == id {
			return true
		}
	}
	return false
}

// disarmSessionClose drops the question and its clock in one place, so the two
// can never drift apart.
func (a *App) disarmSessionClose() {
	a.armedSession, a.armedSessionAt = 0, time.Time{}
}

// expireSessionClose drops a question that has waited longer than its window,
// and keeps the frames coming while one is live so the deadline is reached on
// a still page exactly as it is with a hand on it.
func (a *App) expireSessionClose(c *ui.Context) {
	if a.armedSessionAt.IsZero() {
		return
	}
	if a.now().Sub(a.armedSessionAt) > sessionCloseWindow {
		a.disarmSessionClose()
		return
	}
	if c != nil {
		c.AnimationFrame()
	}
}

// anyConfirmOpen reports whether a disable or removal bar is up anywhere on
// the page, and cancelConfirmations answers them all with "no" — Escape's
// job while a question is open (the reference's document-level listener with
// `stopImmediatePropagation`): the key cancels the question, it does not
// leave the page too.
func (a *App) anyConfirmOpen() bool {
	for _, open := range a.disableConfirm {
		if open {
			return true
		}
	}
	for _, open := range a.removeConfirm {
		if open {
			return true
		}
	}
	for _, open := range a.uninstallOpen {
		if open {
			return true
		}
	}
	return false
}

// cancelConfirmations closes every open confirmation without acting on it.
func (a *App) cancelConfirmations() {
	for id := range a.disableConfirm {
		a.disableConfirm[id] = false
	}
	for id := range a.removeConfirm {
		a.removeConfirm[id] = false
	}
	for id := range a.uninstallOpen {
		a.uninstallOpen[id] = false
	}
}

// removeConfirmation is the bar under an integration whose destructive button
// was pressed: deleting files is not a mark — the package is gone from the
// machine — so it is asked, in the same inline band the disable bar uses
// (`RemovalConfirmation.tsx:38-44`), wearing the error tint and ending in the
// kind's own danger button. The question is the kind's own: the reference read
// its title through the same per-kind `removalTextKey` that named the menu item
// (`ExtensionsPanel.tsx:2140-2151`), and the confirm button is that kind's verb
// too — a system card that says "Disconnect" asks "Disconnect?", never
// "Uninstall?".
func (a *App) removeConfirmation(c *ui.Context, copy i18n.Settings, integration Integration) {
	id := integration.ID
	// The reference's confirm button leads with the kind's own mark: a system
	// integration is unplugged (`<Unplug size={14} />`), every other kind is
	// trashed (`<Trash2 size={14} />`) — `RemovalConfirmation.tsx:47`.
	confirmIcon := mustGlyph("trash")
	if integration.removalKind() == RemovalSystem {
		confirmIcon = mustGlyph("unplug")
	}
	// The componentized door is the reference's fork, not a failure: it opens
	// the dialog without marking this band busy, because the answer to the
	// question the band asked has already landed. The reference's own bar only
	// ever ran `disabled={busy}` when it ran the mutation itself
	// (`RemovalConfirmation.tsx:44`), and its Cancel key never carried the flag
	// at all (`:43`) — a question you cannot cancel while it is being answered
	// is a different question.
	componentized := integration.componentizedRemoval() && a.Actions.UninstallComponents != nil
	busy := a.removeBusy[id] && !componentized
	sentence := integration.removalSentence(copy)
	if message := a.removeError[id]; message != "" {
		// A failed removal stays on the band, in the family's own error
		// register; the reference reached the user through `showError` while
		// the band it kept mounted said nothing
		// (`src/ExtensionsPanel.tsx:1296-1304`).
		sentence = message
	}
	cancel := a.noticeBand(c, true, sentence, func(cancel *ui.Element) {
		*cancel = a.barButton(c, copy.IntegrationsCancel, false, false, nil, func() {
			a.removeConfirm[id] = false
		})
		a.barButton(c, integration.removalLabel(copy), true, busy, confirmIcon, func() {
			// The reference's confirm branches by kind
			// (`src/ExtensionsPanel.tsx:1286-1304`): npm and custom go on to
			// the componentized dialog, which asks which data to keep before
			// anything moves; a system-owned built-in and a linked local
			// package have nothing of floter's to pick over, so their confirm
			// runs the plain removal.
			if componentized {
				a.removeConfirm[id] = false
				if a.uninstallOpen == nil {
					a.uninstallOpen = map[string]bool{}
				}
				// A fresh ask starts clean: last time's failure does not ride the
				// new dialog (`uninstallError` is only about the open one).
				if a.uninstallError != nil {
					delete(a.uninstallError, id)
				}
				a.uninstallOpen[id] = true
				return
			}
			if a.Actions.UninstallIntegration == nil {
				a.removeConfirm[id] = false
				return
			}
			a.markRemoveBusy(id)
			a.Actions.UninstallIntegration(id, integration.Name, integration.removalKind(), func(err error) {
				a.finishRemoveConfirmation(id, err)
			})
		})
	})
	// The reference's useEffect focuses the Cancel button on open, so the
	// first Enter or Escape answers the question without the pointer moving.
	// A zero-value Element's Focus is a no-op, so no nil check is needed.
	cancel.Focus()
}

// markRemoveBusy records that a plain removal is in flight and drops the last
// failure, so the band's busy face speaks for the new attempt rather than the
// old one.
func (a *App) markRemoveBusy(id string) {
	if a.removeBusy == nil {
		a.removeBusy = map[string]bool{}
	}
	if a.removeError != nil {
		delete(a.removeError, id)
	}
	a.removeBusy[id] = true
}

// finishRemoveConfirmation settles the removal band after the shell reports
// back. A nil error is the reference's success arm: the band closes. A
// non-nil error keeps the band up with the failure on it and the answer pill
// usable again (`src/ExtensionsPanel.tsx:1296-1304`).
func (a *App) finishRemoveConfirmation(id string, err error) {
	if a.removeBusy != nil {
		delete(a.removeBusy, id)
	}
	if err != nil {
		if a.removeError == nil {
			a.removeError = map[string]string{}
		}
		a.removeError[id] = err.Error()
	} else {
		if a.removeError != nil {
			delete(a.removeError, id)
		}
		if a.removeConfirm == nil {
			a.removeConfirm = map[string]bool{}
		}
		a.removeConfirm[id] = false
	}
	if a.Actions.Invalidate != nil {
		a.Actions.Invalidate()
	}
}

// installFolderRow installs a package directory the user points at: the same
// local install every other tool goes through, with the permission review
// intact. It is the package form's door, where the local-tool form is the
// program's.
func (a *App) installFolderRow(c *ui.Context, copy i18n.Settings) {
	if a.Actions.InstallFromFolder == nil || a.Actions.ChooseFolder == nil {
		return
	}
	a.card(c, func() {
		a.row(c, copy.InstallFolderLabel, copy.InstallFolderHint, func() {
			if a.actionButton(c, copy.InstallFolderChoose).Clicked() {
				if dir, err := a.Actions.ChooseFolder(); err == nil && dir != "" {
					a.Actions.InstallFromFolder(dir)
				}
			}
		})
	})
}

// RunResult is one background run's captured output, kept for the
// integration's row: the two streams as they were captured (the block joins
// them without a separator, the same concatenation the old panel rendered),
// whether a stream hit the retention cap, and whether the run completed
// successfully.
type RunResult struct {
	Stdout    string
	Stderr    string
	Truncated bool
	Success   bool
}

// Text is the captured output as the block reads it: standard output and
// standard error in the order they were captured, joined as-is.
func (r RunResult) Text() string { return r.Stdout + r.Stderr }

// HasOutput reports whether the run said anything at all, which is what the
// block's empty state and the completion toast both key off.
func (r RunResult) HasOutput() bool { return r.Stdout != "" || r.Stderr != "" }

// SetRunResult records an integration's last background run for its row's
// output block. A run that completed records the output and leaves the block
// folded; one that failed unfolds it, so the reason is one glance away rather
// than behind a click (the old panel's run handler did the same).
func (a *App) SetRunResult(id string, result RunResult) {
	if a.runOutputs == nil {
		a.runOutputs = map[string]RunResult{}
	}
	a.runOutputs[id] = result
	if !result.Success {
		a.setRunOutputOpen(id, true)
	}
}

// LastRun is an integration's recorded run and whether there is one, for a
// caller outside the panel (the shell's own tests and diagnostics).
func (a *App) LastRun(id string) (RunResult, bool) {
	result, ok := a.runOutputs[id]
	return result, ok
}

// setRunOutputOpen opens or closes one integration's output block, keeping the
// two maps in step so the block and its toggle can never disagree.
func (a *App) setRunOutputOpen(id string, open bool) {
	if a.outputOpen == nil {
		a.outputOpen = map[string]bool{}
	}
	a.outputOpen[id] = open
}

// runOutputScroll is the fixed-height output scroller's own offset for one
// integration, created on first use.
func (a *App) runOutputScroll(id string) *ui.ScrollState {
	if a.outputScroll == nil {
		a.outputScroll = map[string]*ui.ScrollState{}
	}
	state, ok := a.outputScroll[id]
	if !ok {
		state = &ui.ScrollState{}
		a.outputScroll[id] = state
	}
	return state
}

// runForm is the run-time parameter form for an integration that declares its
// inputs: one control per parameter, the values collected here, and the Run
// button. No value is ever turned into a command string here — the map goes to
// the shell, which builds the argv — so a value with a space or a quote
// reaches the tool exactly as typed.
func (a *App) runForm(c *ui.Context, copy i18n.Settings, integration Integration) {
	if a.Actions.RunWithParams == nil || len(integration.Params) == 0 {
		return
	}
	// The form is not resident: the row's Run button unfolds it (the
	// reference's `runFormOpen`), so an integration with inputs does not wear
	// a form until the user asks to run it.
	if !a.runOpen[integration.ID] {
		return
	}
	params := make([]extensions.ParamDefinition, 0, len(integration.Params))
	for _, param := range integration.Params {
		params = append(params, extensions.ParamDefinition{
			ID: param.ID, Label: param.Label, Kind: param.Kind,
			Default: param.Default, Required: param.Required,
			Placeholder: param.Placeholder, Options: param.Options, Flag: param.Flag,
		})
	}
	if a.runValues == nil {
		a.runValues = map[string]map[string]string{}
	}
	values, ok := a.runValues[integration.ID]
	if !ok {
		values = extensions.ParamDefaults(params)
		a.runValues[integration.ID] = values
	}
	// The form's own validation, recomputed every frame: the reasons are shown
	// under the inputs that caused them, and the Run button is disabled while
	// any remains — the old form's live `paramValueIssues`, so a typo is never
	// a round-trip to the backend to be explained.
	issues := extensions.ParamIssues(params, values)
	issueByIndex := make(map[int]extensions.ParamValueIssue, len(issues))
	for _, issue := range issues {
		issueByIndex[issue.Index] = issue
	}
	theme := a.cardTheme(c)
	// The form is a `below` block of the Run row, drawn on the card's content
	// column: it carries the row's own inset so the heading, the input rows'
	// card and the Run actions all start on the row labels' column instead of
	// flush against the card's edge (the reference's `.extension-run-params`
	// box, `extensions.css:1224-1236`, sits inside the row's own padding).
	inset := a.rowInset(theme)
	ui.Column(c).FillWidth().Padding(0, inset, 0, inset).Children(func() {
		// The form's own frame: the heading and hint the reference drew above the
		// inputs (`RunParamForm.tsx`: `.extension-run-params__head` +
		// `customRunValuesHint`), so a user meets the form as a named block rather
		// than as rows that appear under the commands.
		ui.Column(c).FillWidth().Gap(theme.Space(0.5)).Children(func() {
			ui.Text(c, copy.RunValues).FontSize(theme.FontSize + 1).FontWeight(600).
				TextColor(a.tokens(c).TextStrong)
			ui.Text(c, copy.RunValuesHint).FontSize(theme.FontSize - 1).TextColor(theme.TextMuted)
		})
		a.runParamsBox(c, func() {
			for index, param := range integration.Params {
				index, param := index, param
				label := param.Label
				if label == "" {
					label = param.ID
				}
				// The row's second line is the parameter's *kind* (the reference's
				// `customParamType.*` `<small>`), with the required marker beside it;
				// the placeholder belongs to the input, where the reference put it.
				sublabel := runParamKindLabel(copy, param.Kind)
				if param.Required {
					sublabel = strings.TrimSpace(sublabel + "  ·  " + copy.RunRequired)
				}
				// The row's own reason, printed under it inside the card.
				below := []func(){}
				if issue, ok := issueByIndex[index]; ok {
					below = append(below, func() {
						ui.Text(c, paramValueIssueMessage(copy, issue)).
							FontSize(theme.FontSize-1).TextColor(theme.Danger).
							Margin(theme.Space(1), theme.Space(3.5))
					})
				}
				switch param.Kind {
				case extensions.ParamBoolean:
					on := truthyValue(values[param.ID])
					labelClicked := a.row(c, label, sublabel, func() {
						if a.switchControl(c, &on).Label(label).Changed() {
							values[param.ID] = boolValue(on)
						}
					}, below...)
					if labelClicked {
						values[param.ID] = boolValue(!on)
					}
				case extensions.ParamSelect:
					index := 0
					for i, option := range param.Options {
						if option == values[param.ID] {
							index = i
						}
					}
					a.row(c, label, sublabel, func() {
						if a.segmentedControl(c, &index, param.Options...).Changed() &&
							index >= 0 && index < len(param.Options) {
							values[param.ID] = param.Options[index]
						}
					}, below...)
				default:
					value := values[param.ID]
					a.row(c, label, sublabel, func() {
						a.extensionField(c, &value, configFieldMetrics).Label(label).Width(220).Placeholder(param.Placeholder)
					}, below...)
					values[param.ID] = value
				}
			}
		})
		// The form's footnote: the injection note (values are argv, never a shell
		// line) and the route the manifest already declares, stated before the Run
		// button so the user knows where the output lands — the reference's
		// `customParamInjectionNote` and its `customRunOutputHint*` pair.
		ui.Text(c, copy.RunInjectionNote).FontSize(theme.FontSize - 1).TextColor(theme.TextMuted)
		outputHint := copy.RunOutputHintBackground
		if integration.OutputMode == extensions.RouteTerminal {
			outputHint = copy.RunOutputHintTerminal
		}
		ui.Text(c, outputHint).FontSize(theme.FontSize - 1).TextColor(theme.TextMuted)
		// The reference's `.extension-run-params__actions` is a right-aligned
		// flex row (justify-content: flex-end, a 8px gap) with Cancel first and
		// the accent-filled Run last (`RunParamForm.tsx:129-145`;
		// `extensions.css:1300-1305`) — the affirmative action ends the line,
		// not begins it.
		ui.Row(c).FillWidth().Gap(theme.Space(2)).AlignItems(ui.Center).Justify(ui.End).Children(func() {
			// Cancel folds the form away without running (the reference's
			// `customRunCancel`). The answers stay in `runValues`, so reopening the
			// form keeps the user's in-progress edits — the reference's
			// `runParamMemory`, session memory seeded only on a fresh open.
			if a.actionButton(c, copy.RunCancel).Clicked() {
				a.runOpen[integration.ID] = false
				a.runError = ""
			}
			run := a.extensionPrimaryButton(c, copy.RunLabel)
			if len(issues) > 0 {
				run = run.Disabled(true)
			}
			if run.Clicked() && len(issues) == 0 {
				a.runError = ""
				if failure := a.Actions.RunWithParams(integration.ID, values); failure != nil {
					a.runError = paramFailureMessage(copy, params, failure)
				}
			}
		})
		if a.runError != "" {
			ui.Text(c, a.runError).FontSize(theme.FontSize - 1).TextColor(theme.Danger)
		}
	})
}

// runParamKindLabel is one parameter kind's own word, the reference's
// `customParamType.*` labels (`RunParamForm.tsx`'s `kindLabelKey`). It is the
// run form's second line and the same five words the parameter editor's picker
// offers, so the two surfaces never disagree about what a kind is called.
func runParamKindLabel(copy i18n.Settings, kind string) string {
	switch kind {
	case extensions.ParamNumber:
		return copy.ParamsKindNumber
	case extensions.ParamBoolean:
		return copy.ParamsKindBoolean
	case extensions.ParamSelect:
		return copy.ParamsKindSelect
	case extensions.ParamPath:
		return copy.ParamsKindPath
	default:
		return copy.ParamsKindText
	}
}

// paramValueIssueMessage words the form's own refusal for one answer: the
// sentence printed under the input that caused it. A required value that is
// missing and a value that does not match its kind are the form's two rules;
// the label is the parameter's own display name (or its id).
func paramValueIssueMessage(copy i18n.Settings, issue extensions.ParamValueIssue) string {
	switch issue.Key {
	case extensions.ParamIssueRequiredMissing:
		return copy.RunParamRequired(issue.Label)
	default:
		return copy.RunParamInvalid(issue.Label)
	}
}

// paramFailureMessage words a refused run: the backend's `run_param_*` key
// names the parameter, and the label the user sees (or the id when there is
// none) is what the sentence interpolates — the old form's
// `paramRunErrorMessage`. The id is never shown when a label exists.
func paramFailureMessage(copy i18n.Settings, params []extensions.ParamDefinition, failure *extensions.ParamFailure) string {
	label := failure.Param
	for _, param := range params {
		if param.ID != failure.Param {
			continue
		}
		if trimmed := strings.TrimSpace(param.Label); trimmed != "" {
			label = trimmed
		}
		break
	}
	switch failure.Key {
	case extensions.RunErrorParamRequired:
		return copy.RunParamRequired(label)
	case extensions.RunErrorParamInvalid:
		return copy.RunParamInvalid(label)
	case extensions.RunErrorParamUnknown:
		return copy.RunParamUnknown(label)
	case extensions.RunErrorParamWindowsUnsafe:
		return copy.RunParamWindowsUnsafe(label)
	default:
		return failure.Error()
	}
}

// The run-output block: one integration's last background run, drawn in its
// row — a folded "View output" line, and the block itself when open, a
// fixed-height scroller in the row's own column rather than an overlay, because
// routine feedback does not deserve a dialog. The reference's
// `.extension-row__output` (`extensions.css:1199-1267`) is the shape; see
// `runoutput_test.go` for the pins.
//
// runOutputFont is the captured-output block's own type: the reference set
// `.extension-row__output-body` in the terminal's monospace stack
// (`extensions.css:1210-1221`) because the text it holds is a program's own
// output, not a sentence.
const runOutputFont = "SF Mono, Menlo, Monaco, Consolas, monospace"

// runOutputFace is the block's own fill: the reference's `--surface-sunken`
// recess (`extensions.css:1199-1205`) — the same shell the run form's box
// wears — never the card's raised `--glass-control` plane the port gave it.
// Named here rather than inlined so the one fact has one home and a test can
// read it without guessing at a composite.
func runOutputFace(tokens theme.Tokens) ui.Color { return tokens.Sunken }

// runOutputToggleFace is the folded affordance's fill at rest and under the
// pointer: transparent, then `--ext-bg-hover` (`extensions.css:1249,1252-1255`).
func runOutputToggleFace(tokens theme.Tokens, hovered bool) ui.Color {
	if hovered {
		return tokens.ControlHover
	}
	return ui.Transparent
}

// runOutput is the last background run's captured output, drawn as the
// reference's `.extension-row__output` block (`extensions.css:1199-1223`, the
// markup at `ExtensionRow.tsx:478-512`).
func (a *App) runOutput(c *ui.Context, copy i18n.Settings, integration Integration) {
	result, ok := a.runOutputs[integration.ID]
	if !ok || integration.Orphan {
		return
	}
	if !a.outputOpen[integration.ID] {
		// The folded affordance is not a settings row with an action pill: the
		// reference's `.extension-row__output-toggle` is a **text control** that
		// names what it opens — a 12px ChevronRight then the word — on its own
		// line so it cannot be mistaken for one of the row's icon actions
		// (`ExtensionRow.tsx:516-527`, `extensions.css:1243-1256`). The port
		// drew a whole ruled row whose whole label was the pill.
		a.runOutputToggle(c, copy, integration.ID)
		return
	}
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	// The block is the row's own full-width line, exactly as the run form's box
	// is: it spans the row's *content column* (the inset every row's label
	// carries), not the card's frame — the reference draws it inside
	// `.extension-row__open`'s `main` cell, one row padding in from the card
	// (`extensions.css:1199`, `ExtensionRow.tsx:478-490`). The port let it span
	// the card, where it read as a second card rather than as the row's own
	// block.
	inset := a.rowInset(t)
	ui.Column(c).FillWidth().Padding(0, inset, 0, inset).Children(func() {
		ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(1.5), t.Space(2)).
			// The block wears the reference's own surface: `.extension-row__output` is a
			// `--surface-sunken` recess under a `--glass-control-edge` hairline with the
			// `--ext-radius` corner — the same shell the run form's box wears, so the two
			// inline blocks read as one family (and neither as a raised card).
			Border(1, tokens.ControlEdge).Radius(tokens.RadiusSM).Background(runOutputFace(tokens)).Clip().
			Children(func() {
				// The head: the block's own name in the caption register at 600 in
				// `--text-tertiary`, the truncation mark as an unstyled aside beside it,
				// and the close chevron at the row's end (`extensions.css:1226-1235`).
				ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
					ui.Text(c, copy.RunLastOutput).FontSize(tokens.Caption).FontWeight(600).
						TextColor(tokens.TextTertiary)
					if result.Truncated {
						ui.Text(c, copy.RunOutputTruncated).FontSize(tokens.Caption).
							TextColor(tokens.TextTertiary)
					}
					ui.Row(c).Grow(1)
					if a.extensionIconButton(c, "chevron-down", copy.RunHideOutput, false).Clicked() {
						a.setRunOutputOpen(integration.ID, false)
					}
				})
				// The captured text itself: the monospace stack at the caption step, 1.45
				// line height and the strong ink, selectable because a reader copies out
				// of it (`extensions.css:1238-1258`). The empty state keeps the same line
				// box, in muted italics rather than as a blank sliver (`:1259-1267`).
				ui.Scroll(c.Key("settings.runoutput." + integration.ID)).TrackScroll(a.runOutputScroll(integration.ID)).
					FillWidth().MaxHeight(t.Space(45)).Children(func() {
					if !result.HasOutput() {
						ui.Text(c, copy.RunNoOutput).FontSize(tokens.Caption).
							TextColor(t.TextMuted).Italic()
						return
					}
					ui.Text(c, result.Text()).Font(runOutputFont).FontSize(tokens.Caption).
						TextColor(tokens.TextStrong).LineHeight(1.45).Wrap().Selectable()
				})
				// The truncation mark at the *end* of the text: the place a reader
				// reaches when the output stops, which the head's own mark cannot say
				// once they have scrolled (`ExtensionRow.tsx:503-511`).
				if result.Truncated {
					ui.Text(c, copy.RunOutputTruncatedTail).FontSize(tokens.Caption).
						TextColor(tokens.TextTertiary).Italic()
				}
			})
	})
}

// runOutputToggle is the folded output block's own affordance: the reference's
// `.extension-row__output-toggle` (`extensions.css:1243-1256`,
// `ExtensionRow.tsx:516-527`) — a 12px ChevronRight then the word, on the row's
// own line, transparent at rest and lifting to the hover fill under the
// pointer. It is a text control, not an icon button, which is what keeps it
// from reading as one of the row's four action pills.
func (a *App) runOutputToggle(c *ui.Context, copy i18n.Settings, id string) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	// The affordance is the row's own full-width line, like the block it opens:
	// it carries the row's content-column inset, so its chevron and word start
	// where the opened block's head does rather than at the card's frame.
	inset := a.rowInset(t)
	ui.Column(c).FillWidth().Padding(0, inset, 0, inset).Children(func() {
		ink := tokens.TextTertiary
		button := ui.ButtonBase(c).Label(copy.RunViewOutput).FillWidth().
			MinHeight(t.Space(7)).Gap(t.Space(1)).Padding(0, t.Space(2)).
			AlignItems(ui.Center).Justify(ui.Start).
			Border(0, ui.Transparent).Radius(tokens.RadiusXS).
			Background(ui.Transparent)
		hovered := button.Hovered()
		if hovered {
			ink = t.Text
		}
		button.Background(runOutputToggleFace(tokens, hovered))
		button.Children(func() {
			ui.Icon(c, mustGlyph("chevron-right")).Size(t.Space(3), t.Space(3)).TextColor(ink)
			ui.Text(c, copy.RunViewOutput).FontSize(tokens.Caption).TextColor(ink)
		})
		if button.Clicked() {
			a.setRunOutputOpen(id, true)
		}
	})
}

// truthyValue and boolValue are a parameter's own two spellings.
func truthyValue(value string) bool {
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func boolValue(on bool) string {
	if on {
		return "true"
	}
	return "false"
}

// toolParam is one declared input the connect form is editing.
type toolParam struct {
	ID          string
	Label       string
	Kind        string
	Placeholder string
	Default     string
	Flag        string
	Required    bool
	Options     string
}

// paramKinds are the kinds the editor offers, in the schema's order.
var paramKinds = []string{
	extensions.ParamText, extensions.ParamNumber, extensions.ParamBoolean,
	extensions.ParamSelect, extensions.ParamPath,
}

// issueUnder prints one row's own validation line: what the row still needs,
// under the row that caused it — the old build's `paramIssues`, whose errors
// rendered where they happened so a save never had to bounce back from the
// backend to explain a typo. A muted warning line, not a red banner.
func (a *App) issueUnder(c *ui.Context, message string) {
	t := a.cardTheme(c)
	ui.Text(c, message).FontSize(t.FontSize-1).TextColor(t.Warning).
		Padding(0, t.Space(3.5)).SingleLine()
}

// paramKindLabels are the kind picker's words.
func paramKindLabels(copy i18n.Settings) []string {
	return []string{
		copy.ParamsKindText, copy.ParamsKindNumber, copy.ParamsKindBoolean,
		copy.ParamsKindSelect, copy.ParamsKindPath,
	}
}

// editorParams is every row of the declared-input editor as a declaration,
// one for one with the editor's own rows, so a validation issue's index names
// the row it belongs to. A blank id survives (declaredParams drops it from
// the request) because the validator is exactly what has to name it.
func (a *App) editorParams() []extensions.ParamDefinition {
	params := make([]extensions.ParamDefinition, 0, len(a.toolParams))
	for _, param := range a.toolParams {
		definition := extensions.ParamDefinition{
			ID:          strings.TrimSpace(param.ID),
			Label:       strings.TrimSpace(param.Label),
			Kind:        param.Kind,
			Placeholder: strings.TrimSpace(param.Placeholder),
			Default:     strings.TrimSpace(param.Default),
			Flag:        strings.TrimSpace(param.Flag),
			Required:    param.Required,
		}
		for _, option := range strings.Split(param.Options, "\n") {
			if trimmed := strings.TrimSpace(option); trimmed != "" {
				definition.Options = append(definition.Options, trimmed)
			}
		}
		params = append(params, definition)
	}
	return params
}

// declaredParams turns the editor's rows into the declarations a request
// carries: a row with no id is skipped rather than refused, because a row
// being typed is not yet a declaration.
func (a *App) declaredParams() []extensions.ParamDefinition {
	var params []extensions.ParamDefinition
	for _, definition := range a.editorParams() {
		if definition.ID == "" {
			continue
		}
		params = append(params, definition)
	}
	return params
}

// paramIssueMessage is a row's validation line in the reader's language.
func paramIssueMessage(copy i18n.Settings, issue extensions.ParamRowIssue) string {
	switch issue {
	case extensions.ParamIssueID:
		return copy.ParamInvalidID
	case extensions.ParamIssueDuplicate:
		return copy.ParamDuplicateID
	case extensions.ParamIssueOptions:
		return copy.ParamNeedsOptions
	case extensions.ParamIssueFlag:
		return copy.ParamNeedsFlag
	case extensions.ParamIssueBadFlag:
		return copy.ParamInvalidFlag
	}
	return ""
}

// beginEdit fills the local-tool form from what an integration declares and
// remembers that the form is editing it rather than connecting something new.
func (a *App) beginEdit(id string, tool CustomTool) {
	a.toolOpen = true
	// Editing fills a fresh form, so it gets a fresh toolchain verdict too.
	a.clearScriptRuntimeCache()
	a.toolEditing = id
	a.toolAdvanced = false
	a.toolIssue = false
	a.toolMode = tool.Mode
	a.toolLanguage = tool.ScriptLanguage
	a.toolScript = tool.ScriptContent
	a.toolName = tool.Name
	a.toolProgram = tool.Program
	a.toolCommand = tool.Command
	a.toolArgs = tool.Args
	a.toolVersionArgs = tool.VersionArgs
	a.toolVersion = tool.Version
	a.toolDescription = tool.Description
	a.toolOutput = tool.Output
	a.toolPlatforms = map[string]bool{}
	for _, platform := range tool.Platforms {
		a.toolPlatforms[platform] = true
	}
	a.toolPermissions = map[string]bool{}
	for _, permission := range tool.Permissions {
		a.toolPermissions[permission] = true
	}
	a.toolParams = nil
	for _, param := range tool.Params {
		a.toolParams = append(a.toolParams, toolParam{
			ID: param.ID, Label: param.Label, Kind: param.Kind, Flag: param.Flag,
			Placeholder: param.Placeholder, Default: param.Default,
			Required: param.Required, Options: strings.Join(param.Options, "\n"),
		})
	}
}

// endEdit clears the edit state, so the next connection starts a new package.
func (a *App) endEdit() {
	a.clearScriptRuntimeCache()
	a.toolEditing = ""
	a.toolAdvanced = false
	a.toolIssue = false
	a.toolMode, a.toolLanguage, a.toolScript = "", "", ""
	a.toolName, a.toolProgram, a.toolCommand = "", "", ""
	a.toolArgs, a.toolVersionArgs, a.toolVersion, a.toolDescription = "", "", "", ""
	a.toolPlatforms = nil
	a.toolParams = nil
	a.toolPermissions = map[string]bool{}
}
