package shell

import (
	"context"
	"log"
	"time"

	"github.com/egoist/mygo"

	"floter/internal/i18n"
	"floter/internal/settingsui"
)

// The app's update check. The build carries its feed and public key (mygo
// build writes them from mygo.json's `updates`); a build without them says so
// rather than pretending to look. The check and the download both run off the
// main thread, and every answer lands on it.
//
// The state machine is the old build's (`App.tsx`'s updateInfo /
// updateDownloading / updateProgress / updateFailed): the page is told what
// happened, not a sentence it has to parse. The check runs once at startup the
// way the old build's mount effect did, so the About page is already answered
// by the time the user opens it.

// checkForUpdates asks the feed for a newer version and records the outcome.
func (a *App) checkForUpdates() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		found, err := mygo.Updater.Check(ctx)
		switch {
		case err == mygo.ErrUpdatesDisabled:
			a.setUpdate(func(u *settingsui.UpdateState) {
				*u = settingsui.UpdateState{Checked: true, Disabled: true}
			})
		case err != nil:
			log.Printf("floter: could not check for updates: %v", err)
			a.setUpdate(func(u *settingsui.UpdateState) {
				*u = settingsui.UpdateState{Checked: true, Failed: true}
			})
		case found == nil:
			a.setUpdate(func(u *settingsui.UpdateState) {
				*u = settingsui.UpdateState{Checked: true}
			})
		default:
			a.setUpdate(func(u *settingsui.UpdateState) {
				*u = settingsui.UpdateState{Checked: true, Available: true, Version: found.Version}
			})
		}
	}()
}

// installUpdate downloads and installs the update the About row's Download or
// Retry control asked for. It re-checks first, as the old build's
// `downloadAndInstallUpdate` did: the feed may have moved since startup, and
// the Retry state has no pending update to install. The running app is not
// touched until the install succeeds; then the app relaunches, which is what
// the old build did (`await relaunch()`).
func (a *App) installUpdate() {
	a.setUpdate(func(u *settingsui.UpdateState) {
		u.Checked, u.Failed = true, false
		u.Downloading, u.Progress = true, nil
	})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		found, err := mygo.Updater.Check(ctx)
		switch {
		case err == mygo.ErrUpdatesDisabled:
			a.setUpdate(func(u *settingsui.UpdateState) {
				*u = settingsui.UpdateState{Checked: true, Disabled: true}
			})
			return
		case err != nil:
			a.failUpdate("could not check for updates: %v", err)
			return
		case found == nil:
			a.setUpdate(func(u *settingsui.UpdateState) {
				*u = settingsui.UpdateState{Checked: true}
			})
			return
		}
		a.setUpdate(func(u *settingsui.UpdateState) {
			u.Checked, u.Available, u.Version = true, true, found.Version
			u.Downloading, u.Failed, u.Progress = true, false, nil
		})
		err = found.Install(ctx, func(downloaded, total int64) {
			a.setUpdate(func(u *settingsui.UpdateState) {
				u.Progress = &settingsui.UpdateProgress{Downloaded: downloaded, Total: total}
			})
		})
		if err != nil {
			a.failUpdate("could not install the update: %v", err)
			return
		}
		a.setUpdate(func(u *settingsui.UpdateState) {
			u.Downloading, u.Progress, u.Available = false, nil, false
		})
		a.relaunchApp()
	}()
}

// failUpdate records a failed check or install and logs the cause.
func (a *App) failUpdate(format string, err error) {
	log.Printf("floter: "+format, err)
	a.setUpdate(func(u *settingsui.UpdateState) {
		u.Checked, u.Downloading, u.Progress = true, false, nil
		u.Failed = true
	})
}

// setUpdate mutates the About page's update state on the main thread and asks
// for a frame, so a background check and download land where the page can see
// them.
func (a *App) setUpdate(mutate func(*settingsui.UpdateState)) {
	a.onMain(func() {
		if a.Settings == nil {
			return
		}
		mutate(&a.Settings.Update)
		if a.Win != nil {
			a.Win.Invalidate()
		}
	})
}

// relaunchApp starts the freshly installed version, which is the old build's
// `await relaunch()` after a successful install. It is a field so a test can
// observe the moment without replacing the running binary.
func (a *App) relaunchApp() {
	if a.relaunch == nil {
		mygo.App.Relaunch()
		return
	}
	a.relaunch()
}

// SettingsCopy is the settings surface's copy in the stored language.
func (a *App) SettingsCopy() i18n.Settings {
	return i18n.For(a.Store.Snapshot().Language).Settings
}
