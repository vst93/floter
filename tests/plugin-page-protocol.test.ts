// R7-12 · the plugin-page protocol as a *published* contract.
//
// Until this round the bridge was an implicit agreement between
// `PluginPageHost.tsx` and `clipboard/main.ts`: the message names lived in
// `src/plugin-pages.ts`, the documentation lived in code comments, and there
// was no version on the wire — so a third-party page author had nothing to
// read and no way to find out they had guessed wrong.
//
// This suite pins the four pieces that make the contract real:
//
//   1. the handshake verdict — accepted / missing / mismatch — decided by the
//      pure `pluginPageHandshake`, plus the error text that names both
//      protocol numbers;
//   2. the document's message table — every message name the protocol defines
//      appears there exactly once, with the right direction and the payload
//      keys the example actually sends;
//   3. the example page — its builders produce payloads the host's own
//      predicates accept, so "copy this example" cannot be wrong;
//   4. two mutation locks — the two ways the handshake could silently rot.
//
// The document is parsed, not re-typed: a message added to `src/plugin-pages.ts`
// without a row (or a row naming a message that does not exist) is red.

import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import test from "node:test";

import {
  BRIDGE_TAG,
  PLUGIN_PAGE_PROTOCOL,
  handshakeErrorDetail,
  isBridgeClose,
  isBridgeDrag,
  isBridgeFrameReady,
  isBridgeNotify,
  isBridgeRequest,
  pluginPageHandshake,
} from "../src/plugin-pages.ts";
import { createTranslator } from "../src/i18n.ts";

// The example's own vocabulary module — the same file a page author copies.
import * as example from "../docs/extensions/examples/hello-page/protocol.js";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const DOC = "docs/extensions/plugin-page-protocol.md";
const EXAMPLE = "docs/extensions/examples/hello-page/protocol.js";

// ── 1 · the handshake verdict ─────────────────────────────────────────────

test("the host and the example agree on the protocol version, and it is 1", () => {
  assert.equal(PLUGIN_PAGE_PROTOCOL, 1, "protocol 1 is what this round ships");
  assert.equal(
    example.PROTOCOL,
    PLUGIN_PAGE_PROTOCOL,
    "the example must be written against the host's own version, not a copy of it",
  );
});

test("handshake: a page announcing this host's version is accepted", () => {
  const ready = { [BRIDGE_TAG]: "frame-ready", protocol: PLUGIN_PAGE_PROTOCOL };
  assert.ok(isBridgeFrameReady(ready), "the shape must be recognized at all");
  assert.deepEqual(pluginPageHandshake(ready), { status: "accepted" });
});

test("handshake: a page that announces no version is refused as missing, never guessed at", () => {
  // The mutation this guards is exactly "be lenient with an old page": treating
  // a missing field as compatible would let a page that never implemented the
  // handshake run against a protocol it was not written for.
  assert.equal(
    pluginPageHandshake({ [BRIDGE_TAG]: "frame-ready" }).status,
    "missing",
    "an absent protocol field is a refusal, not an implicit v1",
  );
  // A version that is not a number at all is the same refusal.
  for (const protocol of ["1", null, {}, Number.NaN, Number.POSITIVE_INFINITY]) {
    assert.equal(
      pluginPageHandshake({ [BRIDGE_TAG]: "frame-ready", protocol }).status,
      "missing",
      `protocol=${JSON.stringify(protocol)} must not pass as a version`,
    );
  }
  // And a message that is not a handshake at all — including a page that
  // starts straight into `invoke` — is refused too.
  assert.equal(pluginPageHandshake({ [BRIDGE_TAG]: "invoke", id: 1, command: "x" }).status, "missing");
  assert.equal(pluginPageHandshake(null).status, "missing");
});

test("handshake: a page announcing another version is refused as mismatch, with that version", () => {
  const verdict = pluginPageHandshake({ [BRIDGE_TAG]: "frame-ready", protocol: 2 });
  assert.deepEqual(verdict, { status: "mismatch", protocol: 2 });
});

test("the verdict tracks the host's number, not a hardcoded 1", () => {
  // Playing a future host: the same page message flips from mismatch to
  // accepted when the host moves. A hardcoded `=== 1` inside the predicate
  // would leave protocol 2 red forever and is what this locks out.
  const v2 = { [BRIDGE_TAG]: "frame-ready", protocol: 2 };
  assert.equal(pluginPageHandshake(v2, 1).status, "mismatch");
  assert.equal(pluginPageHandshake(v2, 2).status, "accepted");
  assert.equal(pluginPageHandshake(v2, 3).status, "mismatch");
});

test("a refused handshake names both protocol numbers, and its copy lives in the dictionary", async () => {
  const mismatch = handshakeErrorDetail({ status: "mismatch", protocol: 4 });
  assert.equal(mismatch.key, "plugin.protocolMismatch");
  assert.deepEqual(mismatch.params, { page: 4, host: PLUGIN_PAGE_PROTOCOL });
  const missing = handshakeErrorDetail({ status: "missing" });
  assert.equal(missing.key, "plugin.protocolMissing");
  assert.deepEqual(missing.params, { host: PLUGIN_PAGE_PROTOCOL });

  // The two variants must not be the same sentence, and both must actually
  // interpolate the numbers they carry — otherwise "which side is older?" is
  // unanswerable from the UI.
  for (const language of ["en", "zh"] as const) {
    const t = createTranslator(language);
    const mismatchText = t(mismatch.key, mismatch.params);
    const missingText = t(missing.key, missing.params);
    assert.notEqual(mismatchText, missingText, `${language}: the two refusals must read differently`);
    assert.ok(mismatchText.includes("4"), `${language}: the page's version must appear`);
    assert.ok(
      mismatchText.includes(String(PLUGIN_PAGE_PROTOCOL)),
      `${language}: the host's version must appear`,
    );
    assert.ok(
      missingText.includes(String(PLUGIN_PAGE_PROTOCOL)),
      `${language}: a missing handshake must still name the supported version`,
    );
    assert.ok(!/\{\w+\}/.test(mismatchText + missingText), `${language}: no unresolved placeholders`);
  }

  // A host that paints a bare "failed to load" is not enough: the detail line
  // is what a page author debugs from.
  const host = await read("src/plugins/PluginPageHost.tsx");
  assert.match(host, /handshakeErrorDetail\(handshake!, PLUGIN_PAGE_PROTOCOL\)/);
  assert.match(host, /plugin-page-host__error-detail/);
  // The refused branch has to be *reachable*. It cannot sit after the
  // `src ? <iframe>` arm: a loaded-but-refused page has a truthy `src` (the
  // frame stays mounted, hidden), so an `if/else` chain would never get there
  // and the user would see a blank iframe with no explanation. The verdict
  // therefore renders as its own sibling arm, and the frame is hidden by
  // `refused` rather than by being absent.
  assert.match(host, /const refused = handshake !== null && handshake\.status !== "accepted";/);
  assert.match(host, /\{refused && src \? \(/, "the refusal must render outside the src conditional");
  assert.match(host, /style=\{refused \? \{ display: "none" \} : undefined\}/, "the refused frame must be hidden, not left painting");
});

// ── 2 · the document's message table ──────────────────────────────────────

type Row = { direction: "page" | "host"; name: string; payload: string; when: string };

/** Parse the message table out of the protocol document. Rows are the lines
 * whose second cell is a backticked message name; the direction column is the
 * Chinese `page → host` / `host → page` prose the document uses. */
const messageTable = (doc: string): Row[] => {
  const rows: Row[] = [];
  for (const line of doc.split("\n")) {
    if (!line.startsWith("|")) continue;
    // `\|` inside a cell is an escaped pipe (the payload columns use it for
    // union types like `ok: true \| false`). Hide them before splitting or
    // the cell boundary is wrong and half the keys go missing.
    const cells = line
      .replace(/\\\|/g, "\u0000")
      .split("|")
      .slice(1, -1)
      .map((cell) => cell.trim().replace(/\u0000/g, "|"));
    if (cells.length < 4) continue;
    const [direction, name, payload, when] = cells;
    const match = /^`([a-z-]+)`$/.exec(name);
    if (!match) continue;
    const fromPage = direction === "page → host";
    const fromHost = direction === "host → page";
    assert.ok(fromPage || fromHost, `unknown direction "${direction}" for ${name}`);
    rows.push({
      direction: fromPage ? "page" : "host",
      name: match[1],
      payload,
      when,
    });
  }
  return rows;
};

test("the document's message table lists every protocol message exactly once", async () => {
  const rows = messageTable(await read(DOC));
  const documented = rows.map((row) => row.name).sort();
  const declared = [
    ...Object.values(example.TO_HOST),
    ...Object.values(example.FROM_HOST),
  ].sort();

  assert.deepEqual(
    documented,
    declared,
    "the table and the example's vocabulary must be the same set of names",
  );
  assert.equal(new Set(documented).size, documented.length, "a message may not appear twice");
  assert.equal(rows.length, 12, "the protocol 1 table has twelve rows");
});

test("each row's direction matches the side the example sends it from", async () => {
  const rows = messageTable(await read(DOC));
  const sentByPage = new Set(Object.values(example.TO_HOST));
  const sentByHost = new Set(Object.values(example.FROM_HOST));
  for (const row of rows) {
    const expected = sentByPage.has(row.name) ? "page" : "host";
    assert.equal(row.direction, expected, `${row.name} is documented in the wrong direction`);
    assert.ok(row.when.length > 0, `${row.name} must say when it is sent`);
    // And the two sets must not overlap: one tag cannot travel both ways.
    assert.ok(
      !(sentByPage.has(row.name) && sentByHost.has(row.name)),
      `${row.name} must have exactly one direction`,
    );
  }
});

/** The keys a payload cell advertises, and whether each is optional. */
const payloadKeys = (cell: string) =>
  [...cell.matchAll(/([A-Za-z][A-Za-z0-9]*)(\??)\s*:/g)].map((match) => ({
    name: match[1],
    optional: match[2] === "?",
  }));

test("the example's payloads carry the keys the table documents, and nothing else", async () => {
  const doc = await read(DOC);
  const rows = new Map(messageTable(doc).map((row) => [row.name, row]));

  // Built with every optional field present, so "documented but never sent"
  // is caught as well as "sent but undocumented".
  const sent = {
    "frame-ready": example.frameReady(),
    invoke: example.invoke(7, "my_command", { a: 1 }, "session-token"),
    close: example.close(),
    drag: example.drag(),
    "host-notify": example.hostNotify(3, "success", "plugin.exampleNotify", true),
  } as const;

  for (const [name, message] of Object.entries(sent)) {
    const row = rows.get(name);
    assert.ok(row, `${name} must have a table row`);
    const documented = payloadKeys(row.payload);
    const actual = Object.keys(message).filter((key) => key !== BRIDGE_TAG);
    for (const key of actual) {
      assert.ok(
        documented.some((entry) => entry.name === key),
        `${name}: the example sends "${key}", which the table does not document`,
      );
    }
    for (const entry of documented.filter((candidate) => !candidate.optional)) {
      assert.ok(
        actual.includes(entry.name),
        `${name}: the table requires "${entry.name}", which the example does not send`,
      );
    }
  }

  // The host's own predicates are the second half of "the example is correct":
  // every payload it builds must be recognized as the message it claims to be.
  assert.ok(isBridgeFrameReady(sent["frame-ready"]));
  assert.ok(isBridgeRequest(sent.invoke));
  assert.ok(isBridgeClose(sent.close));
  assert.ok(isBridgeDrag(sent.drag));
  assert.ok(isBridgeNotify(sent["host-notify"]));
});

test("the document covers the four things a page author has to know", async () => {
  const doc = await read(DOC);

  // (a) the handshake is gate, not convention
  assert.match(doc, /frame-ready/, "the handshake message must be named");
  assert.match(doc, /握手/, "…and explained in the document's own language");
  // (b) the WebKit colour-channel trap
  assert.match(doc, /WebKit/, "the WebKit rgba() trap must be documented");
  assert.match(doc, /颜色通道/, "…naming the colour-channel slot specifically");
  assert.match(doc, /backdrop-filter/, "…and the useless-in-a-sandbox blur");
  // (c) the drag bridge
  assert.match(doc, /isClipboardDragTarget|DRAG_EXEMPT/, "the drag-bridge pattern must be shown");
  assert.match(doc, /data-no-drag/, "…including the opt-out for scrollers");
  // (d) the runtime floor
  assert.match(doc, /Tauri|WebView/, "the minimum runtime must be stated");
  assert.match(doc, /不透明源|opaque origin/, "…and what the sandbox forbids");
  // The examples directory is pointed at, so the document is not a dead end.
  assert.match(doc, /examples\/hello-page\//);
});

// ── 3 · the example page files ────────────────────────────────────────────

test("the example is loadable with no build step and demonstrates the four duties", async () => {
  const html = await read("docs/extensions/examples/hello-page/index.html");
  const main = await read("docs/extensions/examples/hello-page/main.js");
  const css = await read("docs/extensions/examples/hello-page/page.css");
  const readme = await read("docs/extensions/examples/hello-page/README.md");

  // Plain modules loaded relative to the page: no bare specifiers, no build.
  assert.match(html, /<script type="module" src="\.\/main\.js"><\/script>/);
  assert.match(html, /href="\.\/page\.css"/);
  assert.ok(!/from\s+"[^./]/.test(main), "no bare imports — this must run unbundled");
  assert.match(main, /from "\.\/protocol\.js"/);

  // 1 handshake, 2 opacity band, 3 drag region, 4 a message, 5 close.
  assert.match(main, /postMessage\(frameReady\(\), "\*"\)/, "the handshake must be sent");
  assert.match(main, /applyOpacity\(/, "the opacity band must be consumed");
  assert.match(main, /DRAG_EXEMPT/, "the interactive-element exemption must exist");
  assert.match(main, /hostNotify\(/, "a host toast must be sent");
  assert.match(main, /postMessage\(close\(\), "\*"\)/, "close must be wired");
  // The two live channels: bootstrap params AND the message, both required.
  assert.match(main, /params\.get\("terminal-opacity"\)/);
  assert.match(main, /FROM_HOST\.opacity/);

  // The CSS rule the document warns about, demonstrated in the right form.
  assert.match(css, /rgba\(17, 18, 20, var\(--field-alpha\)\)/);
  assert.ok(
    !/rgba\(var\(/.test(css),
    "the example must not put a var() in the colour-channel slot",
  );

  // The README tells a page author how to load it.
  assert.match(readme, /mock-host\.html/, "the browser-only path must be documented");
  assert.match(readme, /rollupOptions|vite\.config\.ts/, "the in-app path must be documented");
  assert.match(readme, /DESCRIPTORS|plugin_pages\.rs/, "…including the host registry entry");
});

// ── 4 · the built-in page is the protocol's first consumer ─────────────────

test("the built-in clipboard page sends the same handshake, from the shared constant", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  assert.match(
    page,
    /\{ \[BRIDGE_TAG\]: "frame-ready", protocol: PLUGIN_PAGE_PROTOCOL \}/,
    "the first consumer must announce the protocol like any other page",
  );
  assert.match(
    page,
    /import \{[^}]*PLUGIN_PAGE_PROTOCOL[^}]*\} from "\.\.\/\.\.\/plugin-pages"/,
    "and read the version from the shared constant, never a literal 1",
  );
  assert.ok(
    !/protocol: 1/.test(page),
    "a literal version in the page is how the two sides drift apart",
  );
});

test("the host gates the bridge on the handshake, and paints a refused one", async () => {
  const host = await read("src/plugins/PluginPageHost.tsx");
  // The handshake is recognized, decided once per document, and every message
  // after it is gated on the verdict.
  assert.match(host, /if \(isBridgeFrameReady\(data\)\)/);
  assert.match(host, /pluginPageHandshake\(data, PLUGIN_PAGE_PROTOCOL\)/);
  assert.match(host, /setHandshake\(\(current\) => current \?\?/);
  assert.match(host, /handshakeRef\.current\?\.status !== "accepted"\) return;/);
  assert.match(host, /HANDSHAKE_TIMEOUT_MS/);
  // Settings are only pushed into an accepted page: the three bootstrap
  // pushes (opacity/theme/glass) and the visibility push all start with it.
  const readyGates = host.match(/if \(!ready\) return;/g) ?? [];
  assert.ok(readyGates.length >= 5, `expected the ready gate across the push effects, saw ${readyGates.length}`);
  assert.match(host, /const ready = handshake\?\.status === "accepted";/);
  // A refused page is never left as a blank iframe: it gets the retry state.
  assert.match(host, /plugin\.pageError/);
  assert.match(host, /onClick=\{\(\) => \{ setDescriptor\(null\); setReloadNonce/);

  // The document-change reset must happen during *render*, in the same commit
  // that changes the iframe's `src`, and never inside `onLoad`: a page's own
  // script runs before the iframe's `load` event, so its `frame-ready` can
  // already have been processed by then. Resetting in `handleFrameLoad` wipes
  // that verdict and the document then times out looking like it never spoke.
  assert.match(
    host,
    /if \(handshakeSrc !== src\) \{\s*\n\s*setHandshakeSrc\(src\);\s*\n\s*setHandshake\(null\);/,
    "a new src must invalidate the verdict during render",
  );
  const loadStart = host.indexOf("const handleFrameLoad");
  const loadBody = host.slice(loadStart, host.indexOf("}, []);", loadStart));
  assert.ok(loadStart > -1 && loadBody.length > 0, "handleFrameLoad must exist");
  assert.ok(
    !/setHandshake\(null\)/.test(loadBody),
    "onLoad must not clear the verdict — the page may already have handshaken",
  );
  // It may only *arm* the missing-verdict deadline, and only in a form that
  // cannot overwrite a verdict that already exists.
  assert.match(loadBody, /setHandshake\(\(current\) => current \?\? \{ status: "missing" \}\)/);
});

// ── 5 · mutation locks ────────────────────────────────────────────────────

/** Load a copy of the example's vocabulary with `mutate` applied, so the two
 * mutations below exercise the real source rather than a re-typed payload. */
const loadMutatedExample = async (mutate: (source: string) => string) => {
  const source = await readFile(new URL(EXAMPLE, root), "utf8");
  const mutated = mutate(source);
  assert.notEqual(mutated, source, "the mutation must land on the source");
  const dir = await mkdtemp(join(tmpdir(), "floter-hello-page-"));
  try {
    const file = join(dir, "protocol.js");
    await writeFile(file, mutated);
    return await import(`${pathToFileURL(file).href}?v=${process.hrtime.bigint()}`);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
};

test("mutation lock: dropping the protocol field from the handshake goes red as missing", async () => {
  const mutated = await loadMutatedExample((source) => {
    const next = source.replace("  [TAG]: TO_HOST.frameReady,\n  protocol,\n", "  [TAG]: TO_HOST.frameReady,\n");
    assert.match(next, /\[TAG\]: TO_HOST\.frameReady/);
    return next;
  });
  const ready = mutated.frameReady();
  assert.deepEqual(Object.keys(ready), [BRIDGE_TAG], "the mutation must leave a bare frame-ready");
  assert.equal(isBridgeFrameReady(ready), false, "the fieldless handshake must not pass the shape check");
  assert.equal(
    pluginPageHandshake(ready).status,
    "missing",
    "dropping the version field must be refused as missing",
  );
});

test("mutation lock: bumping the example to protocol 2 goes red as mismatch", async () => {
  const mutated = await loadMutatedExample((source) =>
    source.replace("export const PROTOCOL = 1;", "export const PROTOCOL = 2;"),
  );
  assert.equal(mutated.PROTOCOL, 2, "the mutation must bump the version");
  const ready = mutated.frameReady();
  assert.equal(isBridgeFrameReady(ready), true, "the shape is still a handshake…");
  assert.deepEqual(
    pluginPageHandshake(ready),
    { status: "mismatch", protocol: 2 },
    "…but a version the host does not speak must be refused, with the page's number",
  );
  // And the refusal is what the error state renders, in both languages.
  const detail = handshakeErrorDetail(pluginPageHandshake(ready));
  assert.deepEqual(detail.params, { page: 2, host: PLUGIN_PAGE_PROTOCOL });
});
