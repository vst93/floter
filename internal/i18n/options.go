package i18n

// This file is how the settings pickers get their rows. Every picker offers a
// value domain — a closed set of ids the settings loader accepts — and its own
// language's label for each. The ids come from `internal/valuedomain` (one list
// per domain, in picker order); the labels are the only part that is per
// language, so they are the only part written twice.
//
// Before P11-w each Copy value spelled the whole table out: `[]Option{{"auto",
// "Auto"}, …}` in copy.go, twice per domain, fourteen tables in all. The ids
// were the same fourteen times, which is exactly the kind of agreement that
// survives until someone adds an option.
//
// The label maps live here rather than in copy.go so a reader can see one
// domain's whole vocabulary — every id and both languages — on one screen.

// optionsFor pairs a domain's ids with this language's labels, in the domain's
// order. A label the map does not carry stays empty rather than falling back to
// the id: an empty row is visibly wrong, while an id shown as its own label
// reads like a real (if untranslated) name. `TestPickerOptionsMatchTheStored
// Domains` fails on either.
func optionsFor(ids []string, labels map[string]string) []Option {
	out := make([]Option, len(ids))
	for i, id := range ids {
		out[i] = Option{ID: id, Label: labels[id]}
	}
	return out
}

var (
	// The app icon's two labels are the same in both copies: "Dark" and
	// "Light" name the icon artwork, not the interface's appearance.
	enAppIconLabels = map[string]string{"dark": "Dark", "light": "Light"}
	zhAppIconLabels = map[string]string{"dark": "Dark", "light": "Light"}

	enThemeLabels = map[string]string{"auto": "Auto", "light": "Light", "dark": "Dark"}
	zhThemeLabels = map[string]string{"auto": "自动", "light": "浅色", "dark": "深色"}

	enGlassStepLabels = map[string]string{
		"off":     "Off",
		"frosted": "Frosted",
		"regular": "Liquid",
		"liquid":  "Liquid Max",
	}
	zhGlassStepLabels = map[string]string{
		"off":     "关闭",
		"frosted": "磨砂",
		"regular": "液态",
		"liquid":  "液态拉满",
	}

	enScaleLabels = map[string]string{
		"tiny":    "Tiny",
		"small":   "Small",
		"default": "Standard",
		"large":   "Large",
	}
	zhScaleLabels = map[string]string{
		"tiny":    "极小",
		"small":   "小",
		"default": "标准",
		"large":   "大",
	}

	enCursorShapeLabels = map[string]string{
		"beam":      "Beam",
		"block":     "Block",
		"underline": "Underline",
	}
	zhCursorShapeLabels = map[string]string{
		"beam":      "竖线",
		"block":     "方块",
		"underline": "下划线",
	}

	enPaddingLabels = map[string]string{
		"compact": "Compact",
		"regular": "Regular",
		"relaxed": "Relaxed",
	}
	zhPaddingLabels = map[string]string{
		"compact": "紧凑",
		"regular": "常规",
		"relaxed": "宽松",
	}

	enPaletteLabels = map[string]string{
		"inherit":  "Follow app",
		"contrast": "High contrast",
		"paper":    "Paper",
		"ink":      "Ink",
		"fog":      "Fog",
		"forest":   "Forest",
		"dusk":     "Dusk",
		"mist":     "Mist",
		"amber":    "Amber",
	}
	zhPaletteLabels = map[string]string{
		"inherit":  "跟随应用",
		"contrast": "高对比",
		"paper":    "纸张",
		"ink":      "墨黑",
		"fog":      "雾灰",
		"forest":   "森林",
		"dusk":     "暮色",
		"mist":     "海雾",
		"amber":    "琥珀",
	}
)
