import type { AppSettings } from "../App";
import { useEffect, useState } from "react";
import {
  UI_SCALE_STEPS,
  normalizeUiScale,
  type UiScale,
} from "../ui-scale";
import {
  GLASS_INTENSITIES,
  GLASS_INTENSITY,
  clampWindowOpacity,
  glassIntensityOf,
  type GlassIntensity,
} from "../glass-material";
import {
  RESIDENCY_CUSTOM_MAX_SECONDS,
  RESIDENCY_PRESET_SECONDS,
  normalizeResidencySeconds,
  residencyCustomSeedSeconds,
  residencyFromSelect,
  residencySelectValue,
} from "../surface-residency";
import {
  LANGUAGE_OPTIONS,
  type Language,
  type MessageKey,
  type Translate,
} from "../i18n";
import {
  SegmentedChoice,
  SettingsCard,
  SettingsRow,
  SettingsScale,
} from "./SettingsRows";
import { TerminalAppearanceSettings } from "./TerminalAppearance";
import { menubarIconSwitchState, toggleMenubarIcon } from "./menubar-icon";

// R42 · the terminal's font size — and the rest of its appearance — now live
// in `terminal/terminal-appearance.ts`, which both this page and the terminal
// page's own settings panel render from. Re-exported so every existing
// importer (App, useSettings, useTerminalView) keeps its import path.
export { normalizeFontSize } from "../terminal/terminal-appearance";

const THEME_OPTIONS: { value: string; labelKey: MessageKey }[] = [
  { value: "auto", labelKey: "settings.theme.auto" },
  { value: "dark", labelKey: "settings.theme.dark" },
  { value: "light", labelKey: "settings.theme.light" },
];

/** R7-13c · the three interface-size steps, in the order the picker paints them
 *  (smallest first). The label keys are the step names; the multiplier each one
 *  writes lives in `ui-scale.ts`, so this list never spells a number. */
const UI_SCALE_OPTIONS: { value: UiScale; labelKey: MessageKey }[] = UI_SCALE_STEPS.map(
  (value) => ({ value, labelKey: `settings.uiScale.${value}` as MessageKey }),
);

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

const MIN_OPACITY = 10;
const MAX_OPACITY = 100;

/** `--main-opacity` / `--terminal-opacity` are the **window transparency**
 *  controls: how solid each frame is. They are orthogonal to the glass effect
 *  (the stop control above), and each is configured on its own. */
export const normalizeOpacity = (value: number): number =>
  clampWindowOpacity(Number.isFinite(value) ? value : 47);

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
  /** R42 · the line-height slider is a per-tick control, so it rides the same
   *  debounce as the font-size slider rather than persisting on every pixel. */
  onChangeLineHeight: (value: number) => void;
  onChangeUiScale: (step: UiScale) => void;
  onChangeOpacity: (target: "main" | "terminal", value: number) => void;
  onChangeGlassIntensity: (level: GlassIntensity) => void;
};

/** R41 · the page-residency control. The R35 select offered six fixed steps;
 *  the user asked for a longer ladder and a free number (「下拉项太少了……再加一个
 *  自定义时间」). The control is the select plus an inline seconds field that
 *  appears only for the custom entry, so the row stays one line in every other
 *  state. The value the select *shows* is `residencySelectValue`'s pure
 *  mapping of the stored number, never a second copy of the state. */
function SurfaceResidencyControl({
  t,
  seconds,
  onChange,
}: {
  t: Translate;
  seconds: number;
  onChange: (seconds: number) => void;
}) {
  const value = normalizeResidencySeconds(seconds);
  const selectValue = residencySelectValue(value);
  // Open when the stored value already is a custom one, so reopening the page
  // on a custom duration shows the field it came from.
  const [customOpen, setCustomOpen] = useState(selectValue === "custom");
  const [draft, setDraft] = useState(() => String(residencyCustomSeedSeconds(value)));

  // A value written by anything other than this field (a hand-edited file, a
  // future page) re-seeds the draft while the field is closed; while it is
  // open the user's own typing is the draft.
  useEffect(() => {
    if (!customOpen) setDraft(String(residencyCustomSeedSeconds(value)));
  }, [value, customOpen]);

  const commitCustom = () => {
    const next = normalizeResidencySeconds(Number(draft));
    onChange(next);
    setDraft(String(next));
    // A number that lands on a preset (30, 120, …) stops being "custom" and
    // the field closes; anything else keeps the field open on the new value.
    setCustomOpen(residencySelectValue(next) === "custom");
  };

  return (
    <div className="residency-control">
      <select
        className="settings-select"
        value={selectValue}
        aria-label={t("settings.surfaceResidency")}
        onChange={(event) => {
          const choice = event.currentTarget.value;
          if (choice === "custom") {
            setDraft(String(residencyCustomSeedSeconds(value)));
            setCustomOpen(true);
            return;
          }
          const next = residencyFromSelect(choice);
          if (next === null) return;
          onChange(next);
          setCustomOpen(false);
        }}
      >
        <option value="off">{t("settings.surfaceResidencyOff")}</option>
        {RESIDENCY_PRESET_SECONDS.map((preset) => (
          <option key={preset} value={String(preset)}>
            {t("settings.surfaceResidencyValue", { seconds: preset })}
          </option>
        ))}
        <option value="never">{t("settings.surfaceResidencyNever")}</option>
        <option value="custom">
          {selectValue === "custom"
            ? t("settings.surfaceResidencyCustomValue", { seconds: value })
            : t("settings.surfaceResidencyCustom")}
        </option>
      </select>
      {customOpen && (
        <span className="residency-control__custom">
          <input
            type="number"
            className="residency-control__input"
            min={1}
            max={RESIDENCY_CUSTOM_MAX_SECONDS}
            step={1}
            value={draft}
            aria-label={t("settings.surfaceResidencyCustomLabel")}
            onChange={(event) => setDraft(event.currentTarget.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                commitCustom();
              }
            }}
          />
          <span className="residency-control__unit">
            {t("settings.surfaceResidencyCustomUnit")}
          </span>
          <button
            type="button"
            className="residency-control__apply"
            onMouseDown={(event) => event.preventDefault()}
            onClick={commitCustom}
          >
            {t("settings.surfaceResidencyCustomApply")}
          </button>
        </span>
      )}
    </div>
  );
}

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
  onChangeLineHeight,
  onChangeUiScale,
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
          {/* R7-13c · Interface size. Raycast parity: three steps that scale the
              whole UI. It sits in Appearance next to theme/language because it
              is the same kind of choice — how the interface looks — and it uses
              the identical three-stop segmented language the glass and cursor
              pickers already speak. The stored value is normalized on read, so
              a settings file that predates the round (no key) or names a step
              that no longer ships shows `default`, never an unhighlighted
              track. */}
          <SettingsRow
            stacked
            label={t("settings.uiScale")}
            sublabel={t("settings.uiScaleHint")}
            control={
              <SegmentedChoice
                label={t("settings.uiScale")}
                value={normalizeUiScale(settings.ui_scale)}
                onChange={onChangeUiScale}
                options={UI_SCALE_OPTIONS.map((option) => ({
                  value: option.value,
                  label: t(option.labelKey),
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
          {/* R35 · the page-residency window. It sits beside `hide_on_blur`
              because the two are read together but answer different questions:
              `hide_on_blur` decides whether the *window* disappears, this one
              decides whether the *surface* survives it. R41 · the control is
              the preset select plus an inline custom seconds field (see
              `SurfaceResidencyControl`). */}
          <SettingsRow
            label={t("settings.surfaceResidency")}
            sublabel={t("settings.surfaceResidencyHint")}
            control={
              <SurfaceResidencyControl
                t={t}
                seconds={settings.surface_residency_seconds}
                onChange={(next) =>
                  onChangeGeneralSetting("surface_residency_seconds", next)
                }
              />
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

      {/* R7-10c: the menu bar / tray icon switch. It lives on its own card
          because it is a *residency* choice, not a window-behaviour one: the
          app keeps running and stays summonable either way. Hiding the icon
          removes one entry point and nothing else — the global shortcut, the
          deep link and the settings page all stay. */}
      <section className="settings-section">
        <div className="settings-section__heading">
          <div className="settings-section__heading-main">
            <h2 className="settings-section__label">{t("settings.group.menuBar")}</h2>
          </div>
        </div>
        <SettingsCard label={t("settings.group.menuBar")}>
          <SettingsRow
            label={t("settings.showMenubarIcon")}
            sublabel={t("settings.showMenubarIconHint")}
            control={
              <button
                type="button"
                className={`settings-switch${menubarIconSwitchState(settings.show_menubar_icon).active ? " settings-switch--active" : ""}`}
                role="switch"
                aria-checked={menubarIconSwitchState(settings.show_menubar_icon).ariaChecked}
                aria-label={t("settings.showMenubarIcon")}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => onChangeGeneralSetting("show_menubar_icon", toggleMenubarIcon(settings.show_menubar_icon))}
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
        {/* R42 · the card body is the shared terminal-appearance component,
            the same one the terminal page's settings panel renders. One
            schema, one set of normalizers, one AppSettings — so the two
            surfaces cannot disagree. */}
        <TerminalAppearanceSettings
          variant="card"
          settings={settings}
          t={t}
          onChangeFontSize={onChangeFontSize}
          onChangeLineHeight={onChangeLineHeight}
          onChange={onChangeGeneralSetting}
        />
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
