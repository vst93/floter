package launcher

import (
	"os"

	"github.com/egoist/mygo/ui"

	"floter/internal/clipboard"
)

// Clipboard image thumbnails: an image entry is hard to recognise by its label
// ("Image 2026-10-09 12:00"), so the row shows a small copy of it.
//
// The decode happens the first time a row is drawn and is cached by the file's
// path, so a redraw is a map lookup and a history of a hundred images decodes
// only the ones on screen.

// clipThumb decodes an image entry's thumbnail, nil for anything else.
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
	var bitmap *ui.Bitmap
	if data, err := os.ReadFile(path); err == nil {
		if decoded, err := ui.DecodeBitmap(data); err == nil {
			bitmap = decoded
		}
	}
	a.thumbs[path] = bitmap
	return bitmap
}

// pruneThumbs bounds the thumbnail memo (the old build's R97). The memo only
// ever grew, so a long session kept one decoded bitmap per image ever scrolled
// past — megabytes each — while the history that produced them is capped. Once
// the memo passes the history's own ceiling, only the paths the current result
// set still references survive; a row that needs one again decodes it again.
// Below the ceiling the map is left untouched, so the common case costs
// nothing.
func (a *App) pruneThumbs(entries []clipboard.Entry) {
	if len(a.thumbs) <= clipboard.MaxMaxItems {
		return
	}
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
