// Command floter is the mygo rewrite's entry point. P0 opens the launcher
// shell: the search field over an empty result area, on the glass panel,
// following the user's existing settings file.
//
//	go run ./cmd/floter
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/launcher"
	"floter/internal/settings"
)

func main() {
	// Read the same settings.json the Tauri build wrote. A missing or
	// unreadable file is not fatal: the loader falls back to the shipped
	// defaults, which is exactly what the old app did.
	loaded, err := settings.LoadDefault()
	if err != nil {
		log.Printf("floter: could not read settings, using defaults: %v", err)
	}
	if path, pathErr := settings.Path(); pathErr == nil {
		log.Printf("floter: settings %s", path)
	}

	app := launcher.New(loaded)

	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title: "floter",
			// The old launcher's geometry: 720 wide, the collapsed input
			// row's height (scaled by ui_scale) plus P0's empty result area.
			Width:  launcher.InputWindowWidth,
			Height: int(launcher.WindowHeight(loaded.UIScale)),
			// A borderless, transparent, centered window with no shadow,
			// hidden from the taskbar — tauri.conf.json's "main" window.
			Frameless:     true,
			Transparent:   true,
			DisableShadow: true,
			SkipTaskbar:   true,
			Content:       ui.View(app.View),
		})
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
