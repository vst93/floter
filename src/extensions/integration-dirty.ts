// R9-4 · "has the user changed anything?" — asked semantically, not by bytes.
//
// The custom-integration drawer used to answer this with a raw
// `JSON.stringify(next) !== JSON.stringify(saved)` (ExtensionsPanel, R9-1 era).
// That comparison is about *serialization*, not about the user's intent, and
// the two disagree in exactly the cases a legacy manifest produces:
//
//   * a field the backend omits (`output`, `params`, `scriptLanguage`,
//     `scriptContent`) arrives as `undefined` / absent, while the editor
//     normalizes it to its default (`"background"`, `[]`, `"shell"`, `""`);
//   * `fromWireParams` always materializes the editor's nullable fields
//     (`default: null`, `options: []`, `flag: null`) where the wire shape
//     omitted them (`skip_serializing_if` on the Rust side).
//
// Byte comparison therefore reported "dirty" the instant an old integration
// was opened — before the user touched a key. Every close attempt then armed
// the discard confirmation, and the discard bar rendered *below* the form's
// scroll viewport, so the user could neither see it nor act on it. The drawer
// looked impossible to close. (Device report: "编辑弹窗没法关闭".)
//
// The fix is a comparison over meaning:
//
//   1. every field is normalized to one canonical shape first (`normalizeForm`),
//      so "absent" and "explicit default" are the same value;
//   2. lists that are order-insensitive in meaning (`permissions`, `platforms`)
//      compare as sets;
//   3. parameter rows compare field by field with the same null/empty folding
//      `toWireParams` already applies on save.
//
// This module is pure and DOM-free so the node suite can drive it directly, and
// the panel reads only its two exports. Mutation: swap `customIntegrationDirty`
// back to `JSON.stringify(a) !== JSON.stringify(b)` and the
// "an untouched edit of a legacy definition is not dirty" test goes red.

import { fromWireParams, type ScriptParam, type ScriptParamWire } from "./script-params.ts";

/** The subset of the drawer form this comparison needs. Structural (not the
 *  panel's `CustomIntegrationForm`) so the module keeps no import cycle with
 *  the panel, and so a future field is a visible decision here rather than an
 *  accidental participant. */
export type ComparableIntegrationForm = {
  mode: "executable" | "script";
  id: string;
  name: string;
  command: string;
  version: string;
  executablePath: string;
  scriptLanguage: string;
  scriptContent: string;
  argsPrefix: readonly string[];
  versionArgs: readonly string[];
  permissions: readonly string[];
  platforms: readonly string[];
  output: "background" | "terminal";
  params: readonly ScriptParam[] | null | undefined;
};

/** The canonical shape of one form for comparison purposes. Every field is
 *  present and non-nullable, so two forms that *mean* the same thing serialize
 *  identically. `undefined` inputs are filled with the same defaults the
 *  drawer applies when it loads a definition. */
type CanonicalForm = {
  mode: "executable" | "script";
  id: string;
  name: string;
  command: string;
  version: string;
  executablePath: string;
  scriptLanguage: string;
  scriptContent: string;
  argsPrefix: string[];
  versionArgs: string[];
  permissions: string[];
  platforms: string[];
  output: "background" | "terminal";
  params: CanonicalParam[];
};

type CanonicalParam = {
  id: string;
  label: string;
  kind: string;
  default: string | null;
  required: boolean;
  placeholder: string | null;
  options: string[];
  flag: string | null;
};

/** The editor's own default when a definition predates the field. Mirrors the
 *  literals the drawer writes on load (`?? "shell"`, `?? ""`, `?? "background"`)
 *  and the Rust serde defaults (`OutputMode::Background`, empty `params`). */
const LANGUAGE_FALLBACK = "shell";
const OUTPUT_FALLBACK = "background" as const;

/** A list that means the same thing regardless of order compares as a sorted,
 *  de-duplicated set. `permissions` and `platforms` are checkboxes: toggling
 *  one off and on again is not a change, and a definition that lists them in a
 *  different order is not a different definition. */
const asSet = (values: readonly string[] | null | undefined): string[] =>
  [...new Set(values ?? [])].sort();

/** `undefined`/`null`/`""` all mean "not set" for the optional string fields —
 *  the same folding `toWireParams` performs before a save. Without it, a row
 *  whose placeholder the user cleared would compare dirty forever. */
const optionalText = (value: string | null | undefined): string | null => {
  const text = value ?? "";
  return text.length === 0 ? null : text;
};

const canonicalParam = (param: ScriptParam): CanonicalParam => ({
  id: param.id ?? "",
  label: param.label ?? "",
  kind: param.kind ?? "text",
  default: optionalText(param.default),
  required: Boolean(param.required),
  placeholder: optionalText(param.placeholder),
  options: [...(param.options ?? [])],
  flag: optionalText(param.flag),
});

/** Read a value that may be either the editor's shape or the raw wire shape
 *  into the editor's shape. The drawer's saved ref is written from a definition
 *  the backend just returned, so a legacy entry can carry `ScriptParamWire`
 *  rows; comparing those against normalized editor rows must not report a
 *  difference. `fromWireParams` is idempotent for editor rows, which is what
 *  makes it safe to call on both sides. */
const asEditorParams = (
  params: readonly (ScriptParam | ScriptParamWire)[] | null | undefined,
): ScriptParam[] =>
  fromWireParams((params ?? []) as readonly ScriptParamWire[]);

/** One canonical projection per form. */
export const normalizeForm = (form: ComparableIntegrationForm): CanonicalForm => ({
  mode: form.mode ?? "executable",
  id: form.id ?? "",
  name: form.name ?? "",
  command: form.command ?? "",
  version: form.version ?? "",
  executablePath: form.executablePath ?? "",
  scriptLanguage: form.scriptLanguage ?? LANGUAGE_FALLBACK,
  scriptContent: form.scriptContent ?? "",
  argsPrefix: [...(form.argsPrefix ?? [])],
  versionArgs: [...(form.versionArgs ?? [])],
  permissions: asSet(form.permissions),
  platforms: asSet(form.platforms),
  output: form.output ?? OUTPUT_FALLBACK,
  params: asEditorParams(form.params).map(canonicalParam),
});

/** The canonical serialization, exported so a test can pin *why* two forms
 *  compare equal (and so a failure message can show both sides). */
export const canonicalFormKey = (form: ComparableIntegrationForm): string =>
  JSON.stringify(normalizeForm(form));

/**
 * Whether the live form differs in meaning from the saved one.
 *
 * `saved` is the definition as the backend returned it (possibly a legacy
 * manifest with fields absent); `current` is the editor's state. Both go
 * through [`normalizeForm`], so "absent" vs "explicit default" is not a
 * difference and the user's first keypress is the first real change.
 */
export const customIntegrationDirty = (
  current: ComparableIntegrationForm,
  saved: ComparableIntegrationForm,
): boolean => canonicalFormKey(current) !== canonicalFormKey(saved);
