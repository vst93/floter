// R7-3b · The toolbar's frequency split (G-24) and the overflow menu's
// keyboard contract. R49 revisited the split.
//
// The finding R7-3b closed: the integrations toolbar carried four
// equal-weight buttons and two always-on caption lines between the heading and
// the list, so the action a user repeats (connecting something local) competed
// with the rarest pair on the page (importing/exporting the whole collection as
// a JSON file), and the explanations for that rare pair were permanently on
// screen.
//
// R49's finding: the split was drawn one action too far in. The user read the
// row with the menu open and reported that the space was there and that
// "Create custom" — 「还是很常见或者很重要的」 — did not belong inside it. The
// measurement agrees for that one action and disagrees for the transfer pair:
//
//   * the row's content box is 502.6px at the shipped default step and 495.4px
//     at `large` (720px window − 148px sidebar − 36u page padding − 10px
//     scrollbar);
//   * in the browser, four labelled pills in the row's two zones wrap onto a
//     second line (row 47.9px -> 85.8px) from the *default* step up on Arial
//     metrics and Noto Sans, and from `small` up on DejaVu Sans — in English;
//   * Connect + Create custom + the trigger stay on one line at every step in
//     both languages, with ≥64px of clearance in the worst measured cell.
//
// So the row's visible *authoring* set grew by one and its overflow kept its
// two rare actions. Four things have to stay true, and each is asserted
// separately:
//
//   1. the visible row is bounded — the two authoring actions plus the overflow
//      trigger, no caption block, and no transfer button (a regression that
//      "adds one more button" still has to fail, not be absorbed);
//   2. the promoted action is a real, visible, secondary button that keeps the
//      menu item's handler and its disabled gate — a promotion that dropped
//      either would be a deletion dressed as a move;
//   3. nothing is deleted — the two actions the menu still holds are reachable
//      from it, with their handlers and their note intact;
//   4. the menu is a first-class keyboard surface — Escape closes it *without*
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

/** One `<button>` from the row, located by a string that appears in its
 *  children (so the slice cannot be confused with a neighbouring control). */
const rowButton = (row: string, needle: string) => {
  const at = row.indexOf(needle);
  assert.notEqual(at, -1, `${needle} must be in the row`);
  const start = row.lastIndexOf("<button", at);
  assert.notEqual(start, -1, `${needle} must sit inside a button`);
  const end = row.indexOf("</button>", at);
  assert.notEqual(end, -1, `${needle}'s button must close`);
  return row.slice(start, end + 9);
};

// ── 1. The visible row is bounded ─────────────────────────────────────────

// The row's budget, stated as a number so "visual balance" cannot drift
// into "one more button". R7-3b set the budget at one visible action (connect)
// plus the overflow trigger; R49 raised it by exactly one, for the authoring
// form the user asked back out of the menu. So the budget is now the two
// actions that *author* an integration — connect an existing package, open the
// blank custom form — plus the trigger, and every action that consumes a
// package or a whole collection is either a menu item or the primary action.
// Two visible actions, not three: the transfer pair is what the width budget
// on `overflowItems` says cannot live in the row.
test("the visible toolbar row keeps the two authoring actions and one trigger", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const row = toolbarRow(panel);

  // Count the action buttons that are in the row before the menu — this is the
  // number a user sees without opening anything.
  const rowButtons = [...row.matchAll(/<button/g)].length;
  const overflowTriggers = [...row.matchAll(/<OverflowMenu/g)].length;
  assert.equal(overflowTriggers, 1, "exactly one overflow trigger in the row");
  assert.equal(
    rowButtons,
    2,
    `the row renders ${rowButtons} plain buttons; the budget is two authoring actions ` +
      "(connect an existing package, open the blank custom form) plus the overflow " +
      "trigger (G-24, raised by one in R49), and the trigger is an <OverflowMenu>, " +
      "not a <button> — any extra <button> is a regression",
  );
  // …and the trigger really is there, immediately after.
  assert.match(panel.slice(panel.indexOf("className=\"extensions-sync-toolbar\"")), /<OverflowMenu/, "the row must carry the overflow trigger");

  // The two actions that *consume* a whole collection stay out of the row.
  // This is the R49 half of the budget: the pair lives inside the menu (section
  // 2 asserts the handlers), so a future "promote them too" has to fail here
  // and argue with the wrap measurements instead of slipping in.
  for (const key of ["settings.extensions.export", "settings.extensions.import"]) {
    assert.ok(
      !row.includes(`t("${key}")`),
      `${key} must not be a button in the row — the transfer pair is the overflow's content`,
    );
  }

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

  // The row still reads as two zones: the actions, then the menu behind a
  // divider, so the split is visible rather than implied. The promoted button
  // joins the *first* group (its own `gap: 8px` spaces the pair) and must not
  // have brought a second divider or a new wrapper with it.
  const divider = rules(css).find((r) => r.selector === ".extensions-sync-toolbar__group--overflow");
  assert.ok(divider, "the overflow group must have its own rule");
  assert.match(divider!.body, /border-inline-start:/, "the overflow zone keeps its divider");
  const group = rules(css).find((r) => r.selector === ".extensions-sync-toolbar__group");
  assert.ok(group, "the leading group must have a rule");
  assert.match(group!.body, /gap:/, "the promoted button reuses the group's existing gap, not a new one");
  assert.ok(
    !/extensions-sync-toolbar__group--(?:create|authoring)/.test(css),
    "promotion must not invent a second leading group (the pair sits in the existing one)",
  );
});

// The primary action is still the one that works on a package that already
// exists: the row leads with Connect, and the blank form is its secondary
// neighbour. R7-3b had "Create custom" in the menu, on the argument that every
// real custom integration starts from a Detected row's prefill (R7-3a); R49
// promoted it because it is the row's only authoring entry and the user named
// it as common/important — but *secondary*, never a second primary.
test("the row leads with the existing-package connect, and the blank form follows it secondary", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const row = toolbarRow(panel);
  assert.match(row, /t\("settings\.extensions\.chooseManifest"\)/, "the connect-package action stays in the row");
  assert.match(row, /t\("settings\.extensions\.createCustom"\)/, "the blank create form is back in the row (R49)");

  // Order: connect first, create second — the primary action leads — and both
  // on the near side of the divider, i.e. in the leading group.
  assert.ok(
    row.indexOf("settings.extensions.chooseManifest") < row.indexOf("settings.extensions.createCustom"),
    "the row must lead with connect, not with the blank form",
  );
  assert.ok(
    row.indexOf("settings.extensions.createCustom") < row.indexOf("extensions-sync-toolbar__group--overflow"),
    "the promoted action sits in the leading group, on Connect's side of the divider",
  );

  const connectButton = rowButton(row, "settings.extensions.chooseManifest");
  const createButton = rowButton(row, "settings.extensions.createCustom");
  assert.match(connectButton, /extensions-action-button--primary/, "connect is the row's primary action");
  assert.ok(
    !/--primary/.test(createButton),
    "the promoted action is secondary: a second accent-filled pill would also spend the " +
      "settings view's accent budget (accent-budget.test.ts)",
  );
  assert.match(
    createButton,
    /className="extensions-action-button"/,
    "the promoted button is the shared action pill, not a new control",
  );
  // The promotion is a *move*: the handler and the disabled gate come with it.
  assert.match(createButton, /onClick=\{openCreateCustomIntegration\}/, "the promoted button keeps the menu item's handler");
  assert.match(
    createButton,
    /disabled=\{Boolean\(syncOperation\) \|\| Boolean\(busy\) \|\| loading\}/,
    "the promoted button keeps the row's disabled gate",
  );
  assert.match(
    connectButton,
    /disabled=\{Boolean\(syncOperation\) \|\| Boolean\(busy\) \|\| loading\}/,
    "every visible action shares one gate, so none of them is clickable mid-mutation",
  );
});

// ── 2. Nothing is deleted, only collected ─────────────────────────────────

test("every collected action is still reachable from the overflow menu", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const items = panel.slice(
    panel.indexOf("const overflowItems = ["),
    panel.indexOf("const overflowItems = [") + panel.slice(panel.indexOf("const overflowItems = [")).indexOf("];") + 2,
  );
  // The two collected actions, each with its handler — the menu is a
  // relocation, never a removal.
  assert.match(items, /id: "export"[\s\S]*?onSelect: \(\) => void exportExtensions\(\)/);
  assert.match(items, /id: "import"[\s\S]*?onSelect: \(\) => void importExtensions\(\)/);
  // …and the promoted action is *not* duplicated here: two entries with one
  // handler would give the row and the menu two different disabled states for
  // the same action (the menu item reads `loading`, the button is the row's).
  assert.ok(
    !/id: "create"/.test(items),
    "the promoted action must have left the menu — one action, one control",
  );
  // Both keep the same disabled gate the row uses, so a menu item cannot
  // become the one clickable control during a mutation.
  const gates = [...items.matchAll(/disabled: Boolean\(syncOperation\) \|\| Boolean\(busy\) \|\| loading/g)];
  assert.equal(gates.length, 2, "every menu item must carry the row's disabled gate");

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
