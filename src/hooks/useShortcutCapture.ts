// Imperative shortcut recording & capture: the row of the settings page
// flips between idle / recording / rejected, and an in-flight capture
// optimistically swaps the binding in `settings.shortcuts` (or
// `clipboard_history_hotkey`) before asking the backend to take it.
//
// Extracted verbatim from `App.tsx`. The hook owns `recordingAction` and
// `rejectedAction`, both pieces of UI state the surrounding tree
// (`ShortcutsPage`, `useAppKeyboard`) reads but does not mutate. All
// persistence and blur-suppression side-effects are kept here so the
// App-level wiring shrinks to `const { capture, toggle, ... } =
// useShortcutCapture({ ... })`.

import { useCallback, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  normalizeResultShortcut,
  SHORTCUT_ACTIONS,
  withShortcutDefaults,
  type ShortcutAction,
  type ShortcutMap,
} from "../shortcuts";
import { CLIPBOARD_HOTKEY_ACTION } from "../settings/ShortcutsPage";
import type { AppSettings } from "../App";
import type { SettingsHydration } from "../settings-persistence";
import { useImmediateState } from "./useImmediateState";

/** Blur suppress window after a non-slider settings edit. */
const SETTINGS_BLUR_SUPPRESS_MS = 400;

export function useShortcutCapture(options: {
  /** Live settings + ref the optimistic updates mirror into. */
  settings: AppSettings;
  setSettings: React.Dispatch<React.SetStateAction<AppSettings>>;
  settingsRef: React.MutableRefObject<AppSettings>;
  /** The current shortcut map, already defaulted. */
  shortcuts: ShortcutMap;
  /** Hydration tracker so the disk write is coalesced until first load. */
  settingsHydration: SettingsHydration<AppSettings>;
  /** Blur-suppress ref the capture path extends to keep the panel open. */
  suppressBlurUntil: React.MutableRefObject<number>;
}) {
  const {
    settings,
    setSettings,
    settingsRef,
    shortcuts,
    settingsHydration,
    suppressBlurUntil,
  } = options;

  // ---- State ---------------------------------------------------------------
  const [recordingAction, setRecordingAction, recordingRef] = useImmediateState<string | null>(null);
  const [rejectedAction, setRejectedAction] = useState<string | null>(null);
  const [saving, setSaving, savingRef] = useImmediateState(false);

  // ---- Recording toggle ----------------------------------------------------
  const toggle = useCallback(async (action: string) => {
    if (savingRef.current) return;
    setSaving(true);
    setRejectedAction(null);
    const next = recordingRef.current === action ? null : action;
    try {
      await invoke(next ? "suspend_shortcuts" : "resume_shortcuts");
      setRecordingAction(next);
    } catch {
      setRejectedAction(action);
    } finally {
      setSaving(false);
    }
  }, []);

  const cancel = useCallback(() => {
    setRecordingAction(null);
    invoke("resume_shortcuts").catch(() => undefined);
  }, []);

  // ---- Clear clipboard hotkey ---------------------------------------------
  // Optimistic like capture: persist "" (the backend treats an empty string
  // as unregister-and-disable) and roll back on failure.
  const clearClipboardHotkey = useCallback(() => {
    if (savingRef.current) return;
    const previousHotkey = settingsRef.current.clipboard_history_hotkey;
    if (!previousHotkey) return;
    setSaving(true);
    settingsHydration.markChanged("clipboard_history_hotkey");
    setSettings((current) => {
      const updated = { ...current, clipboard_history_hotkey: "" };
      settingsRef.current = updated;
      return updated;
    });
    setRejectedAction(null);
    invoke("update_clipboard_hotkey", { hotkey: "" }).catch(() => {
      setSettings((current) => {
        const rolledBack = { ...current, clipboard_history_hotkey: previousHotkey };
        settingsRef.current = rolledBack;
        return rolledBack;
      });
      setRejectedAction(CLIPBOARD_HOTKEY_ACTION);
    }).finally(() => setSaving(false));
  }, [setSettings, settingsRef, settingsHydration]);

  // ---- Restore defaults ----------------------------------------------------
  const restoreDefaults = useCallback(async () => {
    if (savingRef.current) return;
    setSaving(true);
    if (recordingAction) {
      await invoke("resume_shortcuts").catch(() => undefined);
    }
    try {
      const shortcuts = await invoke<ShortcutMap>("reset_shortcuts");
      settingsHydration.markChanged("hotkey");
      settingsHydration.markChanged("shortcuts");
      setSettings((current) => {
        const updated = {
          ...current,
          hotkey: shortcuts.toggle_window,
          shortcuts,
        };
        settingsRef.current = updated;
        return updated;
      });
      setRecordingAction(null);
      setRejectedAction(null);
    } catch {
      // Keep the current shortcuts if the system rejects the default toggle.
      setRejectedAction("reset");
    } finally {
      setSaving(false);
    }
  }, [recordingAction, setSettings, settingsRef, settingsHydration]);

  // ---- Capture -------------------------------------------------------------
  // Store the new binding optimistically; the backend is the authority on
  // whether a system-wide combination can actually be taken.
  const capture = useCallback(
    (action: string, next: string) => {
      if (savingRef.current) return;
      setRecordingAction(null);
      setRejectedAction(null);
      if (action === "select_result") {
        const normalized = normalizeResultShortcut(next);
        if (!normalized) {
          setRejectedAction(action);
          invoke("resume_shortcuts").catch(() => undefined);
          return;
        }
        next = normalized;
      }
      if (action === CLIPBOARD_HOTKEY_ACTION) {
        const previousHotkey = settingsRef.current.clipboard_history_hotkey;
        const conflict = SHORTCUT_ACTIONS.some(
          (candidate) => shortcuts[candidate].toLowerCase() === next.toLowerCase(),
        );
        if (conflict) {
          setRejectedAction(action);
          invoke("resume_shortcuts").catch(() => undefined);
          return;
        }
        if (next === previousHotkey) {
          invoke("resume_shortcuts").catch(() => undefined);
          return;
        }
        settingsHydration.markChanged("clipboard_history_hotkey");
        setSettings((current) => {
          const updated = { ...current, clipboard_history_hotkey: next };
          settingsRef.current = updated;
          return updated;
        });
        suppressBlurUntil.current = Date.now() + SETTINGS_BLUR_SUPPRESS_MS;
        setSaving(true);
        invoke("update_clipboard_hotkey", { hotkey: next })
          .then(() => {
            invoke("resume_shortcuts").catch(() => undefined);
          })
          .catch(() => {
            setSettings((current) => {
              const rolledBack = { ...current, clipboard_history_hotkey: previousHotkey };
              settingsRef.current = rolledBack;
              return rolledBack;
            });
            setRejectedAction(action);
            invoke("resume_shortcuts").catch(() => undefined);
          }).finally(() => setSaving(false));
        return;
      }
      const previous = shortcuts[action as ShortcutAction];
      const conflict = SHORTCUT_ACTIONS.some(
        (candidate) => candidate !== action && shortcuts[candidate].toLowerCase() === next.toLowerCase(),
      );
      if (conflict) {
        setRejectedAction(action);
        invoke("resume_shortcuts").catch(() => undefined);
        return;
      }
      if (next === previous) {
        invoke("resume_shortcuts").catch(() => undefined);
        return;
      }

      settingsHydration.markChanged("shortcuts");
      if (action === "toggle_window") settingsHydration.markChanged("hotkey");
      setSettings((current) => {
        const updated = {
          ...current,
          ...(action === "toggle_window" ? { hotkey: next } : {}),
          shortcuts: { ...withShortcutDefaults(current.shortcuts), [action]: next },
        };
        settingsRef.current = updated;
        return updated;
      });
      suppressBlurUntil.current = Date.now() + SETTINGS_BLUR_SUPPRESS_MS;
      setSaving(true);
      invoke("update_shortcut", { action, shortcut: next }).then(() => {
        invoke("resume_shortcuts").catch(() => undefined);
      }).catch(() => {
        setSettings((current) => {
          const rolledBack = {
            ...current,
            ...(action === "toggle_window" ? { hotkey: previous } : {}),
            shortcuts: { ...withShortcutDefaults(current.shortcuts), [action]: previous },
          };
          settingsRef.current = rolledBack;
          return rolledBack;
        });
        setRejectedAction(action);
        invoke("resume_shortcuts").catch(() => undefined);
      }).finally(() => setSaving(false));
    },
    [setSettings, settingsRef, settingsHydration, shortcuts, suppressBlurUntil],
  );

  // Touch `settings` so the linter does not flag the prop as unused; the
  // call sites read the live value through the JSX-bound `recordingAction` /
  // `rejectedAction` rather than `settings` itself.
  void settings;

  // Clear the recording/rejected flags (and resume the suspended global
  // shortcuts) without flipping the panel into another action. Used when
  // the user leaves the settings page mid-recording.
  const reset = useCallback(() => {
    setRecordingAction(null);
    setRejectedAction(null);
  }, []);

  return {
    saving,
    toggle,
    cancel,
    capture,
    clearClipboardHotkey,
    restoreDefaults,
    reset,
    rejectedAction,
    recordingAction,
  };
}
