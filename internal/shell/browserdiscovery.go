package shell

import (
	"os"
	"time"

	"floter/internal/browser"
	"floter/internal/i18n"
)

type browserScan struct {
	profiles []browser.Profile
	finished time.Time
	done     chan struct{}
}

const browserScanLifetime = 30 * time.Second

// Request discovery without waiting for disk IO. Old results remain usable
// while a refresh is in flight. Only search workers wait for completion.
func (a *App) requestBrowserScan(customBase string, refresh bool) ([]browser.Profile, *browserScan) {
	a.browserMu.Lock()
	defer a.browserMu.Unlock()
	if a.browserScans == nil {
		a.browserScans = make(map[string]*browserScan)
	}
	scan := a.browserScans[customBase]
	if scan != nil && (scan.finished.IsZero() || (!refresh && time.Since(scan.finished) < browserScanLifetime)) {
		return scan.profiles, scan
	}
	previous := scan
	scan = &browserScan{done: make(chan struct{})}
	if previous != nil {
		scan.profiles = previous.profiles
	}
	// Search workers keep their scan handle even if the configured path changes.
	for key, old := range a.browserScans {
		if !old.finished.IsZero() {
			delete(a.browserScans, key)
		}
	}
	a.browserScans[customBase] = scan
	home := a.homeDir
	go func() {
		if home == "" {
			home, _ = os.UserHomeDir()
		}
		profiles := browser.ProfilesIn(home, customBase)
		a.browserMu.Lock()
		scan.profiles, scan.finished = profiles, time.Now()
		close(scan.done)
		a.browserMu.Unlock()
		a.invalidate()
	}()
	return scan.profiles, scan
}

func (a *App) browserProfileList(customBase string) []browser.Profile {
	_, scan := a.requestBrowserScan(customBase, false)
	<-scan.done
	a.browserMu.Lock()
	defer a.browserMu.Unlock()
	return scan.profiles
}

// Configuration reads a snapshot and never waits for a search's file scan.
func (a *App) browserTargets() []i18n.Option {
	profiles, _ := a.requestBrowserScan(browserState(a.settings()).customBaseDir, false)
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
