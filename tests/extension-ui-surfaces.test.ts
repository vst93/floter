import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

// The clipboard history is a shipped surface: its settings toggle defaults to
// on, the panel is reachable from launcher search and `floter clip`, and
// terminal copy/paste rides on the same arboard-backed commands. It must ship
// in the default build — the optional feature exists only so a slim build can
// opt out explicitly.
test("clipboard-history is part of the default cargo feature set", async () => {
  const cargo = await read("src-tauri/Cargo.toml");
  assert.match(
    cargo,
    /^\s*default\s*=\s*\[[^\]]*clipboard-history[^\]]*\]/m,
    "the default feature set must include clipboard-history",
  );
});

// Every overlay the extensions panel renders must have styling; a modal class
// with no rule anywhere drops the dialog into normal document flow (no dim,
// no centering, no card material).
test("every extensions modal class has a CSS rule", async () => {
  const css = [
    await read("src/styles/extensions.css"),
    await read("src/extensions/ComponentizedUninstallDialog.css"),
  ].join("\n");
  for (const className of [
    "extensions-dialog-overlay",
    "extensions-dialog",
    "extensions-dialog-title",
    "extensions-dialog-hint",
    "extensions-dialog-actions",
  ]) {
    assert.ok(
      css.includes(`.${className}`),
      `missing CSS rule for .${className}`,
    );
  }
});

// The in-row operation progress strip is rendered by ExtensionRow; without a
// rule it becomes an unstyled flex item that breaks the row's grid columns.
test("operation progress strip and advisory notice have CSS rules", async () => {
  const css = await read("src/styles/extensions.css");
  assert.ok(css.includes(".extension-row__progress"), "missing .extension-row__progress");
  assert.ok(css.includes(".extensions-notice--warning"), "missing .extensions-notice--warning");
});

// Toasts must be pinned to the card (fixed, via a portal host outside the page
// scroller), never inside the scrollable settings content where they drift off
// screen as the user scrolls a long integrations list.
test("toast stack is a fixed, card-level surface", async () => {
  const css = await read("src/styles/extensions.css");
  const host = css.slice(css.indexOf("#floter-app-toasts"));
  assert.match(host, /position:\s*fixed/, "toast host must be position: fixed");
  for (const className of [".app-toast", ".app-toast--error", ".app-toast--success"]) {
    assert.ok(css.includes(className), `missing ${className}`);
  }
});

// The integrations panel must never move the page scroll position when a
// notification fires. The old bug: `scrollIntoView` on the highlighted tool
// suggestion walked up to `.settings-content` and snapped it to the top, so
// any toast (success/error) raised while scrolled down yanked the list back.
// The panel must confine suggestion scrolling to its own list, and it must
// route every outcome through the app-level toast stack.
test("integrations panel does not scroll the page on feedback", async () => {
  const panel = await read("src/ExtensionsPanel.tsx");
  assert.ok(
    !panel.includes(".scrollIntoView("),
    "scrollIntoView walks up to the page scroller and resets the scroll position",
  );
  // Feedback still runs through the app-level toast stack, not a local surface.
  assert.match(
    panel,
    /const showError = useCallback\(\(text: string\) => onNotify\("error", text\)/,
    "errors must go to the app-level toast stack",
  );
  assert.match(
    panel,
    /const showSuccess = useCallback\(\(text: string\) => onNotify\("success", text\)/,
    "successes must go to the app-level toast stack",
  );
  // The suggestion highlight is kept visible by scrolling only its own list.
  assert.match(
    panel,
    /list\.scrollTop (?:\+=|-=)/,
    "suggestion highlight must scroll its own list, not the page",
  );
});

// The clipboard page's empty/failure state must not reuse the generic
// "Plugin failed to load" copy: when the backend is unavailable (feature off)
// the page has to say so and how to turn it back on, rather than implying the
// page itself failed to load.
test("clipboard page reports an unavailable backend distinctly", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  assert.ok(
    page.includes("clipboard.pageUnavailable"),
    "clipboard page must use the backend-unavailable message",
  );
  assert.ok(
    page.includes("clipboard.loadFailed"),
    "clipboard page must use the load-failed message",
  );
  assert.ok(
    !page.includes('t("plugin.pageError")'),
    "clipboard page must not claim the whole plugin failed to load",
  );
});

// The integrations list must stay mounted across a background refresh. Every
// mutation calls `refresh()`, which flips `loading`; rendering the one-line
// spinner on EVERY load (not just the first) replaced the whole list, collapsed
// `.settings-content`, and clamped its `scrollTop` back to 0 — the reported
// top-jump on every notification. The full-screen spinner must therefore be
// gated on the list being empty.
test("the integrations list stays mounted across refreshes", async () => {
  const panel = await read("src/ExtensionsPanel.tsx");
  assert.match(
    panel,
    /loading && extensions\.length === 0 \?/,
    "the full-screen loading state must only replace an empty list, never a populated one",
  );
});

// ── R6: badge stabilization + fixed height anchors ───────────────────────
//
// These tests read COMMENT-STRIPPED sources on purpose. The prose around each
// fix quotes the very values it explains ("min-height: 66px", "tabular-nums",
// ...), so a reader that kept comments would keep passing after the real
// declaration was deleted. Stripping comments makes every anchor below
// falsifiable: delete the declaration and the matching test fails.

/** Remove CSS block comments so an anchor can only be satisfied by a real rule. */
const stripCssComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

/** Remove JS/TSX comments (line + block) so an anchor can only be met by code. */
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/**
 * Extract one CSS rule body by its exact selector, asserting the selector is
 * unique. Uniqueness is the point: a `grep` for `.extensions-base-plugin {`
 * would also match `.extensions-base-plugin__name {`'s prefix, so a test that
 * merely searched the whole sheet could pass against the wrong rule.
 */
const cssRule = (css: string, selector: string) => {
  const head = `${selector} {`;
  const at = css.indexOf(head);
  assert.notEqual(at, -1, `missing rule: ${head}`);
  assert.equal(
    css.indexOf(head, at + head.length),
    -1,
    `selector must be unique in the sheet: ${head}`,
  );
  const open = at + head.length - 1;
  const close = css.indexOf("}", open);
  assert.notEqual(close, -1, `unterminated rule: ${head}`);
  return css.slice(open + 1, close);
};

// Fix 1 (badge): the count chip must derive its number from the SAME `extensions`
// state the rows render from, and must only show the spinner on the very first
// load (no data yet). Mutation: revert the gate to a bare `loading ?` and the
// number flickers to a spinner on every mutation's background refresh.
test("the section count chip only spins on the first load", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // Isolate the connected-section chip (the base-plugins chip renders
  // `basePlugins.length` with no conditional at all). Anchor on the connected
  // section's own i18n key so the slice can only cover that chip.
  const sectionAt = panel.indexOf('settings.extensions.section.connected');
  assert.notEqual(sectionAt, -1, "the connected-section title must exist");
  const chipAt = panel.indexOf('className="extension-status"', sectionAt);
  assert.notEqual(chipAt, -1, "the connected-section chip must exist");
  // Stop at the heading's own close tag. A fixed-width window (e.g. +400)
  // spilled past `</h3>` into the list's own `loading && extensions.length
  // === 0` gate, so the positive assertion below could be satisfied by the
  // LIST gate while the CHIP's gate was broken (mutations M9/M10 stayed green
  // against a 400-char window). The chip's slice must end before `</h3>`.
  const chipEnd = panel.indexOf("</h3>", chipAt);
  assert.notEqual(chipEnd, -1, "the connected-section heading must close");
  const chip = panel.slice(chipAt, chipEnd);
  assert.match(
    chip,
    /loading && extensions\.length === 0/,
    "the chip's spinner must be gated on an empty list, not on `loading` alone",
  );
  // No bare `loading ?` may survive INSIDE the chip: that is the exact middle
  // state (number -> spinner -> number) this round removed. (The refresh
  // button above legitimately swaps its glyph for a spinner on every load.)
  assert.ok(
    !/\bloading\s*\?/.test(chip),
    "a bare `loading ?` in the chip re-introduces the flicker middle state",
  );
});

// Fix 2 (geometry): the heading is the anchor every row below it hangs from.
// Mutation: drop `min-height`/`line-height` and a 1<->2 digit count or a
// wrapped label reflows the whole list.
test("the section heading has a fixed, font-independent height", async () => {
  const css = stripCssComments(await read("src/styles/extensions.css"));
  const title = cssRule(css, ".extensions-section-title");
  assert.match(title, /min-height:\s*18px/, "heading must pin a min-height");
  assert.match(title, /line-height:\s*1\.5/, "heading must set an explicit line-height");
});

// Fix 2 (chip geometry): the chip box must be identical for a 1-digit count, a
// 2-digit count, and the first-load spinner. Mutation: remove `min-width` and
// the chip narrows by 3px when the spinner (11px icon + 14px padding = 25px)
// is replaced by a one-digit number.
test("the count chip box is constant across number and spinner states", async () => {
  const css = stripCssComments(await read("src/styles/extensions.css"));
  const chip = cssRule(css, ".extensions-section-title .extension-status");
  assert.match(chip, /min-width:\s*25px/, "chip must reserve the spinner's width");
  assert.match(chip, /height:\s*18px/, "chip must be pinned to the heading line box");
  assert.match(chip, /font-variant-numeric:\s*tabular-nums/, "digits must not change width");
  assert.match(chip, /padding:\s*0 7px/, "chip padding must be vertical-free to fit 18px");
});

// Fix 2 (base-plugin card): the toggle card must not change height between
// enabled/disabled or one/two description lines. Mutation: remove `min-height`
// and a description-less card collapses ~4px, shifting the list below it.
test("the base-plugin card has a stable resting height", async () => {
  const css = stripCssComments(await read("src/styles/extensions.css"));
  const card = cssRule(css, ".extensions-base-plugin");
  assert.match(card, /min-height:\s*66px/, "card must pin a min-height");
  // The arithmetic only holds if the name line is explicit; without this the
  // browser's `normal` line-height (~1.2) silently makes the card ~4px shorter.
  const name = cssRule(css, ".extensions-base-plugin__name");
  assert.match(name, /line-height:\s*1\.5/, "name line-height must be explicit");
});

// Fix 3 (unbound row): a connected system tool with no persisted binding and no
// PATH candidates (runtimeAvailable=false, reconnectAvailable=false) previously
// rendered NO action, stranding the row. It must now offer reconnect.
// Mutation: restore the `(reconnectAvailable || homepage)` gate and the row
// loses its only recovery entry.
test("an unbound row with no candidates still offers an action", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  // The OLD render gate: the unavailable-runtime button was only rendered when
  // a candidate existed or a homepage was known, so an unbound row with
  // neither showed no action. Its absence is the fix.
  assert.ok(
    !/!extension\.runtimeAvailable\s*\n?\s*&& \(extension\.reconnectAvailable \|\| extension\.homepage\)/.test(row),
    "the unavailable-runtime button must not be gated on a candidate/homepage",
  );
  // Isolate the button's own decision expressions and assert the no-candidate
  // fallback is present in BOTH the action and the label (so a stale
  // aria-label cannot disagree with what the click does).
  const fallback = row.split("extension.reconnectAvailable || !extension.homepage").length - 1;
  assert.ok(
    fallback >= 3,
    `reconnect must be the fallback for the no-candidate case (found ${fallback}, want >=3)`,
  );
  // Both dictionaries must carry the explanatory hint (en + zh).
  const i18n = await read("src/i18n.ts");
  assert.equal(
    i18n.split('"settings.extensions.reconnectUnboundHint"').length - 1,
    2,
    "the unbound hint must be defined once per language",
  );
});
