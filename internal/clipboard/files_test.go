package clipboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PrepareFiles is the file list's whole decision: what survives, and what the
// surviving paths' identity is. The oracle stands in for the filesystem so the
// decision is testable with no files anywhere.
func TestPrepareFiles(t *testing.T) {
	exists := func(path string) bool { return path != "/tmp/gone.txt" }

	t.Run("keeps the originals and drops the rest", func(t *testing.T) {
		capture, ok := PrepareFiles([]string{"/tmp/a.txt", "/tmp/a.txt", "   ", "/tmp/gone.txt", "/tmp/b.txt"}, exists)
		if !ok {
			t.Fatal("a usable list was rejected")
		}
		want := []string{"/tmp/a.txt", "/tmp/b.txt"}
		if len(capture.Paths) != len(want) {
			t.Fatalf("paths = %q, want %q", capture.Paths, want)
		}
		for index, path := range want {
			if capture.Paths[index] != path {
				t.Errorf("path %d = %q, want %q", index, capture.Paths[index], path)
			}
		}
		if capture.Hash == "" {
			t.Error("the capture has no identity")
		}
	})

	t.Run("nothing left is no capture", func(t *testing.T) {
		if _, ok := PrepareFiles([]string{"/tmp/gone.txt", ""}, exists); ok {
			t.Error("a list of vanished paths was accepted")
		}
		if _, ok := PrepareFiles(nil, exists); ok {
			t.Error("an empty list was accepted")
		}
	})

	t.Run("the count ceiling is the list as it arrived", func(t *testing.T) {
		paths := make([]string, MaxFiles+1)
		for index := range paths {
			paths[index] = "/tmp/file"
		}
		if _, ok := PrepareFiles(paths, exists); ok {
			t.Errorf("%d paths were accepted, the ceiling is %d", len(paths), MaxFiles)
		}
	})

	t.Run("the byte ceiling is the list as it arrived", func(t *testing.T) {
		huge := "/tmp/" + strings.Repeat("x", MaxFilePathBytes)
		if _, ok := PrepareFiles([]string{huge}, exists); ok {
			t.Errorf("a %d-byte path was accepted, the ceiling is %d", len(huge), MaxFilePathBytes)
		}
	})

	// The identity is the resolved spelling, the stored path is the one the
	// user copied: the same file reached through `./` or `..` is one history
	// entry, but history never rewrites the string.
	t.Run("identity is canonical, the stored path is not", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "a.txt")
		if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		direct, ok := PrepareFiles([]string{file}, nil)
		if !ok {
			t.Fatal("the direct path was rejected")
		}
		spelled := filepath.Join(dir, ".", "sub", "..", "a.txt")
		roundabout, ok := PrepareFiles([]string{spelled}, nil)
		if !ok {
			t.Fatal("the roundabout spelling was rejected")
		}
		if direct.Hash != roundabout.Hash {
			t.Errorf("the same file hashed two ways: %q vs %q", direct.Hash, roundabout.Hash)
		}
		if roundabout.Paths[0] != spelled {
			t.Errorf("stored path = %q, want the spelling that arrived %q", roundabout.Paths[0], spelled)
		}
	})

	// Two different files, or the same two in another order, are different
	// captures.
	t.Run("the order is part of the identity", func(t *testing.T) {
		first, _ := PrepareFiles([]string{"/tmp/a.txt", "/tmp/b.txt"}, exists)
		second, _ := PrepareFiles([]string{"/tmp/b.txt", "/tmp/a.txt"}, exists)
		if first.Hash == second.Hash {
			t.Error("a reordered file list hashed the same")
		}
	})
}

func TestCaptionFor(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want string
	}{
		{"trimmed", "  a caption \n", "a caption"},
		{"whitespace only is no caption", " \n\t ", ""},
		{"empty is no caption", "", ""},
		{"over the ceiling is no caption", strings.Repeat("a", MaxTextBytes+1), ""},
		{"at the ceiling is a caption", strings.Repeat("a", MaxTextBytes), strings.Repeat("a", MaxTextBytes)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CaptionFor(tc.text); got != tc.want {
				t.Errorf("CaptionFor = %q, want %q", got, tc.want)
			}
		})
	}
}

// An image's identity is its pixels plus the caption that rode along: the same
// pixels with different text are different history, and no caption hashes
// exactly as the pixels alone so AddImage's digest never moved.
func TestImageHash(t *testing.T) {
	png := testPNG(t, 3, 2)
	other := testPNG(t, 2, 3)
	if got := ImageHash(png, ""); got != hashBytes(png) {
		t.Errorf("an uncaptioned image hashed as %q, want its own digest %q", got, hashBytes(png))
	}
	if ImageHash(png, "one") == ImageHash(png, "two") {
		t.Error("two captions share one identity")
	}
	if ImageHash(png, "same") == ImageHash(other, "same") {
		t.Error("two images share one identity")
	}
	if ImageHash(png, "same") != ImageHash(png, "same") {
		t.Error("the same pair hashed two ways")
	}
}

// AddImageCaption is the one entry a copy carrying pixels and text produces.
func TestAddImageCaption(t *testing.T) {
	store := NewStore(FromConfigRoot(t.TempDir()), 50)
	png := testPNG(t, 4, 4)

	entry, added, err := store.AddImageCaption(png, 4, 4, "  the caption ")
	if err != nil || !added {
		t.Fatalf("AddImageCaption = %v, %v", added, err)
	}
	if entry.Kind != KindImage || entry.Text != "the caption" {
		t.Errorf("entry = %q / %q, want an image captioned %q", entry.Kind, entry.Text, "the caption")
	}
	if got := entry.Preview(40); got != "the caption" {
		t.Errorf("preview = %q, want the caption", got)
	}

	// The same pixels and caption move to the front instead of duplicating.
	if _, added, err := store.AddImageCaption(png, 4, 4, "the caption"); err != nil || added {
		t.Errorf("the same pair was added twice: %v, %v", added, err)
	}

	// The same pixels under another caption are another capture.
	other, added, err := store.AddImageCaption(png, 4, 4, "another caption")
	if err != nil || !added {
		t.Fatalf("a second caption was dropped: %v, %v", added, err)
	}
	if len(store.Entries()) != 2 {
		t.Fatalf("history holds %d entries, want 2", len(store.Entries()))
	}
	if other.Hash == entry.Hash {
		t.Error("two captions share one identity")
	}

	// AddImage is the uncaptioned case of the same call.
	plain := testPNG(t, 5, 5)
	entry, added, err = store.AddImage(plain, 5, 5)
	if err != nil || !added {
		t.Fatalf("AddImage = %v, %v", added, err)
	}
	if entry.Text != "" || entry.Hash != hashBytes(plain) {
		t.Errorf("AddImage stored %q / %q, want no caption and its own digest", entry.Text, entry.Hash)
	}
}

func TestAddFilesCapture(t *testing.T) {
	store := NewStore(FromConfigRoot(t.TempDir()), 50)
	capture := FilesCapture{Paths: []string{"/tmp/a.txt", "/tmp/b.txt"}, Hash: "identity"}
	entry, added, err := store.AddFilesCapture(capture)
	if err != nil || !added {
		t.Fatalf("AddFilesCapture = %v, %v", added, err)
	}
	if entry.Kind != KindFiles || entry.Hash != "identity" || len(entry.Paths) != 2 {
		t.Errorf("entry = %+v, want the capture's paths under its identity", entry)
	}
	if _, added, err := store.AddFilesCapture(capture); err != nil || added {
		t.Errorf("the same capture was added twice: %v, %v", added, err)
	}
	if _, added, err := store.AddFilesCapture(FilesCapture{}); err != nil || added {
		t.Errorf("an empty capture was added: %v, %v", added, err)
	}
}
