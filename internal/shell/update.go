package shell

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"floter/internal/i18n"
)

// The app's update check. The build carries its feed and public key (mygo
// build writes them from mygo.json's `updates`); a build without them says so
// rather than pretending to look. The check and the download both run off the
// main thread, and every answer lands on it.

// update is the update the last check found, waiting to be installed.
var (
	updateMu      sync.Mutex
	pendingUpdate *mygo.Update
)

// checkForUpdates asks the feed for a newer version.
func (a *App) checkForUpdates() {
	go func() {
		copy := a.SettingsCopy()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		found, err := mygo.Updater.Check(ctx)
		switch {
		case err != nil && err != mygo.ErrUpdatesDisabled:
			log.Printf("floter: could not check for updates: %v", err)
			a.setUpdateStatus(copy.UpdateFailed, false)
		case err == mygo.ErrUpdatesDisabled:
			a.setUpdateStatus(copy.UpdateDisabled, false)
		case found == nil:
			a.setUpdateStatus(copy.UpdateCurrent, false)
		default:
			updateMu.Lock()
			pendingUpdate = found
			updateMu.Unlock()
			a.setUpdateStatus(copy.UpdateAvailable(found.Version), true)
		}
	}()
}

// installUpdate downloads and installs the update the check found. The running
// app is untouched: the new version runs at the next launch.
func (a *App) installUpdate() {
	updateMu.Lock()
	found := pendingUpdate
	updateMu.Unlock()
	if found == nil {
		return
	}
	go func() {
		copy := a.SettingsCopy()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		err := found.Install(ctx, func(downloaded, total int64) {
			if total <= 0 {
				return
			}
			percent := int(downloaded * 100 / total)
			a.onMain(func() { a.Settings.UpdateStatus = copy.UpdateInstalling(percent) })
		})
		if err != nil {
			log.Printf("floter: could not install the update: %v", err)
			a.setUpdateStatus(copy.UpdateFailed, true)
			return
		}
		updateMu.Lock()
		pendingUpdate = nil
		updateMu.Unlock()
		a.setUpdateStatus(copy.UpdateInstalled, false)
	}()
}

// setUpdateStatus puts a line on the About page.
func (a *App) setUpdateStatus(status string, ready bool) {
	a.onMain(func() {
		if a.Settings == nil {
			return
		}
		a.Settings.UpdateStatus, a.Settings.UpdateReady = status, ready
		if a.Win != nil {
			a.Win.Invalidate()
		}
	})
}

// SettingsCopy is the settings surface's copy in the stored language.
func (a *App) SettingsCopy() i18n.Settings {
	return i18n.For(a.Store.Snapshot().Language).Settings
}
