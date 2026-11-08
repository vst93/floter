package settingsui

import (
	"reflect"
	"slices"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

// Keep a copy: hosts may edit a schema or its options in place. Values are
// deliberately absent; typing and value-only refreshes must not steal scroll.
type pluginSheetLayoutState struct {
	schema       plugincfg.Schema
	failed       bool
	armed        string
	actionFailed string
	changed      bool
}

func (a *App) pluginSheetLayout(c *ui.Context) *pluginSheetLayoutState {
	key := struct {
		app  *App
		role string
	}{a, "plugin-sheet-layout"}
	return ui.Local(c.Root(), key, func() pluginSheetLayoutState { return pluginSheetLayoutState{} })
}

func (s *pluginSheetLayoutState) observe(sheet PluginSheet) {
	s.changed = s.failed != sheet.Failed || s.armed != sheet.Armed || s.actionFailed != sheet.ActionFailed || !reflect.DeepEqual(s.schema, *sheet.Schema)
	if !s.changed {
		return
	}
	s.schema = *sheet.Schema
	s.schema.Fields = slices.Clone(sheet.Schema.Fields)
	for i := range s.schema.Fields {
		s.schema.Fields[i].Options = slices.Clone(s.schema.Fields[i].Options)
	}
	s.failed, s.armed, s.actionFailed = sheet.Failed, sheet.Armed, sheet.ActionFailed
}

func (a *App) pluginFieldOutlet(c *ui.Context, field ui.Element) {
	pluginFieldOutlet(c, field, a.pluginSheetLayout(c).changed)
}
