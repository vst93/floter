package terminalui

import (
	"slices"

	"github.com/egoist/mygo/plugins/terminal"
)

type appearanceTarget interface {
	SetFont(terminal.Font)
	SetTheme(*terminal.Theme, *terminal.Theme)
	SetCopyOnSelect(terminal.CopyOnSelect)
}

// Returning from settings must not reset terminal zoom or redraw every live
// session when its appearance did not change.
type terminalAppearance struct {
	font        terminal.Font
	theme, dark *terminal.Theme
	copy        terminal.CopyOnSelect
	set         bool
}

func appearanceOf(opts terminal.Options) terminalAppearance {
	font := opts.Font
	font.Features = slices.Clone(font.Features)
	return terminalAppearance{font: font, theme: cloneTheme(opts.Theme), dark: cloneTheme(opts.DarkTheme), copy: opts.CopyOnSelect, set: true}
}

func cloneTheme(value *terminal.Theme) *terminal.Theme {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func sameTheme(a, b *terminal.Theme) bool {
	return a == b || (a != nil && b != nil && *a == *b)
}

func sameFont(a, b terminal.Font) bool {
	return a.Family == b.Family && a.Size == b.Size && a.Weight == b.Weight &&
		a.LineHeight == b.LineHeight && a.Thicken == b.Thicken && slices.Equal(a.Features, b.Features)
}

func (previous *terminalAppearance) apply(target appearanceTarget, opts terminal.Options) {
	if !previous.set || !sameFont(previous.font, opts.Font) {
		target.SetFont(opts.Font)
	}
	if !previous.set || !sameTheme(previous.theme, opts.Theme) || !sameTheme(previous.dark, opts.DarkTheme) {
		target.SetTheme(opts.Theme, opts.DarkTheme)
	}
	if !previous.set || previous.copy != opts.CopyOnSelect {
		target.SetCopyOnSelect(opts.CopyOnSelect)
	}
	*previous = appearanceOf(opts)
}
