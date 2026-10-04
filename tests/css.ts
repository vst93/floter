// R81 · shared CSS rule parser (phase 1).
//
// R78 §3.3 found 23 suites each carrying their own `rules()` / `decl()`. The
// implementations were *not* semantically identical — comment stripping,
// whitespace collapsing and at-rule handling all differ — so this module only
// absorbs the ones that were byte-for-byte the same. The rest stay in place;
// the R81 report lists every difference. Phase 2 (normalising at-rule and
// brace-walk behaviour) is a design decision and is deliberately not taken
// here.
//
// `rules` walks exactly one brace level: a body may not contain a nested
// brace, so an at-rule block (`@media … { … }`) is skipped and its *inner*
// rules are returned. Comment stripping is the caller's job — `rules` never
// strips, so a sheet with a `/* … */` before a selector parses exactly as
// written unless the caller strips first.

import assert from "node:assert/strict";

export interface CssRule {
  selector: string;
  body: string;
}

/** Remove every CSS block comment, multi-line included. */
export const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

/** Every `selector { body }` rule in a sheet, in order. */
export const rules = (css: string): CssRule[] => {
  const out: CssRule[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};

/** The first value declared for `prop`, or `null` when the body omits it. */
export const decl = (body: string, prop: string) =>
  body.match(new RegExp(`(?:^|;)\\s*${prop}\\s*:\\s*([^;]+)`))?.[1].trim() ?? null;

/** The body of the one rule whose selector group contains `selector`. */
export const ruleFor = (css: string, selector: string) => {
  const rule = rules(css).find(({ selector: s }) =>
    s.split(",").map((p) => p.trim()).includes(selector),
  );
  assert.ok(rule, `${selector} must be defined`);
  return rule!.body;
};
