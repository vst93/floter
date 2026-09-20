// R9-1 · the script-language vocabulary, in one place.
//
// Three frontend surfaces used to answer "what is this language" separately:
// the `<select>` wrote the option list inline, `scriptTemplate` held its own
// record, and the export path ran a nested ternary for the file extension. A
// fourth language therefore needed four edits, and missing one produced a
// silently wrong extension or an empty template.
//
// This module is the single source. The *backend* remains the source of truth
// for what can actually run (`extensions_script_runtime_check` and
// `install::script_toolchain`); the `toolchain` field below is only the label
// for the status line, and the `compiled` flag mirrors the Rust classifier so
// the editor can say "this is built, not interpreted" before the check lands.
//
// Nothing here is translated: language names (`Python`, `Rust`) are proper
// nouns and stay identical in both dictionaries, which is why no i18n key
// exists for them.

export type ScriptLanguageId =
  | "js"
  | "shell"
  | "powershell"
  | "python"
  | "ruby"
  | "php"
  | "go"
  | "rust";

export type ScriptLanguageOption = {
  id: ScriptLanguageId;
  /** Shown in the picker. A proper noun; never translated. */
  label: string;
  /** File extension without the dot, for `provider.<ext>` and the export
   *  dialog's filter. Must match `install::script_extension` in Rust. */
  extension: string;
  /** Compiled languages are source-distributed build scripts: the toolchain
   *  produces an artifact and *that* is what Floter runs. */
  compiled: boolean;
  /** The toolchain binary name, for the status line's "not found" message. */
  toolchain: string;
  /** A minimal runnable script. Switching language swaps this in only while
   *  the editor still holds an untouched template (see the drawer). */
  template: string;
};

export const SCRIPT_LANGUAGES: readonly ScriptLanguageOption[] = [
  {
    id: "js",
    label: "JavaScript",
    extension: "js",
    compiled: false,
    toolchain: "node",
    template: "#!/usr/bin/env node\n\n// Floter provider script\n",
  },
  {
    id: "shell",
    label: "Shell",
    extension: "sh",
    compiled: false,
    toolchain: "sh",
    template: "#!/bin/sh\n\n# Floter provider script\n",
  },
  {
    id: "powershell",
    label: "PowerShell",
    extension: "ps1",
    compiled: false,
    toolchain: "pwsh",
    template: "#!/usr/bin/env pwsh\n\n# Floter provider script\n",
  },
  {
    id: "python",
    label: "Python",
    extension: "py",
    compiled: false,
    toolchain: "python3",
    template: "#!/usr/bin/env python3\n\n# Floter provider script\n",
  },
  {
    id: "ruby",
    label: "Ruby",
    extension: "rb",
    compiled: false,
    toolchain: "ruby",
    template: "#!/usr/bin/env ruby\n\n# Floter provider script\n",
  },
  {
    id: "php",
    label: "PHP",
    extension: "php",
    compiled: false,
    toolchain: "php",
    template: "<?php\n\n// Floter provider script\n",
  },
  {
    id: "go",
    label: "Go",
    extension: "go",
    compiled: true,
    toolchain: "go",
    template:
      "package main\n\nimport \"fmt\"\n\nfunc main() {\n\t// Floter provider script\n\tfmt.Println(\"floter provider\")\n}\n",
  },
  {
    id: "rust",
    label: "Rust",
    extension: "rs",
    compiled: true,
    toolchain: "rustc",
    template:
      "fn main() {\n    // Floter provider script\n    println!(\"floter provider\");\n}\n",
  },
] as const;

const byId = new Map(SCRIPT_LANGUAGES.map((language) => [language.id, language]));

/** Look a language up by its wire id. Falls back to JavaScript so a legacy
 *  manifest value this build does not know still renders a usable editor
 *  instead of an empty select. */
export const scriptLanguage = (id: string): ScriptLanguageOption =>
  byId.get(id as ScriptLanguageId) ?? SCRIPT_LANGUAGES[0];

export const scriptTemplate = (id: string): string => scriptLanguage(id).template;

export const scriptExtension = (id: string): string => scriptLanguage(id).extension;

/** Every template, for the "is the editor still holding an untouched one?"
 *  test the language switch uses to decide whether to swap the content. */
export const scriptTemplates = (): string[] =>
  SCRIPT_LANGUAGES.map((language) => language.template);

// ── the runtime check ──────────────────────────────────────────────────────

/** The shape `extensions_script_runtime_check` returns (camelCase on the
 *  wire). `null`/absent means "not asked yet" — never "missing". */
export type ScriptRuntimeCheck = {
  available: boolean;
  path: string | null;
  version: string | null;
  versionOutput: string | null;
  compiled: boolean;
  candidates: string[];
};

export type ScriptRuntimeStatus =
  | { state: "checking" }
  | { state: "available"; name: string; version: string | null; path: string | null; compiled: boolean }
  | { state: "missing"; names: string[] };

/** The inline status line under the language picker, as data.
 *
 *  Deliberately *not* an error: an absent toolchain does not block saving (a
 *  user may install Python and come back), so this projects to a muted line,
 *  never a red banner. The three states are distinct because "we have not
 *  looked" and "we looked and it is missing" are different claims.
 *
 *  No translator here: the module stays DOM- and dictionary-free so the node
 *  suite can drive every state without a renderer, and the drawer owns the
 *  words. */
export const scriptRuntimeStatus = (
  language: ScriptLanguageId,
  check: ScriptRuntimeCheck | null | undefined,
): ScriptRuntimeStatus => {
  if (!check) return { state: "checking" };
  const option = scriptLanguage(language);
  if (!check.available) {
    return {
      state: "missing",
      names: check.candidates.length ? check.candidates : [option.toolchain],
    };
  }
  const name = check.path ? check.path.split(/[\\/]/).pop() ?? option.toolchain : option.toolchain;
  return {
    state: "available",
    name,
    version: check.version,
    path: check.path,
    compiled: check.compiled,
  };
};
