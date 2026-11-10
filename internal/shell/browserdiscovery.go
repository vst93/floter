package shell

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"time"

	"floter/internal/browser"
	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
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
	return browserTargetOptions(profiles, i18n.For(a.settings().Language).Settings)
}

func browserTargetOptions(profiles []browser.Profile, copy i18n.Settings) []i18n.Option {
	seen := map[string]bool{}
	var options []i18n.Option
	for _, profile := range profiles {
		if profile.BrowserID == "" || seen[profile.BrowserID] {
			continue
		}
		seen[profile.BrowserID] = true
		label := profile.Browser
		if profile.ReadError != nil {
			label += " · " + copy.BrowserUnavailable
		}
		options = append(options, i18n.Option{ID: profile.BrowserID, Label: label})
	}
	return options
}

func (a *App) browserConfig(copy i18n.Settings, state settings.BrowserPlugin) (*plugincfg.Schema, plugincfg.Values) {
	profiles, _ := a.requestBrowserScan(state.CustomBaseDir, false)
	schema, values := plugincfg.Browser(copy, state, browserTargetOptions(profiles, copy))
	var names []string
	seen := map[string]bool{}
	permission := false
	for _, profile := range browser.SearchProfiles(profiles, state.Target) {
		if profile.ReadError == nil {
			continue
		}
		permission = permission || errors.Is(profile.ReadError, os.ErrPermission)
		if !seen[profile.BrowserID] {
			seen[profile.BrowserID] = true
			names = append(names, profile.Browser)
		}
	}
	if len(names) > 0 {
		help := copy.BrowserReadHint(strings.Join(names, ", "))
		if permission {
			help += " " + copy.BrowserAccessHint
			if runtime.GOOS == "darwin" {
				help = copy.BrowserReadHint(strings.Join(names, ", ")) + " " + copy.BrowserAccessMacHint
			}
		}
		for i := range schema.Fields {
			if schema.Fields[i].Key == "target" {
				schema.Fields[i].Help = copy.BrowserUnavailable
				schema.Fields[i].Note = help
			}
		}
	}
	return schema, values
}
