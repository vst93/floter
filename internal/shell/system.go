package shell

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	stdpng "image/png"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"

	"floter/internal/apps"
	"floter/internal/clipboard"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/settingsui"
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

// ApplyMenubarIcon installs or removes the tray icon, following the
// show_menubar_icon setting, so a change made in the settings file or the
// settings surface takes effect at once.
func (a *App) ApplyMenubarIcon() {
	if a.Win == nil {
		// Before the window exists the app is not ready to hold a tray icon;
		// Start applies the setting once it is.
		return
	}
	want := settings.ShowMenubarIcon(a.Store.Snapshot())
	switch {
	case want && a.Tray == nil:
		a.InstallTray()
	case !want && a.Tray != nil:
		a.Tray.Destroy()
		a.Tray = nil
	}
}

// trayMenu is the tray icon's menu, as a plain value so a test can check its
// shape on any platform (a real tray needs a window and a desktop session).
//
// It is also the app's only settings and quit door outside macOS: the
// launcher's built-in rows are the reference's six (`launcher.commands`), which
// carry neither, so a build without a tray keeps quit on the macOS app menu
// (`RoleQuit`) and nowhere else — exactly as the reference did
// (`main:src/hooks/useLauncherCatalog.ts:184-292`).
func trayMenu(copy i18n.Launcher, show, settings, reload, quit func()) *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: copy.TrayShow, Click: func(*mygo.MenuItem, *mygo.Window) { show() }},
		{Label: copy.TraySettings, Click: func(*mygo.MenuItem, *mygo.Window) { settings() }},
		{Label: copy.TrayReload, Click: func(*mygo.MenuItem, *mygo.Window) { reload() }},
		mygo.Separator(),
		{Label: copy.TrayQuit, Click: func(*mygo.MenuItem, *mygo.Window) { quit() }},
	})
}

// trayID names floter's tray icon across launches (mygo's TrayOptions.ID,
// v0.6.0). It is a **published contract**: the two platforms that use it keep
// the user's own placement of the icon under this key — macOS remembers where
// the icon was dragged in the menu bar (⌘-drag), and Windows remembers whether
// the user kept it on the taskbar or in the overflow — so changing the string
// is the same as dropping the settings the user chose. The naming surface is
// the one `scheme` and the app id already use: the plain application name.
// Linux's panels place their icons themselves and ignore it.
const trayID = "floter"

// trayOptions is the tray mygo is asked for, apart from the menu and the icon
// data so that the identity it carries can be asserted without a system tray
// (see TestTrayKeepsItsIdentity).
//
// IconIsTemplate stays false: it tells macOS to tint a black-and-transparent
// icon to the menu bar's colour (and Windows to show it white on a dark
// taskbar), which is right for a glyph and wrong for floter's mark — both
// embedded icons are the coloured logo, not a mask (the dark icon carries 951
// of its 1008 opaque pixels in colour), so a tint would flatten the mark to a
// silhouette. The appearance the user picks in settings is the app's own
// answer to the taskbar's brightness.
func trayOptions(icon []byte, menu *mygo.Menu) mygo.TrayOptions {
	return mygo.TrayOptions{
		ID:      trayID,
		Icon:    icon,
		ToolTip: trayID,
		Menu:    menu,
	}
}

// InstallTray adds the menu bar icon: show the panel, open the settings,
// re-scan what is installed, and quit — the four things a tray icon is for.
// The labels follow the stored language, so the tray never lags behind the UI.
func (a *App) InstallTray() {
	copy := i18n.For(a.Store.Snapshot().Language).Launcher
	menu := trayMenu(copy,
		a.Toggle,
		func() { a.Open(SurfaceSettings) },
		a.Rescan,
		a.quit,
	)

	tray, err := mygo.NewTray(trayOptions(appIconBytes(a.appIcon), menu))
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
	if err != nil {
		// A malformed URL is a user-visible dead end, not noise: bring the
		// panel forward and say so — once per window, since an external
		// trigger can repeat on its own (the old build's MalformedUrl). The
		// refusal is about the link, not a reason to abandon the page the
		// user was reading: `refuseLink` reveals without switching surfaces.
		a.refuseLink()
		return
	}
	if !strings.EqualFold(parsed.Scheme, scheme) {
		// A scheme this app does not handle is not ours to complain about
		// (the old build kept UnsupportedScheme silent), but a foreign link
		// is still a reason to come forward — without changing the surface.
		a.reveal()
		return
	}
	action := strings.ToLower(parsed.Host)
	// The query has to be the shape the action's table describes before any
	// branch reads a key out of it: a key the reference does not name is a
	// refusal there (`Reject::UnknownParameter`, `deep_link.rs:491`), not a
	// value to skip over (`linkparams.go`). Like the name rule below, the
	// refusal comes before the page opens.
	if !linkQueryFits(action, parsed.Query()) {
		a.refuseLink()
		return
	}
	switch action {
	case "settings":
		a.Open(SurfaceSettings)
	case "terminal":
		a.Open(SurfaceTerminal)
	case "open":
		// The reference's `open` does nothing but route: focus or summon the
		// main window (`deep_link.rs:12`). It takes **no** parameters;
		// any query key triggers `UnknownParameter` (`parse_url`, `:492`).
		// This build's `search?q=…` feature would be broken by applying that
		// rule, so the divergence is pinned rather than applied (see P10-l
		// spec). The handler here is a no-op stub: bring the window forward
		// without switching surface or prefilling.
		a.Open(SurfaceLauncher)
	case "connect":
		// A tool ships these links in its README: `connect` carries a manifest
		// that is reviewed here, and connecting is still the user's own
		// approval. The table above already refused every key but `manifest`;
		// the value has its own gate (`linkManifestValue`, linkmanifest.go),
		// and a value it refuses — an empty one, a control character, a URL, a
		// `..` traversal — is refused before the page is touched (the old
		// build's MissingManifest, ManifestInsecure and ManifestTraversal).
		manifest, ok := linkManifestValue(parsed.Query().Get("manifest"))
		if !ok {
			a.refuseLink()
			return
		}
		a.Open(SurfaceSettings)
		a.Settings.Page = settingsui.PageIntegrations
		a.connectFromLink(manifest)
	case "register":
		// The register form names the program: `cmd` is the reference's only
		// spelling (`deep_link.rs:472`); `tool`, `program` and a positional
		// path are this build's aliases, which the reference refuses
		// (`UnknownParameter` / `UnknownParameter` / `MissingCommand`). They
		// are pinned, not accidental (`registername_test.go`). A name is what
		// the link is for; one with none is refused like a connect with no
		// manifest — and so is a value that is not a bare command name (a
		// path, a traversal, a shell string), which is the reference's
		// `validate_command` (`bareCommandName`, linkname.go). Both refusals
		// come before the page opens: a refused link is a sentence, not a trip
		// to the integrations page. The table above already refused every key
		// but the three name spellings and the two it accepts without a reader
		// (`linkparams.go`).
		name, ok := bareCommandName(firstNonEmpty(parsed.Query().Get("cmd"),
			parsed.Query().Get("tool"), parsed.Query().Get("program"),
			strings.TrimPrefix(parsed.Path, "/")))
		if !ok {
			a.refuseLink()
			return
		}
		// The optional `args` hint has no reader here, but it still has a
		// shape: the reference refuses a token that is not shell-inert, and a
		// link it refuses must not be accepted here (`linkArgsFit`,
		// linkargshint.go). Same order as the reference's `parse_url`, which
		// validates the name first (`deep_link.rs:496-501`), and the same
		// place: before the page opens.
		if !linkArgsFit(parsed.Query().Get("args")) {
			a.refuseLink()
			return
		}
		a.Open(SurfaceSettings)
		a.Settings.Page = settingsui.PageIntegrations
		a.registerFromLink(name)
	default:
		a.Open(SurfaceLauncher)
		// The prefill belongs to the two spellings this build documents
		// (`floter://search?q=…` and `floter://?q=…`, `mygo-rewrite-plan.md:226`),
		// not to "any host the switch did not match": a link that names
		// nothing this router knows opens the launcher and leaves the search
		// box alone, which is also what the reference's silent
		// `UnknownAction` amounts to (`deep_link.rs:223-228`).
		if query := launcherPrefill(parsed); query != "" {
			a.Launcher.SetQuery(query)
		}
	}
}

// registerHighlightWindow is how long the `floter://register` mark lasts: the
// reference's `useTimedReset(registerTarget, …)` default is 3000ms
// (`src/hooks/useTimedReset.ts:3`), kept a little longer here because the row
// it marks appears only after the connect lands, which runs off this thread.
const registerHighlightWindow = 5 * time.Second

// lookTool is the register link's name probe, a variable so a test can resolve
// against a fixture instead of depending on the machine having the package's
// tool installed. It is the same seam `internal/extensions` keeps for its own
// lookups (`runtimepath.go:70-76`).
var lookTool = extensions.LookTool

// registerFromLink handles the program a `floter://register` link named: the
// name is resolved on the search path, and that is the only way in — the router
// has already refused anything that is not a bare command name
// (`bareCommandName`, linkname.go), so this function never sees a path. The
// reference's register link does **not** connect anything by itself —
// it marks the row the link named so the user still makes the connect
// decision (`src/ExtensionsPanel.tsx:1500-1548`). This build connects through
// the same one-press flow the Detected row uses, and marks the card that
// results; a link that names an already-connected program, or one nothing can
// resolve, leaves the list alone and says which it was instead of connecting
// a second copy or half a package.
func (a *App) registerFromLink(name string) {
	path, ok := lookTool(name)
	if !ok {
		a.registerNotice(func(c i18n.Settings) string { return c.RegisterNotFound(name) })
		return
	}
	// A tool that is already connected has no unconnected card to mark, and
	// saying "not found" would be false, so the two outcomes get two
	// sentences (the reference's `registerMiss.alreadyConnected`).
	if a.hasExecutable(path) {
		a.registerNotice(func(c i18n.Settings) string { return c.RegisterAlreadyConnected(name) })
		return
	}
	a.markRegisterTarget(path)
	a.connectDetected(settingsui.DetectedTool{
		Name:        filepath.Base(path),
		Path:        path,
		VersionArgs: "--version",
	})
}

// registerNotice raises the link's own sentence through the app's notice
// stack, which is where every other externally-triggered outcome lands.
func (a *App) registerNotice(word func(i18n.Settings) string) {
	message := word(i18n.For(a.Store.Snapshot().Language).Settings)
	a.onMain(func() { a.queueNotice(message, refusalNotice) })
}

// markRegisterTarget records the path a register link named and arms the beat
// that clears it, so the mark is the link's moment rather than a mode. A
// second link replaces the first mark and restarts the clock.
func (a *App) markRegisterTarget(path string) {
	a.registerTarget = path
	if a.registerTimer != nil {
		a.registerTimer.Stop()
	}
	a.registerTimer = time.AfterFunc(a.registerHighlight, func() {
		a.onMain(func() {
			a.registerTarget = ""
			if a.Win != nil {
				a.Win.Invalidate()
			}
		})
	})
	if a.Win != nil {
		a.Win.Invalidate()
	}
}

// hasExecutable reports whether an installed integration already owns this
// executable, resolving symlinks on both sides so `/usr/local/bin/rg` and the
// real path it points at are the same program.
func (a *App) hasExecutable(path string) bool {
	for _, integration := range a.Integrations.Inventory().Integrations {
		if sameExecutable(integration.Entry.ExecutablePath, path) {
			return true
		}
	}
	return false
}

// sameExecutable compares two executable paths, treating a symlinked pair as
// one program (the inventory records the resolved path; the search path may
// hand back the link).
func sameExecutable(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	leftResolved, leftErr := filepath.EvalSymlinks(left)
	rightResolved, rightErr := filepath.EvalSymlinks(right)
	return leftErr == nil && rightErr == nil && leftResolved == rightResolved
}

// firstNonEmpty is the first non-blank of the values.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// browserOptions is the browser plugin's settings, as the search uses them.
type browserOptions struct {
	enabled       bool
	target        string
	customBaseDir string
	historyDays   int
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
		sortOrder:     plugin.SortOrder,
		searchField:   plugin.SearchField,
		cdpEnabled:    plugin.CDPEnabled,
		cdpPort:       plugin.CDPPort,
	}
}

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
// new: a file list, an image (with the text that rode along as its caption), or
// text. It reports whether the history grew.
//
// The order is the reference's (`clipboard_history/monitor.rs:107-186`) and it
// is load-bearing: a Finder or Explorer selection ALSO carries a plain text
// rendition of its paths, so text read first would record the paths as one
// string and lose the file semantics. Files are read first, and when no file
// list survives PrepareFiles the same copy falls through to its image and text
// renditions.
//
// Identity is the content, never the clipboard's format list: two different
// screenshots carry the same `image/png` format list, and comparing lists would
// drop every image after the first. `lastClipboardHash` is what the watcher last
// recorded, so a repeated poll of unchanged content costs nothing.
func (a *App) captureClipboard() bool {
	sample := clipboardSample()
	if capture, ok := clipboard.PrepareFiles(sample.Paths, clipboardFileExists); ok {
		if capture.Hash == a.lastClipboardHash {
			return false
		}
		a.lastClipboardHash = capture.Hash
		_, added, err := a.Clipboard.AddFilesCapture(capture)
		if err != nil {
			log.Printf("floter: could not record the copied files (%s): %v", formatKey(sample.formats), err)
		}
		return err == nil && added
	}

	sample.readImage()
	plan, ok := planCapture(sample)
	if !ok || plan.Hash == a.lastClipboardHash {
		return false
	}
	// The watcher must not record what the app itself wrote.
	if plan.Kind == clipboard.KindText && plan.Text == a.selfCopied {
		return false
	}
	a.lastClipboardHash = plan.Hash
	var added bool
	var err error
	if plan.Kind == clipboard.KindImage {
		_, added, err = a.Clipboard.AddImageCaption(plan.PNG, plan.Width, plan.Height, plan.Caption)
	} else {
		_, added, err = a.Clipboard.AddText(plan.Text)
	}
	if err != nil {
		log.Printf("floter: could not record the clipboard (%s): %v", formatKey(sample.formats), err)
	}
	return err == nil && added
}

// clipSample is one reading of the system clipboard. The image's bytes are not
// part of the reading: they are the expensive part, and they only matter once
// no file list won.
type clipSample struct {
	Paths   []string
	Text    string
	PNG     []byte
	Width   int
	Height  int
	formats []transfer.Format
}

// readClipboardSample reads the cheap half of the clipboard: the file list, the
// text and the format list's summary (which the failure log names).
func readClipboardSample() clipSample {
	sample := clipSample{Text: mygo.Clipboard.ReadText()}
	if paths, err := mygo.Clipboard.ReadFiles(); err == nil {
		sample.Paths = paths
	}
	sample.formats = mygo.Clipboard.Formats()
	return sample
}

// readImage fills in the image half of the sample.
func (s *clipSample) readImage() {
	s.PNG, s.Width, s.Height = clipboardImageReader(s.formats)
}

// clipboardImage reads the clipboard's PNG rendition, and only when the format
// list says there is one: fetching a PNG on every poll of a text clipboard would
// be waste. The reference reads files first and returns before touching pixels
// for the same reason.
func clipboardImage(formats []transfer.Format) ([]byte, int, int) {
	if !containsFormat(formats, "image/png") {
		return nil, 0, 0
	}
	data, err := mygo.Clipboard.Read(transfer.PNG)
	if err != nil {
		return nil, 0, 0
	}
	png, err := data.Read(transfer.PNG)
	if err != nil || len(png) == 0 {
		return nil, 0, 0
	}
	width, height := imageSize(png)
	return png, width, height
}

// clipboardImageReader is the image reader; tests replace it alongside
// clipboardSample so a poll can be observed with no system clipboard in the way.
var clipboardImageReader = clipboardImage

// capturePlan is what one sample records: one kind, the payload, and the
// identity the store will dedupe on.
type capturePlan struct {
	Kind    string
	Text    string
	Caption string
	PNG     []byte
	Width   int
	Height  int
	Hash    string
}

// planCapture decides what a sample with no usable file list records, in the
// reference's order: the image first, with the copy's text as its caption, then
// the text alone.
//
// Text and image are decided together because a single copy can carry both: an
// editor handing over pixels plus their caption must store ONE image entry
// whose text is that caption — never two entries, and never the text alone
// shadowing the image. A capture over the size ceilings records nothing at all,
// the text included, because half of an oversized copy is worse than none.
func planCapture(sample clipSample) (capturePlan, bool) {
	if len(sample.PNG) > 0 && sample.Width > 0 && sample.Height > 0 {
		if len(sample.PNG) > clipboard.MaxImagePNGBytes {
			return capturePlan{}, false
		}
		caption := clipboard.CaptionFor(sample.Text)
		return capturePlan{
			Kind:    clipboard.KindImage,
			Caption: caption,
			PNG:     sample.PNG,
			Width:   sample.Width,
			Height:  sample.Height,
			Hash:    clipboard.ImageHash(sample.PNG, caption),
		}, true
	}
	if len(sample.Text) > clipboard.MaxTextBytes || strings.TrimSpace(sample.Text) == "" {
		return capturePlan{}, false
	}
	return capturePlan{
		Kind: clipboard.KindText,
		Text: sample.Text,
		Hash: clipboard.HashText(sample.Text),
	}, true
}

// clipboardSample reads the system clipboard once per poll; tests replace it so
// the watcher's decision can be observed with no system clipboard in the way.
var clipboardSample = readClipboardSample

// clipboardFileExists is the file list's "is this path still there" oracle,
// the only filesystem access the decision needs.
var clipboardFileExists = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageQuestion,
		Message: copy.PermissionsTitle(approval.Name),
		Detail:  strings.Join(permissionLines(approval.Added, copy, language), "\n"),
		Buttons: []string{copy.PermissionsAllow, copy.PermissionsCancel},
	})
	return err == nil && result.Button == 0
}

// permissionLines renders one permission per line, each with the sentence that
// says whether Floter enforces it or only declares it — the two lists the
// native dialogs share (the review and the plain approval).
func permissionLines(permissions []string, copy i18n.Settings, language string) []string {
	lines := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		line := "\u2022 " + i18n.PermissionLabel(language, permission)
		if extensions.PermissionEnforced(permission) {
			line += " \u2014 " + copy.PermissionsEnforced
		} else {
			line += " \u2014 " + copy.PermissionsDeclared
		}
		lines = append(lines, line)
	}
	return lines
}

// localInstallReview is the local package review: the manifest summary the
// reference's LocalInstallDialog shows (source / runtime / supported
// platforms), the declared permissions, and the boundary note, before the
// folder install commits. It always appears — the reference reviews a local
// package whether or not it declares anything (ExtensionsPanel.tsx:1430-1440,
// LocalInstallDialog.tsx).
func localInstallReview(prepared extensions.Prepared, copy i18n.Settings, language string) bool {
	title, detail := localInstallReviewText(prepared, copy, language)
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageQuestion,
		Message: title,
		Detail:  detail,
		Buttons: []string{copy.InstallReviewConfirm, copy.PermissionsCancel},
	})
	return err == nil && result.Button == 0
}

// localInstallReviewText builds the review's title and body: the manifest
// summary, the declared permissions with their enforcement sentence, and the
// boundary note. It is a value so a test can read the sentences without a
// native dialog.
func localInstallReviewText(prepared extensions.Prepared, copy i18n.Settings, language string) (string, string) {
	manifest := prepared.Manifest
	platforms := strings.Join(manifest.Platforms, ", ")
	if platforms == "" {
		platforms = copy.InstallReviewUnavailable
	}
	lines := []string{
		copy.InstallReviewHint,
		"",
		copy.InstallReviewSource + ": " + copy.InstallReviewLocalDirectory,
		copy.InstallReviewRuntime + ": " + manifest.Runtime.Type,
		copy.InstallReviewPlatforms + ": " + platforms,
	}
	if declared := prepared.Approval.Declared; len(declared) > 0 {
		lines = append(lines, "")
		lines = append(lines, permissionLines(declared, copy, language)...)
		lines = append(lines, "", copy.InstallReviewBoundary)
	}
	return copy.InstallReviewTitle(manifest.Name), strings.Join(lines, "\n")
}

// recommendedInstallReview is the recommended-tool connect review: the
// reference's permission review dialog (ExtensionsPanel.tsx:2594-2620). The
// connect flow asks `extensions_recommended_permissions` for the manifest's
// declared permissions and, only when there are any, shows this before
// anything is written (`:1349-1400`). It names the tool, shows the program it
// would run, splits the permissions into the host-enforced and disclosed
// tiers, and ends with the boundary note. It is a value so a test can read
// the sentences without a native dialog.
func recommendedInstallReview(prepared extensions.Prepared, copy i18n.Settings, language string) bool {
	title, detail := recommendedInstallReviewText(prepared, copy, language)
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageQuestion,
		Message: title,
		Detail:  detail,
		Buttons: []string{copy.PermissionsConnect, copy.PermissionsCancel},
	})
	return err == nil && result.Button == 0
}

// recommendedInstallReviewText builds the connect review's title and body:
// the program the tool would run (when the connect flow already resolved
// one), the permissions-required label, the two permission tiers, and the
// boundary note. The title is the reference's `confirmConnectRecommended`
// ("Connect the recommended tool {name} to Floter?").
func recommendedInstallReviewText(prepared extensions.Prepared, copy i18n.Settings, language string) (string, string) {
	lines := make([]string, 0, 8)
	if path := strings.TrimSpace(prepared.ExecutablePath); path != "" {
		lines = append(lines, path, "")
	}
	lines = append(lines, copy.PermissionsRequired)
	lines = append(lines, permissionTierLines(prepared.Approval.Declared, copy, language)...)
	lines = append(lines, "", copy.InstallReviewBoundary)
	return copy.ConnectRecommendedTitle(prepared.Manifest.Name), strings.Join(lines, "\n")
}

// permissionTierLines renders the declared permissions as the two tiers the
// reference's PermissionTierList draws: the host's own decision first, with
// its label and sentence, then the declared-only tier. Each permission is its
// name and what it allows. The split is the message — a user must be able to
// tell a check from an FYI — so the tiers are named, not merely ordered.
func permissionTierLines(permissions []string, copy i18n.Settings, language string) []string {
	var enforced, disclosure []string
	for _, permission := range permissions {
		if extensions.PermissionEnforced(permission) {
			enforced = append(enforced, permission)
			continue
		}
		disclosure = append(disclosure, permission)
	}
	lines := make([]string, 0, len(permissions)+4)
	appendTier := func(label, hint string, tier []string) {
		if len(tier) == 0 {
			return
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, label, hint)
		for _, permission := range tier {
			line := "\u2022 " + i18n.PermissionLabel(language, permission)
			if description := i18n.PermissionDescription(language, permission); description != "" {
				line += " \u2014 " + description
			}
			lines = append(lines, line)
		}
	}
	appendTier(copy.PermissionsEnforcedLabel, copy.PermissionsEnforcedHint, enforced)
	appendTier(copy.PermissionsDisclosureLabel, copy.PermissionsDisclosureHint, disclosure)
	return lines
}

// chooseSystemTool is the reference's tool-selection dialog: when a system
// runtime's declared names resolve to more than one program, the user picks
// the one the integration binds to before anything is connected or reconnected
// (`src/ExtensionsPanel.tsx:2623-2648`, `chooseSystemTool`). It is a value so a
// test can answer without a native dialog, and a goroutine caller (the dialog
// blocks).
func (a *App) chooseSystemTool(name string, candidates []extensions.ExecutableCandidate) (string, bool) {
	if a.chooseTool != nil {
		return a.chooseTool(name, candidates)
	}
	return chooseSystemToolDialog(name, candidates, i18n.For(a.Store.Snapshot().Language).Settings)
}

// chooseSystemToolDialog draws the chooser natively: one button per candidate,
// labeled with its name and path, and a Cancel button last. The detail repeats
// each candidate with the sources it was found through, so a path alone is not
// the only thing a person has to go on.
func chooseSystemToolDialog(name string, candidates []extensions.ExecutableCandidate, copy i18n.Settings) (string, bool) {
	buttons := make([]string, 0, len(candidates)+1)
	for _, candidate := range candidates {
		buttons = append(buttons, candidate.Name+" \u2014 "+candidate.Path)
	}
	buttons = append(buttons, copy.PermissionsCancel)
	_, detail := chooseSystemToolText(name, candidates, copy)
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageQuestion,
		Message: copy.ChooseSystemToolTitle(name),
		Detail:  detail,
		Buttons: buttons,
	})
	if err != nil || result.Button < 0 || result.Button >= len(candidates) {
		return "", false
	}
	return candidates[result.Button].Path, true
}

// chooseSystemToolText builds the chooser's title and body: each candidate's
// name and path, with the sources it was discovered through — the reference's
// `chooseSystemTool` title, `chooseSystemToolHint` and the per-candidate
// name/path/sources rows.
func chooseSystemToolText(name string, candidates []extensions.ExecutableCandidate, copy i18n.Settings) (string, string) {
	lines := make([]string, 0, len(candidates)*2+2)
	lines = append(lines, copy.ChooseSystemToolHint, "")
	for _, candidate := range candidates {
		line := "\u2022 " + candidate.Name + " \u2014 " + candidate.Path
		if len(candidate.Sources) > 0 {
			line += " (" + strings.Join(candidate.Sources, " \u00b7 ") + ")"
		}
		lines = append(lines, line)
	}
	return copy.ChooseSystemToolTitle(name), strings.Join(lines, "\n")
}

// menuActions are what the application menu's own items do.
type menuActions struct {
	Settings func()
	Terminal func()
	Launcher func()
	// SettingsAccelerator is the spelling the Settings… item advertises and
	// answers. It is the user's stored `open_settings` binding, so rebinding
	// the action moves the menu with it instead of leaving a second, frozen
	// Cmd+, behind. Empty falls back to the shipped accelerator.
	SettingsAccelerator string
}

// menuBarBelongs reports whether an application menu bar belongs on this
// platform: macOS draws it in the system menu bar, and Linux and Windows draw
// it inside the window, where a floating panel wants none.
func menuBarBelongs(goos string) bool { return goos == "darwin" }

// menuTemplate is the application menu: the standard macOS app menu, the
// Edit menu whose roles every text field and the terminal need, a small View
// menu for the app's own surfaces, and the window list.
//
// The template is a plain value so a test can check its shape on any
// platform; the roles mygo filters (the macOS-only ones) are left out by the
// platform, not here.
func menuTemplate(copy i18n.Launcher, actions menuActions) []*mygo.MenuItem {
	accelerator := actions.SettingsAccelerator
	if accelerator == "" {
		accelerator = "CmdOrCtrl+,"
	}
	settings := &mygo.MenuItem{Label: copy.CommandSettings, Accelerator: accelerator, Click: func(*mygo.MenuItem, *mygo.Window) { actions.Settings() }}
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

// InstallMenu sets the application menu where a menu bar belongs: macOS draws
// it in the system menu bar, which is where a Mac app's menus live. On Linux
// and Windows the framework's menu bar is drawn *inside* the window — a bar of
// File/Edit/View across the top of a floating panel, which is not what this app
// is — so the menu is removed there and the tray is the menu, exactly as the
// old build had it (it installed no application menu at all).
func (a *App) InstallMenu() {
	if !menuBarBelongs(runtime.GOOS) {
		// Not merely skipping our own menu: without a call to SetMenu the
		// framework installs a default one, so the bar has to be taken away
		// on purpose.
		mygo.App.SetMenu(nil)
		return
	}
	copy := i18n.For(a.Store.Snapshot().Language).Launcher
	snapshot := a.Store.Snapshot()
	mygo.App.SetMenu(mygo.NewMenu(menuTemplate(copy, menuActions{
		Settings:            func() { a.Open(SurfaceSettings) },
		Terminal:            func() { a.Open(SurfaceTerminal) },
		Launcher:            func() { a.Open(SurfaceLauncher) },
		SettingsAccelerator: settings.Shortcut(snapshot, settings.ShortcutOpenSettings),
	})))
}

// Rescan re-reads what is installed: the integrations' providers, the PATH
// commands, the install catalog and the applications. It is the tray's own
// "I changed something on this machine" press, and it is safe from the tray's
// goroutine.
//
// One rescan at a time (see scanFlight). The three scans it starts are latched
// individually, but the provider refresh is synchronous and can take seconds on
// a machine with a slow tool, so a second press while the first is still
// reading is the same request and is dropped: the tray item is a menu entry the
// user can hold down, and every repeat used to start another provider probe on
// the tray's goroutine.
func (a *App) Rescan() {
	if !a.rescan.begin() {
		return
	}
	defer a.rescan.end()
	// The user is saying a program was installed a moment ago, so the registry's
	// own 30-minute memo is the one thing that must not answer this scan (see
	// apps.ResetRegistryCache).
	apps.ResetRegistryCache()
	a.scanApps()
	a.scanTools()
	a.scanToolCatalog()
	// The Detected list is derived from the inventory this rescan just re-read
	// (see detectedtools.go), so its memo is dropped with the rest: the next
	// frame walks again instead of serving the pre-rescan answer.
	invalidateDetectedTools()
	a.RefreshIntegrations(context.Background())
}
