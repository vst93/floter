import type { Translate, MessageKey } from "../i18n";
import type { LocalApplication } from "../App";
import { IS_MAC, formatResultShortcut } from "../shortcuts";
import type { ActionBarKind, ExecutionPlan } from "../launcher";

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
  | { type: "system"; id: string; title: string; subtitle: string; action: SystemAction };

export type ActionBar = { type: ActionBarKind; label: string; value: string };

// Where an application came from, read off the shape of its path: `.app`
// bundles on macOS, `.desktop` entries on Linux, Start Menu shortcuts on
// Windows.
export const appSubtitleKey = (path: string): MessageKey => {
  if (IS_MAC) {
    if (path.startsWith("/Applications/")) return "launcher.application";
    if (path.startsWith("/System/Applications/")) return "launcher.systemApplication";
    if (path.includes("/Applications/")) return "launcher.userApplication";
    return "launcher.application";
  }
  if (/^([A-Za-z]:)?[\\/]Users[\\/]/.test(path)) return "launcher.userApplication";
  if (path.startsWith("/home/") || path.startsWith("/root/")) return "launcher.userApplication";
  if (/^\/(usr|opt|var)\//.test(path)) return "launcher.systemApplication";
  return "launcher.application";
};

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
 * The action bar's icon: Lucide `external-link` for a URL, `folder` for a path,
 * and a shell prompt for everything else.
 *
 * The `$` is a glyph rather than Lucide's `terminal` because it is what the row
 * below it in the terminal will actually say, and it reads as "a command line"
 * to anyone who has ever seen one.
 */
const ActionBarIcon = ({ kind }: { kind: ActionBarKind }) => {
  if (kind === "shell") return <span>$</span>;
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
            const source = item.type === "command"
              ? item.sourceName
              : item.type === "app"
                ? t(appSubtitleKey(item.app.path))
                : t("extensions.builtIn");
            const shortcutSlot = resultShortcutSlots[index];
            return (
              <button
                id={`launcher-option-${index}`}
                key={item.id}
                type="button"
                className={`launcher-result${selected ? " launcher-result--selected" : ""}${
                  unavailable ? " launcher-result--unavailable" : ""
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
                  ) : (
                    // The placeholder for an application whose icon has
                    // not resolved yet: a first letter over a real icon
                    // reads as a different application rather than as a
                    // pending one.
                    <span>$</span>
                  )}
                </span>
                <span className="launcher-result__main">
                  <span className="launcher-result__title">{item.title}</span>
                  <span className="launcher-result__subtitle">{item.subtitle}</span>
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
                <span className="launcher-result__source" title={source}>
                  {source}
                </span>
                <span className="launcher-result__action">
                  {shortcutSlot === null
                    ? ""
                    : formatResultShortcut(selectResultShortcut, shortcutSlot)}
                </span>
              </button>
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
