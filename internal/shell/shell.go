// Package shell is the mygo app itself: one frameless window whose body is
// one of three surfaces (launcher, settings, terminal), the settings store
// they read and write, the theme they paint with, and the global shortcut
// that summons the window.
//
// The old Tauri build used the same model — a single panel that swapped its
// body and resized per surface (see src/surface-residency.ts) — so the
// transition behaviour and the window geometry carry over.
package shell

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/clipboard"
	"floter/internal/extensions"
	"floter/internal/glassmap"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"floter/internal/terminalui"
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

// The window geometry per surface, from the old build: the launcher is the
// 720-wide collapsed panel, settings is a 720×580 work panel capped to the
// screen, and the terminal is the 860×600 default (resizable, min 640×360).
const (
	settingsWindowHeight = 580
	settingsMinHeight    = 420

	// defaultSummonShortcut is DEFAULT_TOGGLE_WINDOW from config.rs: what
	// the old build registered when the user never changed it.
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
}

// App is the running application.
type App struct {
	Store *settings.Store

	Surf     Surface
	Launcher *launcher.App
	Settings *settingsui.App
	Terminal *terminalui.App

	// Win is the window, created by Start.
	Win *mygo.Window

	// Integrations is the extension state: the repository, the installed
	// manifests and the providers' commands.
	Integrations *extensions.Store
	// Paths is where the extension directories live.
	Paths extensions.Paths

	// Registry is the npm registry the install action reads.
	Registry *extensions.Registry

	// Clipboard is the clipboard history the launcher searches.
	Clipboard *clipboard.Store
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
	// LastClipboardSettings is the clipboard state the watcher was last
	// synced with.
	LastClipboardSettings clipboardSettings

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
		Store:               opts.Store,
		Surf:                SurfaceLauncher,
		quit:                quit,
		workArea:            opts.WorkAreaHeight,
		now:                 time.Now,
		Paths:               paths,
		Integrations:        extensions.OpenStore(paths),
		refreshIntegrations: opts.RefreshIntegrations,
		lastLaunchAtStartup: opts.Store.Snapshot().LaunchAtStartup,
		openAtLogin:         opts.OpenAtLogin,
		setOpenAtLogin:      opts.SetOpenAtLogin,
		clipboardInterval:   clipboardPoll,
		Registry:            opts.Registry,
		appIcon:             storedAppIcon(opts.Store.Snapshot()),
		lastAppIcon:         storedAppIcon(opts.Store.Snapshot()),
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
	a.Launcher = launcher.New(opts.Store, launcher.Actions{
		OpenSettings: func() { a.Open(SurfaceSettings) },
		OpenTerminal: func() { a.Open(SurfaceTerminal) },
		Quit:         quit,
		Dismiss:      a.Hide,
		Copy: func(text string) {
			a.selfCopied = text
			mygo.Clipboard.WriteText(text)
		},
		OpenApp: func(app apps.App) {
			if err := app.Open(); err != nil {
				log.Printf("floter: could not open %s: %v", app.Name, err)
				return
			}
			a.Hide()
		},
		RunCommand: a.runCommand,
	})
	a.Settings = settingsui.New(opts.Store, settingsui.Actions{
		Close:        func() { a.Open(SurfaceLauncher) },
		CloseSession: func() { a.Terminal.Close() },
		InstallFromRegistry: func(name, constraint string) {
			go func() {
				entry, err := extensions.InstallFromRegistry(context.Background(), a.Paths, a.Registry, name, constraint)
				if err != nil {
					log.Printf("floter: could not install %s: %v", name, err)
					return
				}
				log.Printf("floter: installed %s %s", entry.Name, entry.PackageVersion)
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
						return
					}
				})
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
	a.Clipboard = clipboard.NewStore(clipboard.FromConfigRoot(paths.Root), clipboardMaxItems(opts.Store.Snapshot()))
	a.Launcher.Clipboard = a.Clipboard
	a.LastClipboardSettings = clipboardState(opts.Store.Snapshot())

	a.Settings.Integrations = func() []settingsui.Integration { return a.integrationList() }
	a.Settings.Sessions = func() []settingsui.Session {
		if a.Terminal.Term == nil {
			return nil
		}
		return []settingsui.Session{{Title: a.Terminal.Label(), Running: true}}
	}

	// The plugin's callbacks run on the terminal's own goroutine, so they
	// hand the main thread the work of touching the app's state.
	termActions := terminalui.Actions{
		Close: func() { a.Open(SurfaceLauncher) },
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
	a.Terminal = terminalui.New(opts.Store, termActions, newTerminal)

	// A settings change that touches the clipboard history is applied to
	// the store and the watcher.
	opts.Store.OnChange(func(s settings.Settings) {
		state := clipboardState(s)
		if state != a.LastClipboardSettings {
			a.LastClipboardSettings = state
			a.onMain(func() {
				a.Clipboard.SetMaxItems(state.maxItems)
				if state.enabled && !a.clipboardWatching {
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

	a.ShortcutErr = mygo.GlobalShortcut.Register(SummonShortcut(s), a.Toggle)
	if a.ShortcutErr != nil {
		log.Printf("floter: could not register %s: %v", SummonShortcut(s), a.ShortcutErr)
	}
	if err := a.RegisterScheme(); err != nil {
		log.Printf("floter: could not register %s://: %v", scheme, err)
	}
	a.ApplyStartup()
	a.InstallMenu()
	a.InstallTray()
	a.ApplyAppIcon()
	a.watchClipboard()
	a.Launcher.FocusSearch()
	a.scanApps()
	if a.refreshIntegrations {
		go a.RefreshIntegrations(context.Background())
	}
}

// RefreshIntegrations reloads the extension state and the providers'
// commands, then hands them to the launcher. It blocks while providers run,
// so the app calls it from a goroutine.
func (a *App) RefreshIntegrations(ctx context.Context) {
	a.Integrations.Refresh(ctx)
	a.onMain(func() { a.Launcher.SetCommands(a.Integrations.CommandEntries()) })
}

// integrationList maps the store's inventory onto the settings list: the
// repository's record, the manifest's copy, and why a provider failed.
func (a *App) integrationList() []settingsui.Integration {
	inventory := a.Integrations.Inventory()
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
		if err := a.Integrations.DescribeError(integration.Entry.ID); err != nil {
			item.Error = err.Error()
		}
		out = append(out, item)
	}
	for _, id := range inventory.Orphans {
		out = append(out, settingsui.Integration{ID: id, Name: id, Orphan: true})
	}
	return out
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

// scanApps reads the installed applications in the background and hands
// them to the launcher, so the window opens at once and the search grows as
// the scan lands.
func (a *App) scanApps() {
	go func() {
		found := apps.Scan(apps.Roots())
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

// Hide hides the window, leaving the app running with its shortcut.
func (a *App) Hide() {
	if a.Win != nil {
		a.Win.Hide()
	}
}

// View builds the window: the transparent root, the inset card, its glass
// material (or plain face for the `off` stop), and the active surface.
func (a *App) View(c *ui.Context) {
	s := a.Store.Snapshot()
	surface := launcher.Resolve(s, c.Theme().Dark)
	c.SetTheme(surface.Tokens.Theme)

	// Only the card paints; the window's transparent margin shows the
	// desktop, which the glass reads through.
	c.Root().Background(ui.Transparent)

	t := c.Theme()
	top, right, bottom, left := a.inset()
	ui.Column(c).Fill().Padding(top, right, bottom, left).Children(func() {
		radius := surface.Tokens.RadiusLG
		card := ui.Column(c).Fill().Radius(radius).DragWindow()
		material := glassmap.Material(surface.Glass, t)
		if material == nil {
			// The `off` stop: a plain, near-solid face and no material.
			card.Background(t.Surface).Padding(t.Space(2.5))
			card.Children(func() { a.surface(c) })
			return
		}
		card.Children(func() {
			// The haze veil is composited *under* the glass, as the old
			// `--glass-step-dim` was under `--glass-tint-alpha`.
			if haze := glassmap.Haze(surface.Glass, t); haze != ui.Transparent {
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius).Background(haze)
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

// targetSize is the window size for the current surface and settings.
func (a *App) targetSize(s settings.Settings) (int, int) {
	switch a.Surf {
	case SurfaceSettings:
		return launcher.InputWindowWidth, SettingsHeight(s.UIScale, a.workArea)
	case SurfaceTerminal:
		return int(math.Round(s.TerminalWidth)), int(math.Round(s.TerminalHeight))
	default:
		return launcher.InputWindowWidth, int(math.Round(launcher.WindowHeight(s.UIScale)))
	}
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
}
