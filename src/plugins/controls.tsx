// R29 · the generic configuration controls.
//
// One component per control kind the schema declares — a switch, a dropdown, a
// radio group, a slider, a number field, a text field and a checkbox group —
// each taking a `PluginConfigField` description and a value, and emitting the
// change upward. Nothing here knows which plugin it is configuring: the same
// `<PluginConfigControl field={…} value={…} onChange={…} />` renders the
// clipboard capacity slider and the browser's history-window field, because
// both are `{ type: "slider", min, max, step }`.
//
// The overlay (`PluginConfigOverlay.tsx`) is the only caller; a new plugin adds
// no control code, only a schema (`config-schema.ts`).

import { useState } from "react";
import type { Translate } from "../i18n";
import { SegmentedChoice, SettingsRow } from "../settings/SettingsRows";
import {
  type PluginConfigField,
  type PluginConfigOption,
  type PluginConfigValue,
  pluginOptionLabel,
} from "./config-schema";

/** What every control receives. `onChange` reports the field's key and its
 *  next value; the overlay owns normalization and persistence. `onRun` is the
 *  `action` control's counterpart: it runs the field's command and resolves to
 *  whether the backend accepted it. */
export type PluginControlProps = {
  t: Translate;
  field: PluginConfigField;
  value: PluginConfigValue;
  disabled?: boolean;
  onChange: (key: string, value: PluginConfigValue) => void;
  onRun?: (key: string) => Promise<boolean>;
};

/** The label + help stack a control's row starts with. R59 · the overlay's
 *  fields render the settings app's own `SettingsRow` label slot (see
 *  `PluginConfigRow`), so this bare-label helper is retired: one row primitive,
 *  one label face. */

function ToggleControl({ t, field, value, disabled, onChange }: PluginControlProps) {
  const on = value === true;
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      aria-label={t(field.labelKey)}
      // The settings screen's own switch, reused: one switch face in the app,
      // one place its accent fill is counted (see `accent-budget.test.ts`).
      className={`settings-switch${on ? " settings-switch--active" : ""}`}
      disabled={disabled}
      onClick={() => onChange(field.key, !on)}
    >
      <span className="settings-switch__thumb" />
    </button>
  );
}

function SelectControl({ t, field, value, disabled, onChange }: PluginControlProps) {
  if (field.type !== "select") return null;
  return (
    <select
      // R59 · the settings page's own select face: pill, `--glass-control-edge`
      // hairline and the field's inset shadow. One select in the app.
      className="settings-select plugin-config-select"
      aria-label={t(field.labelKey)}
      value={typeof value === "string" ? value : ""}
      disabled={disabled}
      onChange={(event) => onChange(field.key, event.target.value)}
    >
      {field.options.map((option) => (
        <option key={option.value} value={option.value}>
          {pluginOptionLabel(option, t)}
        </option>
      ))}
    </select>
  );
}

function RadioControl({ t, field, value, onChange }: PluginControlProps) {
  if (field.type !== "radio") return null;
  const current = typeof value === "string" ? value : "";
  // R59 · the settings pages' segmented control (`SegmentedChoice`), reused
  // whole: one track, pill segments, a radiogroup keyboard contract is the
  // component's own. The plugin contributes options, not a new selection face.
  return (
    <SegmentedChoice
      label={t(field.labelKey)}
      options={field.options.map((option) => ({
        value: option.value,
        label: pluginOptionLabel(option, t),
      }))}
      value={current as string}
      onChange={(next) => onChange(field.key, next)}
    />
  );
}

function CheckboxesControl({ t, field, value, disabled, onChange }: PluginControlProps) {
  if (field.type !== "checkboxes") return null;
  const picked = Array.isArray(value) ? value : [];
  const toggle = (option: PluginConfigOption) => {
    const next = picked.includes(option.value)
      ? picked.filter((entry) => entry !== option.value)
      : [...picked, option.value];
    onChange(field.key, next);
  };
  return (
    <span className="plugin-config-checkboxes" role="group" aria-label={t(field.labelKey)}>
      {field.options.map((option) => {
        const on = picked.includes(option.value);
        return (
          <label key={option.value} className="plugin-config-checkbox">
            <input
              type="checkbox"
              checked={on}
              disabled={disabled}
              onChange={() => toggle(option)}
            />
            <span>{pluginOptionLabel(option, t)}</span>
          </label>
        );
      })}
    </span>
  );
}

function NumberControl({ t, field, value, disabled, onChange }: PluginControlProps) {
  if (field.type !== "number") return null;
  const step = field.step && field.step > 0 ? field.step : 1;
  return (
    <span className="plugin-config-number">
      <input
        type="number"
        className="plugin-config-number__input"
        aria-label={t(field.labelKey)}
        min={field.min}
        max={field.max}
        step={step}
        value={typeof value === "number" ? value : field.min}
        disabled={disabled}
        onChange={(event) => {
          // An empty or half-typed field reports `NaN`; the overlay normalizes,
          // so passing the raw number keeps the caret usable while typing.
          const parsed = Number(event.target.value);
          onChange(field.key, Number.isFinite(parsed) ? parsed : field.min);
        }}
      />
      {field.unitKey && (
        <span className="plugin-config-number__unit">{t(field.unitKey)}</span>
      )}
    </span>
  );
}

function SliderControl({ t, field, value, disabled, onChange }: PluginControlProps) {
  if (field.type !== "slider") return null;
  const step = field.step && field.step > 0 ? field.step : 1;
  const current = typeof value === "number" ? value : field.min;
  return (
    <span className="plugin-config-slider">
      <input
        type="range"
        className="plugin-config-slider__input"
        aria-label={t(field.labelKey)}
        min={field.min}
        max={field.max}
        step={step}
        value={current}
        disabled={disabled}
        onChange={(event) => onChange(field.key, Number(event.target.value))}
      />
      <span className="plugin-config-slider__value">
        {current}
        {field.unitKey ? ` ${t(field.unitKey)}` : ""}
      </span>
    </span>
  );
}

function TextControl({ t, field, value, disabled, onChange }: PluginControlProps) {
  if (field.type !== "text") return null;
  return (
    <input
      type="text"
      className="plugin-config-text"
      aria-label={t(field.labelKey)}
      placeholder={field.placeholderKey ? t(field.placeholderKey) : undefined}
      value={typeof value === "string" ? value : ""}
      disabled={disabled}
      spellCheck={false}
      autoCapitalize="off"
      autoCorrect="off"
      onChange={(event) => onChange(field.key, event.target.value)}
    />
  );
}

/** R38 · a destructive command's control.
 *
 * Two-step, in place: the first press arms the row (the button becomes the
 * confirm, with the question beside it and a Cancel next to that), the second
 * runs it. No system dialog — the confirmation is part of the overlay, the same
 * surface that offers the action. A refused run falls back to the armed state's
 * sibling: the row prints the field's failure line and disarms, so the user is
 * not left thinking it worked. */
function ActionControl({ t, field, onRun }: PluginControlProps) {
  if (field.type !== "action") return null;
  const [armed, setArmed] = useState(false);
  const [failed, setFailed] = useState(false);

  if (!armed) {
    return (
      <span className="plugin-config-action">
        {failed && (
          <span className="plugin-config-action__status" role="status">
            {t(field.failedKey)}
          </span>
        )}
        <button
          type="button"
          className="plugin-config-action__button plugin-config-action__button--danger"
          onClick={() => {
            setFailed(false);
            setArmed(true);
          }}
        >
          {t(field.labelKey)}
        </button>
      </span>
    );
  }

  return (
    <span className="plugin-config-action plugin-config-action--armed" role="group" aria-label={t(field.labelKey)}>
      <span className="plugin-config-action__confirm">{t(field.confirmKey)}</span>
      <button
        type="button"
        className="plugin-config-action__button plugin-config-action__button--danger"
        onClick={() => {
          setArmed(false);
          void onRun?.(field.key).then((ok) => setFailed(ok === false));
        }}
      >
        {t(field.labelKey)}
      </button>
      <button
        type="button"
        className="plugin-config-action__button"
        onClick={() => setArmed(false)}
      >
        {t(field.cancelKey)}
      </button>
    </span>
  );
}

/** The control one field asks for. Unknown kinds render nothing rather than
 *  guessing — a schema is data, and a typo in it must not paint a wrong
 *  control. */
export function PluginConfigControl(props: PluginControlProps) {
  switch (props.field.type) {
    case "toggle":
      return <ToggleControl {...props} />;
    case "select":
      return <SelectControl {...props} />;
    case "radio":
      return <RadioControl {...props} />;
    case "checkboxes":
      return <CheckboxesControl {...props} />;
    case "number":
      return <NumberControl {...props} />;
    case "slider":
      return <SliderControl {...props} />;
    case "text":
      return <TextControl {...props} />;
    case "action":
      return <ActionControl {...props} />;
  }
}

/** R59 · one field's whole row, on the settings app's own `SettingsRow`
 *  primitive: label and help on the left, the control in the trailing slot.
 *
 *  The kinds whose control wants the row's full width — a free-text directory,
 *  a segmented track, the range that is the capacity slider — take
 *  `stacked`, which is the same modifier the settings pages use for their
 *  sliders and pickers. Everything else trails right, aligned on the row's own
 *  inset, so the right edge of a select, a number and a switch is one line. */
const STACKED_KINDS = new Set<PluginConfigField["type"]>([
  "text",
  "radio",
  "checkboxes",
  "slider",
]);

export function PluginConfigRow(props: PluginControlProps) {
  const { t, field } = props;
  return (
    <SettingsRow
      label={t(field.labelKey)}
      sublabel={field.helpKey ? t(field.helpKey) : undefined}
      stacked={STACKED_KINDS.has(field.type)}
      className={`plugin-config-field plugin-config-field--${field.type}`}
      control={<PluginConfigControl {...props} />}
    />
  );
}
