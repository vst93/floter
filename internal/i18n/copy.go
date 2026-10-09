package i18n

// The shipped copy. Keys and wording follow src/i18n.ts where the old build
// already had them, so a user who switches languages reads the same words.
var en = Copy{
	Launcher: Launcher{
		Placeholder:         "Type a command or app name",
		Label:               "Search",
		Hint:                "Type to search",
		NoResults:           "No results",
		Clear:               "Clear",
		Copied:              "Copied",
		ResultsLabel:        "Results",
		CommandSettings:     "Open settings",
		CommandSettingsHint: "Appearance, window behaviour and integrations",
		CommandTerminal:     "Open terminal",
		CommandTerminalHint: "Start a shell session",
		CommandQuit:         "Quit floter",
		CommandQuitHint:     "Exit the application completely",
		ShortcutSettings:    "Cmd+,",
	},
	Settings: Settings{
		Title: "Settings",
		Close: "Close settings",

		PageGeneral:      "General",
		PageSessions:     "Sessions",
		PageShortcuts:    "Shortcuts",
		PageIntegrations: "Integrations",
		PageAbout:        "About",

		PageGeneralHint:      "Appearance, startup and how the windows behave.",
		PageSessionsHint:     "Terminal sessions still running in the background.",
		PageShortcutsHint:    "Every global shortcut floter answers.",
		PageIntegrationsHint: "Built-in plugins and the tools you have connected.",
		PageAboutHint:        "Version, updates and the link scheme.",

		PagePlaceholder: "This page arrives in a later round.",

		GroupAppearance: "Appearance",
		GroupWindow:     "Window behaviour",
		GroupTerminal:   "Terminal appearance",

		Theme:               "Appearance",
		Language:            "Language",
		LanguageHint:        "Applies immediately across the app.",
		Glass:               "Liquid glass effect",
		GlassHint:           "From frosted glass to Apple's liquid glass at full strength. Independent of the transparency below.",
		MainOpacity:         "App transparency",
		MainOpacityHint:     "How solid each window's background is, independent of the glass effect. At 100% the surface is near-opaque.",
		TerminalOpacity:     "Terminal transparency",
		TerminalOpacityHint: "How solid the terminal window's background is.",
		Scale:               "Interface size",
		ScaleHint:           "Scales every interface element - text, buttons and inputs. Small is the default.",

		TerminalHint: "Applies to the running session right away.",
		FontSize:     "Font size",
		FontFamily:   "Font family",
		CursorShape:  "Cursor shape",
		CursorBlink:  "Blinking cursor",
		LineHeight:   "Line height",
		Padding:      "Padding",
		Palette:      "Terminal palette",

		Themes: []Option{
			{"auto", "Auto"},
			{"light", "Light"},
			{"dark", "Dark"},
		},
		Languages: []Option{
			{"en", "English"},
			{"zh", "中文"},
		},
		GlassSteps: []Option{
			{"off", "Off"},
			{"frosted", "Frosted"},
			{"regular", "Liquid"},
			{"liquid", "Liquid Max"},
		},
		Scales: []Option{
			{"tiny", "Tiny"},
			{"small", "Small"},
			{"default", "Standard"},
			{"large", "Large"},
		},
		CursorShapes: []Option{
			{"beam", "Beam"},
			{"block", "Block"},
			{"underline", "Underline"},
		},
		Paddings: []Option{
			{"compact", "Compact"},
			{"regular", "Regular"},
			{"relaxed", "Relaxed"},
		},
		Palettes: []Option{
			{"inherit", "Follow app"},
			{"contrast", "High contrast"},
			{"paper", "Paper"},
			{"ink", "Ink"},
			{"fog", "Fog"},
			{"forest", "Forest"},
			{"dusk", "Dusk"},
			{"mist", "Mist"},
			{"amber", "Amber"},
		},

		SessionsNone:    "No sessions are running.",
		SessionsRunning: "Running",
		SessionsClose:   "Close session",
		SessionsActive:  "The terminal session stays alive in the background.",

		ShortcutsToggle: "Show / hide floter",
		ShortcutsHint:   "More shortcuts arrive with the system integration.",

		AboutVersion:      "Version",
		AboutFramework:    "Framework",
		AboutScheme:       "Link scheme",
		AboutSettingsFile: "Settings file",
		AboutProject:      "Project",

		Percent: percentEN,
	},
	Terminal: Terminal{
		Title: "Terminal",
		Hint:  "The session has not started yet.",
		Close: "Close terminal",
	},
}

var zh = Copy{
	Launcher: Launcher{
		Placeholder:         "输入命令或应用名称",
		Label:               "搜索",
		Hint:                "输入以搜索",
		NoResults:           "没有匹配项",
		Clear:               "清除",
		Copied:              "已复制",
		ResultsLabel:        "结果",
		CommandSettings:     "打开设置",
		CommandSettingsHint: "外观、窗口行为与集成",
		CommandTerminal:     "打开终端",
		CommandTerminalHint: "启动一个 shell 会话",
		CommandQuit:         "退出 floter",
		CommandQuitHint:     "完全退出应用",
		ShortcutSettings:    "Cmd+,",
	},
	Settings: Settings{
		Title: "设置",
		Close: "关闭设置",

		PageGeneral:      "常规",
		PageSessions:     "会话",
		PageShortcuts:    "快捷键",
		PageIntegrations: "集成",
		PageAbout:        "关于",

		PageGeneralHint:      "外观、启动方式与窗口行为。",
		PageSessionsHint:     "仍在后台运行的终端会话。",
		PageShortcutsHint:    "floter 响应的全部全局快捷键。",
		PageIntegrationsHint: "内置插件与你已接入的工具。",
		PageAboutHint:        "版本、更新与链接协议。",

		PagePlaceholder: "此页面将在后续迭代中到来。",

		GroupAppearance: "外观",
		GroupWindow:     "窗口行为",
		GroupTerminal:   "终端外观",

		Theme:               "外观",
		Language:            "语言",
		LanguageHint:        "选择后立即生效。",
		Glass:               "液态玻璃效果",
		GlassHint:           "从磨砂玻璃到苹果液态玻璃拉满。与下方透明度相互独立。",
		MainOpacity:         "应用背景透明度",
		MainOpacityHint:     "每个窗口底色的实底程度，与玻璃效果相互独立。100% 时接近不透明。",
		TerminalOpacity:     "终端背景透明度",
		TerminalOpacityHint: "终端窗口底色的实底程度。",
		Scale:               "界面大小",
		ScaleHint:           "缩放所有界面元素——文字、按钮与输入框。默认为「小」。",

		TerminalHint: "更改会立即应用到正在运行的会话。",
		FontSize:     "字号",
		FontFamily:   "字体",
		CursorShape:  "光标形状",
		CursorBlink:  "光标闪烁",
		LineHeight:   "行距",
		Padding:      "边距",
		Palette:      "终端配色",

		Themes: []Option{
			{"auto", "自动"},
			{"light", "浅色"},
			{"dark", "深色"},
		},
		Languages: []Option{
			{"en", "English"},
			{"zh", "中文"},
		},
		GlassSteps: []Option{
			{"off", "关闭"},
			{"frosted", "磨砂"},
			{"regular", "液态"},
			{"liquid", "液态拉满"},
		},
		Scales: []Option{
			{"tiny", "极小"},
			{"small", "小"},
			{"default", "标准"},
			{"large", "大"},
		},
		CursorShapes: []Option{
			{"beam", "竖线"},
			{"block", "方块"},
			{"underline", "下划线"},
		},
		Paddings: []Option{
			{"compact", "紧凑"},
			{"regular", "常规"},
			{"relaxed", "宽松"},
		},
		Palettes: []Option{
			{"inherit", "跟随应用"},
			{"contrast", "高对比"},
			{"paper", "纸张"},
			{"ink", "墨黑"},
			{"fog", "雾灰"},
			{"forest", "森林"},
			{"dusk", "暮色"},
			{"mist", "海雾"},
			{"amber", "琥珀"},
		},

		SessionsNone:    "没有正在运行的会话。",
		SessionsRunning: "运行中",
		SessionsClose:   "关闭会话",
		SessionsActive:  "终端会话在后台继续运行。",

		ShortcutsToggle: "显示 / 隐藏 floter",
		ShortcutsHint:   "更多快捷键将随系统集成一并到来。",

		AboutVersion:      "版本",
		AboutFramework:    "框架",
		AboutScheme:       "链接协议",
		AboutSettingsFile: "设置文件",
		AboutProject:      "项目",

		Percent: percentEN,
	},
	Terminal: Terminal{
		Title: "终端",
		Hint:  "会话尚未启动。",
		Close: "关闭终端",
	},
}
