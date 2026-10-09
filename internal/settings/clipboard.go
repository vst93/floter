package settings

// The clipboard history's own settings are two top-level keys the app carries
// through rather than fields of its own. They are typed here so the shell's
// watcher and the settings page read one vocabulary.
const (
	clipboardEnabledKey  = "clipboard_history_enabled"
	clipboardMaxItemsKey = "clipboard_history_max_items"
)

// The shipped clipboard history capacity and the band it is clamped to.
const (
	DefaultClipboardMaxItems = 300
	MinClipboardMaxItems     = 10
	MaxClipboardMaxItems     = 500
)

// ClipboardSettings is the clipboard history's state.
type ClipboardSettings struct {
	// Enabled is the history's on/off switch.
	Enabled bool
	// MaxItems is how many entries are kept; favourites are never dropped.
	MaxItems int
}

// DefaultClipboardSettings is the shipped state: on, three hundred entries.
func DefaultClipboardSettings() ClipboardSettings {
	return ClipboardSettings{Enabled: true, MaxItems: DefaultClipboardMaxItems}
}

// ClipboardOf reads the clipboard settings, filling missing or unusable values
// with the shipped defaults.
func ClipboardOf(s Settings) ClipboardSettings {
	state := DefaultClipboardSettings()
	extra := s.Extra()
	if enabled, ok := extra[clipboardEnabledKey].(bool); ok {
		state.Enabled = enabled
	}
	if items, ok := asInt(extra[clipboardMaxItemsKey]); ok {
		state.MaxItems = min(max(items, MinClipboardMaxItems), MaxClipboardMaxItems)
	}
	return state
}

// SetClipboard writes the clipboard settings back.
func (s *Settings) SetClipboard(state ClipboardSettings) {
	s.SetExtra(clipboardEnabledKey, state.Enabled)
	s.SetExtra(clipboardMaxItemsKey, min(max(state.MaxItems, MinClipboardMaxItems), MaxClipboardMaxItems))
}
