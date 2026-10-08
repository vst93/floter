// R149 · the launcher's first frame stops carrying the surfaces it never
// renders.
//
// R143 measured ~117 KB of the single 677,691 B `index-*.js` as code the
// collapsed launcher never runs: the extension panel + settings pages
// (`app-extensions` / `app-settings`), the detached plugin view
// (`app-pluginwindow`), and the calculator's `expr-eval-fork` evaluator
// (`vendor-expr`). R149 splits them out — `React.lazy` + `Suspense` for the
// panel and the settings pages, a dynamic `import()` at the calculator's entry
// for the evaluator, and a label-gated dynamic `import()` for the detached
// view — and this guard is what keeps them split.
//
// Two mutations must turn it red, and both are exercised in the round's report:
//
//   1. putting `ExtensionsPanel` back behind a static import; and
//   2. lifting the evaluator's dynamic `import()` to module top level.
//
// The tokens this guard scans for are assembled from parts (the same
// self-poisoning shape as `r106` / `r107`), and the last test proves the guard
// does not spell them.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const APP = "src/" + "App.tsx";
const CALCULATOR = "src/" + "calculator.ts";
const MAIN = "src/" + "main.tsx";
const SELF = "tests/r149-first-frame-" + "split.test.ts";

/** The evaluator package, assembled so this guard does not spell it. */
const FORK = "expr" + "-eval-fork";

const escapeRegExp = (value: string) => value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/** The surfaces App.tsx must reach through `React.lazy`, not a static import.
 *
 *  `TerminalAppearanceSettings` is deliberately not here: it lives in
 *  `src/settings/` but is the terminal drawer's body, and the R42 guard in
 *  `terminal-settings.test.ts` pins it as App's static import. It cannot split
 *  anyway — `GeneralPage` statically imports it and the first-frame hooks import
 *  `GeneralPage` — so lazy-ing it would be a boundary with no chunk behind it. */
const LAZY_SURFACES = [
  "./ExtensionsPanel",
  "./settings/GeneralPage",
  "./settings/ShortcutsPage",
  "./settings/SessionsPage",
  "./settings/AboutPage",
];

/** A static *value* import of `specifier` — `import type` is erased and legal. */
const staticValueImport = (source: string, specifier: string) =>
  new RegExp(`^import\\s+(?!type\\s)[^\\n]*from\\s*"${escapeRegExp(specifier)}"`, "m").test(source);

test("App mounts the panel and every settings page through React.lazy", async () => {
  const app = await read(APP);

  for (const specifier of LAZY_SURFACES) {
    // Pulled by a dynamic import inside a lazy() factory...
    assert.match(
      app,
      new RegExp(`lazy\\(\\s*\\(\\)\\s*=>\\s*import\\("${escapeRegExp(specifier)}"\\)`),
      `${specifier} must be mounted through React.lazy`,
    );
    // ...and never by a static value import, which would defeat the split.
    assert.ok(
      !staticValueImport(app, specifier),
      `${specifier} must not be imported statically by App`,
    );
  }

  // The split surfaces render behind the app's own spinner, not a new one.
  const boundaries = (app.match(/<Suspense fallback=\{splitSurfaceFallback\}>/g) ?? []).length;
  assert.ok(boundaries >= 1, "the settings pages must render behind a Suspense boundary");
});

test("the evaluator is pulled by the calculator's entry, never at module top level", async () => {
  const calculator = await read(CALCULATOR);
  const dynamic = new RegExp(`import\\(\\s*["']${escapeRegExp(FORK)}["']\\s*\\)`);

  assert.ok(dynamic.test(calculator), "the calculator must pull the evaluator with a dynamic import()");
  // A top-level value import would put the package straight back on the first
  // frame; only `import type` is allowed to name it.
  assert.ok(
    !staticValueImport(calculator, FORK),
    "the evaluator must not be a top-level value import",
  );

  // The dynamic import lives inside the loader function, not at module top
  // level (a top-level `import()` is eager, not a split).
  const loaderStart = calculator.indexOf("export const loadCalculatorEvaluator");
  assert.ok(loaderStart > -1, "the calculator must expose its evaluator loader");
  const loaderEnd = calculator.indexOf("\n};", loaderStart);
  assert.ok(loaderEnd > loaderStart, "the loader function must have a body");
  const loaderBody = calculator.slice(loaderStart, loaderEnd);
  assert.ok(dynamic.test(loaderBody), "the dynamic import must live inside loadCalculatorEvaluator");
  for (const line of calculator.split("\n")) {
    if (dynamic.test(line)) {
      assert.ok(
        /^\s/.test(line),
        `the dynamic import must be indented inside a function, not at module top level: ${line.trim()}`,
      );
    }
  }

  // Opening the calculator is the entry that warms it.
  const app = await read(APP);
  assert.match(
    app,
    /launcherScope === "calculator"[\s\S]{0,160}loadCalculatorEvaluator\(\)/,
    "App must load the evaluator when the calculator scope opens",
  );
});

test("the detached view is pulled by label, not carried by the main card", async () => {
  const main = await read(MAIN);

  assert.match(
    main,
    /lazy\(\(\)\s*=>\s*import\("\.\/plugin-window\/DetachedPluginApp"\)\)/,
    "DetachedPluginApp must be a dynamic import in main.tsx",
  );
  assert.ok(
    !staticValueImport(main, "./plugin-window/DetachedPluginApp"),
    "DetachedPluginApp must not be imported statically by main.tsx",
  );
  // Still gated on the label, still the same render branch.
  assert.match(main, /isPluginWindowLabel\(label\)/);
  assert.match(main, /<Suspense fallback=\{null\}>/);
});

test("this guard assembles the tokens it scans for, it does not spell them", async () => {
  const self = await read(SELF);
  assert.ok(
    !self.includes(`from "${FORK}"`) && !self.includes(`from '${FORK}'`),
    "the guard must build the evaluator's name from parts",
  );
  assert.ok(
    !staticValueImport(self, "./ExtensionsPanel"),
    "the guard must not itself statically import the panel it splits",
  );
});
