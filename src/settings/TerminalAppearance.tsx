// R42 · the terminal's appearance settings, rendered from one schema.
//
// Two surfaces show these controls — the settings page's "Terminal appearance"
// card and the in-terminal settings panel — and they render this one
// component. That is what keeps them from drifting into two truths: the fields,
// the normalizers and the option lists all come from
// `terminal/terminal-appearance.ts`, and both surfaces write the same
// `AppSettings` through the same handlers.
//
// `variant` is presentation only. `"card"` is the macOS System Settings card
// the settings page already uses (`SettingsCard` / `SettingsRow`); `"strip"`
// is the compact, one-screen grid the terminal panel needs, because the user
// asked for a panel that is 「简洁、不繁琐」 — no tabs, no sub-headings, just
// rows.

import type { ReactNode } from "react";
import type { AppSettings } from "../App";
import type { Translate } from "../i18n";
import {
  SegmentedChoice,
  SettingsCard,
  SettingsRow,
  SettingsScale,
} from "./SettingsRows";
import { detectMonospaceFonts } from "../terminal/font-detect";
import {
  CURSOR_SHAPE_OPTIONS,
  LINE_HEIGHT_STEP,
  MAX_FONT_SIZE,
  MAX_LINE_HEIGHT,
  MIN_FONT_SIZE,
  MIN_LINE_HEIGHT,
  TERMINAL_PADDING_OPTIONS,
  TERMINAL_THEME_OPTIONS,
  fontFamilyOptions,
  normalizeCursorBlink,
  normalizeCursorShape,
  normalizeFontSize,
  normalizeLineHeight,
  normalizeScrollbar,
  normalizeTerminalPadding,
  normalizeTerminalTheme,
} from "../terminal/terminal-appearance";

export interface TerminalAppearanceSettingsProps {
  settings: AppSettings;
  t: Translate;
  variant: "card" | "strip";
  onChangeFontSize: (size: number) => void;
  onChangeLineHeight: (value: number) => void;
  onChange: <K extends keyof AppSettings>(field: K, value: AppSettings[K]) => void;
}

/** The settings switch, in the shape the menubar row already uses. */
function TerminalSwitch({
  label,
  checked,
  onToggle,
}: {
  label: string;
  checked: boolean;
  onToggle: (next: boolean) => void;
}) {
  return (
    <button
      type="button"
      className={`settings-switch${checked ? " settings-switch--active" : ""}`}
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onMouseDown={(event) => event.preventDefault()}
      onClick={() => onToggle(!checked)}
    >
      <span className="settings-switch__thumb" />
    </button>
  );
}

/** One row of the schema. `value` is the slider readout, which the two
 *  variants place differently — the card puts it at the far end of the title
 *  line (the settings page's idiom), the strip keeps it beside the track. */
type Row = {
  key: string;
  label: string;
  control: ReactNode;
  value?: string;
  slider?: boolean;
  sublabel?: ReactNode;
};

export function TerminalAppearanceSettings({
  settings,
  t,
  variant,
  onChangeFontSize,
  onChangeLineHeight,
  onChange,
}: TerminalAppearanceSettingsProps) {
  const fontFamilies = fontFamilyOptions(detectMonospaceFonts(), settings.font_family);
  const lineHeight = normalizeLineHeight(settings.terminal_line_height);
  const fontSize = normalizeFontSize(settings.font_size);

  const rows: Row[] = [
    {
      key: "font_family",
      label: t("settings.fontFamily"),
      control: (
        <select
          className="settings-select"
          value={settings.font_family}
          aria-label={t("settings.fontFamily")}
          onChange={(event) => onChange("font_family", event.currentTarget.value)}
        >
          {fontFamilies.map((option) => (
            <option key={option.value} value={option.value}>{option.label}</option>
          ))}
        </select>
      ),
    },
    {
      key: "font_size",
      label: t("settings.fontSize"),
      slider: true,
      value: `${fontSize} px`,
      sublabel: variant === "card" ? t("settings.terminalAppearanceHint") : undefined,
      control: (
        <input
          className="terminal-setting-range"
          type="range"
          min={MIN_FONT_SIZE}
          max={MAX_FONT_SIZE}
          step="1"
          value={fontSize}
          aria-label={t("settings.fontSize")}
          onChange={(event) => onChangeFontSize(Number(event.currentTarget.value))}
        />
      ),
    },
    {
      key: "terminal_line_height",
      label: t("settings.terminalLineHeight"),
      slider: true,
      value: `${lineHeight.toFixed(2)}×`,
      control: (
        <input
          className="terminal-setting-range"
          type="range"
          min={MIN_LINE_HEIGHT}
          max={MAX_LINE_HEIGHT}
          step={LINE_HEIGHT_STEP}
          value={lineHeight}
          aria-label={t("settings.terminalLineHeight")}
          onChange={(event) => onChangeLineHeight(Number(event.currentTarget.value))}
        />
      ),
    },
    {
      key: "terminal_padding",
      label: t("settings.terminalPadding"),
      control: (
        <SegmentedChoice
          label={t("settings.terminalPadding")}
          value={normalizeTerminalPadding(settings.terminal_padding)}
          onChange={(value) => onChange("terminal_padding", value)}
          options={TERMINAL_PADDING_OPTIONS.map((option) => ({
            value: option.value,
            label: t(option.labelKey),
          }))}
        />
      ),
    },
    {
      key: "cursor_shape",
      label: t("settings.cursorShape"),
      control: (
        <SegmentedChoice
          label={t("settings.cursorShape")}
          value={normalizeCursorShape(settings.cursor_shape)}
          onChange={(value) => onChange("cursor_shape", value)}
          options={CURSOR_SHAPE_OPTIONS.map((option) => ({
            value: option.value,
            label: t(option.labelKey),
          }))}
        />
      ),
    },
    {
      key: "terminal_cursor_blink",
      label: t("settings.terminalCursorBlink"),
      control: (
        <TerminalSwitch
          label={t("settings.terminalCursorBlink")}
          checked={normalizeCursorBlink(settings.terminal_cursor_blink)}
          onToggle={(next) => onChange("terminal_cursor_blink", next)}
        />
      ),
    },
    {
      key: "terminal_theme",
      label: t("settings.terminalTheme"),
      control: (
        <SegmentedChoice
          label={t("settings.terminalTheme")}
          value={normalizeTerminalTheme(settings.terminal_theme)}
          onChange={(value) => onChange("terminal_theme", value)}
          options={TERMINAL_THEME_OPTIONS.map((option) => ({
            value: option.value,
            label: t(option.labelKey),
          }))}
        />
      ),
    },
    {
      key: "terminal_scrollbar",
      label: t("settings.terminalScrollbar"),
      control: (
        <TerminalSwitch
          label={t("settings.terminalScrollbar")}
          checked={normalizeScrollbar(settings.terminal_scrollbar)}
          onToggle={(next) => onChange("terminal_scrollbar", next)}
        />
      ),
    },
  ];

  if (variant === "strip") {
    return (
      <div className="terminal-settings" role="group" aria-label={t("settings.terminalAppearance")}>
        {rows.map((row) => (
          <div className="terminal-settings__row" key={row.key}>
            <span className="terminal-settings__label">{row.label}</span>
            <span className="terminal-settings__control">
              {row.control}
              {row.value !== undefined && (
                <output className="terminal-setting-control__value">{row.value}</output>
              )}
            </span>
          </div>
        ))}
      </div>
    );
  }

  return (
    <SettingsCard label={t("settings.terminalAppearance")}>
      {rows.map((row) => (
        <SettingsRow
          key={row.key}
          stacked={row.slider === true}
          label={
            row.slider ? (
              <span className="settings-slider__head">
                <span>{row.label}</span>
                {row.value !== undefined && (
                  <output className="terminal-setting-control__value">{row.value}</output>
                )}
              </span>
            ) : (
              row.label
            )
          }
          sublabel={row.sublabel}
          control={
            row.slider ? (
              <>
                {row.control}
                <SettingsScale low={t("settings.scale.small")} high={t("settings.scale.large")} />
              </>
            ) : (
              row.control
            )
          }
        />
      ))}
    </SettingsCard>
  );
}
