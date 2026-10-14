package settingsui

import (
	"testing"
	"time"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// A settings change the file refuses is not silent: the page says so over its
// content, and the banner clears itself after the old build's five seconds
// (`useTimedReset(settingsSaveFailed, dismissSaveError, 5000)`).
func TestTheSaveFailureBannerAppearsAndExpires(t *testing.T) {
	// A directory where the settings file should be: every write fails and the
	// store rolls the mutation back, while the page says the file was refused.
	store, _ := settings.OpenStore(t.TempDir())
	a := New(store, Actions{})
	if a.SaveFailedVisible() {
		t.Fatal("the banner showed before anything failed")
	}
	now := time.Unix(1_700_000_000, 0)
	a.Now = func() time.Time { return now }

	tt := render(t, a, 720, 580)
	copy := i18n.For("en").Settings
	if tt.HasText(copy.SaveFailedAlert) {
		t.Fatalf("the banner showed before anything failed: %q", tt.Texts())
	}

	before := store.Snapshot().HideOnBlur
	a.set(func(s *settings.Settings) { s.HideOnBlur = !s.HideOnBlur })
	tt.Frame()
	if !a.SaveFailedVisible() {
		t.Fatal("a refused write showed no banner")
	}
	if store.Snapshot().HideOnBlur != before {
		t.Error("the refused write did not roll back to the committed snapshot")
	}
	if !tt.HasText(copy.SaveFailedAlert) {
		t.Fatalf("the banner is not drawn: %q", tt.Texts())
	}

	// Inside the window it is still up.
	now = now.Add(2 * time.Second)
	tt.Frame()
	if !a.SaveFailedVisible() {
		t.Fatal("the banner left inside its window")
	}

	// Past it, the sentence leaves the page.
	now = now.Add(4 * time.Second)
	tt.Frame()
	if a.SaveFailedVisible() {
		t.Errorf("the banner outlived its window")
	}
	if tt.HasText(copy.SaveFailedAlert) {
		t.Errorf("the banner stayed on screen: %q", tt.Texts())
	}
}

// The banner's own ✕ clears it early, and a write that succeeds never raises
// one.
func TestTheSaveFailureBannerDismisses(t *testing.T) {
	store, _ := settings.OpenStore(t.TempDir())
	a := New(store, Actions{})
	a.Now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	tt := render(t, a, 720, 580)
	copy := i18n.For("en").Settings

	a.set(func(s *settings.Settings) { s.SurfaceResidencySeconds = 42 })
	tt.Frame()
	if !tt.HasText(copy.SaveFailedAlert) {
		t.Fatalf("the banner is not drawn: %q", tt.Texts())
	}
	if err := tt.Click(copy.Close); err != nil {
		t.Fatalf("the dismiss control: %v", err)
	}
	tt.Frame()
	if a.SaveFailedVisible() {
		t.Error("the dismiss control left the banner up")
	}
	if tt.HasText(copy.SaveFailedAlert) {
		t.Errorf("the banner stayed on screen: %q", tt.Texts())
	}

	// The shipped store writes in memory, so a change it accepts says
	// nothing.
	ok := New(newStore(t), Actions{})
	okTT := render(t, ok, 720, 580)
	ok.set(func(s *settings.Settings) { s.SurfaceResidencySeconds = 7 })
	okTT.Frame()
	if ok.SaveFailedVisible() {
		t.Error("a write that succeeded raised the banner")
	}
}
