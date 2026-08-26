import { useEffect } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  formatShortcut,
  shortcutFromEvent,
  IS_WINDOWS,
} from "./shortcuts";
import type { Translate } from "./i18n";

type ShortcutRecorderProps = {
  action: string;
  shortcut: string;
  recording: boolean;
  onToggle: (action: string) => void;
  onCapture: (action: string, shortcut: string) => void;
  onCancel: () => void;
  t: Translate;
};

/**
 * A single rebindable shortcut.
 *
 * While recording it owns the keyboard: the listener runs in the capture phase
 * and stops propagation, so neither the app's own handler nor the browser sees
 * the combination being pressed. A press without any modifier is ignored —
 * binding a bare letter would swallow it everywhere in the app.
 *
 * Shared by the shortcuts settings page and the base-plugin rows in the
 * extensions panel (the clipboard hotkey is recorded there now).
 */
export function ShortcutRecorder({
  action,
  shortcut,
  recording,
  onToggle,
  onCapture,
  onCancel,
  t,
}: ShortcutRecorderProps) {
  // Windows swallows Alt+Space so its window menu never opens over the panel,
  // which also keeps the combination from ever reaching this recorder. The flag
  // lifts that for as long as one is listening; the cleanup runs on capture, on
  // cancel and on unmount, so it cannot be left raised. Kept in its own effect,
  // keyed on nothing but `recording`, so a re-render of the settings panel does
  // not lower and raise it again mid-recording.
  useEffect(() => {
    if (!IS_WINDOWS || !recording) return;
    invoke("set_recording_flag", { on: true }).catch(() => undefined);
    return () => {
      invoke("set_recording_flag", { on: false }).catch(() => undefined);
    };
  }, [recording]);

  useEffect(() => {
    if (!recording) return;

    const onKeyDown = (event: KeyboardEvent) => {
      event.preventDefault();
      event.stopPropagation();

      const bare = !event.ctrlKey && !event.altKey && !event.shiftKey && !event.metaKey;
      if (event.key === "Escape" && bare) {
        onCancel();
        return;
      }
      const next = shortcutFromEvent(event);
      if (!next) return;
      if (bare && !/^F\d{1,2}$/.test(next)) return;
      onCapture(action, next);
    };

    // If the window loses focus while recording (user clicked away, Cmd-Tab,
    // etc.), cancel immediately and restore shortcuts. Otherwise the global
    // shortcuts stay suspended and the user's hotkey is dead.
    const onBlur = () => onCancel();

    window.addEventListener("keydown", onKeyDown, true);
    window.addEventListener("blur", onBlur);
    return () => {
      window.removeEventListener("keydown", onKeyDown, true);
      window.removeEventListener("blur", onBlur);
    };
  }, [action, onCancel, onCapture, recording]);

  return (
    <button
      type="button"
      className={`shortcut-recorder${recording ? " shortcut-recorder--recording" : ""}`}
      aria-label={t("settings.shortcut.record")}
      title={t("settings.shortcut.record")}
      onMouseDown={(event) => event.preventDefault()}
      onClick={() => onToggle(action)}
    >
      {recording ? (
        <span className="shortcut-recorder__prompt">{t("settings.shortcut.recording")}</span>
      ) : (
        <span className="shortcut-recorder__keys">{formatShortcut(shortcut)}</span>
      )}
    </button>
  );
}
