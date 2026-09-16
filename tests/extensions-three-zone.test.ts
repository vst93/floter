import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

// These tests read COMMENT-STRIPPED sources on purpose (same discipline as the
// R6 block in extension-ui-surfaces.test.ts): the prose around each anchor
// quotes the very values it explains, so a reader that kept comments could keep
// passing after the real declaration was deleted.

const stripCssComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

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

/** The body of a `const name = (...) => { … }` declaration, taken by brace
 *  matching so the slice ends at the function's own closing brace. An anchor
 *  of "the next declaration we happen to know the name of" silently changes
 *  meaning when that neighbour is renamed or deleted. */
const functionBody = (source: string, signature: string) => {
  const at = source.indexOf(signature);
  assert.notEqual(at, -1, `missing declaration: ${signature}`);
  const open = source.indexOf("{", at);
  assert.notEqual(open, -1, `declaration without a body: ${signature}`);
  let depth = 0;
  for (let i = open; i < source.length; i += 1) {
    if (source[i] === "{") depth += 1;
    else if (source[i] === "}") {
      depth -= 1;
      if (depth === 0) return source.slice(open + 1, i);
    }
  }
  assert.fail(`unterminated declaration: ${signature}`);
};

// ── R7-3a: Base plugins / Connected / Detected three-zone layout ─────────

// The page had a single `Connected` section whose heading lied: the backend
// returns connected AND unconnected rows in one `extensions` array, but only
// `connectedExtensions` was rendered, so `suggestedExtensions` (the local
// detections) were reachable only from inside the create-custom drawer and the
// `extension-row--detected` style was dead code. The fix adds a Detected
// section BELOW Connected.
//
// Mutation: delete the Detected section (or move it above Connected) and this
// fails.
test("the Detected section is rendered after Connected", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const connectedAt = panel.indexOf('settings.extensions.section.connected');
  const detectedAt = panel.indexOf('settings.extensions.section.detected');
  assert.notEqual(connectedAt, -1, "the Connected section title must exist");
  assert.notEqual(detectedAt, -1, "the Detected section title must exist");
  assert.ok(
    detectedAt > connectedAt,
    "Detected must come after Connected (installed first, candidates below)",
  );
  // Both headings reuse the shared section-title language, so the two lists
  // read as siblings instead of one being a different kind of surface.
  const connectedHeadingAt = panel.lastIndexOf("<h3", connectedAt);
  assert.notEqual(connectedHeadingAt, -1, "the Connected heading must exist");
  assert.match(
    panel.slice(connectedHeadingAt, connectedAt),
    /extensions-section-title/,
    "Connected heading must use the shared section-title class",
  );
  const detectedHeadingAt = panel.lastIndexOf("<h3", detectedAt);
  assert.notEqual(detectedHeadingAt, -1, "the Detected heading must exist");
  const detectedHeading = panel.slice(detectedHeadingAt, panel.indexOf("</h3>", detectedAt));
  assert.match(
    detectedHeading,
    /extensions-section-title/,
    "Detected heading must reuse the shared section-title class",
  );
});

// Detected is the local inventory, not a storefront: when nothing is detected
// the whole section must disappear rather than leave an empty-state card behind
// (flat and compact — a "0 detected" placeholder is chrome that pushes the rest
// of the page down for no information).
//
// Mutation: drop the `suggestedExtensions.length > 0 &&` guard and the section
// renders unconditionally; this fails.
test("the Detected section hides itself when nothing is detected", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const detectedAt = panel.indexOf('settings.extensions.section.detected');
  assert.notEqual(detectedAt, -1, "the Detected section title must exist");
  // Walk back to the opening `{... && (` that gates this section.
  const before = panel.slice(0, detectedAt);
  const gateAt = before.lastIndexOf("suggestedExtensions.length > 0 &&");
  assert.notEqual(gateAt, -1, "the Detected section must be gated on a non-empty detection list");
  // The gate must be the nearest structural expression before the section, i.e.
  // it actually wraps the <section>, not some unrelated nearby condition.
  const between = panel.slice(gateAt, detectedAt);
  assert.match(between, /<section/, "the non-empty gate must directly open the Detected <section>");
  // No empty-state placeholder may live inside the Detected section.
  const sectionEnd = panel.indexOf("</section>", detectedAt);
  assert.notEqual(sectionEnd, -1, "the Detected section must close");
  const section = panel.slice(detectedAt, sectionEnd);
  assert.ok(
    !/EmptyState/.test(section),
    "Detected must not render an empty-state placeholder when it has no rows",
  );
});

// A detected row is not a button: `ExtensionRow` renders the row body as a
// <div> for `!connected` (there is no detail drawer to open) and only the
// connected path renders a <button>. The Detected section must also pass NO
// `onOpen`, since a div has no click affordance to wire it to.
//
// Mutation: render the detected body as a button and this fails.
test("detected rows are not buttons and have no open-detail affordance", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  assert.match(
    row,
    /extension\.connected \? \(\s*<button type="button" className="extension-row__open" onClick=\{onOpen\}>/,
    "connected rows must render the row body as a button wired to onOpen",
  );
  assert.match(
    row,
    /\) : \(\s*<div className="extension-row__open">\{rowContent\}<\/div>\s*\)/,
    "unconnected rows must render the row body as a plain div (no button, no click)",
  );
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const detectedAt = panel.indexOf('settings.extensions.section.detected');
  const sectionEnd = panel.indexOf("</section>", detectedAt);
  const section = panel.slice(detectedAt, sectionEnd);
  assert.ok(
    !/onOpen=/.test(section),
    "Detected rows must not wire onOpen — the row body is a div",
  );
});

// Connect, never reconnect. `reconnect` re-scans the tool inventory and WRITES
// a tool binding (R3/G3: an explicit, system-only action). A row that merely
// reports "this device has X" must not pre-select that write. The detected
// section wires `onConnect` to the existing connect flow and never passes
// `onReconnect`.
//
// Mutation: bind the detected row's action to reconnect (pass onReconnect /
// call reconnectSystem from connectDetected) and this fails.
test("detected rows connect through the connect flow, never reconnect", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const detectedAt = panel.indexOf('settings.extensions.section.detected');
  assert.notEqual(detectedAt, -1, "the Detected section must exist");
  const sectionEnd = panel.indexOf("</section>", detectedAt);
  const section = panel.slice(detectedAt, sectionEnd);
  assert.match(section, /onConnect=\{\(\) => connectDetected\(extension\)\}/, "Detected rows must wire onConnect");
  assert.ok(
    !/onReconnect/.test(section),
    "Detected rows must never wire onReconnect (inventory re-discovery + tool-lock write)",
  );
  assert.ok(
    !/reconnectSystem/.test(section),
    "Detected rows must never call reconnectSystem",
  );

  // The connect entry itself must route recommended/manifest rows through the
  // shared connect pipeline and must not touch reconnect. The body is taken by
  // brace matching rather than by "the next declaration with a known name",
  // because R7-3b deleted the `toolSuggestions` block that used to be this
  // test's end anchor — an anchor a later round deletes silently shortens the
  // slice (or fails) instead of failing on the thing under test.
  const entry = functionBody(panel, "const connectDetected = (extension: Extension)");
  assert.match(entry, /connectRecommended\(extension\)/, "authored manifests must use the connect pipeline");
  assert.ok(
    !/reconnectSystem/.test(entry),
    "connectDetected must not reach for reconnectSystem",
  );
});

// R7-3b (F1) closed the R7-3a finding that the detected list was rendered in
// two places: the Detected section AND the create-custom drawer's executable
// picker, under a second label ("Detected on this device"). Connecting a bare
// PATH discovery opens that drawer with the executable prefilled, which is the
// ONLY create entry now — the blank "Create custom" toolbar button moved into
// the overflow menu and is asserted separately.
//
// The invariant: the drawer's suggestion list is search results only, and the
// drawer no longer consumes `suggestedExtensions` at all. Restoring the gated
// fallback re-renders the same list in two places and fails here.
//
// Mutation: put `...suggestedExtensions.map(...)` back into `toolSuggestions`
// (or re-add the `drawerSuggestions` header line) and this fails.
test("the edit drawer no longer re-renders the detected list", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const drawer = stripJsComments(await read("src/extensions/CustomIntegrationDrawer.tsx"));

  // No suggestion-list construction anywhere in the panel: the drawer receives
  // search hits (`toolResults`) and nothing else.
  assert.ok(
    !/toolSuggestions/.test(panel),
    "the panel must not build a suggestion list for the drawer (the Detected section is that list)",
  );
  assert.ok(
    /<CustomIntegrationDrawer[\s\S]*?toolResults=\{toolResults\}/.test(panel),
    "the drawer must receive the executable search results",
  );
  // The drawer itself names neither the removed type nor the removed header.
  assert.ok(!/ToolSuggestion/.test(drawer), "the drawer must not reference ToolSuggestion");
  assert.ok(
    !/drawerSuggestions/.test(drawer),
    "the drawer must not render the \"Detected on this device\" list header",
  );
  assert.ok(
    !/suggestedExtensions/.test(drawer),
    "the drawer must not read the detected inventory",
  );

  // …and the dead i18n string is gone from both dictionaries, so the two-place
  // rendering cannot come back through a translation key either.
  const i18n = await read("src/i18n.ts");
  assert.equal(
    i18n.split('"settings.extensions.drawerSuggestions"').length - 1,
    0,
    "settings.extensions.drawerSuggestions belongs to the removed fallback and must be deleted",
  );
  // The executable field keeps its own, edit-flavoured guidance instead.
  assert.equal(
    i18n.split('"settings.extensions.changeExecutableHint"').length - 1,
    2,
    "the edit drawer's caption must exist once per language",
  );
});

// The connect entry's prefill path (a bare PATH discovery) has to survive the
// narrowing: the discovery's own path is what the user is choosing between, so
// the row must still be able to hand it to the (prefilled) create form.
test("a bare PATH discovery still prefills the create form", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const entry = functionBody(panel, "const connectDetected = (extension: Extension)");
  assert.match(entry, /openCreateCustomIntegration\(\)/, "a discovery must reach the create form");
  assert.match(entry, /chooseToolCandidate\(\{/, "the discovery must prefill the executable");
  assert.match(entry, /path: extension\.executablePath/, "the prefill is the discovered path");
  // The form it opens must accept the prefill — a create-mode drawer, not an
  // edit-mode one (the constructor is the only place `editingCustomId` is set).
  const open = functionBody(panel, "const openCreateCustomIntegration =");
  assert.match(open, /setEditingCustomId\(null\)/, "the prefill form is create mode, not edit mode");
  assert.match(open, /setShowCustomIntegration\(true\)/, "the create form must actually open");
});

// The Detected section reuses the R6-stabilized heading + chip system, so the
// new count badge inherits the exact geometry that was pinned for Connected:
// min-height 18px heading and a tabular-nums, min-width 25px chip. A second,
// bespoke heading style would drift the two sections apart.
test("the Detected heading inherits the stabilized section-title geometry", async () => {
  const css = stripCssComments(await read("src/styles/extensions.css"));
  const title = cssRule(css, ".extensions-section-title");
  assert.match(title, /min-height:\s*18px/, "heading min-height must stay pinned");
  assert.match(title, /line-height:\s*1\.5/, "heading line-height must stay explicit");
  const chip = cssRule(css, ".extensions-section-title .extension-status");
  assert.match(chip, /min-width:\s*25px/, "chip must reserve the spinner width");
  assert.match(chip, /height:\s*18px/, "chip must be pinned to the heading line box");
  assert.match(chip, /font-variant-numeric:\s*tabular-nums/, "digits must not change width");
  // The Detected heading itself must not introduce a competing title rule.
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const detectedAt = panel.indexOf('settings.extensions.section.detected');
  const heading = panel.slice(detectedAt, panel.indexOf("</h3>", detectedAt));
  assert.match(heading, /className="extension-status"/, "Detected count must use the shared chip class");
});

// Detected is a sibling list, not a new surface: no card chrome, no extra
// section gap. The only delta is that the connect action is always visible on
// a detected row (it is the row's reason to exist), which is a visibility
// change, not a layout one.
test("the Detected group stays flat and keeps the connect action visible", async () => {
  const css = stripCssComments(await read("src/styles/extensions.css"));
  const detected = cssRule(css, ".extensions-section--detected");
  assert.ok(
    !/border|background|box-shadow/.test(detected),
    "Detected must not grow card chrome around the list",
  );
  const actions = cssRule(css, ".extension-row--detected .extension-row__actions");
  assert.match(actions, /visibility:\s*visible/, "a detected row's connect action must stay visible");
});

// Both dictionaries must carry the new section label. `zh` is typed as
// Record<MessageKey, string>, so a missing zh key is a compile error too — this
// test additionally pins the exact English label (and keeps the section out of
// storefront vocabulary: no Store / Marketplace / Discover).
test("the Detected section label exists in both dictionaries", async () => {
  const i18n = await read("src/i18n.ts");
  assert.equal(
    i18n.split('"settings.extensions.section.detected"').length - 1,
    2,
    "settings.extensions.section.detected must be defined once per language",
  );
  assert.match(i18n, /"settings\.extensions\.section\.detected":\s*"Detected"/);
  assert.match(i18n, /"settings\.extensions\.section\.detected":\s*"检测到"/);
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const detectedAt = panel.indexOf('settings.extensions.section.detected');
  const detectedEnd = panel.indexOf("</section>", detectedAt);
  const detectedSection = panel.slice(detectedAt, detectedEnd);
  for (const forbidden of ["Store", "Marketplace", "Discover", "store", "marketplace", "discover"]) {
    assert.ok(
      !detectedSection.includes(forbidden),
      `Detected must not adopt storefront vocabulary (${forbidden})`,
    );
  }
});
