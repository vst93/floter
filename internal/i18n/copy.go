package i18n

import "strconv"

// The custom-shortcut rejection sentence, which names the key and the reason.
// The import report line: what landed, and what did not.
func integrationsImportedEN(succeeded, failed, skipped int) string {
	line := strconv.Itoa(succeeded) + " imported"
	if failed > 0 {
		line += ", " + strconv.Itoa(failed) + " failed"
	}
	if skipped > 0 {
		line += ", " + strconv.Itoa(skipped) + " skipped"
	}
	return line
}

func integrationsImportedZH(succeeded, failed, skipped int) string {
	line := "已导入 " + strconv.Itoa(succeeded) + " 个"
	if failed > 0 {
		line += "，失败 " + strconv.Itoa(failed)
	}
	if skipped > 0 {
		line += "，跳过 " + strconv.Itoa(skipped)
	}
	return line
}

func shortcutsRejectedEN(key, reason string) string {
	return "Could not bind " + key + ": " + reason
}

// The calculator's retention windows, as the picker labels them.
func calculatorRetentionEN(days int) string {
	if days <= 0 {
		return "Never expire"
	}
	if days == 1 {
		return "1 day"
	}
	return strconv.Itoa(days) + " days"
}

func calculatorRetentionZH(days int) string {
	if days <= 0 {
		return "永不过期"
	}
	return strconv.Itoa(days) + " 天"
}

func shortcutsRejectedZH(key, reason string) string {
	return "无法绑定 " + key + "：" + reason
}

// freshnessRelativeEN words a past time the way the platform's own
// `Intl.RelativeTimeFormat("en", {numeric: "auto", style: "short"})` did in
// the old build, down to the forms it shortens: "yesterday", "last mo.",
// "last yr.". The month/year units stay abbreviated, matching style: "short".
func freshnessRelativeEN(value int, unit RelativeUnit) string {
	switch unit {
	case RelativeMinute:
		return strconv.Itoa(value) + " min. ago"
	case RelativeHour:
		return strconv.Itoa(value) + " hr. ago"
	case RelativeDay:
		if value == 1 {
			return "yesterday"
		}
		return strconv.Itoa(value) + " days ago"
	case RelativeMonth:
		if value == 1 {
			return "last mo."
		}
		return strconv.Itoa(value) + " mo. ago"
	case RelativeYear:
		if value == 1 {
			return "last yr."
		}
		return strconv.Itoa(value) + " yr. ago"
	}
	return strconv.Itoa(value) + " ago"
}

// freshnessRelativeZH is the same for `Intl.RelativeTimeFormat("zh", …)`,
// whose shortened forms differ in shape: "昨天" and "前天" for one and two
// days, "上个月" and "去年" for one month and one year.
func freshnessRelativeZH(value int, unit RelativeUnit) string {
	switch unit {
	case RelativeMinute:
		return strconv.Itoa(value) + "分钟前"
	case RelativeHour:
		return strconv.Itoa(value) + "小时前"
	case RelativeDay:
		switch value {
		case 1:
			return "昨天"
		case 2:
			return "前天"
		}
		return strconv.Itoa(value) + "天前"
	case RelativeMonth:
		if value == 1 {
			return "上个月"
		}
		return strconv.Itoa(value) + "个月前"
	case RelativeYear:
		if value == 1 {
			return "去年"
		}
		return strconv.Itoa(value) + "年前"
	}
	return strconv.Itoa(value) + "前"
}

// The shipped copy, English first: the source of truth every other language
// is checked against.
var en = Copy{
	Notifications: Notifications{
		Title:                    "floter",
		IntegrationInstalled:     func(name string) string { return name + " was installed" },
		IntegrationInstallFailed: func(name string) string { return name + " could not be installed" },
		IntegrationRemoved:       func(name string) string { return name + " was removed" },
		IntegrationRemoveFailed:  func(name string) string { return name + " could not be removed" },
		IntegrationChecked:       func(name string) string { return name + " is working" },
		IntegrationCheckFailed:   func(name string) string { return name + " reported a problem" },
		IntegrationReconnected:   func(name string) string { return name + " was reconnected" },
		IntegrationRepaired:      func(name string) string { return name + " was repaired" },
		IntegrationRechecked:     func(name string) string { return name + " re-checked: healthy" },
		CommandFinished:          func(name string) string { return name + " finished" },
		CommandStatus:            func(status string) string { return status },
		ClipboardCleared:         func(name string) string { return name + " cleared" },
		ClipboardClearFailed:     func(name string) string { return name + " could not be cleared" },
	},
	Launcher: Launcher{
		Placeholder:              "Type a command or app name",
		Label:                    "Search",
		Hint:                     "Type to search",
		TrayShow:                 "Show floter",
		TraySettings:             "Settings\u2026",
		TrayReload:               "Re-scan",
		TrayQuit:                 "Quit",
		ConfigurePlugin:          "Options",
		ConfigurePluginClose:     "Close options",
		ConfigurePluginHint:      "Configure this plugin",
		ConfigurePluginCloseHint: "Close plugin settings",
		FieldSessions:            "Sessions",
		FieldSettings:            "Settings",
		FieldSessionsHint:        "Manage terminal sessions",
		FieldSettingsHint:        func(shortcut string) string { return "Settings (" + shortcut + ")" },
		ScopeBrowser:             "Browser",
		ScopeClipboard:           "Clipboard",
		ScopeCalculator:          "Calculator",
		ScopePlugin:              "Plugin",
		BrowserFilterHint:        "Tab to switch",
		ClipboardFilterHint:      func(shortcut string) string { return "Tab to switch \u00b7 " + shortcut + " to favorite" },
		CalculatorFilterHint:     func(shortcut string) string { return "Tab to switch \u00b7 " + shortcut + " to favorite" },
		BareTerminal:             "Open a terminal",
		BareTerminalHint:         "A new session with no command typed",
		TriggerHint:              func(name string) string { return name + " \u00b7 enter" },
		FilesSection:             "Dropped files",
		FilesMore:                func(count int) string { return "\u2026and " + strconv.Itoa(count) + " more files" },
		FilesMoreHint:            "Show every dropped file",
		RecentSection:            "Recently launched",
		TriggerHintMore:          func(name string, more int) string { return name + " \u00b7 enter (+" + strconv.Itoa(more) + " more)" },
		AliasMatch:               func(alias string) string { return "Alias: " + alias },
		NoResults:                "No results",
		Clear:                    "Clear",
		Copied:                   "Copied",
		CommandModeHint:          "Enter runs it \u00b7 Tab completes \u00b7 Esc cancels",
		DirectOutputHint:         "Press space to run this command in the search box",
		CommandConflict:          "Command conflict",
		ResultsLabel:             "Results",
		CommandSettings:          "Open settings",
		CommandSettingsHint:      "Appearance, window behaviour and integrations",
		CommandTerminal:          "Open terminal",
		CommandTerminalHint:      "Start a shell session",
		CommandBrowser:           "Browser history",
		CommandBrowserHint:       "Search history and bookmarks",
		CommandClipboard:         "Clipboard history",
		CommandClipboardHint:     "Search what you copied",
		CommandCalculator:        "Calculator",
		CommandCalculatorHint:    "Evaluate an expression and keep the history",
		CommandQuit:              "Quit floter",
		CommandQuitHint:          "Exit the application completely",
		OutputEmpty:              "The command wrote nothing",
		OutputSucceeded:          "Finished",
		OutputTimedOut:           "Timed out",
		OutputTruncated:          "output truncated",
		OutputHint:               "Enter copies \u00b7 Esc closes",
		OutputListHint:           "\u2191\u2193 choose \u00b7 Enter runs \u00b7 Esc closes",
		OutputFailed:             "Exit code %d",
		OutputLoading:            "Loading more\u2026",
		OutputEnd:                "No more results",

		ClipboardFavoriteFailed:  "Could not change the star",
		ClipboardDeleteFailed:    "Could not delete the entry",
		ClipboardCopyFailed:      "Could not copy that entry",
		ClipboardDeleted:         "Entry deleted",
		HistoryDelete:            "Delete",
		HistoryPin:               "Pin as a window",
		DeleteConfirm:            "Press again to delete",
		HistoryCleared:           "Command history cleared",
		HistoryFavorite:          "Favorite",
		HistoryUnfavorite:        "Unfavorite",
		OutputDetach:             "Open in a window",
		OutputDetachHint:         "Open this output in a window that stays put",
		OutputWindowFallback:     "floter command",
		OutputWindowRerun:        "Re-run",
		OutputWindowClose:        "Close",
		OutputWindowIdle:         "Nothing has run yet",
		CalculatorDeleted:        "Calculation deleted",
		CalculatorDeleteFailed:   "Could not delete the calculation",
		CalculatorEmpty:          "No calculations yet",
		CalculatorEmptyFavorites: "No favorites yet",
		ClipboardEmpty:           "Nothing copied yet",
		ClipboardEmptyFilter:     "No matches",
		ClipboardEmptyFavorites:  "No favorites yet",
		ClipboardDisabled:        "Clipboard history is off. Enable it in Settings > Integrations.",
		BrowserEmpty:             "No matches",
		CalculatorRecordFailed:   "Could not save the calculation",
		CalculatorFavoriteFailed: "Could not change the star",
		CalculatorYesterday:      "Yesterday",
		PowerRestart:             "Restart the computer",
		PowerRestartHint:         "Asks first, then restarts",
		PowerShutdown:            "Shut down the computer",
		PowerShutdownHint:        "Asks first, then shuts down",
		PowerCancel:              "Cancel",
		PowerRestartAction:       "Restart",
		PowerShutdownAction:      "Shut Down",
		PowerConfirmMessage: func(action string) string {
			if action == "shutdown" {
				return "Shut down this computer now? Unsaved work may be lost."
			}
			return "Restart this computer now? Unsaved work may be lost."
		},
		PowerFailed:               "Could not ask the system to do that",
		OpenInBrowser:             "Open in browser",
		OpenInFiles:               "Open in file manager",
		OpenFile:                  "Open file",
		RunInShell:                "Run in terminal",
		InstallTool:               func(name string) string { return "Install " + name },
		InvokeFailed:              "Could not start this tool",
		OpenAppFailed:             "Could not open this application",
		OpenPathFailed:            "Could not open this path",
		OpenURLFailed:             "Could not open this link",
		OpenTermFailed:            "Could not continue in the system terminal",
		FileOpen:                  "Open",
		FileCd:                    "Open a terminal in",
		FileCopyPath:              "Copy the path of",
		FileFolder:                "Folder",
		BrowserNoProfile:          "No browser profile was found",
		BrowserDisabled:           "Browser Bookmarks & History is turned off. Enable it in Settings > Integrations.",
		BrowserAll:                "All",
		BrowserBookmarks:          "Bookmarks",
		BrowserHistory:            "History",
		BrowserTabs:               "Tabs",
		ClipboardFilterAll:        "All",
		ClipboardFilterFavorites:  "Favorites",
		ClipboardTypeText:         "Text",
		ClipboardTypeImage:        "Images",
		ClipboardTypeLink:         "Links",
		ClipboardTypeFiles:        "Files",
		CalculatorFilterAll:       "All",
		CalculatorFilterFavorites: "Favorites",

		MenuView:     "View",
		MenuLauncher: "Show floter",
	},
	Settings: Settings{
		Title:     "Settings",
		Close:     "Close settings",
		CloseHint: "Close settings (Esc)",
		Quit:      "Quit floter",
		QuitHint:  "Exit the application completely",

		// The install approval dialog, and the permissions line in
		// the integrations list.
		PermissionsTitle:    permissionsTitleEN,
		PermissionsAllow:    "Allow",
		PermissionsCancel:   "Cancel",
		PermissionsEnforced: "Floter enforces",
		PermissionsDeclared: "Declared, not blocked",
		// The connect flow's permission review: the reference's
		// `permissionsRequired` / `confirmConnectRecommended` / `connect`
		// (i18n.ts:465-469).
		PermissionsRequired:     "Permissions required",
		ConnectRecommendedTitle: func(name string) string { return "Connect the recommended tool " + name + " to Floter?" },
		PermissionsConnect:      "Connect",
		// The tool-selection dialog (i18n.ts:472-473).
		ChooseSystemToolTitle: func(name string) string { return "Choose a runtime for " + name },
		ChooseSystemToolHint:  "Multiple matching executables were discovered. Choose the exact path Floter should bind to.",

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
		SaveFailed:      "Could not save that change.",
		SaveFailedAlert: "Changes could not be applied. Check your settings and try again.",

		GroupAppearance:   "Appearance",
		GroupWindow:       "Window behaviour",
		GroupTerminal:     "Terminal appearance",
		GroupMenuBar:      "Menu bar",
		GroupTransparency: "Transparency",

		Theme:            "Appearance",
		ThemeHint:        "Auto follows your system appearance.",
		AppIcon:          "App icon",
		AppIconHint:      "The icon shown in the menu bar / tray and on the taskbar. Dark is the default.",
		Language:         "Language",
		LanguageHint:     "Applies immediately across the app.",
		Material:         "Glass",
		Glass:            "Liquid glass effect",
		GlassHint:        "How much liquid glass there is: distortion, refraction and the lens quality of every control, from frosted glass to Apple's liquid glass at full strength. Independent of the transparency below.",
		TransparencyHint: "How solid each window's background is, independent of the glass effect. At 100% the surface is near-opaque.",
		MainOpacity:      "App transparency",
		TerminalOpacity:  "Terminal transparency",

		TransparencyScaleLow:  "Transparent",
		TransparencyScaleHigh: "Solid",

		Scale:      "Interface size",
		ScaleHint:  "Scales every interface element - text, buttons and inputs. Small is the default.",
		ScaleSmall: "Small",
		ScaleLarge: "Large",

		TerminalHint:   "Applies to the running session right away.",
		FontSize:       "Font size",
		FontFamily:     "Font family",
		CursorShape:    "Cursor shape",
		CursorBlink:    "Blinking cursor",
		LineHeight:     "Line height",
		Padding:        "Padding",
		Palette:        "Terminal palette",
		PasteSafe:      "Safe paste",
		PalettePreview: "Palette preview",

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

		LaunchAtStartup:             "Launch at startup",
		LaunchAtStartupHint:         "Start floter automatically when you log in.",
		HideOnBlur:                  "Hide when focus is lost",
		HideOnBlurHint:              "Dismiss the panel when you switch to another application.",
		ShowRecentInLauncher:        "Show recently launched apps",
		ShowRecentInLauncherHint:    "Your most-used apps appear when the launcher is empty",
		ShowMenubarIcon:             "Show menu bar icon",
		ShowMenubarIconHint:         "Show the menu bar / tray icon. The global shortcut keeps working when it is hidden.",
		ShowCommandsInSearch:        "Show PATH commands in search",
		ShowCommandsInSearchHint:    "Offer the executables found on your PATH as launcher results.",
		SurfaceResidency:            "Keep pages open for",
		SurfaceResidencyHint:        "A plugin page, its settings, or the terminal stays put when the panel is dismissed, until this long has passed. 0 turns it off.",
		SurfaceResidencyOff:         "Off",
		SurfaceResidencyCustomLabel: "Custom duration in seconds",
		SurfaceResidencyCustomUnit:  "s",
		SurfaceResidencyCustomApply: "Set",
		SurfaceResidencyCustom:      "Custom…",
		SurfaceResidencyNever:       "Never",
		SurfaceResidencyValue:       secondsEN,
		SurfaceResidencyCustomValue: customSecondsEN,

		SessionsNone:         "No sessions are running.",
		SessionsClose:        "Close session",
		SessionsActive:       "The terminal session stays alive in the background.",
		SessionsSortNewest:   "Newest first",
		SessionsSortOldest:   "Oldest first",
		SessionsShow:         "Show",
		SessionsNew:          "New session",
		SessionsCloseConfirm: "Close it?",
		SessionsCloseCancel:  "Keep",

		IntegrationsEmpty:                "No integrations are installed.",
		IntegrationsEnable:               "Enabled",
		IntegrationsRunning:              "Running",
		IntegrationsOff:                  "Disabled",
		IntegrationsBroken:               "Broken",
		IntegrationsOrphan:               "Installed on disk, not recorded",
		IntegrationsHint:                 "Integrations are command-line tools floter discovers and runs in the terminal.",
		IntegrationsUninstall:            "Uninstall",
		IntegrationsDisconnect:           "Disconnect",
		IntegrationsRemovePackage:        "Remove package",
		IntegrationsDisconnectTitle:      func(name string) string { return "Disconnect " + name + "?" },
		IntegrationsDisconnectDetail:     "Floter will remove the connection and its command entries. The system tool itself will remain on disk.",
		IntegrationsRemovePackageTitle:   func(name string) string { return "Remove " + name + "?" },
		IntegrationsRemovePackageDetail:  "Floter will remove this linked extension package from the list. Files outside Floter will remain on disk.",
		IntegrationsKindCustom:           "Custom integration",
		IntegrationsKindNPM:              "NPM integration",
		IntegrationsKindSystem:           "System tool",
		IntegrationsKindPackage:          "Extension package",
		DismissNotice:                    "Dismiss notice",
		BasePlugins:                      "Base plugins",
		BasePluginsHint:                  "Built-in functionality that ships with floter. Disabling one also stops its background services and global hotkey.",
		BaseClipboard:                    "Clipboard history",
		BaseClipboardHint:                "Keeps recent text and images you copy. Favorites never expire.",
		BaseClipboardPrivacy:             "Everything you copy system-wide is recorded — stored locally only.",
		BaseBrowser:                      "Browser bookmarks & history",
		BaseBrowserHint:                  "Search your browser's bookmarks and history from the launcher.",
		BaseCalculator:                   "Calculator",
		BaseCalculatorHint:               "Evaluate expressions from the launcher and keep a history.",
		PluginConfigure:                  "Configure",
		IntegrationsUninstallTitle:       "Uninstall this integration",
		IntegrationsUninstallDescription: "Choose which components to remove. Program files are always removed during uninstall.",
		UninstallHostConfig:              "Host configuration",
		UninstallHostConfigHint:          "Settings and secrets managed by Floter",
		UninstallToolData:                "Tool data",
		UninstallToolDataHint:            "Sessions, completions, and health reports",
		UninstallArtifacts:               "Generated artifacts",
		UninstallArtifactsHint:           "Logs, caches, and user-generated files",
		IntegrationsCheck:                "Check",
		IntegrationsRecheck:              "Re-check",
		IntegrationsInstall:              "Install",
		IntegrationsPackageHint:          "npm package, e.g. @scope/name",
		IntegrationsVersionHint:          "version or range (optional)",
		RemoveTitle:                      func(name string) string { return "Uninstall " + name + "?" },
		RemoveDetail:                     "Floter will remove the installed integration package. Saved integration data will remain.",
		RemoveCustomTitle:                func(name string) string { return "Delete " + name + "?" },
		RemoveCustomDetail:               "This integration was created and stored by Floter. Deleting it removes its generated definition, command files, and script.",
		RemoveCustomWarning:              "This cannot be undone from Floter. Export your integrations first if you may need this script or configuration again.",
		RemoveCancel:                     "Cancel",
		RemoveDelete:                     "Delete integration",
		IntegrationsAdopt:                "Adopt",
		IntegrationsPermissions:          "Permissions",
		PermissionsEnforcedLabel:         "Floter enforces",
		PermissionsEnforcedHint:          "Floter refuses these requests until you approve them. Nothing is started and no environment is handed over without it.",
		PermissionsDisclosureLabel:       "Declared, not blocked",
		PermissionsDisclosureHint:        "The tool says it uses these. Floter does not intercept them \u2014 the command runs with your own user rights.",
		PermissionBoundary:               "Local commands and scripts run as your operating-system user. These controls do not create a sandbox.",
		IntegrationsRecommended:          "Recommended tools",
		IntegrationsRecommendedHint:      "Floter ships a package for these; connecting installs it the same way any local tool is installed.",
		IntegrationsConnect:              "Connect",
		IntegrationsInstallInTerminal:    "Install in terminal",
		IntegrationsInstallTool:          "Install tool first",
		IntegrationsReconnect:            "Reconnect",
		IntegrationsReconnectUnboundHint: "No matching tool was found on PATH. Reconnect to search again.",
		IntegrationsHealthy:              "healthy",
		IntegrationsHealth:               "Health",
		IntegrationsDegraded:             "degraded",
		IntegrationsUnhealthy:            "not working",
		IntegrationsDiagnostics:          "Diagnostics",
		IntegrationsHealthChecks:         "Checks",
		IntegrationsHealthFailures:       "Failures",
		IntegrationsHealthRetryable:      "Retryable",
		IntegrationsHealthExitCode:       func(code string) string { return "exit: " + code },
		IntegrationsHealthCheckedAt:      func(when string) string { return "Checked " + when },
		ApprovalRecord:                   "Approval record",
		ApprovalRecordNone:               "No approval is recorded for this integration.",
		ApprovalRecordApprovedAt:         "Approved",
		ApprovalRecordPermissions:        "Approved permissions",
		ApprovalRecordPermissionsNone:    "None recorded",
		ApprovalRecordDigest:             "Manifest at approval",
		ApprovalRecordDigestCurrent:      "Manifest on disk now",
		ApprovalRecordDigestUnknown:      "Not recorded",
		ApprovalRecordChanged:            "The manifest has changed since this approval. Re-approve to cover the current version.",
		IntegrationsFailureDetail:        "Failure detail",
		ConfigOpen:                       "Configure",
		ConfigSave:                       "Save",
		ConfigSaved:                      "Saved",
		ConfigRequired:                   "required",
		ConfigNumber:                     "a number is expected",
		IntegrationsExport:               "Export",
		IntegrationsImport:               "Import",
		IntegrationsExported:             func(count int, path string) string { return strconv.Itoa(count) + " integrations exported to " + path },
		IntegrationsImported:             integrationsImportedEN,
		IntegrationsTransferBad:          "The file is not an integrations export",
		IntegrationsAdoptHint:            "This package directory has no record in the extension repository. Adopting grafts it in, with the same permission review an install has.",
		IntegrationsDeleteOrphan:         "Delete",
		IntegrationsCommands:             "Commands",
		IntegrationsCommandsHint:         "Turn a command on for direct output — in the search box, type the command name and a space, then its arguments. The command runs and its output is printed right there.",
		IntegrationsUnavailable:          "runtime unavailable",
		IntegrationsInstalled:            "Installed",
		EditLabel:                        "Edit\u2026",
		LocalToolSave:                    "Save",
		ParamsTitle:                      "Parameters",
		ParamsHint:                       "Declare the inputs this tool accepts. Each becomes its own argument when it runs.",
		ParamsEmpty:                      "None declared \u2014 the tool runs fine without inputs.",
		ParamsAdd:                        "Add an input",
		ParamsRemove:                     "Remove",
		ParamsID:                         "ID",
		ParamsLabel:                      "Label",
		ParamsKind:                       "Type",
		ParamsFlag:                       "Flag",
		ParamsRequired:                   "Required",
		ParamsOptions:                    "Choices",
		ParamsOptionsHint:                "One per line.",
		ParamsRow:                        func(number int) string { return "Input " + strconv.Itoa(number) },
		ParamsKindText:                   "Text",
		ParamsKindNumber:                 "Number",
		ParamsKindBoolean:                "Switch",
		ParamsKindSelect:                 "Choice",
		ParamsKindPath:                   "Path",
		RunLabel:                         "Run with these values",
		RunOutputHint:                    "The values are passed as separate arguments.",
		RunLastOutput:                    "Last output",
		RunViewOutput:                    "View output",
		RunHideOutput:                    "Hide output",
		RunNoOutput:                      "(No output)",
		RunOutputTruncated:               "Output was truncated",
		RunOutputTruncatedTail:           "\u2026 output truncated at 64 KB",
		RunParamRequired: func(label string) string {
			return "Required: fill in " + label + " before running"
		},
		RunParamInvalid: func(label string) string {
			return label + " is not a valid value for this parameter"
		},
		RunParamUnknown: func(label string) string {
			return "No parameter is declared with the id " + label
		},
		RunParamWindowsUnsafe: func(label string) string {
			return label + " contains a character (& | < > ^ %) that cmd.exe would re-parse; .cmd/.bat scripts cannot take it safely"
		},
		RunErrorScriptMissing: func(path string) string {
			return "This integration's script file is missing: " + path + ". Edit the integration and save it again to restore it."
		},
		RunErrorProgramMissing: func(path string) string {
			return "Could not start the program because it does not exist: " + path + ". Reconnect the integration, or check that the tool is still installed."
		},
		RunErrorNotExecutable: func(path string) string {
			return "The program exists but is not executable: " + path + ". Check its file permissions."
		},
		RunErrorInterpreterMissing: func(language, names, searched string) string {
			return "No " + language + " interpreter was found. Looked for: " + names + ".\nDirectories searched:\n" + searched
		},
		RunErrorSpawnFailed: func(path, detail string) string {
			return "Could not start " + path + ": " + detail
		},
		RunErrorTimeout: func(seconds string) string {
			return "The run did not finish within " + seconds + " seconds and was stopped."
		},
		RunErrorKilled: func(id string) string {
			return "The run was stopped because " + id + " was disabled or uninstalled."
		},
		RunErrorIntegrationDisabled: func(id string) string {
			return "Integration " + id + " is disabled"
		},
		RunErrorIntegrationBroken: func(id, reason string) string {
			return "Integration " + id + " is broken: " + reason
		},
		RunErrorTaskFailed: func(message string) string {
			return "Run task failed: " + message
		},
		// R11 · the readable half of an unavailable runtime binding: the code
		// labels the old host's dictionary carried, and the three detail
		// sentences a keyed binding detail interpolates.
		ErrorCodeLabel: func(code string) string {
			switch code {
			case "protocol-unsupported":
				return "Protocol version not supported"
			case "describe-parse-failed":
				return "Provider output parsing failed"
			case "stdout-contaminated":
				return "Provider output contaminated"
			case "protocol-error":
				return "Protocol error"
			case "tool-error":
				return "Tool execution error"
			case "timeout":
				return "Operation timed out"
			case "cancelled":
				return "Operation cancelled"
			case "binding-missing":
				return "Executable not found"
			case "binding-changed":
				return "Executable changed"
			case "binding-check-failed":
				return "Binding check failed"
			case "identity-mismatch":
				return "Provider identity mismatch"
			case "manifest-missing":
				return "Manifest missing or invalid"
			case "invalid-descriptor":
				return "Invalid provider descriptor"
			case "integrity-mismatch":
				return "Integrity check failed"
			case "runtime-unavailable":
				return "Runtime unavailable"
			case "manifest-unreadable":
				return "Manifest unreadable"
			case "provider-failed":
				return "Provider failed"
			case "verification-failed":
				return "Verification failed"
			case "probe_failed":
				return "Health check failed"
			}
			return code
		},
		BindingChangedDetail: func(path string) string {
			return "Executable changed: " + path + " — possibly a tool upgrade; click Re-check to recover"
		},
		BindingMissingDetail: func(path string) string {
			return "Executable not found: " + path
		},
		RuntimeUnavailableDetail: func(names string) string {
			return "Runtime unavailable: no " + names + " found on the search path"
		},
		RunSucceeded: func(name, duration string) string {
			return name + " finished in " + duration
		},
		RunFailed: func(name string, code int) string {
			return name + " failed with exit code " + strconv.Itoa(code)
		},
		RunOutputLines: func(lines int) string {
			return strconv.Itoa(lines) + " lines of output"
		},
		RunOutputLinesTruncated: func(lines int) string {
			return strconv.Itoa(lines) + " lines \u00b7 truncated at 64 KB"
		},
		DisableTitle:                func(name string) string { return "Disable " + name + "?" },
		DisableDescription:          "Floter stops any running command at once. This only marks the integration disabled \u2014 its files and data are kept, and you can turn it back on whenever you like.",
		DisableCancel:               "Cancel",
		DisableConfirm:              "Disable",
		DetectedTitle:               "Found on this machine",
		InstallFolderLabel:          "Install from a folder",
		InstallFolderHint:           "A package directory on this machine that carries a floter extension manifest.",
		InstallFolderChoose:         "Choose\u2026",
		InstallReviewTitle:          func(name string) string { return "Connect " + name + "?" },
		InstallReviewHint:           "Review the package details and requested access before connecting.",
		InstallReviewSource:         "Source",
		InstallReviewLocalDirectory: "Local package",
		InstallReviewRuntime:        "Runtime",
		InstallReviewPlatforms:      "Supported platforms",
		InstallReviewUnavailable:    "Not available",
		InstallReviewBoundary:       "Environment inheritance and requests for Floter to start another program are enforced by Floter. Other access is declared for disclosure and audit; native commands still run with your operating-system user rights and are not sandboxed.",
		InstallReviewConfirm:        "Confirm connection",
		DetectedHint:                "Programs floter can see but nothing has connected yet. Connecting opens the same review as any other tool.",
		RegisterHighlight:           "The link you opened points at this tool",
		RegisterNotFound: func(name string) string {
			return "No tool named \u201c" + name + "\u201d was found on this device. Install it, then connect it from the list below."
		},
		RegisterAlreadyConnected: func(name string) string {
			return "\u201c" + name + "\u201d is already connected \u2014 see it in the list above."
		},
		Freshness:                           "Command list",
		FreshnessNever:                      "Never re-scanned",
		FreshnessJustNow:                    "just now",
		FreshnessRelative:                   freshnessRelativeEN,
		FreshnessProbeSource:                "from the last re-scan",
		FreshnessHealthSource:               "from the health report",
		FreshnessResultRunning:              "Scanning\u2026",
		FreshnessResultSuccess:              "Succeeded",
		FreshnessResultDegraded:             "Succeeded with warnings",
		FreshnessResultFailed:               "Failed",
		FreshnessResultUnknown:              "Unknown",
		FreshnessCommands:                   "Commands",
		FreshnessCommandsUnknown:            "Unknown",
		FreshnessDeltaIncrease:              func(count int) string { return "+" + strconv.Itoa(count) + " since the scan before" },
		FreshnessDeltaDecrease:              func(count int) string { return "\u2212" + strconv.Itoa(count) + " since the scan before" },
		FreshnessDeltaUnchanged:             "No change since the scan before",
		FreshnessDeltaUnknown:               "No earlier scan to compare with",
		FreshnessDot:                        func(state string) string { return "Command status: " + state },
		FreshnessDotSynced:                  "in sync",
		FreshnessDotChanged:                 "changed upstream",
		FreshnessDotUnknown:                 "unknown, not compared",
		DeepLinkTitle:                       "Open from a link",
		DeepLinkHint:                        "Floter answers floter:// links from other apps. Copy one below and paste it into a browser or a terminal.",
		DeepLinkCopy:                        "Copy link",
		DeepLinkCopied:                      "Link copied",
		DeepLinkRejected:                    "That link could not be opened",
		LocalToolTitle:                      "Connect a local tool",
		LocalToolHint:                       "Point floter at a program on this machine; it becomes an integration with its own commands.",
		LocalToolAdd:                        "Add a local tool",
		LocalToolName:                       "Name",
		LocalToolProgram:                    "Program",
		LocalToolChoose:                     "Choose\u2026",
		LocalToolCommand:                    "Command",
		LocalToolCommandHint:                "The word that summons it (the program's name by default).",
		LocalToolArgs:                       "Arguments",
		LocalToolArgsHint:                   "Passed before the tool's own arguments.",
		LocalToolVersionArgs:                "Version arguments",
		LocalToolVersionArgsHint:            "How the tool reports its version, e.g. --version.",
		LocalToolVersion:                    "Version",
		LocalToolVersionHint:                "A successful version probe overwrites this value; if the probe fails, this is the fallback.",
		LocalToolAdvanced:                   "Advanced",
		LocalToolAdvancedPermissions:        "Permissions & audit",
		LocalToolPlatforms:                  "Supported platforms",
		LocalToolPlatformRequired:           "Select at least one supported platform",
		LocalToolNameLength:                 "Custom integration name must contain 1 to 80 characters",
		LocalToolCommandShape:               "Command must start with a letter or number and contain only lowercase letters, numbers, hyphens, or underscores",
		LocalToolVersionShape:               "Version must be a semantic version like 1.0.0",
		LocalToolModeInvalid:                "Custom integration mode must be executable or script",
		LocalToolScriptEmpty:                "Custom provider script cannot be empty",
		LocalToolScriptLanguageInvalid:      "Script language is only valid for script integrations",
		LocalToolDescription:                "Description (optional)",
		LocalToolDescriptionPlaceholder:     "Leave blank to show no description",
		LocalToolOutput:                     "Output",
		LocalToolTerminal:                   "In the terminal",
		LocalToolBackground:                 "Show the output",
		LocalToolConnect:                    "Connect",
		LocalToolFailed:                     "Could not connect this tool",
		IntegrationsReprobe:                 "Re-scan commands",
		IntegrationsCommandAlias:            "Alias",
		IntegrationsCommandAliasPlaceholder: "the word you type for it",
		IntegrationsCommandAliasTaken:       "another command uses this alias",

		ShortcutsToggle:   "Show / hide floter",
		ShortcutsHint:     "The global keys floter answers, and the ones you bind yourself.",
		ShortcutRecord:    "Record",
		ShortcutRecording: "Press keys\u2026",
		ShortcutRejected:  "Already used by another app",

		ShortcutsApp:     "App keys",
		ShortcutsAppHint: "These are answered while floter's window has the keyboard; the summon key above is the one the system holds.",
		ShortcutNames: map[string]string{
			"toggle_window":          "Show / hide floter",
			"new_command":            "New command",
			"open_external_terminal": "Open a terminal window",
			"copy_selection":         "Copy the selection",
			"paste":                  "Paste",
			"open_settings":          "Open settings",
			"select_result":          "Run the n-th result",
		},

		ShortcutsCustom:      "Custom shortcuts",
		ShortcutsCustomHint:  "Bind a key to a plugin, an action, or a command line. A command line runs with no window at all.",
		ShortcutsNone:        "No custom shortcuts yet.",
		ShortcutsAdd:         "Add",
		ShortcutsRemove:      "Remove",
		ShortcutsCommand:     "Command line",
		ShortcutsCommandHint: "Run silently, e.g. say done",
		ShortcutsRecordKey:   "Record key",
		ShortcutsReset:       "Restore the shipped keys",
		ShortcutsRejected:    shortcutsRejectedEN,
		ShortcutsActions: []Option{
			{"plugin:clipboard", "Clipboard history"},
			{"plugin:browser", "Browser history"},
			{"plugin:calculator", "Calculator"},
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
		BrowserSort:            "Order",
		BrowserSortHint:        "How the results are ordered.",
		BrowserSearchField:     "Search in",
		BrowserSearchFieldHint: "Which part of a row the query is matched against.",
		BrowserCDPEnabled:      "Read tabs through the debug port",
		BrowserCDPEnabledHint:  "Needs the browser started with --remote-debugging-port. On macOS tabs are read through AppleScript instead.",
		BrowserCDPPort:         "Debug port",
		BrowserCDPPortHint:     "The port the browser's DevTools endpoint listens on.",
		BrowserSectionData:     "Data source",
		BrowserSectionSearch:   "Search scope",
		BrowserSectionTabs:     "Open tabs",
		SectionGeneral:         "General",
		UnitItems:              "items",
		UnitDays:               "days",
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
		ClipboardClear:        "Clear history",
		ClipboardClearHint:    "Removes every entry that is not a favourite.",
		ClipboardClearTitle:   "Clear the clipboard history?",
		ClipboardClearDetail:  "Every entry that is not a favourite is removed. This cannot be undone.",
		ClipboardClearButton:  "Clear",
		ClipboardClearCancel:  "Cancel",
		ClipboardClearFailed:  "Could not clear the history. Try again.",

		CalculatorPlugin:         "Calculator",
		CalculatorMaxItems:       "History size",
		CalculatorMaxItemsHint:   "How many entries to keep; favourites are never dropped.",
		CalculatorRetention:      "Keep for",
		CalculatorRetentionHint:  "How long a calculation is kept; favourites never expire.",
		CalculatorCopyMode:       "Enter copies",
		CalculatorCopyModeHint:   "What Enter copies from a history row.",
		CalculatorRetentionNever: "Never expire",
		CalculatorRetentionDays:  calculatorRetentionEN,
		CalculatorCopyModes: []Option{
			{"full", "Expression and result"},
			{"result", "Result only"},
		},
		CalculatorClearFailed: "Could not clear the calculations. Try again.",
		ParamInvalidID:        "Use letters, numbers, dots, hyphens or underscores",
		ParamDuplicateID:      "This parameter ID is already used",
		ParamNeedsOptions:     "A choice needs its options.",
		ParamNeedsFlag:        "This argument needs its flag.",
		ParamInvalidFlag:      "A flag must start with '-' and contain no spaces or shell characters",
		LocalToolMode:         "Kind",
		LocalToolModeProgram:  "Program",
		LocalToolModeScript:   "Script",
		LocalToolLanguage:     "Language",
		LocalToolLanguageHint: "What the script is written in. The toolchain has to be on this machine.",
		LocalToolScript:       "Script",
		LocalToolScriptHint:   "The provider script. Parameters you declare arrive as argv.",
		LocalToolRuntimeReady: func(name, version string) string {
			if version == "" {
				return name
			}
			return name + " " + version
		},
		LocalToolRuntimeMissing: func(names string) string {
			return "Not found on PATH: " + names + ". Install it, then reopen this form to check again."
		},
		LocalToolRuntimeCompiled: "built before running",
		LocalToolRuntimeChecking: "Checking the local toolchain…",
		// The save-half of the same story: a script may be saved without its
		// toolchain (the user may install it later), so the sentence says what
		// will happen rather than refusing. Fixed by the old build's
		// `customScriptRuntimeSaveWarning`.
		LocalToolRuntimeSaveWarning: func(language string) string {
			return language + " is not available on this machine yet. The integration will be saved, but it cannot run until the toolchain is installed."
		},

		UpdateCheck:      "Check for updates",
		UpdateChecking:   "Checking\u2026",
		UpdateCurrent:    "floter is up to date",
		UpdateDisabled:   "This build has no update feed",
		UpdateFailed:     "Could not check for updates",
		UpdateAvailable:  func(version string) string { return "floter " + version + " is available" },
		UpdateInstall:    "Download and install",
		UpdateInstalling: func(percent int) string { return "Downloading\u2026 " + strconv.Itoa(percent) + "%" },
		UpdateInstalled:  "The update is installed; floter restarts on the next launch",

		AboutVersion:      "Version",
		AboutFramework:    "Framework",
		AboutScheme:       "Link scheme",
		AboutSettingsFile: "Settings file",
		AboutProject:      "Project",

		Percent: percentEN,
	},
	Terminal: Terminal{
		Title:             "Terminal",
		Pin:               "Pin output",
		SettingsOpen:      "Terminal settings",
		SettingsOpenHint:  "Appearance settings for this terminal",
		SettingsClose:     "Close terminal settings",
		Hint:              "The session has not started yet.",
		EmptyTitle:        "No terminal session yet",
		EmptyNew:          "New blank session",
		EmptyBack:         "Back to search",
		Close:             "Close terminal",
		NewCommand:        "New command",
		NewCommandHint:    "Back to the search field",
		OpenExternal:      "Open in terminal",
		SessionAttached:   "Running · attached",
		SessionDetached:   "Running · detached",
		ProcessExited:     func(code int) string { return "Process exited \u00b7 code " + strconv.Itoa(code) },
		ProcessExitedHint: "Output retained \u2014 close when you are done.",
		OpenExternalHint:  "Opens the system's own terminal window",
	},
}

var zh = Copy{
	Notifications: Notifications{
		Title:                    "floter",
		IntegrationInstalled:     func(name string) string { return name + " 已安装" },
		IntegrationInstallFailed: func(name string) string { return name + " 安装失败" },
		IntegrationRemoved:       func(name string) string { return name + " 已移除" },
		IntegrationRemoveFailed:  func(name string) string { return name + " 移除失败" },
		IntegrationChecked:       func(name string) string { return name + " 工作正常" },
		IntegrationReconnected:   func(name string) string { return name + " 已重新连接" },
		IntegrationRepaired:      func(name string) string { return name + " 已修复" },
		IntegrationRechecked:     func(name string) string { return name + " 已重新检测：状态正常" },
		IntegrationCheckFailed:   func(name string) string { return name + " 报告了问题" },
		CommandFinished:          func(name string) string { return name + " 已完成" },
		CommandStatus:            func(status string) string { return status },
		ClipboardCleared:         func(name string) string { return name + " 已清空" },
		ClipboardClearFailed:     func(name string) string { return name + " 清空失败" },
	},
	Launcher: Launcher{
		Placeholder:              "输入命令或应用名称",
		TrayShow:                 "显示 floter",
		TraySettings:             "设置\u2026",
		TrayReload:               "重新扫描",
		TrayQuit:                 "退出",
		ConfigurePlugin:          "选项",
		ConfigurePluginClose:     "关闭选项",
		ConfigurePluginHint:      "配置此插件",
		ConfigurePluginCloseHint: "关闭插件设置",
		FieldSessions:            "会话",
		FieldSettings:            "设置",
		FieldSessionsHint:        "管理终端会话",
		FieldSettingsHint:        func(shortcut string) string { return "设置（" + shortcut + "）" },
		ScopeBrowser:             "浏览器",
		ScopeClipboard:           "剪贴板",
		ScopeCalculator:          "计算器",
		ScopePlugin:              "插件",
		BrowserFilterHint:        "Tab 切换",
		ClipboardFilterHint:      func(shortcut string) string { return "Tab 切换 \u00b7 " + shortcut + " 收藏" },
		CalculatorFilterHint:     func(shortcut string) string { return "Tab 切换 \u00b7 " + shortcut + " 收藏" },
		BareTerminal:             "打开终端",
		BareTerminalHint:         "一个新的会话，不输入任何命令",
		TriggerHint:              func(name string) string { return name + " \u00b7 进入" },
		FilesSection:             "拖入的文件",
		FilesMore:                func(count int) string { return "等 " + strconv.Itoa(count) + " 个文件" },
		FilesMoreHint:            "显示全部拖入的文件",
		RecentSection:            "最近启动",
		TriggerHintMore: func(name string, more int) string {
			return name + " \u00b7 进入（还有 " + strconv.Itoa(more) + " 个）"
		},
		AliasMatch:            func(alias string) string { return "别名：" + alias },
		Label:                 "搜索",
		Hint:                  "输入以搜索",
		NoResults:             "没有匹配项",
		Clear:                 "清除",
		Copied:                "已复制",
		CommandModeHint:       "回车运行 \u00b7 Tab 补全 \u00b7 Esc 取消",
		DirectOutputHint:      "按空格直接运行，输出显示在搜索界面",
		CommandConflict:       "命令冲突",
		ResultsLabel:          "结果",
		CommandSettings:       "打开设置",
		CommandSettingsHint:   "外观、窗口行为与集成",
		CommandTerminal:       "打开终端",
		CommandTerminalHint:   "启动一个 shell 会话",
		CommandBrowser:        "浏览器历史",
		CommandBrowserHint:    "搜索历史记录与书签",
		CommandClipboard:      "剪贴板历史",
		CommandClipboardHint:  "搜索复制过的内容",
		CommandCalculator:     "计算器",
		CommandCalculatorHint: "求值一个算式，并保留历史",
		CommandQuit:           "退出 floter",
		CommandQuitHint:       "完全退出应用",
		OutputEmpty:           "命令没有输出",
		OutputSucceeded:       "已完成",
		OutputTimedOut:        "已超时",
		OutputTruncated:       "输出已截断",
		OutputHint:            "回车复制 · Esc 关闭",
		OutputListHint:        "↑↓ 选择 · 回车执行 · Esc 关闭",
		OutputFailed:          "退出码 %d",
		OutputLoading:         "正在加载…",
		OutputEnd:             "没有更多结果",

		ClipboardFavoriteFailed:  "无法修改收藏",
		ClipboardDeleteFailed:    "无法删除该条目",
		ClipboardCopyFailed:      "无法复制该条目",
		ClipboardDeleted:         "已删除条目",
		HistoryDelete:            "删除",
		HistoryPin:               "钉为独立窗口",
		DeleteConfirm:            "再按一次确认删除",
		HistoryCleared:           "已清空命令历史",
		HistoryFavorite:          "收藏",
		HistoryUnfavorite:        "取消收藏",
		OutputDetach:             "在窗口中打开",
		OutputDetachHint:         "在独立窗口中打开这份输出，不再随主面板消失",
		OutputWindowFallback:     "floter 命令",
		OutputWindowRerun:        "重新执行",
		OutputWindowClose:        "关闭",
		OutputWindowIdle:         "还没有执行过",
		CalculatorDeleted:        "已删除计算记录",
		CalculatorDeleteFailed:   "无法删除该计算记录",
		CalculatorEmpty:          "还没有计算记录",
		CalculatorEmptyFavorites: "还没有收藏",
		ClipboardEmpty:           "暂无复制记录",
		ClipboardEmptyFilter:     "没有匹配项",
		ClipboardEmptyFavorites:  "暂无收藏",
		ClipboardDisabled:        "剪贴板历史已关闭。可在“设置 > 集成”中启用。",
		BrowserEmpty:             "没有匹配项",
		CalculatorRecordFailed:   "无法保存这条计算",
		CalculatorFavoriteFailed: "无法修改收藏",
		CalculatorYesterday:      "昨天",
		PowerRestart:             "重启电脑",
		PowerRestartHint:         "会先确认，然后重启",
		PowerShutdown:            "关机",
		PowerShutdownHint:        "会先确认，然后关机",
		PowerCancel:              "取消",
		PowerRestartAction:       "重启",
		PowerShutdownAction:      "关机",
		PowerConfirmMessage: func(action string) string {
			if action == "shutdown" {
				return "现在关机？未保存的工作会丢失。"
			}
			return "现在重启？未保存的工作会丢失。"
		},
		PowerFailed:               "无法让系统执行该操作",
		OpenInBrowser:             "在浏览器打开",
		OpenInFiles:               "在文件管理器打开",
		OpenFile:                  "打开文件",
		RunInShell:                "在终端运行",
		InstallTool:               func(name string) string { return "安装 " + name },
		InvokeFailed:              "无法启动此工具",
		OpenAppFailed:             "无法打开此应用",
		OpenPathFailed:            "无法打开此路径",
		OpenURLFailed:             "无法打开此链接",
		OpenTermFailed:            "无法在系统终端中继续",
		FileOpen:                  "打开",
		FileCd:                    "在此打开终端",
		FileCopyPath:              "复制路径",
		FileFolder:                "文件夹",
		BrowserNoProfile:          "没有找到浏览器配置",
		BrowserDisabled:           "浏览器书签与历史已停用。可在“设置 > 集成”中启用。",
		BrowserAll:                "全部",
		BrowserBookmarks:          "书签",
		BrowserHistory:            "历史记录",
		BrowserTabs:               "标签页",
		ClipboardFilterAll:        "全部",
		ClipboardFilterFavorites:  "收藏",
		ClipboardTypeText:         "文本",
		ClipboardTypeImage:        "图片",
		ClipboardTypeLink:         "链接",
		ClipboardTypeFiles:        "文件",
		CalculatorFilterAll:       "全部",
		CalculatorFilterFavorites: "收藏",

		MenuView:     "视图",
		MenuLauncher: "显示 floter",
	},
	Settings: Settings{
		Title:     "设置",
		Close:     "关闭设置",
		CloseHint: "关闭设置（Esc）",
		Quit:      "退出 floter",
		QuitHint:  "完全退出应用程序",

		PermissionsTitle:    permissionsTitleZH,
		PermissionsAllow:    "允许",
		PermissionsCancel:   "取消",
		PermissionsEnforced: "Floter 强制拦截",
		PermissionsDeclared: "仅声明，不拦截",
		// 连接流程的权限审阅：原版 `permissionsRequired` /
		// `confirmConnectRecommended` / `connect`（i18n.ts:1352-1356）。
		PermissionsRequired:     "需要以下权限",
		ConnectRecommendedTitle: func(name string) string { return "将推荐工具 " + name + " 接入 Floter？" },
		PermissionsConnect:      "连接",
		// 选择运行工具的弹窗（i18n.ts:1359-1360）。
		ChooseSystemToolTitle: func(name string) string { return "为 " + name + " 选择运行工具" },
		ChooseSystemToolHint:  "发现了多个匹配的可执行文件，请明确选择 Floter 要绑定的路径。",

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
		SaveFailed:      "无法保存该修改。",
		SaveFailedAlert: "无法应用更改，请检查设置后重试。",

		GroupAppearance:   "外观",
		GroupWindow:       "窗口行为",
		GroupTerminal:     "终端外观",
		GroupMenuBar:      "菜单栏",
		GroupTransparency: "透明度",

		Theme:            "外观",
		ThemeHint:        "自动模式跟随系统外观。",
		AppIcon:          "应用图标",
		AppIconHint:      "菜单栏 / 托盘以及任务栏上显示的图标。默认为深色。",
		Language:         "语言",
		LanguageHint:     "选择后立即生效。",
		Material:         "玻璃",
		Glass:            "液态玻璃效果",
		GlassHint:        "液态玻璃的效果强度：扭曲、折射，以及所有控件的透镜质感，从磨砂玻璃到苹果液态玻璃拉满。与下方透明度相互独立。",
		TransparencyHint: "每个窗口底色的实底程度，与玻璃效果相互独立。100% 时接近不透明。",
		MainOpacity:      "应用背景透明度",
		TerminalOpacity:  "终端背景透明度",

		TransparencyScaleLow:  "透明",
		TransparencyScaleHigh: "不透明",

		Scale:      "界面大小",
		ScaleHint:  "缩放所有界面元素——文字、按钮与输入框。默认为「小」。",
		ScaleSmall: "小",
		ScaleLarge: "大",

		TerminalHint:   "更改会立即应用到正在运行的会话。",
		FontSize:       "字号",
		FontFamily:     "字体",
		CursorShape:    "光标形状",
		CursorBlink:    "光标闪烁",
		LineHeight:     "行距",
		Padding:        "边距",
		Palette:        "终端配色",
		PasteSafe:      "粘贴不执行",
		PalettePreview: "配色预览",

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

		LaunchAtStartup:             "开机自启动",
		LaunchAtStartupHint:         "登录系统时自动启动 floter。",
		HideOnBlur:                  "失去焦点时隐藏",
		HideOnBlurHint:              "切换到其他应用时自动收起面板。",
		ShowRecentInLauncher:        "显示最近启动的应用",
		ShowRecentInLauncherHint:    "启动器为空时显示常用应用",
		ShowMenubarIcon:             "显示菜单栏图标",
		ShowMenubarIconHint:         "显示菜单栏 / 托盘图标；隐藏后全局快捷键仍然可用。",
		ShowCommandsInSearch:        "在搜索中显示 PATH 命令",
		ShowCommandsInSearchHint:    "把 PATH 上找到的可执行文件作为启动器结果提供。",
		SurfaceResidency:            "页面驻留时间",
		SurfaceResidencyHint:        "插件页面、其设置或终端在面板收起后会保留这么久；设为 0 则关闭。",
		SurfaceResidencyOff:         "关闭",
		SurfaceResidencyNever:       "永不关闭",
		SurfaceResidencyValue:       secondsZH,
		SurfaceResidencyCustom:      "自定义…",
		SurfaceResidencyCustomValue: customSecondsZH,
		SurfaceResidencyCustomLabel: "自定义秒数",
		SurfaceResidencyCustomUnit:  "秒",
		SurfaceResidencyCustomApply: "确定",

		SessionsNone:         "没有正在运行的会话。",
		SessionsClose:        "关闭会话",
		SessionsActive:       "终端会话在后台继续运行。",
		SessionsSortNewest:   "最新优先",
		SessionsSortOldest:   "最早优先",
		SessionsShow:         "显示",
		SessionsNew:          "新建会话",
		SessionsCloseConfirm: "关闭它？",
		SessionsCloseCancel:  "保留",

		IntegrationsEmpty:                "没有已安装的集成。",
		IntegrationsEnable:               "启用",
		IntegrationsRunning:              "运行中",
		IntegrationsOff:                  "已停用",
		IntegrationsBroken:               "已损坏",
		IntegrationsOrphan:               "磁盘上有包，但没有记录",
		IntegrationsHint:                 "「集成」是 floter 发现并在终端里运行的命令行工具。",
		IntegrationsUninstall:            "卸载",
		IntegrationsDisconnect:           "断开连接",
		IntegrationsRemovePackage:        "移除扩展包",
		IntegrationsDisconnectTitle:      func(name string) string { return "断开 " + name + "？" },
		IntegrationsDisconnectDetail:     "Floter 会移除连接和对应命令，但系统工具本身仍会保留。",
		IntegrationsRemovePackageTitle:   func(name string) string { return "移除 " + name + "？" },
		IntegrationsRemovePackageDetail:  "Floter 会从列表中移除此本地扩展包，但不会删除 Floter 之外的文件。",
		IntegrationsKindCustom:           "自定义集成",
		IntegrationsKindNPM:              "NPM 集成",
		IntegrationsKindSystem:           "系统工具",
		IntegrationsKindPackage:          "扩展包",
		DismissNotice:                    "关闭提示",
		BasePlugins:                      "基础插件",
		BasePluginsHint:                  "随 floter 附带的内置功能。关闭后会同时停止其后台监听与全局快捷键。",
		BaseClipboard:                    "剪贴板历史",
		BaseClipboardHint:                "保留最近复制的文本与图片，收藏永不过期。",
		BaseClipboardPrivacy:             "将记录系统内所有复制内容，仅保存在本地",
		BaseBrowser:                      "浏览器书签与历史记录",
		BaseBrowserHint:                  "在启动器中搜索浏览器的书签和历史记录。",
		BaseCalculator:                   "计算器",
		BaseCalculatorHint:               "在启动器中计算表达式并保留历史。",
		PluginConfigure:                  "配置",
		IntegrationsUninstallTitle:       "卸载这个集成",
		IntegrationsUninstallDescription: "选择要移除的组件。程序文件在卸载时始终会被移除。",
		UninstallHostConfig:              "宿主配置",
		UninstallHostConfigHint:          "由 Floter 管理的设置和密钥",
		UninstallToolData:                "工具数据",
		UninstallToolDataHint:            "会话、补全和健康报告",
		UninstallArtifacts:               "生成产物",
		UninstallArtifactsHint:           "日志、缓存和用户生成的文件",
		IntegrationsCheck:                "检测",
		IntegrationsRecheck:              "重新检测",
		IntegrationsInstall:              "安装",
		IntegrationsPackageHint:          "npm 包名，例如 @scope/name",
		IntegrationsVersionHint:          "版本或范围（可空）",
		RemoveTitle:                      func(name string) string { return "卸载 " + name + "？" },
		RemoveDetail:                     "Floter 会移除已安装的集成包，但保留集成数据。",
		RemoveCustomTitle:                func(name string) string { return "删除 " + name + "？" },
		RemoveCustomDetail:               "该集成由 Floter 创建并保存。删除后，其生成的定义、命令文件和脚本都会被移除。",
		RemoveCustomWarning:              "此操作无法在 Floter 中撤销。如需保留脚本或配置，请先导出集成。",
		RemoveCancel:                     "取消",
		RemoveDelete:                     "删除集成",
		IntegrationsAdopt:                "接管",
		IntegrationsPermissions:          "权限",
		PermissionsEnforcedLabel:         "Floter 强制拦截",
		PermissionsEnforcedHint:          "未经你批准，Floter 会拒绝这类请求，不会启动程序，也不会交出环境变量。",
		PermissionsDisclosureLabel:       "仅声明，不拦截",
		PermissionsDisclosureHint:        "工具声明会用到这些能力。Floter 不拦截，命令以你本人的用户权限运行。",
		PermissionBoundary:               "本地命令和脚本以当前系统用户身份运行，这些选项不会创建安全沙箱。",
		IntegrationsRecommended:          "推荐工具",
		IntegrationsRecommendedHint:      "floter 自带这些工具的包；接管与安装本地工具走同一条路径。",
		IntegrationsConnect:              "接管",
		IntegrationsInstallInTerminal:    "在终端中安装",
		IntegrationsInstallTool:          "请先安装工具",
		IntegrationsReconnect:            "重新连接",
		IntegrationsReconnectUnboundHint: "未在 PATH 中找到匹配的工具，重新连接以再次搜索。",
		IntegrationsHealthy:              "健康",
		IntegrationsHealth:               "健康状态",
		IntegrationsDegraded:             "部分功能可用",
		IntegrationsUnhealthy:            "无法工作",
		IntegrationsDiagnostics:          "诊断结果",
		IntegrationsHealthChecks:         "检查项",
		IntegrationsHealthFailures:       "失败项",
		IntegrationsHealthRetryable:      "可重试",
		IntegrationsHealthExitCode:       func(code string) string { return "退出码：" + code },
		IntegrationsHealthCheckedAt:      func(when string) string { return "检查时间：" + when },
		ApprovalRecord:                   "批准记录",
		ApprovalRecordNone:               "该集成没有批准记录。",
		ApprovalRecordApprovedAt:         "批准时间",
		ApprovalRecordPermissions:        "已批准权限",
		ApprovalRecordPermissionsNone:    "无记录",
		ApprovalRecordDigest:             "批准时的清单",
		ApprovalRecordDigestCurrent:      "当前清单",
		ApprovalRecordDigestUnknown:      "未记录",
		ApprovalRecordChanged:            "此批准之后清单已变化，建议重新审批以覆盖当前版本。",
		IntegrationsFailureDetail:        "故障详情",
		ConfigOpen:                       "配置",
		ConfigSave:                       "保存",
		ConfigSaved:                      "已保存",
		ConfigRequired:                   "必填",
		ConfigNumber:                     "这里要填数字",
		IntegrationsExport:               "导出",
		IntegrationsImport:               "导入",
		IntegrationsExported: func(count int, path string) string {
			return "已导出 " + strconv.Itoa(count) + " 个集成到 " + path
		},
		IntegrationsImported:     integrationsImportedZH,
		IntegrationsTransferBad:  "这个文件不是集成导出文件",
		IntegrationsAdoptHint:    "这个包目录在扩展仓库里没有记录。接管会把它正式登记进来，权限审查与安装一致。",
		IntegrationsDeleteOrphan: "删除",
		IntegrationsCommands:     "命令列表",
		IntegrationsCommandsHint: "打开开关后即为「直出」：在搜索框输入命令名加空格呼出，随后输入的内容即为命令参数，命令输出直接显示在搜索界面。",
		IntegrationsUnavailable:  "运行时不可用",
		IntegrationsInstalled:    "已安装",
		EditLabel:                "编辑\u2026",
		LocalToolSave:            "保存",
		ParamsTitle:              "参数",
		ParamsHint:               "声明这个工具接受的输入。运行时会各自成为一个参数。",
		ParamsEmpty:              "未声明——不带输入也能正常运行。",
		ParamsAdd:                "添加输入",
		ParamsRemove:             "移除",
		ParamsID:                 "标识",
		ParamsLabel:              "名称",
		ParamsKind:               "类型",
		ParamsFlag:               "旗标",
		ParamsRequired:           "必填",
		ParamsOptions:            "选项",
		ParamsOptionsHint:        "每行一个。",
		ParamsRow:                func(number int) string { return "输入 " + strconv.Itoa(number) },
		ParamsKindText:           "文本",
		ParamsKindNumber:         "数字",
		ParamsKindBoolean:        "开关",
		ParamsKindSelect:         "选项",
		ParamsKindPath:           "路径",
		RunLabel:                 "使用这些值运行",
		RunOutputHint:            "这些值会作为独立参数传入。",
		RunLastOutput:            "最近一次输出",
		RunViewOutput:            "查看输出",
		RunHideOutput:            "收起输出",
		RunNoOutput:              "（无输出）",
		RunOutputTruncated:       "输出已被截断",
		RunOutputTruncatedTail:   "……输出已截断（上限 64KB）",
		RunParamRequired: func(label string) string {
			return "必填：运行前请填写 " + label
		},
		RunParamInvalid: func(label string) string {
			return label + " 不是该参数的有效值"
		},
		RunParamUnknown: func(label string) string {
			return "没有声明 id 为 " + label + " 的参数"
		},
		RunParamWindowsUnsafe: func(label string) string {
			return label + " 含 cmd.exe 会重新解析的字符（& | < > ^ %），.cmd/.bat 脚本无法安全接收"
		},
		RunErrorScriptMissing: func(path string) string {
			return "该集成的脚本文件已丢失：" + path + "。请打开集成重新保存以恢复。"
		},
		RunErrorProgramMissing: func(path string) string {
			return "无法启动程序，文件不存在：" + path + "。请重新连接该集成，或确认工具仍然安装。"
		},
		RunErrorNotExecutable: func(path string) string {
			return "程序存在但没有可执行权限：" + path + "。请检查文件权限。"
		},
		RunErrorInterpreterMissing: func(language, names, searched string) string {
			return "未找到 " + language + " 解释器。查找过：" + names + "。\n已搜索目录：\n" + searched
		},
		RunErrorSpawnFailed: func(path, detail string) string {
			return "无法启动 " + path + "：" + detail
		},
		RunErrorTimeout: func(seconds string) string {
			return "运行在 " + seconds + " 秒内没有完成，已停止。"
		},
		RunErrorKilled: func(id string) string {
			return id + " 已停用或卸载，运行已终止。"
		},
		RunErrorIntegrationDisabled: func(id string) string {
			return "集成 " + id + " 已停用"
		},
		RunErrorIntegrationBroken: func(id, reason string) string {
			return "集成 " + id + " 已损坏：" + reason
		},
		RunErrorTaskFailed: func(message string) string {
			return "运行任务失败：" + message
		},
		ErrorCodeLabel: func(code string) string {
			switch code {
			case "protocol-unsupported":
				return "协议版本不支持"
			case "describe-parse-failed":
				return "Provider 输出解析失败"
			case "stdout-contaminated":
				return "Provider 输出被污染"
			case "protocol-error":
				return "协议错误"
			case "tool-error":
				return "工具执行错误"
			case "timeout":
				return "操作超时"
			case "cancelled":
				return "操作已取消"
			case "binding-missing":
				return "可执行文件未找到"
			case "binding-changed":
				return "可执行文件已变更"
			case "binding-check-failed":
				return "绑定检查失败"
			case "identity-mismatch":
				return "Provider 身份不匹配"
			case "manifest-missing":
				return "清单缺失或无效"
			case "invalid-descriptor":
				return "Provider 描述符无效"
			case "integrity-mismatch":
				return "完整性校验未通过"
			case "runtime-unavailable":
				return "运行时不可用"
			case "manifest-unreadable":
				return "清单无法读取"
			case "provider-failed":
				return "提供方执行失败"
			case "verification-failed":
				return "校验失败"
			case "probe_failed":
				return "健康检查失败"
			}
			return code
		},
		BindingChangedDetail: func(path string) string {
			return "可执行文件已变更：" + path + "（可能是工具升级所致，点「重新检测」即可恢复）"
		},
		BindingMissingDetail: func(path string) string {
			return "可执行文件不存在：" + path
		},
		RuntimeUnavailableDetail: func(names string) string {
			return "运行时不可用：搜索路径中找不到 " + names
		},
		RunSucceeded: func(name, duration string) string {
			return name + " 已完成，用时 " + duration
		},
		RunFailed: func(name string, code int) string {
			return name + " 运行失败，退出码 " + strconv.Itoa(code)
		},
		RunOutputLines: func(lines int) string {
			return "输出 " + strconv.Itoa(lines) + " 行"
		},
		RunOutputLinesTruncated: func(lines int) string {
			return "输出 " + strconv.Itoa(lines) + " 行 \u00b7 截断于 64KB"
		},
		DisableTitle:                func(name string) string { return "停用 " + name + "？" },
		DisableDescription:          "正在运行的命令会立即停止。这里只是标记停用——文件与数据都会保留，随时可以重新启用。",
		DisableCancel:               "取消",
		DisableConfirm:              "停用",
		DetectedTitle:               "在本机发现",
		InstallFolderLabel:          "从文件夹安装",
		InstallFolderHint:           "本机上一个带有 floter 扩展清单的包目录。",
		InstallFolderChoose:         "选择\u2026",
		InstallReviewTitle:          func(name string) string { return "连接 " + name + "？" },
		InstallReviewHint:           "连接前请检查扩展包详情和请求的访问权限。",
		InstallReviewSource:         "来源",
		InstallReviewLocalDirectory: "本地扩展包",
		InstallReviewRuntime:        "运行时",
		InstallReviewPlatforms:      "支持的平台",
		InstallReviewUnavailable:    "暂无",
		InstallReviewBoundary:       "环境继承，以及要求 Floter 启动其他程序的执行描述，由 Floter 强制控制。其余访问仅用于披露和审计；原生命令仍以当前系统用户权限运行，不受操作系统沙箱隔离。",
		InstallReviewConfirm:        "确认连接",
		DetectedHint:                "Floter 能看到但还没连接的程序。连接时会走和其他工具一样的权限确认。",
		RegisterHighlight:           "你打开的链接指向此工具",
		RegisterNotFound: func(name string) string {
			return "此设备上没有名为“" + name + "”的工具。请先安装，再从下方列表接入。"
		},
		RegisterAlreadyConnected: func(name string) string {
			return "“" + name + "”已接入——请在上方列表中查看。"
		},
		Freshness:                           "命令列表",
		FreshnessNever:                      "尚未重新扫描",
		FreshnessJustNow:                    "刚刚",
		FreshnessRelative:                   freshnessRelativeZH,
		FreshnessProbeSource:                "来自上次重新扫描",
		FreshnessHealthSource:               "来自健康报告",
		FreshnessResultRunning:              "扫描中\u2026",
		FreshnessResultSuccess:              "成功",
		FreshnessResultDegraded:             "成功但有警告",
		FreshnessResultFailed:               "失败",
		FreshnessResultUnknown:              "未知",
		FreshnessCommands:                   "命令数",
		FreshnessCommandsUnknown:            "未知",
		FreshnessDeltaIncrease:              func(count int) string { return "比上次多 " + strconv.Itoa(count) + " 个" },
		FreshnessDeltaDecrease:              func(count int) string { return "比上次少 " + strconv.Itoa(count) + " 个" },
		FreshnessDeltaUnchanged:             "与上次相比没有变化",
		FreshnessDeltaUnknown:               "没有更早的扫描可供比较",
		FreshnessDot:                        func(state string) string { return "命令状态：" + state },
		FreshnessDotSynced:                  "已同步",
		FreshnessDotChanged:                 "上游已变",
		FreshnessDotUnknown:                 "未知（尚未对比）",
		DeepLinkTitle:                       "从链接打开",
		DeepLinkHint:                        "Floter 会响应其他应用发来的 floter:// 链接。复制下面的一条，粘贴到浏览器或终端即可。",
		DeepLinkCopy:                        "复制链接",
		DeepLinkCopied:                      "已复制链接",
		DeepLinkRejected:                    "无法打开该链接",
		LocalToolTitle:                      "连接本机工具",
		LocalToolHint:                       "指向这台机器上的一个程序，它就会成为一个带有自己命令的集成。",
		LocalToolAdd:                        "添加本机工具",
		LocalToolName:                       "名称",
		LocalToolProgram:                    "程序",
		LocalToolChoose:                     "选择\u2026",
		LocalToolCommand:                    "命令",
		LocalToolCommandHint:                "唤起它时输入的命令词（默认用程序名）。",
		LocalToolArgs:                       "参数",
		LocalToolArgsHint:                   "在工具自身参数之前传入。",
		LocalToolVersionArgs:                "版本参数",
		LocalToolVersionArgsHint:            "工具如何报告版本，例如 --version。",
		LocalToolVersion:                    "版本",
		LocalToolVersionHint:                "探测到真实版本会自动覆盖；探测失败时用你填的值兜底。",
		LocalToolAdvanced:                   "高级",
		LocalToolAdvancedPermissions:        "权限声明与审计",
		LocalToolPlatforms:                  "支持的平台",
		LocalToolPlatformRequired:           "请至少选择一个支持的平台",
		LocalToolNameLength:                 "自定义集成名称必须为 1 到 80 个字符",
		LocalToolCommandShape:               "命令必须以字母或数字开头，且只能包含小写字母、数字、连字符或下划线",
		LocalToolVersionShape:               "版本必须是形如 1.0.0 的语义化版本",
		LocalToolModeInvalid:                "自定义集成模式必须是可执行文件或脚本",
		LocalToolScriptEmpty:                "自定义提供程序脚本不能为空",
		LocalToolScriptLanguageInvalid:      "脚本语言只适用于脚本类型的集成",
		LocalToolDescription:                "描述（可选）",
		LocalToolDescriptionPlaceholder:     "留空则不显示描述",
		LocalToolOutput:                     "输出",
		LocalToolTerminal:                   "在终端中",
		LocalToolBackground:                 "显示输出",
		LocalToolConnect:                    "连接",
		LocalToolFailed:                     "无法连接此工具",
		IntegrationsReprobe:                 "重新扫描命令",
		IntegrationsCommandAlias:            "别名",
		IntegrationsCommandAliasPlaceholder: "你唤起它时输入的词",
		IntegrationsCommandAliasTaken:       "别名已被其他命令占用",

		ShortcutsToggle:   "显示 / 隐藏 floter",
		ShortcutsHint:     "floter 响应的全局按键，以及你自己绑定的按键。",
		ShortcutRecord:    "录制",
		ShortcutRecording: "请按组合键…",
		ShortcutRejected:  "该组合键已被其他应用占用",

		ShortcutsApp:     "应用按键",
		ShortcutsAppHint: "这些按键在 floter 窗口拥有键盘时生效；上面那条唤回键是系统持有的那一个。",
		ShortcutNames: map[string]string{
			"toggle_window":          "显示 / 隐藏 floter",
			"new_command":            "新命令",
			"open_external_terminal": "打开终端窗口",
			"copy_selection":         "复制选中内容",
			"paste":                  "粘贴",
			"open_settings":          "打开设置",
			"select_result":          "运行第 N 个结果",
		},

		ShortcutsCustom:      "自定义快捷键",
		ShortcutsCustomHint:  "把一个按键绑定到插件、动作或命令行；命令行会完全静默地运行。",
		ShortcutsNone:        "还没有自定义快捷键。",
		ShortcutsAdd:         "添加",
		ShortcutsRemove:      "移除",
		ShortcutsCommand:     "命令行",
		ShortcutsCommandHint: "静默运行，例如 say done",
		ShortcutsRecordKey:   "录制按键",
		ShortcutsReset:       "恢复默认按键",
		ShortcutsRejected:    shortcutsRejectedZH,
		ShortcutsActions: []Option{
			{"plugin:clipboard", "剪贴板历史"},
			{"plugin:browser", "浏览器历史"},
			{"plugin:calculator", "计算器"},
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
		BrowserSort:            "排序",
		BrowserSortHint:        "结果的排序方式。",
		BrowserSearchField:     "搜索范围",
		BrowserSearchFieldHint: "查询匹配行里的哪些字段。",
		BrowserCDPEnabled:      "通过调试端口读取标签页",
		BrowserCDPEnabledHint:  "需要浏览器以 --remote-debugging-port 启动。macOS 上改用 AppleScript 读取。",
		BrowserCDPPort:         "调试端口",
		BrowserCDPPortHint:     "浏览器 DevTools 端点监听的端口。",
		BrowserSectionData:     "数据来源",
		BrowserSectionSearch:   "搜索范围",
		BrowserSectionTabs:     "已打开标签页",
		SectionGeneral:         "通用",
		UnitItems:              "项",
		UnitDays:               "天",
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
		ClipboardClear:        "清空历史",
		ClipboardClearHint:    "移除所有未收藏的条目。",
		ClipboardClearTitle:   "清空剪贴板历史？",
		ClipboardClearDetail:  "所有未收藏的条目都会被移除，无法撤销。",
		ClipboardClearButton:  "清空",
		ClipboardClearCancel:  "取消",
		ClipboardClearFailed:  "无法清空历史，请重试。",

		CalculatorPlugin:         "计算器",
		CalculatorMaxItems:       "历史容量",
		CalculatorMaxItemsHint:   "最多保留多少条；收藏的条目不会被淘汰。",
		CalculatorRetention:      "保留时长",
		CalculatorRetentionHint:  "计算记录保留多久；收藏的条目不会过期。",
		CalculatorCopyMode:       "回车复制",
		CalculatorCopyModeHint:   "回车从历史行复制什么。",
		CalculatorRetentionNever: "永不过期",
		CalculatorRetentionDays:  calculatorRetentionZH,
		CalculatorCopyModes: []Option{
			{"full", "算式与结果"},
			{"result", "仅结果"},
		},
		CalculatorClearFailed: "无法清空计算记录，请重试。",
		ParamInvalidID:        "只能包含字母、数字、点、连字符和下划线",
		ParamDuplicateID:      "该参数 ID 已被使用",
		ParamNeedsOptions:     "选项型参数需要它的选项。",
		ParamNeedsFlag:        "这个参数需要它的旗标。",
		ParamInvalidFlag:      "参数前缀必须以 '-' 开头，且不能包含空格或 shell 字符",
		LocalToolMode:         "类型",
		LocalToolModeProgram:  "程序",
		LocalToolModeScript:   "脚本",
		LocalToolLanguage:     "语言",
		LocalToolLanguageHint: "脚本用什么语言写。这台机器上必须有对应的工具链。",
		LocalToolScript:       "脚本",
		LocalToolScriptHint:   "provider 脚本；你声明的参数会作为 argv 传入。",
		LocalToolRuntimeReady: func(name, version string) string {
			if version == "" {
				return name
			}
			return name + " " + version
		},
		LocalToolRuntimeMissing: func(names string) string {
			return "在 PATH 中未找到：" + names + "。安装后重新打开此表单即可再次检查。"
		},
		LocalToolRuntimeCompiled: "运行前先编译",
		LocalToolRuntimeChecking: "正在检查本机工具链…",
		LocalToolRuntimeSaveWarning: func(language string) string {
			return "本机尚未安装 " + language + "。集成会保存，但在安装工具链之前无法运行。"
		},

		UpdateCheck:      "检查更新",
		UpdateChecking:   "正在检查…",
		UpdateCurrent:    "floter 已是最新版本",
		UpdateDisabled:   "此构建没有更新源",
		UpdateFailed:     "无法检查更新",
		UpdateAvailable:  func(version string) string { return "有新版本 floter " + version },
		UpdateInstall:    "下载并安装",
		UpdateInstalling: func(percent int) string { return "正在下载… " + strconv.Itoa(percent) + "%" },
		UpdateInstalled:  "更新已安装，下次启动 floter 时生效",

		AboutVersion:      "版本",
		AboutFramework:    "框架",
		AboutScheme:       "链接协议",
		AboutSettingsFile: "设置文件",
		AboutProject:      "项目",

		Percent: percentEN,
	},
	Terminal: Terminal{
		Title:             "终端",
		Pin:               "钉住输出",
		SettingsOpen:      "终端设置",
		SettingsOpenHint:  "调整此终端的外观",
		SettingsClose:     "关闭终端设置",
		Hint:              "会话尚未启动。",
		EmptyTitle:        "暂无终端会话",
		EmptyNew:          "新建空白会话",
		EmptyBack:         "回到搜索",
		Close:             "关闭终端",
		NewCommand:        "新命令",
		NewCommandHint:    "回到搜索框",
		OpenExternal:      "在终端打开",
		SessionAttached:   "运行中 · 已连接",
		SessionDetached:   "运行中 · 未连接",
		ProcessExited:     func(code int) string { return "进程已退出 \u00b7 退出码 " + strconv.Itoa(code) },
		ProcessExitedHint: "输出已保留，看完后手动关闭。",
		OpenExternalHint:  "打开系统自己的终端窗口",
	},
}

func secondsEN(n uint32) string { return itoa32(n) + " s" }
func secondsZH(n uint32) string { return itoa32(n) + " 秒" }

func customSecondsEN(n uint32) string { return "Custom (" + secondsEN(n) + ")" }
func customSecondsZH(n uint32) string { return "自定义（" + secondsZH(n) + "）" }

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
