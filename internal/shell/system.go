package shell

import (
	"bytes"
	_ "embed"
	"encoding/json"
	stdpng "image/png"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// The app icons, resized to 32×32 for the menu bar and the taskbar. Dark is
// the default.
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

// browserOptions is the browser plugin's settings, as the search uses them.
type browserOptions struct {
	enabled       bool
	target        string
	customBaseDir string
	historyDays   int
	limit         int
	sortOrder     string
	searchField   string
	cdpEnabled    bool
	cdpPort       int
}

// browserState reads the browser plugin's settings block.
func browserState(s settings.Settings) browserOptions {
	plugin := settings.BrowserPluginOf(s)
	return browserOptions{
		enabled:       plugin.Enabled,
		target:        plugin.Target,
		customBaseDir: plugin.CustomBaseDir,
		historyDays:   plugin.HistoryDays,
		limit:         browserResultLimit,
		sortOrder:     plugin.SortOrder,
		searchField:   plugin.SearchField,
		cdpEnabled:    plugin.CDPEnabled,
		cdpPort:       plugin.CDPPort,
	}
}

// browserResultLimit is how many rows one browser search returns: the old
// build's default page size.
const browserResultLimit = 50

// clipboardPoll is how often the watcher reads the clipboard: often enough
// that a copy lands in the history before the panel opens, rare enough to
// cost nothing.
const clipboardPoll = 600 * time.Millisecond

// clipboardState reads the clipboard history's settings.
func clipboardState(s settings.Settings) settings.ClipboardSettings {
	return settings.ClipboardOf(s)
}

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
	if a.clipboardWatching || !clipboardState(a.Store.Snapshot()).Enabled {
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
			if !clipboardState(a.Store.Snapshot()).Enabled {
				a.clipboardWatching = false
				return
			}
			if a.captureClipboard() {
				a.onMain(func() {
					if a.Win != nil {
						a.Win.Invalidate()
					}
				})
			}
		}
	}()
}

// captureClipboard records what is on the clipboard now, if it is something
// new: an image, a file list, or text. It reports whether the history grew.
//
// The formats decide first because reading a large image on every poll would
// be wasteful: text is cheap to read and compare every time, and the image
// path is only taken when the clipboard's format list changed.
func (a *App) captureClipboard() bool {
	text := mygo.Clipboard.ReadText()
	if strings.TrimSpace(text) != "" && text != a.selfCopied && text != a.lastClipboardText {
		a.lastClipboardText = text
		_, added, err := a.Clipboard.AddText(text)
		return err == nil && added
	}

	formats := mygo.Clipboard.Formats()
	key := formatKey(formats)
	if key == a.lastClipboardFormats {
		return false
	}
	a.lastClipboardFormats = key

	if containsFormat(formats, "image/png") {
		if data, err := mygo.Clipboard.Read(transfer.PNG); err == nil {
			if png, err := data.Read(transfer.PNG); err == nil && len(png) > 0 {
				width, height := imageSize(png)
				_, added, err := a.Clipboard.AddImage(png, width, height)
				return err == nil && added
			}
		}
	}
	if paths, err := mygo.Clipboard.ReadFiles(); err == nil && len(paths) > 0 {
		_, added, err := a.Clipboard.AddFiles(paths)
		return err == nil && added
	}
	return false
}

// formatKey is a comparable summary of a format list.
func formatKey(formats []transfer.Format) string {
	names := make([]string, 0, len(formats))
	for _, format := range formats {
		names = append(names, string(format))
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func containsFormat(formats []transfer.Format, name string) bool {
	for _, format := range formats {
		if string(format) == name {
			return true
		}
	}
	return false
}

// imageSize reads a PNG's dimensions, zero when it cannot be decoded.
func imageSize(png []byte) (int, int) {
	config, err := stdpng.DecodeConfig(bytes.NewReader(png))
	if err != nil {
		return 0, 0
	}
	return config.Width, config.Height
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

// permissionDialog asks the user to approve an install's permissions. It is
// a value so a test can answer without a native dialog, and a goroutine
// caller (the dialog blocks).
func permissionDialog(approval extensions.PermissionApproval, copy i18n.Settings, language string) bool {
	var lines []string
	for _, permission := range approval.Added {
		line := "\u2022 " + i18n.PermissionLabel(language, permission)
		if extensions.PermissionEnforced(permission) {
			line += " \u2014 " + copy.PermissionsEnforced
		} else {
			line += " \u2014 " + copy.PermissionsDeclared
		}
		lines = append(lines, line)
	}
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageQuestion,
		Message: copy.PermissionsTitle(approval.Name),
		Detail:  strings.Join(lines, "\n"),
		Buttons: []string{copy.PermissionsAllow, copy.PermissionsCancel},
	})
	return err == nil && result.Button == 0
}

// menuActions are what the application menu's own items do.
type menuActions struct {
	Settings func()
	Terminal func()
	Launcher func()
}

// menuTemplate is the application menu: the standard macOS app menu, the
// Edit menu whose roles every text field and the terminal need, a small View
// menu for the app's own surfaces, and the window list.
//
// The template is a plain value so a test can check its shape on any
// platform; the roles mygo filters (the macOS-only ones) are left out by the
// platform, not here.
func menuTemplate(copy i18n.Launcher, actions menuActions) []*mygo.MenuItem {
	settings := &mygo.MenuItem{Label: copy.CommandSettings, Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { actions.Settings() }}
	terminal := &mygo.MenuItem{Label: copy.CommandTerminal, Accelerator: "CmdOrCtrl+Shift+T", Click: func(*mygo.MenuItem, *mygo.Window) { actions.Terminal() }}
	launcher := &mygo.MenuItem{Label: copy.MenuLauncher, Click: func(*mygo.MenuItem, *mygo.Window) { actions.Launcher() }}

	return []*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			settings,
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Label: copy.MenuView, Submenu: []*mygo.MenuItem{
			launcher,
			terminal,
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleWindowMenu},
	}
}

// InstallMenu sets the application menu, so the app has the standard menus
// (the Edit menu's roles are what a text field and the terminal expect) and
// its own ways back to each surface.
func (a *App) InstallMenu() {
	copy := i18n.For(a.Store.Snapshot().Language).Launcher
	mygo.App.SetMenu(mygo.NewMenu(menuTemplate(copy, menuActions{
		Settings: func() { a.Open(SurfaceSettings) },
		Terminal: func() { a.Open(SurfaceTerminal) },
		Launcher: func() { a.Open(SurfaceLauncher) },
	})))
}
