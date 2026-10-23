// Command floter is the mygo rewrite's entry point: one frameless window
// whose body switches between the launcher, the settings panel and a
// terminal, following the user's existing settings file.
//
//	go run ./cmd/floter
package main

import (
	"log"
	"os"

	"github.com/egoist/mygo"

	"floter/internal/settings"
	"floter/internal/shell"
)

func main() {
	// One instance: a second launch hands its arguments (and any deep link
	// it carried) to the running app and exits.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	// Inherit an earlier build's settings and window placement on the first run
	// of this build. Both migrations are conservative: they never overwrite data
	// the user already has, and they leave a copy of whatever they read beside
	// the settings file.
	if report, err := settings.MigrateDefault(); err != nil {
		log.Printf("floter: settings migration failed: %v", err)
	} else if report.Migrated {
		log.Printf("floter: migrated settings from %s (backup %s, %d keys)", report.Source, report.Backup, len(report.Fields))
	}
	if report, err := settings.MigrateDefaultWindowState(); err != nil {
		log.Printf("floter: window-state migration failed: %v", err)
	} else if report.Migrated {
		log.Printf("floter: migrated window state from %s (backup %s, %d keys)", report.Source, report.Backup, len(report.Keys))
	}

	// Read the user's settings.json. A missing or unreadable file is not
	// fatal: the store falls back to the shipped defaults.
	path, pathErr := settings.Path()
	if pathErr != nil {
		log.Printf("floter: no settings directory: %v", pathErr)
	}
	store, err := settings.OpenStore(path)
	if err != nil {
		log.Printf("floter: could not read settings, using defaults: %v", err)
	}
	if path != "" {
		log.Printf("floter: settings %s", path)
	}

	// Window state (the detached output windows and the pinned text window)
	// lives in the settings file, through the app's own store, rather than in
	// mygo's separate window-state.json. The migration above seeds it the
	// first time from that file, so an in-place upgrade keeps the geometry the
	// user left. Set before the first window with a StateKey is created.
	mygo.App.SetWindowStateStore(settings.NewWindowStore(store))

	app := shell.New(shell.Options{Store: store, RefreshIntegrations: true})

	// The command line is the second spelling of the three actions the
	// floter:// links carry — `floter register rg` beside
	// `floter://register?cmd=rg` (docs/extensions/README.zh-CN.md) — and the
	// arguments of a launch reach nothing else: the framework delivers the URLs
	// among them itself (mygo's OnOpenURL), and these are the words.
	firstLink, hasFirstLink := shell.LinkFromArguments(os.Args[1:])

	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		app.Show()
		// A second launch carries its own words: `floter register rg` from a
		// terminal while the app is already up used to bring the window forward
		// and drop the name.
		if link, ok := shell.LinkFromArguments(args); ok {
			app.HandleURL(link)
		}
	})
	// A click on a completion notification brings the panel back. Registered
	// before Run so the click that launched the app (macOS keeps notifications
	// past the run that raised them) is received too; see NotificationClicked.
	mygo.App.OnNotificationClick(app.NotificationClicked)
	mygo.App.OnOpenURL(func(rawURL string) {
		log.Printf("floter: opening %s", rawURL)
		app.HandleURL(rawURL)
	})

	mygo.App.WhenReady(func() {
		app.Start()
		// After Start, which is where the scheme is registered; before
		// FLOTER_OPEN, which is a smoke test's override of the starting
		// surface rather than a request of its own.
		if hasFirstLink {
			app.HandleURL(firstLink)
		}
		// FLOTER_OPEN names the surface to start on, for a smoke test that
		// drives the packaged app: launcher (the default), settings or
		// terminal.
		if name := os.Getenv("FLOTER_OPEN"); name != "" {
			surface, ok := shell.ParseSurface(name)
			if !ok {
				log.Printf("floter: unknown FLOTER_OPEN %q", name)
			}
			if ok && surface != shell.SurfaceLauncher {
				app.Open(surface)
			}
		}
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
