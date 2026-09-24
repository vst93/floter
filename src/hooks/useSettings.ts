// Settings state, hydration, persistence, and the change* mutators.
//
// Extracted verbatim from `App.tsx`; the hook receives the App-owned refs
// (blur-suppression, autostart busy flag) the change* mutators flip while
// editing, and returns every value downstream code needs.

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";
import { invoke } from "@tauri-apps/api/core";
import { normalizeFontSize } from "../settings/GeneralPage";
import {
  DEFAULT_BOLD_MODE,
  DEFAULT_CURSOR_BLINK,
  DEFAULT_LINE_HEIGHT,
  DEFAULT_PASTE_SAFE,
  DEFAULT_SCROLLBAR,
  DEFAULT_SELECT_COPY,
  DEFAULT_TERMINAL_PADDING,
  DEFAULT_TERMINAL_THEME,
  DEFAULT_WHEEL_LINES,
  normalizeBoldMode,
  normalizeLineHeight,
  normalizePasteSafe,
  normalizeSelectCopy,
  normalizeTerminalPadding,
  normalizeTerminalTheme,
  normalizeWheelLines,
} from "../terminal/terminal-appearance";
import { normalizeUiScale, type UiScale } from "../ui-scale";
import {
  createSerialSettingsWriter,
  createSettingsHydration,
  normalizeSettingsPage,
  rollbackRejectedSettings,
  type SettingsPage,
} from "../settings-persistence";
import { DEFAULT_SHORTCUTS, withShortcutDefaults } from "../shortcuts";
import { normalizeLanguage, type Language } from "../i18n";
import type { AppSettings } from "../App";
import { normalizeGlassStep, clampWindowOpacity, glassIntensitySettings, type GlassIntensity } from "../glass-material";
import { DEFAULT_RESIDENCY_SECONDS, normalizeResidencySeconds } from "../surface-residency";
import { withCommandAlias } from "../command-aliases";
import { normalizeCalculatorSettings } from "../calculator";
/** Defaults applied before the first disk read returns. */
const SETTINGS_DEFAULTS: AppSettings = {
  hotkey: "Ctrl+Space",
  hide_on_blur: true,
  // R35 · the shipped residency window, matching `DEFAULT_SURFACE_RESIDENCY_SECONDS`
  // in Rust. Non-zero: the round asked for the behaviour, not for a switch that
  // ships off.
  surface_residency_seconds: DEFAULT_RESIDENCY_SECONDS,
  launch_at_startup: false,
  theme: "dark",
  font_size: 14,
  font_family: "monospace",
  cursor_shape: "beam",
  // R42 · the terminal's appearance. The defaults are the exact behaviour
  // every build before this round shipped — 1.4 line height, 3px padding
  // (`regular`), a blinking cursor, the inherited palette and a visible
  // scrollbar — so a pre-round settings file renders pixel-identically.
  terminal_line_height: DEFAULT_LINE_HEIGHT,
  terminal_padding: DEFAULT_TERMINAL_PADDING,
  terminal_cursor_blink: DEFAULT_CURSOR_BLINK,
  terminal_theme: DEFAULT_TERMINAL_THEME,
  terminal_scrollbar: DEFAULT_SCROLLBAR,
  // R43 · the interaction axes. Every default is the behaviour the build before
  // this round shipped: a three-line wheel notch, a bold face for bold cells, no
  // copy-on-select and a verbatim paste.
  // R44 · copy-on-select is the exception now: its default flipped to on (the
  // user asked for the selection to be copied without a second gesture), so the
  // constant below — not this comment — is the single truth. An explicit `false`
  // persisted by a user who turned it off still wins over it.
  terminal_wheel_lines: DEFAULT_WHEEL_LINES,
  terminal_bold: DEFAULT_BOLD_MODE,
  terminal_select_copy: DEFAULT_SELECT_COPY,
  terminal_paste_safe: DEFAULT_PASTE_SAFE,
  language: "en",
  main_opacity: 47,
  terminal_opacity: 46,
  glass_step: "regular",
  shortcuts: DEFAULT_SHORTCUTS,
  show_commands_in_search: false,
  show_recent_in_launcher: true,
  clipboard_history_enabled: true,
  // The clipboard panel ships with NO global hotkey; users may bind one on
  // the shortcuts settings page.
  clipboard_history_hotkey: "",
  // R27 · the shipped capacity, matching `DEFAULT_CLIPBOARD_MAX_ITEMS` in Rust.
  clipboard_history_max_items: 300,
  launch_counts: {},
  last_settings_page: "general",
  seen_tip: false,
  // R7-10c: the menu bar / tray icon ships visible, which is the behaviour
  // every earlier build had. The frontend default must match the Rust
  // `default_true` so a pre-hydration frame does not hide the icon.
  show_menubar_icon: true,
  // R7-11: no aliases until the user adds one.
  command_aliases: {},
  // R7-13c: the interface-size step. R47 moved the shipped default from
  // `default` (1) to `small` (0.9) at the user's request, so a pre-hydration
  // frame is the small step rather than the scale every build through R46
  // painted. Must equal Rust's `DEFAULT_UI_SCALE`.
  ui_scale: "small",
  // R26-A: the browser plugin ships working out of the box — auto-detect the
  // browser, no custom directory, a 30-day history window. R26-B adds the
  // DevTools debug-port pair, off and on the browser's own port by default.
  browser_plugin: {
    enabled: true,
    target: "auto",
    custom_base_dir: null,
    history_days: 30,
    cdp_enabled: false,
    cdp_port: 9222,
    sort_order: "relevance",
    search_fields: "all",
  },
  // R50 · the calculator plugin ships with a 100-entry history, a 30-day
  // window and "expression = result" copying.
  calculator_plugin: {
    max_items: 100,
    retention_days: 30,
    copy_mode: "full",
  },
  // R39 · no external plugin command is enabled until the user turns one on
  // ("absence means off"); an empty map is that state.
  plugin_command_switches: {},
};

/** Debounce window for the font-size and transparency sliders' writes. The
 *  glass effect stop is a discrete choice and persists immediately, so it no
 *  longer rides this. */
const SETTINGS_DEBOUNCE_MS = 180;
/** Blur suppress window after a non-slider settings edit. */
const SETTINGS_BLUR_SUPPRESS_MS = 400;

export function useSettings(options: {
  /** Ref the change* mutators extend after an edit so the panel does not
   *  immediately close on the next focus loss. */
  suppressBlurUntil: RefObject<number>;
  /** Busy flag the launch-at-startup toggle flips while the backend confirms
   *  the OS-level change. Also gates re-entry into `changeLaunchAtStartup`. */
  autostartUpdating: boolean;
  /** Setter for the `autostartUpdating` busy flag. */
  setAutostartUpdating: Dispatch<SetStateAction<boolean>>;
}) {
  const { suppressBlurUntil, setAutostartUpdating } = options;

  // ---- State ---------------------------------------------------------------
  const [settings, setSettings] = useState<AppSettings>(SETTINGS_DEFAULTS);
  const [settingsPage, setSettingsPage] = useState<SettingsPage>("general");
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [settingsSaveFailed, setSettingsSaveFailed] = useState(false);
  const [settingsLoading, setSettingsLoading] = useState(true);
  const [settingsLoadFailed, setSettingsLoadFailed] = useState(false);

  // ---- Refs ----------------------------------------------------------------
  const settingsSaveTimer = useRef<number | null>(null);
  const settingsSaveGeneration = useRef(0);
  const settingsHydration = useMemo(
    () => createSettingsHydration<AppSettings>(),
    [],
  );
  const settingsLoadPromise = useRef<Promise<void> | null>(null);
  const hydrationSavePromise = useRef<Promise<void> | null>(null);
  const settingsRef = useRef(settings);
  const confirmedSettings = useRef(settings);
  const pendingFields = useRef(new Set<keyof AppSettings>());
  const autostartBusy = useRef(false);
  useEffect(() => {
    settingsRef.current = settings;
  }, [settings]);

  const saveSettings = useMemo(
    () =>
      createSerialSettingsWriter<AppSettings>((next) =>
        invoke("save_settings", { settings: next }),
      ),
    [],
  );

  // ---- Cleanup -------------------------------------------------------------
  // The launcher/terminal feedback timers are owned (and cleaned up) by
  // `useTimedFeedback`. The settings debounce timer is ours.
  useEffect(
    () => () => {
      if (settingsSaveTimer.current !== null) {
        window.clearTimeout(settingsSaveTimer.current);
      }
    },
    [],
  );

  // ---- Persistence helpers -------------------------------------------------
  const commitSettings = useCallback(
    (next: AppSettings): Promise<void> => {
      const generation = ++settingsSaveGeneration.current;
      setSettingsSaving(true);
      return saveSettings(next).then(
        () => {
          confirmedSettings.current = next;
          if (settingsSaveGeneration.current !== generation) return;
          setSettingsSaving(false);
          setSettingsSaveFailed(false);
        },
        async (error) => {
          // Shortcut commands persist independently of this writer. Re-read
          // their authoritative values before reverting a failed snapshot.
          let confirmed = confirmedSettings.current;
          try {
            const loaded = await invoke<AppSettings>("get_settings");
            confirmed = { ...confirmed, ...loaded, shortcuts: withShortcutDefaults(loaded.shortcuts) };
          } catch {
            // Keep the last acknowledged snapshot if the optional read fails.
          }
          if (settingsSaveGeneration.current === generation) {
            confirmedSettings.current = confirmed;
            const reverted = rollbackRejectedSettings(settingsRef.current, next, confirmed);
            settingsRef.current = reverted;
            setSettings(reverted);
            setSettingsSaving(false);
            setSettingsSaveFailed(true);
          }
          throw error;
        },
      );
    },
    [saveSettings],
  );

  // Startup remains interactive while settings load. Delay and coalesce
  // writes until hydration finishes so a default frontend snapshot cannot
  // overwrite fields that have not arrived from disk yet.
  const persistSettings = useCallback((): Promise<void> => {
    if (settingsHydration.isReady()) {
      return commitSettings(settingsRef.current);
    }
    if (!hydrationSavePromise.current) {
      const pending = settingsHydration
        .waitUntilReady()
        .then(() => commitSettings(settingsRef.current));
      hydrationSavePromise.current = pending;
      const clearPending = () => {
        if (hydrationSavePromise.current === pending) {
          hydrationSavePromise.current = null;
        }
      };
      void pending.then(clearPending, clearPending);
    }
    return hydrationSavePromise.current;
  }, [settingsHydration, commitSettings]);

  // ---- Load ----------------------------------------------------------------
  const loadSettings = useCallback((): Promise<void> => {
    if (settingsLoadPromise.current) return settingsLoadPromise.current;
    setSettingsLoading(true);
    const request = invoke<AppSettings>("get_settings")
      .then((loaded) => {
        const normalized = {
          ...loaded,
          language: normalizeLanguage(loaded.language),
          launch_at_startup: loaded.launch_at_startup ?? false,
          main_opacity: clampWindowOpacity(loaded.main_opacity ?? 47),
          terminal_opacity: clampWindowOpacity(loaded.terminal_opacity ?? 46),
          glass_step: normalizeGlassStep(loaded.glass_step),
          // R35 · a pre-round settings file has no residency key; a hand-edited
          // one may carry an out-of-range number. Both land on the `[0, 30]`
          // integer domain, with a missing value at the shipped default.
          surface_residency_seconds: normalizeResidencySeconds(
            loaded.surface_residency_seconds ?? DEFAULT_RESIDENCY_SECONDS,
          ),
          shortcuts: withShortcutDefaults(loaded.shortcuts),
          clipboard_history_enabled: loaded.clipboard_history_enabled ?? true,
          clipboard_history_hotkey: loaded.clipboard_history_hotkey ?? "",
          clipboard_history_max_items: loaded.clipboard_history_max_items ?? 300,
          launch_counts: loaded.launch_counts ?? {},
          last_settings_page: normalizeSettingsPage(loaded.last_settings_page),
          seen_tip: loaded.seen_tip ?? false,
          show_menubar_icon: loaded.show_menubar_icon ?? true,
          // R7-11: an older config has no alias map; an explicit `null` from a
          // hand-edited file must not leak into the search path either.
          command_aliases: loaded.command_aliases ?? {},
          // R7-13c: a pre-round settings file has no `ui_scale` key, and a
          // hand-edited one may carry a step that no longer ships; both land on
          // the shipped step. R47: a retired `larger` maps to `large` (keep the
          // user's largest surviving size), an absent/unknown value lands on the
          // new default `small`, and an explicit `default` (1) is preserved.
          ui_scale: normalizeUiScale(loaded.ui_scale),
          // R42 · the terminal's appearance. A pre-round file has none of these
          // keys; a hand-edited one may carry an out-of-range number or an
          // unknown step. Every one lands on its shipped value rather than on a
          // `0`/`false` that would silently change how the canvas paints.
          terminal_line_height: normalizeLineHeight(
            loaded.terminal_line_height ?? DEFAULT_LINE_HEIGHT,
          ),
          terminal_padding: normalizeTerminalPadding(
            loaded.terminal_padding ?? DEFAULT_TERMINAL_PADDING,
          ),
          terminal_cursor_blink: loaded.terminal_cursor_blink ?? DEFAULT_CURSOR_BLINK,
          terminal_theme: normalizeTerminalTheme(
            loaded.terminal_theme ?? DEFAULT_TERMINAL_THEME,
          ),
          terminal_scrollbar: loaded.terminal_scrollbar ?? DEFAULT_SCROLLBAR,
          // R43 · the interaction axes. A pre-round file has none of these keys;
          // a hand-edited one may carry an out-of-range number or an unknown
          // bold mode. Every one lands on its shipped value.
          // R44 · `DEFAULT_SELECT_COPY` is now `true`, so a file that predates
          // the key lands on the new default too — the backend's serde default
          // agrees, and the two must keep agreeing.
          terminal_wheel_lines: normalizeWheelLines(
            loaded.terminal_wheel_lines ?? DEFAULT_WHEEL_LINES,
          ),
          terminal_bold: normalizeBoldMode(loaded.terminal_bold ?? DEFAULT_BOLD_MODE),
          terminal_select_copy: normalizeSelectCopy(
            loaded.terminal_select_copy ?? DEFAULT_SELECT_COPY,
          ),
          terminal_paste_safe: normalizePasteSafe(
            loaded.terminal_paste_safe ?? DEFAULT_PASTE_SAFE,
          ),
          // R26-A: a file written before the browser plugin existed has no
          // `browser_plugin` block; the shipped defaults keep the plugin
          // usable without the user visiting its settings page.
          browser_plugin: {
            enabled: loaded.browser_plugin?.enabled ?? true,
            target: loaded.browser_plugin?.target ?? "auto",
            custom_base_dir: loaded.browser_plugin?.custom_base_dir ?? null,
            history_days: loaded.browser_plugin?.history_days ?? 30,
            cdp_enabled: loaded.browser_plugin?.cdp_enabled ?? false,
            cdp_port: loaded.browser_plugin?.cdp_port ?? 9222,
            sort_order: loaded.browser_plugin?.sort_order ?? "relevance",
            // R32 · a file written before this key existed falls back to `all`,
            // the shipped behaviour (title or URL).
            search_fields: loaded.browser_plugin?.search_fields ?? "all",
          },
          // R50 · a file written before the calculator plugin existed has no
          // `calculator_plugin` block; the normalizer fills the shipped
          // defaults and drops any illegal value.
          calculator_plugin: normalizeCalculatorSettings(loaded.calculator_plugin),
        };
        const hydrated = settingsHydration.mergeLoaded(
          settingsRef.current,
          normalized,
        );
        confirmedSettings.current = normalized;
        settingsRef.current = hydrated;
        setSettings(hydrated);
        // Reopen on the page the user last left settings on. The merged value
        // wins over `normalized`: a page chosen while the load was in flight
        // must not be clobbered by the disk snapshot.
        setSettingsPage(hydrated.last_settings_page);
        setSettingsLoadFailed(false);
        settingsHydration.finish();
      })
      .catch(() => {
        settingsHydration.markFailed();
        setSettingsLoadFailed(true);
      })
      .finally(() => {
        if (settingsLoadPromise.current === request) {
          settingsLoadPromise.current = null;
        }
        setSettingsLoading(false);
      });
    settingsLoadPromise.current = request;
    return request;
  }, [settingsHydration]);

  // ---- Change mutators -----------------------------------------------------
  /**
   * GLASS-REAXIS: the two transparency sliders are back, each with its own
   * target. One mutator moves one field — `main` is the launcher/settings
   * frame, `terminal` is the terminal frame — so the two are independently
   * configurable, exactly as the user asked. It never touches `glass_step`:
   * the effect stop and the background opacity are separate axes.
   */
  const changeOpacity = useCallback(
    (target: "main" | "terminal", next: number) => {
      const field = target === "main" ? "main_opacity" : "terminal_opacity";
      const value = clampWindowOpacity(next);
      if (value === settingsRef.current[field]) return;
      const updated: AppSettings = { ...settingsRef.current, [field]: value };
      settingsHydration.markChanged(field);
      settingsRef.current = updated;
      setSettings(updated);
      if (settingsSaveTimer.current !== null) {
        window.clearTimeout(settingsSaveTimer.current);
      }
      settingsSaveTimer.current = window.setTimeout(() => {
        settingsSaveTimer.current = null;
        persistSettings().catch(() => setSettingsSaveFailed(true));
      }, SETTINGS_DEBOUNCE_MS);
    },
    [settingsHydration, persistSettings],
  );

  /**
   * GLASS-REAXIS: the single glass-**effect** control. A stop writes only
   * `glass_step` — the blur, saturation and control-lens quality it selects —
   * and deliberately does **not** move either opacity field. GLASS-UNIFY spent
   * the stops on the tint axis (writing both opacities); the user rejected
   * that, so the stop and the transparency sliders are decoupled again. A stop
   * is a discrete choice, so it persists immediately like `changeGeneralSetting`
   * rather than through the slider debounce.
   */
  const changeGlassIntensity = useCallback(
    (level: GlassIntensity) => {
      const next = glassIntensitySettings(level);
      const current = settingsRef.current;
      if (next.glass_step === current.glass_step) return;
      const updated: AppSettings = {
        ...current,
        glass_step: normalizeGlassStep(next.glass_step),
      };
      settingsHydration.markChanged("glass_step");
      settingsRef.current = updated;
      setSettings(updated);
      suppressBlurUntil.current = Date.now() + SETTINGS_BLUR_SUPPRESS_MS;
      void persistSettings().catch(() => setSettingsSaveFailed(true));
    },
    [settingsHydration, persistSettings, suppressBlurUntil],
  );

  const changeFontSize = useCallback(
    (next: number) => {
      const fontSize = normalizeFontSize(next);
      if (fontSize === settingsRef.current.font_size) return;
      const updated = { ...settingsRef.current, font_size: fontSize };
      settingsHydration.markChanged("font_size");
      settingsRef.current = updated;
      setSettings(updated);
      if (settingsSaveTimer.current !== null) {
        window.clearTimeout(settingsSaveTimer.current);
      }
      settingsSaveTimer.current = window.setTimeout(() => {
        settingsSaveTimer.current = null;
        persistSettings().catch(() => setSettingsSaveFailed(true));
      }, SETTINGS_DEBOUNCE_MS);
    },
    [settingsHydration, persistSettings],
  );

  const changeGeneralSetting = useCallback(
    <K extends keyof AppSettings>(field: K, value: AppSettings[K]) => {
      if (pendingFields.current.has(field) || settingsRef.current[field] === value) return;
      pendingFields.current.add(field);
      const updated = { ...settingsRef.current, [field]: value };
      settingsHydration.markChanged(field);
      settingsRef.current = updated;
      setSettings(updated);
      suppressBlurUntil.current = Date.now() + SETTINGS_BLUR_SUPPRESS_MS;
      void persistSettings().catch(() => setSettingsSaveFailed(true))
        .finally(() => pendingFields.current.delete(field));
    },
    [settingsHydration, persistSettings, suppressBlurUntil],
  );

  /**
   * R42 · the terminal's line-height slider.
   *
   * The same debounce the font-size slider rides, for the same reason: a drag
   * emits a tick per pixel and `changeGeneralSetting` has no debounce, so it
   * would write the settings file dozens of times per gesture. The clamp/snap
   * is `normalizeLineHeight`'s, shared with the backend's own normalizer.
   */
  const changeTerminalLineHeight = useCallback(
    (next: number) => {
      const lineHeight = normalizeLineHeight(next);
      if (lineHeight === settingsRef.current.terminal_line_height) return;
      const updated = { ...settingsRef.current, terminal_line_height: lineHeight };
      settingsHydration.markChanged("terminal_line_height");
      settingsRef.current = updated;
      setSettings(updated);
      if (settingsSaveTimer.current !== null) {
        window.clearTimeout(settingsSaveTimer.current);
      }
      settingsSaveTimer.current = window.setTimeout(() => {
        settingsSaveTimer.current = null;
        persistSettings().catch(() => setSettingsSaveFailed(true));
      }, SETTINGS_DEBOUNCE_MS);
    },
    [settingsHydration, persistSettings],
  );

  /**
   * R7-11: set one command's alias.
   *
   * Rides the same `SETTINGS_DEBOUNCE_MS` window as the sliders, because the
   * field is a text input: one save per pause instead of one per keystroke. The
   * generic `changeGeneralSetting` path would write on every character — it has
   * no debounce — so this is a dedicated mutator rather than a re-use. Writing
   * an empty alias removes the entry (`withCommandAlias`), which is what makes
   * clearing the input a real removal and not an entry that matches everything.
   */
  const changeCommandAlias = useCallback(
    (command: string, alias: string) => {
      const next = withCommandAlias(settingsRef.current.command_aliases, command, alias);
      if (next === settingsRef.current.command_aliases) return;
      const updated: AppSettings = { ...settingsRef.current, command_aliases: next };
      settingsHydration.markChanged("command_aliases");
      settingsRef.current = updated;
      setSettings(updated);
      if (settingsSaveTimer.current !== null) {
        window.clearTimeout(settingsSaveTimer.current);
      }
      settingsSaveTimer.current = window.setTimeout(() => {
        settingsSaveTimer.current = null;
        persistSettings().catch(() => setSettingsSaveFailed(true));
      }, SETTINGS_DEBOUNCE_MS);
    },
    [settingsHydration, persistSettings],
  );

  /**
   * R7-13c · the interface-size step.
   *
   * A discrete choice (like `glass_step`), so it persists immediately rather
   * than through the slider debounce. It writes **only** `ui_scale`: the step is
   * the scale axis, and the terminal's own `font_size` is a separate, user-owned
   * axis this mutator must never touch (a step must not move the canvas's cell
   * size — see the decoupling marker in `terminal.css`). The `--ui-scale` knob
   * itself is applied by the effect in `App.tsx`, so it lands on the same commit
   * as the state change and before any measurement runs.
   */
  const changeUiScale = useCallback(
    (next: UiScale) => {
      const step = normalizeUiScale(next);
      if (step === settingsRef.current.ui_scale) return;
      const updated: AppSettings = { ...settingsRef.current, ui_scale: step };
      settingsHydration.markChanged("ui_scale");
      settingsRef.current = updated;
      setSettings(updated);
      suppressBlurUntil.current = Date.now() + SETTINGS_BLUR_SUPPRESS_MS;
      void persistSettings().catch(() => setSettingsSaveFailed(true));
    },
    [settingsHydration, persistSettings, suppressBlurUntil],
  );

  const changeTheme = useCallback(
    (theme: string) => {
      if (theme === settings.theme) return;
      changeGeneralSetting("theme", theme);
    },
    [settings.theme, changeGeneralSetting],
  );

  const changeLanguage = useCallback(
    (next: Language) => {
      if (next === settings.language) return;
      changeGeneralSetting("language", next);
    },
    [settings.language, changeGeneralSetting],
  );

  const changeLaunchAtStartup = useCallback(
    async (enabled: boolean) => {
      if (
        autostartBusy.current ||
        enabled === settingsRef.current.launch_at_startup
      ) {
        return;
      }
      autostartBusy.current = true;
      const previous = settingsRef.current.launch_at_startup;
      const updated: AppSettings = {
        ...settingsRef.current,
        launch_at_startup: enabled,
      };
      settingsHydration.markChanged("launch_at_startup");
      settingsRef.current = updated;
      setSettings(updated);
      setAutostartUpdating(true);
      suppressBlurUntil.current = Date.now() + SETTINGS_BLUR_SUPPRESS_MS;
      try {
        await invoke("set_launch_at_startup", { enabled });
        const latest = { ...settingsRef.current, launch_at_startup: enabled };
        settingsRef.current = latest;
        await persistSettings();
      } catch {
        setSettingsSaveFailed(true);
        await invoke("set_launch_at_startup", { enabled: previous }).catch(
          () => undefined,
        );
        setSettings((current) => {
          const rolledBack =
            current.launch_at_startup === enabled
              ? { ...current, launch_at_startup: previous }
              : current;
          settingsRef.current = rolledBack;
          return rolledBack;
        });
      } finally {
        autostartBusy.current = false;
        setAutostartUpdating(false);
      }
    },
    [
      settingsHydration,
      persistSettings,
      suppressBlurUntil,
      setAutostartUpdating,
    ],
  );

  /** Cancel any pending debounced save and persist the current snapshot
   *  synchronously. Used by the quit path so the user's last slider value is
   *  not lost when the cleanup effect cancels the timer. */
  const flushPendingSave = useCallback((): Promise<void> => {
    if (settingsSaveTimer.current !== null) {
      window.clearTimeout(settingsSaveTimer.current);
      settingsSaveTimer.current = null;
    }
    return persistSettings();
  }, [persistSettings]);

  return {
    dismissSaveError: () => setSettingsSaveFailed(false),
    // State values (consumed by App.tsx JSX and other hooks)
    settings,
    setSettings,
    settingsPage,
    setSettingsPage,
    settingsSaving,
    settingsSaveFailed,
    settingsLoading,
    settingsLoadFailed,
    // Refs and helpers consumed by other hooks
    settingsRef,
    settingsHydration,
    // Functions
    loadSettings,
    commitSettings,
    persistSettings,
    changeGlassIntensity,
    changeOpacity,
    changeFontSize,
    changeTerminalLineHeight,
    changeUiScale,
    changeGeneralSetting,
    changeCommandAlias,
    changeTheme,
    changeLanguage,
    changeLaunchAtStartup,
    flushPendingSave,
  };
}
