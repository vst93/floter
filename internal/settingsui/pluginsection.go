package settingsui

import (
	"slices"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

// Group titles and positions are presentation, not editor lifetimes. Reuse
// the group containing the focused field first, then match other groups by
// their surviving fields. This also keeps history when a group splits/merges.
type pluginSectionIdentity struct{ fields []string }

func (a *App) pluginEditorRef(key string) *ui.Handle {
	if a.pluginEditorRefs == nil {
		a.pluginEditorRefs = map[string]*ui.Handle{}
	}
	if a.pluginEditorRefs[key] == nil {
		a.pluginEditorRefs[key] = &ui.Handle{}
	}
	return a.pluginEditorRefs[key]
}

func (a *App) pluginSectionIDs(c *ui.Context, sections []plugincfg.Section) []*pluginSectionIdentity {
	keys := make([][]string, len(sections))
	focused := ""
	for i, section := range sections {
		for _, field := range section.Fields {
			keys[i] = append(keys[i], field.Key)
			if ref := a.pluginEditorRefs[field.Key]; ref != nil && ref.FocusWithin(c) {
				focused = field.Key
			}
		}
	}
	next := make([]*pluginSectionIdentity, len(sections))
	used := map[*pluginSectionIdentity]bool{}
	if focused != "" {
		for i, fields := range keys {
			if !slices.Contains(fields, focused) {
				continue
			}
			for _, old := range a.pluginSections {
				if slices.Contains(old.fields, focused) {
					next[i], used[old] = old, true
					break
				}
			}
		}
	}
	for i, fields := range keys {
		if next[i] != nil {
			continue
		}
		best := 0
		for _, old := range a.pluginSections {
			if used[old] {
				continue
			}
			score := 0
			for _, key := range fields {
				if slices.Contains(old.fields, key) {
					score++
				}
			}
			if score > best {
				next[i], best = old, score
			}
		}
		if next[i] == nil {
			next[i] = &pluginSectionIdentity{}
		}
		used[next[i]] = true
	}
	active := map[string]bool{}
	for i, id := range next {
		id.fields = keys[i]
		for _, key := range keys[i] {
			active[key] = true
		}
	}
	for key := range a.pluginEditorRefs {
		if !active[key] {
			delete(a.pluginEditorRefs, key)
		}
	}
	a.pluginSections = next
	return next
}
