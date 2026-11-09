//go:build darwin

package shell

import (
	"fmt"
	"os"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// Native material checks need AppKit's main loop. Ordinary tests keep using
// the headless renderer and never create a desktop window.
func TestMain(m *testing.M) {
	if os.Getenv("FLOTER_NATIVE_GLASS_TEST") != "1" {
		os.Exit(m.Run())
	}
	mygo.App.OnWindowAllClosed(func() {})
	code := 1
	mygo.App.WhenReady(func() {
		go func() {
			code = m.Run()
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

// Inspect the real NSWindow and its backing views, not a PNG composited over
// a neutral background. The test uses an isolated store and a hidden window.
func TestNativeGlassBackground(t *testing.T) {
	if os.Getenv("FLOTER_NATIVE_GLASS_TEST") != "1" {
		t.Skip("set FLOTER_NATIVE_GLASS_TEST=1 in a macOS desktop session")
	}
	s := settings.Default()
	a := New(Options{Store: settings.NewStore(s)})
	a.Surf = SurfaceSettings
	a.Win = mygo.NewWindow(mygo.WindowOptions{
		Title: "Floter glass verification", Width: 720, Height: 580,
		Hidden: true, Frameless: true, Panel: true, Transparent: true,
		CornerRadius: 16, Vibrancy: windowVibrancyFor("darwin", s.GlassStep),
		Content: ui.View(a.View),
	})
	defer a.Win.Close()
	for _, step := range []string{"regular", "off", "frosted", "liquid", "regular"} {
		a.applyWindowMaterial(step)
		mygo.RunOnMain(func() {
			win := objc.ID(a.Win.NativeHandle())
			view := win.Send(objc.RegisterName("contentView"))
			background := win.Send(objc.RegisterName("backgroundColor"))
			alpha := objc.Send[float64](background, objc.RegisterName("alphaComponent"))
			if win.Send(objc.RegisterName("isOpaque")) != 0 || alpha != 0 {
				t.Errorf("%s: NSWindow covers the desktop: background alpha %.2f", step, alpha)
			}
			layer := view.Send(objc.RegisterName("layer"))
			if color := layer.Send(objc.RegisterName("backgroundColor")); color != 0 {
				nsColor := objc.ID(objc.GetClass("NSColor")).Send(objc.RegisterName("colorWithCGColor:"), color)
				if a := objc.Send[float64](nsColor, objc.RegisterName("alphaComponent")); a != 0 {
					t.Errorf("%s: root content layer adds a solid backing: alpha %.2f", step, a)
				}
			}
			children := view.Send(objc.RegisterName("subviews"))
			var effect objc.ID
			glassClass := objc.GetClass("NSGlassEffectView")
			for i, n := uintptr(0), uintptr(children.Send(objc.RegisterName("count"))); i < n; i++ {
				child := children.Send(objc.RegisterName("objectAtIndex:"), i)
				if child.Send(objc.RegisterName("isKindOfClass:"), objc.GetClass("NSVisualEffectView")) != 0 ||
					(glassClass != 0 && child.Send(objc.RegisterName("isKindOfClass:"), glassClass) != 0) {
					effect = child
				}
			}
			if step == "off" {
				if effect != 0 {
					t.Error("off: native backdrop was not removed")
				}
				return
			}
			if effect == 0 {
				t.Errorf("%s: missing native material", step)
				return
			}
			if step == "frosted" {
				return
			}
			if glassClass != 0 {
				if effect.Send(objc.RegisterName("isKindOfClass:"), glassClass) == 0 {
					t.Errorf("%s: liquid glass still uses an NSVisualEffectView backing", step)
					return
				}
				if style := effect.Send(objc.RegisterName("style")); style != 1 {
					t.Errorf("%s: glass style %d, want the transparent style", step, style)
				}
				activeSelector := objc.RegisterName("_hasActiveAppearance")
				if win.Send(objc.RegisterName("respondsToSelector:"), activeSelector) != 0 && win.Send(activeSelector) == 0 {
					t.Errorf("%s: floating panel lets macOS frost the inactive glass", step)
				}
			} else if material := effect.Send(objc.RegisterName("material")); material != 13 {
				t.Errorf("%s: pre-macOS 26 fallback material %d, want HUD", step, material)
			}
			t.Logf("%s: transparent window and root, clear native glass, active appearance", step)
		})
	}
}
