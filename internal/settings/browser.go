package settings

import (
	"strings"
)

// The browser plugin's settings block lives under the `browser_plugin` key.
// The block is carried through as a carried-through key (this package does not
// own it), but its fields are typed here so the settings page and the shell
// agree on the vocabulary and on the normalization.
const browserPluginKey = "browser_plugin"

// The shipped browser-plugin defaults, and the bounds its fields are clamped
// to.
const (
	DefaultBrowserTarget      = "auto"
	DefaultBrowserHistoryDays = 30
	DefaultBrowserSortOrder   = "relevance"
	DefaultBrowserSearchField = "all"
	DefaultCDPPort            = 9222
	// MaxBrowserHistoryDays is the largest history window the plugin honours
	// (ten years).
	MaxBrowserHistoryDays = 3650
)

// The two enumerated vocabularies, in the order a picker shows them.
var (
	BrowserSortOrders     = []string{"relevance", "recent", "alphabetical", "visits"}
	BrowserSearchFields   = []string{"all", "title", "url"}
	browserSortOrderSet   = map[string]bool{"relevance": true, "recent": true, "alphabetical": true, "visits": true}
	browserSearchFieldSet = map[string]bool{"all": true, "title": true, "url": true}
)

// BrowserPlugin is the browser plugin's settings block.
type BrowserPlugin struct {
	// Enabled is the plugin's on/off switch. Off means the launcher's browser
	// mode fetches nothing.
	Enabled bool
	// Target is the browser a page opens in and the browser the launcher
	// searches: a browser id, or `auto` for "the first one with data".
	Target string
	// CustomBaseDir is an extra profile directory to search, empty when none.
	CustomBaseDir string
	// HistoryDays is how far back history goes; 0 means all of it.
	HistoryDays int
	// CDPEnabled and CDPPort are the DevTools-protocol opt-in used to read
	// live tabs on Windows and Linux.
	CDPEnabled bool
	CDPPort    int
	// SortOrder is how results are ordered, SearchField what the query is
	// matched against.
	SortOrder   string
	SearchField string
}

// DefaultBrowserPlugin is the shipped block: on, auto-detect, thirty days of
// history, the launcher's own ordering.
func DefaultBrowserPlugin() BrowserPlugin {
	return BrowserPlugin{
		Enabled:     true,
		Target:      DefaultBrowserTarget,
		HistoryDays: DefaultBrowserHistoryDays,
		CDPPort:     DefaultCDPPort,
		SortOrder:   DefaultBrowserSortOrder,
		SearchField: DefaultBrowserSearchField,
	}
}

// BrowserPluginOf reads the block out of a settings value, filling missing or
// unusable fields with the shipped defaults. A key that is present but has the
// wrong shape is ignored rather than fatal: a hand-edited file must not break
// the app.
func BrowserPluginOf(s Settings) BrowserPlugin {
	plugin := DefaultBrowserPlugin()
	raw, ok := s.Extra()[browserPluginKey].(map[string]any)
	if !ok {
		return plugin
	}
	if enabled, ok := raw["enabled"].(bool); ok {
		plugin.Enabled = enabled
	}
	if target, ok := raw["target"].(string); ok {
		plugin.Target = NormalizeBrowserTarget(target)
	}
	if dir, ok := raw["custom_base_dir"].(string); ok {
		plugin.CustomBaseDir = strings.TrimSpace(dir)
	}
	if days, ok := asInt(raw["history_days"]); ok && days >= 0 {
		plugin.HistoryDays = min(days, MaxBrowserHistoryDays)
	}
	if enabled, ok := raw["cdp_enabled"].(bool); ok {
		plugin.CDPEnabled = enabled
	}
	if port, ok := asInt(raw["cdp_port"]); ok && port > 0 && port <= 65535 {
		plugin.CDPPort = port
	}
	if order, ok := raw["sort_order"].(string); ok {
		plugin.SortOrder = NormalizeBrowserSortOrder(order)
	}
	if field, ok := raw["search_fields"].(string); ok {
		plugin.SearchField = NormalizeBrowserSearchField(field)
	}
	return plugin
}

// SetBrowserPlugin writes the block back, touching only the keys this build
// owns: any other key inside `browser_plugin` is preserved verbatim.
func (s *Settings) SetBrowserPlugin(plugin BrowserPlugin) {
	raw := map[string]any{}
	if existing, ok := s.Extra()[browserPluginKey].(map[string]any); ok {
		for key, value := range existing {
			raw[key] = value
		}
	}
	raw["enabled"] = plugin.Enabled
	raw["target"] = NormalizeBrowserTarget(plugin.Target)
	// An empty directory is the same as none, and the old build wrote it as
	// JSON null.
	if plugin.CustomBaseDir == "" {
		raw["custom_base_dir"] = nil
	} else {
		raw["custom_base_dir"] = plugin.CustomBaseDir
	}
	raw["history_days"] = min(max(plugin.HistoryDays, 0), MaxBrowserHistoryDays)
	raw["cdp_enabled"] = plugin.CDPEnabled
	raw["cdp_port"] = NormalizeCDPPort(plugin.CDPPort)
	raw["sort_order"] = NormalizeBrowserSortOrder(plugin.SortOrder)
	raw["search_fields"] = NormalizeBrowserSearchField(plugin.SearchField)
	s.SetExtra(browserPluginKey, raw)
}

// NormalizeBrowserTarget trims a target: empty becomes `auto`. Whether the
// named browser is installed is decided when the profile is resolved, so an
// uninstalled target falls back to `auto` at use time rather than being
// rewritten here.
func NormalizeBrowserTarget(target string) string {
	target = strings.TrimSpace(strings.ToLower(target))
	if target == "" {
		return DefaultBrowserTarget
	}
	return target
}

// NormalizeBrowserSortOrder accepts one of the four orderings; anything else
// is the shipped one.
func NormalizeBrowserSortOrder(order string) string {
	order = strings.TrimSpace(strings.ToLower(order))
	if browserSortOrderSet[order] {
		return order
	}
	return DefaultBrowserSortOrder
}

// NormalizeBrowserSearchField accepts one of the three fields; anything else
// is `all`.
func NormalizeBrowserSearchField(field string) string {
	field = strings.TrimSpace(strings.ToLower(field))
	if browserSearchFieldSet[field] {
		return field
	}
	return DefaultBrowserSearchField
}

// NormalizeCDPPort maps an unusable debug port onto the browser's own default,
// as the old build did: a port of 0 is not a port.
func NormalizeCDPPort(port int) int {
	if port <= 0 || port > 65535 {
		return DefaultCDPPort
	}
	return port
}
