package shell

import (
	"testing"

	"github.com/egoist/mygo"

	"floter/internal/settings"
	"floter/internal/settingsui"
)

// progressApp builds a shell whose system progress surface is recorded instead
// of a window's, the same way the notification seam is replaced in tests.
func progressApp(t *testing.T) (*App, *[]mygo.ProgressBar) {
	t.Helper()
	var bars []mygo.ProgressBar
	a := New(Options{
		Store:          settings.NewStore(settings.Default()),
		SetProgressBar: func(p mygo.ProgressBar) { bars = append(bars, p) },
	})
	return a, &bars
}

func lastBar(bars []mygo.ProgressBar) mygo.ProgressBar {
	if len(bars) == 0 {
		return mygo.ProgressBar{}
	}
	return bars[len(bars)-1]
}

// TestTheUpdateProgressReachesTheSystemSurface: the About page has drawn the
// download's progress since P7-*, but the system's own surface (the taskbar
// button, the Dock icon, the launcher entry) never heard of it. Every update
// state write goes through setUpdate, so that is where the bar is derived:
// running → the same fraction the page draws, no total → zero, done or failed
// → cleared. The clear is what keeps a finished download from leaving a bar
// pinned at 100% under the icon.
func TestTheUpdateProgressReachesTheSystemSurface(t *testing.T) {
	a, bars := progressApp(t)

	a.setUpdate(func(u *settingsui.UpdateState) {
		u.Downloading, u.Progress = true, &settingsui.UpdateProgress{Downloaded: 1, Total: 4}
	})
	if got := lastBar(*bars); got.Value != 0.25 || got.State != "" {
		t.Errorf("running: progress = %+v, want value 0.25 with the default state", got)
	}

	// A feed that names no total has no fraction; the page draws a bare
	// byte count and the system bar has nothing to say.
	a.setUpdate(func(u *settingsui.UpdateState) {
		u.Progress = &settingsui.UpdateProgress{Downloaded: 3, Total: 0}
	})
	if got := lastBar(*bars); got.Value != 0 {
		t.Errorf("no total: progress = %+v, want value 0", got)
	}

	// More bytes than the feed expected is a finished bar, not an overrun.
	a.setUpdate(func(u *settingsui.UpdateState) {
		u.Progress = &settingsui.UpdateProgress{Downloaded: 9, Total: 4}
	})
	if got := lastBar(*bars); got.Value != 1 {
		t.Errorf("over the total: progress = %+v, want value 1", got)
	}

	// Not downloading wins over a stale Progress: the bar is about the
	// running download, not about the last bytes it saw.
	a.setUpdate(func(u *settingsui.UpdateState) {
		u.Downloading = false
		u.Progress = &settingsui.UpdateProgress{Downloaded: 1, Total: 4}
	})
	if got := lastBar(*bars); got.Value != 0 {
		t.Errorf("not downloading: progress = %+v, want value 0", got)
	}

	// Finished: the state keeps no progress and the system bar is cleared.
	a.setUpdate(func(u *settingsui.UpdateState) {
		u.Downloading, u.Progress = false, nil
	})
	if got := lastBar(*bars); got != (mygo.ProgressBar{}) {
		t.Errorf("finished: progress = %+v, want the zero value", got)
	}
}

// TestTheFailedUpdateClearsTheSystemBar: failure is the other exit, and it
// leaves Progress nil too; the bar must not survive as a red or pinned bar
// under the icon after the download is gone.
func TestTheFailedUpdateClearsTheSystemBar(t *testing.T) {
	a, bars := progressApp(t)
	a.setUpdate(func(u *settingsui.UpdateState) {
		u.Downloading, u.Progress = true, &settingsui.UpdateProgress{Downloaded: 1, Total: 2}
	})
	if got := lastBar(*bars); got.Value != 0.5 {
		t.Fatalf("running: progress = %+v, want value 0.5", got)
	}
	a.failUpdate("boom: %v", nil)
	if got := lastBar(*bars); got != (mygo.ProgressBar{}) {
		t.Errorf("failed: progress = %+v, want the zero value", got)
	}
}

// TestAClickedNotificationRevealsTheLauncher: a completion notification is
// raised only while the panel is hidden, and the run's output lives on the
// launcher. A click therefore returns to that surface, not to the page the
// hidden panel was left on.
func TestAClickedNotificationRevealsTheLauncher(t *testing.T) {
	a := newApp(t)
	a.onScreen = func() bool { return false }
	a.Surf = SurfaceSettings

	a.NotificationClicked("completion")

	if a.Surf != SurfaceLauncher {
		t.Errorf("surface = %v, want the launcher that holds the run's output", a.Surf.Name())
	}
}

// TestAClickedNotificationLeavesAVisiblePanelAlone: the notification was
// stale by the time the user reopened the panel and moved to another page;
// clicking it must not yank them off that page.
func TestAClickedNotificationLeavesAVisiblePanelAlone(t *testing.T) {
	a := newApp(t)
	a.onScreen = func() bool { return true }
	a.Surf = SurfaceSettings

	a.NotificationClicked("completion")

	if a.Surf != SurfaceSettings {
		t.Errorf("surface = %v, want the settings page the user is reading", a.Surf.Name())
	}
}

// TestAClickThatLaunchedTheAppIsHarmless: the click may arrive before the
// window exists (macOS delivers the click that launched the app before Run).
// There is no panel to move yet; the launch itself opens one, so the handler
// must not panic and must leave the default surface.
func TestAClickThatLaunchedTheAppIsHarmless(t *testing.T) {
	a := newApp(t) // Win is nil, as before Start
	a.Surf = SurfaceLauncher

	a.NotificationClicked("completion")

	if a.Surf != SurfaceLauncher {
		t.Errorf("surface = %v, want the default launcher", a.Surf.Name())
	}
}
