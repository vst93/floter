import { X } from "lucide-react";
import { ShortcutRecorder } from "../ShortcutRecorder";
import type { Translate, MessageKey } from "../i18n";
import { SHORTCUT_ACTIONS, type ShortcutMap } from "../shortcuts";
import { SettingsAction, SettingsCard, SettingsRow } from "./SettingsRows";

/** Pseudo action id under which the clipboard hotkey is recorded in settings UI. */
export const CLIPBOARD_HOTKEY_ACTION = "clipboard_hotkey";

type ShortcutsPageProps = {
  busy: boolean;
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
 *  panel hotkey. All state lives in `App` and arrives through props.
 *
 *  The group title row carries "Restore defaults" as the reference pane's
 *  right-aligned blue text action, and the group's explanation sits below the
 *  card as a footnote — the two positions the reference uses. */
export function ShortcutsPage({
  busy,
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
    <fieldset className="settings-controls" disabled={busy} aria-busy={busy}>
    <div className="settings-page">
      <header className="settings-page__header">
        <h1 className="settings-page__title">{t("settings.menu.shortcuts")}</h1>
        <p className="settings-page__subtitle">{t("settings.page.shortcuts")}</p>
      </header>
      {rejectedAction && <div className="settings-save-alert" role="alert">{t("settings.shortcut.rejected")}</div>}
      <section className="settings-section">
        <div className="settings-section__heading">
          <div className="settings-section__heading-main">
            <h2 className="settings-section__label">{t("settings.group.shortcuts")}</h2>
          </div>
          <SettingsAction
            title={t("settings.shortcutsResetHint")}
            onClick={() => void onRestoreDefaults()}
          >
            <span className="settings-reset__icon" aria-hidden="true">↺</span>
            {t("settings.shortcutsReset")}
          </SettingsAction>
        </div>
        <SettingsCard label={t("settings.group.shortcuts")}>
          {SHORTCUT_ACTIONS.map((action) => {
            const labelKey: MessageKey = `shortcut.${action}`;
            const rejected = rejectedAction === action;
            return (
              <SettingsRow
                key={action}
                label={t(labelKey)}
                sublabel={rejected ? (
                  <span className="settings-option__description--warning">
                    {t("settings.shortcut.rejected")}
                  </span>
                ) : undefined}
                control={
                  <ShortcutRecorder
                    action={action}
                    shortcut={shortcuts[action]}
                    recording={recordingAction === action}
                    onToggle={onToggleRecording}
                    onCapture={onCaptureShortcut}
                    onCancel={onCancelRecording}
                    t={t}
                  />
                }
              />
            );
          })}
          {/* The clipboard panel's trigger is stored as its own settings
              field (`clipboard_history_hotkey`), not in the shortcuts
              map — recording reuses the shared CLIPBOARD_HOTKEY_ACTION
              plumbing above. Default is disabled: empty means no global
              hotkey is registered. */}
          <SettingsRow
            label={t("shortcut.clipboard_panel")}
            sublabel={rejectedAction === CLIPBOARD_HOTKEY_ACTION ? (
              <span className="settings-option__description--warning">
                {t("settings.shortcut.rejected")}
              </span>
            ) : undefined}
            control={
              <>
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
              </>
            }
          />
        </SettingsCard>
        <p className="settings-section__hint">{t("settings.shortcutsHint")}</p>
      </section>
    </div>
    </fieldset>
  );
}
