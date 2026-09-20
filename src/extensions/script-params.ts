// R9-2 slice 2 · the script-parameter definition, as data.
//
// The drawer is a large component the node runner does not render, so the two
// things that must not drift from the backend live here: the *shape* of a
// parameter definition, and the *validation* of one. The backend is the
// authority (`manifest::validate_param_definitions`), and this is the same
// rule projected forward so the editor can show an error under the row that
// caused it instead of waiting for a save round-trip.
//
// The flag whitelist is the front half of the argv injection defence: a flag
// becomes its own argv element at run time, so it must already be a single
// token here.

export type ScriptParamKind = "text" | "number" | "boolean" | "select" | "path";

/** The wire shape of `ParamDefinition` (camelCase). `default`, `placeholder`
 *  and `flag` are optional on the wire; the editor keeps them as `string |
 *  null` so an untouched row is distinguishable from an empty one. */
export type ScriptParam = {
  id: string;
  label: string;
  kind: ScriptParamKind;
  default: string | null;
  required: boolean;
  placeholder: string | null;
  options: string[];
  flag: string | null;
};

/** The wire shape as the backend serializes it: the optional fields are
 *  genuinely absent (`skip_serializing_if`), not null. Reading a manifest
 *  through this keeps the run-time form honest about which fields it can
 *  actually see. */
export type ScriptParamWire = {
  id: string;
  label?: string;
  kind: ScriptParamKind;
  default?: string | null;
  required?: boolean;
  placeholder?: string | null;
  options?: string[];
  flag?: string | null;
};

/** The order the kind picker offers. Mirrors `ParamKind` in Rust. */
export const PARAM_KINDS: readonly ScriptParamKind[] = ["text", "number", "boolean", "select", "path"];

/** `id` is a stable key, never handed to the script. Same whitelist as the
 *  configuration-field key rule (`config.rs`). */
export const PARAM_ID_PATTERN = /^[A-Za-z0-9._-]+$/;

/** One token, `-`-led, no shell metacharacter. Mirrors `validate_flag`. */
export const PARAM_FLAG_PATTERN = /^-[A-Za-z0-9_.-]+$/;

/** A fresh row. A blank id is deliberate: the user names it, and the validator
 *  is what refuses to save until they do. */
export const emptyParam = (): ScriptParam => ({
  id: "",
  label: "",
  kind: "text",
  default: null,
  required: false,
  placeholder: null,
  options: [],
  flag: null,
});

/** Which i18n key explains a bad row. The drawer renders the key; the tests
 *  assert on the key, so the wording stays in the dictionary. */
export type ParamIssueKey =
  | "settings.extensions.customParamInvalidId"
  | "settings.extensions.customParamDuplicateId"
  | "settings.extensions.customParamInvalidFlag"
  | "settings.extensions.customParamSelectNoOptions"
  | "settings.extensions.customParamInvalidDefault";

export type ParamIssue = { index: number; key: ParamIssueKey };

/** Parse a comma-separated options field into the wire array. Whitespace is
 *  trimmed and empties dropped, so a trailing comma does not create a blank
 *  choice. */
export const parseParamOptions = (value: string): string[] =>
  value
    .split(",")
    .map((option) => option.trim())
    .filter((option) => option.length > 0);

/** The inverse, for rendering a row's options back into its input. */
export const formatParamOptions = (options: readonly string[]): string => options.join(", ");

/** Whether a default is legal for its kind. `required` plus a default is
 *  *valid* — the default is a pre-fill, not an exemption. */
const defaultIsValid = (param: ScriptParam): boolean => {
  if (param.default === null) return true;
  switch (param.kind) {
    case "number":
      return param.default.trim().length > 0 && Number.isFinite(Number(param.default));
    case "boolean":
      return param.default === "true" || param.default === "false";
    case "select":
      return param.options.includes(param.default);
    case "text":
    case "path":
      return true;
  }
};

/**
 * Every reason this parameter list cannot be saved, in row order. Empty means
 * valid. This is the projection the drawer renders inline; the backend
 * re-checks the same rules before anything reaches disk.
 *
 * Mutation: drop the `PARAM_FLAG_PATTERN` branch and the injection row saves,
 * which the node suite asserts against.
 */
export const paramIssues = (params: readonly ScriptParam[]): ParamIssue[] => {
  const issues: ParamIssue[] = [];
  const seen = new Set<string>();
  params.forEach((param, index) => {
    if (!PARAM_ID_PATTERN.test(param.id)) {
      issues.push({ index, key: "settings.extensions.customParamInvalidId" });
    } else if (seen.has(param.id)) {
      issues.push({ index, key: "settings.extensions.customParamDuplicateId" });
    } else {
      seen.add(param.id);
    }
    if (param.flag !== null && param.flag.length > 0 && !PARAM_FLAG_PATTERN.test(param.flag)) {
      issues.push({ index, key: "settings.extensions.customParamInvalidFlag" });
    }
    if (param.kind === "select" && param.options.length === 0) {
      issues.push({ index, key: "settings.extensions.customParamSelectNoOptions" });
    }
    if (!defaultIsValid(param)) {
      issues.push({ index, key: "settings.extensions.customParamInvalidDefault" });
    }
  });
  return issues;
};

/** Normalize the editor's nullable fields to the wire shape: an empty string
 *  is an absent optional, not an empty flag. Called on save so a row the user
 *  cleared does not serialize `""`. */
export const toWireParams = (params: readonly ScriptParam[]): ScriptParam[] =>
  params.map((param) => ({
    ...param,
    label: param.label.trim(),
    default: param.default === null || param.default.length === 0 ? null : param.default,
    placeholder: param.placeholder === null || param.placeholder.length === 0 ? null : param.placeholder,
    flag: param.flag === null || param.flag.length === 0 ? null : param.flag,
  }));

/** Read the wire shape back into the editor's nullable shape. A manifest that
 *  predates the field (or a definition with no params) yields an empty list. */
export const fromWireParams = (params: readonly ScriptParamWire[] | null | undefined): ScriptParam[] =>
  (params ?? []).map((param) => ({
    id: param.id ?? "",
    label: param.label ?? "",
    kind: PARAM_KINDS.includes(param.kind) ? param.kind : "text",
    default: param.default ?? null,
    required: Boolean(param.required),
    placeholder: param.placeholder ?? null,
    options: [...(param.options ?? [])],
    flag: param.flag ?? null,
  }));
