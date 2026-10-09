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
	"encoding/json"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/glassmap"
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

	quit     func()
	workArea float64

	lastW, lastH       int
	lastResizable      bool
	lastMinW, lastMinH int

	// ShortcutErr is why the global shortcut could not be registered.
	ShortcutErr error
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
	a := &App{
		Store:    opts.Store,
		Surf:     SurfaceLauncher,
		quit:     quit,
		workArea: opts.WorkAreaHeight,
	}
	a.Launcher = launcher.New(opts.Store, launcher.Actions{
		OpenSettings: func() { a.Open(SurfaceSettings) },
		OpenTerminal: func() { a.Open(SurfaceTerminal) },
		Quit:         quit,
		Dismiss:      a.Hide,
		Copy:         func(text string) { mygo.Clipboard.WriteText(text) },
		OpenApp: func(app apps.App) {
			if err := app.Open(); err != nil {
				log.Printf("floter: could not open %s: %v", app.Name, err)
				return
			}
			a.Hide()
		},
	})
	a.Settings = settingsui.New(opts.Store, settingsui.Actions{
		Close:        func() { a.Open(SurfaceLauncher) },
		CloseSession: func() { a.Terminal.Close() },
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

	// A settings change lands on the open surface: the panel re-measures
	// (a new interface size), the terminal takes its new font and palette,
	// and the frame redraws either way.
	opts.Store.OnChange(func(s settings.Settings) {
		a.onMain(func() {
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

	a.ShortcutErr = mygo.GlobalShortcut.Register(SummonShortcut(s), a.Toggle)
	if a.ShortcutErr != nil {
		log.Printf("floter: could not register %s: %v", SummonShortcut(s), a.ShortcutErr)
	}
	a.Launcher.FocusSearch()
	a.scanApps()
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
	if a.Win != nil {
		a.Win.Show()
		a.Win.Focus()
		a.Win.Invalidate()
	}
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
	a.Open(SurfaceLauncher)
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
