package i18n

// The custom-shortcut rejection sentence, which names the key and the reason.
func shortcutsRejectedEN(key, reason string) string {
	return "Could not bind " + key + ": " + reason
}

func shortcutsRejectedZH(key, reason string) string {
	return "无法绑定 " + key + "：" + reason
}

// The shipped copy, English first: the source of truth every other language
// is checked against.
var en = Copy{
	Launcher: Launcher{
		Placeholder:          "Type a command or app name",
		Label:                "Search",
		Hint:                 "Type to search",
		NoResults:            "No results",
		Clear:                "Clear",
		Copied:               "Copied",
		CommandModeHint:      "Enter runs it \u00b7 Tab completes \u00b7 Esc cancels",
		ResultsLabel:         "Results",
		CommandSettings:      "Open settings",
		CommandSettingsHint:  "Appearance, window behaviour and integrations",
		CommandTerminal:      "Open terminal",
		CommandTerminalHint:  "Start a shell session",
		CommandBrowser:       "Browser history",
		CommandBrowserHint:   "Search history and bookmarks",
		CommandClipboard:     "Clipboard history",
		CommandClipboardHint: "Search what you copied",
		CommandQuit:          "Quit floter",
		CommandQuitHint:      "Exit the application completely",
		BrowserTab:           "Open tab",
		BrowserNoProfile:     "No browser profile was found",
		ShortcutSettings:     "Cmd+,",

		MenuView:     "View",
		MenuLauncher: "Show floter",
	},
	Settings: Settings{
		Title: "Settings",
		Close: "Close settings",

		// The install approval dialog, and the permissions line in
		// the integrations list.
		PermissionsTitle:    permissionsTitleEN,
		PermissionsAllow:    "Allow",
		PermissionsCancel:   "Cancel",
		PermissionsEnforced: "Floter enforces",
		PermissionsDeclared: "Declared, not blocked",

		PageGeneral:      "General",
		PageSessions:     "Sessions",
		PageShortcuts:    "Shortcuts",
		PagePlugins:      "Plugins",
		PageIntegrations: "Integrations",
		PageAbout:        "About",

		PageGeneralHint:      "Appearance, startup and how the windows behave.",
		PageSessionsHint:     "Terminal sessions still running in the background.",
		PageShortcutsHint:    "Every global shortcut floter answers.",
		PagePluginsHint:      "The built-in plugins' own settings.",
		PageIntegrationsHint: "Built-in plugins and the tools you have connected.",
		PageAboutHint:        "Version, updates and the link scheme.",

		PagePlaceholder: "This page arrives in a later round.",

		GroupAppearance: "Appearance",
		GroupWindow:     "Window behaviour",
		GroupTerminal:   "Terminal appearance",

		Theme:               "Appearance",
		AppIcon:             "App icon",
		AppIconHint:         "The icon shown in the menu bar / tray and on the taskbar. Dark is the default.",
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

		AppIcons: []Option{
			{"dark", "Dark"},
			{"light", "Light"},
		},
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

		LaunchAtStartup:       "Launch at startup",
		LaunchAtStartupHint:   "Start floter automatically when you log in.",
		HideOnBlur:            "Hide when focus is lost",
		HideOnBlurHint:        "Dismiss the panel when you switch to another application.",
		SurfaceResidency:      "Keep pages open for",
		SurfaceResidencyHint:  "A settings page or the terminal stays put when the panel is dismissed, until this long has passed. 0 turns it off.",
		SurfaceResidencyOff:   "Off",
		SurfaceResidencyNever: "Never",
		SurfaceResidencyValue: secondsEN,

		SessionsNone:    "No sessions are running.",
		SessionsRunning: "Running",
		SessionsClose:   "Close session",
		SessionsActive:  "The terminal session stays alive in the background.",

		IntegrationsEmpty:        "No integrations are installed.",
		IntegrationsEnable:       "Enabled",
		IntegrationsRunning:      "Running",
		IntegrationsOff:          "Disabled",
		IntegrationsBroken:       "Broken",
		IntegrationsOrphan:       "Installed on disk, not recorded",
		IntegrationsHint:         "Integrations are command-line tools floter discovers and runs in the terminal.",
		IntegrationsUninstall:    "Uninstall",
		IntegrationsCheck:        "Check",
		IntegrationsInstall:      "Install",
		IntegrationsPackageHint:  "npm package, e.g. @scope/name",
		IntegrationsVersionHint:  "version or range (optional)",
		IntegrationsRemoveTitle:  removeTitleEN,
		IntegrationsRemoveDetail: "The package is removed. The integration's data in extension-data stays.",
		IntegrationsCommands:     "Commands",
		IntegrationsCommandsHint: "A command only appears in the launcher while its switch is on.",
		IntegrationsUnavailable:  "runtime unavailable",

		ShortcutsToggle:   "Show / hide floter",
		ShortcutsHint:     "The global keys floter answers, and the ones you bind yourself.",
		ShortcutRecord:    "Record",
		ShortcutRecording: "Press keys\u2026",

		ShortcutsCustom:      "Custom shortcuts",
		ShortcutsCustomHint:  "Bind a key to a plugin, an action, or a command line. A command line runs with no window at all.",
		ShortcutsNone:        "No custom shortcuts yet.",
		ShortcutsAdd:         "Add",
		ShortcutsRemove:      "Remove",
		ShortcutsAction:      "Action",
		ShortcutsCommand:     "Command line",
		ShortcutsCommandHint: "Run silently, e.g. say done",
		ShortcutsRecordKey:   "Record key",
		ShortcutsRejected:    shortcutsRejectedEN,
		ShortcutsActions: []Option{
			{"plugin:clipboard", "Clipboard history"},
			{"plugin:browser", "Browser history"},
			{"action:toggle_window", "Show / hide floter"},
			{"action:new_command", "New command"},
			{"action:open_settings", "Open settings"},
			{"action:open_external_terminal", "Open a terminal window"},
			{"command", "Command line\u2026"},
		},

		BrowserPlugin:          "Browser history",
		BrowserPluginHint:      "Searches the installed browsers' history, bookmarks and open tabs from the launcher.",
		BrowserEnabled:         "Search browser data",
		BrowserTarget:          "Browser",
		BrowserTargetHint:      "Which browser the launcher searches, and which one a result opens in.",
		BrowserAuto:            "Automatic",
		BrowserCustomDir:       "Extra profile directory",
		BrowserCustomDirHint:   "A browser base directory or a single profile to search as well.",
		BrowserHistoryDays:     "History window",
		BrowserHistoryDaysHint: "How far back history goes; bookmarks are never filtered.",
		BrowserHistoryAll:      "All",
		BrowserSort:            "Order",
		BrowserSortHint:        "How the results are ordered.",
		BrowserSearchField:     "Search in",
		BrowserSearchFieldHint: "Which part of a row the query is matched against.",
		BrowserCDP:             "Open tabs",
		BrowserCDPEnabled:      "Read tabs through the debug port",
		BrowserCDPEnabledHint:  "Needs the browser started with --remote-debugging-port. On macOS tabs are read through AppleScript instead.",
		BrowserCDPPort:         "Debug port",
		BrowserCDPPortHint:     "The port the browser's DevTools endpoint listens on.",
		BrowserSortOrders: []Option{
			{"relevance", "Launcher ranking"},
			{"recent", "Most recent"},
			{"alphabetical", "A to Z"},
			{"visits", "Most visited"},
		},
		BrowserSearchFields: []Option{
			{"all", "Title and address"},
			{"title", "Title only"},
			{"url", "Address only"},
		},
		ClipboardPlugin:       "Clipboard history",
		ClipboardPluginHint:   "Keeps what you copy so the launcher can find it again.",
		ClipboardEnabled:      "Keep a clipboard history",
		ClipboardMaxItems:     "History size",
		ClipboardMaxItemsHint: "How many entries to keep; favourites are never dropped.",

		AboutVersion:      "Version",
		AboutFramework:    "Framework",
		AboutScheme:       "Link scheme",
		AboutSettingsFile: "Settings file",
		AboutProject:      "Project",

		Percent: percentEN,
	},
	Terminal: Terminal{
		Title: "Terminal",
		Pin:   "Pin output",
		Hint:  "The session has not started yet.",
		Close: "Close terminal",
	},
}

var zh = Copy{
	Launcher: Launcher{
		Placeholder:          "输入命令或应用名称",
		Label:                "搜索",
		Hint:                 "输入以搜索",
		NoResults:            "没有匹配项",
		Clear:                "清除",
		Copied:               "已复制",
		CommandModeHint:      "回车运行 \u00b7 Tab 补全 \u00b7 Esc 取消",
		ResultsLabel:         "结果",
		CommandSettings:      "打开设置",
		CommandSettingsHint:  "外观、窗口行为与集成",
		CommandTerminal:      "打开终端",
		CommandTerminalHint:  "启动一个 shell 会话",
		CommandBrowser:       "浏览器历史",
		CommandBrowserHint:   "搜索历史记录与书签",
		CommandClipboard:     "剪贴板历史",
		CommandClipboardHint: "搜索复制过的内容",
		CommandQuit:          "退出 floter",
		CommandQuitHint:      "完全退出应用",
		BrowserTab:           "已打开的标签页",
		BrowserNoProfile:     "没有找到浏览器配置",
		ShortcutSettings:     "Cmd+,",

		MenuView:     "视图",
		MenuLauncher: "显示 floter",
	},
	Settings: Settings{
		Title: "设置",
		Close: "关闭设置",

		PermissionsTitle:    permissionsTitleZH,
		PermissionsAllow:    "允许",
		PermissionsCancel:   "取消",
		PermissionsEnforced: "Floter 强制拦截",
		PermissionsDeclared: "仅声明，不拦截",

		PageGeneral:      "常规",
		PageSessions:     "会话",
		PageShortcuts:    "快捷键",
		PagePlugins:      "插件",
		PageIntegrations: "集成",
		PageAbout:        "关于",

		PageGeneralHint:      "外观、启动方式与窗口行为。",
		PageSessionsHint:     "仍在后台运行的终端会话。",
		PageShortcutsHint:    "floter 响应的全部全局快捷键。",
		PagePluginsHint:      "内置插件各自的设置。",
		PageIntegrationsHint: "内置插件与你已接入的工具。",
		PageAboutHint:        "版本、更新与链接协议。",

		PagePlaceholder: "此页面将在后续迭代中到来。",

		GroupAppearance: "外观",
		GroupWindow:     "窗口行为",
		GroupTerminal:   "终端外观",

		Theme:               "外观",
		AppIcon:             "应用图标",
		AppIconHint:         "菜单栏 / 托盘以及任务栏上显示的图标。默认为深色。",
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

		AppIcons: []Option{
			{"dark", "Dark"},
			{"light", "Light"},
		},
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

		LaunchAtStartup:       "开机自启动",
		LaunchAtStartupHint:   "登录系统时自动启动 floter。",
		HideOnBlur:            "失去焦点时隐藏",
		HideOnBlurHint:        "切换到其他应用时自动收起面板。",
		SurfaceResidency:      "页面驻留时间",
		SurfaceResidencyHint:  "设置页或终端在面板收起后会保留这么久；设为 0 则关闭。",
		SurfaceResidencyOff:   "关闭",
		SurfaceResidencyNever: "永不关闭",
		SurfaceResidencyValue: secondsZH,

		SessionsNone:    "没有正在运行的会话。",
		SessionsRunning: "运行中",
		SessionsClose:   "关闭会话",
		SessionsActive:  "终端会话在后台继续运行。",

		IntegrationsEmpty:        "没有已安装的集成。",
		IntegrationsEnable:       "启用",
		IntegrationsRunning:      "运行中",
		IntegrationsOff:          "已停用",
		IntegrationsBroken:       "已损坏",
		IntegrationsOrphan:       "磁盘上有包，但没有记录",
		IntegrationsHint:         "「集成」是 floter 发现并在终端里运行的命令行工具。",
		IntegrationsUninstall:    "卸载",
		IntegrationsCheck:        "检测",
		IntegrationsInstall:      "安装",
		IntegrationsPackageHint:  "npm 包名，例如 @scope/name",
		IntegrationsVersionHint:  "版本或范围（可空）",
		IntegrationsRemoveTitle:  removeTitleZH,
		IntegrationsRemoveDetail: "将删除扩展包；extension-data 中的集成数据会保留。",
		IntegrationsCommands:     "命令",
		IntegrationsCommandsHint: "只有开关打开的命令才会出现在启动器里。",
		IntegrationsUnavailable:  "运行时不可用",

		ShortcutsToggle:   "显示 / 隐藏 floter",
		ShortcutsHint:     "floter 响应的全局按键，以及你自己绑定的按键。",
		ShortcutRecord:    "录制",
		ShortcutRecording: "请按组合键…",

		ShortcutsCustom:      "自定义快捷键",
		ShortcutsCustomHint:  "把一个按键绑定到插件、动作或命令行；命令行会完全静默地运行。",
		ShortcutsNone:        "还没有自定义快捷键。",
		ShortcutsAdd:         "添加",
		ShortcutsRemove:      "移除",
		ShortcutsAction:      "动作",
		ShortcutsCommand:     "命令行",
		ShortcutsCommandHint: "静默运行，例如 say done",
		ShortcutsRecordKey:   "录制按键",
		ShortcutsRejected:    shortcutsRejectedZH,
		ShortcutsActions: []Option{
			{"plugin:clipboard", "剪贴板历史"},
			{"plugin:browser", "浏览器历史"},
			{"action:toggle_window", "显示 / 隐藏 floter"},
			{"action:new_command", "新命令"},
			{"action:open_settings", "打开设置"},
			{"action:open_external_terminal", "打开终端窗口"},
			{"command", "命令行…"},
		},

		BrowserPlugin:          "浏览器历史",
		BrowserPluginHint:      "在启动器里搜索已安装浏览器的历史、书签与已打开的标签页。",
		BrowserEnabled:         "搜索浏览器数据",
		BrowserTarget:          "浏览器",
		BrowserTargetHint:      "启动器搜索哪个浏览器，结果就在哪个浏览器里打开。",
		BrowserAuto:            "自动",
		BrowserCustomDir:       "额外配置目录",
		BrowserCustomDirHint:   "额外的浏览器 base 目录，或单个 profile 目录。",
		BrowserHistoryDays:     "历史范围",
		BrowserHistoryDaysHint: "历史往前查多久；书签不受此限制。",
		BrowserHistoryAll:      "全部",
		BrowserSort:            "排序",
		BrowserSortHint:        "结果的排序方式。",
		BrowserSearchField:     "搜索范围",
		BrowserSearchFieldHint: "查询匹配行里的哪些字段。",
		BrowserCDP:             "已打开标签页",
		BrowserCDPEnabled:      "通过调试端口读取标签页",
		BrowserCDPEnabledHint:  "需要浏览器以 --remote-debugging-port 启动。macOS 上改用 AppleScript 读取。",
		BrowserCDPPort:         "调试端口",
		BrowserCDPPortHint:     "浏览器 DevTools 端点监听的端口。",
		BrowserSortOrders: []Option{
			{"relevance", "启动器排序"},
			{"recent", "最近访问"},
			{"alphabetical", "按名称"},
			{"visits", "访问最多"},
		},
		BrowserSearchFields: []Option{
			{"all", "标题与地址"},
			{"title", "仅标题"},
			{"url", "仅地址"},
		},
		ClipboardPlugin:       "剪贴板历史",
		ClipboardPluginHint:   "记录你复制的内容，方便在启动器里再次找到。",
		ClipboardEnabled:      "保留剪贴板历史",
		ClipboardMaxItems:     "历史容量",
		ClipboardMaxItemsHint: "最多保留多少条；收藏的条目不会被淘汰。",

		AboutVersion:      "版本",
		AboutFramework:    "框架",
		AboutScheme:       "链接协议",
		AboutSettingsFile: "设置文件",
		AboutProject:      "项目",

		Percent: percentEN,
	},
	Terminal: Terminal{
		Title: "终端",
		Pin:   "钉住输出",
		Hint:  "会话尚未启动。",
		Close: "关闭终端",
	},
}

func removeTitleEN(name string) string { return "Remove " + name + "?" }
func removeTitleZH(name string) string { return "移除 " + name + "？" }

func secondsEN(n uint32) string { return itoa32(n) + " s" }
func secondsZH(n uint32) string { return itoa32(n) + " 秒" }

// itoa32 renders a number, for the residency labels (a value up to a day,
// or the "never" sentinel).
func itoa32(n uint32) string {
	if n == 0 {
		return "0"
	}
	var digits [10]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

func permissionsTitleEN(name string) string { return "Allow " + name + " to run?" }
func permissionsTitleZH(name string) string { return "允许 " + name + " 运行？" }
