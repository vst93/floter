// R9-2 slice 3 · the run-time parameter form, inline.
//
// The runtime half of "the user fills values in and the script receives them".
// It renders *inside* the row (never a dialog or an overlay): the same surface
// the output switch already lives on, reusing the run-routing projection so a
// row that cannot run cannot show a live Run button either.
//
// The forms are kind-driven, matching the four controls `ConfigFieldControl`
// uses for settings: text and path are text inputs, number is a decimal input,
// select is a native `<select>`, and boolean is the house switch. No value is
// ever turned into a command string here; the collected map goes to
// `extensions_run` and the backend owns argv.

import { AlertCircle, LoaderCircle, Play } from "lucide-react";
import type { Translate } from "../i18n";
import type { ScriptParam } from "./script-params";
import { collectParamValues, paramValueIssues, type ParamValues } from "./run-params";

type Props = {
  params: readonly ScriptParam[];
  values: ParamValues;
  /** R9-2 slice 4 · where this run's output will go, from the manifest. The
   *  form states it up front so the user knows before pressing Run whether
   *  the result lands on the terminal page or in a completion notice. */
  outputMode: "background" | "terminal";
  onChange: (id: string, value: string) => void;
  onRun: (values: ParamValues) => void;
  onCancel: () => void;
  busy: boolean;
  /** A backend refusal mapped to a message, if the last run failed on a value. */
  error?: string | null;
  t: Translate;
};

const kindLabelKey = (kind: ScriptParam["kind"]) =>
  `settings.extensions.customParamType.${kind}` as Parameters<Translate>[0];

export function RunParamForm({ params, values, outputMode, onChange, onRun, onCancel, busy, error, t }: Props) {
  const issues = paramValueIssues(params, values);
  const issueByIndex = new Map(issues.map((issue) => [issue.index, issue]));
  const canRun = issues.length === 0 && !busy;
  const outputHint = t(
    outputMode === "terminal"
      ? "settings.extensions.customRunOutputHintTerminal"
      : "settings.extensions.customRunOutputHintBackground",
  );

  return (
          <div className="extension-run-params" onClick={(event) => event.stopPropagation()}>
      <div className="extension-run-params__head">
        <span>{t("settings.extensions.customRunValues")}</span>
      </div>
      <p className="extension-run-params__hint">{t("settings.extensions.customRunValuesHint")}</p>
      {params.map((param, index) => {
        const value = values[param.id] ?? "";
        const issue = issueByIndex.get(index);
        const label = param.label.trim().length > 0 ? param.label : param.id;
        const describedBy = `run-param-${param.id}-issue`;
        return (
          <div className="extension-run-params__row" key={param.id}>
            <label className="extension-run-params__label" htmlFor={`run-param-${param.id}`}>
              <span>{label}</span>
              {param.required && <em>{t("settings.extensions.customParamRequired")}</em>}
              <small>{t(kindLabelKey(param.kind))}</small>
            </label>
            {param.kind === "boolean" ? (
              <button
                type="button"
                role="switch"
                id={`run-param-${param.id}`}
                aria-checked={value === "true"}
                aria-label={label}
                aria-describedby={issue ? describedBy : undefined}
                className={`settings-switch${value === "true" ? " settings-switch--active" : ""}`}
                disabled={busy}
                onClick={() => onChange(param.id, value === "true" ? "false" : "true")}
              >
                <span className="settings-switch__thumb" />
              </button>
            ) : param.kind === "select" ? (
              <select
                id={`run-param-${param.id}`}
                value={value}
                disabled={busy}
                aria-invalid={issue ? true : undefined}
                aria-describedby={issue ? describedBy : undefined}
                onChange={(event) => onChange(param.id, event.target.value)}
              >
                <option value="" />
                {param.options.map((option) => (
                  <option key={option} value={option}>{option}</option>
                ))}
              </select>
            ) : (
              <input
                id={`run-param-${param.id}`}
                type={param.kind === "number" ? "text" : "text"}
                inputMode={param.kind === "number" ? "decimal" : undefined}
                spellCheck={false}
                value={value}
                placeholder={param.placeholder ?? undefined}
                disabled={busy}
                aria-invalid={issue ? true : undefined}
                aria-describedby={issue ? describedBy : undefined}
                onChange={(event) => onChange(param.id, event.target.value)}
              />
            )}
            {issue && (
              <p className="extension-run-params__error" id={describedBy} role="alert">
                <AlertCircle size={12} strokeWidth={2} aria-hidden="true" />
                <span>{t(issue.key, { label: issue.label })}</span>
              </p>
            )}
          </div>
        );
      })}
      {error && (
        <p className="extension-run-params__error" role="alert">
          <AlertCircle size={12} strokeWidth={2} aria-hidden="true" />
          <span>{error}</span>
        </p>
      )}
      <p className="extension-run-params__note">{t("settings.extensions.customParamInjectionNote")}</p>
      {/* Where the output goes, at the foot of the form where the Run button
          is: one line naming the route the manifest already declares (the
          same two states the row's switch and the drawer's radio use). No new
          control — it only states the decision the user is about to act on. */}
      <p className="extension-run-params__output-hint">{outputHint}</p>
      <div className="extension-run-params__actions">
        <button type="button" className="extensions-action-button" disabled={busy} onClick={onCancel}>
          {t("settings.extensions.customRunCancel")}
        </button>
        <button
          type="button"
          className="extensions-action-button extensions-action-button--primary"
          disabled={!canRun}
          aria-busy={busy}
          onClick={() => onRun(collectParamValues(params, values))}
        >
          {busy ? (
            <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" />
          ) : (
            <Play size={14} strokeWidth={2} aria-hidden="true" />
          )}
          {t("settings.extensions.customRunConfirm")}
        </button>
      </div>
    </div>
  );
}
