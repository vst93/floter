// R9-2 · the pure half of the manual-run entry.
//
// The panel is a 2000-line component the node runner does not render, so the
// two decisions that must not drift are projections here: *may this row run?*
// and *how is the duration phrased?* The JSX reads them; the node suite drives
// them directly and reads the JSX facts off the source (the same split
// `script-language-runtime.test.ts` established).

import type { Translate } from "../i18n";

/** The subset of an extension row the run entry depends on. Kept structural
 *  (not `Extension`) so this module has no import cycle with the panel. */
export type RunnableExtension = {
  connected: boolean;
  enabled: boolean;
  state: "enabled" | "disabled" | "broken";
  runtimeAvailable: boolean;
};

/**
 * Whether the row's Run control is live.
 *
 * The run command re-checks every one of these on the backend
 * (`extensions::run::runnable_entry`), so this is a *projection of the same
 * rule* — not a second policy. A row that is disconnected, disabled, broken,
 * or missing its runtime renders the control disabled with a reason rather
 * than hiding it, so the affordance does not flicker between states.
 */
export const runAvailability = (extension: RunnableExtension): boolean =>
  extension.connected
  && extension.enabled
  && extension.state !== "broken"
  && extension.runtimeAvailable;

/**
 * The duration shown in the completion toast. Sub-second runs read in
 * milliseconds ("420 ms"); anything longer rounds to one decimal of a second
 * ("3.4 s"). A non-finite or negative value (a clock that went backwards)
 * degrades to "0 ms" rather than rendering "NaN ms".
 */
export const formatRunDuration = (milliseconds: number): string => {
  if (!Number.isFinite(milliseconds) || milliseconds < 0) return "0 ms";
  if (milliseconds < 1000) return `${Math.round(milliseconds)} ms`;
  return `${(milliseconds / 1000).toFixed(1)} s`;
};

/** Where a run's output goes, given the manifest's declared mode. The backend
 *  is the authority (`extensions_run` returns the route it took); this names
 *  the two states so the drawer's radio and the run-parameter form cannot
 *  disagree about the vocabulary. */
export type OutputMode = "background" | "terminal";

/** The mode a freshly created integration starts in (R9-2 slice 5). The
 *  drawer is the only surface that edits the mode, and this is the value its
 *  form is seeded with — kept here so the default and the two state names are
 *  one declaration rather than a literal repeated in the panel.
 *
 *  Mutation: flip this to `"terminal"` and the default-mode test in
 *  `extension-run-output` goes red. */
export const DEFAULT_OUTPUT_MODE: OutputMode = "background";

export const outputModeLabel = (mode: OutputMode): string =>
  mode === "terminal" ? "terminal" : "background";

/** The part of a captured run output the completion toast summarises. Kept
 *  structural (not the panel's `RunOutput`) so this module keeps no import
 *  cycle with the panel. */
export type CapturedRunOutput = {
  stdout: string;
  stderr: string;
  truncated: boolean;
};

/** Whether a run produced anything at all. Drives the empty state of the
 *  inline output block and whether the completion toast offers a "view
 *  output" action. */
export const hasRunOutput = (output: CapturedRunOutput): boolean =>
  output.stdout.length > 0 || output.stderr.length > 0;

/**
 * How many lines a run's combined output produced.
 *
 * Both streams read in the order they were captured (the same join the inline
 * block renders), a single trailing newline does not invent an extra empty
 * line, and a stream of exactly one newline still counts as one line. An empty
 * capture is zero lines, which is what lets the toast say "0 lines" rather
 * than "1 line" for a silent script.
 */
export const runOutputLineCount = (output: CapturedRunOutput): number => {
  const text = `${output.stdout}${output.stderr}`;
  if (text.length === 0) return 0;
  const withoutTrailingNewline = text.endsWith("\n") ? text.slice(0, -1) : text;
  return withoutTrailingNewline.split("\n").length;
};

/**
 * The output half of the completion toast, localised by the caller's
 * translator. "12 lines" becomes "12 lines · truncated at 64 KB" when either
 * stream hit the retention cap, so the user knows the scroller is showing a
 * prefix rather than the whole thing.
 *
 * One function so the line-count wording cannot drift between the toast and
 * the inline block's own truncation line.
 */
export const runOutputSummary = (output: CapturedRunOutput, t: Translate): string => {
  const lines = runOutputLineCount(output);
  return t(
    output.truncated
      ? "settings.extensions.customRunOutputLinesTruncated"
      : "settings.extensions.customRunOutputLines",
    { lines },
  );
};
