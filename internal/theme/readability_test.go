package theme

import (
	"fmt"
	"testing"

	"floter/internal/glassmap"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

// Exercise the public tokens and the same compensated two-layer frame the
// shell paints, including nested control fills. Native blur is not assumed.
func TestTextSurvivesBlackAndWhiteDesktops(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, app := range []string{"light", "dark"} {
			for _, step := range []string{"off", "frosted", "regular", "liquid"} {
				for _, opacity := range []uint8{72, 80, 86, 92, 100} {
					for _, hc := range []bool{false, true} {
						s := settings.Default()
						s.Theme, s.GlassStep, s.MainOpacity = app, step, opacity
						tok := ForPlatform(s, false, ui.Preferences{HighContrast: hc}, goos)
						spec := glassmap.SpecForPreferencesOS(goos, step, opacity, hc)
						for _, desktop := range []ui.Color{ui.RGB(0, 0, 0), ui.RGB(255, 255, 255)} {
							for _, focus := range []bool{false, true} {
								top, bot := tok.FaceFillAt(spec.TintAlpha, focus)
								st, sb := tok.SheenAt(spec.TintAlpha)
								if focus {
									st, sb = tok.SheenAtFocus(spec.TintAlpha)
								}
								for _, frame := range []ui.Color{st.Over(top.Over(desktop)), sb.Over(bot.Over(desktop))} {
									pane := tok.Sunken.Over(frame)
									for name, bg := range map[string]ui.Color{
										"frame": frame, "hover": tok.Theme.SurfaceHover.Over(frame),
										"row": tok.RowSelected.Over(pane), "pressed": tok.RowPressed.Over(pane),
										"thumb": tok.Thumb.Over(tok.Control.Over(pane)), "selection": tok.Theme.Selection.Over(pane),
									} {
										for inkName, ink := range map[string]ui.Color{"body": tok.Theme.Text, "secondary": tok.TextSecondary, "hint": tok.Theme.TextMuted, "tertiary": tok.TextTertiary, "accent": tok.Theme.Accent, "warning": tok.Theme.Warning} {
											if ratio := ContrastRatio(ink.Over(bg), bg); ratio < 4.5 {
												t.Fatalf("%s/%s/%s/%d contrast=%t desktop=%d focus=%t %s %s %v on %v: %.2f:1", goos, app, step, opacity, hc, desktop.R, focus, name, inkName, ink, bg, ratio)
											}
										}
									}
								}
							}
						}
						for _, fill := range []ui.Color{tok.Theme.Accent, tok.Theme.AccentHover, tok.Theme.AccentPressed} {
							if ratio := ContrastRatio(tok.Theme.AccentText, fill); ratio < 4.5 {
								t.Fatal(fmt.Sprintf("%s %s %d accent button %.2f", goos, app, opacity, ratio))
							}
						}
					}
				}
			}
		}
	}
}
