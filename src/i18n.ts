export type Language = "en" | "zh";

// English is the source of truth: its keys define the message set, so a missing
// translation is a type error rather than a blank label at runtime.
const en = {
  "input.placeholder": "Type a command or app name",
  "input.scanning": "Scanning applications…",
  "launcher.runInShell": "Run in shell",
  "launcher.openInBrowser": "Open in browser",
  "launcher.openInFiles": "Open in files",
  "launcher.application": "Application",
  "launcher.systemApplication": "System application",
  "launcher.userApplication": "User application",
  "system.restart": "Restart",
  "system.restartSubtitle": "Restart the computer",
  "system.shutdown": "Shut Down",
  "system.shutdownSubtitle": "Turn off the computer",
  "terminal.openInTerminal": "Open Current Directory in Terminal",
  "terminal.openInTerminalHint": "Open Current Directory in Terminal ({shortcut})",
  "terminal.newCommand": "New command",
  "terminal.newCommandHint": "New command ({shortcut})",
  "settings.open": "Settings",
  "settings.openHint": "Settings ({shortcut})",
  "settings.title": "Settings",
  "settings.close": "Close settings",
  "settings.closeHint": "Close settings (Esc)",
  "settings.quit": "Quit floter",
  "settings.quitHint": "Exit the application completely",
  "settings.language": "Language",
  "settings.languageHint": "Applies immediately across the app.",
  "settings.language.en": "English",
  "settings.language.zh": "Chinese (Simplified)",
  "settings.theme": "Appearance",
  "settings.themeHint": "Auto follows your system appearance.",
  "settings.theme.dark": "Dark",
  "settings.theme.darkDescription": "Always dark, whatever the system does",
  "settings.theme.light": "Light",
  "settings.theme.lightDescription": "Always light, whatever the system does",
  "settings.theme.auto": "Auto",
  "settings.theme.autoDescription": "Follow the system appearance",
  "settings.shortcuts": "Shortcuts",
  "settings.shortcutsHint": "Click a shortcut, then press the new combination. Esc cancels.",
  "settings.shortcut.record": "Record a new shortcut",
  "settings.shortcut.recording": "Press keys…",
  "settings.shortcut.rejected": "Already used by another app",
  "shortcut.toggle_window": "Show / hide floter",
  "shortcut.toggle_window.description": "Works while floter is in the background",
  "shortcut.new_command": "New command",
  "shortcut.new_command.description": "Closes the terminal and returns to the input",
  "shortcut.open_external_terminal": "Open in external terminal",
  "shortcut.open_external_terminal.description":
    "Opens the current directory in the system terminal",
  "shortcut.copy_selection": "Copy selection",
  "shortcut.copy_selection.description": "Without a selection the key goes to the shell",
  "shortcut.paste": "Paste",
  "shortcut.paste.description": "Pastes the clipboard into the terminal",
  "shortcut.open_settings": "Open settings",
  "shortcut.open_settings.description": "Opens this panel",
  "shortcut.select_result": "Select result 1-9",
  "shortcut.select_result.description": "The other digits use the same modifiers",
} as const;

export type MessageKey = keyof typeof en;

const zh: Record<MessageKey, string> = {
  "input.placeholder": "输入命令或应用名称",
  "input.scanning": "正在扫描应用…",
  "launcher.runInShell": "在终端中运行",
  "launcher.openInBrowser": "在浏览器中打开",
  "launcher.openInFiles": "在文件管理器中打开",
  "launcher.application": "应用程序",
  "launcher.systemApplication": "系统应用",
  "launcher.userApplication": "用户应用",
  "system.restart": "重启",
  "system.restartSubtitle": "重启电脑",
  "system.shutdown": "关机",
  "system.shutdownSubtitle": "关闭电脑",
  "terminal.openInTerminal": "在终端中打开当前目录",
  "terminal.openInTerminalHint": "在终端中打开当前目录（{shortcut}）",
  "terminal.newCommand": "新建命令",
  "terminal.newCommandHint": "新建命令（{shortcut}）",
  "settings.open": "设置",
  "settings.openHint": "设置（{shortcut}）",
  "settings.title": "设置",
  "settings.close": "关闭设置",
  "settings.closeHint": "关闭设置（Esc）",
  "settings.quit": "退出 floter",
  "settings.quitHint": "完全退出应用程序",
  "settings.language": "语言",
  "settings.languageHint": "选择后立即生效。",
  "settings.language.en": "英语",
  "settings.language.zh": "简体中文",
  "settings.theme": "外观",
  "settings.themeHint": "自动模式跟随系统外观。",
  "settings.theme.dark": "深色",
  "settings.theme.darkDescription": "始终使用深色，不随系统变化",
  "settings.theme.light": "浅色",
  "settings.theme.lightDescription": "始终使用浅色，不随系统变化",
  "settings.theme.auto": "自动",
  "settings.theme.autoDescription": "跟随系统外观",
  "settings.shortcuts": "快捷键",
  "settings.shortcutsHint": "点击快捷键后按下新的组合键，按 Esc 取消。",
  "settings.shortcut.record": "录制新的快捷键",
  "settings.shortcut.recording": "请按组合键…",
  "settings.shortcut.rejected": "该组合键已被其他应用占用",
  "shortcut.toggle_window": "显示 / 隐藏 floter",
  "shortcut.toggle_window.description": "在后台运行时同样有效",
  "shortcut.new_command": "新建命令",
  "shortcut.new_command.description": "关闭终端并返回输入框",
  "shortcut.open_external_terminal": "在外部终端中打开",
  "shortcut.open_external_terminal.description": "在系统终端中打开当前目录",
  "shortcut.copy_selection": "复制所选内容",
  "shortcut.copy_selection.description": "没有选中内容时，按键会转发给 shell",
  "shortcut.paste": "粘贴",
  "shortcut.paste.description": "把剪贴板内容粘贴到终端",
  "shortcut.open_settings": "打开设置",
  "shortcut.open_settings.description": "打开当前面板",
  "shortcut.select_result": "选择第 1-9 项结果",
  "shortcut.select_result.description": "其余数字沿用相同的修饰键",
};

const messages: Record<Language, Record<MessageKey, string>> = { en, zh };

// Language options, labelled in their own script so they stay recognisable
// whichever language the UI is currently in.
export const LANGUAGE_OPTIONS: { value: Language; label: string; descriptionKey: MessageKey }[] = [
  { value: "en", label: "English", descriptionKey: "settings.language.en" },
  { value: "zh", label: "中文", descriptionKey: "settings.language.zh" },
];

export const normalizeLanguage = (value: string | undefined | null): Language =>
  value === "zh" ? "zh" : "en";

export type Translate = (key: MessageKey, params?: Record<string, string | number>) => string;

export const createTranslator = (language: Language): Translate => {
  const table = messages[language] ?? en;
  return (key, params) => {
    const template: string = table[key] ?? en[key];
    if (!params) return template;
    // `{shortcut}` and friends are filled in by the caller, so a message can
    // name a key combination the user has rebound.
    return template.replace(/\{(\w+)\}/g, (match, name: string) =>
      name in params ? String(params[name]) : match,
    );
  };
};
