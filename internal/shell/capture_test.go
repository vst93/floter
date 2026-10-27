package shell

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"

	"floter/internal/clipboard"
)

// capturePNG is a real PNG of one flat colour: two calls with different fills
// are two different screenshots that carry the same format list.
func capturePNG(t *testing.T, width, height int, fill uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: fill, G: fill, B: fill, A: 0xff})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// captureApp is an App with a clipboard store in a temp dir and nothing else:
// captureClipboard reads the store, the sample and its own memory, no window.
func captureApp(t *testing.T) *App {
	t.Helper()
	return &App{Clipboard: clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), 50)}
}

// clipboardStage stands in for the system clipboard: the sample one poll reads,
// and the image rendition that poll would fetch.
type clipboardStage struct {
	sample clipSample
	// imageReads counts how often the PNG was asked for, so a test can pin that
	// a winning file list never touches the pixels.
	imageReads int
}

// set stages one sample.
func (s *clipboardStage) set(sample clipSample) { s.sample = sample }

// stageClipboard installs the stand-in and restores the real readers at the end
// of the test.
func stageClipboard(t *testing.T) *clipboardStage {
	t.Helper()
	stage := &clipboardStage{}
	originalSample, originalImage := clipboardSample, clipboardImageReader
	clipboardSample = func() clipSample {
		sample := stage.sample
		sample.PNG, sample.Width, sample.Height = nil, 0, 0
		return sample
	}
	clipboardImageReader = func([]transfer.Format) ([]byte, int, int) {
		stage.imageReads++
		if len(stage.sample.PNG) == 0 {
			return nil, 0, 0
		}
		return stage.sample.PNG, stage.sample.Width, stage.sample.Height
	}
	t.Cleanup(func() { clipboardSample, clipboardImageReader = originalSample, originalImage })
	return stage
}

// pngFormats is what a screenshot's clipboard reports: one PNG rendition and
// nothing else.
var pngFormats = []transfer.Format{transfer.PNG}

// The regression this round exists for: the watcher used to decide "is this a
// new image?" by comparing the clipboard's FORMAT LIST with the last poll's.
// Two different screenshots carry the same list, so every image after the first
// was dropped. Identity is the content now.
func TestCaptureClipboardKeepsEveryImage(t *testing.T) {
	a := captureApp(t)
	first := capturePNG(t, 4, 3, 0x10)
	second := capturePNG(t, 4, 3, 0x20)

	stage := stageClipboard(t)
	stage.set(clipSample{PNG: first, Width: 4, Height: 3, formats: pngFormats})
	if !a.captureClipboard() {
		t.Fatal("the first image was not recorded")
	}

	stage.set(clipSample{PNG: second, Width: 4, Height: 3, formats: pngFormats})
	if !a.captureClipboard() {
		t.Fatal("the second image was dropped: the poll compared format lists, not content")
	}

	entries := a.Clipboard.Entries()
	if len(entries) != 2 {
		t.Fatalf("history holds %d entries, want 2", len(entries))
	}
	if entries[0].Hash == entries[1].Hash {
		t.Error("two screenshots share one identity")
	}
	for _, entry := range entries {
		if entry.Kind != clipboard.KindImage {
			t.Errorf("entry kind = %q, want an image", entry.Kind)
		}
	}

	// The unchanged clipboard is not recorded again: identity is the content.
	if a.captureClipboard() {
		t.Error("the same image was recorded twice")
	}
	if got := len(a.Clipboard.Entries()); got != 2 {
		t.Errorf("history holds %d entries after a repeat poll, want 2", got)
	}
}

// A copy can carry pixels AND text: the text is the image's caption, one entry,
// never the text alone shadowing the image.
func TestCaptureClipboardKeepsTheCaptionBesideTheImage(t *testing.T) {
	a := captureApp(t)
	png := capturePNG(t, 2, 2, 0x40)
	stage := stageClipboard(t)
	stage.set(clipSample{PNG: png, Width: 2, Height: 2, Text: "  a caption \n", formats: pngFormats})

	if !a.captureClipboard() {
		t.Fatal("the captioned image was not recorded")
	}
	entries := a.Clipboard.Entries()
	if len(entries) != 1 {
		t.Fatalf("history holds %d entries, want one", len(entries))
	}
	if entries[0].Kind != clipboard.KindImage {
		t.Fatalf("kind = %q, want the image, not the text alone", entries[0].Kind)
	}
	if entries[0].Text != "a caption" {
		t.Errorf("caption = %q, want the trimmed text", entries[0].Text)
	}
	if got := entries[0].Preview(40); got != "a caption" {
		t.Errorf("preview = %q, want the caption", got)
	}
}

// A Finder or Explorer selection carries its paths as text too. Files first is
// what keeps the copy's file semantics; the text rendition is the fallback for
// a list whose paths have all vanished.
func TestCaptureClipboardRecordsFilesBeforeTheirTextRendition(t *testing.T) {
	a := captureApp(t)
	originalExists := clipboardFileExists
	clipboardFileExists = func(path string) bool { return path == "/tmp/one.txt" }
	defer func() { clipboardFileExists = originalExists }()
	stage := stageClipboard(t)
	stage.set(clipSample{
		Paths:   []string{"/tmp/one.txt", "/tmp/gone.txt"},
		Text:    "/tmp/one.txt\n/tmp/gone.txt",
		formats: []transfer.Format{transfer.Text, "text/uri-list"},
		PNG:     capturePNG(t, 2, 2, 0x55), // a file copy that also carries pixels
		Width:   2,
		Height:  2,
	})

	if !a.captureClipboard() {
		t.Fatal("the file copy was not recorded")
	}
	entries := a.Clipboard.Entries()
	if len(entries) != 1 {
		t.Fatalf("history holds %d entries, want one", len(entries))
	}
	if entries[0].Kind != clipboard.KindFiles {
		t.Fatalf("kind = %q, want the file list, not its text rendition", entries[0].Kind)
	}
	if len(entries[0].Paths) != 1 || entries[0].Paths[0] != "/tmp/one.txt" {
		t.Errorf("paths = %q, want the one path that still exists", entries[0].Paths)
	}

	if stage.imageReads != 0 {
		t.Errorf("the winning file list still read the pixels %d times", stage.imageReads)
	}

	// Every path gone: the same copy falls through to its text.
	a = captureApp(t)
	stage.set(clipSample{Paths: []string{"/tmp/gone.txt"}, Text: "/tmp/gone.txt", formats: []transfer.Format{transfer.Text}})
	if !a.captureClipboard() {
		t.Fatal("the text rendition of a vanished file list was not recorded")
	}
	entries = a.Clipboard.Entries()
	if len(entries) != 1 || entries[0].Kind != clipboard.KindText {
		t.Fatalf("entries = %+v, want one text entry", entries)
	}
}

// The watcher must not record what the app itself put on the clipboard.
func TestCaptureClipboardSkipsItsOwnWrite(t *testing.T) {
	a := captureApp(t)
	a.selfCopied = "copied by floter"
	stage := stageClipboard(t)
	stage.set(clipSample{Text: "copied by floter", formats: []transfer.Format{transfer.Text}})

	if a.captureClipboard() {
		t.Error("the app's own copy was recorded")
	}
	if got := len(a.Clipboard.Entries()); got != 0 {
		t.Errorf("history holds %d entries, want none", got)
	}
}

// planCapture is the whole decision for a sample with no usable file list, and
// its order and ceilings are the reference's.
func TestPlanCapture(t *testing.T) {
	small := capturePNG(t, 2, 2, 0x11)

	t.Run("the image wins and the text is its caption", func(t *testing.T) {
		plan, ok := planCapture(clipSample{PNG: small, Width: 2, Height: 2, Text: "caption"})
		if !ok {
			t.Fatal("the sample was rejected")
		}
		if plan.Kind != clipboard.KindImage || plan.Caption != "caption" {
			t.Errorf("plan = %q / %q, want an image captioned", plan.Kind, plan.Caption)
		}
		if plan.Hash != clipboard.ImageHash(small, "caption") {
			t.Errorf("hash = %q, want the image's identity", plan.Hash)
		}
	})

	t.Run("text alone", func(t *testing.T) {
		plan, ok := planCapture(clipSample{Text: "hello"})
		if !ok {
			t.Fatal("plain text was rejected")
		}
		if plan.Kind != clipboard.KindText || plan.Hash != clipboard.HashText("hello") {
			t.Errorf("plan = %q / %q, want a text entry", plan.Kind, plan.Hash)
		}
	})

	t.Run("nothing capturable", func(t *testing.T) {
		for name, sample := range map[string]clipSample{
			"empty":                 {},
			"whitespace text":       {Text: " \n\t "},
			"over the text ceiling": {Text: strings.Repeat("a", clipboard.MaxTextBytes+1)},
			"pixels with no size":   {PNG: small},
		} {
			if plan, ok := planCapture(sample); ok {
				t.Errorf("%s produced %+v, want nothing", name, plan)
			}
		}
	})

	t.Run("an oversized image records nothing, not even its text", func(t *testing.T) {
		huge := make([]byte, clipboard.MaxImagePNGBytes+1)
		if plan, ok := planCapture(clipSample{PNG: huge, Width: 2, Height: 2, Text: "caption"}); ok {
			t.Errorf("an oversized image produced %+v, want nothing", plan)
		}
	})
}
