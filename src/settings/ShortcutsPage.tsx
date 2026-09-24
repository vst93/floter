import { useEffect, useState } from "react";
import { Trash2, X } from "lucide-react";
import { invoke } from "@tauri-apps/api/core";
import { ShortcutRecorder } from "../ShortcutRecorder";
import type { Translate, MessageKey } from "../i18n";
import { SHORTCUT_ACTIONS, type ShortcutMap } from "../shortcuts";
import {
  CUSTOM_SHORTCUT_PICKER,
  classifyCustomShortcutAction,
  customShortcutKeysEqual,
  duplicateCustomShortcutKey,
  type CustomShortcut,
} from "../custom-shortcuts";
import { SettingsAction, SettingsCard, SettingsRow } from "./SettingsRows";

type ShortcutsPageProps = {
  busy: boolean;
  t: Translate;
  shortcuts: ShortcutMap;
  rejectedAction: string | null;
  recordingAction: string | null;
  onToggleRecording: (action: string) => void;
  onCaptureShortcut: (action: string, next: string) => void;
  onCancelRecording: () => void;
  onRestoreDefaults: () => void;
  /** R55 · clear one action's binding. The only action that ships empty is the
   *  clipboard panel (empty = disabled), but the path is generic: the map owns
   *  the "disabled" meaning, exactly as `toggle_window`'s absence would. */
  onClearShortcut: (action: string) => void;
  /** R55 · the user-defined global shortcuts and their persistence path. */
  customShortcuts: CustomShortcut[];
  /** Persist `list`; resolves with the keys the OS refused and why. */
  onCommitCustomShortcuts: (
    list: CustomShortcut[],
  ) => Promise<{ key: string; reason: string }[]>;
  /** Keys the last commit could not register, keyed by key for display. */
  customRejections: { key: string; reason: string }[];
};

/** R55 · the custom global shortcuts section.
 *
 *  Each row binds one OS key to one launcher action. The action picker lists
 *  the plugins (opened normally) and the app's own actions; a row whose action
 *  is a free-form command line keeps a text field. Recording reuses
 *  `ShortcutRecorder`, and the global shortcuts are suspended for the duration
 *  exactly as they are for the built-in map. */
function CustomShortcutsSection({
  busy,
  t,
  customShortcuts,
  onCommitCustomShortcuts,
  customRejections,
}: Pick<
  ShortcutsPageProps,
  "busy" | "t" | "customShortcuts" | "onCommitCustomShortcuts" | "customRejections"
>) {
  // -1 records a brand-new row; otherwise it is the row index being edited.
  const [recording, setRecording] = useState<number | null>(null);
  const [localRejection, setLocalRejection] = useState<string | null>(null);
  const [commandDrafts, setCommandDrafts] = useState<Record<number, string>>({});

  // A recording that is abandoned by navigating away must not leave the global
  // shortcuts suspended, or the window toggle would be dead until restart.
  useEffect(
    () => () => {
      invoke("resume_shortcuts").catch(() => undefined);
    },
    [],
  );

  const rejectionFor = (key: string) =>
    customRejections.find((entry) => customShortcutKeysEqual(entry.key, key));

  const beginRecording = (index: number) => {
    setLocalRejection(null);
    setRecording(index);
    invoke("suspend_shortcuts").catch(() => undefined);
  };

  const finishRecording = () => {
    setRecording(null);
    invoke("resume_shortcuts").catch(() => undefined);
  };

  const capture = async (index: number, next: string) => {
    finishRecording();
    const otherKeys = customShortcuts.map((entry) => entry.key);
    const duplicate = duplicateCustomShortcutKey(next, otherKeys, index);
    if (duplicate) {
      setLocalRejection(next);
      return;
    }
    setLocalRejection(null);
    const list =
      index === -1
        ? [...customShortcuts, { key: next, action: CUSTOM_SHORTCUT_PICKER[0].action }]
        : customShortcuts.map((entry, at) => (at === index ? { ...entry, key: next } : entry));
    await onCommitCustomShortcuts(list);
  };

  const setAction = async (index: number, action: string) => {
    const list = customShortcuts.map((entry, at) => (at === index ? { ...entry, action } : entry));
    await onCommitCustomShortcuts(list);
  };

  const remove = async (index: number) => {
    await onCommitCustomShortcuts(customShortcuts.filter((_, at) => at !== index));
  };

  return (
    <section className="settings-section">
      <div className="settings-section__heading">
        <div className="settings-section__heading-main">
          <h2 className="settings-section__label">{t("settings.group.customShortcuts")}</h2>
        </div>
        <SettingsAction
          title={t("settings.customShortcutsAddHint")}
          onClick={() => beginRecording(-1)}
        >
          <span className="settings-reset__icon" aria-hidden="true">＋</span>
          {t("settings.customShortcutsAdd")}
        </SettingsAction>
      </div>
      <SettingsCard label={t("settings.group.customShortcuts")}>
        {recording === -1 && (
          <SettingsRow
            label={t("settings.customShortcutsNew")}
            control={
              <ShortcutRecorder
                action="custom:-1"
                shortcut=""
                recording
                onToggle={() => finishRecording()}
                onCapture={(_action, next) => void capture(-1, next)}
                onCancel={finishRecording}
                t={t}
              />
            }
          />
        )}
        {customShortcuts.length === 0 && recording !== -1 && (
          <p className="settings-section__hint">{t("settings.customShortcutsEmpty")}</p>
        )}
        {customShortcuts.map((entry, index) => {
          const binding = classifyCustomShortcutAction(entry.action);
          const rejected = rejectionFor(entry.key) ?? (localRejection === entry.key ? { key: entry.key, reason: "conflict" } : undefined);
          const commandDraft = commandDrafts[index] ?? binding.value;
          const pickerValue = binding.kind === "command" ? "custom" : entry.action;
          return (
            <SettingsRow
              key={`${index}-${entry.key}`}
              label={
                <span className="custom-shortcut__key">
                  <ShortcutRecorder
                    action={`custom:${index}`}
                    shortcut={entry.key}
                    recording={recording === index}
                    onToggle={() => beginRecording(index)}
                    onCapture={(_action, next) => void capture(index, next)}
                    onCancel={finishRecording}
                    t={t}
                  />
                  <button
                    type="button"
                    className="session-manager__icon-button session-manager__icon-button--danger"
                    aria-label={t("settings.customShortcutsRemove")}
                    title={t("settings.customShortcutsRemove")}
                    disabled={busy}
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => void remove(index)}
                  >
                    <Trash2 size={14} strokeWidth={1.9} aria-hidden="true" />
                  </button>
                </span>
              }
              sublabel={
                rejected ? (
                  <span className="settings-option__description--warning">
                    {rejected.reason === "occupied"
                      ? t("settings.customShortcutsOccupied")
                      : t("settings.customShortcutsConflict")}
                  </span>
                ) : undefined
              }
              control={
                <span className="custom-shortcut__action">
                  <select
                    className="settings-select"
                    value={pickerValue}
                    aria-label={t("settings.customShortcutsAction")}
                    disabled={busy}
                    onChange={(event) => {
                      const value = event.currentTarget.value;
                      if (value === "custom") {
                        setCommandDrafts((drafts) => ({ ...drafts, [index]: binding.value }));
                        return;
                      }
                      void setAction(index, value);
                    }}
                  >
                    {CUSTOM_SHORTCUT_PICKER.map((option) => (
                      <option key={option.action} value={option.action}>
                        {t(option.labelKey)}
                      </option>
                    ))}
                    <option value="custom">{t("settings.customShortcutsCustom")}</option>
                  </select>
                  {pickerValue === "custom" && (
                    <input
                      type="text"
                      className="settings-select custom-shortcut__command"
                      value={commandDraft}
                      placeholder={t("settings.customShortcutsCommandPlaceholder")}
                      aria-label={t("settings.customShortcutsCommand")}
                      disabled={busy}
                      spellCheck={false}
                      onChange={(event) =>
                        setCommandDrafts((drafts) => ({ ...drafts, [index]: event.currentTarget.value }))
                      }
                      onBlur={() => {
                        const next = (commandDrafts[index] ?? "").trim();
                        if (next && next !== binding.value) void setAction(index, next);
                      }}
                      onKeyDown={(event) => {
                        if (event.key === "Enter") {
                          event.preventDefault();
                          const next = (commandDrafts[index] ?? "").trim();
                          if (next && next !== binding.value) void setAction(index, next);
                        }
                      }}
                    />
                  )}
                </span>
              }
            />
          );
        })}
      </SettingsCard>
      <p className="settings-section__hint">{t("settings.customShortcutsHint")}</p>
    </section>
  );
}

/** The shortcuts settings page: one recorder per action, all driven by the one
 *  `SHORTCUT_ACTIONS` registry, plus the custom global shortcut section. All
 *  state lives in `App` and arrives through props.
 *
 *  R55 · the clipboard panel's trigger used to be a bespoke row outside the
 *  map (`clipboard_history_hotkey` + its own clear plumbing). It is now an
 *  ordinary member of `SHORTCUT_ACTIONS`, so it shares the row family, the
 *  restore-defaults behaviour and the conflict checker with every other key;
 *  the only thing that stays special is that an empty value means "disabled",
 *  which is why it is the one row that offers a clear button.
 *
 *  The group title row carries "Restore defaults" as the reference pane's
 *  right-aligned blue text action, and the group's explanation sits below the
 *  card as a footnote — the two positions the reference uses. */
export function ShortcutsPage({
  busy,
  t,
  shortcuts,
  rejectedAction,
  recordingAction,
  onToggleRecording,
  onCaptureShortcut,
  onCancelRecording,
  onRestoreDefaults,
  onClearShortcut,
  customShortcuts,
  onCommitCustomShortcuts,
  customRejections,
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
            // The clipboard panel is the one disabled-by-default action: its
            // empty value is a legitimate state and gets a clear control.
            const clearable = action === "clipboard_panel";
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
                  <>
                    <ShortcutRecorder
                      action={action}
                      shortcut={shortcuts[action]}
                      recording={recordingAction === action}
                      onToggle={onToggleRecording}
                      onCapture={onCaptureShortcut}
                      onCancel={onCancelRecording}
                      t={t}
                    />
                    {clearable && shortcuts[action] && (
                      <button
                        type="button"
                        className="session-manager__icon-button session-manager__icon-button--danger"
                        aria-label={t("settings.clipboardHotkeyClear")}
                        title={t("settings.clipboardHotkeyClear")}
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => onClearShortcut(action)}
                      >
                        <X size={14} strokeWidth={1.9} aria-hidden="true" />
                      </button>
                    )}
                  </>
                }
              />
            );
          })}
        </SettingsCard>
        <p className="settings-section__hint">{t("settings.shortcutsHint")}</p>
      </section>

      <CustomShortcutsSection
        busy={busy}
        t={t}
        customShortcuts={customShortcuts}
        onCommitCustomShortcuts={onCommitCustomShortcuts}
        customRejections={customRejections}
      />
    </div>
    </fieldset>
  );
}
