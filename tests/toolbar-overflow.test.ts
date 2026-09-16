// R7-3b · The toolbar's frequency split (G-24) and the overflow menu's
// keyboard contract.
//
// The finding this round closes: the integrations toolbar carried four
// equal-weight buttons and two always-on caption lines between the heading and
// the list, so the action a user repeats (connecting something local) competed
// with the rarest pair on the page (importing/exporting the whole collection as
// a JSON file), and the explanations for that rare pair were permanently on
// screen.
//
// Three things have to stay true, and each is asserted separately:
//
//   1. the visible row is bounded — one primary action plus the overflow
//      trigger, and no caption block (a regression that "adds one more button"
//      has to fail, not be absorbed);
//   2. nothing is deleted — every action the toolbar had is still reachable,
//      from the menu, with its handler intact;
//   3. the menu is a first-class keyboard surface — Escape closes it *without*
//      dismissing the settings surface underneath, and ↑/↓/Home/End move the
//      active item.
//
// The menu's material is checked here too: it is a floater, so it consumes the
// glass tokens and carries no backdrop-filter of its own (the performance red
// line, whose general form lives in `glass-material.test.ts`).
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripCssComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim(), body: match[2] });
  }
  return out;
};

const declarations = (body: string, property: string) =>
  [...body.matchAll(new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g"))].map((m) => m[1].trim());

/** The JSX of the toolbar: from its opening tag through the end of the
 *  overflow group. Slicing to the group's close — not to `<OverflowMenu` — is
 *  the point: a third button placed after the trigger would otherwise escape
 *  the budget (reviewer mutation c2 proved exactly that hole). The group has
 *  no nested divs, so a single indexOf of its closing tag is a stable slice. */
const toolbarRow = (source: string) => {
  const at = source.indexOf('className="extensions-sync-toolbar"');
  assert.notEqual(at, -1, "the toolbar row must exist");
  const groupStart = source.indexOf('extensions-sync-toolbar__group--overflow', at);
  assert.notEqual(groupStart, -1, "the overflow group must exist in the row");
  const groupEnd = source.indexOf('</div>', groupStart);
  assert.notEqual(groupEnd, -1, "the overflow group's closing </div> must exist");
  return source.slice(at, groupEnd + 6);
};

// ── 1. The visible row is bounded ─────────────────────────────────────────

// The row's own budget, stated as a number so "visual balance" cannot drift
// into "one more button". Two actions in the row, one of which is the overflow
// trigger; anything beyond that is a G-24 regression.
test("the visible toolbar row stays within its two-control budget", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const row = toolbarRow(panel);

  // Count the action buttons that are in the row before the menu — this is the
  // number a user sees without opening anything.
  const rowButtons = [...row.matchAll(/<button/g)].length;
  const overflowTriggers = [...row.matchAll(/<OverflowMenu/g)].length;
  assert.equal(overflowTriggers, 1, "exactly one overflow trigger in the row");
  assert.ok(
    rowButtons <= 1,
    `the row renders ${rowButtons} plain buttons across the whole overflow group; ` +
      "the budget is one action plus the overflow trigger (G-24), and the trigger " +
      "is an <OverflowMenu>, not a <button> — any extra <button> is a G-24 regression",
  );
  // …and the trigger really is there, immediately after.
  assert.match(panel.slice(panel.indexOf("className=\"extensions-sync-toolbar\"")), /<OverflowMenu/, "the row must carry the overflow trigger");

  // The standing caption block is gone: the sentence travels with the actions
  // it describes, inside the menu.
  assert.ok(
    !/extensions-package-hint/.test(panel),
    "the two always-on toolbar captions must not come back (their sentence lives in the menu)",
  );
  const css = stripCssComments(await read("src/styles/extensions.css"));
  assert.ok(
    !/\.extensions-package-hints?\s*\{/.test(css),
    "the deleted caption block must not leave a dead rule behind",
  );
  assert.ok(
    !/extensions-sync-toolbar__group--transfer/.test(css),
    "the transfer group's divider belonged to the removed second button group",
  );

  // The row still reads as two zones: the action, then the menu behind a
  // divider, so the split is visible rather than implied.
  const divider = rules(css).find((r) => r.selector === ".extensions-sync-toolbar__group--overflow");
  assert.ok(divider, "the overflow group must have its own rule");
  assert.match(divider!.body, /border-inline-start:/, "the overflow zone keeps its divider");
});

// The primary action is the one that works on a package that already exists.
// "Create custom" is the blank authoring form, and after R7-3a every real
// custom integration starts from a Detected row's prefill — which is why the
// blank entry is the one that moved into the menu.
test("the row leads with the existing-package connect, not the blank form", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const row = toolbarRow(panel);
  assert.match(row, /t\("settings\.extensions\.chooseManifest"\)/, "the connect-package action stays in the row");
  assert.ok(
    !/t\("settings\.extensions\.createCustom"\)/.test(row),
    "the blank create form must not be in the row — it is a menu item now",
  );
  assert.match(row, /extensions-action-button--primary/, "the surviving action is the primary one");
});

// ── 2. Nothing is deleted, only collected ─────────────────────────────────

test("every collected action is still reachable from the overflow menu", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const items = panel.slice(
    panel.indexOf("const overflowItems = ["),
    panel.indexOf("const overflowItems = [") + panel.slice(panel.indexOf("const overflowItems = [")).indexOf("];") + 2,
  );
  // The three collected actions, each with its handler — the menu is a
  // relocation, never a removal.
  assert.match(items, /id: "create"[\s\S]*?onSelect: openCreateCustomIntegration/);
  assert.match(items, /id: "export"[\s\S]*?onSelect: \(\) => void exportExtensions\(\)/);
  assert.match(items, /id: "import"[\s\S]*?onSelect: \(\) => void importExtensions\(\)/);
  // All three keep the same disabled gate the row uses, so a menu item cannot
  // become the one clickable control during a mutation.
  const gates = [...items.matchAll(/disabled: Boolean\(syncOperation\) \|\| Boolean\(busy\) \|\| loading/g)];
  assert.equal(gates.length, 3, "every menu item must carry the row's disabled gate");

  // The transfer pair's explanation moves into the menu rather than vanishing.
  assert.match(
    panel,
    /note=\{t\("settings\.extensions\.fileTransferHint"\)\}/,
    "the transfer caption must travel into the menu as its note",
  );
  const i18n = await read("src/i18n.ts");
  assert.equal(
    i18n.split('"settings.extensions.fileTransferHint"').length - 1,
    2,
    "the note must exist in both dictionaries",
  );

  // In-flight transfer feedback survives the move: the row used to show it on
  // the buttons themselves.
  assert.match(panel, /extensions-sync-status/, "a transfer in flight must still report on the row");
  // The literal list above means nothing unless the menu actually receives it:
  // assert the render site passes the collected array as its items (reviewer
  // mutation f: items={[]} rendered an empty menu with this file all green).
  assert.match(
    panel,
    /<OverflowMenu[\s\S]*?items=\{overflowItems\}/,
    "the overflow menu must be wired to the collected items array",
  );
});

// ── 3. The menu is a keyboard surface ─────────────────────────────────────

// The behaviours asserted here are the ones a custom popup does not get for
// free from `<details>`: open-on-arrow, arrow navigation, Escape, and Escape
// being *consumed* so the settings surface behind the menu does not also read
// the press as "close settings".
test("the overflow menu is fully keyboard driven", async () => {
  const menu = stripJsComments(await read("src/components/OverflowMenu.tsx"));

  // Open: ↓/↑ from the trigger lands on the first/last enabled item and the
  // trigger reports its state to assistive tech.
  assert.match(menu, /onTriggerKeyDown/, "the trigger must handle arrow keys");
  assert.match(menu, /event\.key !== "ArrowDown" && event\.key !== "ArrowUp"/, "↓/↑ open the menu");
  assert.match(menu, /aria-haspopup="menu"/, "the trigger must advertise the popup");
  assert.match(menu, /aria-expanded=\{open\}/, "the trigger must report the open state");

  // Navigate: ↓/↑ walk and wrap, Home/End jump to the ends.
  assert.match(menu, /const step = \(from: string \| null, delta: number\)/, "arrow navigation needs a step helper");
  assert.match(menu, /\(index \+ delta \+ enabled\.length\) % enabled\.length/, "↓/↑ must wrap");
  assert.match(menu, /event\.key === "Home" \|\| event\.key === "End"/, "Home/End must jump to the ends");

  // Activate: the items are native buttons and Enter/Space are left to them —
  // the menu must not swallow them.
  assert.match(menu, /role="menuitem"/, "items must be menu items");
  assert.ok(
    !/event\.key === "Enter"/.test(menu),
    "Enter must not be intercepted: native button activation is the keyboard path",
  );

  // Escape closes AND consumes the press. `stopPropagation` is the whole point:
  // the window-level dismiss table (surface-policy) would otherwise read the
  // same Escape as the settings surface's dismiss.
  const escapeAt = menu.indexOf('event.key === "Escape"');
  assert.notEqual(escapeAt, -1, "Escape must be handled");
  const escape = menu.slice(escapeAt, escapeAt + 320);
  assert.match(escape, /event\.preventDefault\(\)/, "Escape must prevent the default");
  assert.match(
    escape,
    /event\.stopPropagation\(\)/,
    "Escape must be consumed, or the settings surface behind the menu also dismisses",
  );
  assert.match(escape, /close\(true\)/, "Escape must close and hand the keyboard back to the trigger");

  // Closing is pointer-safe too: a click outside closes without stealing focus.
  assert.match(menu, /document\.addEventListener\("mousedown", handlePointerDown\)/);
});

// The trigger and the items have to be real, focusable controls — a div with a
// keydown handler is not a keyboard path, and `tabIndex={-1}` is correct for
// the items because the *trigger* is the tab stop (menu semantics).
test("the menu is built from focusable native controls", async () => {
  const menu = stripJsComments(await read("src/components/OverflowMenu.tsx"));
  assert.match(menu, /<button[\s\S]*?aria-haspopup="menu"/, "the trigger must be a button");
  assert.match(menu, /role="menuitem"[\s\S]*?tabIndex=\{-1\}/, "items are roving (the trigger holds the tab stop)");
  assert.match(menu, /itemRefs\.current\.get\(id\)\?\.focus/, "navigation must move real focus");
  // The trigger's accessible name is the caller's label; the panel passes the
  // localized one rather than a literal.
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(panel, /label=\{t\("settings\.extensions\.moreActions"\)\}/);
});

// ── 4. The menu is a floater, not a new material ──────────────────────────

// R7-1's motion tokens and R8's material ladder were both in place; a new
// overlay has to consume them rather than re-derive a fill or a curve.
test("the overflow menu consumes the glass and motion tokens", async () => {
  const css = stripCssComments(await read("src/styles/extensions.css"));
  const items = rules(css).find((r) => r.selector === ".extensions-overflow__items");
  assert.ok(items, "the menu panel must have its own rule");
  assert.match(items!.body, /background:\s*var\(--glass-float\)/, "a floater uses the float rung");
  assert.match(items!.body, /box-shadow:\s*var\(--elev-3\)/, "a floater uses the top elevation rung");
  assert.match(
    items!.body,
    /animation:\s*extensions-overflow-in var\(--dur-\d\) var\(--spring\)/,
    "the menu enters on the house spring at a token duration",
  );
  // No literal fill and no filter of its own.
  assert.ok(
    !/(?:^|;)\s*background:[^;]*rgba\(/.test(items!.body),
    "the menu must not carry a hand-written fill",
  );
  for (const body of [items!.body, rules(css).find((r) => r.selector === ".extensions-overflow__item")!.body]) {
    assert.ok(!/backdrop-filter/.test(body), "controls never filter the backdrop");
  }
  // Its entrance is neutralized under reduced motion like every other arrival.
  const base = stripCssComments(await read("src/styles/base.css"));
  const reduced = base.slice(base.indexOf("@media (prefers-reduced-motion: reduce)"));
  assert.match(
    reduced,
    /\.extensions-overflow__items/,
    "prefers-reduced-motion must neutralize the menu's entrance",
  );
  assert.match(reduced, /animation:\s*none/);
});

// The focus ring is one system-wide width and offset (R7-HIG); a new control
// must join it rather than invent an outline.
test("menu items use the shared focus ring", async () => {
  const css = stripCssComments(await read("src/styles/extensions.css"));
  const focus = rules(css).find((r) => r.selector === ".extensions-overflow__item:focus-visible");
  assert.ok(focus, "menu items need a focus-visible rule");
  assert.match(focus!.body, /outline:\s*var\(--focus-ring-width\) solid var\(--accent-ring\)/);
});
