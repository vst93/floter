// R7-5 · one feedback channel: a plugin page's notice becomes the host's toast.
//
// The bridge carries feedback as a *data* message with a dictionary key,
// validated on both ends, and the host is the side that translates. R33 retired
// the built-in iframe pages and R76 deleted the clipboard page's source, so the
// page-side halves of this suite (its notice markup, its dismiss timer, its
// state machine) are gone with the page.
//
// What stays is the *published* protocol and the host's own stack: the message
// shapes, the key validation, the per-key failure dedupe, the retry registry
// keyed by notification id, and the single toast placement.
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
import { MAX_TOASTS } from "../src/toast-state.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

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

test("App's notify grew an optional action slot and no plugin layer is left", async () => {
  const app = await read("src/App.tsx");
  assert.match(
    app,
    /const notify = useCallback\(\s*\(kind: ToastKind, text: string, action\?: ToastAction\)/,
    "notify must accept an optional action without changing its existing call shape",
  );
  // R33 · the plugin layer is retired with the iframe page; the toast host is
  // now the leading sibling of every mode tree and nothing may hide it inside
  // a plugin wrapper again.
  assert.equal((app.match(/\{pluginLayer\}/g) ?? []).length, 0);
  assert.ok(!app.includes("const pluginLayer = ("), "the plugin layer must be gone");
});

// ── 2 · a persistent outage raises one toast, not one per poll ────────────

test("five consecutive poll failures raise exactly one toast (30s dedupe per key)", () => {
  // Drive the shared dedupe policy directly: the 2s poll calls this once per
  // failed poll, and the burst below is 20s of outage. Only the first may paint;
  // the other four must be swallowed.
  assert.ok(
    FAILURE_NOTIFY_DEDUP_MS >= 30_000,
    "the window must dwarf the 2s poll — at 30s a poll can earn at most one toast",
  );
  const deduper = createFailureDeduper();
  const raised: number[] = [];
  let now = 1_000_000;
  for (let poll = 0; poll < 5; poll += 1) {
    if (deduper.allow("clipboard.loadFailed", now)) raised.push(now);
    now += 2000; // the poll interval
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

// ── 3 · each toast's retry keeps its own action ───────────────────────────

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

// ── 4 · one lifetime, one stack ───────────────────────────────────────────

test("the plugin surface reuses the default toast placement — no second stack", async () => {
  const css = stripComments(await read("src/styles/extensions.css"));
  const host = css.slice(css.indexOf("#floter-app-toasts {"));
  // The default anchor: full-height windows (settings, terminal) and plugin
  // pages all clear their chrome at 64px.
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
