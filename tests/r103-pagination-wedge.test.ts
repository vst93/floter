// R103 · the launcher's page-append can never wedge on a frame again.
//
// R72 recorded the fact this round is built on: a panel that is not the active
// app stops firing `requestAnimationFrame` altogether — the normal state of
// this accessory panel, and the state the user is looking at while they scroll
// and click. R95 fixed the detached plugin window's load-more trigger by making
// its append synchronous, but the launcher's copy of the same trigger
// (`useLauncherCatalog.ts` · `loadMorePluginPage`) was left on a frame. The
// frame never came, so the in-flight ref stayed `true`, the scroller was refused
// every later page, and the footer stayed on its loading line: a permanent
// wedge, not a missed scroll.
//
// The repair is the detached window's own mechanism — the append lands in the
// beat the trigger arrives in, and the in-flight ref is released by the render
// the increment causes, never by a clock — so the two lists cannot drift. The
// one second-clock use in this round is elsewhere: the ExtensionsPanel
// deep-link register scroll measures the DOM *after* the row is painted, so it
// keeps its frame but gains a short timer fallback, exactly the launcher's R72
// beat. The node runner has no DOM, so both halves are pinned at the source,
// the way the neighbouring R95 and R34 suites do.
//
// Mutations this file exists to catch:
//   * wrapping the append back in `window.requestAnimationFrame(…)` — the wedge
//     returns and the no-frame guard goes red;
//   * releasing the in-flight ref from a clock instead of the render — the
//     release guard goes red;
//   * giving the launcher a second scheduling mechanism (a timer-only append)
//     that the detached window does not have — the no-clock guard goes red;
//   * dropping the register scroll's timer fallback and leaving the bare frame —
//     the both-clocks guard goes red;
//   * letting the two lists drift apart (one synchronous, one on a frame) — the
//     same-mechanism guard goes red.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** The body of a `const loadMorePluginPage = useCallback(() => { … }, []);`. */
const loadMoreBody = (source: string): string => {
  const match = source.match(
    /const loadMorePluginPage = useCallback\(\(\) => \{([\s\S]*?)\n  \}, \[\]\);/,
  );
  assert.ok(match, "the source still declares `loadMorePluginPage` as a stable callback");
  return match![1];
};

// ── A · the launcher's append has no clock to stall on ────────────────────

test("R103 · the launcher's load-more appends in memory, with no frame", async () => {
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  const body = loadMoreBody(catalog);
  // The wedge: a frame that a non-active panel never fires. The append must not
  // wait for one — it is a memory step with nothing to defer.
  assert.doesNotMatch(
    body,
    /requestAnimationFrame/,
    "R103 · no `requestAnimationFrame`: an inactive panel stops firing it, and the ref would stay `true` forever",
  );
  // And it must not grow a *second* clock the detached window does not have:
  // one mechanism for both lists, not two.
  assert.doesNotMatch(
    body,
    /setTimeout/,
    "R103 · no timer either: the append is synchronous, not deferred to another clock",
  );
  assert.match(
    body,
    /setPluginPages\(\(pages\) => pages \+ 1\)/,
    "the page lands in the same beat the trigger arrived in",
  );
});

test("R103 · the in-flight ref is released by the render, not by a clock", async () => {
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  // The increment is the release signal — the same effect the detached window
  // uses — so a stalled frame can never hold the ref open.
  assert.match(
    catalog,
    /useEffect\(\(\) => \{\s*pluginLoadingRef\.current = false;[\s\S]{0,80}?\}, \[pluginPages\]\)/,
    "the ref is released when the appended page renders",
  );
  // The guard itself is unchanged: one append in flight, and none when the list
  // is complete.
  assert.match(
    catalog,
    /if \(pluginLoadingRef\.current \|\| !pluginHasMoreRef\.current\) return;/,
    "the one-in-flight guard stays",
  );
});

// ── B · the footer's loading line survives, on the render's clock ─────────

test("R103 · the footer's loading line is raised and lowered, never stranded", async () => {
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  // The line is kept: the launcher's footer has a loading state, and the append
  // is synchronous, so it is raised with the increment and lowered by the same
  // render-triggered effect. It lives for exactly the commit the increment
  // paints — no frame in between to be stranded on.
  assert.match(catalog, /setPluginLoadingMore\(true\)/, "the line is raised with the append");
  assert.match(catalog, /setPluginLoadingMore\(false\)/, "and lowered again");
  // The footer still reads the protocol's own three-state rule with that bit.
  const renderer = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(
    renderer,
    /pluginFooterState\(pluginPage, pluginLoadingMore\)/,
    "the footer is still the protocol's rule, fed the loading bit",
  );
});

// ── C · one mechanism, not two: the detached window's own ─────────────────

test("R103 · both lists page through the identical synchronous mechanism", async () => {
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  const detached = stripJsComments(await read("src/plugin-window/DetachedPluginApp.tsx"));
  const catalogBody = loadMoreBody(catalog);
  const detachedBody = loadMoreBody(detached);
  // The R95 guard stays green: the detached append is still synchronous.
  assert.doesNotMatch(
    detachedBody,
    /requestAnimationFrame/,
    "R95 · the detached window's append is still synchronous",
  );
  // Both append exactly one page, the same way.
  assert.match(catalogBody, /setPluginPages\(\(pages\) => pages \+ 1\)/);
  assert.match(detachedBody, /setPluginPages\(\(pages\) => pages \+ 1\)/);
  // Both release the in-flight ref from the render the increment causes.
  assert.match(
    catalog,
    /useEffect\(\(\) => \{\s*pluginLoadingRef\.current = false;/,
    "the launcher releases the ref from the render",
  );
  assert.match(
    detached,
    /useEffect\(\(\) => \{\s*pluginLoadingRef\.current = false;\s*\}, \[pluginPages\]\)/,
    "the detached window releases the ref from the render",
  );
});

// ── D · the deep-link register scroll keeps its frame, gains a fallback ───

test("R103 · the register highlight is armed on both clocks", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // The frame is the right first clock here — the row was just rendered and one
  // frame lets the layout settle before it is measured — but an accessory panel
  // stops firing it, so the short timer is the fallback. The shape is the
  // launcher's R72 beat (`armMeasureBeat`): arm the frame under a `typeof`
  // guard, then always arm the timer.
  assert.match(
    panel,
    /if \(typeof requestAnimationFrame === "function"\) requestAnimationFrame\(fire\);\s*window\.setTimeout\(fire, 48\);/,
    "the scroll is armed on the frame clock and on a 48ms timer",
  );
  // The scroll itself is unchanged: it stays confined to the Detected list
  // (never `scrollIntoView`, which would drag `.settings-content` to the top)
  // and moves that list's own `scrollTop` only.
  const block = panel.slice(panel.indexOf("const fire = () => {"), panel.indexOf("window.setTimeout(fire, 48)"));
  assert.match(block, /querySelector<HTMLElement>\("\.extension-row--register"\)/);
  assert.match(block, /list\.scrollTop (?:\+=|-=)/, "the scroll is the list's own box");
  assert.doesNotMatch(block, /scrollIntoView/, "and never walks up the scrollable ancestors");
  // The fallback is the only frame clock left in the block: no bare
  // `window.requestAnimationFrame(…)` the `typeof` guard does not cover.
  assert.doesNotMatch(block, /window\.requestAnimationFrame\(/, "the frame call goes through the guarded beat");
});
