package shell

import (
	_ "embed"
	"log"
	"net/url"
	"strings"

	"github.com/egoist/mygo"

	"floter/internal/i18n"
)

// trayIcon is the application icon shown in the menu bar or notification
// area: the 32×32 app icon the Tauri build shipped, copied beside this
// package.
//
//go:embed assets/tray.png
var trayIcon []byte

// scheme is the URL scheme the app answers, as the old build registered
// `floter://`.
const scheme = "floter"

// InstallTray adds the menu bar icon: show, settings, terminal, quit.
func (a *App) InstallTray() {
	copy := i18n.For(a.Store.Snapshot().Language).Launcher
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: copy.CommandSettings, Click: func(*mygo.MenuItem, *mygo.Window) { a.Open(SurfaceSettings) }},
		{Label: copy.CommandTerminal, Click: func(*mygo.MenuItem, *mygo.Window) { a.Open(SurfaceTerminal) }},
		mygo.Separator(),
		{Label: copy.CommandQuit, Click: func(*mygo.MenuItem, *mygo.Window) { a.quit() }},
	})

	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:    trayIcon,
		ToolTip: "floter",
		Menu:    menu,
	})
	if err != nil {
		log.Printf("floter: could not add the tray icon: %v", err)
		return
	}
	a.Tray = tray
	tray.OnClick(func() { a.Toggle() })
}

// RegisterScheme makes the app the system's handler for floter:// URLs. A
// packaged app declares the scheme in its bundle; a portable build registers
// it at run time, which is cheap to repeat.
func (a *App) RegisterScheme() error {
	return mygo.App.RegisterURLScheme(scheme)
}

// HandleURL routes a floter:// URL: settings and terminal open their
// surface, a query prefills the launcher, and anything else opens the
// launcher.
func (a *App) HandleURL(rawURL string) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.EqualFold(parsed.Scheme, scheme) {
		a.Open(SurfaceLauncher)
		return
	}
	switch strings.ToLower(parsed.Host) {
	case "settings":
		a.Open(SurfaceSettings)
	case "terminal":
		a.Open(SurfaceTerminal)
	default:
		a.Open(SurfaceLauncher)
		// The query rides either the host (floter://?q=x) or the path
		// (floter://search?q=x).
		query := parsed.Query().Get("q")
		if query == "" {
			query = parsed.Query().Get("query")
		}
		if query != "" {
			a.Launcher.SetQuery(query)
		}
	}
}

// ApplyStartup syncs the system's login item with the setting, so a change
// made in another build (or by hand in the file) takes effect here.
func (a *App) ApplyStartup() {
	want := a.Store.Snapshot().LaunchAtStartup
	if a.openAtLogin == nil || a.setOpenAtLogin == nil {
		return
	}
	if a.openAtLogin() == want {
		return
	}
	if err := a.setOpenAtLogin(want); err != nil {
		log.Printf("floter: could not set the login item: %v", err)
	}
}
