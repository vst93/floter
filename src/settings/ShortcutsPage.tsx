import { useEffect, useRef, useState } from "react";
import { Keyboard, Trash2 } from "lucide-react";
import { invoke } from "@tauri-apps/api/core";
import { ShortcutRecorder } from "../ShortcutRecorder";
import type { Translate, MessageKey } from "../i18n";
import { SHORTCUT_ACTIONS, type ShortcutMap } from "../shortcuts";
import {
  CUSTOM_SHORTCUT_PICKER,
  classifyCustomShortcutAction,
  customShortcutKeysEqual,
  duplicateCustomShortcutKey,
  normalizeCustomShortcuts,
  type CustomShortcut,
} from "../custom-shortcuts";
import { SettingsAction, SettingsCard, SettingsEmpty, SettingsRow } from "./SettingsRows";

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
  /** R55 · the user-defined global shortcuts and their persistence path. */
  customShortcuts: CustomShortcut[];
  /** Persist `list`; resolves with the keys the OS refused and why. */
  onCommitCustomShortcuts: (
    list: CustomShortcut[],
  ) => Promise<{ key: string; reason: string }[]>;
  /** Keys the last commit could not register, keyed by key for display. */
  customRejections: { key: string; reason: string }[];
};

/** R56 · one editable custom-shortcut row, including the drafts.
 *
 *  The persisted shape is still `CustomShortcut` (a key and an action), but the
 *  UI needs three extra things the stored shape cannot express: a stable id so
 *  React keys and the recording flag survive a key edit, the text of an
 *  unfinished command line, and whether the picker is currently on "Command
 *  line…". A row is *complete* — and only then persisted — when both halves are
 *  set; an incomplete row is a local draft that never reaches the OS. */
type CustomRow = {
  id: number;
  key: string;
  /** The stored action string; "" while the row is an unconfigured draft. */
  action: string;
  /** The command-line text. Kept apart from `action` so an unfinished command
   *  never overwrites a working binding. */
  command: string;
  /** The picker shows "Command line…" and the text field is visible. */
  commandMode: boolean;
};

const rowFromEntry = (entry: CustomShortcut, id: number): CustomRow => {
  const binding = classifyCustomShortcutAction(entry.action);
  return {
    id,
    key: entry.key,
    action: entry.action,
    command: binding.kind === "command" ? binding.value : "",
    commandMode: binding.kind === "command",
  };
};

const rowToEntry = (row: CustomRow): CustomShortcut => ({ key: row.key, action: row.action });

/** A row earns its place on disk only when both halves are configured. */
const rowIsComplete = (row: CustomRow): boolean =>
  Boolean(row.key.trim() && row.action.trim());

/** R56 · the custom global shortcuts section.
 *
 *  Each row binds one OS key to one launcher action. The action picker lists
 *  the plugins (opened normally) and the app's own actions; a row whose action
 *  is a free-form command line keeps a text field. Recording reuses
 *  `ShortcutRecorder`, and the global shortcuts are suspended for the duration
 *  exactly as they are for the built-in map.
 *
 *  R56 · the key and the action are independently editable. "Add" drops a
 *  draft row straight away (no key, the first picker action), so the action can
 *  be chosen before the key or the key recorded before the action; a draft with
 *  no key is neither registered nor conflict-checked, and is dropped from the
 *  persisted list until it is complete. */
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
  const idRef = useRef(0);
  const [rows, setRows] = useState<CustomRow[]>(() =>
    customShortcuts.map((entry) => rowFromEntry(entry, ++idRef.current)),
  );
  // The last complete list this component handed to the backend, so an echo of
  // its own write does not clobber the local drafts.
  const committedRef = useRef(JSON.stringify(normalizeCustomShortcuts(customShortcuts)));
  const [recording, setRecording] = useState<number | null>(null);
  const [localRejection, setLocalRejection] = useState<{ id: number; key: string } | null>(null);

  // A recording that is abandoned by navigating away must not leave the global
  // shortcuts suspended, or the window toggle would be dead until restart.
  useEffect(
    () => () => {
      invoke("resume_shortcuts").catch(() => undefined);
    },
    [],
  );

  // Adopt an external change (the first hydration, or a list written elsewhere)
  // without discarding the local drafts: only the persisted rows are replaced,
  // and only when the snapshot is not the one this component just sent.
  useEffect(() => {
    const serialized = JSON.stringify(normalizeCustomShortcuts(customShortcuts));
    if (serialized === committedRef.current) return;
    committedRef.current = serialized;
    setRows((current) => {
      const drafts = current.filter((row) => !rowIsComplete(row));
      return [
        ...customShortcuts.map((entry) => rowFromEntry(entry, ++idRef.current)),
        ...drafts,
      ];
    });
  }, [customShortcuts]);

  const rejectionFor = (key: string) =>
    key.trim() ? customRejections.find((entry) => customShortcutKeysEqual(entry.key, key)) : undefined;

  /** Persist the complete rows. Incomplete drafts stay in the UI; a round that
   *  changes nothing complete (adding or removing a draft) skips the backend
   *  round-trip entirely. */
  const commit = (next: CustomRow[]) => {
    const complete = normalizeCustomShortcuts(next.filter(rowIsComplete).map(rowToEntry));
    const serialized = JSON.stringify(complete);
    if (serialized === committedRef.current) return;
    committedRef.current = serialized;
    void onCommitCustomShortcuts(complete);
  };

  const update = (next: CustomRow[]) => {
    setRows(next);
    commit(next);
  };

  const addDraft = () => {
    setLocalRejection(null);
    setRows((current) => [
      ...current,
      {
        id: ++idRef.current,
        key: "",
        action: CUSTOM_SHORTCUT_PICKER[0].action,
        command: "",
        commandMode: false,
      },
    ]);
  };

  const beginRecording = (id: number) => {
    setLocalRejection(null);
    setRecording(id);
    invoke("suspend_shortcuts").catch(() => undefined);
  };

  const finishRecording = () => {
    setRecording(null);
    invoke("resume_shortcuts").catch(() => undefined);
  };

  const capture = (id: number, next: string) => {
    finishRecording();
    const index = rows.findIndex((row) => row.id === id);
    const duplicate = duplicateCustomShortcutKey(next, rows.map((row) => row.key), index);
    if (duplicate) {
      setLocalRejection({ id, key: next });
      return;
    }
    setLocalRejection(null);
    update(rows.map((row) => (row.id === id ? { ...row, key: next } : row)));
  };

  const setAction = (id: number, value: string) => {
    update(
      rows.map((row) => {
        if (row.id !== id) return row;
        if (value === "custom") {
          // Switching to a command line retires the old action: the row is a
          // draft again until a command is typed.
          return { ...row, commandMode: true, action: "", command: "" };
        }
        return { ...row, commandMode: false, action: value, command: "" };
      }),
    );
  };

  const setCommandDraft = (id: number, value: string) => {
    setRows((current) =>
      current.map((row) => (row.id === id ? { ...row, command: value } : row)),
    );
  };

  const commitCommand = (id: number) => {
    const row = rows.find((entry) => entry.id === id);
    if (!row) return;
    const next = row.command.trim();
    if (!next || next === row.action) return;
    update(
      rows.map((entry) =>
        entry.id === id ? { ...entry, action: next, command: next } : entry,
      ),
    );
  };

  const remove = (id: number) => {
    update(rows.filter((row) => row.id !== id));
  };

  return (
    <section className="settings-section">
      <div className="settings-section__heading">
        <div className="settings-section__heading-main">
          <h2 className="settings-section__label">{t("settings.group.customShortcuts")}</h2>
        </div>
        <SettingsAction
          title={t("settings.customShortcutsAddHint")}
          onClick={addDraft}
        >
          <span className="settings-reset__icon" aria-hidden="true">＋</span>
          {t("settings.customShortcutsAdd")}
        </SettingsAction>
      </div>
      <SettingsCard label={t("settings.group.customShortcuts")}>
        {rows.length === 0 ? (
          <SettingsEmpty
            icon={<Keyboard size={22} strokeWidth={1.6} aria-hidden="true" />}
            title={t("settings.customShortcutsEmpty")}
            hint={t("settings.customShortcutsEmptyHint")}
            action={
              <SettingsAction
                title={t("settings.customShortcutsAddHint")}
                onClick={addDraft}
              >
                <span className="settings-reset__icon" aria-hidden="true">＋</span>
                {t("settings.customShortcutsAddFirst")}
              </SettingsAction>
            }
          />
        ) : (
          rows.map((row) => {
            const complete = rowIsComplete(row);
            const rejected =
              rejectionFor(row.key) ??
              (localRejection?.id === row.id
                ? { key: localRejection.key, reason: "conflict" }
                : undefined);
            const pickerValue = row.commandMode ? "custom" : row.action;
            return (
              <SettingsRow
                key={row.id}
                className={complete ? undefined : "settings-row--muted"}
                label={
                  <span className="custom-shortcut__key">
                    <ShortcutRecorder
                      action={`custom:${row.id}`}
                      shortcut={row.key}
                      recording={recording === row.id}
                      onToggle={() => beginRecording(row.id)}
                      onCapture={(_action, next) => capture(row.id, next)}
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
                      onClick={() => remove(row.id)}
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
                      onChange={(event) => setAction(row.id, event.currentTarget.value)}
                    >
                      {CUSTOM_SHORTCUT_PICKER.map((option) => (
                        <option key={option.action} value={option.action}>
                          {t(option.labelKey)}
                        </option>
                      ))}
                      <option value="custom">{t("settings.customShortcutsCustom")}</option>
                    </select>
                    {row.commandMode && (
                      <input
                        type="text"
                        className="settings-select custom-shortcut__command"
                        value={row.command}
                        placeholder={t("settings.customShortcutsCommandPlaceholder")}
                        aria-label={t("settings.customShortcutsCommand")}
                        disabled={busy}
                        spellCheck={false}
                        onChange={(event) => setCommandDraft(row.id, event.currentTarget.value)}
                        onBlur={() => commitCommand(row.id)}
                        onKeyDown={(event) => {
                          if (event.key === "Enter") {
                            event.preventDefault();
                            commitCommand(row.id);
                          }
                        }}
                      />
                    )}
                  </span>
                }
              />
            );
          })
        )}
      </SettingsCard>
      <p className="settings-section__hint">{t("settings.customShortcutsHint")}</p>
    </section>
  );
}

/** The shortcuts settings page: one recorder per action, all driven by the one
 *  `SHORTCUT_ACTIONS` registry, plus the custom global shortcut section. All
 *  state lives in `App` and arrives through props.
 *
 *  R56 · the built-in group comes first and the custom group second, so the
 *  page reads "what floter ships, then what you add" — the order every other
 *  settings surface uses. The clipboard panel's global trigger is gone with
 *  its row: the panel is reached through launcher search and `floter clip`, so
 *  no hidden binding is left behind.
 *
 *  The group title row carries "Restore defaults" as the reference pane's
 *  right-aligned blue text action, and the group's explanation sits below the
 *  card as a footnote — the two positions the reference uses; the custom
 *  group's heading carries its own "Add" in the same slot and language. */
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
