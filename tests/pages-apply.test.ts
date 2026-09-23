// PAGES-APPLY · the SETTINGS-APPLE card/list language, extended to the pages
// that were still outside it.
//
// The user's ask was 「按照苹果设置页面的风格，整体优化下各个页面」. SETTINGS-APPLE
// (e5b682f) built the language for the settings *pages*: a flat grouped card,
// rows of label + grey sublabel + trailing control, an inset separator drawn as
// its own element, a blue text action, group titles outside the card. This
// round spends the same language on the surfaces the earlier round left as
// one-offs, and the failure mode it guards against is the obvious one: a page
// that *copies the look* instead of rendering the primitive, which is how a
// design language silently becomes three dialects.
//
//   1. SessionsPage: the private session-row markup (a marker chip, a name
//      line, an inset `::before` rule) becomes the shared `SettingsRow`; its
//      empty/loading/failure region becomes the shared `SettingsEmpty`, whose
//      numbers are the clipboard page's empty language lifted into a primitive.
//   2. The launcher's group title drops the uppercase 10px caption for the
//      hierarchy SETTINGS-APPLE chose for groups ("a caption reads as
//      metadata; the reference labels groups with a title").
//   3. The integrations page (the fourth settings page) was the one that had
//      never been given the page vocabulary: no large-title header, and its two
//      empty/first-load placeholders were a private `.extensions-empty` block.
//      It now opens on the shared page header and paints `SettingsEmpty`.
//   4. The settings window's own titlebar card is already the glass shell and
//      is left alone — asserted, so "the round missed it" cannot become "a
//      later round re-materialises it".
//
// The mutation locks at the bottom replay the two regressions this round is
// most likely to see: a session row that grows its separator back into a
// full-width border, and the shared primitive being swapped for a local
// look-alike.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

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
const sessionsPage = async () => stripJsComments(await read("src/settings/SessionsPage.tsx"));

/** The source of the page's one `<SettingsRow …>` element: from the opening tag
 *  to the `/>` that closes the row and is followed by the map callback's `);`.
 *  A plain `indexOf("/>")` would stop at the first nested self-closing icon
 *  (`<Play … />`) inside the control slot, which is what made the first cut of
 *  this helper read an empty element. */
const sessionRowElement = (source: string) => {
  const at = source.indexOf("<SettingsRow");
  assert.notEqual(at, -1, "SessionsPage must render the shared SettingsRow");
  const close = /\n\s*\/>\n\s*\);/g;
  close.lastIndex = at;
  const match = close.exec(source);
  assert.ok(match, "the session row must be a self-closing element");
  return source.slice(at, match!.index);
};

// ── 1 · the session list is the shared card + the shared row ────────────────

test("the session list renders the shared card and row primitives", async () => {
  const source = await sessionsPage();
  assert.match(source, /from "\.\/SettingsRows"/, "the page must import the shared primitives");
  assert.match(source, /<SettingsCard\b/, "the list is the shared grouped card");
  assert.match(source, /<SettingsRow\b/, "each session is the shared list row");
  // No page-private row implementation: the marker chip, the text column and
  // the title line are the primitive's job now.
  for (const ghost of [
    "session-manager__marker",
    "session-manager__main",
    "session-manager__name",
    "function SettingsRow",
    "function SettingsCard",
  ]) {
    assert.ok(!source.includes(ghost), `${ghost} is the private implementation this round retired`);
  }
});

test("the row is label-first: glyph, then title, then the trailing actions", async () => {
  const source = await sessionsPage();
  const row = sessionRowElement(source);
  const at = (attr: string) => row.indexOf(`${attr}=`);
  for (const attr of ["icon", "label", "control"]) {
    assert.notEqual(at(attr), -1, `the session row must pass ${attr}`);
  }
  assert.ok(at("icon") < at("label"), "the glyph leads the row (the App Store's icon slot)");
  assert.ok(at("label") < at("control"), "the control slot ends the row; label-first is the contract");
  // The row's title is the session name (falling back to the id), and the grey
  // second line carries the cwd plus the state/size/date footer.
  assert.match(row, /terminal\.sessionTitle/, "the label is the session's own name");
  assert.match(row, /session-manager__cwd/, "the cwd rides the grey sublabel");
  assert.match(row, /session-state/, "the state marker rides the grey sublabel");
});

test("the row's separator is the primitive's inset rule, not the page's own", async () => {
  const css = await settings();
  const row = ruleFor(css, ".session-manager__row");
  // The regression this guards: a private rule on the session row. It was a
  // `::before` element inset by a literal 14px; it is now the shared divider.
  assert.ok(!/border-(bottom|top)\s*:/.test(row), "the session row must not draw a full-width rule");
  assert.ok(!/::before/.test(row), "the session row must not re-implement the separator");
  assert.ok(!/inset-inline/.test(row), "the inset belongs to the primitive's divider");
  // …and the primitive's divider is still the inset one it relies on.
  const divider = ruleFor(css, ".settings-row__divider");
  assert.match(divider, /inset-inline:\s*var\(--settings-card-inset\)/, "the shared divider stays inset");
  assert.match(divider, /position:\s*absolute/, "the shared divider stays its own element");
  // The row keeps only its own geometry: three columns and a two-line floor.
  assert.match(row, /grid-template-columns:\s*auto minmax\(0, 1fr\) auto/, "glyph / text / actions");
  assert.match(row, /min-height:\s*56px/, "a session row is two lines of text plus the metadata footer");
});

test("the session row spends no accent fill and keeps the old hit-target contract", async () => {
  const css = await settings();
  const row = ruleFor(css, ".session-manager__row");
  // The accent budget: a row is a plane, not an accent face. The only colour a
  // row may take is the neutral hover wash the primitive already declares.
  assert.ok(!/background[^:]*:[^;]*var\(--accent/.test(row), "no accent fill on a session row");
  assert.ok(!/var\(--glass-raised\)/.test(row), "no accent-tinted raised pane either");
  assert.ok(!/box-shadow/.test(row), "the row still casts nothing of its own");
  // The tappable-row affordance (hover wash + inset focus ring) is the
  // primitive's, declared once for `[role="button"]`, and it must not be
  // restated per page.
  const source = await read("src/styles/settings.css");
  assert.match(
    source,
    /\.settings-row\[role="button"\]:focus-visible\s*\{[^}]*inset 0 0 0 1px var\(--accent\)/,
    "the primitive draws the tappable row's inset focus ring",
  );
  assert.match(
    source,
    /\.settings-row\[role="button"\]:hover\s*\{[^}]*background:\s*var\(--glass-control\)/,
    "and its hover wash",
  );
  // Reduce-motion still neutralizes the one transition the affordance adds.
  // Parsed rather than grepped: the neutralizer is a selector *list*, and a
  // substring match would pass on a comment that merely named the class.
  const base = stripCssComments(await read("src/styles/base.css"));
  const block = base.slice(base.indexOf("@media (prefers-reduced-motion: reduce)"));
  const neutralized = new Set<string>();
  for (const { selector, body } of rules(block)) {
    if (!/(?:^|;)\s*(?:transition|animation)\s*:\s*none/.test(body)) continue;
    for (const part of selector.split(",")) neutralized.add(part.trim());
  }
  assert.ok(
    neutralized.has('.settings-row[role="button"]'),
    "the tappable row's new transition must be neutralized under reduce-motion",
  );
});

// ── 2 · one empty-state language for the whole app ─────────────────────────

test("SessionsPage's empty/loading/failure region is the shared primitive", async () => {
  const source = await sessionsPage();
  assert.match(source, /<SettingsEmpty\b/, "the three states render the shared empty state");
  assert.ok(!source.includes("session-manager__empty"), "the page's private empty region is retired");
  assert.ok(!source.includes("session-manager__empty-hint"), "and its private hint line with it");
  // The three states still say what they said: the failure announcements are
  // the same translated lines, and only the failure is an `alert`.
  assert.match(source, /terminal\.sessionsError/, "the failure copy is unchanged");
  assert.match(source, /terminal\.sessionsLoading/, "the loading copy is unchanged");
  assert.match(source, /terminal\.sessionsEmpty/, "the empty copy is unchanged");
  assert.match(source, /alert=\{error\}/, "only the failure announces itself");

  const css = await settings();
  // The retired selectors are gone from the sheet too, not just the markup.
  for (const ghost of [".session-manager__empty", ".session-manager__empty-hint"]) {
    assert.ok(!css.includes(`${ghost} {`), `${ghost} must be deleted, not orphaned`);
  }
});

test("the shared empty state speaks the clipboard page's empty language", async () => {
  const css = await settings();
  const title = ruleFor(css, ".settings-empty__title");
  const hint = ruleFor(css, ".settings-empty__hint");
  // The clipboard page is where this language was first written down: a title
  // one step brighter than the body, and a quieter hint capped to a 320px
  // measure. The host-side primitive is that language, not a new one.
  const clipboard = stripCssComments(await read("src/plugins/clipboard/page.css"));
  const clipboardTitle = ruleFor(clipboard, ".clipboard-panel__empty-title");
  const clipboardHint = ruleFor(clipboard, ".clipboard-panel__empty-hint");
  assert.equal(decl(title, "color"), decl(clipboardTitle, "color"), "the same title colour");
  assert.equal(decl(title, "font-weight"), decl(clipboardTitle, "font-weight"), "the same title weight");
  assert.equal(decl(hint, "color"), decl(clipboardHint, "color"), "the same hint colour");
  assert.equal(
    decl(hint, "max-width"),
    decl(clipboardHint, "max-width"),
    "the same 320px measure on the hint",
  );
  // Chrome-free: an empty region is a sentence, not a card.
  const empty = ruleFor(css, ".settings-empty");
  assert.ok(!/border|background|box-shadow|border-radius/.test(empty), "the empty region paints no chrome");
  // The integrations list's placeholder is the primitive's own 92px floor —
  // the height the private `.extensions-empty` block pinned, so the list does
  // not jump between the loading and the empty state.
  assert.match(empty, /min-height:\s*92px/, "the list-placeholder floor is preserved");
  // A page whose empty region is its whole body raises the floor in its own
  // scope rather than forking the primitive.
  assert.match(
    css,
    /\.session-manager > \.settings-empty \{[^}]*min-height:\s*160px/,
    "the session page's taller stand-in is a scope override, not a second primitive",
  );
  // The glyph is muted below the sentence it introduces.
  const icon = ruleFor(css, ".settings-empty__icon");
  assert.match(icon, /color:\s*var\(--text-tertiary\)/, "the glyph is quiet");
  assert.match(icon, /opacity:\s*0\.72/, "and muted below the copy");
});

// ── 3 · the launcher's group title joins the title hierarchy ────────────────

test("the launcher group title is a title, not an uppercase caption", async () => {
  const launcher = stripCssComments(await read("src/styles/launcher.css"));
  const title = ruleFor(launcher, ".launcher-section-title");
  // SETTINGS-APPLE retired the uppercase caption on the settings pages; the
  // launcher was the last surface still using it for group titles, so it takes
  // the same hierarchy: sentence case at the body step, muted.
  assert.ok(!/text-transform:\s*uppercase/.test(title), "the uppercase caption is retired here too");
  assert.match(title, /font-size:\s*var\(--text-body\)/, "the group title takes the body step");
  assert.match(title, /color:\s*var\(--text-muted\)/, "and the muted group-title colour");
  assert.ok(!/letter-spacing/.test(title), "the caption's tracking goes with the caption treatment");
  // The heading still cannot reflow the list below it: its box is its own line
  // box plus one padding pair.
  assert.match(title, /line-height:\s*1\.4/, "an explicit line box");
  // R46 · the leading inset is the launcher's 16u text column (the group title
  // sits in the same column as the rows' icon plates; the panel's own 4u plus
  // this 12u), and it is now written in units so the label and the rows it
  // labels stay on one line at every interface step. The pin is the *shape*
  // (its own line box plus one padding pair), not the old literal.
  assert.match(title, /padding:\s*calc\(var\(--u\) \* 6\) calc\(var\(--u\) \* 12\) calc\(var\(--u\) \* 4\)/, "the pinned padding");
  // And the class is still the shared one the launcher's own tests name.
  assert.match(stripJsComments(await read("src/launcher/LauncherResults.tsx")), /launcher-section-title/);
});

// ── 4 · the integrations page joins the page vocabulary ────────────────────

test("the integrations page opens on the shared title header", async () => {
  const source = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // It is a settings page like the other four, so it opens the same way. The
  // settings-apple test asserts this for the four pages in `src/settings/`; the
  // integrations panel lives in `src/` and was the one it could not see.
  assert.match(source, /className="settings-page(?: settings-page--wide)?"/, "the panel opens the page frame");
  assert.match(source, /settings-page__title/, "it renders the large title");
  assert.match(source, /settings-page__subtitle/, "and the grey subtitle");
  // The subtitle is the translated `settings.page.integrations` line the
  // dictionary already carried for exactly this slot.
  assert.match(source, /settings\.page\.integrations/, "the subtitle comes from i18n");
  assert.ok(
    !/settings-section__heading[^"]*"[^>]*>\s*<h1/.test(source),
    "the header is the page header, not the group heading promoted to one",
  );
});

test("the integrations placeholders are the shared empty state", async () => {
  const source = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(source, /import \{[^}]*\bSettingsEmpty\b[^}]*\} from "\.\/settings\/SettingsRows"/, "the panel imports the shared empty primitive");
  assert.match(source, /<SettingsEmpty\b/, "and renders it");
  assert.ok(
    !/className="extensions-empty(?!__)"/.test(source),
    "the private empty *block* is retired (the query echo is a line inside the primitive's hint, not a block)",
  );
  // The two placeholders still say what they said: the first-load spinner and
  // the genuinely-empty list, each with its translated line.
  assert.match(source, /settings\.extensions\.loading/, "the loading copy is unchanged");
  assert.match(source, /settings\.extensions\.emptyInstalled/, "the empty copy is unchanged");
  assert.match(source, /extensions-spinner/, "the first-load placeholder keeps its spinner");
  // The retired selector is gone from the sheet, not orphaned in it.
  const css = stripCssComments(await read("src/styles/extensions.css"));
  assert.ok(!css.includes(".extensions-empty {"), ".extensions-empty must be deleted, not orphaned");
});

test("the integrations section headings are titles, not uppercase captions", async () => {
  const css = stripCssComments(await read("src/styles/extensions.css"));
  const title = ruleFor(css, ".extensions-section-title");
  // Same retirement as the launcher's group title: Connected / Detected /
  // Base plugins are groups inside the settings window, and they now label
  // themselves with a title the way the settings cards do.
  assert.ok(!/text-transform:\s*uppercase/.test(title), "the uppercase caption is retired");
  assert.ok(!/letter-spacing/.test(title), "and its tracking with it");
  assert.match(title, /font-size:\s*var\(--text-body\)/, "the section step is the body step");
  assert.match(title, /color:\s*var\(--text-muted\)/, "titles are muted, not tertiary metadata");
  // The geometry the panel's own tests pin must survive the type change: an
  // 18px heading box and the count chip measured against it.
  assert.match(title, /min-height:\s*18px/, "the heading box stays pinned");
  assert.match(title, /line-height:\s*1\.5/, "with an explicit line box");
  const chip = ruleFor(css, ".extensions-section-title .extension-status");
  assert.match(chip, /min-width:\s*25px/, "the count chip still reserves the spinner's width");
  assert.match(chip, /height:\s*18px/, "and stays pinned to the heading line box");
});

test("every shell header speaks the one title language", async () => {
  // Priority 2 of the round: the shells' title bars. They were already one
  // language — the terminal bar and the plugin host's chrome share the exact
  // same title ramp — so the round's job is to *keep* it that way while the
  // pages move, and to say so. The terminal's canvas, its input semantics and
  // the bar's drag behaviour are untouched: this is a type comparison, not a
  // restyle.
  const terminal = stripCssComments(await read("src/styles/terminal.css"));
  const bar = ruleFor(terminal, ".terminal-bar__title");
  const host = ruleFor(terminal, ".plugin-page-host__topbar-title");
  const ramp = (body: string) =>
    ["color", "font-size", "font-weight", "line-height"]
      .map((property) => decl(body, property))
      .join("|");
  assert.equal(ramp(host), ramp(bar), "the plugin chrome's title is the terminal bar's title");
  // Both are the *secondary* step: a shell title names the window, it is not
  // the page's heading (which is `--text-strong` at the display step).
  assert.equal(decl(bar, "color"), "var(--text-secondary)", "a shell title is secondary text");
  // The plugin bar stays material-free: the shell behind it owns the one blur,
  // so the chrome must not pick up a filter or a fill of its own.
  const topbar = ruleFor(terminal, ".plugin-page-host__topbar");
  assert.ok(!/backdrop-filter|background\s*:/.test(topbar), "the plugin chrome paints no material");
});

// ── 5 · the shell card the round deliberately left alone ───────────────────

test("the settings window card stays the glass shell, not a settings card", async () => {
  // The titlebar card is the *window* (the one sheet of glass), not a grouped
  // card inside it: SETTINGS-APPLE and this round both leave it as the shell.
  // Asserted so a later "consistency" pass cannot restyle it into a plane.
  const css = await settings();
  const shell = ruleFor(css, ".settings-card");
  assert.match(shell, /backdrop-filter:\s*blur\(var\(--glass-blur\)\)/, "the shell keeps the one blur");
  assert.match(shell, /border-radius:\s*var\(--window-radius\)/, "and the window radius");
  // Its header separator is the shared gradient hairline, not a border.
  const header = ruleFor(css, ".settings-card__header");
  assert.ok(!/border-(bottom|top)\s*:/.test(header), "the header draws no border of its own");
  assert.match(css, /\.settings-card__header::after\s*\{[^}]*background:\s*var\(--hairline-fade\)/, "the header's rule is the shared fade");
  // The version chip in the header is not a second card and not an accent face.
  const version = ruleFor(css, ".settings-card__version");
  assert.match(version, /color:\s*var\(--text-tertiary\)/, "the version chip is quiet metadata");
  assert.ok(!/background|border/.test(version), "it is text, not a chip with a face");
});

// ── 6 · semantics and i18n are untouched ───────────────────────────────────

test("no stored key, hotkey or bridge message changed", async () => {
  // The round is a visual one. The session page still speaks the same broker
  // fields and the same command surface; the Rust shape is the contract.
  const source = await sessionsPage();
  for (const field of ["sessionId", "createdAt", "cwd", "name", "exited", "attached", "exitCode", "size", "width", "height"]) {
    assert.match(source, new RegExp(`session\\.${field}`), `SessionsPage must still read session.${field}`);
  }
  // The two-step kill confirmation survives the restyle intact.
  assert.match(source, /data-destructive-confirm/, "the armed kill button keeps its marker");
  assert.match(source, /KILL_CONFIRM_TIMEOUT/, "and its disarm timeout");
  assert.match(source, /SESSIONS_POLL_INTERVAL/, "the visibility-gated refresh poll is unchanged");
  assert.match(source, /Escape/, "Escape still disarms the confirmation");
});

test("the round added no i18n key, so the dictionaries stay symmetric", async () => {
  // The empty states reuse the `terminal.sessions*` lines, the integrations
  // panel its own two, and the header reuses `settings.page.integrations` — the
  // dictionary slot that existed but had no consumer. Nothing new to translate;
  // if a key had been added it would have to be declared twice, and this pins
  // that every key the two pages read is.
  const sources = [
    await sessionsPage(),
    stripJsComments(await read("src/ExtensionsPanel.tsx")),
  ];
  const keys = sources.flatMap((source) =>
    [...source.matchAll(/t\("([a-z][\w.]*)"/g)].map((m) => m[1]),
  );
  assert.ok(keys.length > 0, "the pages still read their copy through i18n");
  const dictionary = await read("src/i18n.ts");
  for (const key of new Set(keys)) {
    const hits = dictionary.match(new RegExp(`"${key}":\\s*"`, "g"))?.length ?? 0;
    assert.equal(hits, 2, `${key} must be declared in both dictionaries, found ${hits}`);
  }
  // And the one slot this round gave its first consumer is really used.
  assert.equal(
    dictionary.match(/"settings\.page\.integrations":\s*"/g)?.length,
    2,
    "settings.page.integrations must stay declared in both dictionaries",
  );
});

// ── 7 · the mutation locks ─────────────────────────────────────────────────

/** The predicate the separator test really asserts: the session row draws no
 *  rule of its own and the shared divider is still the inset one. */
const sessionSeparatorIsShared = (css: string) => {
  const styles = stripCssComments(css);
  const row = rules(styles).find(({ selector }) => selector === ".session-manager__row");
  if (!row) return false;
  if (/border-(bottom|top)\s*:/.test(row.body) || /::before/.test(row.selector)) return false;
  const divider = rules(styles).find(({ selector }) => selector === ".settings-row__divider");
  return Boolean(divider) && /inset-inline:\s*var\(--settings-card-inset\)/.test(divider!.body);
};

test("mutation: a full-width session-row border goes red", async () => {
  const css = await settings();
  assert.ok(sessionSeparatorIsShared(css), "the shipped session row shares the inset rule");
  // Replay the regression: the row takes the rule back as a plain full-width
  // border, and the primitive's divider loses its inset.
  const mutated = css
    .replace(/inset-inline:\s*var\(--settings-card-inset\);/, "")
    .replace(/\.session-manager__row \{([^}]*)\}/, (_m, body: string) => `.session-manager__row {${body}\n  border-bottom: 1px solid var(--hairline);\n}`);
  assert.notEqual(mutated, css, "the mutation must land");
  assert.ok(!sessionSeparatorIsShared(mutated), "a full-width row border must fail the shared-separator predicate");
});

/** The predicate the primitive test asserts: the page imports and renders the
 *  shared row, carries no local row implementation, and keeps no private
 *  session-row markup. */
const sessionRowsUseSharedPrimitive = (source: string) =>
  /import\s*\{[^}]*\bSettingsRow\b[^}]*\}\s*from\s*"\.\/SettingsRows"/.test(source) &&
  /<SettingsRow\b/.test(source) &&
  !/function SettingsRow\b/.test(source) &&
  !/session-manager__marker|session-manager__main|session-manager__name/.test(source);

test("mutation: a local look-alike row goes red", async () => {
  const source = await sessionsPage();
  assert.ok(sessionRowsUseSharedPrimitive(source), "the shipped page renders the shared primitive");
  // Replay the regression: the import is dropped and the page grows its own
  // `SettingsRow` plus the private markup the round retired.
  const mutated = source
    .replace(/import \{[^}]*\} from "\.\/SettingsRows";/, "function SettingsRow() { return null; }")
    .replace("<SettingsRow", "<div className=\"session-manager__marker\"");
  assert.notEqual(mutated, source, "the mutation must land");
  assert.ok(!sessionRowsUseSharedPrimitive(mutated), "a local look-alike must fail the primitive predicate");
});

/** The predicate the label-first test asserts: within the row element the
 *  glyph leads, the label follows, and the control slot ends the row. */
const sessionRowRendersLabelFirst = (source: string) => {
  const at = source.indexOf("<SettingsRow");
  if (at === -1) return false;
  const close = /\n\s*\/>\n\s*\);/g;
  close.lastIndex = at;
  const match = close.exec(source);
  if (!match) return false;
  const row = source.slice(at, match.index);
  const icon = row.indexOf("icon=");
  const label = row.indexOf("label=");
  const control = row.indexOf("control=");
  return icon !== -1 && label !== -1 && control !== -1 && icon < label && label < control;
};

test("mutation: hoisting the control above the label goes red", async () => {
  const source = await sessionsPage();
  assert.ok(sessionRowRendersLabelFirst(source), "the shipped row is label-first");
  // Replay the regression: the trailing action slot is hoisted to the head of
  // the element, so the row would read control-first.
  const mutated = source.replace("<SettingsRow", "<SettingsRow\n                control={null}");
  assert.notEqual(mutated, source, "the mutation must land");
  assert.ok(!sessionRowRendersLabelFirst(mutated), "control-before-label must fail the order predicate");
});

/** The predicate the integrations-placeholder test asserts: the panel renders
 *  the shared empty primitive and keeps no private empty block. */
const integrationsEmptiesAreShared = (source: string) =>
  /import\s*\{[^}]*\bSettingsEmpty\b[^}]*\}\s*from\s*"\.\/settings\/SettingsRows"/.test(source) &&
  /<SettingsEmpty\b/.test(source) &&
  !/className="extensions-empty(?!__)"/.test(source);

test("mutation: a private extensions empty block goes red", async () => {
  const source = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.ok(integrationsEmptiesAreShared(source), "the shipped panel renders the shared empty state");
  // Replay the regression: the import is dropped and the panel paints its own
  // empty block again (the pre-round markup, verbatim).
  const mutated = source
    .replace(/import \{[^}]*\bSettingsEmpty\b[^}]*\} from "\.\/settings\/SettingsRows";/, "")
    .replace("<SettingsEmpty", '<div className="extensions-empty"');
  assert.notEqual(mutated, source, "the mutation must land");
  assert.ok(!integrationsEmptiesAreShared(mutated), "a private empty block must fail the shared predicate");
});
