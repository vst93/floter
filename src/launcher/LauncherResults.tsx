import { Fragment, useCallback, useEffect, useLayoutEffect, useRef } from "react";
import type { Translate } from "../i18n";
import type { LocalApplication } from "../App";
import { formatResultShortcut } from "../shortcuts";
import { resultRowContent } from "./row-content";
import { visibleRowRange, type RowSpan, type VisibleRowRange } from "./result-budget";
import {
  Terminal as TerminalIcon,
  History as HistoryIcon,
  File as FileIcon,
  Folder as FolderIcon,
  Globe as GlobeIcon,
  Info as InfoIcon,
  Bookmark as BookmarkIcon,
  Clock as ClockIcon,
  AppWindow as AppWindowIcon,
} from "lucide-react";import type { ActionBarKind, ExecutionPlan } from "../launcher";
import {
  PLUGIN_LOAD_MORE_THRESHOLD,
  pluginFooterState,
  type PluginPage,
} from "./plugin-mode";
import type { ClipboardEntry } from "../clipboard-history";
import type { DroppedFile } from "./file-drops";

export type SystemAction = "restart" | "shutdown" | "clipboard" | "browser";

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
  | {
      type: "system";
      id: string;
      title: string;
      subtitle: string;
      action: SystemAction;
      /** R26-D · a status row rather than a runnable action (the browser plugin
       *  switched off). Rendered dimmed and skipped by Enter, the numbered
       *  shortcuts and the pointer, like a disabled browser row. */
      disabled?: boolean;
    }
  /**
   * R30 · a plugin's status line — "tabs are unavailable", "nothing copied
   *  yet", "the plugin is off". It is information *about* the list, not an
   *  entry in it, so it is drawn as a muted note in the list's own column
   *  rather than as a result row: no icon plate, no title/subtitle stack, no
   *  `⌘N`, no pointer state, no Enter. The capability layer produces it from a
   *  `PluginRow` whose `kind` is `"status"` (see `launcher/plugin-mode.ts`),
   *  and this variant is the "降级" half of that protocol.
   */
  | { type: "status"; id: string; title: string }
  /**
   * R26-A · a browser bookmark or history row, produced by the launcher's
   * browser result mode. `url` is what Enter opens; `profileKey` says which
   * browser to open it in. A `disabled` row is a status line ("no browser
   * found", "no matches") and is not runnable.
   *
   * R26-B adds the second flavour: a row with `tab` set is a tab the browser
   * has open *right now*, and Enter switches to it rather than opening the URL
   * a second time. The tab identity is carried here rather than looked up from
   * `url`, because the same page can be open in two windows and the two rows
   * must stay distinct.
   */
  | {
      type: "browser";
      id: string;
      title: string;
      subtitle: string;
      url: string;
      profileKey: string;
      /** R31 · which browser list the row came from, so the renderer can mark a
       *  bookmark with a bookmark glyph and a history entry with a clock. A
       *  live-tab row is told apart by `tab` instead, and a row that predates
       *  this field falls back to the shared globe. */
      source?: "bookmark" | "history";
      disabled?: boolean;
      tab?: { browserId: string; windowIndex: number; tabIndex: number };
    }
  /**
   * R27 · a clipboard history row, produced by the launcher's clipboard result
   *  mode (`clip `). Enter copies the entry back to the system clipboard and
   *  closes the launcher — the same act the clipboard panel's own row performs,
   *  reached without leaving the search field. A `disabled` row is a status line
   *  ("nothing copied yet", "the plugin is off") and is not runnable.
   */
  | {
      type: "clipboard";
      id: string;
      title: string;
      subtitle: string;
      entry?: ClipboardEntry;
      disabled?: boolean;
    }
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
    ) : action === "browser" ? (
      <>
        <circle cx="12" cy="12" r="10" />
        <path d="M2 12h20" />
        <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10Z" />
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
 * R31 · the browser list's per-type glyph.
 *
 * Every browser row used to wear the same globe, which told the user "this opens
 * a page" but not *which list* the page came from — and a browser mode is three
 * lists at once (bookmarks, history, the tabs open right now). The row already
 * carries the fact (the plugin tags `source` on the row it merges, and a live
 * tab is told apart by `tab`), so this is the one place that maps the row's type
 * to a glyph:
 *
 *   · bookmark  → Lucide `bookmark`
 *   · history   → Lucide `clock` (a history entry is a page *at a time*, which
 *                 the clock says more directly than the history arrow does)
 *   · live tab  → Lucide `app-window` (the tab is a window the browser already
 *                 has open — the row switches to it rather than opening it)
 *   · no source → the shared `globe`, so a row from a plugin that predates this
 *                 field still gets an icon instead of nothing.
 *
 * Lucide marks its own SVG `aria-hidden` when no accessibility prop is given;
 * the row's title and subtitle already carry the meaning, so the glyph is
 * deliberately silent. The 28u icon column and the muted no-plate treatment are
 * R30's and are untouched — only the glyph inside the column changes.
 */
const BrowserRowIcon = ({
  item,
}: {
  item: Extract<LauncherItem, { type: "browser" }>;
}) => {
  if (item.tab) return <AppWindowIcon size={16} aria-hidden="true" />;
  if (item.source === "bookmark") return <BookmarkIcon size={16} aria-hidden="true" />;
  if (item.source === "history") return <ClockIcon size={16} aria-hidden="true" />;
  return <GlobeIcon size={16} aria-hidden="true" />;
};

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
  /** R28 · whether the rows take the keyboard. The capability layer's list tier:
   *  an `interactive` list is the R26/R27 behaviour, a `display` one keeps its
   *  rows but none of the selection — no highlight, no pointer selection, no
   *  Enter. Defaults to interactive so every non-plugin list is unchanged. */
  interactive?: boolean;
  /** Whether the recent-items heading goes above the list (empty query). */
  showRecentTitle: boolean;
  /** R29 · the list's pagination state, when the plugin pages. `null`/omitted
   *  for every non-plugin list and for a plugin that emits everything at once,
   *  which draws no footer at all. */
  pluginPage?: PluginPage | null;
  /** R29 · a page is in flight: the footer shows its loading line. */
  pluginLoadingMore?: boolean;
  /** R29 · ask the plugin for the next page. Called when the scroller comes
   *  within {@link PLUGIN_LOAD_MORE_THRESHOLD} of the bottom. */
  onLoadMore?: () => void;
  onSelectResult: (index: number) => void;
  onSelectActionBar: () => void;
  onRunResult: (item: LauncherItem) => void;
  onRunActionBar: () => void;
  /** R34 · the scroller's visible rows, reported whenever they change (scroll,
   *  resize, a new result set). App turns this into the numbered `⌘N` slots, so
   *  the badges and the key handler follow the viewport. Omitted by the node
   *  tests, which drive the slot map directly. */
  onVisibleRowsChange?: (range: VisibleRowRange) => void;
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
  interactive = true,
  showRecentTitle,
  pluginPage = null,
  pluginLoadingMore = false,
  onLoadMore,
  onSelectResult,
  onSelectActionBar,
  onRunResult,
  onRunActionBar,
  onVisibleRowsChange,
}: LauncherResultsProps) {
  // R14/R31 · the scroll-edge band is no longer painted on the launcher's
  // list. R14 gated it on the scroller's measured box so a list that already fit
  // would not show a grey stripe under the field; R31 removed the band from this
  // scroller altogether (the user read it as a shadow under the input and asked
  // for a 1px line — see `styles/launcher.css`). The 4px top reservation stays,
  // so the scroller still needs its ref and its measurement for the R30
  // selection-into-view rule below.
  const resultsRef = useRef<HTMLDivElement | null>(null);
  // R29 · the list footer's one state: no footer for a list that does not page,
  // a loading line while a page is in flight, an end line once every row shows,
  // and nothing (the scroll itself is the affordance) while more remain.
  const pluginFooter = pluginFooterState(pluginPage, pluginLoadingMore);

  // R34 · the scroll viewport → the numbered `⌘N` slots. The scroller measures
  // which rows are on screen and reports the half-open index range to App, which
  // turns it into the badges and the key map (see `shortcutSlotsWithFixedTail`).
  // "What you see is what you select": scrolling renumbers the list.
  //
  // The measurement is the only DOM-aware half; the range itself comes from
  // `visibleRowRange` in `result-budget.ts`, so the rule is testable without a
  // DOM. The report is deduplicated against the last one, so an App re-render
  // triggered by a range change cannot loop back into another report.
  const reportedRange = useRef<VisibleRowRange | null>(null);
  const measureVisibleRows = useCallback(() => {
    const list = resultsRef.current;
    if (!list || !onVisibleRowsChange) return;
    const listRect = list.getBoundingClientRect();
    const spans: Array<RowSpan | null> = results.map(() => null);
    list.querySelectorAll<HTMLElement>('button[id^="launcher-option-"]').forEach((row) => {
      const index = Number(row.id.slice("launcher-option-".length));
      if (!Number.isInteger(index) || index < 0 || index >= spans.length) return;
      const rect = row.getBoundingClientRect();
      spans[index] = {
        top: rect.top - listRect.top + list.scrollTop,
        height: rect.height,
      };
    });
    const range = visibleRowRange(spans, list.scrollTop, list.clientHeight);
    const previous = reportedRange.current;
    if (previous && previous.start === range.start && previous.end === range.end) return;
    reportedRange.current = range;
    onVisibleRowsChange(range);
  }, [results, onVisibleRowsChange]);
  // One report per frame, at most: a scroll is a stream of events and the
  // badges only have to keep up with the paint. The fallback keeps the helper
  // drivable outside a WebView (the node suite has no `requestAnimationFrame`).
  const visibleReportFrame = useRef(0);
  const scheduleVisibleRows = useCallback(() => {
    if (visibleReportFrame.current) return;
    const run = () => {
      visibleReportFrame.current = 0;
      measureVisibleRows();
    };
    visibleReportFrame.current =
      typeof requestAnimationFrame === "function"
        ? requestAnimationFrame(run)
        : (setTimeout(run, 16) as unknown as number);
  }, [measureVisibleRows]);
  // Measure once per result set, in the layout pass, so the badges are correct
  // before the frame that first shows the new list paints.
  useLayoutEffect(() => {
    measureVisibleRows();
  }, [measureVisibleRows]);
  // The list's own box can change without a scroll (the window resizing, the
  // interface step changing); re-report when it does.
  useEffect(() => {
    const list = resultsRef.current;
    if (!list || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => scheduleVisibleRows());
    observer.observe(list);
    return () => observer.disconnect();
  }, [scheduleVisibleRows, results.length]);
  useEffect(
    () => () => {
      if (!visibleReportFrame.current) return;
      if (typeof cancelAnimationFrame === "function") cancelAnimationFrame(visibleReportFrame.current);
      else clearTimeout(visibleReportFrame.current);
    },
    [],
  );

  // R30 · keyboard navigation walks the whole *loaded* list, not only the nine
  // rows the scroller's ceiling shows, so the row the selection lands on can be
  // outside the box. Keep it visible by moving the scroller's own `scrollTop` —
  // the same thing `ExtensionsPanel.tsx` does, and for the same reason: a
  // `scrollIntoView` walks up every scrollable ancestor and would drag the
  // launcher window with it.
  //
  // It fires when the *selection* moves, and when the list shrinks (a new query
  // is a new result set: the selection returns to its top, so the scroller goes
  // back there too). It deliberately does **not** fire when the list grows: that
  // is the scroll-to-load-more append, and re-asserting the selected row then
  // would yank the user back to the top of the list they are scrolling through.
  const scrolledToIndex = useRef(-1);
  const previousLength = useRef(results.length);
  useLayoutEffect(() => {
    const list = resultsRef.current;
    const shrank = results.length < previousLength.current;
    previousLength.current = results.length;
    const moved = selectedResultIndex !== scrolledToIndex.current;
    if (!list || selectedActionBar || (!moved && !shrank)) return;
    scrolledToIndex.current = selectedResultIndex;
    const row = list.querySelector<HTMLElement>(`#launcher-option-${selectedResultIndex}`);
    if (!row) return;
    const listRect = list.getBoundingClientRect();
    const rowRect = row.getBoundingClientRect();
    if (rowRect.top < listRect.top) list.scrollTop -= listRect.top - rowRect.top;
    else if (rowRect.bottom > listRect.bottom) list.scrollTop += rowRect.bottom - listRect.bottom;
  }, [selectedResultIndex, selectedActionBar, results]);

  // The container stays mounted even with nothing to show. Returning `null`
  // here used to unmount and rebuild every row on the keystroke that emptied
  // or refilled the list, which is a layout and paint of the whole subtree at
  // exactly the moment the window is being resized. Empty, it is a zero-height
  // grid; the enclosing clip wrapper is what hides the bottom area, and it
  // takes this out of the accessibility tree with it.
  return (
    <div id="launcher-options" className="launcher-options" role="listbox" aria-label={t("launcher.results")}>
      {results.length > 0 && (
        <div
          ref={resultsRef}
          className="launcher-results"
          role="presentation"
          // R29 · the pagination trigger. Only a list that actually has another
          // page (and is not already fetching one) reacts; every other list
          // keeps a plain scroller. The check is the scroller's own geometry, so
          // it stays correct through a window resize without a second budget.
          onScroll={(event) => {
            // R34 · the numbered slots follow the viewport, so every scroll is
            // a re-measure (rAF-throttled).
            scheduleVisibleRows();
            if (!onLoadMore || !pluginPage?.hasMore || pluginLoadingMore) return;
            const node = event.currentTarget;
            const remaining = node.scrollHeight - node.scrollTop - node.clientHeight;
            if (remaining <= PLUGIN_LOAD_MORE_THRESHOLD) onLoadMore();
          }}
        >
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
            // R30 · a status line is not a result row: it prints one muted
            // sentence in the list's own icon column and takes nothing else —
            // no `⌘N` slot, no pointer state, no Enter, and no icon plate. See
            // the `status` variant on `LauncherItem`.
            if (item.type === "status") {
              return (
                <div key={item.id} className="launcher-status" role="presentation">
                  <span className="launcher-status__icon" aria-hidden="true">
                    <InfoIcon size={14} />
                  </span>
                  <span className="launcher-status__title" title={item.title}>
                    {item.title}
                  </span>
                </div>
              );
            }
            const selected = interactive && !selectedActionBar && index === selectedResultIndex;
            const unavailable =
              (item.type === "command" && !item.execution) ||
              (item.type === "system" && item.disabled === true) ||
              (item.type === "browser" && item.disabled === true) ||
              (item.type === "clipboard" && item.disabled === true);
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
            // R26-B · the Tabs group sits below bookmarks and history and gets
            // its own heading; the heading is printed once, above the first tab
            // row, so the group reads as one block rather than three.
            const browserTabsSectionStartsHere =
              item.type === "browser" &&
              Boolean(item.tab) &&
              !(
                index > 0 &&
                results[index - 1].type === "browser" &&
                Boolean((results[index - 1] as Extract<LauncherItem, { type: "browser" }>).tab)
              );
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
                {browserTabsSectionStartsHere && (
                  <div
                    className="launcher-section-title"
                    role="presentation"
                    title={t("browserPage.tabsHint")}
                  >
                    {t("browserPage.tabs")}
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
                  // R21 · the pointer takes the selection over on *entry*, not on
                  // every move. `onMouseMove` re-asserted the hovered row on
                  // every pixel of travel, so a keyboard step was undone by a
                  // mouse resting over the row the selection had just left —
                  // two highlights, one of them stale. Entering a row selects
                  // it; nothing fires while the pointer sits still, so the
                  // keyboard continues from the row the pointer is on and the
                  // two devices share one state.
                  onPointerEnter={() => {
                    if (unavailable || !interactive) return;
                    onSelectResult(index);
                  }}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => {
                    // R28 · a display-only list has nothing to run.
                    if (!interactive) return;
                    onRunResult(item);
                  }}
                >
                  <span className={`launcher-result__icon launcher-result__icon--${item.type}`}>
                    {item.type === "app" && appIconUrls[item.app.path] ? (
                      <img src={appIconUrls[item.app.path]} alt="" />
                    ) : item.type === "system" ? (
                      <SystemActionIcon action={item.action} />
                    ) : item.type === "clipboard" ? (
                      <SystemActionIcon action="clipboard" />
                    ) : item.type === "browser" ? (
                      <BrowserRowIcon item={item} />
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
          {pluginFooter !== null && (
            <div
              className={`launcher-plugin-footer launcher-plugin-footer--${pluginFooter}`}
              role="presentation"
            >
              {pluginFooter === "loading" && (
                <span className="launcher-plugin-footer__spinner" aria-hidden="true" />
              )}
              <span className="launcher-plugin-footer__label">
                {pluginFooter === "loading"
                  ? t("launcher.pluginLoadingMore")
                  : pluginFooter === "end"
                    ? t("launcher.pluginEnd")
                    : ""}
              </span>
            </div>
          )}
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
          // R21 · same rule as a result row: the pointer selects on entry, so
          // the row under the cursor and the row Enter runs are the same row.
          onPointerEnter={() => onSelectActionBar()}
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
