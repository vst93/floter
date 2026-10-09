// Command floter is the mygo rewrite's entry point: one frameless window
// whose body switches between the launcher, the settings panel and a
// terminal, following the user's existing settings file.
//
//	go run ./cmd/floter
package main

import (
	"log"

	"github.com/egoist/mygo"

	"floter/internal/settings"
	"floter/internal/shell"
)

func main() {
	// Read the same settings.json the Tauri build wrote. A missing or
	// unreadable file is not fatal: the store falls back to the shipped
	// defaults, which is exactly what the old app did.
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

	app := shell.New(shell.Options{Store: store})

	mygo.App.WhenReady(app.Start)

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
