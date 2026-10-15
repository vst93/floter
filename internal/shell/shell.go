// Package shell is the mygo app itself: one frameless window whose body is
// one of three surfaces (launcher, settings, terminal), the settings store
// they read and write, the theme they paint with, and the global shortcut
// that summons the window.
//
// The panel swaps its body and resizes per surface, and a surface that is not
// the launcher survives a hide for the residency window, so a summon returns
// the user to where they were.
package shell

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/browser"
	"floter/internal/calculator"
	"floter/internal/clipboard"
	"floter/internal/drops"
	"floter/internal/extensions"
	"floter/internal/extensions/recommended"
	"floter/internal/glassmap"
	"floter/internal/i18n"
	"floter/internal/inventory"
	"floter/internal/launcher"
	"floter/internal/scriptlang"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"floter/internal/shortcuts"
	"floter/internal/spawn"
	"floter/internal/terminalui"
	"floter/internal/tools"
	"floter/internal/usage"
)

// Surface is the body the window shows.
type Surface int

const (
	// SurfaceLauncher is the search surface (the app's home).
	SurfaceLauncher Surface = iota
	// SurfaceSettings is the settings panel.
	SurfaceSettings
	// SurfaceTerminal is a terminal session.
	SurfaceTerminal
)

// ParseSurface reads a surface name, for a caller that wants to open one by
// name (FLOTER_OPEN in cmd/floter, so a smoke test can start on any of the
// three). The empty string is the launcher.
func ParseSurface(name string) (Surface, bool) {
	switch name {
	case "", "launcher":
		return SurfaceLauncher, true
	case "settings":
		return SurfaceSettings, true
	case "terminal":
		return SurfaceTerminal, true
	default:
		return SurfaceLauncher, false
	}
}

// The window geometry per surface: the launcher is the 720-wide collapsed
// panel, settings is a 720×580 work panel capped to the screen, and the
// terminal is the 860×600 default (resizable, min 640×360).
const (
	settingsWindowHeight = 580
	settingsMinHeight    = 420

	// defaultSummonShortcut is what the app registers when the user never
	// changed it.
	defaultSummonShortcut = "Ctrl+Space"

	// repoURL is where the About page points.
	repoURL = "https://github.com/vst93/floter"

	// showGrace is how long after a reveal a blur is ignored, as the old
	// shell suppressed blur for a moment after showing the panel.
	showGrace = 400 * time.Millisecond
)

// Options configures the app.
type Options struct {
	// Store is the settings store to read and write. Required.
	Store *settings.Store
	// NewTerminal creates a terminal session; nil means the terminal
	// plugin's terminal.New. Tests replace it so no native library is
	// needed. The options come from the terminal surface, which owns the
	// appearance mapping.
	NewTerminal func(terminal.Options) (*terminal.Terminal, error)
	// RefreshTerminal applies the current appearance to the running terminal
	// sessions. Nil means the terminal surface's own Refresh; tests replace it
	// so entering the surface can be observed without a native session.
	RefreshTerminal func()
	// Quit ends the app.
	Quit func()
	// WorkAreaHeight is the primary display's work area, for the settings
	// panel's cap. Zero means "no cap" (tests, headless runs).
	WorkAreaHeight float64
	// Paths is where the extension directories and state live. Zero means
	// the platform's config directory, discovered in New; tests point it at
	// a fixture.
	Paths extensions.Paths
	// RefreshIntegrations runs the providers when the app starts. Tests set
	// it to a no-op so no process is spawned.
	RefreshIntegrations bool
	// OpenAtLogin and SetOpenAtLogin read and write the system's login
	// item; nil means the framework's, and tests provide stubs so they
	// never touch the machine's login items.
	OpenAtLogin    func() bool
	SetOpenAtLogin func(open bool) error
	// Registry is the npm registry the install action uses; nil means the
	// shipped one, and tests point it at a fixture server.
	Registry *extensions.Registry
	// ConfirmPermissions approves an install's permissions; nil means the
	// native dialog, and tests answer for themselves.
	ConfirmPermissions func(extensions.PermissionApproval) bool
	// ReviewInstall shows the local package review (what the package is and
	// what it asks for) before a folder install commits; nil means the native
	// dialog. It is separate from ConfirmPermissions because the reference's
	// folder flow always shows the review, permissions or not.
	ReviewInstall func(extensions.Prepared) bool
	// ReviewRecommended shows the recommended-tool permission review before a
	// shipped package is connected; nil means the native dialog. It is separate
	// from ConfirmPermissions because the reference's connect flow reviews the
	// manifest's own permissions, tiered, rather than the plain approval.
	ReviewRecommended func(extensions.Prepared) bool
	// ChooseSystemTool picks which of several matching executables a system
	// runtime binds to, before a connect or a reconnect; nil means the native
	// dialog. It only appears when a manifest's declared names resolve to more
	// than one program (the reference's `chooseSystemTool`).
	ChooseSystemTool func(name string, candidates []extensions.ExecutableCandidate) (string, bool)
	// OpenPinned replaces the pinned-window implementation; nil opens the
	// real window, and tests record what would have been pinned.
	OpenPinned func(title, text string)
	// OpenDetached replaces the detached-output-window implementation; nil
	// opens the real window, and tests record what would have been opened.
	OpenDetached func(view launcher.OutputView) *mygo.Window
	// WriteClipboard puts a clipboard payload back on the system
	// clipboard; nil uses the framework's clipboard, and tests record what
	// would have been written.
	WriteClipboard func(payload ClipboardPayload)
	// RegisterShortcut registers a global shortcut; nil uses the framework's
	// global shortcuts, and tests record what would have been registered.
	RegisterShortcut func(accelerator string, fn func()) error
	// UnregisterShortcut releases a global shortcut; nil uses the framework's.
	UnregisterShortcut func(accelerator string)
	// OpenExternalTerminal opens the user's terminal emulator; nil uses the
	// platform's own list, and tests record the call.
	OpenExternalTerminal func() error
	// RunSilentCommand runs a custom shortcut's command line; nil spawns the
	// user's shell, and tests record the command.
	RunSilentCommand func(command string) error
	// SaveFileDialog and OpenFileDialog pick a path for the integrations
	// export and import; nil uses the native dialogs, and tests answer for
	// themselves.
	SaveFileDialog func(defaultName string) (string, error)
	OpenFileDialog func() (string, error)
	// Notify raises a system notification; nil uses the framework's, and
	// tests record what would have been shown.
	Notify func(title, body string)
	// Relaunch starts the freshly installed version after an update; nil uses
	// the framework's own relaunch, and tests record the call.
	Relaunch func()
	// ConfirmDialog answers a confirmation the settings ask for (clearing the
	// clipboard history); nil uses the framework's own dialog, and tests
	// answer for themselves. The launcher's confirmations are the panel's own
	// docked rows and never reach here.
	ConfirmDialog func(title string) bool
	// OpenPath hands a path to the system's own opener; nil runs the
	// platform's own opener, and tests record what would have been opened.
	OpenPath func(path string) error
	// OpenURL opens a page in a browser (the id it came from, empty for the
	// system's own); nil uses the platform, and tests record what would have
	// been opened or return the failure to exercise.
	OpenURL func(browserID, url string) error
	// RunPowerCommand runs a power action; nil runs the platform's own
	// command, and tests record what would have run.
	RunPowerCommand func(action string) error
	// HomeDir is the user's home directory, where the browser plugin looks
	// for profiles; empty means os.UserHomeDir, and tests point it at a
	// fixture tree.
	HomeDir string
	// LookTools resolves the install catalog against this machine's search
	// path; nil uses the real tools.Look. Tests replace it so a refresh runs
	// with no filesystem, and no process, in the way.
	LookTools func() []tools.State
}

// ClipboardPayload is one clipboard entry, ready to be written back: its
// kind, its text, its paths or its image's bytes.
type ClipboardPayload struct {
	Kind  string
	Text  string
	Paths []string
	Image []byte
}

// App is the running application.
type App struct {
	Store *settings.Store

	// homeDir is where the browser plugin looks for profiles.
	homeDir string

	Surf     Surface
	Launcher *launcher.App
	Settings *settingsui.App
	Terminal *terminalui.App
	// refreshTerminal applies the current appearance to the running sessions;
	// the terminal surface's own Refresh unless a test replaced it.
	refreshTerminal func()

	// Win is the window, created by Start.
	Win *mygo.Window
	// windowVisible replaces the window's own visibility reading in Show. A
	// test has no window to put on screen, so it drives the reveal decision
	// through this seam; nil reads the real window (and, with no window at
	// all, reports it as on screen — there is nothing to reveal).
	windowVisible func() bool
	// hideWindow replaces the window's own Hide inside Hide. A test has no
	// window to dismiss, so the dismiss table's outcome would be unobservable;
	// nil drives the real window. Not part of Options: the app always has a
	// real hide, only tests need to see it.
	hideWindow func()
	// notices are the toasts queued for the next frame, and refusals is the
	// gate that keeps one bad external trigger from repeating its sentence in
	// a loop (see notices.go).
	noticeMu sync.Mutex
	notices  []ui.Toast
	refusals *refusalGate
	// registerTarget is the executable path a `floter://register` link named,
	// and registerTimer the beat that clears the mark: the highlight is a
	// moment, not a mode (the reference's `useTimedReset(registerTarget, …)`
	// in `src/ExtensionsPanel.tsx:1546`). The integration list marks the card
	// whose executable is this path, and nothing about its actions changes.
	registerTarget string
	registerTimer  *time.Timer
	// registerHighlight is how long that mark lasts; tests shorten it.
	registerHighlight time.Duration
	// driftGate is the session-scoped "tell the user once" gate for the silent
	// background drift re-probe (R7-7): one toast per (integration, version,
	// direction, magnitude) per session, and no toast at all for a no-op
	// frame. Its decisions come from the pure `extensions.DecideDriftNotice`.
	driftGate *extensions.DriftNoticeGate
	// driftRunning guards the background version-drift loop, so rapid listings
	// cannot stack probes (the reference's `try_start_drift_reprobe`).
	driftMu      sync.Mutex
	driftRunning bool
	// transferMu/transferRunning guard the export/import flow: the reference's
	// `syncOperationRef` ignores a second press while one transfer is in
	// flight (the buttons are disabled too, but the guard is what makes the
	// claim atomic with the action).
	transferMu      sync.Mutex
	transferRunning bool
	// detached are the output windows the user pinned: each one stays up
	// whatever the panel does (see detach.go).
	detached []*detachedOutput

	// Integrations is the extension state: the repository, the installed
	// manifests and the providers' commands.
	Integrations *extensions.Store
	// Runs tracks the background runs in flight, so that disabling or
	// uninstalling an integration can stop them (the old RunAbortRegistry).
	Runs *extensions.RunRegistry
	// Usage is what the user launches, for the launcher's empty state.
	Usage *usage.Store
	// Paths is where the extension directories live.
	Paths extensions.Paths

	// Registry is the npm registry the install action reads.
	Registry *extensions.Registry
	// confirmPermissions approves an install's permissions, nil for the
	// native dialog.
	confirmPermissions func(extensions.PermissionApproval) bool
	// reviewInstall shows the local package review before a folder install
	// commits, nil for the native dialog.
	reviewInstall func(extensions.Prepared) bool
	// reviewRecommended shows the recommended-tool permission review before a
	// shipped package is connected, nil for the native dialog.
	reviewRecommended func(extensions.Prepared) bool
	// chooseTool is the tool-selection dialog, nil for the native dialog.
	chooseTool func(name string, candidates []extensions.ExecutableCandidate) (string, bool)
	// diagnoses is the last health check per integration, and diagnosisMu
	// guards it: a check runs off the main thread.
	diagnoses   map[string]settingsui.Integration
	diagnosisMu sync.Mutex

	// pins are the pinned-output windows, and openPinned replaces them in
	// tests.
	pins         map[*mygo.Window]*pinned
	pinMu        sync.Mutex
	openPinned   func(title, text string)
	openDetached func(view launcher.OutputView) *mygo.Window
	// writeClipboard puts a payload back on the system clipboard.
	writeClipboard func(ClipboardPayload)
	// registerShortcut and unregisterShortcut bind and release global
	// shortcuts, and summonKey is the summon accelerator currently
	// registered.
	registerShortcut   func(string, func()) error
	unregisterShortcut func(string)
	summonKey          string

	// Clipboard is the clipboard history the launcher searches, and
	// Calculator its calculation history.
	Clipboard  *clipboard.Store
	Calculator *calculator.Store
	// lastCalculatorPlugin is the calculator block the store was last synced
	// with.
	lastCalculatorPlugin settings.CalculatorPlugin
	// selfCopied is the text the app itself put on the clipboard, which the
	// watcher must not record again.
	selfCopied string
	// clipboardInterval is how often the watcher reads the clipboard;
	// tests shorten it.
	clipboardInterval time.Duration
	// clipboardWatching is set while the watcher goroutine runs.
	clipboardWatching bool
	// appIcon is the stored app icon ("dark" or "light"), and lastAppIcon
	// what the tray and window were last given.
	appIcon     string
	lastAppIcon string
	// lastMenubarIcon is the show_menubar_icon value the tray was last
	// synced with.
	lastMenubarIcon bool
	// lastGlassStep is the glass_step the window's vibrancy material was last
	// built with: a stop change re-hands the material, so the blur ladder
	// follows the setting (see vibrancyForStop).
	lastGlassStep string
	// lastShowRecent and lastShowCommands are the show_recent_in_launcher and
	// show_commands_in_search values the launcher was last synced with: a
	// change re-hands the recents, or re-scans the PATH.
	lastShowRecent   bool
	lastShowCommands bool
	// lastTrayLanguage is the language the tray's labels were built with: a
	// change rebuilds them, so the tray never lags the UI.
	lastTrayLanguage string
	// shortcutsSuspended is true while a settings recorder holds the keyboard:
	// the global shortcuts are released then.
	shortcutsSuspended bool
	// The terminal's size write-back: the size the last resize settled on,
	// its generation, and the timer that writes it.
	sizeMu         sync.Mutex
	pendingWidth   float64
	pendingHeight  float64
	sizeGeneration int
	sizeTimer      *time.Timer
	// terminalSizeDelay is how long a resize waits before it is stored; tests
	// shorten it.
	terminalSizeDelay time.Duration

	// customKeys are the user-defined shortcuts currently registered, so a
	// settings change can release exactly those; customSignature is the list
	// they were registered from.
	customKeys      []string
	customSignature string
	// openExternalTerminal is how the external-terminal action starts an
	// emulator, and runSilentCommand how a command-line shortcut runs; nil
	// uses the platform's own paths.
	openExternalTerminal func() error
	silentCommand        func(command string) error
	saveFileDialog       func(defaultName string) (string, error)
	openFileDialog       func() (string, error)
	notify               func(title, body string)
	relaunch             func()
	// onScreen overrides the panel's on-screen answer; tests replace it so
	// the two completion channels (the panel's own row, the system
	// notification) can both be exercised without a live window.
	onScreen        func() bool
	confirmDialog   func(title string) bool
	runPowerCommand func(action string) error
	openPathCommand func(path string) error
	openURLCommand  func(browserID, url string) error
	// lastClipboardText and lastClipboardFormats are what the watcher saw
	// last, so a poll records only what changed.
	lastClipboardText    string
	lastClipboardFormats string
	// browserProfiles is the discovered browser profiles, and browserMu
	// guards it: the first search discovers them. browserBase is the custom
	// base directory they were discovered with, so a settings change
	// rediscovers.
	browserProfiles []browser.Profile
	browserBase     string
	browserMu       sync.Mutex
	browserLoaded   bool
	// LastClipboardSettings is the clipboard state the watcher was last
	// synced with.
	LastClipboardSettings settings.ClipboardSettings

	// Tray is the menu bar icon, nil when it could not be added.
	Tray *mygo.Tray
	// lastLaunchAtStartup is the setting the login item was last synced
	// with, and the accessors that do the syncing.
	lastLaunchAtStartup bool
	openAtLogin         func() bool
	setOpenAtLogin      func(bool) error

	// refreshIntegrations runs the providers at startup.
	refreshIntegrations bool
	// lookTools resolves the install catalog against this machine's search
	// path (see Options.LookTools).
	lookTools func() []tools.State
	// catalogMu guards catalogScanning: the catalog is read one scan at a
	// time, so a refresh that arrives while one is in flight shares it rather
	// than duplicating it (the old build's in-flight fetch).
	catalogMu       sync.Mutex
	catalogScanning bool

	quit     func()
	workArea float64

	lastW, lastH       int
	lastResizable      bool
	lastMinW, lastMinH int
	// launcherTarget is the height the launcher's list asked for (the View
	// measures it); walkLauncherHeight eases the window there.
	launcherTarget int

	// ShortcutErr is why the global shortcut could not be registered.
	ShortcutErr error

	// now is the clock the residency rule reads; tests replace it.
	now func() time.Time
	// enteredAt is when the current surface was opened, and shownAt when
	// the window was last revealed (a blur within the grace period after a
	// reveal is the platform settling, not the user leaving).
	enteredAt time.Time
	shownAt   time.Time
	// residencyOn is the residency surface the last frame noted, so entering
	// one starts the clock exactly once (see noteResidency).
	residencyOn string
}

// New builds the app and its surfaces.
func New(opts Options) *App {
	quit := opts.Quit
	if quit == nil {
		quit = func() { mygo.App.Quit() }
	}
	newTerminal := opts.NewTerminal
	if newTerminal == nil {
		newTerminal = terminal.New
	}
	paths := opts.Paths
	if paths.Root == "" {
		discovered, err := extensions.DiscoverPaths()
		if err != nil {
			log.Printf("floter: no extension directory: %v", err)
		} else {
			paths = discovered
		}
	}
	// The host's own version is what a manifest's `compatibility.floter`
	// range is checked against at install time, as the old host checked it
	// with its own compile-time version.
	if paths.HostVersion == "" {
		paths.HostVersion = appVersion()
	}
	a := &App{
		Store:                opts.Store,
		Surf:                 SurfaceLauncher,
		quit:                 quit,
		workArea:             opts.WorkAreaHeight,
		now:                  time.Now,
		Paths:                paths,
		Integrations:         extensions.OpenStore(paths),
		Runs:                 extensions.NewRunRegistry(),
		Usage:                usage.Open(paths.Root),
		refreshIntegrations:  opts.RefreshIntegrations,
		lastLaunchAtStartup:  opts.Store.Snapshot().LaunchAtStartup,
		openAtLogin:          opts.OpenAtLogin,
		setOpenAtLogin:       opts.SetOpenAtLogin,
		clipboardInterval:    clipboardPoll,
		Registry:             opts.Registry,
		confirmPermissions:   opts.ConfirmPermissions,
		reviewInstall:        opts.ReviewInstall,
		reviewRecommended:    opts.ReviewRecommended,
		chooseTool:           opts.ChooseSystemTool,
		openPinned:           opts.OpenPinned,
		openDetached:         opts.OpenDetached,
		writeClipboard:       opts.WriteClipboard,
		registerShortcut:     opts.RegisterShortcut,
		unregisterShortcut:   opts.UnregisterShortcut,
		openExternalTerminal: opts.OpenExternalTerminal,
		silentCommand:        opts.RunSilentCommand,
		saveFileDialog:       opts.SaveFileDialog,
		openFileDialog:       opts.OpenFileDialog,
		notify:               opts.Notify,
		relaunch:             opts.Relaunch,
		confirmDialog:        opts.ConfirmDialog,
		runPowerCommand:      opts.RunPowerCommand,
		openPathCommand:      opts.OpenPath,
		openURLCommand:       opts.OpenURL,
		homeDir:              opts.HomeDir,
		lookTools:            opts.LookTools,
		appIcon:              storedAppIcon(opts.Store.Snapshot()),
		lastAppIcon:          storedAppIcon(opts.Store.Snapshot()),
		lastGlassStep:        opts.Store.Snapshot().GlassStep,
	}
	if a.lookTools == nil {
		a.lookTools = func() []tools.State {
			return tools.Look(extensions.SearchDirectories(), tools.Platform())
		}
	}
	// The refusal gate reads the app's own clock, so a test that replaces it
	// drives the 30s window without waiting.
	a.refusals = newRefusalGate(refusalWindow, func() time.Time { return a.now() })
	if a.registerHighlight == 0 {
		a.registerHighlight = registerHighlightWindow
	}
	if a.writeClipboard == nil {
		a.writeClipboard = systemClipboardWrite
	}
	if a.registerShortcut == nil {
		a.registerShortcut = func(accelerator string, fn func()) error {
			return mygo.GlobalShortcut.Register(accelerator, fn)
		}
	}
	if a.unregisterShortcut == nil {
		a.unregisterShortcut = func(accelerator string) {
			mygo.GlobalShortcut.Unregister(accelerator)
		}
	}
	if a.Registry == nil {
		a.Registry = extensions.NewRegistry()
	}
	if a.openAtLogin == nil {
		a.openAtLogin = mygo.App.OpenAtLogin
	}
	if a.setOpenAtLogin == nil {
		a.setOpenAtLogin = mygo.App.SetOpenAtLogin
	}
	// The quit path saves the terminal's screen first, so the next start
	// restores what the user was looking at. Every way out goes through
	// a.quit (the launcher row, the tray menu, the application menu).
	innerQuit := a.quit
	a.quit = func() {
		if a.Settings != nil {
			a.Settings.Flush()
		}
		if a.Terminal != nil {
			if err := a.Terminal.SaveSnapshot(); err != nil {
				log.Printf("floter: could not save the terminal snapshot: %v", err)
			}
			// Every session's program ends with the app: one left running
			// behind a closed panel is a process nobody can reach.
			a.Terminal.CloseAll()
		}
		// A detached output window outlives the panel by design, but not the
		// app: quitting closes them with everything else.
		a.closeDetached()
		innerQuit()
	}
	a.Launcher = launcher.New(opts.Store, launcher.Actions{
		OpenSettings: func() { a.Open(SurfaceSettings) },
		OpenSessions: func() {
			// The field row's terminal button: the settings, on the page
			// that lists the sessions, as the old build's own button did.
			a.Settings.Page = settingsui.PageSessions
			a.Open(SurfaceSettings)
		},
		ConfigFor:    a.pluginConfig,
		ConfigChange: a.changePluginConfig,
		ConfigAction: a.runPluginAction,
		DrawConfig:   a.drawPluginConfig,
		OpenTerminal: func() { a.Open(SurfaceTerminal) },
		Quit:         a.quit,
		Dismiss:      a.Hide,
		ResizeTo:     a.resizeLauncher,
		Copy: func(text string) {
			a.selfCopied = text
			mygo.Clipboard.WriteText(text)
		},
		CopyClip: a.copyClipboardEntry,
		OpenApp: func(app apps.App) {
			if err := app.Open(); err != nil {
				log.Printf("floter: could not open %s: %v", app.Name, err)
				a.onMain(func() { a.Launcher.WarnFeedback(i18n.For(a.Store.Snapshot().Language).Launcher.OpenAppFailed) })
				return
			}
			// Remember the launch before hiding: the next empty query
			// offers it.
			if err := a.Usage.Record(app.Path); err != nil {
				log.Printf("floter: could not record the launch: %v", err)
			}
			a.refreshRecents()
			a.Hide()
		},
		RunCommand:         a.runCommand,
		RunCommandCaptured: a.runCommandCaptured,
		LoadOutputPage:     a.loadOutputPage,
		Power:              a.power,
		OpenPath:           a.openPath,
		OpenInTerminal:     a.openInTerminal,
		OpenTerminalWindow: a.openTerminalWindow,
		OpenInstallSession: a.openInstallSession,
		Complete:           a.completeCommand,
		PinText:            a.PinText,
		DetachOutput:       a.detachOutput,
		OpenURL:            a.openURL,
		ActivateTab: func(tab browser.Tab) {
			options := settings.BrowserPluginOf(a.Store.Snapshot())
			go func() {
				if err := browser.ActivateTab(context.Background(), tab, options.CDPEnabled, options.CDPPort); err != nil {
					log.Printf("floter: could not focus the tab: %v", err)
				}
			}()
		},
		RunInTerminal: func(argv []string) {
			if err := a.Terminal.RunCommand(argv, "current", nil); err != nil {
				log.Printf("floter: could not run %v: %v", argv, err)
			}
			a.Open(SurfaceTerminal)
		},
		// RunInShell is the launcher's own shell line (`ls | grep go`, which only
		// a shell can read): the session exists to carry that line, so it is
		// command-started and leaves with it (R62). The install rows keep the
		// bare hand-off below, where the user's shell outlives the install.
		RunInShell: a.openShellCommand,
		// SpawnDetached starts a tool's own GUI action: no terminal, no
		// capture, and the launcher clears and hides the way any row does.
		SpawnDetached: func(argv []string) {
			if len(argv) == 0 {
				return
			}
			if err := spawn.Program(argv[0], argv[1:]...); err != nil {
				log.Printf("floter: could not start %v: %v", argv, err)
				a.Launcher.WarnFeedback(i18n.For(a.Store.Snapshot().Language).Launcher.InvokeFailed)
				return
			}
			a.Launcher.ResetQuery()
			a.Hide()
		},
		RunInTerminalWithEnv: func(argv []string, env []string) {
			if err := a.Terminal.RunCommand(argv, "current", env); err != nil {
				log.Printf("floter: could not run %v: %v", argv, err)
			}
			a.Open(SurfaceTerminal)
		},
		SearchBrowser: a.searchBrowser,
	})
	a.Settings = settingsui.New(opts.Store, settingsui.Actions{
		Close:        func() { a.Open(SurfaceLauncher) },
		Quit:         a.quit,
		CloseSession: func() { a.Terminal.Close() },
		// Resuming a session is showing it: the settings page hands the
		// index back, and the terminal surface comes to the front with that
		// session's screen.
		SelectSession: func(index int) {
			a.Terminal.SelectSession(index)
			a.Open(SurfaceTerminal)
		},
		NewSession: func() {
			a.Terminal.NewSession()
			a.Open(SurfaceTerminal)
		},
		CloseSessionAt:       a.Terminal.CloseSession,
		DiagnoseIntegration:  a.diagnoseIntegration,
		RepairIntegration:    a.repairIntegration,
		ReprobeCommands:      a.reprobeCommands,
		ReconnectIntegration: a.reconnectIntegration,
		OpenHomepage:         a.openHomepage,
		ScriptRuntime: func(language string) scriptlang.Check {
			return extensions.ScriptRuntimeFor(context.Background(), language)
		},
		// The toolchain probe runs off the render thread, so it asks for a
		// frame when it lands: mygo's Invalidate is safe from any goroutine.
		Invalidate: func() {
			if a.Win != nil {
				a.Win.Invalidate()
			}
		},
		// A warning the form cannot draw for itself goes to the app's own
		// notice, which shows on every surface — the form's save already
		// happened, so the sentence is about the machine, not the sheet.
		NotifyWarning: func(text string) {
			a.onMain(func() { a.warnNotice(text) })
		},
		// A confirmation the panel cannot draw for itself goes to the app's own
		// notice too: the deep-link copy is the reference's `onCopied`, and its
		// "Link copied" belongs on the one stack in either surface.
		NotifySuccess: func(text string) {
			a.onMain(func() { a.notifySuccess(text) })
		},
		ConnectLocalTool:   a.connectLocalTool,
		EditCustomTool:     a.editCustomTool,
		ToolForEdit:        a.toolForEdit,
		Freshness:          a.integrationFreshness,
		Detected:           a.detectedTools,
		ConnectDetected:    a.connectDetected,
		InstallDetected:    a.openShellCommand,
		InstallFromFolder:  a.installPackageFolder,
		RunWithParams:      a.runWithParams,
		ChooseFolder:       a.chooseProgram,
		CopyText:           func(text string) { mygo.Clipboard.WriteText(text) },
		ChooseProgram:      a.chooseProgram,
		SetShortcut:        a.setShortcut,
		SetPage:            a.rememberSettingsPage,
		SetCommandEnabled:  a.setCommandEnabled,
		SetCommandAlias:    a.setCommandAlias,
		CustomShortcuts:    func() []settings.CustomShortcut { return settings.CustomShortcutsOf(a.Store.Snapshot()) },
		SetCustomShortcuts: a.setCustomShortcuts,
		InstallFromRegistry: func(name, constraint string) {
			go func() {
				prepared, err := extensions.PrepareRegistry(context.Background(), a.Paths, a.Registry, name, constraint)
				if err != nil {
					log.Printf("floter: could not install %s: %v", name, err)
					a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstallFailed(name) })
					return
				}
				approved := true
				if prepared.Approval.NeedsApproval() {
					approved = a.confirmInstallPermissions(prepared.Approval)
				}
				entry, err := prepared.Commit(approved)
				if err != nil {
					if errors.Is(err, extensions.ErrPermissionApprovalRequired) {
						log.Printf("floter: %s was not approved, so it was not installed", prepared.Name())
						return
					}
					log.Printf("floter: could not install %s: %v", name, err)
					a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstallFailed(name) })
					return
				}
				log.Printf("floter: installed %s %s", entry.Name, entry.PackageVersion)
				a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstalled(entry.Name) })
				a.RefreshIntegrations(context.Background())
			}()
		},
		UninstallComponents: func(id, name, kind string, components settingsui.UninstallComponents) {
			// The settings page's own componentized dialog already asked (the
			// checkboxes are the question); arriving here is the answer. A
			// removal stops the integration's runs before any file moves: a
			// program still writing its own package directory would race the
			// deletion (the old kill_extension_runs).
			a.Runs.Kill(id)
			go func() {
				result2, err := extensions.UninstallComponentized(a.Paths, id, extensions.UninstallComponents{
					RemoveHostConfig: components.RemoveHostConfig,
					RemoveToolData:   components.RemoveToolData,
					RemoveArtifacts:  components.RemoveArtifacts,
				})
				if err != nil {
					log.Printf("floter: could not uninstall %s: %v", id, err)
					return
				}
				a.notifyIntegrationRemoval(kind, name)
				log.Printf("floter: uninstalled %s (config=%v data=%v artifacts=%v)",
					id, result2.RemovedHostConfig, result2.RemovedToolData, result2.RemovedArtifacts)
				a.RefreshIntegrations(context.Background())
			}()
		},
		UninstallIntegration: func(id, name, kind string) {
			// The settings page has already asked inline (the removal bar);
			// arriving here is the answer. A removal stops the integration's
			// runs before any file moves: a program still writing its own
			// package directory would race the deletion (the old
			// kill_extension_runs).
			a.Runs.Kill(id)
			go func() {
				if err := extensions.Uninstall(a.Paths, id, false); err != nil {
					log.Printf("floter: could not uninstall %s: %v", id, err)
					a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationRemoveFailed(name) })
					return
				}
				a.notifyIntegrationRemoval(kind, name)
				a.RefreshIntegrations(context.Background())
			}()
		},
		InstallUpdate:         a.installUpdate,
		ConnectRecommended:    a.connectRecommended,
		SaveConfiguration:     a.saveIntegrationConfiguration,
		ClearClipboardHistory: func() { a.clearClipboardHistoryAsks(a.confirmClearClipboard) },
		OpenPluginConfig:      a.openPluginConfig,
		SetPluginEnabled: func(id string, enabled bool) {
			a.changePluginConfig(id, "enabled", enabled)
		},
		ResetShortcuts:        a.resetShortcuts,
		SetShortcutsSuspended: a.setShortcutsSuspended,
		ExportIntegrations:    a.exportIntegrations,
		ImportIntegrations:    a.importIntegrations,
		ExportScript:          a.exportScript,
		AdoptIntegration: func(id string) {
			// The approval dialog blocks, so the whole adoption runs off the
			// main thread and refreshes from there.
			go func() {
				prepared, err := extensions.PrepareAdopt(a.Paths, id)
				if err != nil {
					log.Printf("floter: could not adopt %s: %v", id, err)
					a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstallFailed(id) })
					return
				}
				approved := true
				if prepared.Approval.NeedsApproval() {
					approved = a.confirmInstallPermissions(prepared.Approval)
				}
				entry, err := prepared.Commit(approved)
				if err != nil {
					log.Printf("floter: could not adopt %s: %v", id, err)
					a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstallFailed(id) })
					return
				}
				a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstalled(entry.Name) })
				a.RefreshIntegrations(context.Background())
			}()
		},
		DeleteOrphan: func(id string) {
			// The settings page has already asked inline; arriving here is
			// the answer.
			a.Runs.Kill(id)
			go func() {
				if err := extensions.DeleteOrphan(a.Paths, id); err != nil {
					log.Printf("floter: could not delete %s: %v", id, err)
				}
				a.RefreshIntegrations(context.Background())
			}()
		},
		SetIntegrationEnabled: func(id string, enabled bool) {
			if !enabled {
				// Disabling is a mark, but a run in flight is stopped now: the
				// old set_enabled's disable branch kills its runs before the flag
				// is written (R98).
				a.Runs.Kill(id)
			}
			if err := a.Integrations.SetEnabled(id, enabled); err != nil {
				log.Printf("floter: could not change %s: %v", id, err)
			}
			// Enabling an integration asks its provider again, in the
			// background: its commands must appear without a restart.
			go a.RefreshIntegrations(context.Background())
		},
	})
	a.Settings.About = settingsui.About{
		Name:         mygo.App.Name(),
		Version:      appVersion(),
		Framework:    "mygo " + mygo.Version,
		Scheme:       "floter://",
		SettingsPath: opts.Store.Path(),
		RepoURL:      repoURL,
	}
	a.Settings.Shortcut = SummonShortcut(opts.Store.Snapshot())
	a.Settings.ShortcutID = shortcutToggleWindow
	a.summonKey = shortcuts.NormalizeOr(SummonShortcut(opts.Store.Snapshot()))
	a.Clipboard = clipboard.NewStore(clipboard.FromConfigRoot(paths.Root), clipboardState(opts.Store.Snapshot()).MaxItems)
	a.Launcher.Clipboard = a.Clipboard
	a.LastClipboardSettings = clipboardState(opts.Store.Snapshot())

	calculatorPlugin := settings.CalculatorPluginOf(opts.Store.Snapshot())
	a.Calculator = calculator.NewStore(calculator.FromConfigRoot(paths.Root),
		calculatorPlugin.MaxItems, calculatorPlugin.RetentionDays)
	a.Launcher.Calculator = a.Calculator
	a.Launcher.CopyMode = calculatorPlugin.CopyMode
	a.lastCalculatorPlugin = calculatorPlugin

	// The launch counts an earlier build kept in the settings file seed this
	// build's usage history once, so the "most used" list survives the move.
	if a.Usage.Seed(settings.LaunchCountsOf(opts.Store.Snapshot())) {
		if err := a.Usage.Save(); err != nil {
			log.Printf("floter: could not import the launch counts: %v", err)
		}
	}
	a.Settings.Integrations = func() []settingsui.Integration { return a.integrationList() }
	a.Settings.Recommended = a.recommendedTools
	a.Settings.Sessions = func() []settingsui.Session {
		sessions := a.Terminal.Sessions
		out := make([]settingsui.Session, 0, len(sessions))
		for i := range sessions {
			out = append(out, settingsui.Session{
				ID:      sessions[i].ID,
				Title:   a.Terminal.LabelOf(i),
				Running: !a.Terminal.Exited(i),
				Active:  i == a.Terminal.Active,
				Index:   i,
				Note:    a.Terminal.NoteOf(i),
				Dir:     a.Terminal.DirOf(i),
				Size:    a.Terminal.SizeOf(i),
				Created: sessions[i].Created,
			})
		}
		return out
	}

	// The plugin's callbacks run on the terminal's own goroutine, so they
	// hand the main thread the work of touching the app's state.
	termActions := terminalui.Actions{
		Close:      func() { a.Open(SurfaceLauncher) },
		NewCommand: func() { a.returnToInput() },
		OpenExternal: func() {
			// The session is handed to the system's own terminal: the Go
			// build runs its terminal in-process, so this opens a new window
			// rather than attaching to the running one. A machine with no
			// emulator to open says so in the panel itself (the old build's
			// `terminal-feedback` line) instead of appearing to do nothing.
			if err := a.OpenExternalTerminal(); err != nil {
				log.Printf("floter: could not open a terminal: %v", err)
				a.onMain(func() {
					copy := i18n.For(a.Store.Snapshot().Language).Launcher
					a.Terminal.ShowFeedback(copy.OpenTermFailed)
					a.Terminal.FocusTerminal()
				})
			}
		},
		Title: func(title string) {
			a.onMain(func() {
				a.Terminal.SetTitle(title)
				if a.Win != nil && title != "" {
					a.Win.SetTitle(title)
				}
			})
		},
		// The program ended. R62: a session that exists only to carry a command
		// takes the page with it — the launcher is where the user came from, and
		// a finished one-shot frame is not worth holding. A bare shell keeps its
		// resident page (the old build's own hold). The session names itself:
		// the plugin's callback runs on the terminal's goroutine, so the index is
		// resolved here, on the main thread.
		Exit: func(session *terminalui.Session, code int) {
			if session == nil {
				return
			}
			a.onMain(func() {
				index := a.Terminal.IndexOf(session)
				if index < 0 {
					return
				}
				a.Terminal.ResidentAt(index, code)
				if a.Terminal.CommandSessionEnded(index, a.Surf == SurfaceTerminal) {
					a.Open(SurfaceLauncher)
				}
			})
		},
		// A Command+click on the output opens what it points at: a page in
		// the user's browser, a path in the file manager.
		OpenLink: a.openTerminalLink,
		// A program may ask for a notification of its own (OSC 9, OSC 777):
		// it goes through the same channel the app's own completions use.
		Notify: a.notifyFromProgram,
	}
	termActions.Pin = a.PinText
	// The bar's gear opens the terminal's *own* appearance drawer, docked at
	// the panel's bottom edge: the reference's `.terminal-settings-drawer`.
	// The session is never left to change how it looks — the drawer is a
	// sibling of the canvas, so the PTY resizes while the scrollback stays.
	termActions.Settings = func() { a.Terminal.ToggleDrawer() }
	a.Terminal = terminalui.New(opts.Store, termActions, newTerminal)
	a.Terminal.SnapshotPath = filepath.Join(paths.Root, "terminal-snapshot")
	a.refreshTerminal = a.Terminal.Refresh
	if opts.RefreshTerminal != nil {
		a.refreshTerminal = opts.RefreshTerminal
	}

	// A settings change that touches the calculator's retention is applied to
	// its store, so a smaller capacity or a shorter window takes effect now.
	opts.Store.OnChange(func(s settings.Settings) {
		plugin := settings.CalculatorPluginOf(s)
		if plugin == a.lastCalculatorPlugin {
			return
		}
		a.lastCalculatorPlugin = plugin
		if a.Calculator != nil {
			_ = a.Calculator.SetRetention(plugin.MaxItems, plugin.RetentionDays)
		}
		a.onMain(func() {
			a.Launcher.CopyMode = plugin.CopyMode
			a.Launcher.RefreshCalculator()
		})
	})

	// A settings change that touches the clipboard history is applied to
	// the store and the watcher.
	opts.Store.OnChange(func(s settings.Settings) {
		state := clipboardState(s)
		if state != a.LastClipboardSettings {
			a.LastClipboardSettings = state
			a.onMain(func() {
				a.Clipboard.SetMaxItems(state.MaxItems)
				if state.Enabled && !a.clipboardWatching {
					a.watchClipboard()
				}
			})
		}
	})

	// A settings change lands on the open surface: the panel re-measures
	// (a new interface size), the terminal takes its new font and palette,
	// and the frame redraws either way.
	opts.Store.OnChange(func(s settings.Settings) {
		a.onMain(func() {
			if s.LaunchAtStartup != a.lastLaunchAtStartup {
				a.lastLaunchAtStartup = s.LaunchAtStartup
				a.ApplyStartup()
			}
			if s.GlassStep != a.lastGlassStep {
				// The glass stop is the effect ladder: a change swaps the
				// window's own material, which is the blur the panel cannot
				// run itself (see vibrancyForStop).
				a.lastGlassStep = s.GlassStep
				if a.Win != nil {
					a.Win.SetVibrancy(vibrancyForStop(runtime.GOOS, s.GlassStep))
				}
			}
			if icon := storedAppIcon(s); icon != a.lastAppIcon {
				a.lastAppIcon, a.appIcon = icon, icon
				a.ApplyAppIcon()
				// The tray carries the same mark: switching the app icon has
				// to switch the tray's, or the two disagree.
				if a.Tray != nil {
					a.Tray.Destroy()
					a.Tray = nil
					a.ApplyMenubarIcon()
				}
			}
			if s.Language != a.lastTrayLanguage {
				// The app menu and the tray outlive a language change, so
				// both are rebuilt with it: a menu that lags the UI is worse
				// than no menu.
				a.InstallMenu()
			}
			if s.Language != a.lastTrayLanguage && a.Tray != nil {
				// The tray outlives a language change, so its labels are
				// rebuilt with it: a menu that lags the UI is worse than no
				// menu.
				a.lastTrayLanguage = s.Language
				a.Tray.Destroy()
				a.Tray = nil
				a.ApplyMenubarIcon()
			}
			if settings.ShowMenubarIcon(s) != a.lastMenubarIcon {
				a.lastMenubarIcon = settings.ShowMenubarIcon(s)
				a.ApplyMenubarIcon()
			}
			if settings.ShowRecentInLauncher(s) != a.lastShowRecent {
				a.lastShowRecent = settings.ShowRecentInLauncher(s)
				a.refreshRecents()
			}
			if settings.ShowCommandsInSearch(s) != a.lastShowCommands {
				a.lastShowCommands = settings.ShowCommandsInSearch(s)
				a.scanTools()
			}
			if customShortcutSignature(s) != a.customSignature {
				a.ApplyCustomShortcuts()
			}
			if a.Surf == SurfaceTerminal {
				a.refreshTerminal()
			}
			if a.Surf != SurfaceLauncher {
				a.resize()
			}
			if a.Win != nil {
				a.Win.Invalidate()
			}
		})
	})
	return a
}

// onMain runs fn on the window's main thread, right away on the main
// goroutine and through Update otherwise.
func (a *App) onMain(fn func()) {
	if a.Win == nil {
		fn()
		return
	}
	a.Win.Update(fn)
}

// Start creates the window and installs the global shortcut. It must run
// from App.WhenReady (or later).
func (a *App) Start() {
	// The panel is a resident tray app: a terminal that launched it closing
	// sends SIGHUP to the whole session, and the framework's own handler
	// answers SIGINT and SIGTERM but leaves SIGHUP at its default - which is
	// to die. The panel ignores it instead, so the shell that started the
	// launcher cannot take it down from behind (a real Quit still runs the
	// quit listeners, through the tray, the launcher's own row, or the
	// window's close).
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			log.Printf("floter: SIGHUP received; the panel stays")
		}
	}()
	if a.workArea <= 0 {
		if displays := mygo.Screen.Displays(); len(displays) > 0 {
			a.workArea = float64(displays[0].WorkArea.Height)
		}
	}
	s := a.Store.Snapshot()
	w, h := a.targetSize(s)
	a.lastW, a.lastH = w, h

	a.Win = mygo.NewWindow(mygo.WindowOptions{
		Title:       "floter",
		Width:       w,
		Height:      h,
		Frameless:   true,
		Transparent: true,
		// The window's own shadow: macOS's window server casts one that
		// follows the card's rounded, transparent content; the other
		// platforms' would follow the window's rectangle (a square shadow
		// around rounded corners), so the card draws its own there instead.
		DisableShadow: runtime.GOOS != "darwin",
		SkipTaskbar:   true,
		// The launcher and settings are fixed panels; only the terminal
		// is resizable, which resize turns on per surface.
		DisableResize: true,
		// The desktop behind the panel is what the glass reads: the old
		// build's `backdrop-filter` smeared it (a WebKitGTK build composites
		// the window's own backdrop), and this is the framework's material
		// for the same thing - macOS's NSVisualEffectView, Windows 11's
		// system backdrops. Linux has none, and there the glass tint and
		// the haze veil are the whole of the material.
		Vibrancy: vibrancyForStop(runtime.GOOS, s.GlassStep),
		Content:  ui.View(a.View),
	})
	a.Win.OnClosed(func() {
		if runtime.GOOS != "darwin" {
			a.quit()
		}
	})
	a.Win.OnFocus(func() { a.shownAt = a.now() })
	// The panel is placed in the primary display's work area, centred, rather
	// than left to the platform's own default: a multi-display desktop would
	// otherwise open it on whichever screen the platform picks, and the
	// launcher is a summon — it belongs where the user's attention is.
	a.placeWindow()
	a.Win.OnResize(a.trackTerminalSize)
	a.Win.OnFileDrop(a.handleFileDrop)
	a.Win.OnBlur(func() {
		// A half-finished confirmation leaves with the keyboard: the old
		// build's arm was cancelled by a window blur as well as by a key.
		a.Launcher.DisarmDelete()
		// A blur right after a reveal is the platform settling; only a
		// real focused-to-unfocused leave hides the panel.
		if a.now().Sub(a.shownAt) < showGrace {
			return
		}
		if a.Store.Snapshot().HideOnBlur {
			a.Hide()
		}
	})

	accelerator := SummonShortcut(s)
	if normalized, ok := shortcuts.Normalize(accelerator); ok {
		accelerator = normalized
	}
	a.summonKey = accelerator
	a.ShortcutErr = a.registerShortcut(accelerator, a.Toggle)
	a.ApplyCustomShortcuts()
	if a.ShortcutErr != nil {
		log.Printf("floter: could not register %s: %v", accelerator, a.ShortcutErr)
	}
	if err := a.RegisterScheme(); err != nil {
		log.Printf("floter: could not register %s://: %v", scheme, err)
	}
	// Every way out saves the terminal's screen: the app-level hook covers
	// the menu's quit role, which the framework handles itself.
	mygo.App.OnQuit(func() {
		if a.Terminal != nil {
			if err := a.Terminal.SaveSnapshot(); err != nil {
				log.Printf("floter: could not save the terminal snapshot: %v", err)
			}
			// Every session's program ends with the app: one left running
			// behind a closed panel is a process nobody can reach.
			a.Terminal.CloseAll()
		}
	})
	a.ApplyStartup()
	// The old build checked for an update as soon as it mounted, so the About
	// page was already answered when the user opened it; this is that check.
	a.checkForUpdates()
	a.InstallMenu()
	a.lastMenubarIcon = settings.ShowMenubarIcon(a.Store.Snapshot())
	a.lastTrayLanguage = a.Store.Snapshot().Language
	a.lastShowRecent = settings.ShowRecentInLauncher(a.Store.Snapshot())
	a.lastShowCommands = settings.ShowCommandsInSearch(a.Store.Snapshot())
	a.ApplyMenubarIcon()
	a.ApplyAppIcon()
	a.watchClipboard()
	a.Launcher.FocusSearch()
	a.scanApps()
	a.scanTools()
	a.scanToolCatalog()
	if a.refreshIntegrations {
		go a.RefreshIntegrations(context.Background())
	}
}

// shortcutToggleWindow is the action id of the summon shortcut, as the
// settings file names it.
const shortcutToggleWindow = "toggle_window"

// setShortcut records a new accelerator for an action: the summon key is
// re-registered, and the settings file keeps it, so the next launch answers
// the same way.
//
// A registration the system refuses (another app holds the combination)
// leaves the old one in place, and nothing is written.
func (a *App) setShortcut(id, accelerator string) {
	normalized, ok := shortcuts.Normalize(accelerator)
	if !ok {
		log.Printf("floter: %q is not a shortcut", accelerator)
		return
	}
	if id != shortcutToggleWindow {
		// The app's own keys are handled by the window, not the system: they
		// only have to be stored (and be one this build can parse).
		probe := settings.Default()
		if !probe.SetShortcut(id, normalized) {
			log.Printf("floter: %q is not an action this build knows", id)
			return
		}
		if err := a.Store.Update(func(s *settings.Settings) { s.SetShortcut(id, normalized) }); err != nil {
			log.Printf("floter: could not save %s: %v", id, err)
		}
		a.onMain(func() {
			if a.Win != nil {
				a.Win.Invalidate()
			}
			// The app menu advertises `open_settings` on macOS; re-installing
			// it keeps the item's accelerator the user's own binding rather
			// than the one that shipped.
			if id == settings.ShortcutOpenSettings {
				a.InstallMenu()
			}
		})
		return
	}
	if normalized == a.summonKey {
		return
	}
	if err := a.registerShortcut(normalized, a.Toggle); err != nil {
		log.Printf("floter: could not register %s: %v", normalized, err)
		return
	}
	if a.summonKey != "" {
		mygo.GlobalShortcut.Unregister(a.summonKey)
	}
	a.summonKey = normalized
	a.Settings.Shortcut = normalized
	a.ShortcutErr = nil

	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetExtra("hotkey", normalized)
		shortcutsMap, _ := s.Extra()["shortcuts"].(map[string]any)
		updated := map[string]any{}
		for key, value := range shortcutsMap {
			updated[key] = value
		}
		updated[shortcutToggleWindow] = normalized
		s.SetExtra("shortcuts", updated)
	}); err != nil {
		log.Printf("floter: could not save %s: %v", normalized, err)
	}
}

// scanTools lists the commands on the PATH, in the background, when the
// show_commands_in_search setting asks for them.
func (a *App) scanTools() {
	snapshot := a.Store.Snapshot()
	// The aliases belong to the catalog, not the PATH scan: resolve them
	// before the PATH rows are decided, so a user who hides the PATH commands
	// still reaches an extension command by its alias.
	a.Launcher.SetCommandAliases(settings.ResolveCommandAliases(settings.CommandAliasesOf(snapshot)))
	if !settings.ShowCommandsInSearch(snapshot) {
		a.Launcher.SetTools(nil, false)
		return
	}
	go func() {
		found := apps.ScanCommands(apps.CommandDirs())
		a.onMain(func() { a.Launcher.SetTools(found, true) })
	}()
}

// scanToolCatalog resolves the install catalog against this machine's search
// path, in the background: detection is a stat scan, and the rows only matter
// once the launcher is open.
//
// The catalog is read one scan at a time: a refresh that arrives while one is
// already in flight (a summon landing on a Rescan, the way back from an
// install session landing on a reveal) shares that scan rather than starting a
// second one, exactly as the old build's `refreshToolCatalog` reused its
// in-flight promise.
func (a *App) scanToolCatalog() {
	a.catalogMu.Lock()
	if a.catalogScanning {
		a.catalogMu.Unlock()
		return
	}
	a.catalogScanning = true
	a.catalogMu.Unlock()
	go func() {
		states := a.lookTools()
		a.onMain(func() {
			a.Launcher.SetToolCatalog(states)
			a.catalogMu.Lock()
			a.catalogScanning = false
			a.catalogMu.Unlock()
		})
	}()
}

// launcherCommands is the extensions' commands the user has switched on. A
// command with no switch entry has never been enabled, so the launcher does
// not offer it: the per-command switches are the gate, exactly as the old
// build's integrations panel was.
func (a *App) launcherCommands() []extensions.CommandEntry {
	switches := settings.CommandSwitchesOf(a.Store.Snapshot())
	var out []extensions.CommandEntry
	for _, entry := range a.Integrations.CommandEntries() {
		if switches.Enabled(entry.IntegrationID, entry.Command.ID) {
			out = append(out, entry)
		}
	}
	return out
}

// refreshRecents hands the launcher the most-launched applications, limited
// to the ones that are still installed, and whether the empty query should
// show them. The ranking mirrors the old build's `recentItems`
// (src/launcher.ts): count descending, and between equals the application
// list's own scan order — `b.count - a.count || a.index - b.index`. `usage.Top`
// breaks the tie by recency instead, so the scan order is re-applied here,
// where the application list is known.
func (a *App) refreshRecents() {
	order := map[string]int{}
	for i, app := range a.Launcher.Apps {
		if _, seen := order[app.Path]; !seen {
			order[app.Path] = i
		}
	}
	paths := a.Usage.Top(0, func(path string) bool { _, ok := order[path]; return ok })
	sort.SliceStable(paths, func(i, j int) bool {
		left, right := a.Usage.Count(paths[i]), a.Usage.Count(paths[j])
		if left != right {
			return left > right
		}
		return order[paths[i]] < order[paths[j]]
	})
	if len(paths) > maxRecentApps {
		paths = paths[:maxRecentApps]
	}
	a.Launcher.SetRecent(paths, a.showRecent())
}

// showRecent is the show_recent_in_launcher setting, on by default.
func (a *App) showRecent() bool {
	value, ok := a.Store.Snapshot().Extra()["show_recent_in_launcher"].(bool)
	if !ok {
		return true
	}
	return value
}

// maxRecentApps is how many recent applications the launcher offers: the old
// build's whole result budget (`MAX_RESULTS` = 10 in src/launcher/
// result-budget.ts), since the empty query is the recents' own page.
const maxRecentApps = 10

// RefreshIntegrations reloads the extension state and the providers'
// commands, then hands them to the launcher. It blocks while providers run,
// so the app calls it from a goroutine. The upstream command-drift check runs
// after it, on the goroutine it spawns itself.
func (a *App) RefreshIntegrations(ctx context.Context) {
	a.Integrations.Refresh(ctx)
	a.onMain(func() { a.Launcher.SetCommands(a.launcherCommands()) })
	// R7-7 · the silent drift re-probe. The list is already assembled from the
	// previous descriptor, so the check runs behind it: a slow or hung tool
	// can never hold up a listing.
	a.reprobeDriftedIntegrations()
}

// integrationList maps the store's inventory onto the settings list: the
// repository's record, the manifest's copy, and why a provider failed.
func (a *App) integrationList() []settingsui.Integration {
	inventory := a.Integrations.Inventory()
	switches := settings.CommandSwitchesOf(a.Store.Snapshot())
	aliases := settings.CommandAliasesOf(a.Store.Snapshot())
	resolved := settings.ResolveCommandAliases(aliases)
	commands := map[string][]settingsui.Command{}
	for _, info := range a.Integrations.CommandRegistry() {
		alias := aliases[info.CommandID]
		_, live := resolved[info.CommandID]
		commands[info.ExtensionID] = append(commands[info.ExtensionID], settingsui.Command{
			ID:          info.CommandID,
			Name:        info.Name,
			Description: info.Description,
			Enabled:     switches.Enabled(info.ExtensionID, info.CommandID),
			Available:   info.Available,
			Alias:       alias,
			Taken:       strings.TrimSpace(alias) != "" && !live,
		})
	}
	out := make([]settingsui.Integration, 0, len(inventory.Integrations))
	for _, integration := range inventory.Integrations {
		item := settingsui.Integration{
			ID:          integration.Entry.ID,
			Name:        integration.Name,
			Description: integration.Description,
			Publisher:   integration.Publisher,
			Version:     integration.Version,
			ToolVersion: integration.ToolVersion,
			Enabled:     integration.Entry.Enabled,
			Running:     integration.Running(),
			Broken:      integration.Broken(),
			// The repository's kind fields, so the card's removal verb is the
			// reference's four-way pick rather than one generic word.
			DistributionSource: integration.Entry.DistributionSource,
			RuntimeOwnership:   integration.Entry.RuntimeOwnership,
		}
		// The runtime the entry was bound to, resolved again for this frame: a
		// row whose program moved or vanished offers the reference's repair
		// action instead of a run it could not perform
		// (`src/extensions/ExtensionRow.tsx:345-370`).
		if _, err := extensions.ResolveRuntime(integration); err == nil {
			item.RuntimeAvailable = true
		}
		// The project's own page, when the manifest names one: the repair a row
		// whose runtime floter does not own can only point at.
		item.Homepage = integration.Manifest.Homepage
		// The `floter://register` mark rides the card whose executable the link
		// named, nothing else (see markRegisterTarget).
		if a.registerTarget != "" && sameExecutable(integration.Entry.ExecutablePath, a.registerTarget) {
			item.Highlighted = true
		}
		if integration.Entry.LastErrorCode != nil {
			item.FailureCode = *integration.Entry.LastErrorCode
		}
		// The recorded reason is the detail the row words; the keyed
		// `lastErrorDetail` is the fallback for an entry that has no reason yet.
		switch {
		case integration.Entry.BrokenReason != nil:
			item.FailureDetail = *integration.Entry.BrokenReason
		case integration.Entry.LastErrorDetail != nil:
			item.FailureDetail = *integration.Entry.LastErrorDetail
		}
		for _, permission := range integration.Manifest.Permissions {
			if !knownPermission(permission) {
				continue
			}
			item.Permissions = append(item.Permissions, permission)
		}
		if err := a.Integrations.DescribeError(integration.Entry.ID); err != nil {
			item.Error = err.Error()
		}
		if len(item.Permissions) > 0 {
			item.Enforced = map[string]bool{}
			for _, permission := range item.Permissions {
				item.Enforced[permission] = extensions.PermissionEnforced(permission)
			}
		}
		// The approval record (R7-8): what the recorded approval covered, when,
		// and against which manifest — the on-disk digest read beside it, so
		// "what I approved" and "what is here now" are two readable facts.
		// One read serves the digest only; an unreadable manifest leaves the
		// digest empty, and "unknown" stays distinguishable from "changed".
		if integration.Entry.ApprovedAt != 0 {
			approval := &settingsui.ApprovalRecord{At: integration.Entry.ApprovedAt}
			if integration.Entry.ApprovedManifestDigest != nil {
				approval.ApprovedDigest = *integration.Entry.ApprovedManifestDigest
			}
			if digest, err := extensions.ManifestDigest(integration.ManifestPath()); err == nil {
				approval.CurrentDigest = digest
			}
			for _, permission := range integration.Entry.ApprovedPermissions {
				if extensions.PermissionEnforced(permission) {
					approval.Enforced = append(approval.Enforced, permission)
				} else {
					approval.Disclosure = append(approval.Disclosure, permission)
				}
			}
			item.Approval = approval
		}
		item.Commands = commands[item.ID]
		// The manifest's declared inputs, for the card's run form.
		for _, param := range integration.Manifest.Params {
			item.Params = append(item.Params, settingsui.Param{
				ID: param.ID, Label: param.Label, Kind: param.Kind,
				Default: param.Default, Required: param.Required,
				Placeholder: param.Placeholder, Options: param.Options, Flag: param.Flag,
			})
		}
		item.OutputMode = integration.Manifest.Route()
		// Only the host's own generated integrations can be re-probed: the
		// button rebuilds a descriptor it wrote, never publisher content.
		item.Generated = integration.Entry.PublisherID == "local-user" &&
			integration.Entry.ProviderKind == "static-descriptor"
		// The stored lifecycle health survives across runs, so a row shows how
		// the tool was last found to be even before a check runs.
		if report := integration.Entry.ProbeReport; report != nil && report.Status != "" {
			copy := i18n.For(a.Store.Snapshot().Language).Settings
			switch report.Status {
			case extensions.HealthHealthy:
				item.Health = copy.IntegrationsHealthy
			case extensions.HealthDegraded:
				item.Health = copy.IntegrationsDegraded
			case extensions.HealthUnhealthy:
				item.Health = copy.IntegrationsUnhealthy
			}
			item.HealthReport = healthReportView(report, integration.Manifest.Lifecycle.Probes)
		}
		if a.Integrations == nil {
			log.Printf("floter: the extension store is nil in integrationList")
		}
		applyIntegrationConfig(a.Integrations, a.Paths, &item, integration)
		a.diagnosisMu.Lock()
		if diagnosis, ok := a.diagnoses[item.ID]; ok {
			item.Diagnosis, item.DiagnosisFailed = diagnosis.Diagnosis, diagnosis.DiagnosisFailed
			item.Health = diagnosis.Health
		}
		a.diagnosisMu.Unlock()
		out = append(out, item)
	}
	for _, id := range inventory.Orphans {
		out = append(out, settingsui.Integration{ID: id, Name: id, Orphan: true})
	}
	return out
}

// healthReportView projects the repository's stored probe report onto the
// settings card's own shape. `retryable` is the manifest's `required`
// inverted — a failed required probe is a broken tool, an optional one is a
// transient failure worth retrying — so the report never has to persist the
// same fact twice (and the disk format stays the old build's).
func healthReportView(report *extensions.HealthReport, schema []extensions.Probe) *settingsui.HealthReport {
	required := make(map[string]bool, len(schema))
	for _, probe := range schema {
		required[probe.ID] = probe.Required
	}
	view := &settingsui.HealthReport{CheckedAt: report.CheckedAt}
	for _, record := range report.Probes {
		view.Probes = append(view.Probes, settingsui.HealthProbe{
			ID:         record.ID,
			Passed:     record.Passed,
			DurationMs: record.DurationMs,
			ExitCode:   record.ExitCode,
			Stderr:     record.Stderr,
			Retryable:  !required[record.ID],
		})
	}
	return view
}

// diagnoseIntegration asks a provider to check itself, off the main thread,
// and remembers what it said for the list.
func (a *App) diagnoseIntegration(id string) {
	go func() {
		integration, ok := a.Integrations.Inventory().WithID(id)
		if !ok {
			return
		}
		result := settingsui.Integration{Diagnosis: "ok"}
		// The lifecycle probes run first: they are the tool's own health
		// check, and a required failure marks the integration broken. The
		// diagnose protocol runs after it, when the integration is still
		// usable.
		if len(integration.Manifest.Lifecycle.Probes) > 0 {
			report, err := extensions.Reprobe(context.Background(), a.Paths, id)
			switch {
			case errors.Is(err, extensions.ErrNoProbes):
			case err != nil:
				result.Diagnosis, result.DiagnosisFailed = err.Error(), true
			default:
				result.Diagnosis, result.DiagnosisFailed = report.Status, report.Status != extensions.HealthHealthy && report.Status != extensions.HealthDegraded
				a.diagnosisMu.Lock()
				if a.diagnoses == nil {
					a.diagnoses = map[string]settingsui.Integration{}
				}
				a.diagnoses[id] = result
				a.diagnosisMu.Unlock()
				a.onMain(func() {
					if a.Win != nil {
						a.Win.Invalidate()
					}
				})
			}
		}
		diagnosis, err := extensions.Diagnose(context.Background(), integration)
		switch {
		case err != nil:
			result.Diagnosis, result.DiagnosisFailed = err.Error(), true
		default:
			var problems []string
			for _, check := range diagnosis.Checks {
				if check.Status != "ok" {
					message := check.Message
					if message == "" {
						message = check.ID
					}
					problems = append(problems, message)
				}
			}
			if diagnosis.Status != "" && diagnosis.Status != "ok" || len(problems) > 0 {
				result.DiagnosisFailed = true
				result.Diagnosis = strings.Join(problems, "; ")
				if result.Diagnosis == "" {
					result.Diagnosis = diagnosis.Status
				}
			} else if diagnosis.Status != "" {
				result.Diagnosis = diagnosis.Status
			} else {
				result.Diagnosis = "ok"
			}
		}
		// The stored lifecycle health survives across runs, so a row shows how
		// the tool was last found to be even before a check runs.
		if report := integration.Entry.ProbeReport; report != nil && report.Status != "" {
			copy := i18n.For(a.Store.Snapshot().Language).Settings
			switch report.Status {
			case extensions.HealthHealthy:
				result.Health = copy.IntegrationsHealthy
			case extensions.HealthDegraded:
				result.Health = copy.IntegrationsDegraded
			case extensions.HealthUnhealthy:
				result.Health = copy.IntegrationsUnhealthy
			}
		}
		a.diagnosisMu.Lock()
		if a.diagnoses == nil {
			a.diagnoses = map[string]settingsui.Integration{}
		}
		a.diagnoses[id] = result
		a.diagnosisMu.Unlock()
		a.notifyCompletion(func(c i18n.Notifications) string {
			if result.DiagnosisFailed {
				return c.IntegrationCheckFailed(integration.Name)
			}
			return c.IntegrationChecked(integration.Name)
		})
		// The probes may have written a report (and marked the integration
		// broken): the list is re-read from the repository, so the row shows
		// it.
		if len(integration.Manifest.Lifecycle.Probes) > 0 {
			a.Integrations.Refresh(context.Background())
		}
		a.onMain(func() {
			if a.Win != nil {
				a.Win.Invalidate()
			}
		})
	}()
}

// reconnectIntegration re-derives a system integration's runtime binding from
// the search path — the old host's `extensions_reconnect_system`, which the
// row offers when the program it was bound to is no longer there. The notice
// is the reference's repaired/rechecked split: a record that changed says so,
// one that was already right says the tool is working (`ExtensionsPanel.tsx`,
// `report.repaired ? repairedNotice : recheckedNotice`).
func (a *App) reconnectIntegration(id string) {
	name := id
	var manifest extensions.Manifest
	recorded := ""
	known := false
	if integration, ok := a.Integrations.Inventory().WithID(id); ok {
		known = true
		manifest = integration.Manifest
		recorded = integration.Entry.ExecutablePath
		if integration.Name != "" {
			name = integration.Name
		}
	}
	go func() {
		// More than one matching program is the reference's chooser: the row
		// asks which executable to bind before it re-derives anything
		// (`ExtensionsPanel.tsx:1975-1988`).
		path := ""
		if known {
			chosen, ok := a.recommendedToolPath(name, manifest, recorded)
			if !ok {
				return
			}
			path = chosen
		}
		report, err := extensions.ReconnectAt(a.Paths, id, path)
		// The list is re-read first, as the reference's `refreshAfterMutation`
		// did before its toast: a reconnect may have rewritten the entry (a new
		// executable path, or a cleared binding failure), and the notice must
		// not announce a state the page has not caught up with yet.
		a.Integrations.Refresh(context.Background())
		switch {
		case err != nil:
			log.Printf("floter: could not reconnect %s: %v", id, err)
			a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationCheckFailed(name) })
		case report.Reconnected:
			a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationReconnected(name) })
		default:
			a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationChecked(name) })
		}
		a.onMain(func() {
			if a.Win != nil {
				a.Win.Invalidate()
			}
		})
	}()
}

// repairIntegration re-checks an installed package and repairs it when the
// record drifted from what is on disk — the old host's `extensions_repair`,
// behind the Wrench a broken row shows. The notice is the reference's
// repaired/rechecked split (`src/ExtensionsPanel.tsx:1255-1267`,
// `report.repaired ? repairedNotice : recheckedNotice`).
func (a *App) repairIntegration(id string) {
	name := id
	if integration, ok := a.Integrations.Inventory().WithID(id); ok && integration.Name != "" {
		name = integration.Name
	}
	go func() {
		report, err := extensions.Repair(a.Paths, id)
		// The list is re-read before the notice, as `refreshAfterMutation` did
		// in the reference: a repair may have rewritten the entry, and the
		// notice must not announce a state the page has not caught up with.
		a.Integrations.Refresh(context.Background())
		switch {
		case err != nil:
			log.Printf("floter: could not re-check %s: %v", id, err)
			a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationCheckFailed(name) })
		case report.Repaired:
			a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationRepaired(name) })
		default:
			a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationRechecked(name) })
		}
		a.onMain(func() {
			if a.Win != nil {
				a.Win.Invalidate()
			}
		})
	}()
}

// openHomepage hands a project's own page to the system's default browser. The
// integrations page's "Install tool first" is the only caller: a row whose
// runtime floter does not own can point the user at the project and nothing
// more, which is exactly what the reference's `installTool` arm did.
func (a *App) openHomepage(url string) {
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	// The system's default browser, not one of the configured browser plugins:
	// a project page is not a search, and the user's own choice of browser is
	// the one that should open it (the reference opened the page the same way,
	// `onRepair`'s `openExternal`).
	if err := a.openURL("", url); err != nil {
		log.Printf("floter: could not open %s: %v", url, err)
		// The reference's `errorMessage(error, t)` localizes the one keyed
		// refusal (`settings.extensions.openNonWebUrl`) and passes every other
		// backend error through as it was written; both become an error toast
		// (`ExtensionsPanel.tsx:2577-2580`).
		message := err.Error()
		if errors.Is(err, errNonWebURL) {
			message = i18n.For(a.Store.Snapshot().Language).Settings.OpenNonWebURL(url)
		}
		a.queueNotice(message, refusalNotice)
	}
}

// searchBrowser searches the browser the settings name, off the main thread,
// and hands the answer back to it.
//
// The file read (bookmarks and history) is published the moment it is in; the
// live-tab read is the one that can be slow — AppleScript against a busy
// browser, up to its own three-second deadline — so it fills its group in with
// a second answer instead of holding the first one back. A plugin list that
// arrives a second late reads as a plugin list that does not work.
func (a *App) searchBrowser(query string, done func(launcher.BrowserResults)) {
	options := browserState(a.Store.Snapshot())
	if !options.enabled {
		// The plugin is off: the mode says so instead of claiming the machine
		// has no browser at all.
		a.onMain(func() { done(launcher.BrowserResults{Disabled: true}) })
		return
	}
	go func() {
		profiles := a.browserProfileList(options.customBaseDir)
		profile, found := browser.DefaultProfile(profiles, options.target)
		if !found {
			a.onMain(func() { done(launcher.BrowserResults{}) })
			return
		}
		// The backend's own per-profile SQL window bounds the read
		// (`internal/browser/browser.go:411`); the mode decides what to *hold*
		// (`launcher.BrowserFetchLimit`, the reference's `BROWSER_FETCH_LIMIT`).
		// Capping here instead would starve the mode's range chip: the cap would
		// run before the chip narrows the set, so a browser full of bookmarks
		// would leave the `history` chip with nothing to show.
		results := browser.Search(context.Background(), []browser.Profile{profile}, query, browser.Options{
			Days:      options.historyDays,
			SortOrder: browser.SortOrder(options.sortOrder),
			Field:     browser.SearchField(options.searchField),
		})
		answer := launcher.BrowserResults{Found: true, Profile: profile, Results: results}
		a.onMain(func() { done(answer) })

		tabs, err := browser.Tabs(context.Background(), profile.BrowserID, options.cdpEnabled, options.cdpPort)
		if err != nil {
			// An unreachable tab source is the expected state, not an error
			// the user needs: the guidance lives in the plugin's settings.
			log.Printf("floter: browser tabs: %v", err)
			return
		}
		tabs = browser.FilterTabs(tabs, query, browser.SearchField(options.searchField))
		answer.Tabs = tabs
		a.onMain(func() { done(answer) })
	}()
}

// browserProfileList discovers the browser profiles once per custom base
// directory: a settings change to that directory rediscovers.
func (a *App) browserProfileList(customBase string) []browser.Profile {
	a.browserMu.Lock()
	defer a.browserMu.Unlock()
	if a.browserLoaded && a.browserBase == customBase {
		return a.browserProfiles
	}
	home := a.homeDir
	if home == "" {
		found, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		home = found
	}
	a.browserLoaded, a.browserBase = true, customBase
	a.browserProfiles = browser.ProfilesIn(home, customBase)
	return a.browserProfiles
}

// resetShortcuts restores the shipped bindings: the app's own map is dropped
// (so every action falls back to its default) and the summon key goes back to
// the default one, re-registered with the system.
func (a *App) resetShortcuts() {
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetExtra("shortcuts", map[string]any{})
		s.SetExtra("hotkey", settings.DefaultSummonShortcut)
	}); err != nil {
		log.Printf("floter: could not reset the shortcuts: %v", err)
		return
	}
	a.ApplySummonShortcut()
	a.ApplyCustomShortcuts()
}

// ApplySummonShortcut re-registers the global summon key from the settings,
// releasing the previous one. A key the system refuses leaves the old one in
// place: a shortcut that stops working because of a rejected change would be
// worse than one that did not change.
func (a *App) ApplySummonShortcut() {
	accelerator := shortcuts.NormalizeOr(SummonShortcut(a.Store.Snapshot()))
	if accelerator == a.summonKey {
		return
	}
	if err := a.registerShortcut(accelerator, a.Toggle); err != nil {
		log.Printf("floter: could not register %s: %v", accelerator, err)
		return
	}
	if a.summonKey != "" {
		a.unregisterShortcut(a.summonKey)
	}
	a.summonKey = accelerator
	a.Settings.Shortcut = accelerator
	a.ShortcutErr = nil
}

// setShortcutsSuspended releases the global shortcuts while a settings
// recorder waits for a key, and takes them back afterwards: pressing the key
// that currently summons floter must be recorded, not acted on.
func (a *App) setShortcutsSuspended(suspended bool) {
	if suspended {
		if a.summonKey != "" {
			a.unregisterShortcut(a.summonKey)
		}
		for _, key := range a.customKeys {
			a.unregisterShortcut(key)
		}
		a.customKeys = nil
		a.shortcutsSuspended = true
		return
	}
	a.shortcutsSuspended = false
	if a.summonKey != "" {
		if err := a.registerShortcut(a.summonKey, a.Toggle); err != nil {
			log.Printf("floter: could not re-register %s: %v", a.summonKey, err)
		}
	}
	a.ApplyCustomShortcuts()
}

// saveIntegrationConfiguration validates and stores one integration's
// configuration: the values go through the same validation the old build ran,
// and the password fields land in a new secrets generation rather than in the
// values file.
func (a *App) saveIntegrationConfiguration(id string, values map[string]any) error {
	description, ok := a.Integrations.Description(id)
	if !ok {
		return errors.New("extensions: no such integration")
	}
	if err := extensions.SaveConfiguration(a.Paths, id, description.Configuration.Schema, values); err != nil {
		return err
	}
	// The provider sees the new values the next time it is asked.
	go a.RefreshIntegrations(context.Background())
	return nil
}

// applyIntegrationConfig copies an integration's configuration schema and its
// stored values onto the settings row. The whole thing is guarded: a schema
// this build cannot render must not take the list down.
func applyIntegrationConfig(store *extensions.Store, paths extensions.Paths, item *settingsui.Integration, integration extensions.Integration) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("floter: could not read the configuration of %s: %v", item.ID, r)
		}
	}()
	// The schema comes from the provider's description, whose configuration
	// block is optional: an integration without one has nothing to render.
	description, ok := store.Description(item.ID)
	if !ok || description.Configuration == nil || len(description.Configuration.Schema) == 0 {
		return
	}
	configuration := description.Configuration
	for _, field := range configuration.Schema {
		item.Config = append(item.Config, settingsui.ConfigField{
			Key:         field.Key,
			Label:       field.Label,
			Type:        field.Type,
			Description: field.Description,
			Required:    field.Required,
			Default:     field.Default,
			Options:     optionsAsStrings(field.Options),
			Minimum:     field.Minimum,
			Maximum:     field.Maximum,
			MinLength:   field.MinLength,
			MaxLength:   field.MaxLength,
		})
	}
	stored, err := extensions.LoadStoredConfiguration(paths.Data, item.ID)
	if err != nil {
		return
	}
	item.ConfigValues = stored.Values
}

// optionsAsStrings renders a field's options for the form: a select's options
// are text values in the schema, and anything else is shown as its text.
func optionsAsStrings(options []any) []string {
	out := make([]string, 0, len(options))
	for _, option := range options {
		switch value := option.(type) {
		case string:
			out = append(out, value)
		case float64:
			out = append(out, strconv.FormatFloat(value, 'f', -1, 64))
		case bool:
			out = append(out, strconv.FormatBool(value))
		default:
			if text, err := json.Marshal(option); err == nil {
				out = append(out, string(text))
			}
		}
	}
	return out
}

// connectFromLink installs (after the user's approval) a manifest a
// `floter://connect` link carried. A relative path resolves against the home
// directory, and a missing or unreadable manifest is refused rather than
// installed.
func (a *App) connectFromLink(manifest string) {
	go func() {
		expanded := manifest
		if rest, ok := strings.CutPrefix(expanded, "~"); ok {
			home, _ := os.UserHomeDir()
			expanded = filepath.Join(home, strings.TrimPrefix(rest, "/"))
		} else if !filepath.IsAbs(expanded) {
			home, _ := os.UserHomeDir()
			expanded = filepath.Join(home, expanded)
		}
		info, err := os.Stat(expanded)
		if err != nil {
			log.Printf("floter: the connected manifest is not there: %v", err)
			a.refuseLink()
			return
		}
		staged := expanded
		if !info.IsDir() {
			// The install takes a package directory: the manifest's own
			// directory is the package.
			staged = filepath.Dir(expanded)
		}
		prepared, err := extensions.PrepareLocal(a.Paths, staged)
		if err != nil {
			log.Printf("floter: could not review %s: %v", expanded, err)
			a.refuseLink()
			return
		}
		approved := true
		if prepared.Approval.NeedsApproval() {
			approved = a.confirmInstallPermissions(prepared.Approval)
		}
		entry, err := prepared.Commit(approved)
		if err != nil {
			log.Printf("floter: %s was not connected: %v", prepared.Name(), err)
			return
		}
		log.Printf("floter: connected %s %s", entry.Name, entry.PackageVersion)
		a.RefreshIntegrations(context.Background())
	}()
}

// clearClipboardHistory drops every clipboard entry that is not a favourite,
// after the user has confirmed it: the removal cannot be undone. The confirm
// is injectable, so a test answers for itself.
func (a *App) clearClipboardHistoryAsks(confirm func(title string) bool) {
	go func() {
		copy := a.SettingsCopy()
		if !confirm(copy.ClipboardClearTitle) {
			return
		}
		a.onMain(func() { a.clearClipboardHistory() })
	}()
}

// openInstallSession opens a bare terminal session with the install command
// typed into it: the shell is the user's (their proxy, mirror and environment
// already in effect), and it outlives the install.
func (a *App) openInstallSession(command string) {
	a.Terminal.RunShellWithCommand(command)
	a.Open(SurfaceTerminal)
}

// openShellCommand opens the launcher's shell-syntax line in the user's own
// shell. The session carries the line rather than merely hosting it: when the
// shell ends, the terminal page leaves with it (R62, see Terminal.RunCommandLine).
func (a *App) openShellCommand(command string) {
	a.Terminal.RunCommandLine(command)
	a.Open(SurfaceTerminal)
}

// recommendedTools is the shipped packages and whether the inventory has
// them, for the Integrations page's connect rows.
func (a *App) recommendedTools() []settingsui.RecommendedTool {
	installed := recommended.Installed(a.Integrations.Inventory())
	out := make([]settingsui.RecommendedTool, 0, len(recommended.Tools))
	for _, tool := range recommended.Tools {
		out = append(out, settingsui.RecommendedTool{
			ID:          tool.ID,
			Name:        tool.Name,
			Description: tool.Description,
			Installed:   installed[tool.ID],
		})
	}
	return out
}

// connectRecommended installs one of the shipped tool packages: its files are
// materialized and handed to the same local install every other tool goes
// through, so its permissions are still reviewed.
func (a *App) connectRecommended(id string) {
	go func() {
		prepared, cleanup, err := recommended.Prepare(a.Paths, id)
		if err != nil {
			log.Printf("floter: could not prepare %s: %v", id, err)
			return
		}
		defer cleanup()
		// Resolve the program first, exactly where the reference's
		// `connectRecommended` did (`ExtensionsPanel.tsx:1393-1401`): a manifest
		// whose declared names match more than one program asks which one, and
		// the resolved path is what both the review and the install carry.
		if path, ok := a.recommendedToolPath(prepared.Manifest.Name, prepared.Manifest, ""); !ok {
			return
		} else if path != "" {
			prepared.ExecutablePath = path
		}
		// The reference asks for the manifest's own permissions first and only
		// shows the review when there are any (ExtensionsPanel.tsx:1349-1400);
		// its confirm connects with the whole declared set approved.
		if len(prepared.Approval.Declared) > 0 && !a.reviewRecommendedConnect(prepared) {
			return
		}
		entry, err := prepared.Commit(true)
		if err != nil {
			log.Printf("floter: could not connect %s: %v", id, err)
			a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstallFailed(id) })
			return
		}
		a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstalled(entry.Name) })
		a.RefreshIntegrations(context.Background())
	}()
}

// recommendedToolPath resolves the program a system manifest's declared names
// should bind to. Exactly one match is the answer; several are the reference's
// `chooseSystemTool` dialog, and the user's pick is the answer; none leaves the
// path empty, so the install reports the missing runtime the way it always has.
// ok is false only when the user cancelled the chooser.
func (a *App) recommendedToolPath(name string, manifest extensions.Manifest, recordedPath string) (string, bool) {
	candidates := extensions.SystemRuntimeCandidates(manifest, recordedPath)
	switch len(candidates) {
	case 0:
		return "", true
	case 1:
		return candidates[0].Path, true
	default:
		return a.chooseSystemTool(name, candidates)
	}
}

// detectedTools is the programs this machine has that nothing has connected
// yet: the inventory's own ranking, cut to the ones a person would recognise
// and capped so the page stays a list of what is installed.
func (a *App) detectedTools() []settingsui.DetectedTool {
	candidates := inventory.Shared().Candidates()
	connected := map[string]bool{}
	for _, integration := range a.Integrations.Inventory().Integrations {
		if path := integration.Entry.ExecutablePath; path != "" {
			connected[path] = true
			if resolved, err := filepath.EvalSymlinks(path); err == nil {
				connected[resolved] = true
			}
		}
	}
	out := make([]settingsui.DetectedTool, 0, maxDetectedTools)
	dirs := extensions.SearchDirectories()
	platform := tools.Platform()
	managers := tools.DetectManagers(dirs)
	for _, candidate := range candidates {
		if !candidate.Available || candidate.Path == "" || connected[candidate.Path] {
			continue
		}
		// A GUI bundle's own helper executables are not "tools a person
		// connects": the curated ranking already puts the recognisable names
		// first, so the cap collects them.
		out = append(out, settingsui.DetectedTool{
			Name:             candidate.Name,
			Path:             candidate.Path,
			Description:      candidate.Description,
			RuntimeAvailable: candidate.Available,
			InstallCommand:   detectedInstallCommand(candidate, platform, managers),
		})
		if len(out) >= maxDetectedTools {
			break
		}
	}
	return out
}

// maxDetectedTools is how many discovered programs the page offers: the
// inventory knows thousands, and a page that lists them all is a file browser,
// not a settings panel.
const maxDetectedTools = 12

// detectedInstallCommand is the recipe command that installs the tool a
// discovered candidate is, empty when the catalog does not know it. The
// reference matched the row's id against the catalog's own ids
// (`src/extensions/tool-rows.ts`'s `installCommandForId`); a discovered
// candidate carries no catalog id, so the match is the executable's name
// against the entry's probes — the same names the catalog uses to detect the
// tool. The recipe is chosen by a manager the machine actually has, exactly as
// the launcher's install rows do.
func detectedInstallCommand(candidate inventory.Candidate, platform string, managers []string) string {
	name := strings.ToLower(executableName(candidate.Path))
	for _, entry := range tools.Catalog {
		for _, probe := range entry.Probes {
			if strings.ToLower(probe) != name {
				continue
			}
			recipe, ok := tools.ChosenRecipe(entry, platform, managers)
			if !ok {
				return ""
			}
			return tools.InstallCommand(recipe.Manager, recipe.Package)
		}
	}
	return ""
}

// runWithParams runs an integration's command with the answers to its declared
// inputs: the host turns them into argv (never into a command string), and the
// run goes to the surface the manifest's output mode names — a terminal route
// streams through the PTY, a background route runs headless and leaves its
// output on the integration's row.
//
// A combination the manifest would not allow is refused here with a
// `run_param_*` key, and the surface words it; the run never proceeds with a
// partially-built argv.
func (a *App) runWithParams(id string, values map[string]string) *extensions.ParamFailure {
	integration, ok := a.Integrations.Inventory().WithID(id)
	if !ok {
		return nil
	}
	for _, entry := range a.Integrations.CommandEntries() {
		if entry.IntegrationID != id {
			continue
		}
		argv, err := extensions.ParamArguments(
			integration.Manifest.Params, values, extensions.ProgramIsCmdScript(entry.Program))
		if err != nil {
			var failure *extensions.ParamFailure
			if errors.As(err, &failure) {
				return failure
			}
			return nil
		}
		if integration.Manifest.Route() == extensions.RouteTerminal {
			a.runCommand(entry, argv)
			return nil
		}
		a.runCommandFromPanel(entry, argv)
		return nil
	}
	log.Printf("floter: %s declares parameters but contributes no command", id)
	return nil
}

// runCommandFromPanel runs a panel run whose manifest sends its output to the
// background: headless with both streams captured, and the result recorded on
// the integration's row for its "Last output" block (the old panel's
// `runOutputs`). A run that finishes while the panel is hidden still raises
// the usual completion notification.
func (a *App) runCommandFromPanel(entry extensions.CommandEntry, args []string) {
	a.runCommandCaptured(entry, args, func(run extensions.CapturedRun, err error) {
		a.Settings.SetRunResult(entry.IntegrationID, settingsui.RunResult{
			Stdout:    run.Stdout,
			Stderr:    run.Stderr,
			Truncated: run.Truncated,
			Success:   run.Success,
		})
	})
}

// installPackageFolder installs a local package directory: the reference's
// folder flow, which always shows the package review — what the package is
// and what it asks for — and only its own confirm commits. The review is the
// permission review and the manifest summary together (the reference's
// `extensions_local_manifest_review` → `LocalInstallDialog`, ExtensionsPanel.tsx
// :1430-1440,1550-1560; LocalInstallDialog.tsx), so a package that declares
// nothing still gets its source/runtime/platforms read back before install.
func (a *App) installPackageFolder(dir string) {
	go func() {
		prepared, err := extensions.PrepareLocal(a.Paths, dir)
		if err != nil {
			log.Printf("floter: could not review %s: %v", dir, err)
			a.onMain(func() {
				a.Launcher.WarnFeedback(i18n.For(a.Store.Snapshot().Language).Settings.LocalToolFailed)
			})
			return
		}
		if !a.reviewLocalInstall(prepared) {
			return
		}
		// Confirming the review approves everything the manifest declares, the
		// same set the reference handed to `extensions_install`
		// (ExtensionsPanel.tsx:1554-1556).
		entry, err := prepared.Commit(true)
		if err != nil {
			log.Printf("floter: %s was not installed: %v", dir, err)
			a.onMain(func() {
				a.Launcher.WarnFeedback(i18n.For(a.Store.Snapshot().Language).Settings.LocalToolFailed)
			})
			return
		}
		a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstalled(entry.Name) })
		a.RefreshIntegrations(context.Background())
	}()
}

// reviewLocalInstall asks the user to confirm a local package before it is
// committed: the injected answer in tests, the native review dialog
// otherwise.
func (a *App) reviewLocalInstall(prepared extensions.Prepared) bool {
	if a.reviewInstall != nil {
		return a.reviewInstall(prepared)
	}
	language := a.Store.Snapshot().Language
	return localInstallReview(prepared, i18n.For(language).Settings, language)
}

// reviewRecommendedConnect asks the user to review a shipped package's
// permissions before it is connected: the injected answer in tests, the native
// review dialog otherwise. The reference only shows it when the manifest
// declares anything (`ExtensionsPanel.tsx:1349-1400`).
func (a *App) reviewRecommendedConnect(prepared extensions.Prepared) bool {
	if a.reviewRecommended != nil {
		return a.reviewRecommended(prepared)
	}
	language := a.Store.Snapshot().Language
	return recommendedInstallReview(prepared, i18n.For(language).Settings, language)
}

// connectDetected connects one of the discovered programs, with the words the
// discovery already knows about it.
func (a *App) connectDetected(tool settingsui.DetectedTool) {
	name := strings.TrimSpace(tool.Name)
	if name == "" {
		name = executableName(tool.Path)
	}
	versionArgs := tool.VersionArgs
	if versionArgs == "" {
		versionArgs = "--version"
	}
	a.connectLocalTool(settingsui.CustomTool{
		Name:        name,
		Program:     tool.Path,
		Description: tool.Description,
		VersionArgs: versionArgs,
		// The detected and register flows skip the connect form, which is
		// what ticks the platform list; without this the request is refused
		// with `platformRequired` before anything is written (the reference's
		// connect went through a backend that knew the platform).
		Platforms: []string{settingsui.CurrentPlatform()},
	})
}

// executableName is a program's own file name without its launcher suffix.
func executableName(path string) string {
	name := filepath.Base(path)
	for _, suffix := range []string{".exe", ".cmd", ".bat", ".com"} {
		if trimmed, ok := strings.CutSuffix(strings.ToLower(name), suffix); ok {
			return trimmed
		}
	}
	return name
}

// integrationFreshness reports what is known about one integration's command
// list, for the card's freshness row.
func (a *App) integrationFreshness(id string) (extensions.Freshness, bool) {
	integration, ok := a.Integrations.Inventory().WithID(id)
	if !ok {
		return extensions.Freshness{}, false
	}
	freshness := extensions.FreshnessOf(a.Paths, integration)
	if !freshness.Known() {
		return extensions.Freshness{}, false
	}
	return freshness, true
}

// toolForEdit is what a generated integration declares now, for the form to
// pre-fill: the same fields the connect flow collects, read back from the
// manifest, the descriptor and the repository.
func (a *App) toolForEdit(id string) (settingsui.CustomTool, bool) {
	integration, ok := a.Integrations.Inventory().WithID(id)
	if !ok {
		return settingsui.CustomTool{}, false
	}
	manifest := integration.Manifest
	descriptor, err := extensions.Describe(context.Background(), integration)
	if err != nil || len(descriptor.Commands) == 0 {
		return settingsui.CustomTool{}, false
	}
	root := descriptor.Commands[0]
	// R158 · the description is read back for an executable integration only:
	// a script manifest carries the host's own internal sentence, which must
	// not be shown to the user as if it were content they could clear (R159).
	description := ""
	if manifest.Runtime.Type != "script" {
		description = manifest.Description
	}
	tool := settingsui.CustomTool{
		Name:           integration.Name,
		Program:        integration.Entry.ExecutablePath,
		Command:        root.ID,
		Mode:           manifest.Runtime.Type,
		ScriptLanguage: manifest.Runtime.Language,
		ScriptContent:  scriptContent(integration),
		Args:           strings.Join(manifest.Provider.ArgsPrefix, " "),
		VersionArgs:    strings.Join(manifest.Runtime.VersionArgs, " "),
		Version:        integration.Version,
		Description:    description,
		Output:         manifest.Output,
		Permissions:    append([]string{}, manifest.Permissions...),
		Platforms:      append([]string{}, manifest.Platforms...),
		Params:         append([]extensions.ParamDefinition{}, manifest.Params...),
	}
	return tool, true
}

// editCustomTool rewrites a generated integration from the form: the package
// is regenerated in place, its command list re-derived, and the same install
// path records it — with the permission review a changed manifest requires.
func (a *App) editCustomTool(id string, tool settingsui.CustomTool) {
	go func() {
		request := customRequest(tool)
		// The approval is checked against the *new* manifest before anything
		// is written: a changed declaration is exactly what invalidates the
		// old one, and the user answers once.
		approved := true
		if approval := extensions.ApprovalForCustom(a.Paths, id, request); approval.NeedsApproval() {
			approved = a.confirmInstallPermissions(approval)
		}
		entry, err := extensions.UpdateCustom(context.Background(), a.Paths, id, request, approved)
		if err != nil {
			log.Printf("floter: could not edit %s: %v", id, err)
			return
		}
		a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstalled(entry.Name) })
		a.RefreshIntegrations(context.Background())
	}()
}

// chooseProgram asks the user to point at a program on this machine, through
// the framework's own file dialog unless a test answered for itself.
func (a *App) chooseProgram() (string, error) {
	if a.openFileDialog != nil {
		return a.openFileDialog()
	}
	return "", errors.New("floter: no file dialog on this build")
}

// connectLocalTool connects a program already on this machine as an
// integration: the package is written, the user is asked about what it
// declares, and the same local install every other tool goes through records
// it. The work happens off the main thread, since writing the package runs
// the tool's own help for its command list.
func (a *App) connectLocalTool(tool settingsui.CustomTool) {
	go func() {
		paths := a.Paths
		request := customRequest(tool)
		prepared, err := extensions.PrepareCustom(context.Background(), paths, request)
		if err != nil {
			log.Printf("floter: could not prepare the local tool %s: %v", tool.Name, err)
			a.onMain(func() {
				a.Launcher.WarnFeedback(i18n.For(a.Store.Snapshot().Language).Settings.LocalToolFailed)
			})
			return
		}
		approved := true
		if prepared.Approval.NeedsApproval() {
			approved = a.confirmInstallPermissions(prepared.Approval)
		}
		entry, err := prepared.Commit(approved)
		if err != nil {
			log.Printf("floter: could not connect %s: %v", tool.Name, err)
			a.onMain(func() {
				a.Launcher.WarnFeedback(i18n.For(a.Store.Snapshot().Language).Settings.LocalToolFailed)
			})
			return
		}
		a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstalled(entry.Name) })
		a.RefreshIntegrations(context.Background())
	}()
}

// reprobeCommands re-runs a generated custom integration's help derivation
// off the main thread: the command list is rebuilt from the tool's own
// --help, and the launcher's command list follows. A failure says so as a
// diagnosis, and the previous descriptor stays.
func (a *App) reprobeCommands(id string) {
	go func() {
		report, err := extensions.ReprobeCommands(context.Background(), a.Paths, id)
		if err != nil {
			log.Printf("floter: could not re-probe %s: %v", id, err)
			a.diagnosisMu.Lock()
			if a.diagnoses == nil {
				a.diagnoses = map[string]settingsui.Integration{}
			}
			a.diagnoses[id] = settingsui.Integration{Diagnosis: err.Error(), DiagnosisFailed: true}
			a.diagnosisMu.Unlock()
		} else {
			log.Printf("floter: re-probed %s (commands=%d, was %v)", id, report.CommandCount, report.PreviousCommandCount)
		}
		a.onMain(func() {
			a.Launcher.SetCommands(a.launcherCommands())
			a.RefreshIntegrations(context.Background())
			if a.Win != nil {
				a.Win.Invalidate()
			}
		})
	}()
}

// setCommandEnabled records a command's switch and re-hands the launcher its
// command list, so a switch takes effect without a provider round trip.
func (a *App) setCommandEnabled(extensionID, commandID string, enabled bool) {
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetCommandSwitch(extensionID, commandID, enabled)
	}); err != nil {
		log.Printf("floter: could not save the command switch: %v", err)
		return
	}
	a.onMain(func() { a.Launcher.SetCommands(a.launcherCommands()) })
}

// setCommandAlias records one command's alias and re-hands the launcher its
// command list. The alias map is keyed by command name (not extension), as
// the old settings card's was.
func (a *App) setCommandAlias(extensionID, commandID, alias string) {
	_ = extensionID
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetCommandAlias(commandID, alias)
	}); err != nil {
		log.Printf("floter: could not save the command alias: %v", err)
		return
	}
	a.onMain(func() {
		a.Launcher.SetCommands(a.launcherCommands())
		a.scanTools()
	})
}

// browserTargets lists the browsers the browser plugin can be pointed at, as
// the settings page's picker shows them: one entry per installed browser, in
// discovery order.
func (a *App) browserTargets() []i18n.Option {
	profiles := a.browserProfileList(browserState(a.Store.Snapshot()).customBaseDir)
	seen := map[string]bool{}
	var options []i18n.Option
	for _, profile := range profiles {
		if profile.BrowserID == "" || seen[profile.BrowserID] {
			continue
		}
		seen[profile.BrowserID] = true
		options = append(options, i18n.Option{ID: profile.BrowserID, Label: profile.Browser})
	}
	return options
}

// openURL opens a page in the browser a result came from, so a search of one
// browser does not open its pages in another. An unknown or empty browser id
// opens it in the system's default browser.
//
// An empty browser id is the generic opener, the old host's `open_url`, and
// that command refuses anything outside http/https/ftp (`commands/actions.rs`
// `is_web_url`). A browser row always carries the id it came from, so the
// allow-list cannot narrow what a browser plugin opens.
func (a *App) openURL(browserID, url string) error {
	if browserID == "" && !isWebURL(url) {
		return errNonWebURL
	}
	if a.openURLCommand != nil {
		return a.openURLCommand(browserID, url)
	}
	if browserID != "" {
		if err := browser.OpenURL(browserID, url); err == nil {
			return nil
		} else {
			log.Printf("floter: could not open %s in %s: %v", url, browserID, err)
			return err
		}
	}
	return mygo.Shell.OpenExternal(url)
}

// copyClipboardEntry puts a clipboard entry back on the clipboard: its text,
// its file list, or its image. The payload is built here (an image is read
// from the history) and written by the injected writer.
func (a *App) copyClipboardEntry(entry clipboard.Entry) {
	payload := ClipboardPayload{Kind: entry.Kind, Text: entry.Text}
	switch entry.Kind {
	case clipboard.KindFiles:
		payload.Paths = append([]string{}, entry.Paths...)
	case clipboard.KindImage:
		path := a.Clipboard.ImagePath(entry)
		if path == "" {
			return
		}
		png, err := os.ReadFile(path)
		if err != nil {
			log.Printf("floter: could not read the pinned image: %v", err)
			a.onMain(func() {
				a.Launcher.WarnFeedback(i18n.For(a.Store.Snapshot().Language).Launcher.ClipboardCopyFailed)
			})
			return
		}
		payload.Image = png
	}
	// The watcher must not record what the app itself wrote.
	if entry.Kind == clipboard.KindText {
		a.selfCopied = entry.Text
	} else {
		a.selfCopied = ""
	}
	if a.writeClipboard != nil {
		a.writeClipboard(payload)
	}
}

// systemClipboardWrite is the default clipboard writer: the system clipboard
// through the framework.
func systemClipboardWrite(payload ClipboardPayload) {
	switch payload.Kind {
	case clipboard.KindFiles:
		if len(payload.Paths) > 0 {
			if err := mygo.Clipboard.WriteFiles(payload.Paths...); err != nil {
				log.Printf("floter: could not copy the files back: %v", err)
			}
		}
	case clipboard.KindImage:
		if len(payload.Image) == 0 {
			return
		}
		data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, payload.Image)))
		if err := mygo.Clipboard.Write(data); err != nil {
			log.Printf("floter: could not copy the image back: %v", err)
		}
	default:
		mygo.Clipboard.WriteText(payload.Text)
	}
}

// completeCommand asks a provider for completions of what is being typed.
// It runs off the main thread and hands the answer back to it; a provider
// that does not implement the operation, or answers too slowly, leaves the
// static completions alone.
func (a *App) completeCommand(entry extensions.CommandEntry, tokens []string, done func([]extensions.Completion)) {
	go func() {
		integration, ok := a.Integrations.Inventory().WithID(entry.IntegrationID)
		if !ok || integration.ManifestErr != nil {
			return
		}
		cwd, err := os.Getwd()
		if err != nil {
			cwd = ""
		}
		items, err := extensions.Complete(context.Background(), integration, extensions.CompletionRequest{
			Command: entry.Command.ID,
			Tokens:  tokens,
			CWD:     cwd,
		})
		if err != nil {
			log.Printf("floter: %s offered no completions: %v", entry.Command.ID, err)
			return
		}
		a.onMain(func() { done(items) })
	}()
}

// confirmInstallPermissions asks the user about an install's permissions,
// through the injected answer or the native dialog.
func (a *App) confirmInstallPermissions(approval extensions.PermissionApproval) bool {
	if a.confirmPermissions != nil {
		return a.confirmPermissions(approval)
	}
	language := a.Store.Snapshot().Language
	copy := i18n.For(language)
	return permissionDialog(approval, copy.Settings, language)
}

// knownPermission is the schema's permission ids, for the settings list.
func knownPermission(permission string) bool {
	for _, candidate := range extensions.AllPermissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

// runCommand runs an extension's command in the terminal surface, as the old
// app handed a command off to its terminal. args are the argument words the
// user typed in the command mode.
func (a *App) runCommand(entry extensions.CommandEntry, args []string) {
	argv := append([]string{entry.Program}, entry.Args...)
	argv = append(argv, args...)
	if err := a.Terminal.RunCommand(argv, entry.Dir, entry.Env); err != nil {
		log.Printf("floter: could not run %s: %v", entry.Command.ID, err)
	}
	a.Open(SurfaceTerminal)
}

// runCommandCaptured runs a command whose manifest sends its output to the
// background: headless, with both streams captured, and the answer handed back
// on the main thread so the launcher can show it. The run is time-boxed, so a
// program that hangs is killed rather than pinning the app.
func (a *App) runCommandCaptured(entry extensions.CommandEntry, args []string, done func(extensions.CapturedRun, error)) {
	ctx, cancel := context.WithCancel(context.Background())
	forget := a.Runs.Add(entry.IntegrationID, cancel)
	go func() {
		defer forget()
		run, err := extensions.RunCaptured(ctx, entry, args)
		a.onMain(func() {
			done(run, err)
			a.notifyRun(entry, run, err)
		})
	}()
}

// loadOutputPage runs a self-paging command again with the cursor from its
// last emission, for the next page of an output list. Unlike the first run,
// a page load does not raise a completion notification: it is a continuation
// of the same run, not a new piece of news.
func (a *App) loadOutputPage(entry extensions.CommandEntry, args []string, cursor string, done func(extensions.CapturedRun, error)) {
	ctx, cancel := context.WithCancel(context.Background())
	forget := a.Runs.Add(entry.IntegrationID, cancel)
	go func() {
		defer forget()
		run, err := extensions.RunCapturedPage(ctx, entry, args, cursor)
		a.onMain(func() { done(run, err) })
	}()
}

// notifyRun raises a system notification for a background run that finished
// while the panel was hidden. A visible panel shows the output itself, so a
// notification there would be a second copy of the same news.
func (a *App) notifyRun(entry extensions.CommandEntry, run extensions.CapturedRun, err error) {
	if a.panelOnScreen() {
		return
	}
	lang := a.Store.Snapshot().Language
	notif := i18n.NotificationsFor(lang)
	name := entry.Command.Name
	if name == "" {
		name = entry.Command.ID
	}
	a.Notify(notif.Title, a.runNotificationBody(notif, i18n.For(lang).Settings, name, run, err))
}

// runNotificationBody phrases a finished background run the way the old build's
// completion toast did: a completed run names its duration and, when it said
// anything, its line count. A completed run that failed names its exit code. A
// run that never started or was killed keeps the keyed failure's own words
// (already localised by RunStatus); an exit code is not the fact the user needs
// there.
func (a *App) runNotificationBody(notif i18n.Notifications, settings i18n.Settings, name string, run extensions.CapturedRun, err error) string {
	switch {
	case run.Success:
		body := settings.RunSucceeded(name, extensions.FormatRunDuration(run.Duration.Seconds()*1000))
		if extensions.RunHasOutput(run) {
			lines := extensions.RunOutputLineCount(run)
			summary := settings.RunOutputLines(lines)
			if run.Truncated {
				summary = settings.RunOutputLinesTruncated(lines)
			}
			body += " \u00b7 " + summary
		}
		return body
	case err == nil:
		return settings.RunFailed(name, run.ExitCode)
	default:
		return notif.CommandFinished(name) + "  \u00b7  " + notif.CommandStatus(a.Launcher.RunStatus(run, err))
	}
}

// panelForeground reports whether the panel is on screen, from the two answers
// the framework gives about the window. The old build's `panel_foreground_from`
// combined three signals and let *any* one saying "on screen" win; mygo exposes
// two of them, and the rule is the same:
//
//   - `Window::is_visible()` — the platform's answer;
//   - `Window::is_focused()` — a focused window is on screen by definition,
//     even on the platforms where the panel's visibility is reported late
//     (macOS `NSPanel` before its first map).
//
// A missing window counts as hidden: there is no surface the user could be
// looking at, which is precisely when a notification is the right channel. The
// asymmetry is deliberate — a missed notification is an annoyance the user can
// still discover by opening the panel, while a notification raised over a
// visible panel is the double report this rule exists to prevent.
func panelForeground(visible, focused bool) bool {
	return visible || focused
}

// panelOnScreen reports whether the panel is on screen, so a completion lands
// where the user is looking. Production reads the window's two signals through
// panelForeground; a test can answer for itself.
func (a *App) panelOnScreen() bool {
	if a.onScreen != nil {
		return a.onScreen()
	}
	return a.Win != nil && panelForeground(a.Win.IsVisible(), a.Win.IsFocused())
}

// integrationRemovalNotice is the notification sentence for one removal,
// picked by the same kind the settings page's removal bar spoke in. The
// reference had four words (`src/ExtensionsPanel.tsx:1306-1308,1336-1339`): a
// generated custom integration is deleted, an npm package is uninstalled, a
// system-owned built-in is disconnected, and a linked package is removed from
// the list. A single generic "was removed" told the user less than the bar
// they had just confirmed.
func integrationRemovalNotice(copy i18n.Notifications, kind, name string) string {
	switch kind {
	case settingsui.RemovalCustom:
		return copy.IntegrationCustomDeleted(name)
	case settingsui.RemovalNPM:
		return copy.IntegrationUninstalled(name)
	case settingsui.RemovalSystem:
		return copy.IntegrationDisconnected(name)
	default:
		return copy.IntegrationPackageRemoved(name)
	}
}

// notifyIntegrationRemoval raises the kind's own removal completion through the
// app's one completion path.
func (a *App) notifyIntegrationRemoval(kind, name string) {
	a.notifyCompletion(func(c i18n.Notifications) string {
		return integrationRemovalNotice(c, kind, name)
	})
}

// notifyCompletion raises a system notification for a background integration
// task that finished while the panel was hidden. A visible panel shows the
// result itself, so a notification there would be a second copy of the same
// news.
func (a *App) notifyCompletion(render func(i18n.Notifications) string) {
	copy := i18n.NotificationsFor(a.Store.Snapshot().Language)
	message := render(copy)
	// The window that is on screen is told in its own panel: a system
	// notification for something the user is already looking at is noise, and
	// a completion that says nothing at all is worse.
	if a.panelOnScreen() {
		a.onMain(func() { a.Launcher.Feedback(message) })
		return
	}
	a.Notify(copy.Title, message)
}

// notifyClipboardClear reports the clipboard history's one long action the way
// the old build's `clipboard_clear_history` did: through the same completion
// rule as every other background task — the panel's own row when the user is
// looking at it, a system notification when the panel is hidden. The subject
// is the plugin page, so the name is that page's own localized name.
func (a *App) notifyClipboardClear(success bool) {
	name := i18n.For(a.Store.Snapshot().Language).Settings.ClipboardPlugin
	a.notifyCompletion(func(c i18n.Notifications) string {
		if success {
			return c.ClipboardCleared(name)
		}
		return c.ClipboardClearFailed(name)
	})
}

// Notify shows a system notification, through the framework unless a test
// provided its own.
func (a *App) Notify(title, body string) {
	if a.notify != nil {
		a.notify(title, body)
		return
	}
	if a.Win == nil {
		// No window means no running app to notify from: a test, or a
		// headless run. The framework's notification needs the app.
		return
	}
	notification := mygo.NewNotification(mygo.NotificationOptions{
		Title: title,
		Body:  body,
		Group: "floter.commands",
	})
	if err := notification.Show(); err != nil {
		log.Printf("floter: could not show a notification: %v", err)
	}
}

// scanApps hands the launcher the installed applications: the previous scan
// immediately when the sources have not changed, otherwise a fresh scan in the
// background. Either way the search grows or stays without the window waiting
// on a filesystem walk.
func (a *App) scanApps() {
	roots := apps.Roots()
	if found, upToDate := apps.CachedScan(roots); upToDate {
		a.onMain(func() { a.Launcher.SetApps(found) })
		return
	}
	go func() {
		found := apps.Scan(roots)
		apps.StoreScan(roots, found)
		a.onMain(func() { a.Launcher.SetApps(found) })
	}()
}

// Open switches to a surface, sizing and focusing it.
func (a *App) Open(s Surface) {
	// Leaving the settings surface writes any debounced slider/alias change
	// before the page goes away; the value was already live in the frame.
	if a.Surf == SurfaceSettings && s != SurfaceSettings && a.Settings != nil {
		a.Settings.Flush()
	}
	if s != SurfaceLauncher {
		// Entering a surface starts the residency clock (see summon).
		a.enteredAt = a.now()
		// The launcher's half-finished gestures do not outlive its surface: an
		// armed delete waits for its confirming press only while the list it
		// belongs to is the one on screen.
		a.Launcher.DisarmDelete()
	}
	a.Surf = s
	switch s {
	case SurfaceSettings:
		a.restoreSettingsPage()
		a.Settings.FocusSidebar()
	case SurfaceTerminal:
		a.Terminal.EnsureSession()
		// A session that outlived a visit to another surface takes the
		// appearance the user set while it was away. The reference's terminal
		// view re-ran its font and theme effects whenever the surface was
		// mounted again; this build's session is kept alive across surfaces, so
		// the same rule is spelled on the way back in. The store listener only
		// refreshes while the terminal is the active surface, so a change made
		// on the settings page would otherwise never reach the live session.
		a.refreshTerminal()
		a.Terminal.FocusTerminal()
	default:
		a.Launcher.FocusSearch()
	}
	a.resize()
	a.Show()
}

// The terminal's window is the one surface the user can resize, so a drag is
// what the terminal's stored size follows: the next launch opens the size the
// user left it at.
const terminalSizeSettle = 600 * time.Millisecond

// trackTerminalSize notes a window resize. Only the terminal surface is
// resizable, so a resize there is the user's own.
func (a *App) trackTerminalSize() {
	if a.Win == nil || a.Surf != SurfaceTerminal {
		return
	}
	width, height := a.Win.Size()
	if width <= 0 || height <= 0 {
		return
	}
	a.scheduleTerminalSize(float64(width), float64(height))
}

// scheduleTerminalSize writes a settled resize back to the settings.
func (a *App) scheduleTerminalSize(width, height float64) {
	settle := a.terminalSizeDelay
	if settle <= 0 {
		settle = terminalSizeSettle
	}
	a.sizeMu.Lock()
	a.pendingWidth, a.pendingHeight = width, height
	a.sizeGeneration++
	generation := a.sizeGeneration
	if a.sizeTimer != nil {
		a.sizeTimer.Stop()
	}
	// A drag fires a resize per frame; the write happens once it settles.
	a.sizeTimer = time.AfterFunc(settle, func() {
		a.sizeMu.Lock()
		if generation != a.sizeGeneration {
			a.sizeMu.Unlock()
			return
		}
		storedW, storedH := a.pendingWidth, a.pendingHeight
		a.sizeMu.Unlock()
		if err := a.Store.Update(func(s *settings.Settings) {
			s.TerminalWidth, s.TerminalHeight = storedW, storedH
		}); err != nil {
			log.Printf("floter: could not store the terminal size: %v", err)
		}
	})
	a.sizeMu.Unlock()
}

// handleFileDrop turns a drop into the files mode: the paths are normalized
// (a `~`, a relative path, a file that is gone), and the launcher lists what
// can be done with them. Nothing runs on arrival.
func (a *App) handleFileDrop(event *mygo.FileDropEvent) {
	if event == nil || len(event.Paths) == 0 {
		return
	}
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	files := drops.Normalize(event.Paths, home, cwd)
	if len(files) == 0 {
		return
	}
	a.Launcher.SetDropped(files)
	a.Open(SurfaceLauncher)
	a.Launcher.EnterFiles()
}

// restoreSettingsPage reopens the settings surface on the page the user last
// looked at (the `last_settings_page` key), falling back to General for a
// missing or unknown value.
func (a *App) restoreSettingsPage() {
	// An unknown or missing id lands on General, which is what the old
	// build's settings normalizer did with the key.
	page, _ := settingsui.PageByName(settings.LastSettingsPage(a.Store.Snapshot()))
	a.Settings.Page = page
}

// rememberSettingsPage records the page the user switched to, so the next
// visit reopens it.
func (a *App) rememberSettingsPage(name string) {
	if name == "" {
		return
	}
	_ = a.Store.Update(func(s *settings.Settings) { s.SetLastSettingsPage(name) })
}

// Toggle shows the launcher, or hides the window when it is already the
// frontmost thing the user is looking at.
func (a *App) Toggle() {
	if a.Win == nil {
		return
	}
	if a.Win.IsVisible() && a.Win.IsFocused() {
		a.Hide()
		return
	}
	a.summon()
}

// summon reveals the window from a dismissal. The surface the user was on — a
// plugin mode or its configuration overlay, settings, or the terminal — is
// kept only while the residency window still holds; otherwise this is a fresh
// launcher, and the half-finished gestures of the previous session (the query
// and any dropped rows) go with the reveal that starts the next one. This is
// the old build's reveal handler in src/App.tsx (R35/R41), the same policy the
// pure surface table in src/surface-residency.ts describes.
func (a *App) summon() {
	// R68 · one catalog refresh per reveal. A tool installed in the user's own
	// shell while the window was hidden must stop being offered as an install
	// row on the next summon; between reveals the catalog memo serves every
	// query, and a scan already in flight is shared rather than duplicated.
	a.scanToolCatalog()
	if a.residencyHolds() {
		// A summon back into the surface is the user confirming they are still
		// in it: the clock restarts here, exactly as `residency.refresh()` did.
		a.enteredAt = a.now()
		a.Show()
		return
	}
	a.Launcher.ResetQuery()
	a.Launcher.ClearDropped()
	a.Open(SurfaceLauncher)
	a.Show()
}

// vibrancyFor is the window material for a platform: the translucent blur the
// panel's glass is read through. It is a pure function of GOOS so the one
// decision is testable without a window.
//
// mygo maps `under-window` to the Mica system backdrop on Windows 11
// (internal/windows/window.go's backdropFor: every material it does not name
// falls through to backdropMica). Mica is an opaque wallpaper tint, not a
// blur, so a panel that asked for it got an opaque底 and the main-opacity
// slider moved nothing. Acrylic is the system backdrop that actually blurs
// what is behind the window, which is what the panel's tint is meant to sit
// on. macOS keeps the under-window NSVisualEffectView material, and Linux has
// no material at all - there the tint's own alpha is the whole of the glass.
func vibrancyFor(goos string) mygo.Vibrancy {
	switch goos {
	case "windows":
		return mygo.VibrancyAcrylic
	case "linux":
		return mygo.VibrancyNone
	default:
		return mygo.VibrancyUnderWindow
	}
}

// vibrancyForStop is the window material for a platform and a glass stop. The
// stop is the effect ladder, and the old build's `--glass-step-blur`
// (10/22/28px) is the rung the framework can carry: the panel cannot run its
// own `backdrop-filter`, so the backdrop's blur is the OS material, and a
// stop that only moved the tint left `frosted` and `regular` reading the same
// (「玻璃效果…看起来好像没什么区别」).
//
// macOS has a behind-window material per weight: `under-page` is the thinnest
// (the frosted end), `under-window` the standard pane, and `sidebar` the
// denser, more opaque end the liquid stop asks for. Windows 11 has only one
// backdrop that blurs - Acrylic - so all stops keep it and the tint and lens
// carry the ladder there. Linux has none.
func vibrancyForStop(goos, step string) mygo.Vibrancy {
	if goos != "darwin" {
		return vibrancyFor(goos)
	}
	switch settings.NormalizeGlassStep(step) {
	case "off":
		return mygo.VibrancyNone
	case "frosted":
		return mygo.VibrancyUnderPage
	case "liquid":
		return mygo.VibrancySidebar
	default:
		return mygo.VibrancyUnderWindow
	}
}

// placeWindow centres the panel in a display's work area, in that display's own
// coordinates: the old build placed it on the monitor the user was looking at
// and clamped against the *terminal* height so the expanded window still fits
// after the panel grows into it.
//
// The display is the one the window is already on, falling back to the primary
// one: the framework does not report which monitor has the keyboard's focus, and
// the window's own position is the honest stand-in — a panel the user carried to
// a second monitor is re-homed on that monitor, not yanked back to the first.
//
// mygo's Display.Bounds/WorkArea are already in DIPs (internal/platform: "Rect
// is a rectangle in screen coordinates (DIPs)"), and Window.SetPosition takes
// DIPs too, so the work area is handed to placement as it is: the old build's
// placement divided by the monitor's scale because Tauri's work_area was
// physical, and doing that here (twice, once in each function) put a panel on a
// scaled display into the corner instead of the middle.
func (a *App) placeWindow() {
	if a.Win == nil {
		return
	}
	display := a.windowDisplay()
	area := display.WorkArea
	areaX := float64(area.X)
	areaY := float64(area.Y)
	areaWidth := float64(area.Width)
	areaHeight := float64(area.Height)
	if areaWidth <= 0 || areaHeight <= 0 {
		return
	}
	width := float64(launcher.InputWindowWidth)
	// The reference height is the terminal's, not the launcher's: the window
	// grows into the terminal when a session opens, and it must still fit.
	reference := a.Store.Snapshot().TerminalHeight
	x, y := placement(areaX, areaY, areaWidth, areaHeight, width, reference)
	a.Win.SetPosition(x, y)
}

// windowDisplay is the display the window is on: the one whose bounds hold its
// centre, and the primary one when the window is nowhere (or the framework
// cannot say).
func (a *App) windowDisplay() mygo.Display {
	displays := mygo.Screen.Displays()
	if len(displays) == 0 {
		return mygo.Screen.PrimaryDisplay()
	}
	x, y := a.Win.Position()
	w, h := a.Win.Size()
	if display, ok := displayContaining(displays, x, y, w, h); ok {
		return display
	}
	return mygo.Screen.PrimaryDisplay()
}

// displayContaining is the display a window's centre falls in, reporting
// whether one did: the window's own coordinates and the displays' bounds are
// the same space, so this is arithmetic rather than a query.
func displayContaining(displays []mygo.Display, x, y, w, h int) (mygo.Display, bool) {
	centreX, centreY := x+w/2, y+h/2
	for _, display := range displays {
		bounds := display.Bounds
		if centreX >= bounds.X && centreX < bounds.X+bounds.Width &&
			centreY >= bounds.Y && centreY < bounds.Y+bounds.Height {
			return display, true
		}
	}
	return mygo.Display{}, false
}

// placement is where a panel of windowW by referenceH goes on a work area, in
// the area's own coordinates: centred, and clamped so it never runs off the
// display. The reference height is the tallest the window will grow, so the
// placement is stable across a surface change.
//
// The area and the window's size are both in DIPs, so there is no scale to
// apply here: mygo's display geometry is logical (see placeWindow), and the
// old build's division by the monitor's scale was for Tauri's physical
// work_area. The regression it left is pinned by
// TestPlacementIgnoresTheDisplayScale.
func placement(areaX, areaY, areaWidth, areaHeight, windowW, referenceH float64) (int, int) {
	if areaWidth <= 0 || areaHeight <= 0 {
		return 0, 0
	}
	maxX := areaX + math.Max(areaWidth-windowW, 0)
	maxY := areaY + math.Max(areaHeight-referenceH, 0)
	x := areaX + (areaWidth-windowW)/2
	y := areaY + (areaHeight-referenceH)/2
	return int(math.Round(math.Min(math.Max(x, areaX), maxX))),
		int(math.Round(math.Min(math.Max(y, areaY), maxY)))
}

// Show reveals the window and takes the focus, arming the blur grace.
//
// A summon of a window that is not on screen re-homes the panel, as the old
// build's own reveal did: the launcher is a summon, so it belongs where the
// user's attention is rather than wherever the window was dragged to last. A
// switch of the surface on a window the user can already see does not: the old
// build's `show_input` only called `move_to_default_position` when
// `!preserve_anchor` (i.e. `state.window_visible` was false), so returning from
// the settings to the search box kept the position the user had dragged the
// panel to. Re-homing here on every `Open` is what yanked that window back to
// the centre.
//
// The placement happens before the reveal and again after it — macOS drops
// geometry set on a window that is not yet mapped — and both land in the same
// frame, so the panel is never seen in between.
//
// A reveal is also the one moment the install catalog is re-read (R68): a tool
// installed in the user's own shell while the window was hidden must stop being
// offered as an install row on the next reveal. Show is the one door every
// reveal goes through — a summon, a deep link, the tray, a second launch — and
// the decision reads the same visibility the framework's own OnShow does, so a
// scan already in flight is shared rather than duplicated (see scanToolCatalog).
func (a *App) Show() {
	rehome := placementForReveal(a.windowWasVisible())
	if rehome {
		a.scanToolCatalog()
	}
	if a.Win == nil {
		return
	}
	a.shownAt = a.now()
	if rehome {
		a.placeWindow()
	}
	a.Win.Show()
	if rehome {
		a.placeWindow()
	}
	a.Win.Focus()
	// R41 · Hyprland tiles a new toplevel, so the size the placement above set
	// is overridden by the compositor. Float and centre the window we just
	// focused, then place it again so the panel's own size lands on a floating
	// window. No-op on every other compositor and platform.
	if a.ensureHyprlandFloating() && rehome {
		a.placeWindow()
	}
	a.Win.Invalidate()
}

// placementForReveal decides whether revealing the window re-homes it. A
// window that was not on screen is placed (the old build's reveal re-homed
// onto the focused monitor); one the user can already see keeps where it is.
// This is the old build's `preserve_anchor`: a mode switch is not a summon,
// so it must not centre the panel.
func placementForReveal(wasVisible bool) bool {
	return !wasVisible
}

// windowWasVisible reports whether the panel was on screen before this Show,
// which is what tells a reveal (a hidden window coming forward) from a surface
// switch. The reading is the framework's own: Window.Show fires OnShow only on
// a hidden-to-visible transition, `if !visible`, and this is the same check.
// A test has no window to read, so it drives the decision through the
// windowVisible seam; with no window and no seam there is nothing to reveal.
func (a *App) windowWasVisible() bool {
	if a.windowVisible != nil {
		return a.windowVisible()
	}
	return a.Win == nil || a.Win.IsVisible()
}

// currentResidencySurface is the surface the residency rule tracks, derived
// from the same state the old build's `residencySurface` read: settings and
// the terminal are full surfaces, the plugin configuration overlay wins over
// the plugin mode beneath it, and everything else on the launcher (the
// ordinary search page) is "" — the one place a summon is allowed to reset to.
func (a *App) currentResidencySurface() string {
	switch a.Surf {
	case SurfaceSettings:
		return "settings"
	case SurfaceTerminal:
		return "terminal"
	case SurfaceLauncher:
		switch {
		case a.Launcher.ConfigOpen():
			return "plugin-config"
		case a.Launcher.PluginModeActive():
			return "plugin-mode"
		}
	}
	return ""
}

// noteResidency keeps the residency clock in step with the surface the app is
// on: entering a tracked surface starts it, and leaving it clears the note so a
// leftover clock can never claim the next summon. This is the old build's
// `useSurfaceResidency`, which noted the same transition on every render; the
// View calls it once a frame.
func (a *App) noteResidency() {
	surface := a.currentResidencySurface()
	if surface == a.residencyOn {
		return
	}
	a.residencyOn = surface
	if surface != "" {
		a.enteredAt = a.now()
	}
}

// residencyHolds reports whether the current surface survives this summon:
// the shipped ten seconds since it was opened, a custom window, "never", or
// nothing at all (0). A surface the app is not on (the ordinary search page)
// never holds. See src/surface-residency.ts.
func (a *App) residencyHolds() bool {
	if a.currentResidencySurface() == "" {
		return false
	}
	seconds := a.Store.Snapshot().SurfaceResidencySeconds
	switch {
	case seconds == 0:
		return false
	case seconds >= settings.SurfaceResidencyNever:
		return true
	default:
		return a.now().Sub(a.enteredAt) <= time.Duration(seconds)*time.Second
	}
}

// resizeLauncher records the height the launcher's list asks for. The
// walk to it happens in the View (see walkLauncherHeight), where the
// animation has a frame to live in; a height the window already carries
// never starts one.
func (a *App) resizeLauncher(height int) {
	if a.Surf != SurfaceLauncher {
		return
	}
	a.launcherTarget = height
}

// walkLauncherHeight moves the window to the launcher's target height on the
// reference's own walk (R68): an ease-out curve — the framework's EaseOut, the
// same `1-(1-t)^3` shape — over its RESIZE_TWEEN_MS of 150ms. A spring keeps
// its velocity when the target changes mid-flight, but it also overshoots and
// takes 500ms to settle, which reads as the edge lagging a keystroke; the
// reference deliberately chose a short curve. The element's AnimateTo keeps
// the frames coming until the value lands, and the last step's SetSize is the
// only one the platform sees at the end. A window that is already there does
// nothing - the common case, a keystroke that left the list the same size.
func (a *App) walkLauncherHeight(c *ui.Context) {
	if a.Win == nil || a.Surf != SurfaceLauncher || a.launcherTarget == 0 {
		return
	}
	current := float32(a.lastH)
	target := float32(a.launcherTarget)
	// The animation's carrier: a zero-size element that lays out nothing,
	// whose Animate state lives across the frames the walk needs.
	animated := ui.Box(c).Size(0, 0).Opacity(0)
	height := animated.AnimateTo("launcher.height", target, launcherHeightAnimation)
	if int(math.Round(float64(height))) == int(current) {
		return
	}
	a.Win.SetSize(launcher.InputWindowWidth, int(math.Round(float64(height))))
	a.lastW, a.lastH = launcher.InputWindowWidth, int(math.Round(float64(height)))
}

// launcherHeightAnimation is the walk's motion: the reference's 150ms
// ease-out (useLauncherHeight's RESIZE_TWEEN_MS and `1-(1-t)^3` step), not a
// spring — the curve lands without overshoot at the pace a keystroke asks for.
var launcherHeightAnimation = ui.Animation{Duration: 150 * time.Millisecond}

// Hide hides the window, leaving the app running with its shortcut.
func (a *App) Hide() {
	// A hidden window stops drawing, so the debounce's frame-driven deadline
	// would never arrive: write the pending value before it goes.
	if a.Settings != nil {
		a.Settings.Flush()
	}
	if a.hideWindow != nil {
		a.hideWindow()
		return
	}
	if a.Win != nil {
		a.Win.Hide()
	}
}

// appShortcuts runs the old build's dismiss table, surface by surface. The
// literal ⌘W is not rebindable and its meaning is not the same everywhere:
// on the settings it closes the panel, on the terminal it is the shell's own
// delete-word and is left alone, and on the launcher it is nothing at all —
// only the configurable new-command binding dismisses there. That binding
// ships as ⌘W, so the two look alike until the user rebinds it; folding the
// literal chord into the rule would keep hiding the launcher after a rebind,
// which the reference's `collapsed["mod-w"]: null` explicitly forbids.
// Escape is each surface view's own — the terminal shell's on the terminal, a
// question's first on the settings — and the terminal shell keeps it.
func (a *App) appShortcuts(c *ui.Context) {
	// One Shortcut() call per key per frame consumes the delivered press, so
	// the triggers are resolved per surface: the terminal asks only the
	// new-command binding (the literal ⌘W is the shell's delete-word), the
	// settings ask the two close chords, and the launcher asks only the
	// new-command binding. Escape is deliberately absent from both the launcher
	// and the settings here: this handler runs before the surface view is
	// built, and the reference let the surface's own listener answer first. The
	// launcher's view turns an empty field's Escape into a hide and a filled
	// one into a clear; the settings view turns it into "answer the open
	// confirmation" or, failing that, close the panel. Claiming it here would
	// outrun those listeners — pressing Escape on a removal bar would close the
	// whole panel instead of cancelling the bar.
	switch a.Surf {
	case SurfaceTerminal:
		if mods, key, ok := shortcuts.Parse(settings.Shortcut(a.Store.Snapshot(), settings.ShortcutNewCommand)); ok && c.Shortcut(mods, key) {
			a.returnToInput()
		}
	case SurfaceSettings:
		if c.Shortcut(ui.Cmd, ui.KeyW) {
			a.Open(SurfaceLauncher)
			return
		}
		if mods, key, ok := shortcuts.Parse(settings.Shortcut(a.Store.Snapshot(), settings.ShortcutNewCommand)); ok && c.Shortcut(mods, key) {
			a.Open(SurfaceLauncher)
		}
	default:
		if mods, key, ok := shortcuts.Parse(settings.Shortcut(a.Store.Snapshot(), settings.ShortcutNewCommand)); ok && c.Shortcut(mods, key) {
			a.Hide()
		}
	}
}

// returnToInput is the way back from another surface: the launcher, with
// an empty field, the way the old build's returnToInputMode did.
func (a *App) returnToInput() {
	// R74 · the install row's own door into the terminal page never hides the
	// window, so the summon-time reveal refresh cannot see this return: a tool
	// installed while the user watched that session would still be offered as
	// an install row on the way back. The explicit way back asks for one
	// refresh of its own.
	a.scanToolCatalog()
	a.Launcher.ResetQuery()
	a.Open(SurfaceLauncher)
}

// View builds the window: the transparent root, the inset card, its glass
// material (or plain face for the `off` stop), and the active surface.
func (a *App) View(c *ui.Context) {
	s := a.Store.Snapshot()
	surface := launcher.Resolve(s, c.Theme().Dark, c.Preferences())
	c.SetTheme(surface.Tokens.Theme)

	// A notice raised while the user was elsewhere (a refused floter:// link)
	// is shown on the surface they are on, not on the launcher's docked row
	// they cannot see.
	a.flushNotices(c)

	// The notices paint with the app's own viewport, not the framework's: the
	// old build's `.app-toast` carried the kind's mark and tone, and the
	// theme's default paints every toast the same inverted pill (see
	// toastViewport). It must run after flushNotices, which is what puts this
	// frame's notice into the stack the viewport reads.
	a.toastViewport(c)

	// The app's own keys answer on every surface: ⌘W hides whatever the
	// window is showing (the old build's dismiss table was surface-wide),
	// and a new command returns to the launcher from the terminal or the
	// settings as well as from the launcher's own modes.
	a.appShortcuts(c)

	// The residency clock follows the surface the app is on, so a summon
	// knows whether the plugin mode (or settings, or terminal) the user left
	// is still "where they left it".
	a.noteResidency()

	// The launcher's window walks to its target height here, where the
	// animation has a frame to live in.
	a.walkLauncherHeight(c)

	// Only the card paints; the window's transparent margin shows the
	// desktop, which the glass reads through.
	c.Root().Background(ui.Transparent)

	t := c.Theme()
	top, right, bottom, left := a.inset()
	// The card's one edge (base.css's --input-stroke): a plain 1px stroke,
	// lit one step while the window has the keyboard — the rim the old
	// build's R72 kept when it deleted the second one.
	stroke := surface.Tokens.InputStroke
	if a.Win != nil && a.Win.IsFocused() {
		stroke = surface.Tokens.InputStrokeActive
	}
	ui.Column(c).Fill().Padding(top, right, bottom, left).Children(func() {
		radius := surface.Tokens.RadiusLG
		// The card clips, so a full-bleed block inside it takes the window's
		// rounded corners — the old build's `.collapsed-card` had
		// `overflow: hidden` for exactly this.
		card := ui.Column(c).Fill().Radius(radius).DragWindow().Border(1, stroke).Clip()
		// The card's own depth where the platform has no window-server shadow:
		// Linux compositors disagree on whether an undecorated transparent
		// window gets a native one, and a frameless Windows window's shadow
		// follows the window's rectangle rather than the card's corners. macOS
		// draws none — the window server casts it (see the window options).
		switch runtime.GOOS {
		case "linux":
			card.Shadow(0, t.Space(1), t.Space(4.5), 0, surface.Tokens.ShadowAmbient)
			card.Shadow(0, t.Space(0.25), t.Space(1.25), 0, surface.Tokens.ShadowContact)
		case "windows":
			card.Shadow(0, t.Space(1), t.Space(4), 0, surface.Tokens.ShadowAmbient)
			card.Shadow(0, t.Space(0.25), t.Space(1), 0, surface.Tokens.ShadowContact)
		}
		// The launcher's card is its own floating window and carries no rim (the
		// old build's R72 removed it); the settings and terminal panels sit *on*
		// a surface, so they take `--glass-rim-shadow`: the lit top edge, the
		// dark bottom edge and the whisper of a stroke on both sides.
		if a.Surf != SurfaceLauncher {
			card.InsetShadow(0, 1, 0, 0, surface.Tokens.EdgeHighlight)
			card.InsetShadow(0, -1, 0, 0, surface.Tokens.EdgeShadow)
			card.InsetShadow(1, 0, 0, 0, surface.Tokens.EdgeHighlightX)
			card.InsetShadow(-1, 0, 0, 0, surface.Tokens.EdgeHighlightX)
		}
		// The launcher's blocks run edge to edge: the field row is full-bleed
		// (`.collapsed-card__input-row`) and the panel below it is the card's
		// own recess, so the surface owns no horizontal inset of its own —
		// every block carries the 4u/16u its reference rule names. The other
		// two surfaces are panels and keep the card's 2.5u gutter.
		padX := t.Space(2.5)
		if a.Surf == SurfaceLauncher {
			padX = 0
		}
		// The terminal has its own transparency control: the old build painted
		// the terminal's canvas at `--terminal-opacity` over a veil of
		// `--glass-tint-terminal`, so the panel's fill follows the terminal
		// slider there and the main one everywhere else.
		spec := surface.Glass
		if a.Surf == SurfaceTerminal {
			spec = glassmap.SpecForPreferences(s.GlassStep, s.TerminalOpacity, c.Preferences().HighContrast)
		}
		faceTop, faceBottom := surface.Tokens.FaceFill(spec.TintAlpha)
		card.Children(func() {
			// The panel's fill is one translucent tint at the frame's own alpha —
			// the reference's `--glass-tint-alpha` — and the stop's sheen is the
			// lit layer over it, exactly as base.css's `linear-gradient(180deg,
			// sheen-top, sheen-bottom), var(--launcher-tint)` stacked the two.
			// The tint's alpha is chosen so that stack composites back to the
			// frame's own alpha (see theme.Tokens.FaceFill): the sheen used to
			// sit over the *frame's* tint, adding its own alpha on top, and that
			// layer is what the user read as 「开玻璃透明底失效」 — a glass-on
			// frame was denser than a glass-off one (the light appearance added
			// up to 0.11 in the result band at the default 47%). Now the frame's
			// alpha is the transparency slider's on every stop, while the stop
			// still grades the lit face. The window's vibrancy blurs the
			// backdrop where the platform has one; where it does not, the tint's
			// alpha is the whole of the glass.
			//
			// The `off` stop paints the same tint with no glass effect: the
			// frame's own transparency is the opacity slider's on every stop, so
			// turning the glass off never turns the window opaque (「关闭玻璃效果
			// 的时候，应用的透明底是失效的」). Its sheen and lens are zero, so the
			// face is a plain translucent pane.
			//
			// The launcher's own insets are the old panel's: 4u above and 2u
			// below (`.launcher-bottom`), with the field's row and its breath
			// inside the card — `66 + rows` is the window. The other surfaces
			// are panels, not a panel that grows a row at a time, but they
			// share the top inset: the old build pinned the launcher's field
			// row and the settings pages' header to the same band (R37), and a
			// band that starts elsewhere is a band that jumps when the surface
			// changes.
			padTop, padBottom := t.Space(launcher.TopInsetUnits), t.Space(2.5)
			if a.Surf == SurfaceLauncher {
				padBottom = t.Space(launcher.TailUnits)
			}
			face := ui.Column(c).Fill().Radius(radius).
				LinearGradient(ui.LinearGradient{From: faceTop, To: faceBottom, Angle: 180}).DragWindow()
			face.Children(func() {
				// The panel's own sheen (`linear-gradient(180deg, sheen-top,
				// sheen-bottom)`): the lit face that keeps a translucent pane
				// from reading as a flat wash. The off stop's sheen tokens are
				// zero, so the same gradient draws nothing there.
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius).PassThrough().
					LinearGradient(ui.LinearGradient{From: surface.Tokens.SheenTop, To: surface.Tokens.SheenBottom, Angle: 180})
				ui.Column(c).Fill().Padding(padTop, padX, padBottom, padX).Children(func() { a.surface(c) })
			})
		})
	})
}

// surface builds the body of the window: the active surface's content.
func (a *App) surface(c *ui.Context) {
	switch a.Surf {
	case SurfaceSettings:
		a.Settings.View(c)
	case SurfaceTerminal:
		a.Terminal.View(c)
	default:
		a.Launcher.View(c)
	}
}

// appVersion is the version shown on the About page: the packaged app's,
// and, under `go run` and `go test` (where the bundle has none), the one in
// the project file the CLI builds from, read beside the executable or in
// the working directory.
func appVersion() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	candidates := []string{"mygo.json"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "mygo.json"))
	}
	for _, path := range candidates {
		if v := versionInFile(path); v != "" {
			return v
		}
	}
	return ""
}

// versionInFile reads the `version` of a project file, empty when it is
// missing or unreadable.
func versionInFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var project struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		return ""
	}
	return strings.TrimSpace(project.Version)
}

// SummonShortcut is the global shortcut that opens the launcher: the user's
// stored `hotkey`, or the shipped Ctrl+Space. The key is one this package
// does not own, so it is read from the carried-through settings.
func SummonShortcut(s settings.Settings) string {
	if v, ok := s.Extra()["hotkey"].(string); ok {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return defaultSummonShortcut
}

// inset is the card's distance to the window edge per platform, from
// base.css's `.collapsed-shell` and the panels' own margin.
func (a *App) inset() (top, right, bottom, left float32) {
	if a.Surf != SurfaceLauncher {
		if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
			return 10, 10, 10, 10
		}
		return 0, 0, 0, 0
	}
	switch runtime.GOOS {
	case "linux":
		return 10, 10, 10, 10
	case "windows":
		// More room at the bottom and right for the card's shadow.
		return 4, 10, 12, 4
	default:
		return 0, 0, 0, 0
	}
}

// targetSize is the window size for the current surface and settings. The
// launcher opens at the band its last query left it at (the height follows
// the list once the View has measured it), so a summon lands where the
// search stood rather than at a fixed slab.
func (a *App) targetSize(s settings.Settings) (int, int) {
	switch a.Surf {
	case SurfaceSettings:
		return launcher.InputWindowWidth, SettingsHeight(s.UIScale, a.workArea)
	case SurfaceTerminal:
		return int(math.Round(s.TerminalWidth)), int(math.Round(s.TerminalHeight))
	default:
		return launcher.InputWindowWidth, a.launcherHeight(s)
	}
}

// launcherHeight is the launcher window's height: the band its held row
// count asks for when the View has measured the list, and the full slab
// before the first frame (a window opens at its query's height, not at a
// placeholder's).
func (a *App) launcherHeight(s settings.Settings) int {
	if a.Launcher == nil || a.Launcher.HeldRows == 0 {
		return int(math.Round(launcher.WindowHeight(s.UIScale)))
	}
	cap := 0.0
	if a.workArea > 0 {
		cap = a.workArea - 24
	}
	// The target height's filter band is the launcher's own: a mode's chips
	// row is on screen exactly while that mode owns the field, which the
	// launcher's state knows and the shell does not.
	return launcher.Geometry{
		Font:         a.Launcher.Font,
		Spacing:      a.Launcher.Spacing,
		TitleLine:    a.Launcher.TitleLine,
		SubtitleLine: a.Launcher.SubtitleLine,
		Rows:         a.Launcher.Rows,
		Held:         a.Launcher.HeldRows,
		Filter:       a.Launcher.FiltersVisible(),
		Feedback:     a.Launcher.FeedbackVisible(),
		Config:       a.Launcher.ConfigSchema(),
		Cap:          cap,
	}.Height()
}

// SettingsHeight is the settings panel's height at an interface step, capped
// to the work area exactly as the old panel did: the base height scaled by
// the step, the screen's 72%, and the screen less 24 DIPs, whichever is
// smallest (with the 420 minimum). A zero work area means no cap.
func SettingsHeight(uiScale string, workAreaHeight float64) int {
	base := settingsWindowHeight * settings.UIScaleFactor(uiScale)
	if workAreaHeight <= 0 {
		return int(math.Round(base))
	}
	cap1 := math.Max(settingsMinHeight, math.Floor(workAreaHeight*0.72))
	cap2 := math.Max(240, workAreaHeight-24)
	return int(math.Round(math.Min(base, math.Min(cap1, cap2))))
}

// resize applies the surface's size and resizability to the window.
func (a *App) resize() {
	if a.Win == nil {
		return
	}
	resizable := a.Surf == SurfaceTerminal
	if resizable != a.lastResizable {
		a.Win.SetResizable(resizable)
		a.lastResizable = resizable
	}
	minW, minH := 0, 0
	if resizable {
		minW, minH = settings.MinTerminalWidth, settings.MinTerminalHeight
	}
	if minW != a.lastMinW || minH != a.lastMinH {
		a.Win.SetMinimumSize(minW, minH)
		a.lastMinW, a.lastMinH = minW, minH
	}
	w, h := a.targetSize(a.Store.Snapshot())
	if w == a.lastW && h == a.lastH {
		return
	}
	a.lastW, a.lastH = w, h
	a.Win.SetSize(w, h)
	// The launcher's walk follows the surface's own resize: without this a
	// summon would land the window at the slab and the next View frame
	// would walk it somewhere else.
	if a.Surf == SurfaceLauncher {
		a.launcherTarget = h
	}
}

// customRequest is the form's draft as the host's request: one place, so the
// connect path and the edit path cannot disagree about what a field means.
func customRequest(tool settingsui.CustomTool) extensions.CustomRequest {
	request := extensions.CustomRequest{
		Name:           tool.Name,
		Mode:           tool.Mode,
		Command:        tool.Command,
		Version:        tool.Version,
		ExecutablePath: tool.Program,
		ScriptLanguage: tool.ScriptLanguage,
		ScriptContent:  tool.ScriptContent,
		Description:    tool.Description,
		ArgsPrefix:     strings.Fields(tool.Args),
		VersionArgs:    strings.Fields(tool.VersionArgs),
		Permissions:    tool.Permissions,
		Output:         tool.Output,
		Platforms:      tool.Platforms,
		Params:         tool.Params,
	}
	if request.Mode == "" {
		request.Mode = "executable"
	}
	if request.Mode == "script" {
		// A script's own word summons it; the program row is not part of the
		// form, so nothing here may carry one.
		request.ExecutablePath = ""
		request.ArgsPrefix = nil
		request.VersionArgs = nil
	} else {
		// The script fields belong to the script runtime alone: an executable
		// that carried a language would be refused as scriptLanguageInvalid
		// (the old host checked exactly this), and the form's language state
		// outlives a switch back to the program kind.
		request.ScriptLanguage = ""
		request.ScriptContent = ""
	}
	return request
}

// scriptContent reads a script integration's own file back, for the form that
// edits it. Empty when the file is gone, which the editor shows as an empty
// one rather than an error.
func scriptContent(integration extensions.Integration) string {
	if integration.Manifest.Runtime.Type != "script" {
		return ""
	}
	path := integration.Manifest.Runtime.Path
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(integration.PackageDir(), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("floter: could not read the script of %s: %v", integration.Entry.ID, err)
		return ""
	}
	return string(data)
}
