package launcher

import (
	"os"
	"sync"

	"github.com/egoist/mygo/ui"

	"floter/internal/clipboard"
)

// Clipboard image thumbnails: an image entry is hard to recognise by its label
// ("Image 2026-10-09 12:00"), so the row shows a small copy of it.
//
// The decode happens off the drawing thread, and is cached by the file's path,
// so a redraw is a map lookup and a history of a hundred images decodes only
// the ones on screen.
//
// It is off the drawing thread because it used to be on it: `clipboardItems`
// runs on the search path, which runs on every keystroke, and a decoded
// clipboard image is whatever the user last captured — a 4K screenshot is tens
// of megabytes of pixels. Typing in the clipboard mode decoded the visible
// rows again on every edit, and the whole window stopped answering (the report
// 「很多界面经常卡死」). The first frame for a path now draws the row without
// its thumbnail, asks for a decode in the background (two at a time, like the
// icon extractions), and asks for another frame when the bitmap lands.

var (
	// thumbMu guards the background decode bookkeeping below: the drawing
	// thread and the decode goroutines both touch it.
	thumbMu sync.Mutex
	// thumbReady holds the decoded bitmaps by path. A nil entry is a decode
	// that produced nothing (a missing or unreadable file): remembered so a
	// redraw does not try it again every frame.
	thumbReady = map[string]*ui.Bitmap{}
	// thumbPending is the set of paths a background decode is producing right
	// now, so a redraw does not start a second one for the same row.
	thumbPending = map[string]bool{}
	// thumbInvalidate asks the interface for another frame once a thumbnail
	// landed. It is set by the shell (SetThumbInvalidate); nil simply means the
	// next frame that would draw the row draws it.
	thumbInvalidate func()
	// thumbDecodes bounds the background decodes: a decode is megabytes of
	// pixel work, and a history of images must not start twenty at once.
	thumbDecodes = make(chan struct{}, 2)
)

// SetThumbInvalidate installs the callback that asks the interface for a frame
// when a background thumbnail decode finishes. Safe from any goroutine.
func SetThumbInvalidate(fn func()) {
	thumbMu.Lock()
	thumbInvalidate = fn
	thumbMu.Unlock()
}

// clipThumb decodes an image entry's thumbnail, nil for anything else and nil
// while the background decode is still running (the row draws its plate, and
// the next frame draws the image).
func (a *App) clipThumb(entry clipboard.Entry) *ui.Bitmap {
	if entry.Kind != clipboard.KindImage || a.Clipboard == nil {
		return nil
	}
	path := a.Clipboard.ImagePath(entry)
	if path == "" {
		return nil
	}
	if a.thumbs == nil {
		a.thumbs = map[string]*ui.Bitmap{}
	}
	if bitmap, ok := a.thumbs[path]; ok {
		return bitmap
	}
	thumbMu.Lock()
	bitmap, ready := thumbReady[path]
	pending := thumbPending[path]
	if ready {
		thumbMu.Unlock()
		// The decode landed since the last frame: remember it on this
		// (drawing) thread's own memo and draw it now.
		a.thumbs[path] = bitmap
		return bitmap
	}
	if !pending {
		thumbPending[path] = true
	}
	thumbMu.Unlock()
	if !ready && !pending {
		decodeThumbAsync(path)
	}
	return nil
}

// decodeThumbAsync runs one image decode off the drawing thread and asks for a
// frame when it lands. The path is marked pending by the caller, so the frames
// in between ask for the thumbnail without starting a second decode.
func decodeThumbAsync(path string) {
	go func() {
		thumbDecodes <- struct{}{}
		var bitmap *ui.Bitmap
		if data, err := os.ReadFile(path); err == nil {
			if decoded, err := ui.DecodeBitmap(data); err == nil {
				bitmap = decoded
			}
		}
		<-thumbDecodes
		thumbMu.Lock()
		thumbReady[path] = bitmap
		delete(thumbPending, path)
		invalidate := thumbInvalidate
		thumbMu.Unlock()
		if invalidate != nil {
			invalidate()
		}
	}()
}

// pruneThumbs bounds the thumbnail memo (the old build's R97). The memo only
// ever grew, so a long session kept one decoded bitmap per image ever scrolled
// past — megabytes each — while the history that produced them is capped. Once
// the memo passes the history's own ceiling, only the paths the current result
// set still references survive; a row that needs one again decodes it again.
// Below the ceiling the map is left untouched, so the common case costs
// nothing.
//
// The background cache is bounded by the same rule: a path that left the
// history is a bitmap nobody will ask for again.
func (a *App) pruneThumbs(entries []clipboard.Entry) {
	if len(a.thumbs) > clipboard.MaxMaxItems {
		keep := make(map[string]bool, len(entries))
		if a.Clipboard != nil {
			for _, entry := range entries {
				if path := a.Clipboard.ImagePath(entry); path != "" {
					keep[path] = true
				}
			}
		}
		for path := range a.thumbs {
			if !keep[path] {
				delete(a.thumbs, path)
			}
		}
	}
	thumbMu.Lock()
	if len(thumbReady) > clipboard.MaxMaxItems {
		keep := make(map[string]bool, len(entries))
		if a.Clipboard != nil {
			for _, entry := range entries {
				if path := a.Clipboard.ImagePath(entry); path != "" {
					keep[path] = true
				}
			}
		}
		for path := range thumbReady {
			if !keep[path] {
				delete(thumbReady, path)
			}
		}
	}
	thumbMu.Unlock()
}
