// R7-5 · one feedback channel: a plugin page's notice becomes the host's toast.
//
// The clipboard page used to paint its own notice bar (`.clipboard-panel__notice`,
// 5s, warning colour, docked under the list) while every surface in the host
// document raised toasts through `toast-state.ts` (8s error / 4s success, one
// stack, one position). Two lifetimes, two visuals, two positions for the same
// event — the exact split Raycast does not have. This round removes the page's
// channel and routes its failures over the bridge to the one stack.
//
// Three things are pinned here, all structural (the node suite has no DOM):
//
//   1. The bridge carries feedback as a *data* message with a dictionary key,
//      validated on both ends, and the host is the side that translates.
//   2. The page has no notice surface left and no way to paint one: no
//      container, no CSS rule, no `TOAST_DISMISS_MS`-parallel timer.
//   3. The three states (empty / loading / error) follow one shape, and the
//      loading one is inline — it must not replace a populated list.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  BRIDGE_TAG,
  FAILURE_NOTIFY_DEDUP_MS,
  RETRY_REGISTRY_CAPACITY,
  createFailureDeduper,
  createRetryRegistry,
  isBridgeNotify,
  isBridgeNotifyRetry,
} from "../src/plugin-pages.ts";
import { isMessageKey } from "../src/i18n.ts";
import { MAX_TOASTS, TOAST_DISMISS_MS } from "../src/toast-state.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const code = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim(), body: match[2] });
  }
  return out;
};

// ── 1 · the bridge message ────────────────────────────────────────────────

test("host-notify is recognized with an id, a kind, a dictionary key and an optional retry flag", () => {
  assert.ok(
    isBridgeNotify({ [BRIDGE_TAG]: "host-notify", id: 1, kind: "error", messageKey: "clipboard.copyFailed" }),
  );
  assert.ok(
    isBridgeNotify({
      [BRIDGE_TAG]: "host-notify",
      id: 42,
      kind: "success",
      messageKey: "launcher.historyCleared",
      retryable: true,
    }),
  );
  for (const bad of [
    // No tag, wrong tag.
    { id: 1, kind: "error", messageKey: "clipboard.copyFailed" },
    { [BRIDGE_TAG]: "notify", id: 1, kind: "error", messageKey: "clipboard.copyFailed" },
    // The correlation id is required and numeric: it is what a retry replies
    // with, so a message without one can never be answered unambiguously.
    { [BRIDGE_TAG]: "host-notify", kind: "error", messageKey: "clipboard.copyFailed" },
    { [BRIDGE_TAG]: "host-notify", id: "1", kind: "error", messageKey: "clipboard.copyFailed" },
    { [BRIDGE_TAG]: "host-notify", id: Number.NaN, kind: "error", messageKey: "clipboard.copyFailed" },
    // Kind is one of the two the stack knows, nothing else.
    { [BRIDGE_TAG]: "host-notify", id: 1, kind: "warning", messageKey: "clipboard.copyFailed" },
    { [BRIDGE_TAG]: "host-notify", id: 1, messageKey: "clipboard.copyFailed" },
    // The key has to look like a dictionary key: no markup, no prose, no
    // unbounded string. A page must not be able to paint text as host chrome.
    { [BRIDGE_TAG]: "host-notify", id: 1, kind: "error", messageKey: "<b>hi</b>" },
    { [BRIDGE_TAG]: "host-notify", id: 1, kind: "error", messageKey: "has space" },
    { [BRIDGE_TAG]: "host-notify", id: 1, kind: "error", messageKey: "x".repeat(200) },
    { [BRIDGE_TAG]: "host-notify", id: 1, kind: "error", messageKey: "" },
    { [BRIDGE_TAG]: "host-notify", id: 1, kind: "error", messageKey: 7 },
    // Retryable is a boolean when present.
    { [BRIDGE_TAG]: "host-notify", id: 1, kind: "error", messageKey: "clipboard.copyFailed", retryable: "yes" },
    null,
    "host-notify",
  ]) {
    assert.equal(isBridgeNotify(bad), false, JSON.stringify(bad));
  }
});

test("the host→page retry reply carries the id of the toast it answers", () => {
  assert.ok(isBridgeNotifyRetry({ [BRIDGE_TAG]: "notify-retry", id: 7 }));
  // The id is the whole point: without it three coexisting retryable toasts
  // would each mean "the newest failure".
  assert.equal(isBridgeNotifyRetry({ [BRIDGE_TAG]: "notify-retry" }), false);
  assert.equal(isBridgeNotifyRetry({ [BRIDGE_TAG]: "notify-retry", id: "7" }), false);
  assert.equal(
    isBridgeNotifyRetry({ [BRIDGE_TAG]: "host-notify", id: 1, kind: "error", messageKey: "a" }),
    false,
  );
  assert.equal(isBridgeNotifyRetry({ [BRIDGE_TAG]: "reload" }), false);
  assert.equal(isBridgeNotifyRetry(null), false);
});

test("the host resolves the page's key against its own dictionary and drops unknown keys", () => {
  // The key the page names has to exist in the host's table; `isMessageKey` is
  // what keeps a page from naming something that does not.
  assert.ok(isMessageKey("clipboard.copyFailed"));
  assert.ok(isMessageKey("clipboard.deleteFailed"));
  assert.ok(isMessageKey("clipboard.clearFailed"));
  assert.ok(isMessageKey("clipboard.favoriteFailed"));
  assert.ok(isMessageKey("plugin.retry"));
  assert.equal(isMessageKey("clipboard.actionFailed"), false, "the single generic string is gone");
  assert.equal(isMessageKey("totally.made.up"), false);
  assert.equal(isMessageKey("__proto__"), false);
  assert.equal(isMessageKey("constructor"), false);
});

test("the host raises the page's feedback on the app stack, one pipeline", async () => {
  const host = await read("src/plugins/PluginPageHost.tsx");
  assert.match(host, /if \(isBridgeNotify\(data\)\)/, "the host must handle host-notify");
  assert.match(host, /if \(!isMessageKey\(data\.messageKey\)\) return;/, "unknown keys are dropped, never painted");
  assert.match(host, /notifyRef\.current\(/, "the toast goes through App's own notify");
  // The retry action is a host-side closure: the toast carries a button, and
  // pressing it messages the page — the page is the only side that knows what
  // to re-run.
  assert.match(host, /label: translate\("plugin\.retry"\)/, "the retry button is labelled from the host dictionary");
  assert.match(
    host,
    /const retry: BridgeNotifyRetry = \{ \[BRIDGE_TAG\]: "notify-retry", id: notifyId \}/,
    "the retry reply echoes the page's own id; the host never mints one",
  );
  assert.match(host, /const notifyId = data\.id;/, "the host reads the page's id off the message");
  // The property the whole round is about: not a host-side replay. The host
  // has no clipboard action to re-run — it only forwards the intent.
  assert.ok(
    !/invokeCommand|clipboard_copy_entry|clipboard_delete/.test(host),
    "the host must not replay the page's command; it only relays the retry intent",
  );
  const app = await read("src/App.tsx");
  assert.match(app, /onNotify=\{notify\}/, "App hands its notify to the plugin host");
});

test("App's notify grew an optional action slot and the plugin layer is untouched", async () => {
  const app = await read("src/App.tsx");
  assert.match(
    app,
    /const notify = useCallback\(\s*\(kind: ToastKind, text: string, action\?: ToastAction\)/,
    "notify must accept an optional action without changing its existing call shape",
  );
  // Keep-alive hard red line: the layer is still the first sibling in all four
  // modes, the toast host second, and the host's own props are the only change.
  assert.equal((app.match(/\{pluginLayer\}/g) ?? []).length, 4);
  const start = app.indexOf("const pluginLayer = (");
  const jsx = code(app.slice(start, app.indexOf("const toastHost = (", start)));
  assert.ok(!/\bkey=/.test(jsx), "the plugin layer must stay unkeyed");
  assert.ok(
    !/<ToastHost|TOAST_PORTAL_ID|floter-app-toasts/.test(jsx),
    "the toast host must not move inside the plugin layer",
  );
});

// ── 2 · the page's notice is gone, at the source ──────────────────────────

test("the clipboard page has no notice surface left — markup, styles and timer", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  const clean = code(page);
  // The self-painted notice and its 5s lifetime are deleted, not commented out.
  assert.ok(!clean.includes("clipboard-panel__notice"), "the in-page notice container must be gone");
  assert.ok(!/\bnoticeTimer\b/.test(clean), "the page must not keep its own dismiss timer");
  assert.ok(!/\bshowError\b/.test(clean), "the notice helper must be gone");
  assert.ok(
    !/hidden = true/.test(clean),
    "the old 'hide the notice after 5s' write must not survive in another form",
  );
  // Every failure goes over the bridge instead. The four action paths each
  // name their own message key, so the toast says what actually failed.
  for (const key of [
    "clipboard.copyFailed",
    "clipboard.deleteFailed",
    "clipboard.clearFailed",
    "clipboard.favoriteFailed",
    "clipboard.loadFailed",
  ]) {
    assert.ok(page.includes(key), `the page must raise ${key}`);
  }
  assert.match(page, /NOTIFY\b|host-notify/, "the page must speak the bridge's notify message");
  assert.match(
    page,
    /const notifyFailure = \(messageKey: string, onRetry\?: \(\) => void\) => \{/,
    "the page's failures must funnel through one bridge helper",
  );
  // And the page never writes host DOM (it cannot) — no window.parent.document.
  assert.ok(
    !/parent\.document|top\.document/.test(clean),
    "a sandboxed page reaches the host only through postMessage",
  );
});

test("clipboard/page.css removed the notice rules and added tri-state styles (net +40)", async () => {
  // N4: this test asserts *which* rules exist, not the diff direction. The
  // former name ("only shrank") was false: the notice's 9 lines went away and
  // the new empty/loading/spinner rules added ~49, a net increase. What
  // matters is that no replacement *feedback surface* was reintroduced.
  const css = stripComments(await read("src/plugins/clipboard/page.css"));
  assert.ok(
    !css.includes("clipboard-panel__notice"),
    "the notice rule must be deleted from the page sheet, not emptied",
  );
  // The three states that remain are the shared shape, and the loading one is
  // a row — not a block that replaces the list.
  const loading = rules(css).find(({ selector }) => selector === ".clipboard-panel__loading");
  assert.ok(loading, "the page must define its inline loading row");
  assert.match(loading!.body, /display:\s*flex/, "the loading state is an inline row");
  assert.match(loading!.body, /min-height:\s*44px/, "the loading row is sized, so its arrival does not shift layout");
  const spinner = rules(css).find(({ selector }) => selector === ".clipboard-panel__spinner");
  assert.ok(spinner, "the loading row has its spinner");
  assert.ok(!/backdrop-filter/.test(css), "the page still declares zero backdrop-filters");
  assert.ok(!/blur\(\s*[\d.]+px/.test(css), "the page still declares zero blur literals");
});

test("the three states share one shape: a title, a why, and a control slot", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  const css = stripComments(await read("src/plugins/clipboard/page.css"));
  // Empty: title + explanation.
  assert.match(page, /clipboard-panel__empty-title/, "the empty state carries a title element");
  assert.match(page, /const hint = scoped/, "the empty state carries an explanation line");
  assert.ok(page.includes("clipboard.emptyHint"), "the empty state explains what to do");
  // Error: a title, a why, and the retry/dismiss slot.
  assert.match(page, /const renderLoadFailure = \(\): DocumentFragment/, "the failure state is its own builder");
  assert.match(page, /clipboard-panel__empty-actions/, "the failure state has a control slot");
  assert.match(page, /t\("clipboard\.retry"\)/, "the failure state offers a retry");
  assert.match(page, /t\("clipboard\.dismiss"\)/, "the failure state offers a dismissal");
  const actions = rules(css).find(({ selector }) => selector === ".clipboard-panel__empty-actions");
  assert.ok(actions, "the control slot must be styled");
  assert.match(actions!.body, /display:\s*flex/);
});

test("the loading state is gated on having nothing on screen yet", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  // `loaded` flips only with a completed fetch (success or failure), so a
  // background poll over a populated list never repaints the spinner over it.
  assert.match(page, /let loaded = false;/, "the page must track whether the first fetch landed");
  assert.match(page, /if \(!loaded\) \{/, "the loading branch is gated on `!loaded`");
  assert.match(page, /loaded = true;/, "both the success and failure paths must clear it");
  assert.ok(
    (page.match(/loaded = true;/g) ?? []).length >= 2,
    "a failed first fetch has to clear the loading state too, or the spinner would spin forever",
  );
  // Re-rendering during the load must not restart the spinner animation: the
  // row is only built when it is not already the content.
  assert.match(
    page,
    /if \(!content\.querySelector\("\.clipboard-panel__loading"\)\) \{/,
    "an in-flight repaint must reuse the existing spinner node",
  );
  // The failure state likewise survives repaints rather than being rebuilt.
  assert.match(page, /data-failure-state/, "the failure state is marked so a repaint can recognise it");
});

test("a retry that fails again is still pressable (the failure node is reused, never locked)", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  const clean = code(page);
  // The failure node survives repaints (that is the whole reason its
  // `data-failure-state` exists), so a `retry.disabled = true` set inside the
  // click handler outlives the click: if the retry also fails, the node is
  // reused with its flag still set and the user can never retry a second
  // time. The button's semantics are the data's, so no flag is carried at all
  // — re-entrancy is already handled by `reload()`'s in-flight dedupe.
  assert.ok(
    !/retry\.disabled/.test(clean),
    "the retry button must not be disabled by hand: it outlives the click on the reused node",
  );
  // Pin the reuse path that made the flag fatal, so a future "fix" cannot
  // re-add the flag against a node that is no longer reused.
  assert.match(
    page,
    /painted\?\.dataset\.failureState !== String\(backendUnavailable\)/,
    "the failure node must be reused when the failure kind is unchanged",
  );
  // And the guard against a double-click storm is the reload dedupe, not a
  // disabled flag.
  assert.match(
    page,
    /const reload = \(\): Promise<void> => \{\s*\n\s*if \(pageDisposed \|\| !pageVisible\) return Promise\.resolve\(\);\s*\n\s*if \(reloadPending\) return reloadPending;/,
    "a second press must be absorbed by the in-flight dedupe instead of a disabled flag",
  );
});

// ── 2b · a persistent outage raises one toast, not one per poll ───────────

test("five consecutive poll failures raise exactly one toast (30s dedupe per key)", () => {
  // Drive the page's own dedupe policy directly rather than reading source:
  // the 2s poll calls this once per failed poll, and the burst below is 20s of
  // outage. Only the first may paint; the other four must be swallowed.
  assert.ok(
    FAILURE_NOTIFY_DEDUP_MS >= 30_000,
    "the window must dwarf the 2s poll — at 30s a poll can earn at most one toast",
  );
  const deduper = createFailureDeduper();
  const raised: number[] = [];
  let now = 1_000_000;
  for (let poll = 0; poll < 5; poll += 1) {
    if (deduper.allow("clipboard.loadFailed", now)) raised.push(now);
    now += 2000; // startPeriodicRefresh's interval
  }
  assert.deepEqual(raised, [1_000_000], "the outage must be one toast, not five");
  // The window is per key: a *different* failure in the middle of the outage
  // is still news and is not swallowed by the load key.
  assert.equal(deduper.allow("clipboard.copyFailed", now), true, "dedupe must be per message key");
  // Recovery re-arms: failure → success → failure is two toasts, not one.
  deduper.clear("clipboard.loadFailed");
  assert.equal(deduper.allow("clipboard.loadFailed", now), true, "a success in between makes the relapse news again");
  // And the window really elapses: the next failure after it is announced.
  const later = createFailureDeduper();
  assert.equal(later.allow("k", 0), true);
  assert.equal(later.allow("k", FAILURE_NOTIFY_DEDUP_MS - 1), false);
  assert.equal(later.allow("k", FAILURE_NOTIFY_DEDUP_MS), true);
});

test("the dedupe is wired to the background poll, not to user-gesture failures", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  // The poll's catch funnels through the deduper...
  assert.match(
    page,
    /if \(loadFailureDeduper\.allow\("clipboard\.loadFailed"\)\) \{\s*\n[\s\S]*?notifyFailure\("clipboard\.loadFailed"/,
    "the load-failure toast must be gated on the deduper",
  );
  // ...and a successful load clears the key so a relapse is announced again.
  assert.match(
    page,
    /loadFailureDeduper\.clear\("clipboard\.loadFailed"\)/,
    "a good poll must re-arm the outage toast",
  );
  // The toast's own Retry is a user gesture: pressing it re-arms the window so
  // a retry that fails again reports, instead of being swallowed by the stale
  // 30s entry the user is looking at.
  assert.match(
    page,
    /notifyFailure\("clipboard\.loadFailed", \(\) => \{\s*\n\s*loadFailureDeduper\.clear\("clipboard\.loadFailed"\);\s*\n\s*void reload\(\);/,
    "the toast retry must re-arm the deduper before reloading",
  );
  // A copy/delete/clear failure is a direct answer to a click: it must report
  // every time, or the second click would fail silently. Those paths call
  // `notifyFailure` directly, with no deduper in front.
  for (const key of ["clipboard.copyFailed", "clipboard.deleteFailed", "clipboard.clearFailed"]) {
    assert.ok(page.includes(`notifyFailure("${key}"`), `${key} must stay undeduped`);
  }
});

// ── 2c · each toast's retry keeps its own action ──────────────────────────

test("an old toast's retry runs its own action, never the newest one", () => {
  // Three failures with three retry actions coexisting on the stack (the host
  // keeps MAX_TOASTS=3). The old single slot remembered only the newest, so
  // pressing toast A's Retry ran C's action. The registry is keyed by the id
  // the host echoes back.
  assert.equal(RETRY_REGISTRY_CAPACITY, MAX_TOASTS, "the registry must hold as many retries as the stack holds toasts");
  const registry = createRetryRegistry();
  const ran: string[] = [];
  registry.add(1, () => ran.push("copy"));
  registry.add(2, () => ran.push("delete"));
  registry.add(3, () => ran.push("favorite"));
  registry.run(1);
  assert.deepEqual(ran, ["copy"], "pressing the oldest toast must run its own action, not the newest");
  registry.run(3);
  registry.run(2);
  assert.deepEqual(ran, ["copy", "favorite", "delete"], "each id maps to the action that raised it");
  // A retry runs once: the toast is dismissed as it fires, so a duplicate
  // reply (a double click, a replayed message) must be a no-op.
  registry.run(1);
  assert.deepEqual(ran, ["copy", "favorite", "delete"], "a consumed retry must not run twice");
});

test("a retry for an id the page never issued — or evicted — is dropped, not rerouted", () => {
  const registry = createRetryRegistry(3);
  const ran: string[] = [];
  registry.add(10, () => ran.push("a"));
  registry.run(999);
  assert.deepEqual(ran, [], "an unknown id (a stale toast from a previous page life) must run nothing");
  // Insertion order is eviction order: the oldest falls off exactly as the
  // host's stack drops its oldest toast.
  registry.add(11, () => ran.push("b"));
  registry.add(12, () => ran.push("c"));
  registry.add(13, () => ran.push("d"));
  registry.run(10);
  assert.deepEqual(ran, [], "an evicted retry must be dropped, not fall back to a live action");
  registry.run(12);
  assert.deepEqual(ran, ["c"], "the still-registered retry works");
});

test("the page stamps every notification with an id and replies by that id", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  const clean = code(page);
  assert.match(clean, /let nextNotifyId = 1;/, "the page mints ids locally");
  assert.match(
    code(page),
    /const id = nextNotifyId\+\+;\s*\n\s*if \(onRetry\) retryRegistry\.add\(id, onRetry\);/,
    "each failure registers its thunk under its own id",
  );
  assert.match(
    code(page),
    /\{ \[BRIDGE_TAG\]: "host-notify", id, kind: "error", messageKey, retryable: Boolean\(onRetry\) \}/,
    "the id goes out on the wire",
  );
  assert.match(clean, /retryRegistry\.run\(data\.id\)/, "the reply is dispatched by the echoed id");
  assert.ok(
    !/pendingRetry|runPendingRetry/.test(clean),
    "the single pendingRetry slot must be gone, not shadowed",
  );
});

// ── 3 · one lifetime, one stack ───────────────────────────────────────────

test("the page has no lifetime of its own: the stack's table is the only one", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  const clean = code(page);
  // Any literal duration in the page that is not one of the page's own
  // (unrelated) UI timers would be a second toast lifetime. The two survivors
  // are the clear-confirmation arming window and the bridge timeout. Read the
  // second argument of each `setTimeout(…, delay)` by brace/paren depth, since
  // a callback argument contains parens of its own.
  const delays: string[] = [];
  for (let at = clean.indexOf("setTimeout("); at > -1; at = clean.indexOf("setTimeout(", at + 1)) {
    let depth = 0;
    let end = at + "setTimeout".length;
    for (let i = end; i < clean.length; i += 1) {
      if (clean[i] === "(") depth += 1;
      else if (clean[i] === ")") {
        depth -= 1;
        if (depth === 0) { end = i; break; }
      }
    }
    const args = clean.slice(at + "setTimeout(".length, end);
    const parts = args.split(",");
    delays.push(parts[parts.length - 1].trim());
  }
  assert.deepEqual(
    [...new Set(delays)].sort(),
    ["3000", "BRIDGE_TIMEOUT_MS"],
    "the page's only timers are the clear-confirm arming and the bridge timeout — no toast lifetime",
  );
  assert.equal(TOAST_DISMISS_MS.error, 8000);
  assert.equal(TOAST_DISMISS_MS.success, 4000);
  // The page keeps a keyed retry registry, not a notice queue and no longer a
  // single slot (see the M3 test below for why a slot was wrong).
  assert.match(page, /const retryRegistry = createRetryRegistry\(\);/, "the page keys retries by notification id");
  assert.match(page, /retryRegistry\.run\(data\.id\);/, "the host's retry reply is matched by id");
  assert.ok(
    !/pendingRetry/.test(code(page)),
    "the single retry slot must be gone — it fired the newest action for any old toast",
  );
});

test("the plugin surface reuses the default toast placement — no second stack", async () => {
  const css = stripComments(await read("src/styles/extensions.css"));
  const host = css.slice(css.indexOf("#floter-app-toasts {"));
  // The default anchor: full-height windows (settings, terminal) and plugin
  // pages all clear their chrome at 64px. The page's chrome is the R7-4 28px
  // topbar, so 64px leaves 36px of clearance below it.
  assert.match(host, /top:\s*64px/, "the default toast anchor stays at 64px");
  const pluginOverrides = [...host.matchAll(/#floter-app-toasts\[data-surface="plugin"\][^{]*\{/g)];
  assert.deepEqual(
    pluginOverrides.map((m) => m[0]),
    [],
    "the plugin surface must not add a placement override — a second position is exactly the split this round removes",
  );
  // The surface still travels to the host element; it is `mode`, so a plugin
  // page reports "plugin" without any new plumbing.
  const toast = await read("src/components/ToastStack.tsx");
  assert.match(toast, /data-surface=\{dataSurface\}/);
});
