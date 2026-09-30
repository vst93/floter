// R71 · the launcher's window height is automated, and it can never wedge.
//
// The user's report, verbatim: 「感觉还是窗口高度没有能自动化，剪切板插件就正常，书签
// 插件就有问题，应该做成统一的」 — the window's height is not automated; the
// clipboard plugin behaves, the browser one does not; it should be one mechanism.
//
// The asymmetry is the diagnosis. Both plugin modes compute exactly the same
// height (`launcherContentHeight` over their own rows — one policy for every
// plugin) and both go through `useLauncherHeight`. What differs is *when*: the
// clipboard's entries are fetched locally and land in the same beat as the mode
// entry, while the browser's bookmarks and history arrive after its fetch. So
// the browser resizes the window *while the mode is already on screen*, and that
// later resize is the one the WebView can miss (a panel that is not the active
// app, or one macOS has momentarily taken off screen, stops updating its
// viewport metrics). `window.innerHeight` then keeps reporting the height the
// window was asked for — the *satisfied* direction, which is the harmful one:
// every guard agreed the work was done, the card was laid out for the empty mode
// (field + chips), the twenty rows were clipped out of a window that was in fact
// tall enough, and only a *different* height (opening the configuration overlay,
// revealing the panel) could move it — 「点击一次设置再返回就显示了」.
//
// The mechanism was measured, not guessed: with the policy traced through one
// real summon, the window walked 541→58 (the empty mode), then the *corrective
// pass* of that walk re-measured the still-empty card and walked the window to
// 90 — cancelling the walk that was already carrying the list to 563 — and
// nothing asked again, because the sample and the window now agreed. A settings
// round trip asked for a *different* height, which is why it appeared to fix it.
//
// Three holes, all closed here, and each is a separate test below:
//
//   1. **The guard trusted the WebView.** `window.innerHeight` is the viewport,
//      not the window. The "already there" answer is now audited against the
//      platform (`innerSize()` + `scaleFactor()`) and a disagreement is
//      corrected.
//   2. **The observer kept its own rules.** `applied` / `launcherHeightNeedsResize`
//      were a second answer to the same question, and they refused to restore a
//      height that had been asked for before the window was moved behind the
//      frontend's back. The observer now makes the same call the layout effect
//      makes — one policy.
//   3. **A walk could stall forever.** A throttled `requestAnimationFrame` (the
//      panel is often not the active app) or a refused `setSize` left the promise
//      pending: the window stayed at whatever the last painted frame reached, the
//      module-level claim was never released so every later ask for that height
//      was dropped, and `tweenFrame` stayed set, muting the card's observer — the
//      one thing that corrects a height the window missed. The walk now always
//      settles, and a stall is resolved by asking the platform for the target
//      directly.
//
// Mutations that must turn this file red:
//   * dropping the audit from `applyLauncherHeight` (the synchronous fast path
//     believed on its own);
//   * believing `window.innerHeight` in the audit instead of `innerSize()`;
//   * dropping `heightRequest` (a stale audit could resize over a newer ask);
//   * dropping `RESIZE_WATCHDOG_MS`, its landing `setSize`, or the `setSize`
//     rejection handler from the walk;
//   * giving the observer its own `applied`/predicate rules back;
//   * dropping the generation check from the corrective pass, or the
//     at-the-band gate from the measurement.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

// ── A · the window's height is the content's, verified ──────────────────────

test("a height the WebView claims is taken at its word (no platform round trip)", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherHeight.ts"));
  // R72 · the reverse of the audit this round first shipped: two IPC round trips
  // per height check, plus a correction that re-enters through the card observer,
  // is a feedback loop on a fast typist (「搜索键入时非常卡…甚至都卡住了」). The
  // synchronous answer is final again — and there is no platform read left to
  // oscillate with.
  assert.match(
    source,
    /if \(Math\.abs\(height - currentWindowHeight\(\)\) <= HEIGHT_TOLERANCE\) \{\s*return;\s*\}/,
    "the 'already there' branch returns without asking anything",
  );
  assert.doesNotMatch(source, /innerSize\(\)/, "no platform size read");
  assert.doesNotMatch(source, /scaleFactor\(\)/, "no display-scale read");
  assert.doesNotMatch(source, /heightRequest/, "and no ask counter to keep in step");
});

// ── B · the observer has no rules of its own ────────────────────────────────

test("the card observer runs the one height policy", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherHeight.ts"));
  assert.match(
    source,
    /const observer = new ResizeObserver\(\(\) => \{[\s\S]*?if \(tweenFrame\) return;[\s\S]*?applyLauncherHeight\(current, launcherTargetHeight\(current, windowHeight\)\);/,
    "the observer calls the policy, not a private copy of it",
  );
  assert.doesNotMatch(source, /launcherHeightNeedsResize/, "the second rulebook is gone");
  assert.doesNotMatch(source, /RESIZE_LANDING_SLACK/, "…and so is its slack");
  assert.doesNotMatch(source, /applied\.current/, "…and its memory");
});

test("a superseded walk's corrective pass stands down", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherHeight.ts"));
  // The walk's own generation is read *while it is the newest ask* — before any
  // await — so the pass can tell "my walk is still the one on screen" from "a
  // newer ask replaced me".
  assert.match(
    source,
    /const walk = tweenLauncherHeight\(card, height\);\s*const generation = tweenGeneration;/,
    "the pass captures the generation of the walk it belongs to",
  );
  const pass = /afterPaint\(\(\) => \{([\s\S]*?)\n      \}\);/.exec(source);
  assert.ok(pass, "the corrective pass exists");
  assert.match(
    pass![1],
    /if \(generation !== tweenGeneration\) return;/,
    "a stale pass must not cancel the height a newer ask is walking to",
  );
  // …and it only speaks for a window that really is the height it was given.
  assert.match(
    pass![1],
    /Math\.abs\(current - height\) > HEIGHT_TOLERANCE\) return;/,
    "a mid-flight window is not evidence of an overflow",
  );
});

test("the measurement may only raise a window that is at its band", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherHeight.ts"));
  // R20's overflow guard survives, but gated: transient samples (a mid-walk
  // window, a viewport still reporting the previous surface) are not evidence.
  assert.match(
    source,
    /const atBand = Number\.isFinite\(current\) && Math\.abs\(current - base\) <= HEIGHT_TOLERANCE;/,
    "the overflow guard asks whether the window is at the band first",
  );
  assert.match(source, /if \(atBand && measured > current \+ 1\)/, "…and only then may raise");
});

// ── C · the walk always settles ─────────────────────────────────────────────

test("the edge walk is watched, and every exit settles it", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherHeight.ts"));

  assert.match(
    source,
    /const RESIZE_WATCHDOG_MS = RESIZE_TWEEN_MS \* 2 \+ 100/,
    "the walk declares a bounded lifetime",
  );
  assert.match(source, /let tweenWatchdog = 0/, "the walk's watchdog slot exists");
  assert.match(
    source,
    /tweenWatchdog = setTimeout\(\(\) => \{[\s\S]{0,500}?setSize\(new LogicalSize\(INPUT_WINDOW_WIDTH, height\)\)[\s\S]{0,120}?settle\(\);/,
    "a stalled walk lands the target height itself, not just its bookkeeping",
  );

  // The one exit lands the height, releases the frame slot and settles.
  const settle = /const settle = \(\) => \{[\s\S]*?tweenFrame = 0;[\s\S]*?tweenHeight = height;[\s\S]*?tweenDone = null;[\s\S]*?resolve\(\);[\s\S]*?\};/.exec(
    source,
  );
  assert.ok(settle, "the walk has one exit that settles it");
  assert.match(settle![0], /clearTimeout\(tweenWatchdog\)/, "the exit clears the watchdog");

  // A refused `setSize` ends the walk instead of stranding it.
  assert.match(
    source,
    /void platform\.setSize\(new LogicalSize\(INPUT_WINDOW_WIDTH, next\)\)\.then\([\s\S]{0,400}?\},\s*[\s\S]{0,80}?\(\) => \{\s*if \(generation === tweenGeneration\) settle\(\);/,
    "a refused frame settles the walk",
  );

  // A cancelled walk must not leave its watchdog armed: it would land a height
  // the walk that replaced it does not own.
  const cancel = /const cancelTween = \(\) => \{([\s\S]*?)\n\};/.exec(source);
  assert.ok(cancel, "cancelTween exists");
  assert.match(cancel![1], /clearTimeout\(tweenWatchdog\)/, "cancelTween clears the watchdog");
});

test("a settled height is never left claimed", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherHeight.ts"));
  // The ask is released on every outcome of the walk, not only a landing.
  const releases = [...source.matchAll(/\.finally\(\(\) => \{\s*if \(pendingLauncherHeight === height\) pendingLauncherHeight = 0;/g)];
  assert.ok(releases.length >= 1, "the walk releases its claim however it ends");
});
