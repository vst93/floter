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
import { normalizeFontSize, normalizeOpacity } from "../settings/GeneralPage";
import {
  createSerialSettingsWriter,
  createSettingsHydration,
  normalizeSettingsPage,
  type SettingsPage,
} from "../settings-persistence";
import { DEFAULT_SHORTCUTS, withShortcutDefaults } from "../shortcuts";
import { normalizeLanguage, type Language } from "../i18n";
import type { AppSettings } from "../App";

/** Defaults applied before the first disk read returns. */
const SETTINGS_DEFAULTS: AppSettings = {
  hotkey: "Ctrl+Space",
  hide_on_blur: true,
  launch_at_startup: false,
  theme: "dark",
  font_size: 14,
  font_family: "monospace",
  cursor_shape: "beam",
  language: "en",
  main_opacity: 94,
  terminal_opacity: 92,
  shortcuts: DEFAULT_SHORTCUTS,
  show_commands_in_search: false,
  show_recent_in_launcher: true,
  clipboard_history_enabled: true,
  // The clipboard panel ships with NO global hotkey; users may bind one on
  // the shortcuts settings page.
  clipboard_history_hotkey: "",
  launch_counts: {},
  last_settings_page: "general",
};

/** Debounce window for the slider-driven opacity and font-size writes. */
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
  const { suppressBlurUntil, autostartUpdating, setAutostartUpdating } = options;

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
          if (settingsSaveGeneration.current !== generation) return;
          setSettingsSaving(false);
          setSettingsSaveFailed(false);
        },
        (error) => {
          if (settingsSaveGeneration.current === generation) {
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
          main_opacity: normalizeOpacity(loaded.main_opacity ?? 94),
          terminal_opacity: normalizeOpacity(loaded.terminal_opacity ?? 92),
          shortcuts: withShortcutDefaults(loaded.shortcuts),
          clipboard_history_enabled: loaded.clipboard_history_enabled ?? true,
          clipboard_history_hotkey: loaded.clipboard_history_hotkey ?? "",
          launch_counts: loaded.launch_counts ?? {},
          last_settings_page: normalizeSettingsPage(loaded.last_settings_page),
        };
        const hydrated = settingsHydration.mergeLoaded(
          settingsRef.current,
          normalized,
        );
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
  const changeOpacity = useCallback(
    (field: "main_opacity" | "terminal_opacity", next: number) => {
      const value = normalizeOpacity(next);
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
      if (settingsRef.current[field] === value) return;
      const updated = { ...settingsRef.current, [field]: value };
      settingsHydration.markChanged(field);
      settingsRef.current = updated;
      setSettings(updated);
      suppressBlurUntil.current = Date.now() + SETTINGS_BLUR_SUPPRESS_MS;
      persistSettings().catch(() => setSettingsSaveFailed(true));
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
        autostartUpdating ||
        enabled === settingsRef.current.launch_at_startup
      ) {
        return;
      }
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
        setAutostartUpdating(false);
      }
    },
    [
      autostartUpdating,
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
    changeOpacity,
    changeFontSize,
    changeGeneralSetting,
    changeTheme,
    changeLanguage,
    changeLaunchAtStartup,
    flushPendingSave,
  };
}
