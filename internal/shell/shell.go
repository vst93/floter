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
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"floter/internal/shortcuts"
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
	// OpenPinned replaces the pinned-window implementation; nil opens the
	// real window, and tests record what would have been pinned.
	OpenPinned func(title, text string)
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
	// ConfirmPower answers the power confirmation; nil uses the native dialog,
	// and tests answer for themselves.
	ConfirmPower func(title string) bool
	// RunPowerCommand runs a power action; nil runs the platform's own
	// command, and tests record what would have run.
	RunPowerCommand func(action string) error
	// HomeDir is the user's home directory, where the browser plugin looks
	// for profiles; empty means os.UserHomeDir, and tests point it at a
	// fixture tree.
	HomeDir string
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

	// Win is the window, created by Start.
	Win *mygo.Window

	// Integrations is the extension state: the repository, the installed
	// manifests and the providers' commands.
	Integrations *extensions.Store
	// Usage is what the user launches, for the launcher's empty state.
	Usage *usage.Store
	// Paths is where the extension directories live.
	Paths extensions.Paths

	// Registry is the npm registry the install action reads.
	Registry *extensions.Registry
	// confirmPermissions approves an install's permissions, nil for the
	// native dialog.
	confirmPermissions func(extensions.PermissionApproval) bool
	// diagnoses is the last health check per integration, and diagnosisMu
	// guards it: a check runs off the main thread.
	diagnoses   map[string]settingsui.Integration
	diagnosisMu sync.Mutex

	// pins are the pinned-output windows, and openPinned replaces them in
	// tests.
	pins       map[*mygo.Window]*pinned
	pinMu      sync.Mutex
	openPinned func(title, text string)
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
	confirmPowerDialog   func(title string) bool
	runPowerCommand      func(action string) error
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
	a := &App{
		Store:                opts.Store,
		Surf:                 SurfaceLauncher,
		quit:                 quit,
		workArea:             opts.WorkAreaHeight,
		now:                  time.Now,
		Paths:                paths,
		Integrations:         extensions.OpenStore(paths),
		Usage:                usage.Open(paths.Root),
		refreshIntegrations:  opts.RefreshIntegrations,
		lastLaunchAtStartup:  opts.Store.Snapshot().LaunchAtStartup,
		openAtLogin:          opts.OpenAtLogin,
		setOpenAtLogin:       opts.SetOpenAtLogin,
		clipboardInterval:    clipboardPoll,
		Registry:             opts.Registry,
		confirmPermissions:   opts.ConfirmPermissions,
		openPinned:           opts.OpenPinned,
		writeClipboard:       opts.WriteClipboard,
		registerShortcut:     opts.RegisterShortcut,
		unregisterShortcut:   opts.UnregisterShortcut,
		openExternalTerminal: opts.OpenExternalTerminal,
		silentCommand:        opts.RunSilentCommand,
		saveFileDialog:       opts.SaveFileDialog,
		openFileDialog:       opts.OpenFileDialog,
		notify:               opts.Notify,
		confirmPowerDialog:   opts.ConfirmPower,
		runPowerCommand:      opts.RunPowerCommand,
		homeDir:              opts.HomeDir,
		appIcon:              storedAppIcon(opts.Store.Snapshot()),
		lastAppIcon:          storedAppIcon(opts.Store.Snapshot()),
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
		if a.Terminal != nil {
			if err := a.Terminal.SaveSnapshot(); err != nil {
				log.Printf("floter: could not save the terminal snapshot: %v", err)
			}
		}
		innerQuit()
	}
	a.Launcher = launcher.New(opts.Store, launcher.Actions{
		OpenSettings: func() { a.Open(SurfaceSettings) },
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
		Complete:           a.completeCommand,
		PinText:            a.PinText,
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
		RunInTerminalWithEnv: func(argv []string, env []string) {
			if err := a.Terminal.RunCommand(argv, "current", env); err != nil {
				log.Printf("floter: could not run %v: %v", argv, err)
			}
			a.Open(SurfaceTerminal)
		},
		SearchBrowser: a.searchBrowser,
	})
	a.Settings = settingsui.New(opts.Store, settingsui.Actions{
		Close:               func() { a.Open(SurfaceLauncher) },
		CloseSession:        func() { a.Terminal.Close() },
		DiagnoseIntegration: a.diagnoseIntegration,
		ReprobeCommands:     a.reprobeCommands,
		SetShortcut:         a.setShortcut,
		SetPage:             a.rememberSettingsPage,
		BrowserTargets:      a.browserTargets,
		SetCommandEnabled:   a.setCommandEnabled,
		SetCommandAlias:     a.setCommandAlias,
		CustomShortcuts:     func() []settings.CustomShortcut { return settings.CustomShortcutsOf(a.Store.Snapshot()) },
		SetCustomShortcuts:  a.setCustomShortcuts,
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
		UninstallComponents: func(id, name string, components settingsui.UninstallComponents) {
			// The dialog blocks, so the removal runs off the main thread.
			go func() {
				copy := i18n.For(a.Store.Snapshot().Language).Settings
				result, err := mygo.Dialog.Message(mygo.MessageOptions{
					Type:    mygo.MessageWarning,
					Message: copy.IntegrationsRemoveTitle(name),
					Detail:  copy.IntegrationsRemoveDetail,
					Buttons: []string{copy.IntegrationsUninstall, "Cancel"},
				})
				if err != nil || result.Button != 0 {
					return
				}
				result2, err := extensions.UninstallComponentized(a.Paths, id, extensions.UninstallComponents{
					RemoveHostConfig: components.RemoveHostConfig,
					RemoveToolData:   components.RemoveToolData,
					RemoveArtifacts:  components.RemoveArtifacts,
				})
				if err != nil {
					log.Printf("floter: could not uninstall %s: %v", id, err)
					return
				}
				a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationRemoved(name) })
				log.Printf("floter: uninstalled %s (config=%v data=%v artifacts=%v)",
					id, result2.RemovedHostConfig, result2.RemovedToolData, result2.RemovedArtifacts)
				a.RefreshIntegrations(context.Background())
			}()
		},
		UninstallIntegration: func(id, name string) {
			// The dialog blocks, so it runs off the main thread and the
			// result comes back to it.
			go func() {
				copy := i18n.For(a.Store.Snapshot().Language).Settings
				result, err := mygo.Dialog.Message(mygo.MessageOptions{
					Type:    mygo.MessageWarning,
					Message: copy.IntegrationsRemoveTitle(name),
					Detail:  copy.IntegrationsRemoveDetail,
					Buttons: []string{copy.IntegrationsUninstall, "Cancel"},
				})
				if err != nil || result.Button != 0 {
					return
				}
				a.onMain(func() {
					if err := extensions.Uninstall(a.Paths, id, false); err != nil {
						log.Printf("floter: could not uninstall %s: %v", id, err)
						a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationRemoveFailed(name) })
						return
					}
					a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationRemoved(name) })
				})
				a.RefreshIntegrations(context.Background())
			}()
		},
		CheckForUpdates:       a.checkForUpdates,
		InstallUpdate:         a.installUpdate,
		ConnectRecommended:    a.connectRecommended,
		SaveConfiguration:     a.saveIntegrationConfiguration,
		ClearClipboardHistory: func() { a.clearClipboardHistoryAsks(a.confirmClearClipboard) },
		ResetShortcuts:        a.resetShortcuts,
		SetShortcutsSuspended: a.setShortcutsSuspended,
		ExportIntegrations:    a.exportIntegrations,
		ImportIntegrations:    a.importIntegrations,
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
			go func() {
				copy := i18n.For(a.Store.Snapshot().Language).Settings
				result, err := mygo.Dialog.Message(mygo.MessageOptions{
					Type:    mygo.MessageWarning,
					Message: copy.IntegrationsRemoveTitle(id),
					Detail:  copy.IntegrationsRemoveDetail,
					Buttons: []string{copy.IntegrationsDeleteOrphan, "Cancel"},
				})
				if err != nil || result.Button != 0 {
					return
				}
				if err := extensions.DeleteOrphan(a.Paths, id); err != nil {
					log.Printf("floter: could not delete %s: %v", id, err)
				}
				a.RefreshIntegrations(context.Background())
			}()
		},
		SetIntegrationEnabled: func(id string, enabled bool) {
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
		if a.Terminal.Term == nil {
			return nil
		}
		return []settingsui.Session{{Title: a.Terminal.Label(), Running: true}}
	}

	// The plugin's callbacks run on the terminal's own goroutine, so they
	// hand the main thread the work of touching the app's state.
	termActions := terminalui.Actions{
		Close:      func() { a.Open(SurfaceLauncher) },
		NewCommand: func() { a.Launcher.ResetQuery(); a.Open(SurfaceLauncher) },
		OpenExternal: func() {
			// The session is handed to the system's own terminal: the Go
			// build runs its terminal in-process, so this opens a new window
			// rather than attaching to the running one.
			if err := a.OpenExternalTerminal(); err != nil {
				log.Printf("floter: could not open a terminal: %v", err)
			}
		},
		Title: func(title string) {
			a.onMain(func() {
				a.Terminal.Title = title
				if a.Win != nil && title != "" {
					a.Win.SetTitle(title)
				}
			})
		},
		Exit: func(int) { a.onMain(func() { a.Open(SurfaceLauncher) }) },
	}
	termActions.Pin = a.PinText
	a.Terminal = terminalui.New(opts.Store, termActions, newTerminal)
	a.Terminal.SnapshotPath = filepath.Join(paths.Root, "terminal-snapshot")

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
			if icon := storedAppIcon(s); icon != a.lastAppIcon {
				a.lastAppIcon, a.appIcon = icon, icon
				a.ApplyAppIcon()
			}
			if settings.ShowMenubarIcon(s) != a.lastMenubarIcon {
				a.lastMenubarIcon = settings.ShowMenubarIcon(s)
				a.ApplyMenubarIcon()
			}
			if customShortcutSignature(s) != a.customSignature {
				a.ApplyCustomShortcuts()
			}
			if a.Surf == SurfaceTerminal {
				a.Terminal.Refresh()
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
	if a.workArea <= 0 {
		if displays := mygo.Screen.Displays(); len(displays) > 0 {
			a.workArea = float64(displays[0].WorkArea.Height)
		}
	}
	s := a.Store.Snapshot()
	w, h := a.targetSize(s)
	a.lastW, a.lastH = w, h

	a.Win = mygo.NewWindow(mygo.WindowOptions{
		Title:         "floter",
		Width:         w,
		Height:        h,
		Frameless:     true,
		Transparent:   true,
		DisableShadow: true,
		SkipTaskbar:   true,
		// The launcher and settings are fixed panels; only the terminal
		// is resizable, which resize turns on per surface.
		DisableResize: true,
		Content:       ui.View(a.View),
	})
	a.Win.OnClosed(func() {
		if runtime.GOOS != "darwin" {
			a.quit()
		}
	})
	a.Win.OnFocus(func() { a.shownAt = a.now() })
	a.Win.OnResize(a.trackTerminalSize)
	a.Win.OnFileDrop(a.handleFileDrop)
	a.Win.OnBlur(func() {
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
		}
	})
	a.ApplyStartup()
	a.InstallMenu()
	a.lastMenubarIcon = settings.ShowMenubarIcon(a.Store.Snapshot())
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
	if !settings.ShowCommandsInSearch(snapshot) {
		a.Launcher.SetTools(nil, false, nil)
		return
	}
	aliases := settings.ResolveCommandAliases(settings.CommandAliasesOf(snapshot))
	go func() {
		found := apps.ScanCommands(apps.CommandDirs())
		a.onMain(func() { a.Launcher.SetTools(found, true, aliases) })
	}()
}

// scanToolCatalog resolves the install catalog against this machine's search
// path, in the background: detection is a stat scan, and the rows only matter
// once the launcher is open.
func (a *App) scanToolCatalog() {
	go func() {
		states := tools.Look(extensions.SearchDirectories(), tools.Platform())
		a.onMain(func() { a.Launcher.SetToolCatalog(states) })
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
// show them.
func (a *App) refreshRecents() {
	known := map[string]bool{}
	for _, app := range a.Launcher.Apps {
		known[app.Path] = true
	}
	paths := a.Usage.Top(maxRecentApps, func(path string) bool { return known[path] })
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

// maxRecentApps is how many recent applications the launcher offers.
const maxRecentApps = 5

// RefreshIntegrations reloads the extension state and the providers'
// commands, then hands them to the launcher. It blocks while providers run,
// so the app calls it from a goroutine.
func (a *App) RefreshIntegrations(ctx context.Context) {
	a.Integrations.Refresh(ctx)
	a.onMain(func() { a.Launcher.SetCommands(a.launcherCommands()) })
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
		item.Commands = commands[item.ID]
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
		a.onMain(func() { done(launcher.BrowserResults{}) })
		return
	}
	go func() {
		profiles := a.browserProfileList(options.customBaseDir)
		profile, found := browser.DefaultProfile(profiles, options.target)
		if !found {
			a.onMain(func() { done(launcher.BrowserResults{}) })
			return
		}
		results := browser.Search(context.Background(), []browser.Profile{profile}, query, browser.Options{
			Days:      options.historyDays,
			Limit:     options.limit,
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
		if options.limit > 0 && len(tabs) > options.limit {
			tabs = tabs[:options.limit]
		}
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
		a.onMain(func() {
			if err := a.Clipboard.Clear(); err != nil {
				log.Printf("floter: could not clear the clipboard history: %v", err)
				return
			}
			if a.Win != nil {
				a.Win.Invalidate()
			}
		})
	}()
}

// openInstallSession opens a bare terminal session with the install command
// typed into it: the shell is the user's (their proxy, mirror and environment
// already in effect), and it outlives the install.
func (a *App) openInstallSession(command string) {
	a.Terminal.RunShellWithCommand(command)
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
		approved := true
		if prepared.Approval.NeedsApproval() {
			approved = a.confirmInstallPermissions(prepared.Approval)
		}
		entry, err := prepared.Commit(approved)
		if err != nil {
			log.Printf("floter: could not connect %s: %v", id, err)
			a.notifyCompletion(func(c i18n.Notifications) string { return c.IntegrationInstallFailed(id) })
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
func (a *App) openURL(browserID, url string) {
	if browserID != "" {
		if err := browser.OpenURL(browserID, url); err == nil {
			return
		} else {
			log.Printf("floter: could not open %s in %s: %v", url, browserID, err)
		}
	}
	mygo.Shell.OpenExternal(url)
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
	go func() {
		run, err := extensions.RunCaptured(context.Background(), entry, args)
		a.onMain(func() {
			done(run, err)
			a.notifyRun(entry, run, err)
		})
	}()
}

// notifyRun raises a system notification for a background run that finished
// while the panel was hidden. A visible panel shows the output itself, so a
// notification there would be a second copy of the same news.
func (a *App) notifyRun(entry extensions.CommandEntry, run extensions.CapturedRun, err error) {
	if a.Win != nil && a.Win.IsVisible() {
		return
	}
	copy := i18n.NotificationsFor(a.Store.Snapshot().Language)
	name := entry.Command.Name
	if name == "" {
		name = entry.Command.ID
	}
	a.Notify(copy.Title, copy.CommandFinished(name)+"  \u00b7  "+copy.CommandStatus(a.Launcher.RunStatus(run, err)))
}

// notifyCompletion raises a system notification for a background integration
// task that finished while the panel was hidden. A visible panel shows the
// result itself, so a notification there would be a second copy of the same
// news.
func (a *App) notifyCompletion(render func(i18n.Notifications) string) {
	if a.Win != nil && a.Win.IsVisible() {
		return
	}
	copy := i18n.NotificationsFor(a.Store.Snapshot().Language)
	a.Notify(copy.Title, render(copy))
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
	if s != SurfaceLauncher {
		// Entering a surface starts the residency clock (see summon).
		a.enteredAt = a.now()
	}
	a.Surf = s
	switch s {
	case SurfaceSettings:
		a.restoreSettingsPage()
		a.Settings.FocusSidebar()
	case SurfaceTerminal:
		a.Terminal.EnsureSession()
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
	if a.residencyHolds() {
		// The surface the user was on is still "where they left it": show
		// it again rather than throwing it away.
		a.Show()
		return
	}
	a.Open(SurfaceLauncher)
	a.Show()
}

// Show reveals the window and takes the focus, arming the blur grace.
func (a *App) Show() {
	if a.Win == nil {
		return
	}
	a.shownAt = a.now()
	a.Win.Show()
	a.Win.Focus()
	a.Win.Invalidate()
}

// residencyHolds reports whether the current surface survives this summon:
// the shipped ten seconds since it was opened, a custom window, "never", or
// nothing at all (0). See src/surface-residency.ts.
func (a *App) residencyHolds() bool {
	if a.Surf == SurfaceLauncher {
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

// walkLauncherHeight moves the window to the launcher's target height, one
// eased step per frame: the old shell's edge walk. The element's Animate
// keeps the frames coming until the value lands, and the last step's
// SetSize is the only one the platform sees at the end. A window that is
// already there does nothing — the common case, a keystroke that left the
// list the same size.
func (a *App) walkLauncherHeight(c *ui.Context) {
	if a.Win == nil || a.Surf != SurfaceLauncher || a.launcherTarget == 0 {
		return
	}
	current := float32(a.lastH)
	target := float32(a.launcherTarget)
	// The animation's carrier: a zero-size element that lays out nothing,
	// whose Animate state lives across the frames the walk needs.
	animated := ui.Box(c).Size(0, 0).Opacity(0)
	height := animated.Animate("launcher.height", target, 150*time.Millisecond)
	if int(math.Round(float64(height))) == int(current) {
		return
	}
	a.Win.SetSize(launcher.InputWindowWidth, int(math.Round(float64(height))))
	a.lastW, a.lastH = launcher.InputWindowWidth, int(math.Round(float64(height)))
}

// Hide hides the window, leaving the app running with its shortcut.
func (a *App) Hide() {
	if a.Win != nil {
		a.Win.Hide()
	}
}

// appShortcuts runs the old build's dismiss table, surface by surface. The
// literal ⌘W is not rebindable, and neither is its meaning everywhere: on
// the launcher and the settings it hides (or closes), on the terminal it is
// the shell's own delete-word and is left alone. The user's new-command
// binding (⌘W by default) has a rule of its own per surface: on the
// launcher it hides, from the terminal it returns to the launcher, from
// the settings it closes them. Escape is the launcher's and the settings'
// own, and the terminal shell keeps it.
func (a *App) appShortcuts(c *ui.Context) {
	// One Shortcut() call per key per frame consumes the delivered press, so
	// the triggers are resolved per surface: the terminal asks only the
	// new-command binding (the literal ⌘W is the shell's delete-word), the
	// settings ask all three, the launcher folds the literal ⌘W into the
	// new-command rule (the same key by default) and leaves Escape to the
	// launcher's own view (an empty field's Escape hides, one with text
	// clears).
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
			return
		}
		if c.Shortcut(0, ui.KeyEscape) {
			a.Open(SurfaceLauncher)
		}
	default:
		if c.Shortcut(ui.Cmd, ui.KeyW) {
			a.Hide()
			return
		}
		if mods, key, ok := shortcuts.Parse(settings.Shortcut(a.Store.Snapshot(), settings.ShortcutNewCommand)); ok && c.Shortcut(mods, key) {
			a.Hide()
		}
	}
}

// returnToInput is the way back from another surface: the launcher, with
// an empty field, the way the old build's returnToInputMode did.
func (a *App) returnToInput() {
	a.Launcher.ResetQuery()
	a.Open(SurfaceLauncher)
}

// View builds the window: the transparent root, the inset card, its glass
// material (or plain face for the `off` stop), and the active surface.
func (a *App) View(c *ui.Context) {
	s := a.Store.Snapshot()
	surface := launcher.Resolve(s, c.Theme().Dark)
	c.SetTheme(surface.Tokens.Theme)

	// The app's own keys answer on every surface: ⌘W hides whatever the
	// window is showing (the old build's dismiss table was surface-wide),
	// and a new command returns to the launcher from the terminal or the
	// settings as well as from the launcher's own modes.
	a.appShortcuts(c)

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
		card := ui.Column(c).Fill().Radius(radius).DragWindow().Border(1, stroke)
		material := glassmap.Material(surface.Glass, t)
		if material == nil {
			// The `off` stop: a plain, near-solid face and no material.
			card.Background(t.Surface).Padding(t.Space(2.5))
			card.Children(func() { a.surface(c) })
			return
		}
		card.Children(func() {
			// The haze veil is composited *under* the glass, as the old
			// `--glass-step-dim` was under `--glass-tint-alpha`. Built after
			// the face it paints over it — and a plain box over the rows
			// would eat their clicks, so the veil lets the pointer through:
			// it is a color, never a control.
			if haze := glassmap.Haze(surface.Glass, t); haze != ui.Transparent {
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius).Background(haze).PassThrough()
			}
			face := ui.Column(c).Fill().Radius(radius).Material(material).Padding(t.Space(2.5)).DragWindow()
			face.Children(func() { a.surface(c) })
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
		Font:     a.Launcher.Font,
		Spacing:  a.Launcher.Spacing,
		RowLines: a.Launcher.RowLines,
		Held:     a.Launcher.HeldRows,
		Filter:   a.Launcher.FiltersVisible(),
		Cap:      cap,
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
