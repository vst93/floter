package settings

// The calculator plugin's settings block lives under the `calculator_plugin`
// key. The block is carried through (this package does not model it as a
// field) but typed here so the settings page and the shell agree on the shape
// and on the normalization.
const calculatorPluginKey = "calculator_plugin"

// The shipped calculator defaults and the bands its fields are clamped to.
const (
	DefaultCalculatorMaxItems      = 100
	MinCalculatorMaxItems          = 10
	MaxCalculatorMaxItems          = 500
	DefaultCalculatorRetentionDays = 30
)

// CalculatorRetentionDays is the age-window vocabulary, in the order the
// settings picker lists it; 0 means "never expire".
var CalculatorRetentionDays = []int{0, 1, 7, 30}

// The two copy modes: what Enter copies from a history row.
const (
	CalculatorCopyFull   = "full"
	CalculatorCopyResult = "result"
)

// CalculatorPlugin is the calculator plugin's settings block.
type CalculatorPlugin struct {
	// MaxItems is the capacity for the non-favourite entries.
	MaxItems int
	// RetentionDays is the age window; 0 means "never expire". A favourite is
	// exempt from both.
	RetentionDays int
	// CopyMode is what Enter copies: the whole `expression = result` line, or
	// the result alone.
	CopyMode string
}

// DefaultCalculatorPlugin is the shipped block.
func DefaultCalculatorPlugin() CalculatorPlugin {
	return CalculatorPlugin{
		MaxItems:      DefaultCalculatorMaxItems,
		RetentionDays: DefaultCalculatorRetentionDays,
		CopyMode:      CalculatorCopyFull,
	}
}

// CalculatorPluginOf reads the block, filling missing or unusable fields with
// the shipped defaults.
func CalculatorPluginOf(s Settings) CalculatorPlugin {
	plugin := DefaultCalculatorPlugin()
	raw, ok := s.Extra()[calculatorPluginKey].(map[string]any)
	if !ok {
		return plugin
	}
	if items, ok := asInt(raw["max_items"]); ok {
		plugin.MaxItems = NormalizeCalculatorMaxItems(items)
	}
	if days, ok := asInt(raw["retention_days"]); ok {
		plugin.RetentionDays = NormalizeCalculatorRetentionDays(days)
	}
	if mode, ok := raw["copy_mode"].(string); ok {
		plugin.CopyMode = NormalizeCalculatorCopyMode(mode)
	}
	return plugin
}

// SetCalculatorPlugin writes the block back, touching only the keys this build
// owns: any other key inside `calculator_plugin` is preserved verbatim.
func (s *Settings) SetCalculatorPlugin(plugin CalculatorPlugin) {
	raw := map[string]any{}
	if existing, ok := s.Extra()[calculatorPluginKey].(map[string]any); ok {
		for key, value := range existing {
			raw[key] = value
		}
	}
	raw["max_items"] = NormalizeCalculatorMaxItems(plugin.MaxItems)
	raw["retention_days"] = NormalizeCalculatorRetentionDays(plugin.RetentionDays)
	raw["copy_mode"] = NormalizeCalculatorCopyMode(plugin.CopyMode)
	s.SetExtra(calculatorPluginKey, raw)
}

// NormalizeCalculatorMaxItems snaps a capacity into the band; a non-positive
// value falls back to the shipped default rather than to a bound.
func NormalizeCalculatorMaxItems(value int) int {
	if value <= 0 {
		return DefaultCalculatorMaxItems
	}
	if value < MinCalculatorMaxItems {
		return MinCalculatorMaxItems
	}
	if value > MaxCalculatorMaxItems {
		return MaxCalculatorMaxItems
	}
	return value
}

// NormalizeCalculatorRetentionDays accepts one of the offered windows;
// anything else is the shipped one.
func NormalizeCalculatorRetentionDays(days int) int {
	for _, allowed := range CalculatorRetentionDays {
		if days == allowed {
			return days
		}
	}
	return DefaultCalculatorRetentionDays
}

// NormalizeCalculatorCopyMode accepts one of the two modes; anything else is
// the whole line.
func NormalizeCalculatorCopyMode(mode string) string {
	if mode == CalculatorCopyResult {
		return CalculatorCopyResult
	}
	return CalculatorCopyFull
}
