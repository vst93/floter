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

import type { Translate } from "../i18n";
import {
  type PluginConfigField,
  type PluginConfigOption,
  type PluginConfigValue,
  pluginOptionLabel,
} from "./config-schema";

/** What every control receives. `onChange` reports the field's key and its
 *  next value; the overlay owns normalization and persistence. */
export type PluginControlProps = {
  t: Translate;
  field: PluginConfigField;
  value: PluginConfigValue;
  disabled?: boolean;
  onChange: (key: string, value: PluginConfigValue) => void;
};

/** The label + help stack a control's row starts with. */
function ControlLabel({ t, field }: { t: Translate; field: PluginConfigField }) {
  return (
    <span className="plugin-config-field__main">
      <span className="plugin-config-field__label">{t(field.labelKey)}</span>
      {field.helpKey && (
        <span className="plugin-config-field__help">{t(field.helpKey)}</span>
      )}
    </span>
  );
}

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
      className="plugin-config-select"
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

function RadioControl({ t, field, value, disabled, onChange }: PluginControlProps) {
  if (field.type !== "radio") return null;
  const current = typeof value === "string" ? value : "";
  return (
    <span className="plugin-config-radio" role="radiogroup" aria-label={t(field.labelKey)}>
      {field.options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="radio"
          aria-checked={current === option.value}
          className={`plugin-config-radio__option${
            current === option.value ? " plugin-config-radio__option--on" : ""
          }`}
          disabled={disabled}
          onClick={() => onChange(field.key, option.value)}
        >
          {pluginOptionLabel(option, t)}
        </button>
      ))}
    </span>
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
  }
}

/** One field's whole row: its label/help and its control. */
export function PluginConfigRow(props: PluginControlProps) {
  return (
    <div className={`plugin-config-field plugin-config-field--${props.field.type}`}>
      <ControlLabel t={props.t} field={props.field} />
      <span className="plugin-config-field__control">
        <PluginConfigControl {...props} />
      </span>
    </div>
  );
}
