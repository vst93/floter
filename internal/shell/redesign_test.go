package shell

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"floter/internal/apps"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"floter/internal/terminalui"
	"github.com/egoist/mygo/ui"
)

func redesignFrame(appearance, language string, opacity uint8, surface Surface, page, width, height int, scale ...string) *ui.Tester {
	s := settings.Default()
	s.Theme, s.Language, s.GlassStep, s.MainOpacity = appearance, language, "liquid", opacity
	if len(scale) > 0 {
		s.UIScale = scale[0]
	}
	a := New(Options{Store: settings.NewStore(s)})
	a.Surf, a.Settings.Page = surface, page
	found := []apps.App{
		{Name: "Visual Studio Code", Path: "/Applications/Code.app", Comment: "Code editor"},
		{Name: "Safari", Path: "/Applications/Safari.app", Comment: "Web browser"},
		{Name: "Terminal", Path: "/Applications/Terminal.app", Comment: "Command line"},
		{Name: "Notes", Path: "/Applications/Notes.app", Comment: "Notes and ideas"},
	}
	paths := make([]string, len(found))
	for i, app := range found {
		paths[i] = app.Path
	}
	a.Launcher.SetApps(found)
	a.Launcher.SetRecent(paths, true)
	for i, title := range []string{"floter · go test ./...", "Website · pnpm dev", "Workspace · zsh"} {
		a.Terminal.Sessions = append(a.Terminal.Sessions, &terminalui.Session{
			ID: uint64(i + 1), Title: title, Created: time.Date(2026, 11, 9, 13, i*5, 0, 0, time.Local),
		})
	}
	if surface == SurfaceTerminal {
		a.Terminal.Drawer = true
	}
	tt := ui.NewTester(a.View, width, height)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

// Render the modifier gesture at the actual content height, including its
// unchanged height over a query with an existing footer.
func TestUIPolishPreview(t *testing.T) {
	dir := os.Getenv("FLOTER_UI_POLISH_PNG")
	if dir == "" {
		t.Skip("set FLOTER_UI_POLISH_PNG to render modifier states")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, appearance := range []string{"light", "dark"} {
		for _, query := range []string{"", "Recent"} {
			s := settings.Default()
			s.Theme, s.Language = appearance, "zh"
			a := New(Options{Store: settings.NewStore(s)})
			recentLauncher(a, 5)
			a.Launcher.SetQuery(query)
			tt := ui.NewTester(a.View, 720, 580)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			for _, held := range []bool{false, true} {
				if held {
					tt.HoldModifiers(ui.Cmd)
				}
				tt.SetSize(720, a.launcherTarget)
				tt.Frame()
				tt.Frame()
				name := fmt.Sprintf("launcher-%s-query-%t-super-%t.png", appearance, query != "", held)
				f, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				encodeErr, closeErr := png.Encode(f, tt.Image()), f.Close()
				if encodeErr != nil || closeErr != nil {
					t.Fatalf("write screenshot: %v / %v", encodeErr, closeErr)
				}
			}
		}
	}
}

// Reproduce the full recents list at its measured window height. Installed
// artwork is read only for this opt-in preview; no application is launched.
func TestLauncherProportionPreview(t *testing.T) {
	dir := os.Getenv("FLOTER_LAUNCHER_PNG")
	if dir == "" {
		t.Skip("set FLOTER_LAUNCHER_PNG to render the full launcher")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	names := []string{"DBX", "bili-FM", "Multica", "OpenVPN Connect", "CC Switch", "Code", "Clash Verge", "Sikarugir Creator", "Apifox", "企业微信"}
	installed := map[string]apps.App{}
	if runtime.GOOS == "darwin" {
		for _, app := range apps.ScanDarwin("/Applications") {
			installed[app.Name], installed[app.Localized] = app, app
		}
	}
	for _, appearance := range []string{"light", "dark"} {
		for _, subtitles := range []bool{false, true} {
			s := settings.Default()
			s.Theme, s.Language, s.UIScale, s.GlassStep, s.MainOpacity = appearance, "zh", "default", "liquid", 47
			a := New(Options{Store: settings.NewStore(s)})
			var found []apps.App
			var paths []string
			for i, name := range names {
				app, ok := installed[name]
				if !ok {
					app = apps.App{Path: fmt.Sprintf("/preview/application-%d.app", i)}
				}
				app.Name, app.Localized = name, ""
				if subtitles && i%2 == 0 {
					app.Comment = "应用程序 · Applications"
				}
				apps.Icon(app)
				found, paths = append(found, app), append(paths, app.Path)
			}
			deadline := time.Now().Add(5 * time.Second)
			for _, app := range found {
				for apps.IconPending(app) && time.Now().Before(deadline) {
					time.Sleep(10 * time.Millisecond)
				}
			}
			a.Launcher.SetApps(found)
			a.Launcher.SetRecent(paths, true)
			tt := ui.NewTester(a.View, 720, 720)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.SetScale(2)
			tt.Frame()
			mods := ui.Ctrl
			if runtime.GOOS == "darwin" {
				mods = ui.Cmd
			}
			tt.HoldModifiers(mods)
			tt.Frame()
			height := a.launcherHeight(s)
			tt.SetSize(720, height)
			tt.Frame()
			tt.Frame()
			name := fmt.Sprintf("launcher-%s-subtitles-%t.png", appearance, subtitles)
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatalf("write preview: %v / %v", err, closeErr)
			}
			t.Logf("%s: 720 × %d DIP", name, height)
		}
	}
}

// Read the rendered page, including its cards and scroll edges. Changing the
// opacity setting must reach the content, not just the window's outer margin.
func TestSettingsGlassFollowsOpacity(t *testing.T) {
	for _, appearance := range []string{"light", "dark"} {
		for page := settingsui.PageGeneral; page <= settingsui.PageAbout; page++ {
			t.Run(fmt.Sprintf("%s/%s", appearance, settingsui.PageName(page)), func(t *testing.T) {
				var medians []int
				for _, opacity := range []uint8{74, 86, 98} {
					tt := redesignFrame(appearance, "en", opacity, SurfaceSettings, page, 720, 580)
					img := tt.Image()
					var alphas []int
					for y := 100; y < 550; y += 3 {
						for x := 190; x < 690; x += 3 {
							alphas = append(alphas, int(img.RGBAAt(x, y).A))
						}
					}
					sort.Ints(alphas)
					medians = append(medians, alphas[len(alphas)/2])
				}
				if medians[0] > 195 || medians[1]-medians[0] < 25 || medians[2]-medians[1] < 25 {
					t.Fatalf("settings content covers the glass: median alpha at 74%%/86%%/98%% = %v", medians)
				}
			})
		}
	}
}

// FLOTER_REDESIGN_PNG points to an optional review-artifact directory.
func TestRedesignPreview(t *testing.T) {
	dir := os.Getenv("FLOTER_REDESIGN_PNG")
	if dir == "" {
		t.Skip("set FLOTER_REDESIGN_PNG to render the surface matrix")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, appearance := range []string{"light", "dark"} {
		for _, size := range [][2]int{{720, 580}, {620, 420}} {
			for page := -2; page <= settingsui.PageAbout; page++ {
				surface, name := SurfaceSettings, settingsui.PageName(page)
				if page == -2 {
					surface, name = SurfaceLauncher, "launcher"
				} else if page == -1 {
					surface, name = SurfaceTerminal, "terminal"
				}
				tt := redesignFrame(appearance, "zh", 47, surface, page, size[0], size[1])
				path := filepath.Join(dir, fmt.Sprintf("%s-%s-%dx%d.png", name, appearance, size[0], size[1]))
				f, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, tt.Image())
				closeErr := f.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("write %s: %v / %v", path, err, closeErr)
				}
			}
		}
	}
}

// Review the settings at the standard scale used in the reported screenshot.
func TestSettingsProportionPreview(t *testing.T) {
	dir := os.Getenv("FLOTER_SETTINGS_PNG")
	if dir == "" {
		t.Skip("set FLOTER_SETTINGS_PNG to render settings proportions")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	language, scale := os.Getenv("FLOTER_SETTINGS_LANGUAGE"), os.Getenv("FLOTER_SETTINGS_SCALE")
	if language == "" {
		language = "zh"
	}
	if scale == "" {
		scale = "default"
	}
	for _, appearance := range []string{"light", "dark"} {
		for page := settingsui.PageGeneral; page <= settingsui.PageAbout; page++ {
			for _, width := range []int{720, 620} {
				tt := redesignFrame(appearance, language, 47, SurfaceSettings, page, width, 580, scale)
				tt.SetScale(2)
				tt.Frame()
				if os.Getenv("FLOTER_SETTINGS_BOTTOM") != "" {
					tt.Wheel(float32(width)-100, 300, 0, 10000)
					tt.Advance(time.Second)
					tt.Frame()
				}
				name := fmt.Sprintf("%s-%s-%d.png", settingsui.PageName(page), appearance, width)
				f, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, tt.Image())
				closeErr := f.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("write preview: %v / %v", err, closeErr)
				}
			}
		}
	}
}
