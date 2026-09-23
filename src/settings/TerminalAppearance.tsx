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

import type { CSSProperties, ReactNode } from "react";
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
  BOLD_MODE_OPTIONS,
  CURSOR_SHAPE_OPTIONS,
  LINE_HEIGHT_STEP,
  MAX_FONT_SIZE,
  MAX_LINE_HEIGHT,
  MAX_WHEEL_LINES,
  MIN_FONT_SIZE,
  MIN_LINE_HEIGHT,
  MIN_WHEEL_LINES,
  TERMINAL_PADDING_OPTIONS,
  TERMINAL_PALETTES,
  TERMINAL_THEME_OPTIONS,
  fontFamilyOptions,
  normalizeBoldMode,
  normalizeCursorBlink,
  normalizeCursorShape,
  normalizeFontSize,
  normalizeLineHeight,
  normalizePasteSafe,
  normalizeScrollbar,
  normalizeSelectCopy,
  normalizeTerminalPadding,
  normalizeTerminalTheme,
  normalizeWheelLines,
  packedHex,
  type TerminalTheme,
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

/** R43 · a miniature terminal, painted from the selected palette.
 *
 * The user asked for the palette to show what it does (「这个配色最好可以在下方
 * 或者边上有一个大概的预览」). This is a *preview*, not a second terminal: it is
 * a three-line block of static text styled with the palette's own colours, so it
 * costs no canvas, no renderer and no PTY. `inherit` has no fixed palette, so it
 * reads the document's `--terminal-*` tokens and follows the app's dark/light
 * theme automatically; every override paints its packed colours as hex.
 *
 * It is inline (a normal block in the settings row, never `position: absolute`),
 * so it cannot overlap the panel or the canvas. `aria-hidden`: the palette's own
 * label is the accessible name, and the fake command line is decoration. */
export function TerminalPalettePreview({ theme }: { theme: TerminalTheme }) {
  const palette = theme === "inherit" ? null : TERMINAL_PALETTES[theme];
  const style = (palette
    ? {
        background: packedHex(palette.bg),
        color: packedHex(palette.fg),
        "--preview-cursor": packedHex(palette.cursor),
        "--preview-selection": palette.selection,
      }
    : {
        background: "var(--terminal-bg)",
        color: "var(--terminal-fg)",
        "--preview-cursor": "var(--terminal-cursor)",
        "--preview-selection": "var(--terminal-selection)",
      }) as CSSProperties;
  return (
    <div className="terminal-palette-preview" style={style} aria-hidden="true">
      <span className="terminal-palette-preview__line">
        <span className="terminal-palette-preview__prompt">$</span> floter --version
      </span>
      <span className="terminal-palette-preview__line">
        <span className="terminal-palette-preview__selection">0.3.5-preview</span>
      </span>
      <span className="terminal-palette-preview__line">
        <span className="terminal-palette-preview__prompt">$</span>{" "}
        <span className="terminal-palette-preview__cursor" />
      </span>
    </div>
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
  /** R43 · force the full-width stacked layout without a slider readout (the
   *  palette row: a select plus the preview block under it). */
  stacked?: boolean;
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
  const wheelLines = normalizeWheelLines(settings.terminal_wheel_lines);

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
      // R43 · nine palettes do not fit a segmented track, so the control is the
      // settings select and the preview below it shows what the choice paints.
      stacked: true,
      control: (
        <div className="terminal-palette">
          <select
            className="settings-select"
            value={normalizeTerminalTheme(settings.terminal_theme)}
            aria-label={t("settings.terminalTheme")}
            onChange={(event) => onChange("terminal_theme", normalizeTerminalTheme(event.currentTarget.value))}
          >
            {TERMINAL_THEME_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{t(option.labelKey)}</option>
            ))}
          </select>
          <TerminalPalettePreview theme={normalizeTerminalTheme(settings.terminal_theme)} />
        </div>
      ),
    },
    {
      key: "terminal_bold",
      label: t("settings.terminalBold"),
      control: (
        <SegmentedChoice
          label={t("settings.terminalBold")}
          value={normalizeBoldMode(settings.terminal_bold)}
          onChange={(value) => onChange("terminal_bold", value)}
          options={BOLD_MODE_OPTIONS.map((option) => ({
            value: option.value,
            label: t(option.labelKey),
          }))}
        />
      ),
    },
    {
      key: "terminal_wheel_lines",
      label: t("settings.terminalWheelLines"),
      slider: true,
      value: t("settings.terminalWheelLinesValue", { lines: wheelLines }),
      control: (
        <input
          className="terminal-setting-range"
          type="range"
          min={MIN_WHEEL_LINES}
          max={MAX_WHEEL_LINES}
          step="1"
          value={wheelLines}
          aria-label={t("settings.terminalWheelLines")}
          onChange={(event) => onChange("terminal_wheel_lines", normalizeWheelLines(Number(event.currentTarget.value)))}
        />
      ),
    },
    {
      key: "terminal_select_copy",
      label: t("settings.terminalSelectCopy"),
      control: (
        <TerminalSwitch
          label={t("settings.terminalSelectCopy")}
          checked={normalizeSelectCopy(settings.terminal_select_copy)}
          onToggle={(next) => onChange("terminal_select_copy", next)}
        />
      ),
    },
    {
      key: "terminal_paste_safe",
      label: t("settings.terminalPasteSafe"),
      control: (
        <TerminalSwitch
          label={t("settings.terminalPasteSafe")}
          checked={normalizePasteSafe(settings.terminal_paste_safe)}
          onToggle={(next) => onChange("terminal_paste_safe", next)}
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
          stacked={row.slider === true || row.stacked === true}
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
