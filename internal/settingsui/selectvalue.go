package settingsui

import (
	"floter/internal/theme"
	"github.com/egoist/mygo/ui"
	"slices"
)

type selectHighlight[T comparable] struct {
	values   []T
	value    T
	valid    bool
	selected T
}

type selectLifetime[T comparable] struct {
	values     []T
	selected   T
	enabled    bool
	generation uint64
	open       bool
	trigger    ui.Handle
}

// valueSelect keeps the framework's Select geometry and keyboard model while
// separating a choice's identity from its display name. Labels need not be
// unique; an unavailable stored value remains visible until explicitly replaced.
func valueSelect[T comparable](c *ui.Context, selected *T, labels []string, values []T, fallback string, tokens theme.Tokens, enabled bool) ui.Element {
	t := c.Theme()
	// Distinct names may share an ID in an external schema. They are still
	// one action; preserve the first name and the caller's slices.
	seen := make(map[T]bool, len(values))
	uniqueValues := make([]T, 0, len(values))
	uniqueLabels := make([]string, 0, len(labels))
	for i, value := range values {
		if !seen[value] {
			seen[value] = true
			uniqueValues = append(uniqueValues, value)
			uniqueLabels = append(uniqueLabels, labels[i])
		}
	}
	values, labels = uniqueValues, uniqueLabels
	// Keep the trigger's lifetime separate from the selected value. A
	// non-layout marker remembers invalidations without wrapping the field
	// or changing its sizing. Only stale/unavailable menus get a new identity;
	// explicitly replacing an unknown value keeps the same keyboard target.
	marker := ui.Box(c).Absolute().Size(0, 0).PassThrough()
	lifetime := ui.Local(marker, "select.lifetime", func() selectLifetime[T] { return selectLifetime[T]{} })
	unknownChanged := !slices.Contains(values, *selected) &&
		(lifetime.selected != *selected || !slices.Equal(lifetime.values, values))
	// An already closed select has no stale menu to dismiss. Carry its
	// keyboard target across invalidation without reopening it or changing
	// the value. An open invalidated menu keeps its existing dismissal path.
	keepFocus := enabled && unknownChanged && lifetime.trigger.Focused(c)
	if unknownChanged || lifetime.enabled != enabled {
		lifetime.generation++
	}
	identity := struct {
		owner      *selectLifetime[T]
		generation uint64
	}{lifetime, lifetime.generation}
	sel := ui.SelectBase(c.Key(identity), selected)
	b := sel.Trigger
	b.Bind(&lifetime.trigger)
	if keepFocus {
		lifetime.trigger.Focus(c)
	}
	if !enabled {
		b = lockedControl(b)
	}
	highlight := ui.Local(b, "floter.highlight", func() selectHighlight[T] { return selectHighlight[T]{} })
	external := highlight.valid && highlight.selected != *selected && !b.Changed()
	reordered := highlight.valid && (!slices.Equal(highlight.values, values) || external)
	target := highlight.value
	if external {
		target = *selected
	}
	if reordered && !slices.Contains(values, target) {
		target = *selected
		if !slices.Contains(values, target) && len(values) > 0 {
			target = values[0]
		}
	}
	var current T
	hasCurrent := false
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
	b.Value(display)
	b.Children(func() {
		ui.Text(c, display).SingleLine()
		ui.Box(c).Size(t.Space(2.5), t.Space(2.5)).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
			var path ui.Path
			path.MoveTo(r.X+r.W*0.1, r.Y+r.H*0.3).LineTo(r.X+r.W*0.5, r.Y+r.H*0.7).LineTo(r.X+r.W*0.9, r.Y+r.H*0.3)
			p.StrokePath(&path, 1.5, t.TextMuted)
		})
	})
	if enabled {
		sel.Popup(func(panel ui.Element) {
			panel.Margin(t.Space(1), 0, 0, 0).Padding(t.Space(1)).Radius(t.Radius+2).
				Background(t.Background).Border(1, t.Border).Shadow(0, 6, 20, 0, tokens.ShadowAmbient)
			anchor, frame := b.Bounds(), c.Root().Bounds()
			room := max(anchor.Y-frame.Y, frame.Y+frame.H-anchor.Y-anchor.H)
			// The popup itself is window-clamped by SelectBase; its inner
			// scroller also needs a bound so ScrollIntoView can move the list.
			ui.Scroll(c).MaxHeight(max(0, room-t.Space(3)-2)).Shrink(1).Children(func() {
				for i, value := range values {
					item := sel.Item(value).Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius)
					switch {
					case item.Highlighted():
						current, hasCurrent = value, true
						item.Background(t.Accent).TextColor(t.AccentText)
					case value == *selected:
						item.Background(t.Surface)
					}
					item.Children(func() { ui.Text(c, labels[i]).SingleLine() })
				}
			})
		})
	}
	// SelectBase stores an index. Reapply the previously highlighted value
	// after Popup has published this frame's order, before the next input.
	if reordered && sel.Open() {
		sel.Highlight(target)
		current, hasCurrent = target, true
	}
	highlight.values = append(highlight.values[:0], values...)
	highlight.value, highlight.valid = current, hasCurrent
	highlight.selected = *selected
	lifetime.values = append(lifetime.values[:0], values...)
	lifetime.selected, lifetime.enabled = *selected, enabled
	lifetime.open = sel.Open()
	return b
}
