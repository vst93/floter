package shell

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"floter/internal/clipboard"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// The app icons: the two the Tauri build shipped, resized to 32×32 for the
// menu bar and the taskbar. Dark is the default, as config.rs had it.
var (
	//go:embed assets/tray-dark.png
	trayIconDark []byte
	//go:embed assets/tray-light.png
	trayIconLight []byte
)

// appIconBytes is the icon for a stored app icon name: anything but "light"
// is the dark icon, as the old normalization decided.
func appIconBytes(name string) []byte {
	if name == "light" {
		return trayIconLight
	}
	return trayIconDark
}

// storedAppIcon is the app icon the settings file asks for.
func storedAppIcon(s settings.Settings) string {
	value, _ := s.Extra()["app_icon"].(string)
	if value == "light" {
		return "light"
	}
	return "dark"
}

// ApplyAppIcon puts the stored icon on the tray and the window, so a change
// lands at once, as the old build's did.
func (a *App) ApplyAppIcon() {
	icon := appIconBytes(a.appIcon)
	if a.Tray != nil {
		if err := a.Tray.SetIcon(icon, false); err != nil {
			log.Printf("floter: could not set the tray icon: %v", err)
		}
	}
	if a.Win != nil {
		if err := a.Win.SetIcon(icon); err != nil {
			log.Printf("floter: could not set the window icon: %v", err)
		}
	}
}

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
		Icon:    appIconBytes(a.appIcon),
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

// clipboardPoll is how often the watcher reads the clipboard: often enough
// that a copy lands in the history before the panel opens, rare enough to
// cost nothing.
const clipboardPoll = 600 * time.Millisecond

// clipboardSettings is the clipboard state the settings file carries: the
// keys are ones the app does not own, so they ride in the carried-through
// map.
type clipboardSettings struct {
	enabled  bool
	maxItems int
}

// clipboardState reads the clipboard settings, with the shipped defaults:
// the history is on, holding three hundred entries.
func clipboardState(s settings.Settings) clipboardSettings {
	state := clipboardSettings{enabled: true, maxItems: clipboard.DefaultMaxItems}
	extra := s.Extra()
	if value, ok := extra["clipboard_history_enabled"].(bool); ok {
		state.enabled = value
	}
	if value, ok := extra["clipboard_history_max_items"]; ok {
		if number, ok := numeric(value); ok {
			state.maxItems = int(number)
		}
	}
	return state
}

// clipboardMaxItems is the capacity the settings file asks for.
func clipboardMaxItems(s settings.Settings) int { return clipboardState(s).maxItems }

// numeric reads a JSON number of either shape the decoder may produce.
func numeric(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// watchClipboard records what the user copies while the app runs: the store
// owns the history, the watcher only feeds it.
func (a *App) watchClipboard() {
	if a.clipboardWatching || !clipboardState(a.Store.Snapshot()).enabled {
		return
	}
	a.clipboardWatching = true
	interval := a.clipboardInterval
	if interval <= 0 {
		interval = clipboardPoll
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if !clipboardState(a.Store.Snapshot()).enabled {
				a.clipboardWatching = false
				return
			}
			text := mygo.Clipboard.ReadText()
			if strings.TrimSpace(text) == "" || text == a.selfCopied {
				continue
			}
			entry, added, err := a.Clipboard.AddText(text)
			if err != nil || !added {
				continue
			}
			_ = entry
			a.onMain(func() {
				if a.Win != nil {
					a.Win.Invalidate()
				}
			})
		}
	}()
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
