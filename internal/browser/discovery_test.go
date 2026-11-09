package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func discoveryFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoveryEdgeChannelsAcrossPlatforms(t *testing.T) {
	for _, tc := range []struct{ goos, home, local, id, want string }{
		{"windows", `C:\Users\u`, "", BrowserEdge, `C:\Users\u\AppData\Local\Microsoft\Edge\User Data`},
		{"windows", `C:\Users\u`, `D:\Local`, BrowserEdgeBeta, `D:\Local\Microsoft\Edge Beta\User Data`},
		{"windows", `C:\Users\u`, `D:\Local`, BrowserEdgeDev, `D:\Local\Microsoft\Edge Dev\User Data`},
		{"windows", `C:\Users\u`, `D:\Local`, BrowserEdgeCanary, `D:\Local\Microsoft\Edge SxS\User Data`},
		{"darwin", "/Users/u", "", BrowserEdge, "/Users/u/Library/Application Support/Microsoft Edge"},
		{"darwin", "/Users/u", "", BrowserEdgeCanary, "/Users/u/Library/Application Support/Microsoft Edge Canary"},
		{"linux", "/home/u", "", BrowserEdgeBeta, "/home/u/.config/microsoft-edge-beta"},
		{"linux", "/home/u", "", BrowserEdgeDev, "/home/u/.config/microsoft-edge-dev"},
	} {
		t.Run(tc.goos+"/"+tc.id, func(t *testing.T) {
			for _, location := range chromiumBrowsers(tc.goos, tc.home, tc.local) {
				if location.id == tc.id {
					if location.base != tc.want {
						t.Fatalf("path %q, want %q", location.base, tc.want)
					}
					return
				}
			}
			t.Fatal("browser absent from platform table")
		})
	}
}

func TestDiscoveryNativeEdgeProfilesAndCustomDirectory(t *testing.T) {
	home := t.TempDir()
	local := filepath.Join(home, "AppData", "Local")
	var base string
	for _, location := range chromiumBrowsers(runtime.GOOS, home, local) {
		if location.id == BrowserEdge {
			base = location.base
		}
	}
	if base == "" {
		t.Fatal("missing Edge location")
	}
	discoveryFile(t, filepath.Join(base, "Default", "Preferences"), "{}")
	dir := filepath.Join(base, "Profile 2")
	discoveryFile(t, filepath.Join(dir, "Bookmarks"), `{"roots":{"bookmark_bar":{"children":[{"type":"url","name":"Edge work","url":"https://edge.example/work"}]}}}`)
	discoveryFile(t, filepath.Join(base, "Local State"), `{"profile":{"info_cache":{"Profile 2":{"name":"Work"}}}}`)
	profiles := profilesFor(runtime.GOOS, home, "", local, "")
	selected := SearchProfiles(profiles, BrowserEdge)
	if len(selected) != 2 || selected[1].Name != "Work" {
		t.Fatalf("profiles: %+v", selected)
	}
	if got := readBookmarks(selected[1]); len(got) != 1 || got[0].Title != "Edge work" {
		t.Fatalf("bookmarks: %+v", got)
	}
	custom := profilesFor(runtime.GOOS, t.TempDir(), "", "", dir)
	if len(custom) != 1 || custom[0].BookmarksFile != filepath.Join(dir, "Bookmarks") || !custom[0].HasBookmarks() {
		t.Fatalf("single custom profile resolved wrong path: %+v", custom)
	}
}

func TestDiscoveryLinuxPackagesAndXDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux filesystem fixture")
	}
	home, config := t.TempDir(), t.TempDir()
	for _, path := range []string{
		filepath.Join(config, "microsoft-edge", "Profile 1", "Bookmarks"),
		filepath.Join(home, ".var/app/com.microsoft.Edge/config/microsoft-edge/Default/Bookmarks"),
		filepath.Join(home, "snap/chromium/common/chromium/Default/Bookmarks"),
		filepath.Join(home, "snap/firefox/common/.mozilla/firefox/work/places.sqlite"),
	} {
		discoveryFile(t, path, "{}")
	}
	profiles := profilesFor("linux", home, "", "", "", config)
	counts := map[string]int{}
	for _, p := range profiles {
		counts[p.BrowserID]++
	}
	if counts[BrowserEdge] != 2 || counts[BrowserChromium] != 1 || counts[BrowserFirefox] != 1 {
		t.Fatalf("Linux package profiles: %+v", profiles)
	}
}

func TestDiscoveryFirefoxRegistryAndDeduplication(t *testing.T) {
	root, external := t.TempDir(), t.TempDir()
	discoveryFile(t, filepath.Join(root, "profiles.ini"), "[Profile0]\nIsRelative=1\nPath=Profiles/work\n[Profile1]\nIsRelative=0\nPath="+filepath.ToSlash(external)+"\n")
	dirs := firefoxINIProfiles(root)
	if len(dirs) != 2 || dirs[0] != filepath.Join(root, "Profiles", "work") || dirs[1] != external {
		t.Fatalf("registry: %v", dirs)
	}
	p := Profile{BrowserID: BrowserFirefox, HistoryDB: filepath.Join(external, "places.sqlite")}
	if got := uniqueProfiles([]Profile{p, p}); len(got) != 1 {
		t.Fatalf("duplicate profiles: %+v", got)
	}
}

func TestSearchProfilesIncludesEdgeAlongsideChrome(t *testing.T) {
	profiles := []Profile{{BrowserID: BrowserChrome}, {BrowserID: BrowserEdge, Dir: "Default"}, {BrowserID: BrowserEdge, Dir: "Profile 1"}}
	if got := SearchProfiles(profiles, AutoTarget); len(got) != 3 {
		t.Fatalf("automatic lost browsers: %+v", got)
	}
	if got := SearchProfiles(profiles, BrowserEdge); len(got) != 2 || got[1].Dir != "Profile 1" {
		t.Fatalf("Edge lost signed-in profile: %+v", got)
	}
}
