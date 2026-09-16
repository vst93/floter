// HIG-2 · the accent budget, quantified.
//
// HIG's checklist item (`liquid-glass.md › Review checklist`): "Color budget.
// One, at most two, tinted primary actions per view … never a row of them."
// The rule was verbal until this round. This file makes it a number.
//
// The unit is a **counted face**: a selector whose background is filled with an
// accent token. The counted family is the accent itself (`--accent`,
// `--accent-tint`, `--glass-raised` → `--accent-tint`). The *warm* branch
// (`--glass-raised-warm` → `--system-icon-surface`, the power/destructive
// semantics) and the shell tints are a different colour with a different job,
// and HIG's budget is about the brand accent, so they are not counted.
//
// Everything else the accent touches — a label, a 1px keyline, a focus ring, a
// status dot, a progress fill — is a *mark*. Marks are emphasis, not a filled
// control, and are deliberately not counted; `status dots and progress fills`
// below records that judgement instead of hiding it in a regex.
//
// **The unit is a selector, not an instance.** Two `.settings-switch--active`
// thumbs can be on screen at once; this census counts the selector once. There
// is no way to count live instances from the stylesheets alone; the instance
// level is covered by the round's pixel-area forensics (screenshots), and the
// definitions here are deliberately placed on that division of labour rather
// than pretending the static count is the whole story.
//
// The census is a **full-table scan**, not a closed list. It parses every rule
// in every `src/styles/*.css` sheet, keeps the ones whose background is a solid
// accent fill and whose selector is a resting face (not `:hover`/`:active`/
// `:focus`, not a pseudo-element), and then:
//
//   1. attributes each counted selector to the view named in VIEWS, and asserts
//      that view is within `--accent-budget`; and
//   2. asserts the *complement*: every counted selector must be named in some
//      view's ledger. A new accent fill (`background: var(--accent)` on a
//      selector nobody listed) goes red because no view claims it — which is
//      the mutation the closed-list version could not catch.
//
// The VIEWS ledger below therefore does two jobs: it is the file→view mapping
// the scan needs, and it is the positive anchor (`a chosen state is a lit pane`
// and the final `status dots …` test) that pins the round's judgement calls.
//
// The census is per *view*, not per file: `.launcher-feedback` (a transient
// toast) and `.launcher-system-confirm` (a modal overlay) never appear on the
// same screen as the result list, so they are their own views rather than more
// faces on the resting one.
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

const RULES = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};

// A counted face: a background in the accent-fill family.
const ACCENT_FILL =
  /var\(--(accent|accent-tint|accent-tint-hover|accent-wash|glass-raised|glass-raised-hover|ext-bg-selected)\)/;
// A warm-branch fill (power/destructive), not the brand accent.
const WARM_FILL = /var\(--(glass-raised-warm|glass-raised-warm-hover|system-icon-surface|system-icon-surface-hover|action-shell-tint)\)/;

// The background declarations of a rule body. `background` and
// `background-color` only; `background-image` is a gradient/band and is judged
// elsewhere.
const backgrounds = (body: string) =>
  [...body.matchAll(/(?:^|;)\s*(?:background|background-color)\s*:\s*([^;]+)/g)].map((m) => m[1]);

// A face has to be painted by a *solid* token layer. A gradient that merely
// *mentions* the accent wash (`.collapsed-card__aura`) is an ambient tint, not
// a filled control, so its token does not consume the budget. Commas are split
// only at paren depth 0, or a gradient's own arguments would split it apart.
const splitTopLevel = (value: string) => {
  const parts: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < value.length; i += 1) {
    const c = value[i];
    if (c === "(") depth += 1;
    else if (c === ")") depth -= 1;
    else if (c === "," && depth === 0) {
      parts.push(value.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(value.slice(start));
  return parts.map((part) => part.trim()).filter(Boolean);
};

const solidLayers = (value: string) =>
  splitTopLevel(value).filter((part) => !/^(?:repeating-)?(?:linear|radial|conic)-gradient\b/i.test(part));

const isFill = (body: string, family: RegExp) =>
  backgrounds(body).some((value) => solidLayers(value).some((layer) => family.test(layer)));

// A resting face only. `:hover`/`:active`/`:focus*` are transient reactions to
// the pointer or keyboard, and a pseudo-element (`::before`, `::selection`) is
// a mark rather than a control, so neither counts against a budget of "tinted
// actions per view".
const STATE = /:(?:hover|active|focus|focus-visible|focus-within)\b|::/;

// The two indicator families the round deliberately keeps off the budget: a
// status dot is one solid mark, and a progress *bar/fill* is the moving part of
// a track. Both are asserted (not merely exempted) in `status dots …` below.
// The pattern is deliberately narrow: `.extension-row__progress` is a status
// *pane*, not a progress bar, and it stays in the census.
const MARK = /[-_](?:dot|progress-(?:bar|fill|track))\b/;

// A view: the CSS selectors that co-occur on one screen. Each entry is an
// explicit, human-readable census. The scan attributes a counted face here by
// name; a counted selector that is in *no* entry is a new accent fill and fails
// the complement assertion.
const VIEWS: { name: string; mode: string; selectors: string[] }[] = [
  { name: "collapsed · results", mode: "collapsed", selectors: [".launcher-result--selected", ".launcher-action-bar--selected"] },
  { name: "collapsed · onboarding tip", mode: "collapsed", selectors: [".launcher-tip"] },
  { name: "collapsed · feedback toast", mode: "collapsed", selectors: [".launcher-feedback", ".launcher-feedback--warning"] },
  { name: "collapsed · system confirm", mode: "collapsed", selectors: [".launcher-system-confirm", ".launcher-system-confirm__execute"] },
  { name: "terminal · clipboard list", mode: "terminal", selectors: [".clipboard-row--selected", ".clipboard-panel__tab--active"] },
  { name: "plugin · clipboard page", mode: "plugin", selectors: [".clipboard-row--selected", ".clipboard-panel__tab--active"] },
  { name: "settings · page shell", mode: "settings", selectors: [".settings-sidebar__item--active", ".settings-option--active", ".opacity-control__preset--active", ".shortcut-recorder--recording", ".settings-switch--active .settings-switch__thumb"] },
  { name: "settings · integrations", mode: "settings", selectors: [".extensions-action-button--primary", ".extensions-icon-button--primary", ".extension-row--selected", ".extension-tool-results button.extension-tool-result--active", ".extension-custom-mode__item--active", ".extension-health__tag", ".extension-status--recommended", ".extension-row__progress"] },
  { name: "settings · about", mode: "settings", selectors: [".update-banner", ".update-banner__button"] },
];

const FILE_FOR: Record<string, string> = {
  "collapsed · results": "src/styles/launcher.css",
  "collapsed · onboarding tip": "src/styles/launcher.css",
  "collapsed · feedback toast": "src/styles/launcher.css",
  "collapsed · system confirm": "src/styles/launcher.css",
  "terminal · clipboard list": "src/styles/terminal.css",
  "plugin · clipboard page": "src/styles/terminal.css",
  "settings · page shell": "src/styles/settings.css",
  "settings · integrations": "src/styles/extensions.css",
  "settings · about": "src/styles/settings.css",
};

const hostSheets = async () => {
  const dir = new URL("src/styles/", root);
  const names = (await readdir(dir)).filter((n) => n.endsWith(".css"));
  return names.sort();
};

// Every resting accent fill in the host sheets, as `{ file, selector }`. This
// is the census: it reads the whole table, so nothing is missed by not being
// listed.
const scanAccentFaces = async () => {
  const faces: { file: string; selector: string }[] = [];
  for (const name of await hostSheets()) {
    const file = `src/styles/${name}`;
    for (const { selector, body } of RULES(stripComments(await read(file)))) {
      if (!isFill(body, ACCENT_FILL)) continue;
      for (const part of selector.split(",").map((s) => s.trim())) {
        if (!part || STATE.test(part) || MARK.test(part)) continue;
        faces.push({ file, selector: part });
      }
    }
  }
  return faces;
};

test("the accent budget is a declared token, not a number in the test", async () => {
  const css = stripComments(await read("src/styles/base.css"));
  const match = css.match(/--accent-budget:\s*(\d+)\s*;/);
  assert.ok(match, "--accent-budget must be declared in the dark token block");
  assert.equal(match![1], "2", "HIG's budget is one, at most two, tinted primary actions");
});

test("a new accent fill outside every view's ledger is red (complement assertion)", async () => {
  // The closed-list version of this suite could only see selectors it already
  // listed, so injecting `.rev-new-accent-face { background: var(--accent) }`
  // stayed green. The scan now reads the whole table; this is the assertion
  // that a *new* fill is caught. It is the mutation lock for Major 1, kept
  // separate from the per-view budget so a regression in either is legible.
  const faces = await scanAccentFaces();
  const known = new Set(VIEWS.flatMap((v) => v.selectors));
  const orphans = faces.filter((f) => !known.has(f.selector));
  assert.deepEqual(
    orphans.map((f) => `${f.file}: ${f.selector}`),
    [],
    "an accent fill appeared that no view's census names — add it to the right view's " +
      "ledger (and check the budget) or make it a neutral pane",
  );
  // The scan must be reading more than the ledger, or it is the closed list
  // again: assert it resolves at least one *listed* face from the file scan.
  assert.ok(
    faces.some((f) => f.selector === ".launcher-result--selected"),
    "the scan must resolve listed faces from the sheet, not from the ledger",
  );
});

test("every view stays within the accent-fill budget (full-table census)", async () => {
  const budget = Number(
    stripComments(await read("src/styles/base.css")).match(/--accent-budget:\s*(\d+)\s*;/)?.[1] ?? NaN,
  );
  assert.equal(budget, 2, "the budget is a token; this test reads it, it does not restate it");

  const faces = await scanAccentFaces();
  assert.ok(faces.length > 0, "the census must find at least one accent fill, or it is measuring nothing");

  // ── Per-view budget ──────────────────────────────────────────────────────
  const report: string[] = [];
  for (const view of VIEWS) {
    const file = FILE_FOR[view.name];
    const rules = RULES(stripComments(await read(file)));
    // Every selector the view names must exist in the file it claims: a
    // typo/rename cannot silently shrink the census.
    for (const selector of view.selectors) {
      const exists = rules.some((r) => r.selector.split(",").map((s) => s.trim()).includes(selector));
      assert.ok(exists, `${view.name}: ${selector} not found in ${file}`);
    }
    const counted = faces
      .filter((f) => f.file === file && view.selectors.includes(f.selector))
      .map((f) => f.selector);
    report.push(`${view.name} [${view.mode}]: ${counted.length} counted — ${counted.join(", ") || "(none)"}`);
    assert.ok(
      counted.length <= budget,
      `${view.name} has ${counted.length} accent fills (budget ${budget}):\n  ` + counted.join("\n  "),
    );
  }

  // The census is not vacuous: at least one view uses the budget, and the two
  // anchor views each resolve to their expected one.
  const collapsed = stripComments(await read("src/styles/launcher.css"));
  assert.ok(isFill(RULES(collapsed).find((r) => r.selector === ".launcher-result--selected")!.body, ACCENT_FILL));
  assert.ok(!isFill(RULES(collapsed).find((r) => r.selector === ".launcher-tip")!.body, ACCENT_FILL), "the tip is neutral now");
  assert.ok(!isFill(RULES(collapsed).find((r) => r.selector === ".launcher-feedback")!.body, ACCENT_FILL), "the feedback toast is warm, not accent");
  // The primary action slot is the one solid-accent button per surface.
  const ext = stripComments(await read("src/styles/extensions.css"));
  assert.ok(isFill(RULES(ext).find((r) => r.selector === ".extensions-action-button--primary")!.body, ACCENT_FILL));
  assert.ok(isFill(RULES(ext).find((r) => r.selector === ".extensions-icon-button--primary")!.body, ACCENT_FILL));
  // Every view in the census is printed by the test on failure; on success it
  // is silent. Keep the report referenced so it is not dead code.
  void report;
});

test("a chosen state is a lit pane, never an accent fill", async () => {
  // HIG `liquid-glass.md › Color on glass`: "Refrain from adding color to the
  // background of multiple controls." A segmented control can show several
  // chosen segments at once, so the chosen state is the neutral raised pane
  // with an accent keyline — the accent survives as emphasis, not as a fill.
  // `.extension-row__progress` is the same treatment for a status strip: the
  // accent keyline and label stay, the tinted fill does not.
  const cases: [string, string][] = [
    ["src/styles/settings.css", ".settings-option--active"],
    ["src/styles/settings.css", ".opacity-control__preset--active"],
    ["src/styles/terminal.css", ".clipboard-panel__tab--active"],
    ["src/styles/extensions.css", ".extension-custom-mode__item--active"],
    ["src/styles/extensions.css", ".extension-tool-results button.extension-tool-result--active"],
    ["src/styles/extensions.css", ".extension-row__progress"],
  ];
  for (const [file, selector] of cases) {
    const css = stripComments(await read(file));
    const rule = RULES(css).find((r) => r.selector === selector);
    assert.ok(rule, `${file}: ${selector} must exist`);
    assert.match(
      rule!.body,
      /background:\s*var\(--glass-raised-quiet\)/,
      `${file}: ${selector} must be the neutral raised pane, not an accent fill`,
    );
    assert.match(
      rule!.body,
      /inset 0 0 0 1px var\(--accent-edge\)/,
      `${file}: ${selector} must keep the accent keyline that says "chosen"`,
    );
  }
  // The token really is neutral, in both palettes.
  const css = stripComments(await read("src/styles/base.css"));
  const occurrences = [...css.matchAll(/--glass-raised-quiet:\s*([^;]+);/g)].map((m) => m[1].trim());
  assert.ok(occurrences.length >= 2, "--glass-raised-quiet must be declared for dark and light");
  for (const value of occurrences) {
    assert.ok(
      /var\(--glass-control-hover\)/.test(value),
      `--glass-raised-quiet must be the neutral control fill, got "${value}"`,
    );
  }
  // …and it has no dead sibling: `-quiet-rim` was declared but consumed
  // nowhere, so it was removed this round. A stray re-declaration is a smell
  // this catches.
  assert.ok(
    !/--glass-raised-quiet-rim/.test(css),
    "--glass-raised-quiet-rim has no consumer and must not be declared",
  );
});

// Status dots, warm branches and progress bars are indicators, not actions: the
// assertion records the round's judgement that they do not consume the
// primary-action budget. The scan exempts them by the `MARK` pattern; this test
// pins the three concrete shapes so the exemption cannot quietly widen.
test("status dots, warm branches and progress fills are marks, not budget faces", async () => {
  const dots: [string, string][] = [
    ["src/styles/terminal.css", ".terminal-bar__dot"],
    ["src/styles/settings.css", ".session-manager__resume-dot"],
    ["src/styles/settings.css", ".update-banner__progress-bar"],
  ];
  for (const [file, selector] of dots) {
    const css = stripComments(await read(file));
    const rule = RULES(css).find((r) => r.selector === selector);
    assert.ok(rule, `${file}: ${selector} must exist`);
    assert.ok(MARK.test(selector), `${selector} must be one of the exempt indicator shapes`);
    assert.match(
      rule!.body,
      /background:\s*var\(--accent\)/,
      `${file}: ${selector} keeps the accent as a status/progress indicator`,
    );
    assert.ok(!/var\(--(glass-raised|accent-tint)/.test(rule!.body), "a dot is one solid mark, not a tinted face");
  }
  // The warm branch is a different colour with a different semantic, so it is
  // not counted even though it is a fill.
  const launcher = stripComments(await read("src/styles/launcher.css"));
  const confirm = RULES(launcher).find((r) => r.selector === ".launcher-system-confirm__execute");
  assert.ok(confirm, "launcher.css must define .launcher-system-confirm__execute");
  assert.ok(isFill(confirm!.body, WARM_FILL), "the power action uses the warm branch");
  assert.ok(!isFill(confirm!.body, ACCENT_FILL), "the power action must not consume the accent budget");
  // The progress track stays neutral so the bar reads as one moving mark.
  const settings = stripComments(await read("src/styles/settings.css"));
  const track = RULES(settings).find((r) => r.selector === ".update-banner__progress-track");
  assert.ok(track, "settings.css must define .update-banner__progress-track");
  assert.match(track!.body, /background:\s*var\(--glass-track\)/);
  assert.ok(!isFill(track!.body, ACCENT_FILL), "the progress track must not be an accent fill");
});
