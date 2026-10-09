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

	app := shell.New(shell.Options{Store: store, RefreshIntegrations: true})

	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		app.Show()
	})
	mygo.App.OnOpenURL(func(rawURL string) {
		log.Printf("floter: opening %s", rawURL)
		app.HandleURL(rawURL)
	})

	mygo.App.WhenReady(func() {
		app.Start()
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
