package settingsui

import (
	"testing"

	"floter/internal/settings"
)

func newTestApp() *App {
	return New(settings.NewStore(settings.Default()), Actions{})
}

// The sidebar's rows answer a click: the page switches, and the click does
// not fall through whatever holds the focus.
func TestClickingASidebarRowSwitchesThePage(t *testing.T) {
	a := newTestApp()
	tt := render(t, a, 720, 560)
	tt.Frame()
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("click: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if a.Page != PageIntegrations {
		t.Errorf("page = %v, want integrations", a.Page)
	}
	// And again to another page.
	if err := tt.Click("Plugins"); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if a.Page != PagePlugins {
		t.Errorf("page = %v, want plugins", a.Page)
	}
}
