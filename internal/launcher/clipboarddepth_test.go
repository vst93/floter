package launcher

import (
	"fmt"
	"testing"

	clipboardpkg "floter/internal/clipboard"
)

// P7-ee · how deep the clipboard mode holds its rows.
//
// The reference's inline launcher hands the mode `MAX_CLIPBOARD_MAX_ITEMS`
// (`main:src/hooks/useLauncherCatalog.ts:833-834`), which is the history's own
// ceiling (`main:src/clipboard-history.ts:440` = 500) — the whole *bounded*
// history, not the ten-row group budget (`CLIPBOARD_FETCH_LIMIT = MAX_RESULTS`)
// the plugin output view spends. This build held fifty.
func TestTheClipboardModeHoldsTheReferenceFetchDepth(t *testing.T) {
	if ClipboardFetchLimit != clipboardpkg.MaxMaxItems {
		t.Fatalf("ClipboardFetchLimit = %d, want the history ceiling %d",
			ClipboardFetchLimit, clipboardpkg.MaxMaxItems)
	}
	if ClipboardFetchLimit != 500 {
		t.Fatalf("ClipboardFetchLimit = %d, want 500 (src/clipboard-history.ts:440)", ClipboardFetchLimit)
	}

	store := clipboardpkg.NewStore(clipboardpkg.FromConfigRoot(t.TempDir()), clipboardpkg.MaxMaxItems)
	for i := 0; i < 600; i++ {
		if _, _, err := store.AddText(fmt.Sprintf("clip %03d", i)); err != nil {
			t.Fatal(err)
		}
	}
	// The store's capacity is the same ceiling, so the oldest hundred are
	// pruned: the mode may hold five hundred rows, never six hundred.
	if got := len(store.Entries()); got != clipboardpkg.MaxMaxItems {
		t.Fatalf("stored entries = %d, want %d", got, clipboardpkg.MaxMaxItems)
	}

	a := testApp()
	a.Clipboard = store
	a.EnterClipboard()
	rows := a.clipboardItems()
	if len(rows) != ClipboardFetchLimit {
		t.Fatalf("the clipboard mode held %d rows, want %d", len(rows), ClipboardFetchLimit)
	}
	if rows[0].Title != "clip 599" {
		t.Fatalf("newest row = %q, want clip 599", rows[0].Title)
	}
}

// The chip filter runs against the whole held set, not against a slice of the
// newest matches taken first: the reference filters by the chip *before* it
// spends the limit (`clipboardModeRows`:
// `filterClipboardEntries(needle).filter(clipboardEntryMatchesFilter)`, then
// `matched.slice(0, limit)`), so an image that sits past the newest five
// hundred rows still answers the `images` chip.
func TestTheClipboardChipFilterSeesTheWholeHeldSet(t *testing.T) {
	store := clipboardpkg.NewStore(clipboardpkg.FromConfigRoot(t.TempDir()), clipboardpkg.MaxMaxItems)
	// The image is the oldest entry, and starred so the age prune keeps it: a
	// fetch limit spent on the newest matches would cut it off, and the chip
	// would then find nothing even though the history holds an image.
	image, _, err := store.AddImage([]byte{1, 2, 3}, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetFavorite(image.ID, true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 600; i++ {
		if _, _, err := store.AddText(fmt.Sprintf("clip %03d", i)); err != nil {
			t.Fatal(err)
		}
	}

	a := testApp()
	a.Clipboard = store
	a.EnterClipboard()
	a.clipboardFilter = filterImage
	rows := a.clipboardItems()
	if len(rows) != 1 {
		t.Fatalf("the images chip shows %d rows, want 1 (the oldest entry is the only image)", len(rows))
	}
	if rows[0].clip == nil || rows[0].clip.Kind != clipboardpkg.KindImage {
		t.Fatalf("the images chip kept a non-image row: %+v", rows[0])
	}
}
