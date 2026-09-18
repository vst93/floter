import type { AppSettings, CursorShape } from "../App";
import {
  GLASS_INTENSITIES,
  GLASS_INTENSITY,
  clampWindowOpacity,
  glassIntensityOf,
  type GlassIntensity,
} from "../glass-material";
import {
  LANGUAGE_OPTIONS,
  type Language,
  type MessageKey,
  type Translate,
} from "../i18n";
import {
  SettingsCard,
  SettingsRow,
  SettingsScale,
} from "./SettingsRows";

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

/** The three glass-effect stops (GLASS-3STOP). The label names the *effect*,
 *  not a tint: Frosted → Liquid → Liquid Max. One control drives the
 *  liquid-glass effect (blur / saturation / control-lens quality); the two
 *  transparency sliders below are the app's own background opacity and stay
 *  independent of it. Each stop's `(blur, saturate, lens)` triple lives in
 *  `glass-material.ts`; `settings.glassIntensity.*` carries the labels. */
const GLASS_INTENSITY_OPTIONS = GLASS_INTENSITIES.map((value) => ({
  value,
  labelKey: GLASS_INTENSITY[value].label as MessageKey,
}));

export const normalizeFontSize = (value: number): number =>
  Math.round(Math.min(MAX_FONT_SIZE, Math.max(MIN_FONT_SIZE, Number.isFinite(value) ? value : 14)));

const MIN_OPACITY = 10;
const MAX_OPACITY = 100;

/** `--main-opacity` / `--terminal-opacity` are the **window transparency**
 *  controls: how solid each frame is. They are orthogonal to the glass effect
 *  (the stop control above), and each is configured on its own. */
export const normalizeOpacity = (value: number): number =>
  clampWindowOpacity(Number.isFinite(value) ? value : 47);

/** One segmented picker inside a row's control slot — the trailing
 *  three-way choice the reference pane uses for Appearance. The keyboard
 *  contract is the same radiogroup the page has always shipped: one tab stop
 *  (the chosen segment) and `role="radio"` on each segment. */
function SegmentedChoice<T extends string>({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: { value: T; label: string }[];
  value: T;
  onChange: (value: T) => void;
}) {
  return (
    <div
      className="settings-options settings-options--inline settings-options--trailing"
      role="radiogroup"
      aria-label={label}
    >
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={active}
            tabIndex={active ? 0 : -1}
            className={`settings-option${active ? " settings-option--active" : ""}`}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => onChange(option.value)}
          >
            <span className="settings-option__label">{option.label}</span>
          </button>
        );
      })}
    </div>
  );
}

type OpacityControlProps = {
  label: string;
  low: string;
  high: string;
  value: number;
  onChange: (value: number) => void;
};

/** One transparency slider — the macOS range row: label and live `%` readout
 *  on the title line, one word at each end of the track underneath. The
 *  readout is a plain number, not a primary action, so it spends no accent
 *  fill; the end words name the axis without a caption. */
function OpacityControl({ label, low, high, value, onChange }: OpacityControlProps) {
  return (
    <SettingsRow
      stacked
      label={
        <span className="settings-slider__head">
          <span>{label}</span>
          <output className="opacity-control__value">{value}%</output>
        </span>
      }
      control={
        <>
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
          <SettingsScale low={low} high={high} />
        </>
      }
    />
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
  onChangeOpacity: (target: "main" | "terminal", value: number) => void;
  onChangeGlassIntensity: (level: GlassIntensity) => void;
};

/** The single glass-effect control: three segments in the shared track, the
 *  same selection language the theme/cursor pickers use (accent tint, lit top
 *  rim, accent edge). The chosen stop is derived from the stored `glass_step`
 *  by `glassIntensityOf`, which ignores the transparency values — the two axes
 *  are independent, so nudging a slider never moves the highlighted stop. */
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
    </div>
  );
}

/** The general settings page: theme, language, window behaviour, terminal
 *  appearance and the material axes. All state lives in `App` and arrives
 *  through props. */
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
  onChangeGlassIntensity,
}: GeneralPageProps) {
  return (
    <fieldset className="settings-controls" disabled={busy || autostartUpdating} aria-busy={busy || autostartUpdating}>
    <div className="settings-page">
      <header className="settings-page__header">
        <h1 className="settings-page__title">{t("settings.menu.general")}</h1>
        <p className="settings-page__subtitle">{t("settings.page.general")}</p>
      </header>

      <section className="settings-section">
        <div className="settings-section__heading">
          <div className="settings-section__heading-main">
            <h2 className="settings-section__label">{t("settings.group.appearance")}</h2>
          </div>
        </div>
        <SettingsCard label={t("settings.group.appearance")}>
          <SettingsRow
            stacked
            label={t("settings.theme")}
            sublabel={t("settings.themeHint")}
            control={
              <SegmentedChoice
                label={t("settings.theme")}
                value={settings.theme}
                onChange={onChangeTheme}
                options={THEME_OPTIONS.map((option) => ({
                  value: option.value,
                  label: t(option.labelKey),
                }))}
              />
            }
          />
          <SettingsRow
            stacked
            label={t("settings.language")}
            sublabel={t("settings.languageHint")}
            control={
              <SegmentedChoice
                label={t("settings.language")}
                value={language}
                onChange={onChangeLanguage}
                options={LANGUAGE_OPTIONS.map((option) => ({
                  value: option.value,
                  label: option.label,
                }))}
              />
            }
          />
        </SettingsCard>
      </section>

      <section className="settings-section">
        <div className="settings-section__heading">
          <div className="settings-section__heading-main">
            <h2 className="settings-section__label">{t("settings.group.startup")}</h2>
          </div>
        </div>
        <SettingsCard label={t("settings.group.startup")}>
          <SettingsRow
            label={t("settings.launchAtStartup")}
            sublabel={t("settings.launchAtStartupHint")}
            control={
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
            }
          />
          <SettingsRow
            label={t("settings.hideOnBlur")}
            sublabel={t("settings.hideOnBlurHint")}
            control={
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
            }
          />
          <SettingsRow
            label={t("settings.showRecentInLauncher")}
            sublabel={t("settings.showRecentInLauncherHint")}
            control={
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
            }
          />
        </SettingsCard>
      </section>

      <section className="settings-section terminal-appearance-settings">
        <div className="settings-section__heading">
          <div className="settings-section__heading-main">
            <h2 className="settings-section__label">{t("settings.terminalAppearance")}</h2>
          </div>
        </div>
        <SettingsCard label={t("settings.terminalAppearance")}>
          <SettingsRow
            stacked
            label={
              <span className="settings-slider__head">
                <span>{t("settings.fontSize")}</span>
                <output className="terminal-setting-control__value">
                  {normalizeFontSize(settings.font_size)} px
                </output>
              </span>
            }
            sublabel={t("settings.terminalAppearanceHint")}
            control={
              <>
                <input
                  type="range"
                  min={MIN_FONT_SIZE}
                  max={MAX_FONT_SIZE}
                  step="1"
                  value={normalizeFontSize(settings.font_size)}
                  aria-label={t("settings.fontSize")}
                  onChange={(event) => onChangeFontSize(Number(event.currentTarget.value))}
                />
                <SettingsScale low={t("settings.scale.small")} high={t("settings.scale.large")} />
              </>
            }
          />
          <SettingsRow
            label={t("settings.fontFamily")}
            control={
              <select
                className="settings-select"
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
            }
          />
          <SettingsRow
            stacked
            label={t("settings.cursorShape")}
            control={
              <SegmentedChoice
                label={t("settings.cursorShape")}
                value={settings.cursor_shape}
                onChange={(value) => onChangeGeneralSetting("cursor_shape", value)}
                options={CURSOR_SHAPE_OPTIONS.map((option) => ({
                  value: option.value,
                  label: t(option.labelKey),
                }))}
              />
            }
          />
        </SettingsCard>
      </section>

      {/* The material group: the *effect* stop on one card and the two
          transparency sliders on a second. They answer different questions
          (how much liquid glass vs. how solid the frame is), so they are two
          cards under one title; the group's two explanations sit below the
          cards, where the reference puts a group's footnote. */}
      <section className="settings-section settings-section--material">
        <div className="settings-section__heading">
          <div className="settings-section__heading-main">
            <h2 className="settings-section__label">{t("settings.material")}</h2>
          </div>
        </div>
        <div className="settings-cards">
          <SettingsCard label={t("settings.glassIntensity")}>
            <SettingsRow
              stacked
              label={t("settings.glassIntensity")}
              control={
                <GlassIntensityControl
                  t={t}
                  value={glassIntensityOf(settings.glass_step, settings.main_opacity / 100)}
                  onChange={onChangeGlassIntensity}
                />
              }
            />
          </SettingsCard>
          <SettingsCard label={t("settings.group.transparency")} className="opacity-controls">
            <OpacityControl
              label={t("settings.transparency.main")}
              low={t("settings.transparency.scaleLow")}
              high={t("settings.transparency.scaleHigh")}
              value={normalizeOpacity(settings.main_opacity)}
              onChange={(value) => onChangeOpacity("main", value)}
            />
            <OpacityControl
              label={t("settings.transparency.terminal")}
              low={t("settings.transparency.scaleLow")}
              high={t("settings.transparency.scaleHigh")}
              value={normalizeOpacity(settings.terminal_opacity)}
              onChange={(value) => onChangeOpacity("terminal", value)}
            />
          </SettingsCard>
        </div>
        <p className="settings-section__hint">{t("settings.glassIntensityHint")}</p>
        <p className="settings-section__hint">{t("settings.transparencyHint")}</p>
      </section>
    </div>
    </fieldset>
  );
}
