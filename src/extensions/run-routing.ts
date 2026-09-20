// R9-2 · the pure half of the manual-run entry.
//
// The panel is a 2000-line component the node runner does not render, so the
// two decisions that must not drift are projections here: *may this row run?*
// and *how is the duration phrased?* The JSX reads them; the node suite drives
// them directly and reads the JSX facts off the source (the same split
// `script-language-runtime.test.ts` established).

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
 *  the two states so the row's switch and the drawer's radio cannot disagree
 *  about the vocabulary. */
export type OutputMode = "background" | "terminal";

export const outputModeLabel = (mode: OutputMode): string =>
  mode === "terminal" ? "terminal" : "background";
