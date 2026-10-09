package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// BenchmarkGeneralFrame measures one frame of the settings surface's General
// page, the surface with the most controls.
func BenchmarkGeneralFrame(b *testing.B) {
	a := New(newStoreFor(b), Actions{})
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
	tt.Frame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Frame()
	}
}

// BenchmarkIntegrationsFrame measures the Integrations page with a full list
// of integrations and their command switches.
func BenchmarkIntegrationsFrame(b *testing.B) {
	a := New(newStoreFor(b), Actions{
		SetIntegrationEnabled: func(string, bool) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	a.Integrations = func() []Integration {
		out := make([]Integration, 0, 20)
		for i := 0; i < 20; i++ {
			commands := make([]Command, 0, 5)
			for j := 0; j < 5; j++ {
				commands = append(commands, Command{ID: "c", Name: "Command", Enabled: true, Available: true})
			}
			out = append(out, Integration{
				ID: "ext", Name: "Integration", Enabled: true, Running: true,
				Permissions: []string{"filesystem-read", "environment"},
				Enforced:    map[string]bool{"environment": true},
				Commands:    commands,
			})
		}
		return out
	}
	a.Page = PageIntegrations
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
	tt.Frame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Frame()
	}
}
