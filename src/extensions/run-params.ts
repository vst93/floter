// R9-2 slice 3 · the pure half of the run-time parameter form.
//
// Slice 2 let a script author *declare* inputs. This slice is the other half:
// the user fills values in and the Rust run path turns them into argv. The
// panel is a large component the node runner does not render, so the two
// decisions that must not drift are projections here:
//
//   * how the inputs are seeded (session memory, then default, then blank) and
//     validated before a run is allowed;
//   * how the backend's `run_param_*` error keys become localised messages.
//
// The JSX reads these; the node suite drives them directly and reads the JSX
// facts off the source (the same split `run-routing.ts` established).
//
// The frontend never assembles argv. It collects a `Record<string, string>` and
// hands it to `extensions_run`; the backend owns the order, the flags and the
// conversion. That is the injection defence's front edge: a value here is data,
// never a command fragment.

import type { ScriptParam } from "./script-params";

/** Parameter answers keyed by parameter id. Values are always strings on the
 *  wire; the backend converts by kind. */
export type ParamValues = Record<string, string>;

/** The seeded value for one parameter: session memory wins, then the declared
 *  default, then a blank (or `"false"` for a boolean, which is the state its
 *  switch shows). A default is a pre-fill, not an exemption. */
export const seedParamValue = (param: ScriptParam, remembered?: string): string => {
  if (remembered !== undefined) return remembered;
  if (param.default !== null && param.default !== undefined) return param.default;
  return param.kind === "boolean" ? "false" : "";
};

/** Seed every input of a form. `remembered` is the same session's last answer
 *  for this integration, if any — the run-time form is a memory aid, never a
 *  persistence layer. */
export const seedParamValues = (
  params: readonly ScriptParam[],
  remembered?: ParamValues,
): ParamValues => {
  const values: ParamValues = {};
  for (const param of params) {
    values[param.id] = seedParamValue(param, remembered?.[param.id]);
  }
  return values;
};

/** What the frontend refuses before it even calls `extensions_run`. Mirrors the
 *  backend's required/number/select rules so the user sees the problem under the
 *  input that caused it instead of a round-trip error. The backend re-checks
 *  everything (it is the authority); this is the same rule projected forward. */
export type ParamValueIssueKey =
  | "settings.extensions.customParamRequiredMissing"
  | "settings.extensions.customParamInvalidValue";

/** Every key a `run_param_*` refusal from the backend can map to. */
export type ParamRunErrorKey =
  | ParamValueIssueKey
  | "settings.extensions.customParamUnknown"
  | "settings.extensions.customParamWindowsUnsafe";

export type ParamValueIssue = {
  index: number;
  key: ParamValueIssueKey;
  /** The label to interpolate, already resolved (`label` or the id). */
  label: string;
};

/** Every reason the current answers cannot run, in declaration order. Empty
 *  means the run may proceed. */
export const paramValueIssues = (
  params: readonly ScriptParam[],
  values: ParamValues,
): ParamValueIssue[] => {
  const issues: ParamValueIssue[] = [];
  params.forEach((param, index) => {
    const raw = values[param.id] ?? "";
    const label = param.label.trim().length > 0 ? param.label : param.id;
    const hasDefault = param.default !== null && param.default !== undefined && param.default !== "";
    if (raw.trim().length === 0 && param.required && !hasDefault) {
      issues.push({ index, key: "settings.extensions.customParamRequiredMissing", label });
      return;
    }
    if (raw.length === 0) return;
    if (param.kind === "number" && !Number.isFinite(Number(raw.trim()))) {
      issues.push({ index, key: "settings.extensions.customParamInvalidValue", label });
      return;
    }
    if (param.kind === "select" && !param.options.includes(raw)) {
      issues.push({ index, key: "settings.extensions.customParamInvalidValue", label });
      return;
    }
    if (param.kind === "boolean" && raw !== "true" && raw !== "false") {
      issues.push({ index, key: "settings.extensions.customParamInvalidValue", label });
    }
  });
  return issues;
};

/** Normalize the form to the wire shape: only declared ids, only non-empty
 *  values. A blank optional is *absent*, which is what the backend reads as
 *  "use the default / skip"; a blank required is caught by
 *  [`paramValueIssues`] before this is called. */
export const collectParamValues = (
  params: readonly ScriptParam[],
  values: ParamValues,
): ParamValues => {
  const collected: ParamValues = {};
  for (const param of params) {
    const raw = values[param.id];
    if (raw === undefined || raw === "") continue;
    collected[param.id] = raw;
  }
  return collected;
};

/** The localised message for a backend refusal, or `null` when the error did
 *  not come from the parameter path (so the caller keeps its ordinary message).
 *  The backend answers with a stable key and the id, never a prose string, so
 *  the wording stays in the dictionary. */
export const paramRunErrorMessage = (
  message: string,
  params: readonly ScriptParam[],
  translate: (key: ParamRunErrorKey, values: { label: string }) => string,
): string | null => {
  const match = /^run_param_(required|invalid|unknown|windows_unsafe):(.+)$/.exec(message.trim());
  if (!match) return null;
  const [, kind, id] = match;
  const param = params.find((candidate) => candidate.id === id);
  const label = param ? (param.label.trim().length > 0 ? param.label : param.id) : id;
  switch (kind) {
    case "required":
      return translate("settings.extensions.customParamRequiredMissing", { label });
    case "invalid":
      return translate("settings.extensions.customParamInvalidValue", { label });
    case "unknown":
      return translate("settings.extensions.customParamUnknown", { label });
    case "windows_unsafe":
      return translate("settings.extensions.customParamWindowsUnsafe", { label });
    default:
      return null;
  }
};
