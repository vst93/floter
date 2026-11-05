package settingsui

import (
	"floter/internal/theme"
	"github.com/egoist/mygo/ui"
)

// valueSelect keeps the framework's Select geometry and keyboard model while
// separating a choice's identity from its display name. Labels need not be
// unique; an unavailable stored value remains visible until explicitly replaced.
func valueSelect[T comparable](c *ui.Context, selected *T, labels []string, values []T, fallback string, tokens theme.Tokens) ui.Element {
	t := c.Theme()
	sel := ui.SelectBase(c, selected)
	b := sel.Trigger
	b.Padding(t.Space(1.5), t.Space(3.5)).Gap(t.Space(1.5)).Radius(t.Radius).
		Background(t.Surface).TextColor(t.Text).Border(1, t.Border).
		Justify(ui.SpaceBetween).MinWidth(t.Space(35))
	display := fallback
	for i, value := range values {
		if value == *selected {
			display = labels[i]
			break
		}
	}
	b.Children(func() {
		ui.Text(c, display).SingleLine()
		ui.Box(c).Size(t.Space(2.5), t.Space(2.5)).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
			var path ui.Path
			path.MoveTo(r.X+r.W*0.1, r.Y+r.H*0.3).LineTo(r.X+r.W*0.5, r.Y+r.H*0.7).LineTo(r.X+r.W*0.9, r.Y+r.H*0.3)
			p.StrokePath(&path, 1.5, t.TextMuted)
		})
	})
	sel.Popup(func(panel ui.Element) {
		panel.Margin(t.Space(1), 0, 0, 0).Padding(t.Space(1)).Radius(t.Radius+2).
			Background(t.Background).Border(1, t.Border).Shadow(0, 6, 20, 0, tokens.ShadowAmbient)
		ui.Scroll(c).Shrink(1).Children(func() {
			for i, value := range values {
				item := sel.Item(value).Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius)
				switch {
				case item.Highlighted():
					item.Background(t.Accent).TextColor(t.AccentText)
				case value == *selected:
					item.Background(t.Surface)
				}
				item.Children(func() { ui.Text(c, labels[i]).SingleLine() })
			}
		})
	})
	return b
}
