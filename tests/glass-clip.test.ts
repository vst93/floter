// GLASS-CLIP · the surviving half of a two-problem round, plus the R77 freeze
// lock for the content-band orphans R76 exposed.
//
// 1. 「内置的功能，比如剪切板也要液态玻璃化，也要受到配置的影响」 and
//    「现在透明度的滑杆拖动会中断」 were GLASS-CLIP's two reports. The first was
//    about the built-in clipboard page's content recess; that page was retired
//    onto the launcher's configuration overlay (R33), its source deleted (R76),
//    and the glass-material content-band exports only its documents consumed
//    were deleted by R77. The second — the opacity slider's per-tick repaint
//    stalling the drag — is still live and is asserted in full below.
//
// 2. 「现在透明度的滑杆拖动会中断」 — the opacity effect in `App.tsx` ran
//    `renderer.updateTheme()` (a `getComputedStyle` over the whole document
//    root) plus a full terminal canvas redraw on *every tick* of a drag. The
//    main thread spent the drag repainting a canvas whose pixels do not depend
//    on the window alpha at all, and the range input's own event handling was
//    starved. The CSS custom properties stay per-tick (that is the visual
//    feedback); the repaint is coalesced to one trailing call.
//
// The node suite has no DOM, so the source-shape assertions read the sources
// and the scheduling assertions drive the extracted scheduler on fake time.
// The mutation lock at the bottom reproduces the exact repaint regression the
// round removed.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createDeferredRepaint, TERMINAL_REPAINT_DEBOUNCE_MS } from "../src/deferred-repaint.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
/** JS comments are stripped where a source-shape assertion would otherwise be
 * satisfied by the comment explaining the seam (the very common trap: the
 * comment above a removed call names the call). */
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

// ── B · the R77 orphans: the content-band exports stay deleted ────────────

/** The content-band exports R77 deleted. The names are assembled from parts so
 *  this guard does not itself reintroduce the tokens it bans — the repo's
 *  zero-hit grep over `src/`, `tests/` and `docs/` must stay clean. */
const RETIRED_BAND_EXPORTS = [
  "GLASS_" + "CONTENT_BAND",
  "glass" + "ContentAlpha",
  "glass" + "ContentStyle",
  "GLASS_PAGE_" + "CONTENT_BAND",
  "glassPage" + "ContentAlpha",
  "GLASS_PAGE_ROW_" + "LIFT",
  "glassPage" + "RowAlpha",
  "glassPage" + "ContentStyle",
] as const;

test("the retired glass-material content-band exports stay deleted", async () => {
  // R76 deleted the iframe page layer; the only live importers of these eight
  // exports were this suite and the retired page's documents. The host's own
  // `--glass-content-alpha` still lives in base.css (asserted by
  // `glass-controls` / `glass-material`), and the live step hand-off is
  // `GLASS_STEP_TOKENS` / `glassStepStyle` — so the module's export surface
  // must not carry these names again.
  const mod = (await import("../src/glass-material.ts")) as Record<string, unknown>;
  // The scan is not vacuous: a live export of the same module is found.
  assert.ok("glassStepStyle" in mod, "the live step hand-off must still be exported");
  for (const name of RETIRED_BAND_EXPORTS) {
    assert.ok(!(name in mod), `${name} was deleted in R77 and must stay gone`);
  }
});

// ── C · the repaint is coalesced, not per-tick ────────────────────────────

/** Deterministic fake timers: ids are numbers, exactly as the browser's are. */
const fakeTimers = () => {
  let next = 1;
  const scheduled = new Map<number, { fn: () => void; ms: number }>();
  return {
    scheduled,
    timers: {
      setTimeout: (fn: () => void, ms: number) => {
        const id = next++;
        scheduled.set(id, { fn, ms });
        return id;
      },
      clearTimeout: (id: number) => {
        scheduled.delete(id);
      },
    },
    /** Run every pending timer, oldest first, as a real event loop would. */
    runAll: () => {
      const pending = [...scheduled.entries()];
      scheduled.clear();
      for (const [, entry] of pending) entry.fn();
    },
  };
};

test("ten opacity ticks inside one window produce exactly one repaint", () => {
  const clock = fakeTimers();
  let renders = 0;
  const repaint = createDeferredRepaint(() => { renders += 1; }, TERMINAL_REPAINT_DEBOUNCE_MS, clock.timers);

  for (let tick = 0; tick < 10; tick += 1) {
    repaint.schedule();
    // Each tick restarts the window, so the 10th is the only one that fires.
    assert.equal(renders, 0, `tick ${tick} must not repaint synchronously`);
    assert.equal(repaint.isPending(), true);
  }
  assert.equal(clock.scheduled.size, 1, "only the newest timer may survive the restart");

  clock.runAll();
  assert.equal(renders, 1, "the whole drag must coalesce into one repaint");
  assert.equal(repaint.isPending(), false);

  // A second burst (the next drag) is independent and fires its own one.
  repaint.schedule();
  clock.runAll();
  assert.equal(renders, 2);
});

test("flush lands a pending repaint immediately, cancel drops it, and both clear the flag", () => {
  const clock = fakeTimers();
  let renders = 0;
  const repaint = createDeferredRepaint(() => { renders += 1; }, 140, clock.timers);

  repaint.flush();
  assert.equal(renders, 0, "flush with nothing pending is a no-op");

  repaint.schedule();
  repaint.schedule();
  repaint.flush();
  assert.equal(renders, 1, "flush must land the coalesced repaint now");
  assert.equal(repaint.isPending(), false, "…and leave nothing to fire later");
  assert.equal(clock.scheduled.size, 0, "…and clear the timer");

  repaint.schedule();
  repaint.cancel();
  clock.runAll();
  assert.equal(renders, 1, "a cancelled repaint must never run");
});

test("the opacity effect writes the CSS variables per tick but never repaints synchronously", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  const start = app.indexOf("useEffect(() => {\n    const root = document.documentElement.style;");
  assert.notEqual(start, -1, "the opacity effect must still exist");
  const end = app.indexOf("}, [settings.main_opacity, settings.terminal_opacity]);", start);
  assert.notEqual(end, -1, "the opacity effect must still be keyed on the two opacity fields");
  const effect = app.slice(start, end);

  // The per-tick half: the two custom properties are the slider's feedback.
  assert.match(effect, /setProperty\("--main-opacity"/);
  assert.match(effect, /setProperty\("--terminal-opacity"/);
  // The coalesced half.
  assert.match(effect, /repaintTerminalSoon\(\)/, "the repaint must be scheduled, not run inline");
  // …and nothing that re-reads the palette or redraws the canvas on this path.
  assert.ok(
    !/updateTheme\(\)/.test(effect),
    "the opacity effect must not re-read the palette per tick — that is the drag stall",
  );
  assert.ok(
    !/\brender\(\)/.test(effect),
    "the opacity effect must not repaint the canvas per tick — that is the drag stall",
  );
});

test("a pending repaint is landed before the terminal surface stops being painted", async () => {
  const hook = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  // The renderer-teardown cleanup is the single funnel every exit from the
  // terminal surface goes through. It must land the repaint while
  // `rendererRef` is still live — a flush after the nulling is a no-op — and
  // then drop the timer so a later mode flip cannot run a stale repaint.
  const cleanup = hook.slice(hook.indexOf('canvasRef.current?.removeEventListener("wheel", onWheelNative);'));
  const body = cleanup.slice(0, cleanup.indexOf("rendererRef.current = null;"));
  assert.match(body, /repaintFlushRef\.current\(\)/, "the teardown must land a pending repaint");
  assert.match(body, /repaintScheduler\.current\?\.cancel\(\)/, "…and then drop the timer");
  // Order matters: the flush has to precede the nulling of the renderer.
  const flushAt = cleanup.indexOf("repaintFlushRef.current()");
  const nullAt = cleanup.indexOf("rendererRef.current = null;");
  assert.ok(flushAt > -1 && flushAt < nullAt, "the flush must precede the renderer teardown");

  // The window-hidden paths flush too (blur → hide_window, document hidden),
  // through the same stable ref.
  assert.match(hook, /document\.hidden\) repaintFlushRef\.current\(\)/);
  assert.match(hook, /addEventListener\("blur", onBlur\)/);
  // The scheduler is built from the shared factory, not a second timer.
  assert.match(hook, /createDeferredRepaint\(repaintTerminal\)/);

  // And App must not grow a second, competing flush: the mode effect there
  // would see a null renderer and be a silent no-op, which is exactly the
  // "looks like a guard, does nothing" shape this round has to avoid.
  const app = stripJsComments(await read("src/App.tsx"));
  assert.ok(
    !/if \(mode === "terminal"\) return;/.test(app),
    "the flush belongs to the renderer lifecycle, not a second mode effect in App",
  );
});

// ── D · mutation lock ─────────────────────────────────────────────────────

test("mutation lock: running the repaint synchronously per tick goes red", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  const mutated = app.replace(
    "    repaintTerminalSoon();\n  }, [settings.main_opacity, settings.terminal_opacity]);",
    "    const renderer = rendererRef.current;\n    if (renderer) {\n      renderer.updateTheme();\n      render();\n    }\n  }, [settings.main_opacity, settings.terminal_opacity]);",
  );
  assert.notEqual(mutated, app, "the mutation must land");
  const start = mutated.indexOf("useEffect(() => {\n    const root = document.documentElement.style;");
  const end = mutated.indexOf("}, [settings.main_opacity, settings.terminal_opacity]);", start);
  const effect = mutated.slice(start, end);
  assert.ok(
    /updateTheme\(\)/.test(effect) || /\brender\(\)/.test(effect),
    "the 'no synchronous repaint on the opacity path' predicate must reject the old body",
  );
  assert.ok(
    !/repaintTerminalSoon\(\)/.test(effect),
    "the scheduling predicate must reject a body that runs the repaint inline",
  );
});
