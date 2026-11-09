package browser

import (
	"os"
	"path/filepath"
	"strings"

	"floter/internal/pathid"
	"floter/internal/textlines"
)

// browserLocations includes native installs and the common Linux packages.
// Environment paths are arguments so all platform tables can be tested.
func browserLocations(goos, home, appData, localData string, configHome ...string) []chromiumBrowser {
	locations := chromiumBrowsers(goos, home, localData)
	if goos == "windows" {
		if appData == "" {
			appData = joinFor(goos, home, "AppData", "Roaming")
		}
		locations = append(locations, chromiumBrowser{BrowserOpera, "Opera", joinFor(goos, appData, "Opera Software", "Opera Stable")})
	}
	if goos != "linux" {
		return locations
	}
	if len(configHome) > 0 && filepath.IsAbs(configHome[0]) {
		standard := joinFor(goos, home, ".config")
		for i := range locations {
			locations[i].base = configHome[0] + strings.TrimPrefix(locations[i].base, standard)
		}
	}
	for _, entry := range []struct{ id, name, app, folder string }{
		{BrowserChrome, "Google Chrome", "com.google.Chrome", "google-chrome"},
		{BrowserEdge, "Microsoft Edge", "com.microsoft.Edge", "microsoft-edge"},
		{BrowserBrave, "Brave", "com.brave.Browser", "BraveSoftware/Brave-Browser"},
		{BrowserChromium, "Chromium", "org.chromium.Chromium", "chromium"},
		{BrowserVivaldi, "Vivaldi", "com.vivaldi.Vivaldi", "vivaldi"},
		{BrowserOpera, "Opera", "com.opera.Opera", "opera"},
	} {
		locations = append(locations, chromiumBrowser{entry.id, entry.name, filepath.Join(home, ".var", "app", entry.app, "config", entry.folder)})
	}
	locations = append(locations,
		chromiumBrowser{BrowserChromium, "Chromium", filepath.Join(home, "snap", "chromium", "common", "chromium")},
		chromiumBrowser{BrowserBrave, "Brave", filepath.Join(home, "snap", "brave", "current", ".config", "BraveSoftware", "Brave-Browser")},
	)
	return locations
}

func firefoxDirectories(goos, home, appData string) []string {
	root := firefoxRoot(goos, home, appData)
	roots := []string{root}
	if goos == "linux" {
		roots = append(roots,
			filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox"),
			filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", "firefox"))
	}
	var dirs []string
	for _, root := range roots {
		dirs = append(dirs, subdirectories(root)...)
		registry := root
		if goos == "windows" || goos == "darwin" {
			registry = filepath.Dir(root)
		}
		dirs = append(dirs, firefoxINIProfiles(registry)...)
	}
	return dirs
}

// profiles.ini may point outside Profiles, including a different drive.
func firefoxINIProfiles(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, "profiles.ini"))
	if err != nil {
		return nil
	}
	var dirs []string
	section, path, relative := "", "", true
	appendProfile := func() {
		if !strings.HasPrefix(section, "Profile") || path == "" {
			return
		}
		dir := filepath.FromSlash(path)
		if relative {
			dir = filepath.Join(root, dir)
		}
		dirs = append(dirs, dir)
	}
	for _, raw := range textlines.Split(string(data)) {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			appendProfile()
			section, path, relative = strings.Trim(line, "[]"), "", true
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Path":
			path = strings.TrimSpace(value)
		case "IsRelative":
			relative = strings.TrimSpace(value) != "0"
		}
	}
	appendProfile()
	return dirs
}

func uniqueProfiles(profiles []Profile) []Profile {
	seen := map[string]bool{}
	out := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		path := filepath.Dir(profile.HistoryDB)
		if profile.HistoryDB == "" {
			path = filepath.Dir(profile.BookmarksFile)
		}
		keys := pathid.Keys(path)
		duplicate := false
		for _, key := range keys {
			duplicate = duplicate || seen[profile.BrowserID+"\x00"+key]
		}
		if duplicate {
			continue
		}
		for _, key := range keys {
			seen[profile.BrowserID+"\x00"+key] = true
		}
		out = append(out, profile)
	}
	return out
}
