import type { AppSettings, CursorShape } from "../App";
import {
  LANGUAGE_OPTIONS,
  type Language,
  type MessageKey,
  type Translate,
} from "../i18n";

const THEME_OPTIONS: { value: string; labelKey: MessageKey }[] = [
  { value: "auto", labelKey: "settings.theme.auto" },
  { value: "dark", labelKey: "settings.theme.dark" },
  { value: "light", labelKey: "settings.theme.light" },
];

const MIN_FONT_SIZE = 8;
const MAX_FONT_SIZE = 48;
const FONT_FAMILY_OPTIONS = [
  { value: "monospace", label: "System Mono" },
  { value: "JetBrains Mono", label: "JetBrains Mono" },
  { value: "SF Mono", label: "SF Mono" },
  { value: "Cascadia Mono", label: "Cascadia Mono" },
  { value: "Menlo", label: "Menlo" },
  { value: "Consolas", label: "Consolas" },
  { value: "DejaVu Sans Mono", label: "DejaVu Sans Mono" },
  { value: "Liberation Mono", label: "Liberation Mono" },
] as const;
const CURSOR_SHAPE_OPTIONS: { value: CursorShape; labelKey: MessageKey }[] = [
  { value: "beam", labelKey: "settings.cursor.beam" },
  { value: "block", labelKey: "settings.cursor.block" },
  { value: "underline", labelKey: "settings.cursor.underline" },
];

export const normalizeFontSize = (value: number): number =>
  Math.round(Math.min(MAX_FONT_SIZE, Math.max(MIN_FONT_SIZE, Number.isFinite(value) ? value : 14)));

const MIN_OPACITY = 10;
const MAX_OPACITY = 100;
const OPACITY_PRESETS = [25, 50, 75, 100];
const OPACITY_SNAP_DISTANCE = 2;

export const normalizeOpacity = (value: number): number => {
  const safeValue = Number.isFinite(value) ? value : MAX_OPACITY;
  const clamped = Math.round(Math.min(MAX_OPACITY, Math.max(MIN_OPACITY, safeValue)));
  return OPACITY_PRESETS.find((preset) => Math.abs(preset - clamped) <= OPACITY_SNAP_DISTANCE)
    ?? clamped;
};

type OpacityControlProps = {
  label: string;
  value: number;
  onChange: (value: number) => void;
};

function OpacityControl({ label, value, onChange }: OpacityControlProps) {
  return (
    <div className="opacity-control">
      <div className="opacity-control__header">
        <label className="opacity-control__label">{label}</label>
        <output className="opacity-control__value">{value}%</output>
      </div>
      <input
        className="opacity-control__range"
        type="range"
        min={MIN_OPACITY}
        max={MAX_OPACITY}
        step="1"
        value={value}
        aria-label={label}
        onChange={(event) => onChange(normalizeOpacity(Number(event.currentTarget.value)))}
      />
      <div className="opacity-control__presets" aria-label={label}>
        {[25, 50, 75, 100].map((preset) => (
          <button
            key={preset}
            type="button"
            className={`opacity-control__preset${value === preset ? " opacity-control__preset--active" : ""}`}
            aria-pressed={value === preset}
            onClick={() => onChange(preset)}
          >
            {preset}%
          </button>
        ))}
      </div>
    </div>
  );
}

type GeneralPageProps = {
  busy: boolean;
  t: Translate;
  settings: AppSettings;
  language: Language;
  autostartUpdating: boolean;
  onChangeTheme: (theme: string) => void;
  onChangeLanguage: (language: Language) => void;
  onChangeGeneralSetting: <K extends keyof AppSettings>(field: K, value: AppSettings[K]) => void;
  onChangeLaunchAtStartup: (enabled: boolean) => void;
  onChangeFontSize: (size: number) => void;
  onChangeOpacity: (field: "main_opacity" | "terminal_opacity", value: number) => void;
};

/** The general settings page: theme, language, window behaviour and terminal
 * appearance. All state lives in `App` and arrives through props. */
export function GeneralPage({
  busy,
  t,
  settings,
  language,
  autostartUpdating,
  onChangeTheme,
  onChangeLanguage,
  onChangeGeneralSetting,
  onChangeLaunchAtStartup,
  onChangeFontSize,
  onChangeOpacity,
}: GeneralPageProps) {
  return (
    <fieldset className="settings-controls" disabled={busy || autostartUpdating} aria-busy={busy || autostartUpdating}>
    <div className="settings-preferences">
      <section className="settings-section">
        <h2 className="settings-section__label">{t("settings.theme")}</h2>
        <div
          className="settings-options settings-options--inline"
          role="radiogroup"
          aria-label={t("settings.theme")}
        >
          {THEME_OPTIONS.map((option) => {
            const active = option.value === settings.theme;
            return (
              <button
                key={option.value}
                type="button"
                role="radio"
                aria-checked={active}
                tabIndex={active ? 0 : -1}
                className={`settings-option${active ? " settings-option--active" : ""}`}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => onChangeTheme(option.value)}
              >
                <span className="settings-option__label">{t(option.labelKey)}</span>
              </button>
            );
          })}
        </div>
        <p className="settings-section__hint">{t("settings.themeHint")}</p>
      </section>

      <section className="settings-section">
        <h2 className="settings-section__label">{t("settings.language")}</h2>
        <div
          className="settings-options settings-options--inline"
          role="radiogroup"
          aria-label={t("settings.language")}
        >
          {LANGUAGE_OPTIONS.map((option) => {
            const active = option.value === language;
            return (
              <button
                key={option.value}
                type="button"
                role="radio"
                aria-checked={active}
                tabIndex={active ? 0 : -1}
                className={`settings-option${active ? " settings-option--active" : ""}`}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => onChangeLanguage(option.value)}
              >
                <span className="settings-option__label">{option.label}</span>
              </button>
            );
          })}
        </div>
        <p className="settings-section__hint">{t("settings.languageHint")}</p>
      </section>
    </div>

    <section className="settings-section">
      <div className="settings-option settings-option--static">
        <span className="settings-option__main">
          <span className="settings-option__label">
            {t("settings.launchAtStartup")}
          </span>
          <span className="settings-option__description">
            {t("settings.launchAtStartupHint")}
          </span>
        </span>
        <button
          type="button"
          className={`settings-switch${settings.launch_at_startup ? " settings-switch--active" : ""}`}
          role="switch"
          aria-checked={settings.launch_at_startup}
          aria-label={t("settings.launchAtStartup")}
          disabled={autostartUpdating}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => void onChangeLaunchAtStartup(!settings.launch_at_startup)}
        >
          <span className="settings-switch__thumb" />
        </button>
      </div>
      <div className="settings-option settings-option--static">
        <span className="settings-option__main">
          <span className="settings-option__label">
            {t("settings.hideOnBlur")}
          </span>
          <span className="settings-option__description">
            {t("settings.hideOnBlurHint")}
          </span>
        </span>
        <button
          type="button"
          className={`settings-switch${settings.hide_on_blur ? " settings-switch--active" : ""}`}
          role="switch"
          aria-checked={settings.hide_on_blur}
          aria-label={t("settings.hideOnBlur")}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => onChangeGeneralSetting("hide_on_blur", !settings.hide_on_blur)}
        >
          <span className="settings-switch__thumb" />
        </button>
      </div>
      <div className="settings-option settings-option--static">
        <span className="settings-option__main">
          <span className="settings-option__label">
            {t("settings.showRecentInLauncher")}
          </span>
          <span className="settings-option__description">
            {t("settings.showRecentInLauncherHint")}
          </span>
        </span>
        <button
          type="button"
          className={`settings-switch${settings.show_recent_in_launcher ? " settings-switch--active" : ""}`}
          role="switch"
          aria-checked={settings.show_recent_in_launcher}
          aria-label={t("settings.showRecentInLauncher")}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => onChangeGeneralSetting("show_recent_in_launcher", !settings.show_recent_in_launcher)}
        >
          <span className="settings-switch__thumb" />
        </button>
      </div>
    </section>

    <section className="settings-section terminal-appearance-settings">
      <h2 className="settings-section__label">{t("settings.terminalAppearance")}</h2>
      <div className="terminal-appearance-settings__grid">
        <label className="terminal-setting-control">
          <span className="terminal-setting-control__header">
            <span>{t("settings.fontSize")}</span>
            <output>{normalizeFontSize(settings.font_size)} px</output>
          </span>
          <input
            type="range"
            min={MIN_FONT_SIZE}
            max={MAX_FONT_SIZE}
            step="1"
            value={normalizeFontSize(settings.font_size)}
            aria-label={t("settings.fontSize")}
            onChange={(event) => onChangeFontSize(Number(event.currentTarget.value))}
          />
        </label>
        <label className="terminal-setting-control">
          <span className="terminal-setting-control__header">
            <span>{t("settings.fontFamily")}</span>
          </span>
          <select
            value={settings.font_family}
            aria-label={t("settings.fontFamily")}
            onChange={(event) => onChangeGeneralSetting("font_family", event.currentTarget.value)}
          >
            {!FONT_FAMILY_OPTIONS.some((option) => option.value === settings.font_family) && (
              <option value={settings.font_family}>{settings.font_family}</option>
            )}
            {FONT_FAMILY_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </label>
      </div>
      <div className="terminal-setting-control terminal-setting-control--cursor">
        <span className="terminal-setting-control__header">
          <span>{t("settings.cursorShape")}</span>
        </span>
        <div
          className="settings-options settings-options--inline"
          role="radiogroup"
          aria-label={t("settings.cursorShape")}
        >
          {CURSOR_SHAPE_OPTIONS.map((option) => {
            const active = option.value === settings.cursor_shape;
            return (
              <button
                key={option.value}
                type="button"
                role="radio"
                aria-checked={active}
                tabIndex={active ? 0 : -1}
                className={`settings-option${active ? " settings-option--active" : ""}`}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => onChangeGeneralSetting("cursor_shape", option.value)}
              >
                <span className="settings-option__label">{t(option.labelKey)}</span>
              </button>
            );
          })}
        </div>
      </div>
      <p className="settings-section__hint">{t("settings.terminalAppearanceHint")}</p>
    </section>

    <section className="settings-section settings-section--material">
      <h2 className="settings-section__label">{t("settings.opacity")}</h2>
      <div className="opacity-controls">
        <OpacityControl
          label={t("settings.opacity.main")}
          value={normalizeOpacity(settings.main_opacity)}
          onChange={(value) => onChangeOpacity("main_opacity", value)}
        />
        <OpacityControl
          label={t("settings.opacity.terminal")}
          value={normalizeOpacity(settings.terminal_opacity)}
          onChange={(value) => onChangeOpacity("terminal_opacity", value)}
        />
      </div>
      <p className="settings-section__hint">{t("settings.opacityHint")}</p>
    </section>
    </fieldset>
  );
}
