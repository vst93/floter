package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The settings panel's keyboard home is the sidebar. The shell asks for it as
// the surface opens (the reference's `focusCurrentSettingsSidebar`,
// `main:src/App.tsx:1718`), but on the very first open the list has not been
// built yet, so View delivers the request itself once it has. The focus is
// marked as *entry* focus: the active pill already says where the keyboard is,
// so the list's ring is withheld until the first real interaction — the
// reference's `data-entry-focus` mark, dropped on the nav's
// `keydown`/`pointerdown`/`blur` (R152, `main:src/App.tsx:2899-2901` and
// `main:src/styles/settings.css:185`).
func TestTheSettingsPanelTakesTheSidebarAsItsKeyboardHome(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 580)
	if !tt.Focused("Settings") {
		t.Fatal("the panel opened with the keyboard off the sidebar")
	}
	if !a.SidebarEntryFocus() {
		t.Fatal("the entry focus did not withhold the sidebar's ring")
	}

	// A key that drives the sidebar is the first interaction: the mark goes.
	a.stepPage(1)
	if a.Page != PageSessions {
		t.Fatalf("stepPage did not move to the sessions page: page = %d", a.Page)
	}
	tt.Frame()
	if a.SidebarEntryFocus() {
		t.Error("the entry-focus mark outlived a sidebar key")
	}

	// A press on a sidebar item drops it too.
	a.FocusSidebar()
	tt.Frame()
	if !a.SidebarEntryFocus() {
		t.Fatal("a fresh entry did not arm the mark")
	}
	if err := tt.Click("About"); err != nil {
		t.Fatalf("the About sidebar item: %v", err)
	}
	tt.Frame()
	if a.Page != PageAbout {
		t.Fatalf("the press did not change the page: page = %d", a.Page)
	}
	if a.SidebarEntryFocus() {
		t.Error("the entry-focus mark outlived a sidebar press")
	}

	// And the keyboard leaving the sidebar drops it, as the reference's
	// `onBlur` on the nav did.
	a.FocusSidebar()
	tt.Frame()
	if !a.SidebarEntryFocus() {
		t.Fatal("the re-armed mark did not hold")
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if tt.Focused("Settings") {
		t.Fatal("Tab did not move the keyboard off the sidebar")
	}
	if a.SidebarEntryFocus() {
		t.Error("the entry-focus mark outlived the keyboard leaving the sidebar")
	}
}
