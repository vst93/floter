import { X } from "lucide-react";
import { ShortcutRecorder } from "../ShortcutRecorder";
import type { Translate, MessageKey } from "../i18n";
import { SHORTCUT_ACTIONS, type ShortcutMap } from "../shortcuts";

/** Pseudo action id under which the clipboard hotkey is recorded in settings UI. */
export const CLIPBOARD_HOTKEY_ACTION = "clipboard_hotkey";

type ShortcutsPageProps = {
  t: Translate;
  shortcuts: ShortcutMap;
  clipboardHotkey: string;
  rejectedAction: string | null;
  recordingAction: string | null;
  onToggleRecording: (action: string) => void;
  onCaptureShortcut: (action: string, next: string) => void;
  onCancelRecording: () => void;
  onRestoreDefaults: () => void;
  onClearClipboardHotkey: () => void;
};

/** The shortcuts settings page: one recorder per action plus the clipboard
 * panel hotkey. All state lives in `App` and arrives through props. */
export function ShortcutsPage({
  t,
  shortcuts,
  clipboardHotkey,
  rejectedAction,
  recordingAction,
  onToggleRecording,
  onCaptureShortcut,
  onCancelRecording,
  onRestoreDefaults,
  onClearClipboardHotkey,
}: ShortcutsPageProps) {
  return (
    <section className="settings-section">
      <div className="settings-section__heading">
        <h2 className="settings-section__label">{t("settings.shortcuts")}</h2>
        <button
          type="button"
          className="settings-reset"
          title={t("settings.shortcutsResetHint")}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => void onRestoreDefaults()}
        >
          <span className="settings-reset__icon" aria-hidden="true">↺</span>
          <span>{t("settings.shortcutsReset")}</span>
        </button>
      </div>
      <div className="settings-options">
        {SHORTCUT_ACTIONS.map((action) => {
          const labelKey: MessageKey = `shortcut.${action}`;
          const rejected = rejectedAction === action;
          return (
            <div key={action} className="settings-option settings-option--static">
              <span className="settings-option__main">
                <span className="settings-option__label">{t(labelKey)}</span>
                {rejected && (
                  <span className="settings-option__description settings-option__description--warning">
                    {t("settings.shortcut.rejected")}
                  </span>
                )}
              </span>
              <ShortcutRecorder
                action={action}
                shortcut={shortcuts[action]}
                recording={recordingAction === action}
                onToggle={onToggleRecording}
                onCapture={onCaptureShortcut}
                onCancel={onCancelRecording}
                t={t}
              />
            </div>
          );
        })}
        {/* The clipboard panel's trigger is stored as its own settings
            field (`clipboard_history_hotkey`), not in the shortcuts
            map — recording reuses the shared CLIPBOARD_HOTKEY_ACTION
            plumbing above. Default is disabled: empty means no global
            hotkey is registered. */}
        <div className="settings-option settings-option--static">
          <span className="settings-option__main">
            <span className="settings-option__label">{t("shortcut.clipboard_panel")}</span>
            {rejectedAction === CLIPBOARD_HOTKEY_ACTION && (
              <span className="settings-option__description settings-option__description--warning">
                {t("settings.shortcut.rejected")}
              </span>
            )}
          </span>
          <span className="settings-option__hotkey-controls">
            <ShortcutRecorder
              action={CLIPBOARD_HOTKEY_ACTION}
              shortcut={clipboardHotkey}
              recording={recordingAction === CLIPBOARD_HOTKEY_ACTION}
              onToggle={onToggleRecording}
              onCapture={onCaptureShortcut}
              onCancel={onCancelRecording}
              t={t}
            />
            {clipboardHotkey && (
              <button
                type="button"
                className="session-manager__icon-button session-manager__icon-button--danger"
                aria-label={t("settings.clipboardHotkeyClear")}
                title={t("settings.clipboardHotkeyClear")}
                onMouseDown={(event) => event.preventDefault()}
                onClick={onClearClipboardHotkey}
              >
                <X size={14} strokeWidth={1.9} aria-hidden="true" />
              </button>
            )}
          </span>
        </div>
      </div>
      <p className="settings-section__hint">{t("settings.shortcutsHint")}</p>
    </section>
  );
}
