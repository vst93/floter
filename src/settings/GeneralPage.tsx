import type { AppSettings, CursorShape } from "../App";
import {
  GLASS_INTENSITIES,
  GLASS_INTENSITY,
  glassIntensityOf,
  type GlassIntensity,
} from "../glass-material";
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

/** The five glass-intensity stops (GLASS-UNIFY). The label names the material,
 *  not a number: Clear → Balanced → Strong → Deep → Solid. One control replaces
 *  R8's separate step picker and transparency sliders, because the two read as
 *  one perceived axis. Each stop's `(glass_step, tint)` pair lives in
 *  `glass-material.ts`; `settings.glassIntensity.*` carries the labels. */
const GLASS_INTENSITY_OPTIONS = GLASS_INTENSITIES.map((value) => ({
  value,
  labelKey: GLASS_INTENSITY[value].label as MessageKey,
}));

export const normalizeFontSize = (value: number): number =>
  Math.round(Math.min(MAX_FONT_SIZE, Math.max(MIN_FONT_SIZE, Number.isFinite(value) ? value : 14)));

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
  onChangeGlassIntensity: (level: GlassIntensity) => void;
};

/** The single glass-intensity control: five segments in the shared track, the
 *  same selection language the theme/cursor pickers use (accent tint, lit top
 *  rim, accent edge). The chosen stop is derived from the stored
 *  `(glass_step, main_opacity)` pair by `glassIntensityOf`, so an upgraded or
 *  hand-edited file displays on the stop it is closest to instead of on a
 *  default. */
function GlassIntensityControl({
  t,
  value,
  onChange,
}: {
  t: Translate;
  value: GlassIntensity;
  onChange: (level: GlassIntensity) => void;
}) {
  return (
    <div className="glass-intensity">
      {/* TODO(F9): roving tab stop is in place, but ArrowLeft/ArrowRight/
          Home/End are not handled. The other radiogroups in this file
          (theme, cursor, font family) share the gap, so it is a pre-existing
          pattern rather than a regression; fix them together in an a11y
          pass rather than one control at a time. */}
      <div
        className="settings-options settings-options--inline glass-intensity__segments"
        role="radiogroup"
        aria-label={t("settings.glassIntensity")}
      >
        {GLASS_INTENSITY_OPTIONS.map((option) => {
          const active = option.value === value;
          return (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={active}
              tabIndex={active ? 0 : -1}
              data-glass-intensity={option.value}
              className={`settings-option${active ? " settings-option--active" : ""}`}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => onChange(option.value)}
            >
              <span className="settings-option__main">
                <span className="settings-option__label">{t(option.labelKey)}</span>
              </span>
            </button>
          );
        })}
      </div>
      <p className="settings-section__hint">{t("settings.glassIntensityHint")}</p>
    </div>
  );
}

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
  onChangeGlassIntensity,
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
      <h2 className="settings-section__label">{t("settings.material")}</h2>
      <GlassIntensityControl
        t={t}
        value={glassIntensityOf(settings.glass_step, settings.main_opacity / 100)}
        onChange={onChangeGlassIntensity}
      />
    </section>
    </fieldset>
  );
}
