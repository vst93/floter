// R7-10a · File drops → launcher rows.
//
// A file dropped on the launcher becomes *results*, never an execution. That is
// the whole point of the module: `FILE_DROP_ACTIONS` describes three things the
// user can do with a dropped file, `fileDropRows` turns the dropped paths into
// launcher rows, and nothing here runs anything. The rows are offered; the
// action runs only when the user presses Enter on one (see `runLauncherItem` in
// `hooks/useLauncherActions.ts`).
//
// Everything in this file is pure so the node suite can pin the two rules that
// are easy to break silently:
//   * a dropped file never executes on arrival — `fileDropRows` produces rows
//     with no execution plan and no side effect, and `acceptsFileDrop` is the
//     only gate the listener has, and
//   * the list truncates at 5 rows and offers an expander past that.
//
// The three actions are deliberately the *safe* three: hand the path to the
// system's own opener, `cd` a terminal into the containing directory, or put
// the absolute path on the clipboard. None of them runs the file.

import type { ActionBarKind } from "../launcher";
import type { ActionBar, LauncherItem } from "./LauncherResults";
import type { MessageKey, Translate } from "../i18n";

/** The window label the drop listener is allowed to act on. Mirrors
 *  `MAIN_WINDOW_LABEL` in `commands/drops.rs`. */
export const MAIN_WINDOW_LABEL = "main";

/** The normalized description of one dropped path, as the backend reports it. */
export type DroppedFile = {
  /** Absolute, `~`-expanded path. Every action is built on this. */
  path: string;
  /** The file name — the row's title. */
  name: string;
  /** The containing directory — the row's small text. */
  directory: string;
  /** A folder is a file too: it opens in the file manager and `cd`s into itself. */
  isDirectory: boolean;
};

/** What the user can do with a dropped file. */
export type FileDropActionKind = "open" | "cd" | "copy-path";

export type FileDropAction = {
  kind: FileDropActionKind;
  /** The action bar's label for this action. */
  labelKey: MessageKey;
};

/**
 * The three actions, in the order the switcher shows them. "Open" leads because
 * it is what a double-click does and therefore what a user dropping a file most
 * often means; `cd` and copy-path are the two things a launcher can offer that a
 * file manager cannot.
 */
export const FILE_DROP_ACTIONS: readonly FileDropAction[] = [
  { kind: "open", labelKey: "launcher.fileOpen" },
  { kind: "cd", labelKey: "launcher.fileCd" },
  { kind: "copy-path", labelKey: "launcher.fileCopyPath" },
];

/** How many dropped files are listed before the list collapses into an expander. */
export const MAX_VISIBLE_DROPPED_FILES = 5;

/** The group heading the file rows sit under. */
export const FILE_DROP_SOURCE_KEY: MessageKey = "launcher.files";

/**
 * Whether a drop that landed on `windowLabel` while the app was in `mode` may
 * become launcher results.
 *
 * The main window is the only window that owns a launcher; the terminal and
 * settings windows are ignored. In this build both are modes of the same
 * window, so the mode check is the one that actually fires — but the label check
 * is the one that keeps the rule true the moment a second window exists, and it
 * is what the backend command re-checks on the payload.
 */
export const acceptsFileDrop = (windowLabel: string, mode: string): boolean =>
  windowLabel === MAIN_WINDOW_LABEL && mode === "collapsed";

/** The `ActionBarKind` a file action renders as. */
export const actionBarKindFor = (kind: FileDropActionKind): ActionBarKind =>
  kind === "open" ? "file-open" : kind === "cd" ? "file-cd" : "file-copy";

/**
 * The directory a `cd` action moves a terminal to.
 *
 * For a folder, `cd` means *enter it*; for a file, it means *stand next to it*.
 * The same rule a shell's tab-completion teaches.
 */
export const directoryForCd = (file: DroppedFile): string =>
  file.isDirectory ? file.path : file.directory;

/**
 * Quote a path for the interactive shell, but only when it needs it.
 *
 * A plain path is left alone so the command the user sees is the command they
 * would have typed. Anything with a space, quote or shell metacharacter gets
 * single quotes, with an embedded single quote closed and reopened the POSIX way
 * (`'it'\''s'`). Windows' `cmd` has no such escape, so there a double-quoted
 * string is used and embedded quotes are dropped rather than guessed at.
 */
export const shellQuote = (value: string, windows = false): string => {
  if (windows) {
    if (!/[\s"&|<>^%]/.test(value)) return value;
    return `"${value.replace(/"/g, "")}"`;
  }
  if (!/[\s'"\\$`&|;<>()*?[\]{}~!#]/.test(value)) return value;
  return `'${value.replace(/'/g, `'\\''`)}'`;
};

/** The command line a `cd` action types into the terminal.
 *
 * Typing it into a fresh interactive shell (rather than passing a cwd to the
 * spawn call) is what reuses the existing session path with no backend change:
 * `ensureTerminalSession` already hands `initialCommand` to the PTY verbatim,
 * so the shell starts in the user's login directory and moves itself. */
export const cdCommandForPath = (directory: string, windows = false): string =>
  `cd ${shellQuote(directory, windows)}`;

/** The command line a `cd` action types into the terminal. */
export const cdCommandLine = (file: DroppedFile, windows = false): string =>
  cdCommandForPath(directoryForCd(file), windows);

/**
 * Turn the dropped files into launcher rows.
 *
 * `expanded` is the state of the "…and N more" row: collapsed shows the first
 * [`MAX_VISIBLE_DROPPED_FILES`], expanded shows them all. The expander row is
 * itself a result, so it is reachable with the arrow keys and the numbered
 * shortcuts like any other row — and running it expands the list rather than
 * doing anything to a file.
 *
 * Order is the drop order: the user picked the files in it.
 */
export const fileDropRows = (
  files: readonly DroppedFile[],
  expanded: boolean,
  t: Translate,
): LauncherItem[] => {
  if (!files.length) return [];
  const visible = expanded ? files : files.slice(0, MAX_VISIBLE_DROPPED_FILES);
  const rows: LauncherItem[] = visible.map((file) => ({
    type: "file",
    // The path is the identity: two drops of the same file are one row, and a
    // re-drop of the same set does not rebuild every row under the pointer.
    id: `drop:${file.path}`,
    title: file.name,
    subtitle: file.directory,
    file,
  }));
  const hidden = files.length - visible.length;
  if (hidden > 0) {
    // The expander is an ordinary result row: reachable with the arrows and the
    // numbered shortcuts, and running it reveals the rest of the drop. It is
    // the only row in the group that is not a file, so it carries no `file`.
    rows.push({
      type: "file-more",
      id: "drop:more",
      hidden,
      title: t("launcher.fileMore", { count: hidden }),
      subtitle: t("launcher.fileMoreHint"),
    });
  }
  return rows;
};

export const clampActionIndex = (index: number): number => {
  const count = FILE_DROP_ACTIONS.length;
  if (!Number.isFinite(index)) return 0;
  return ((Math.trunc(index) % count) + count) % count;
};

/** Move the active action one step, wrapping at both ends. */
export const nextFileActionIndex = (index: number, direction: -1 | 1): number =>
  clampActionIndex(clampActionIndex(index) + direction);

/** The action the switcher is showing, and therefore the one Enter runs. */
export const activeFileDropAction = (index: number): FileDropAction =>
  FILE_DROP_ACTIONS[clampActionIndex(index)];

/** The path the action bar displays and the action acts on. */
export const actionValue = (file: DroppedFile, kind: FileDropActionKind): string =>
  kind === "cd" ? directoryForCd(file) : file.path;

/**
 * The action bar row for a dropped file: the same `ActionBar` the query-driven
 * kinds produce, so the switcher is the existing control and not a new one.
 */
export const fileDropActionBar = (
  file: DroppedFile,
  actionIndex: number,
  t: Translate,
): ActionBar => {
  const action = activeFileDropAction(actionIndex);
  return {
    type: actionBarKindFor(action.kind),
    label: t(action.labelKey),
    value: actionValue(file, action.kind),
  };
};

/**
 * The dropped file the selection is on, or `null`.
 *
 * Resolved off the *row* rather than off the index: the dropped list can be
 * replaced by a re-drop between a render and a key press, and the actions must
 * stay attached to the file the user is looking at.
 */
export const selectedDroppedFile = (
  results: readonly LauncherItem[],
  selectedResultIndex: number,
): DroppedFile | null => {
  const item = results[selectedResultIndex];
  return item?.type === "file" ? item.file : null;
};

/** The action bar's label for a file action, translated. */
export const fileDropActionLabel = (index: number, t: Translate): string =>
  t(activeFileDropAction(index).labelKey);

/**
 * What an action actually asks the host to do.
 *
 * Kept as data so the execution layer can be read against it: `open` hands a
 * path to the system opener, `cd` types a command into a fresh interactive
 * shell, `copy` writes to the clipboard. None of the three runs the file, and
 * there is no fourth shape — a dropped file is never executed.
 */
export type FileActionRequest =
  | { kind: "open"; path: string }
  | { kind: "cd"; commandLine: string }
  | { kind: "copy"; path: string };

/**
 * Build the request for one action.
 *
 * `value` is the string the action bar is showing — the path for open/copy and
 * the directory for `cd` (see [`actionValue`]) — so the row the user is looking
 * at and the thing that runs can never disagree.
 */
export const fileActionRequest = (
  action: FileDropActionKind,
  value: string,
  windows = false,
): FileActionRequest =>
  action === "open"
    ? { kind: "open", path: value }
    : action === "cd"
      ? { kind: "cd", commandLine: cdCommandForPath(value, windows) }
      : { kind: "copy", path: value };

/** The request for the action the switcher is currently showing. */
export const requestForActionIndex = (
  index: number,
  file: DroppedFile,
  windows = false,
): FileActionRequest => {
  const action = activeFileDropAction(index);
  return fileActionRequest(action.kind, actionValue(file, action.kind), windows);
};

/** Whether an action bar is describing a dropped file rather than the query.
 *  Only these three kinds claim Enter on the bar and the ←/→ switcher. */
export const isFileActionKind = (kind: ActionBarKind | undefined): boolean =>
  kind === "file-open" || kind === "file-cd" || kind === "file-copy";

/**
 * The action a file action bar is showing, read back off its kind.
 *
 * The execution path asks the *bar* which action it is, rather than re-deriving
 * it from the switcher index: the row the user is looking at and the thing that
 * runs are then the same value by construction, and cannot drift apart.
 */
export const fileActionKindForBar = (kind: ActionBarKind): FileDropActionKind =>
  kind === "file-open" ? "open" : kind === "file-cd" ? "cd" : "copy-path";

