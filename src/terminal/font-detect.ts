// R42 · which monospace faces this machine actually has.
//
// The picker should not offer a font the user cannot see: a `<select>` full of
// candidates that all fall through to the generic `monospace` is worse than a
// short, honest list. There is no web API for "list the installed fonts", so
// the browser's own fallback is used as the probe — the classic width-diff
// trick:
//
//   * measure a string in a bare generic family (`monospace`, `serif`,
//     `sans-serif`) to get that family's width for the string;
//   * measure the same string with `'<candidate>', <generic>` prepended. If
//     the candidate is installed the width changes; if it is not, the generic
//     answers exactly as before and the width is identical.
//
// A candidate counts as available when *any* generic's width moves, which
// makes the probe robust against a candidate that happens to match one
// generic's metrics. System faces are always resolvable synchronously, so no
// `document.fonts.ready` wait is needed.
//
// The result is memoised: the answer cannot change within a session without a
// font install, and the probe is a handful of `measureText` calls that should
// not run on every panel open.

import { MONO_FONT_CANDIDATES } from "./terminal-appearance";

const PROBE_TEXT = "mmmmmmmmmmlli0O1Il|";
const GENERIC_BASES = ["monospace", "serif", "sans-serif"] as const;

let cache: string[] | null = null;

/** Detect which of the shipped monospace candidates the platform has.
 *
 *  Returns `[]` when there is no DOM/canvas (the node test runner, a headless
 *  build) — `fontFamilyOptions` then falls back to the shipped static list. */
export function detectMonospaceFonts(): string[] {
  if (cache) return cache;
  if (typeof document === "undefined") return (cache = []);
  const canvas = document.createElement("canvas");
  const ctx = canvas.getContext("2d");
  if (!ctx) return (cache = []);

  const baseline = new Map<string, number>();
  for (const base of GENERIC_BASES) {
    ctx.font = `16px ${base}`;
    baseline.set(base, ctx.measureText(PROBE_TEXT).width);
  }

  const detected = MONO_FONT_CANDIDATES.filter((option) => {
    const quoted = `'${option.value.replace(/['\\]/g, "\\$&")}'`;
    return GENERIC_BASES.some((base) => {
      ctx.font = `16px ${quoted},${base}`;
      return ctx.measureText(PROBE_TEXT).width !== baseline.get(base);
    });
  }).map((option) => option.value);

  return (cache = detected);
}

/** Test hook: forget the memoised probe (a font installed mid-session). */
export function resetMonospaceFontCache(): void {
  cache = null;
}
