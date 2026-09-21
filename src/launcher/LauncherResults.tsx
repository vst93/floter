import { Fragment } from "react";
import type { Translate } from "../i18n";
import type { LocalApplication } from "../App";
import { formatResultShortcut } from "../shortcuts";
import { resultRowContent } from "./row-content";
import {
  Terminal as TerminalIcon,
  History as HistoryIcon,
  File as FileIcon,
  Folder as FolderIcon,
} from "lucide-react";import type { ActionBarKind, ExecutionPlan } from "../launcher";
import type { DroppedFile } from "./file-drops";

export type SystemAction = "restart" | "shutdown" | "clipboard";

/** Command-row warnings kept out of the subtitle string: they are rendered as
 *  an always-visible dot with the text as tooltip, so a narrow window can
 *  never truncate the warning into oblivion. */
export type CommandWarning = "unavailable" | "conflict";

export type LauncherItem =
  | { type: "app"; id: string; title: string; subtitle: string; app: LocalApplication }
  | {
      type: "command";
      id: string;
      title: string;
      subtitle: string;
      warnings: CommandWarning[];
      sourceName: string;
      commandLine: string;
      execution: ExecutionPlan | null;
      completion: boolean;
    }
  | { type: "system"; id: string; title: string; subtitle: string; action: SystemAction }
  /**
   * A previously typed command line, surfaced in the empty-query state so the
   * user can recall a recent command with a click or Enter. Not a result and
   * not a catalog entry — it has no execution plan, only the text to put
   * back into the input.
   */
  | { type: "history"; id: string; title: string; commandLine: string }
  /**
   * A file dropped onto the launcher (R7-10a). A *result*, not an execution:
   * the row carries the normalized description and nothing runs until the user
   * presses Enter on it or on the action bar beneath it.
   */
  | { type: "file"; id: string; title: string; subtitle: string; file: DroppedFile }
  /**
   * The "…and N more" row of a drop longer than the visible limit. Running it
   * expands the file rows; it never touches a file.
   */
  | { type: "file-more"; id: string; hidden: number; title: string; subtitle: string };

export type ActionBar = { type: ActionBarKind; label: string; value: string };

/** Lucide `rotate-cw` for restart, `power` for shutdown, `clipboard` for the
 *  clipboard panel. */
const SystemActionIcon = ({ action }: { action: SystemAction }) => (
  <svg
    viewBox="0 0 24 24"
    width="16"
    height="16"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
  >
    {action === "restart" ? (
      <>
        <path d="M21 12a9 9 0 1 1-2.64-6.36" />
        <path d="M21 3v6h-6" />
      </>
    ) : action === "shutdown" ? (
      <>
        <path d="M12 2v10" />
        <path d="M18.4 6.6a9 9 0 1 1-12.77.04" />
      </>
    ) : (
      <>
        <rect width="8" height="4" x="8" y="2" rx="1" ry="1" />
        <path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2" />
      </>
    )}
  </svg>
);

/**
 * The action bar's icon: Lucide `terminal` for a shell, `external-link` for a
 * URL, `folder` for a path, and for R7-10a's three file actions `file`/`folder`
 * (open), `terminal` (cd) and `clipboard` (copy path).
 */
const ActionBarIcon = ({ kind }: { kind: ActionBarKind }) => {
  if (kind === "shell") return <TerminalIcon size={16} />;
  // A dropped file's actions reuse the same glyphs as the kinds they mean: an
  // open is an open, a cd is a terminal, a copy is the clipboard.
  if (kind === "file-cd") return <TerminalIcon size={16} />;
  if (kind === "file-copy") return <SystemActionIcon action="clipboard" />;
  if (kind === "file-open") return <FolderIcon size={16} />;
  return (
    <svg
      viewBox="0 0 24 24"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {kind === "url" ? (
        <>
          <path d="M15 3h6v6" />
          <path d="M10 14 21 3" />
          <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
        </>
      ) : (
        <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
      )}
    </svg>
  );
};

type LauncherResultsProps = {
  t: Translate;
  results: LauncherItem[];
  actionBar: ActionBar | null;
  appIconUrls: Record<string, string>;
  selectedResultIndex: number;
  selectedActionBar: boolean;
  resultShortcutSlots: Array<number | null>;
  /** Rendered `select_result` shortcut, shown on the action bar row. */
  actionBarShortcut: string;
  /** Rendered `select_result` shortcut, shown on numbered result rows. */
  selectResultShortcut: string;
  /** Whether the recent-items heading goes above the list (empty query). */
  showRecentTitle: boolean;
  onSelectResult: (index: number) => void;
  onSelectActionBar: () => void;
  onRunResult: (item: LauncherItem) => void;
  onRunActionBar: () => void;
};

/** The launcher's result list plus the action bar row beneath it. Pure
 * presentation: selection state and execution stay in `App`. */
export function LauncherResults({
  t,
  results,
  actionBar,
  appIconUrls,
  selectedResultIndex,
  selectedActionBar,
  resultShortcutSlots,
  actionBarShortcut,
  selectResultShortcut,
  showRecentTitle,
  onSelectResult,
  onSelectActionBar,
  onRunResult,
  onRunActionBar,
}: LauncherResultsProps) {
  // The container stays mounted even with nothing to show. Returning `null`
  // here used to unmount and rebuild every row on the keystroke that emptied
  // or refilled the list, which is a layout and paint of the whole subtree at
  // exactly the moment the window is being resized. Empty, it is a zero-height
  // grid; the enclosing clip wrapper is what hides the bottom area, and it
  // takes this out of the accessibility tree with it.
  return (
    <div id="launcher-options" className="launcher-options" role="listbox" aria-label={t("launcher.results")}>
      {results.length > 0 && (
        <div className="launcher-results" role="presentation">
          {showRecentTitle && (
            <div
              className="launcher-section-title"
              role="presentation"
              title={t("launcher.recentHint")}
            >
              {t("launcher.recent")}
            </div>
          )}
          {results.map((item, index) => {
            const selected = !selectedActionBar && index === selectedResultIndex;
            const unavailable = item.type === "command" && !item.execution;
            const warnings = item.type === "command" ? item.warnings : [];
            const isHistory = item.type === "history";
            // R7-10a: the dropped-file group. Both of its row kinds count, so
            // the heading sits above the first file and the expander does not
            // look like the start of a second group.
            const isFileGroup = item.type === "file" || item.type === "file-more";
            // R12: which of the two optional strings this row actually earns.
            // An application drops the right-hand type word entirely, and any
            // row whose subtitle is only its type word drops that too, so the
            // row collapses to one line (see `row-content.ts`).
            const { source, subtitle } = resultRowContent(item, t);
            const compact = subtitle === null;
            const shortcutSlot = resultShortcutSlots[index];
            // The empty-query state stacks two sections inside a single result
            // list: recents first, then the last few typed commands. The first
            // history row gets the section title; later rows flow under it
            // without their own heading.
            const historySectionStartsHere =
              isHistory && (index === 0 || results[index - 1].type !== "history");
            // A drop prepends one section of its own. Only when it is the first
            // row, so a future composition that puts recents above the drop
            // does not print two headings in a row.
            const filesSectionStartsHere =
              isFileGroup && (index === 0 || !["file", "file-more"].includes(results[index - 1].type));
            return (
              <Fragment key={item.id}>
                {historySectionStartsHere && (
                  <div
                    className="launcher-section-title"
                    role="presentation"
                    title={t("launcher.historyHint")}
                  >
                    {t("launcher.history")}
                  </div>
                )}
                {filesSectionStartsHere && (
                  <div
                    className="launcher-section-title"
                    role="presentation"
                    title={t("launcher.filesHint")}
                  >
                    {t("launcher.files")}
                  </div>
                )}
                <button
                  id={`launcher-option-${index}`}
                  type="button"
                  className={`launcher-result${selected ? " launcher-result--selected" : ""}${
                    unavailable ? " launcher-result--unavailable" : ""
                  }${isHistory ? " launcher-result--history" : ""}${
                    compact ? " launcher-result--compact" : ""
                  }`}
                  role="option"
                  aria-selected={selected}
                  aria-disabled={unavailable}
                  tabIndex={-1}
                  onMouseMove={() => {
                    if (unavailable) return;
                    onSelectResult(index);
                  }}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => onRunResult(item)}
                >
                  <span className={`launcher-result__icon launcher-result__icon--${item.type}`}>
                    {item.type === "app" && appIconUrls[item.app.path] ? (
                      <img src={appIconUrls[item.app.path]} alt="" />
                    ) : item.type === "system" ? (
                      <SystemActionIcon action={item.action} />
                    ) : isHistory ? (
                      <HistoryIcon />
                    ) : item.type === "file" ? (
                      // A folder is a file too, and only the glyph differs.
                      item.file.isDirectory ? <FolderIcon /> : <FileIcon />
                    ) : item.type === "file-more" ? (
                      <FileIcon />
                    ) : (
                      <TerminalIcon />
                    )}
                  </span>
                  <span className="launcher-result__main">
                    <span className="launcher-result__title">{item.title}</span>
                    {subtitle !== null && (
                      <span className="launcher-result__subtitle">{subtitle}</span>
                    )}
                  </span>
                  {warnings.map((warning) => (
                    <span
                      key={warning}
                      className="launcher-result__warning"
                      title={t(warning === "unavailable"
                        ? "extensions.runtimeUnavailable"
                        : "extensions.conflict")}
                    />
                  ))}
                  {source !== null && (
                    <span className="launcher-result__source" title={source}>
                      {source}
                    </span>
                  )}
                  <span className="launcher-result__action">
                    {shortcutSlot === null
                      ? ""
                      : formatResultShortcut(selectResultShortcut, shortcutSlot)}
                  </span>
                </button>
              </Fragment>
            );
          })}
        </div>
      )}
      {actionBar && (
        <button
          id="launcher-option-action"
          type="button"
          className={`launcher-action-bar launcher-action-bar--${actionBar.type}${
            selectedActionBar ? " launcher-action-bar--selected" : ""
          }`}
          role="option"
          aria-selected={selectedActionBar}
          // The label is the action's own words, not a fixed "run in shell": a
          // dropped file's bar offers Open / cd / Copy path, and the assistive
          // label has to be the action the bar is actually showing.
          aria-label={actionBar.label}
          tabIndex={-1}
          onMouseMove={() => onSelectActionBar()}
          onMouseDown={(event) => event.preventDefault()}
          onClick={onRunActionBar}
        >
          <span className="launcher-action-bar__icon">
            <ActionBarIcon kind={actionBar.type} />
          </span>
          <span className="launcher-action-bar__main">
            <span className="launcher-action-bar__title">{actionBar.value}</span>
            <span className="launcher-action-bar__subtitle">{actionBar.label}</span>
          </span>
          <span className="launcher-action-bar__hint">
            {actionBarShortcut}
          </span>
        </button>
      )}
    </div>
  );
}
