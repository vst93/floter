// SETTINGS-APPLE · the grouped-card language, and the two mutation locks.
//
// The user's ask was 「参考苹果设置页面的 ui 风格，再整体优化下各个页面」 with a
// macOS System Settings screenshot, then 「再参考下appstore」 with a Mac App
// Store screenshot. The two references share one structure, and the round
// implements exactly that structure rather than a coat of paint:
//
//   * a group title *outside* the card, above it, with the group's explanation
//     as a footnote *below* it;
//   * a flat grouped card (radius, fill, edge — no filter, no glass: HIG puts
//     Liquid Glass on the functional layer, and the reference card is a plane);
//   * rows inside it — label, optional grey sublabel, a trailing control, and a
//     1px rule between rows that is *inset* to the label's own start and stops
//     short of the right edge;
//   * right-aligned blue *text* actions ("Restore defaults"), which are the App
//     Store's "See All" idiom and spend the accent on text, never on a fill;
//   * a large title with a grey subtitle at the top of each page.
//
// Three things make this a round rather than a restyle, and each is asserted
// below: the primitives are *shared* (every page renders the same two
// components), the language is *structural* (the inset rule and the control
// slot are real elements, not incidental CSS), and the red lines hold (no
// blur on a card, no new accent face, the keyboard contract unchanged).
//
// The mutation locks are at the bottom: they replay the two regressions the
// round is most likely to see — a row that grows its separator back into a
// full-width border, and a row whose control slot is reordered ahead of its
// label — and prove the structural assertions fire on both.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { createTranslator } from "../src/i18n.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripCssComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};

const ruleFor = (css: string, selector: string) => {
  const rule = rules(css).find(({ selector: s }) => s.split(",").map((p) => p.trim()).includes(selector));
  assert.ok(rule, `${selector} must be defined`);
  return rule!.body;
};

const decl = (body: string, property: string) =>
  body.match(new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`))?.[1].trim() ?? null;

const settings = async () => stripCssComments(await read("src/styles/settings.css"));
const rows = async () => await read("src/settings/SettingsRows.tsx");

/** The body of `SettingsRow`, sliced from its `export function` line to the
 *  `return (` of the *next* top-level declaration. A `\n}` shortcut would stop
 *  at the destructuring pattern's own brace, which is what made the first cut
 *  of these predicates vacuous. */
const rowBody = (source: string) => {
  const at = source.indexOf("export function SettingsRow");
  assert.notEqual(at, -1, "SettingsRows.tsx must export SettingsRow");
  const next = source.indexOf("export function ", at + 1);
  return source.slice(at, next === -1 ? source.length : next);
};

// ── 1 · the primitives are shared, and every page renders them ──────────────

test("the row and card primitives exist and are the only row dialect", async () => {
  const source = stripJsComments(await rows());
  for (const symbol of ["export function SettingsCard", "export function SettingsRow", "export function SettingsAction", "export function SettingsScale"]) {
    assert.ok(source.includes(symbol), `SettingsRows.tsx must export ${symbol}`);
  }
  // The primitives are pure presentation: no state, no Tauri call, no store.
  assert.ok(!/useState|useEffect|invoke\(/.test(source), "the primitives hold no state and call nothing");
  // No third-party UI kit: the imports are React's node type and nothing else.
  const imports = [...source.matchAll(/from "([^"]+)"/g)].map((m) => m[1]);
  assert.deepEqual(imports, ["react"], `the primitives must stay hand-written, got ${imports.join(", ")}`);

  // Every settings page renders the shared card; none builds its own.
  const pages = ["GeneralPage", "ShortcutsPage", "SessionsPage", "AboutPage", "DeepLinkRow"];
  for (const page of pages) {
    const file = await read(`src/settings/${page}.tsx`);
    assert.match(file, /from "\.\/SettingsRows"/, `${page} must import the shared primitives`);
    assert.match(stripJsComments(file), /<Settings(Card|Row)/, `${page} must render a shared primitive`);
  }
  // And no page carries a private row/card implementation.
  for (const page of pages) {
    const file = stripJsComments(await read(`src/settings/${page}.tsx`));
    assert.ok(
      !/function SettingsRow|function SettingsCard/.test(file),
      `${page} must not carry a second row/card implementation`,
    );
  }
});

// ── 2 · the card is a plane, and the row's separator is an inset rule ───────

test("the card is a flat grouped plane, never a second sheet of glass", async () => {
  const css = await settings();
  const card = ruleFor(css, ".settings-section__card");
  // The reference's card: one radius from the shared ladder, a fill, an edge.
  assert.match(card, /border-radius:\s*var\(--radius-md\)/, "the card takes the ladder's container step");
  assert.match(card, /background:\s*var\(--glass-control\)/, "the card is the control ladder's plane");
  // The red line: a card that blurred would be a second material inside the
  // one sheet the app allows, and the reference is explicitly not a nested
  // floating layer.
  assert.ok(!/backdrop-filter|filter/.test(card), "the card must not filter the backdrop");
  assert.ok(!/var\(--glass-(tint|float|blur|saturate)/.test(card), "the card is not the frame's material");
  // It is clipped so the first and last rows respect its radius.
  assert.match(card, /overflow:\s*hidden/);
});

test("the row separator is an inset rule drawn as its own element", async () => {
  const css = await settings();
  const divider = ruleFor(css, ".settings-row__divider");
  // Two insets, not a border: the rule starts where the label starts and stops
  // short of the card's own edge — the reference's separator is inset on both
  // sides. A `border-bottom` on the row could only ever be full width.
  assert.match(divider, /position:\s*absolute/, "the rule is its own element, not a border on the row");
  assert.match(divider, /inset-inline:\s*var\(--settings-card-inset\)/, "the rule is inset to the text's start");
  assert.match(divider, /inset-block-end:\s*0/, "the rule sits on the row's own bottom edge");
  assert.match(divider, /height:\s*1px/, "a hairline");
  assert.match(divider, /background:\s*var\(--hairline\)/, "the rule is the shared hairline token");
  // The row that would otherwise grow a border must not: the separator is the
  // divider's job alone.
  const row = ruleFor(css, ".settings-row");
  assert.ok(!/border-bottom|border-top\s*:/.test(row), "the row must not draw its own rule");
  // A card never ends on a rule.
  const last = (await read("src/styles/settings.css")).match(/\.settings-row:last-child \.settings-row__divider\s*\{([^}]*)\}/);
  assert.ok(last, "the last row's divider must be suppressed");
  assert.match(last![1], /display:\s*none/);

  // The inset is the same offset the row's own padding uses, or "inset to the
  // text" would be a claim rather than a fact.
  assert.equal(decl(row, "padding")?.split(/\s+/).pop(), "var(--settings-card-inset)", "the row pads to the card inset");
});

test("a row is label + sublabel + a trailing control, and the control ends the row", async () => {
  const css = await settings();
  const row = ruleFor(css, ".settings-row");
  // The reference's row height band: System Settings is ~44px, the App Store
  // ~52px; 44 is the floor, so a two-line row grows rather than clipping.
  assert.match(row, /min-height:\s*44px/, "the row clears the reference's 44px floor");
  const main = ruleFor(css, ".settings-row__main");
  assert.match(main, /display:\s*grid/, "the label/sublabel stack is a grid, so a sublabel is optional");
  const control = ruleFor(css, ".settings-row__control");
  assert.match(control, /margin-inline-start:\s*auto/, "the control is pushed to the trailing edge");
  // The sublabel is the reference's grey second line, one step quieter than
  // the label — both are ladder steps, not literals.
  const sublabel = ruleFor(css, ".settings-row__sublabel");
  assert.match(sublabel, /color:\s*var\(--text-muted\)/);
  assert.match(sublabel, /font-size:\s*var\(--text-body\)/);
  const label = ruleFor(css, ".settings-row__label");
  assert.match(label, /color:\s*var\(--text-strong\)/);
  assert.match(label, /font-size:\s*var\(--text-emphasis\)/);

  // In the markup the order really is label → control, and the divider is the
  // last child, so a reorder is a structural change rather than a CSS one.
  const source = await rows();
  const body = rowBody(source);
  const order = ["settings-row__main", "settings-row__control", "settings-row__divider"]
    .map((cls) => ({ cls, at: body.indexOf(cls) }));
  for (const { cls, at } of order) assert.notEqual(at, -1, `${cls} must render`);
  assert.deepEqual(
    order.map((o) => o.cls),
    order.slice().sort((a, b) => a.at - b.at).map((o) => o.cls),
    "the row must render label → control → divider, in that order",
  );
});

// ── 3 · the group title lives outside the card, with its footnote below ─────

test("the group title sits outside the card and takes the App Store's title hierarchy", async () => {
  const css = await settings();
  const label = ruleFor(css, ".settings-section__label");
  // Upgraded from the uppercase 10px caption to the reference's mid-title: a
  // caption reads as metadata; the reference labels groups with a title.
  assert.match(label, /font-size:\s*var\(--text-title\)/, "the group title is a title, not a caption");
  assert.match(label, /color:\s*var\(--text-strong\)/);
  assert.ok(!/text-transform:\s*uppercase/.test(label), "the uppercase caption treatment is retired");
  // The page header carries the App Store's large title + grey subtitle.
  const pageTitle = ruleFor(css, ".settings-page__title");
  assert.match(pageTitle, /font-size:\s*var\(--text-display\)/, "the page takes the large title step");
  const pageSub = ruleFor(css, ".settings-page__subtitle");
  assert.match(pageSub, /color:\s*var\(--text-muted\)/, "the page subtitle is grey");
  // The single centred column is what makes the stack read as a settings page.
  const page = ruleFor(css, ".settings-page");
  assert.match(page, /max-width:\s*640px/, "the content column is capped");
  assert.match(page, /margin-inline:\s*auto/, "and centred");
});

test("every page opens on the large title + subtitle header", async () => {
  for (const page of ["GeneralPage", "ShortcutsPage", "SessionsPage", "AboutPage"]) {
    const file = stripJsComments(await read(`src/settings/${page}.tsx`));
    assert.match(file, /className="settings-page"/, `${page} must open the page frame`);
    assert.match(file, /settings-page__title/, `${page} must render the large title`);
    assert.match(file, /settings-page__subtitle/, `${page} must render the subtitle`);
    // The subtitle is a translated line, not a hardcoded English string.
    assert.match(file, /settings\.page\.\w+/, `${page}'s subtitle must come from i18n`);
  }
});

// ── 4 · the blue text action, and the accent budget ──────────────────────────

test("the right-aligned action is accent text, never an accent fill", async () => {
  const css = await settings();
  const action = ruleFor(css, ".settings-reset");
  assert.match(action, /color:\s*var\(--accent\)/, "the action's accent lives in its text");
  assert.match(action, /background:\s*transparent/, "a text action paints no face");
  assert.ok(!/box-shadow/.test(action), "a text action casts nothing");
  // The budget census counts a *solid accent fill*; this rule must not be one,
  // which is what keeps the App Store idiom off the two-face budget.
  assert.ok(!/background[^:]*:[^;]*var\(--accent/.test(action), "no accent fill on the action");
});

// ── 5 · the red lines ───────────────────────────────────────────────────────

test("no settings card or row carries a filter (the one-sheet rule)", async () => {
  const css = await settings();
  for (const selector of [".settings-section__card", ".settings-row", ".settings-page"]) {
    const body = ruleFor(css, selector);
    assert.ok(!/backdrop-filter|filter\s*:/.test(body), `${selector} must not filter`);
  }
});

test("the sidebar selection keeps the keyboard contract and takes the accent on its icon", async () => {
  const css = await settings();
  const active = ruleFor(css, ".settings-sidebar__item--active");
  // The App Store's selection: a quiet grey pill, not an accent-tinted pane.
  assert.match(active, /background:\s*var\(--glass-raised-quiet\)/, "the selection is the neutral raised pane");
  assert.ok(!/var\(--glass-raised\)(?!-quiet)/.test(active), "the selection is not an accent tint");
  assert.match(active, /box-shadow:\s*var\(--elev-0\)/, "on the resting rung");
  // The accent survives as the one mark a single glyph can carry.
  assert.match(css, /\.settings-sidebar__item--active svg\s*\{[^}]*color:\s*var\(--accent\)/, "the icon takes the accent");
  // The focus ring and the roving tab stop are untouched by the round.
  assert.match(css, /\.settings-sidebar__item:focus-visible\s*\{[^}]*var\(--focus-ring-width\)/, "the focus ring survives");
  const app = await read("src/App.tsx");
  assert.match(app, /settingsSidebarTabIndex/, "the roving tab stop survives");
});

test("the glass stop is never placed on a card (glass belongs to the function layer)", async () => {
  // The user's reference has a plane card and a glass window behind it. The
  // round must not read "make it look like the reference" as "give the card
  // the liquid-glass effect": the stop stays on the shell, written to <html>.
  const css = await settings();
  for (const selector of [".settings-section__card", ".settings-row"]) {
    const body = ruleFor(css, selector);
    assert.ok(!/glass-intensity|data-glass/.test(body), `${selector} must not consume the glass stop`);
  }
  const app = await read("src/App.tsx");
  assert.match(app, /setAttribute\("data-glass", settings\.glass_step\)/, "the stop still writes <html>");
});

test("no stored settings key or semantics changed", async () => {
  // The round is a visual one. The Rust shape is the contract, so the fields
  // the pages read are asserted by name on both sides of the bridge.
  const rust = await read("src-tauri/src/commands/config.rs");
  for (const field of [
    "glass_step", "main_opacity", "terminal_opacity", "font_size", "font_family",
    "cursor_shape", "launch_at_startup", "hide_on_blur", "show_recent_in_launcher",
    "theme", "language", "clipboard_history_hotkey", "show_menubar_icon",
  ]) {
    assert.match(rust, new RegExp(`\\b${field}\\b`), `the Rust settings shape must keep ${field}`);
  }
  const page = await read("src/settings/GeneralPage.tsx");
  for (const field of [
    "glass_step", "main_opacity", "terminal_opacity", "font_size", "font_family",
    "cursor_shape", "launch_at_startup", "hide_on_blur", "show_recent_in_launcher",
    "theme", "show_menubar_icon",
  ]) {
    assert.match(page, new RegExp(`settings\\.${field}`), `GeneralPage must still read ${field}`);
  }
});

// ── 6 · i18n symmetry for the keys the round added ──────────────────────────

test("every new key is declared in both dictionaries and is real Chinese", () => {
  const keys = [
    "settings.page.general",
    "settings.page.sessions",
    "settings.page.shortcuts",
    "settings.page.integrations",
    "settings.page.about",
    "settings.group.appearance",
    "settings.group.startup",
    "settings.group.transparency",
    "settings.group.shortcuts",
    "settings.group.sessions",
    "settings.group.update",
    "settings.group.link",
    "settings.scale.small",
    "settings.scale.large",
    "settings.transparency.scaleLow",
    "settings.transparency.scaleHigh",
  ] as const;
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  for (const key of keys) {
    const english = en(key);
    const chinese = zh(key);
    assert.ok(english.length > 0, `${key} must have en text`);
    assert.ok(chinese.length > 0, `${key} must have zh text`);
    assert.notEqual(chinese, english, `${key} must be translated, not an English fallback`);
    assert.ok(/[\u4e00-\u9fff]/.test(chinese), `${key} must contain Chinese text`);
  }
});

// ── 7 · the mutation locks ──────────────────────────────────────────────────

/** The predicate the separator test really asserts: the rule is inset on both
 *  sides and the row points it at the card's own inset. A mutation that turns
 *  it back into a full-width `border-bottom` fails this. */
const separatorIsInset = (css: string) => {
  const styles = stripCssComments(css);
  const rule = rules(styles).find(({ selector }) => selector === ".settings-row__divider");
  if (!rule) return false;
  if (!/inset-inline:\s*var\(--settings-card-inset\)/.test(rule.body)) return false;
  const row = rules(styles).find(({ selector }) => selector === ".settings-row");
  // A row that grew its own full-width rule is exactly the regression.
  return Boolean(row) && !/border-(bottom|top)\s*:/.test(row!.body);
};

test("mutation: dropping the divider's inset class goes red", async () => {
  const css = await settings();
  assert.ok(separatorIsInset(css), "the shipped separator must be an inset rule");
  // Replay the regression: the divider loses its element and the row takes a
  // plain full-width border instead.
  const mutated = css
    .replace(/inset-inline:\s*var\(--settings-card-inset\);/, "")
    .replace(/\.settings-row \{([^}]*)\}/, (_m, body: string) => `.settings-row {${body}\n  border-bottom: 1px solid var(--hairline);\n}`);
  assert.notEqual(mutated, css, "the mutation must land");
  assert.ok(!separatorIsInset(mutated), "a full-width row border must fail the inset predicate");
});

/** The predicate the row-order test asserts: the row renders its label stack,
 *  then its control slot, then the divider. */
const rowRendersLabelFirst = (source: string) => {
  const at = source.indexOf("export function SettingsRow");
  if (at === -1) return false;
  const body = rowBody(source);
  const main = body.indexOf("settings-row__main");
  const control = body.indexOf("settings-row__control");
  return main !== -1 && control !== -1 && main < control;
};

test("mutation: a label placed after the control goes red", async () => {
  const source = stripJsComments(await rows());
  assert.ok(rowRendersLabelFirst(source), "the shipped row must render its label before its control");
  // Replay the regression: the control slot is hoisted above the label, so the
  // row would read control-first (a trailing picker becomes a leading one).
  const mainAt = source.indexOf('<span className="settings-row__main">');
  const controlAt = source.indexOf('<span className="settings-row__control">');
  assert.ok(mainAt !== -1 && controlAt !== -1 && mainAt < controlAt, "the shipped order must be label-first");
  const block = source.slice(mainAt, controlAt);
  const mutated = source.slice(0, mainAt) + source.slice(controlAt, controlAt + block.length) + block + source.slice(controlAt + block.length);
  assert.ok(!rowRendersLabelFirst(mutated), "control-before-label must fail the order predicate");
});
