package apps

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The conversion is the reference's own command line
// (`main:src-tauri/src/commands/apps/macos.rs:520-528`), argument for argument:
// a different flag would silently produce no icon on the one platform this
// build cannot run its own tests on.
func TestICNSConvertArgsMatchTheReference(t *testing.T) {
	got := icnsConvertArgs("/Applications/App.app/Contents/Resources/AppIcon.icns", "/tmp/out.png")
	want := []string{"-s", "format", "png", "/Applications/App.app/Contents/Resources/AppIcon.icns", "--out", "/tmp/out.png"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

// convertICNS returns the PNG the converter wrote, and leaves no temporary file
// behind: this runs once per legacy bundle per session, not once per frame, so
// a stray file per run would accumulate.
func TestConvertICNSReturnsTheConvertedPNG(t *testing.T) {
	png := pngBytes(t, 32)
	var target string
	got := convertICNS("/somewhere/Legacy.icns", func(args []string) error {
		target = args[len(args)-1]
		return os.WriteFile(target, png, 0o644)
	})
	if !bytes.Equal(got, png) {
		t.Fatalf("converted = %d bytes, want %d", len(got), len(png))
	}
	if !strings.HasSuffix(target, ".png") {
		t.Errorf("target %q is not a .png", target)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("the temporary target survived the conversion (%v)", err)
	}
}

// A converter that fails, or that leaves something which is not a PNG, gives no
// icon rather than half an answer: the reference only takes the result when the
// process succeeded and the file exists, and this build checks the bytes too.
func TestConvertICNSRefusesAHalfAnswer(t *testing.T) {
	if got := convertICNS("/somewhere/Legacy.icns", func([]string) error {
		return os.ErrNotExist
	}); got != nil {
		t.Errorf("a failed conversion returned %d bytes", len(got))
	}
	if got := convertICNS("/somewhere/Legacy.icns", func(args []string) error {
		return os.WriteFile(args[len(args)-1], []byte("not a png at all"), 0o644)
	}); got != nil {
		t.Errorf("a non-PNG result returned %d bytes", len(got))
	}
}

// writeLegacyICNS writes an `.icns` whose only entry is a pre-10.7 bitmap: the
// container is valid, and this build's own reader finds no PNG in it.
func writeLegacyICNS(t *testing.T, path string) {
	t.Helper()
	// `is32` is RLE bitmap data, not a PNG; the payload here is opaque to this
	// build, which is the point — only the platform converter can read it.
	writeBytes(t, path, icnsContainer(t, []icnsEntry{{kind: "is32", payload: []byte("\x00\x01\x02\x03\xff")}}))
}

// A bundle whose `.icns` holds no PNG is not iconless: the platform is asked,
// and its answer is the row's icon. Stopping at the reader would show a first
// letter where the reference shows the application's icon.
func TestReadIconAsksThePlatformForALegacyICNS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Legacy.icns")
	writeLegacyICNS(t, path)
	if LargestPNGFromICNS(mustReadFile(t, path)) != nil {
		t.Fatal("the fixture holds a PNG; it cannot stand for a legacy icon")
	}

	want := pngBytes(t, 48)
	restore := swapPlatformExtract(t, func(app App) []byte {
		if app.IconPath != path {
			t.Errorf("the platform was asked about %q, want %q", app.IconPath, path)
		}
		return want
	})
	defer restore()

	if got := readIcon(App{Name: "Legacy", Path: "/Applications/Legacy.app", IconPath: path}); !bytes.Equal(got, want) {
		t.Errorf("readIcon = %d bytes, want the platform's %d", len(got), len(want))
	}
}

// A file this build *can* read is the whole answer: the platform is not asked,
// which is what keeps a launch off the drawing thread for every modern bundle.
func TestReadIconKeepsThePNGEntryWithoutAsking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Modern.icns")
	small := pngBytes(t, 16)
	large := pngBytes(t, 64)
	writeBytes(t, path, icnsContainer(t, []icnsEntry{{kind: "icp4", payload: small}, {kind: "ic08", payload: large}}))

	asked := false
	restore := swapPlatformExtract(t, func(App) []byte {
		asked = true
		return nil
	})
	defer restore()

	if got := readIcon(App{Path: "/Applications/Modern.app", IconPath: path}); !bytes.Equal(got, large) {
		t.Errorf("readIcon = %d bytes, want the largest PNG entry (%d)", len(got), len(large))
	}
	if asked {
		t.Error("the platform was asked about a bundle whose icon this build can read")
	}
}

// An unreadable icon file is also the platform's question, not a dead end: the
// reference re-resolves the source and converts, and this build asks the
// platform rather than remembering "no icon" for the session.
func TestReadIconAsksThePlatformForAnUnreadableFile(t *testing.T) {
	want := pngBytes(t, 32)
	restore := swapPlatformExtract(t, func(App) []byte { return want })
	defer restore()

	if got := readIcon(App{Path: "/Applications/Gone.app", IconPath: filepath.Join(t.TempDir(), "Gone.icns")}); !bytes.Equal(got, want) {
		t.Errorf("readIcon = %d bytes, want the platform's %d", len(got), len(want))
	}
}

// The file rule itself: what a file's bytes mean, before any platform is asked.
func TestIconFromFileTruthTable(t *testing.T) {
	png := pngBytes(t, 16)
	if got := iconFromFile("/icons/a.png", png); !bytes.Equal(got, png) {
		t.Errorf("a PNG file returned %d bytes", len(got))
	}
	if got := iconFromFile("/icons/a.png", []byte("GIF89a....")); got != nil {
		t.Errorf("a non-PNG file returned %d bytes", len(got))
	}
	legacy := icnsContainer(t, []icnsEntry{{kind: "is32", payload: []byte("\x00\x01\x02\x03\xff")}})
	if got := iconFromFile("/icons/a.icns", legacy); got != nil {
		t.Errorf("a legacy .icns returned %d bytes", len(got))
	}
	modern := icnsContainer(t, []icnsEntry{{kind: "ic07", payload: png}})
	if got := iconFromFile("/icons/A.ICNS", modern); !bytes.Equal(got, png) {
		t.Errorf("an uppercase .ICNS returned %d bytes", len(got))
	}
}

// swapPlatformExtract installs a platform extraction for one test and returns
// the restore. It is the seam that lets this host (Linux, whose own extraction
// is a constant nil) pin the macOS fall-through.
func swapPlatformExtract(t *testing.T, fn func(App) []byte) func() {
	t.Helper()
	previous := platformExtract
	platformExtract = fn
	return func() { platformExtract = previous }
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
