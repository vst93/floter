export type Language = "en" | "zh";

// English is the source of truth: its keys define the message set, so a missing
// translation is a type error rather than a blank label at runtime.
const en = {
  "input.placeholder": "Type a command or app name",
  "launcher.runInShell": "Run in shell",
  "launcher.application": "Application",
  "launcher.systemApplication": "System application",
  "launcher.userApplication": "User application",
  "terminal.openInTerminal": "Open Current Directory in Terminal",
  "terminal.openInTerminalHint": "Open Current Directory in Terminal (⌘N)",
  "terminal.newCommand": "New command",
  "terminal.newCommandHint": "New command (⌘W)",
  "settings.open": "Settings",
  "settings.openHint": "Settings (⌘,)",
  "settings.title": "Settings",
  "settings.close": "Close settings",
  "settings.closeHint": "Close settings (Esc)",
  "settings.language": "Language",
  "settings.languageHint": "Applies immediately across the app.",
  "settings.language.en": "English",
  "settings.language.zh": "Chinese (Simplified)",
} as const;

export type MessageKey = keyof typeof en;

const zh: Record<MessageKey, string> = {
  "input.placeholder": "输入命令或应用名称",
  "launcher.runInShell": "在终端中运行",
  "launcher.application": "应用程序",
  "launcher.systemApplication": "系统应用",
  "launcher.userApplication": "用户应用",
  "terminal.openInTerminal": "在终端中打开当前目录",
  "terminal.openInTerminalHint": "在终端中打开当前目录（⌘N）",
  "terminal.newCommand": "新建命令",
  "terminal.newCommandHint": "新建命令（⌘W）",
  "settings.open": "设置",
  "settings.openHint": "设置（⌘,）",
  "settings.title": "设置",
  "settings.close": "关闭设置",
  "settings.closeHint": "关闭设置（Esc）",
  "settings.language": "语言",
  "settings.languageHint": "选择后立即生效。",
  "settings.language.en": "英语",
  "settings.language.zh": "简体中文",
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

export type Translate = (key: MessageKey) => string;

export const createTranslator = (language: Language): Translate => {
  const table = messages[language] ?? en;
  return (key) => table[key] ?? en[key];
};
