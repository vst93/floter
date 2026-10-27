package apps

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The icon search path, pinned against the reference
// (`main:src-tauri/src/commands/apps/linux.rs`): the name rule, the order of the
// directories, the size table, the three theme layouts, and the one deliberate
// difference (PNG only, because this build's renderer is `ui.DecodeBitmap`).
//
// Every test builds its own theme tree under a temporary `$HOME` /
// `$XDG_DATA_HOME`, and every icon name carries a `p10s-` prefix, so a file on
// the machine running the test cannot answer for a file the test did not
// create. `resetIconCaches` drops the per-theme tables between cases: a theme
// tree is indexed once per process, and these tests keep adding files to it.

// iconTree writes one file into a theme tree under the fake data home and
// returns the path it wrote. `parts` are the path elements below the data
// home's `icons` directory.
func iconTree(t *testing.T, dataHome string, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{dataHome, "icons"}, parts...)...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// fakeXDG points `$HOME`, `$XDG_DATA_HOME` and `$XDG_DATA_DIRS` at temporary
// directories and drops the icon tables before and after the test, so no
// resolution from another test (or the real machine) can leak in.
func fakeXDG(t *testing.T) (home, dataHome string) {
	t.Helper()
	home = t.TempDir()
	dataHome = filepath.Join(home, "data")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(home, "share"))
	resetIconCaches()
	t.Cleanup(resetIconCaches)
	return home, dataHome
}

// The name an `Icon=` value stands for: a known icon extension is dropped, and
// nothing else is. The second half is the deliberate difference from the
// reference's `Path::file_stem`, which would turn `org.p10s.kept` into
// `org.p10s` — and real machines are full of those names
// (`/usr/share/icons/hicolor/*/apps/org.gnome.gedit.png`,
// `org.cachyos.KernelManager.png`, and two dozen `Icon=org.gnome.X` entries in
// `/usr/share/applications`).
func TestTheIconNameLosesOnlyAKnownExtension(t *testing.T) {
	_, dataHome := fakeXDG(t)

	plain := iconTree(t, dataHome, "hicolor", "48x48", "apps", "p10s-a.png")
	// `Icon=p10s-a.svg` names the same icon: the entry named the icon, and the
	// format it happened to spell is not the one this build draws.
	for _, value := range []string{"p10s-a", "p10s-a.png", "p10s-a.PNG", "p10s-a.svg", "p10s-a.xpm"} {
		if got := desktopIconFile(value); got != plain {
			t.Errorf("Icon=%q resolved to %q, want %q", value, got, plain)
		}
	}

	dotted := iconTree(t, dataHome, "hicolor", "48x48", "apps", "org.p10s.kept.png")
	for _, value := range []string{"org.p10s.kept", "org.p10s.kept.png"} {
		if got := desktopIconFile(value); got != dotted {
			t.Errorf("Icon=%q resolved to %q, want %q (a dotted name is not truncated)", value, got, dotted)
		}
	}
	// The stem the reference would compute is a name this build must not look
	// for: it belongs to no file.
	if got := desktopIconFile("org.p10s"); got != "" {
		t.Errorf("the truncated name resolved to %q", got)
	}
}

// The reference searches the flat pixmap directories **before** the themes
// (`resolve_icon_uncached`, linux.rs:603-635): a name in `pixmaps` is an
// explicit flat install of that one icon.
func TestPixmapsComeBeforeThemes(t *testing.T) {
	_, dataHome := fakeXDG(t)

	pixmap := filepath.Join(dataHome, "pixmaps", "p10s-both.png")
	if err := os.MkdirAll(filepath.Dir(pixmap), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(pixmap, []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	iconTree(t, dataHome, "hicolor", "256x256", "apps", "p10s-both.png")

	if got := desktopIconFile("p10s-both"); got != pixmap {
		t.Errorf("resolved to %q, want the pixmap %q", got, pixmap)
	}
	want := []string{filepath.Join(dataHome, "pixmaps"), "/usr/share/pixmaps"}
	if got := pixmapDirs(); !reflect.DeepEqual(got, want) {
		t.Errorf("pixmapDirs = %v, want %v", got, want)
	}
}

// Every theme directory under a base is searched, not just the one the old
// build spelled into its directory list (`<base>/hicolor`), and the preferred
// themes come first in the reference's order.
func TestEveryThemeUnderABaseIsSearched(t *testing.T) {
	_, dataHome := fakeXDG(t)

	// A theme the old build could never reach: it only ever tried
	// `<base>/<size>/apps`, so `<base>/Papirus/48x48/apps` was invisible.
	papirus := iconTree(t, dataHome, "Papirus", "48x48", "apps", "p10s-pap.png")
	if got := desktopIconFile("p10s-pap"); got != papirus {
		t.Errorf("Papirus icon resolved to %q, want %q", got, papirus)
	}

	// Preferred order: `Adwaita` before `Papirus`, whatever the directory
	// listing order is.
	adwaita := iconTree(t, dataHome, "Adwaita", "48x48", "apps", "p10s-pref.png")
	iconTree(t, dataHome, "Papirus", "48x48", "apps", "p10s-pref.png")
	if got := desktopIconFile("p10s-pref"); got != adwaita {
		t.Errorf("preferred theme resolved to %q, want Adwaita's %q", got, adwaita)
	}

	// The preferred list itself, in the reference's order.
	wantThemes := []string{"hicolor", "Adwaita", "breeze", "Papirus", "gnome", "default"}
	if got := preferredThemes; !reflect.DeepEqual(got, wantThemes) {
		t.Errorf("preferredThemes = %v, want %v", got, wantThemes)
	}
}

// The three layouts the reference tries per size: `48x48/apps` (hicolor),
// `apps/48x48` and `apps/48` (breeze-style themes). Each case carries a decoy
// in the theme's flat `apps/` directory, which the bounded walk would return
// instead — so a missing fast path fails here rather than passing on the walk.
func TestTheThreeThemeLayouts(t *testing.T) {
	_, dataHome := fakeXDG(t)

	cases := []struct {
		name string
		rel  []string
	}{
		{"p10s-l1.png", []string{"48x48", "apps"}},
		{"p10s-l2.png", []string{"apps", "48x48"}},
		{"p10s-l3.png", []string{"apps", "48"}},
	}
	for _, c := range cases {
		want := iconTree(t, dataHome, append([]string{"hicolor"}, append(c.rel, c.name)...)...)
		iconTree(t, dataHome, "hicolor", "apps", c.name) // the decoy
		resetIconCaches()
		got := desktopIconFile(trimPNG(c.name))
		if got != want {
			t.Errorf("layout %v resolved to %q, want %q", c.rel, got, want)
		}
	}
}

// trimPNG drops the `.png` an icon name is written with.
func trimPNG(name string) string {
	if len(name) > 4 {
		return name[:len(name)-4]
	}
	return name
}

// The size table is the reference's `ICON_SIZES` verbatim, and the largest
// entry wins: on this machine `qemu` ships both a 512x512 and a 256x256 icon and
// the old six-entry table returned the 256.
func TestTheLargestSizeWins(t *testing.T) {
	_, dataHome := fakeXDG(t)

	want := []string{
		"512x512", "256x256", "192x192", "128x128", "96x96", "72x72", "64x64",
		"48x48", "scalable", "36x36", "32x32", "24x24", "16x16",
	}
	if got := iconSizes; !reflect.DeepEqual(got, want) {
		t.Errorf("iconSizes = %v, want the reference's %v", got, want)
	}

	iconTree(t, dataHome, "hicolor", "256x256", "apps", "p10s-big.png")
	big := iconTree(t, dataHome, "hicolor", "512x512", "apps", "p10s-big.png")
	if got := desktopIconFile("p10s-big"); got != big {
		t.Errorf("resolved to %q, want the 512x512 %q", got, big)
	}

	// A size only the reference's table has is still searched: `16x16` is not
	// in the old table, and a name that exists only there used to resolve to
	// nothing.
	small := iconTree(t, dataHome, "hicolor", "16x16", "apps", "p10s-small.png")
	if got := desktopIconFile("p10s-small"); got != small {
		t.Errorf("16x16-only icon resolved to %q, want %q", got, small)
	}
}

// The bases are the reference's `icon_base_dirs`: the user's `~/.icons`, the
// XDG data home (its `icons` and its flatpak exports), the two system roots,
// the system flatpak exports, then `$XDG_DATA_DIRS`' own `icons` directories —
// with duplicates dropped.
func TestTheIconBaseDirsAreTheReferences(t *testing.T) {
	home, dataHome := fakeXDG(t)
	t.Setenv("XDG_DATA_DIRS", "/opt/p10s-a/share:/opt/p10s-b/share:"+dataHome)

	want := []string{
		filepath.Join(home, ".icons"),
		filepath.Join(dataHome, "icons"),
		filepath.Join(dataHome, "flatpak", "exports", "share", "icons"),
		"/usr/local/share/icons",
		"/usr/share/icons",
		"/var/lib/flatpak/exports/share/icons",
		"/opt/p10s-a/share/icons",
		"/opt/p10s-b/share/icons",
	}
	got := iconBaseDirs()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("iconBaseDirs = %v, want %v", got, want)
	}

	// `$XDG_DATA_HOME` really is read: an icon under it resolves, which the
	// old hard-coded `~/.local/share` could not do.
	wantIcon := iconTree(t, dataHome, "hicolor", "48x48", "apps", "p10s-xdg.png")
	if got := desktopIconFile("p10s-xdg"); got != wantIcon {
		t.Errorf("$XDG_DATA_HOME icon resolved to %q, want %q", got, wantIcon)
	}

	// And `~/.icons` is a base too, with the same theme rule under it.
	wantHome := filepath.Join(home, ".icons", "hicolor", "48x48", "apps", "p10s-home.png")
	if err := os.MkdirAll(filepath.Dir(wantHome), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(wantHome, []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := desktopIconFile("p10s-home"); got != wantHome {
		t.Errorf("~/.icons icon resolved to %q, want %q", got, wantHome)
	}
}

// Themes with an unusual layout are found by a bounded walk: the reference
// descends at most three levels below a theme directory
// (`search_recursive`, linux.rs:726-760).
func TestAnUnusualLayoutIsFoundThreeDeep(t *testing.T) {
	_, dataHome := fakeXDG(t)

	deep := iconTree(t, dataHome, "hicolor", "a", "b", "c", "p10s-deep.png")
	iconTree(t, dataHome, "hicolor", "a", "b", "c", "d", "p10s-toodeep.png")

	if got := desktopIconFile("p10s-deep"); got != deep {
		t.Errorf("three-deep icon resolved to %q, want %q", got, deep)
	}
	if got := desktopIconFile("p10s-toodeep"); got != "" {
		t.Errorf("four-deep icon resolved to %q, want nothing", got)
	}
}

// Only PNG files are candidates. The renderer is `ui.DecodeBitmap`
// (`internal/launcher/launcher.go:506`), so an `.svg`/`.xpm` answer would be a
// file `readIcon` rejects — and a rejected answer would shadow the smaller PNG
// the same name has. This is the one deliberate difference from the reference,
// whose `ICON_EXTENSIONS` is `png, svg, xpm`.
func TestOnlyPNGIconsAreReturned(t *testing.T) {
	_, dataHome := fakeXDG(t)

	// A name that exists only as `.svg` — Alacritty's shape on a stock Arch
	// install, where `/usr/share/pixmaps/Alacritty.svg` is the only file.
	iconTree(t, dataHome, "hicolor", "scalable", "apps", "p10s-svg.svg")
	if got := desktopIconFile("p10s-svg"); got != "" {
		t.Errorf("an svg-only name resolved to %q, want nothing", got)
	}

	// The same name with a small PNG must resolve to the PNG: the svg is not
	// a candidate at all, so it cannot shadow it.
	png := iconTree(t, dataHome, "hicolor", "16x16", "apps", "p10s-svg.png")
	if got := desktopIconFile("p10s-svg"); got != png {
		t.Errorf("resolved to %q, want the PNG %q", got, png)
	}

	// The bounded walk obeys the same rule.
	iconTree(t, dataHome, "hicolor", "odd", "p10s-walk-svg.svg")
	if got := desktopIconFile("p10s-walk-svg"); got != "" {
		t.Errorf("the walk returned the svg %q", got)
	}
}
