// GLASS-CLIP · two user-reported problems, one round.
//
// 1. 「内置的功能，比如剪切板也要液态玻璃化，也要受到配置的影响」 — the built-in
//    clipboard page had been *partly* glassed by CLIP-DISSOLVE: its top band
//    dissolved into the host-injected `--page-fill`/`--panel-bg` material, but
//    the page's own content recess (`--surface-sunken`) was still a frozen
//    `rgba(17, 18, 20, 0.5)` / `rgba(243, 244, 247, 0.58)`. That recess is the
//    surface the 全部/收藏 tab strip and the list field sit on, so the one
//    visible "field" in the page ignored both configuration axes — the glass
//    step *and* the transparency sliders. It now reads the host's
//    standard-material band (`--glass-content-alpha`) from the same
//    `glass-material.ts` hand-off the step tokens already use.
//
// 2. 「现在透明度的滑杆拖动会中断」 — the opacity effect in `App.tsx` ran
//    `renderer.updateTheme()` (a `getComputedStyle` over the whole document
//    root) plus a full terminal canvas redraw on *every tick* of a drag. The
//    main thread spent the drag repainting a canvas whose pixels do not depend
//    on the window alpha at all, and the range input's own event handling was
//    starved. The CSS custom properties stay per-tick (that is the visual
//    feedback); the repaint is coalesced to one trailing call.
//
// The node suite has no DOM, so the CSS assertions read the source and the
// scheduling assertions drive the extracted scheduler on fake time. Both
// mutation locks at the bottom reproduce the exact regressions the round
// removed.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createDeferredRepaint, TERMINAL_REPAINT_DEBOUNCE_MS } from "../src/deferred-repaint.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
/** JS comments are stripped where a source-shape assertion would otherwise be
 * satisfied by the comment explaining the seam (the very common trap: the
 * comment above a removed call names the call). */
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};
const decl = (body: string, prop: string) =>
  body.match(new RegExp(`(?:^|;)\\s*${prop}\\s*:\\s*([^;]+)`))?.[1].trim() ?? null;

const pageCss = async () => stripComments(await read("src/plugins/clipboard/page.css"));
/** One palette block's declarations, keyed by custom property. */
const palette = (css: string, selector: string) => {
  const block = rules(css).find(({ selector: s }) => s === selector);
  assert.ok(block, `page.css must declare ${selector}`);
  const out = new Map<string, string>();
  for (const declaration of block!.body.matchAll(/(--[\w-]+)\s*:\s*([^;]+)/g)) {
    out.set(declaration[1], declaration[2].trim());
  }
  return out;
};

// ── A · the page's recess is the host's injected band ─────────────────────

test("clipboard page.css derives --surface-sunken from the injected content band in both palettes", async () => {
  const css = await pageCss();
  for (const [selector, base] of [
    [":root", "17, 18, 20"],
    ['[data-theme="light"]', "243, 244, 247"],
  ] as const) {
    const tokens = palette(css, selector);
    const sunken = tokens.get("--surface-sunken");
    assert.ok(sunken, `${selector} must declare --surface-sunken`);
    // The whole point: the alpha comes from the host, the *colour base* stays
    // the page's own (it is a separate document with its own dark/light pair).
    assert.equal(
      sunken,
      `rgba(${base}, var(--glass-content-alpha))`,
      `${selector}'s recess must read the host-injected --glass-content-alpha`,
    );
  }
});

test("the page sheet carries no static glass number: no literal recess alpha, no restated step token", async () => {
  const css = await pageCss();
  // A hardcoded alpha in --surface-sunken is the bug. `var(--glass-content-alpha)`
  // is the only accepted form; an rgba() whose last argument is a number fails.
  for (const selector of [":root", '[data-theme="light"]']) {
    const sunken = palette(css, selector).get("--surface-sunken")!;
    assert.ok(
      !/rgba\([^)]*,\s*[\d.]+\s*\)/.test(sunken),
      `${selector}'s recess must not carry a literal alpha, got "${sunken}"`,
    );
  }
  // The step tokens stay host-only (the GLASS-3STOP rule, re-pinned here so a
  // future page-local "fix" cannot reintroduce the same class of bug on the
  // effect axis while this round fixes the alpha axis).
  for (const token of ["--glass-step-dim", "--glass-solid-top", "--glass-frame-floor"]) {
    assert.ok(
      !new RegExp(`${token}\\s*:`).test(css),
      `clipboard/page.css must not declare ${token} — the host injects it`,
    );
  }
  // The one --glass-* declaration the page may keep is the pre-JS first-paint
  // fallback for the band, and it must be *labelled* as such in the source.
  const raw = await read("src/plugins/clipboard/page.css");
  assert.match(
    raw,
    /--glass-content-alpha:\s*0\.7028;/,
    "the page keeps a single pre-JS fallback value for the band",
  );
  assert.match(
    raw,
    /fallback[\s\S]{0,400}?--glass-content-alpha:\s*0\.7028;/,
    "the fallback must be documented as a fallback, not read as a second source of truth",
  );
});

// ── B · one source of truth: the JS table mirrors base.css ────────────────

test("GLASS_CONTENT_BAND mirrors base.css's --glass-content-alpha coefficients", async () => {
  const base = stripComments(await read("src/styles/base.css"));
  const rootBlock = base.slice(base.indexOf(":root {"), base.indexOf('[data-theme="light"]'));
  const declaration = rootBlock.match(/--glass-content-alpha:\s*calc\(([^;]+)\);/);
  assert.ok(declaration, "base.css must declare --glass-content-alpha as a calc()");
  const coefficients = declaration![1].match(
    /^\s*([\d.]+)\s*\+\s*([\d.]+)\s*\*\s*var\(--main-opacity\)\s*$/,
  );
  assert.ok(coefficients, `unexpected band formula: ${declaration![1]}`);

  const { GLASS_CONTENT_BAND, glassContentAlpha, glassContentStyle } = await import(
    "../src/glass-material.ts"
  );
  assert.equal(GLASS_CONTENT_BAND.base, Number(coefficients![1]), "base coefficient must match base.css");
  assert.equal(GLASS_CONTENT_BAND.slope, Number(coefficients![2]), "slope coefficient must match base.css");

  // The evaluator is the same arithmetic, clamped, and the style bag names the
  // token the page's sheet consumes.
  assert.equal(glassContentAlpha(0.46), Number(coefficients![1]) + Number(coefficients![2]) * 0.46);
  assert.equal(glassContentAlpha(1), Number(coefficients![1]) + Number(coefficients![2]));
  assert.deepEqual(glassContentStyle(0.46), {
    "--glass-content-alpha": String(glassContentAlpha(0.46)),
  });
});

test("the page and the host both resolve the band from the shared table, never a literal", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  assert.match(page, /glassContentAlpha/, "the page must evaluate the shared band");
  assert.match(
    page,
    /setProperty\("--glass-content-alpha",\s*String\(glassContentAlpha\(/,
    "the page must write the band onto its own root",
  );
  // It is written from `applyOpacity`, which both the bootstrap params and the
  // live bridge message funnel through — so a slider move reaches the recess
  // as well as the sheet.
  const apply = page.slice(page.indexOf("function applyOpacity"));
  const body = apply.slice(0, apply.indexOf("}", apply.indexOf("applyPageBackground")));
  assert.match(body, /--glass-content-alpha/, "applyOpacity must carry the band update");
  assert.match(body, /--page-fill|applyPageBackground/, "…alongside the sheet fill it already derived");

  // The host injects the same bag through the same channel the step uses.
  const host = await read("src/plugins/PluginPageHost.tsx");
  assert.match(host, /glassContentStyle\(/, "the host must inject the content band");
  assert.match(
    host,
    /\.\.\.glassStepStyle\(glassStep\),\s*\.\.\.glassContentStyle\(/,
    "the step and band bags must be composed at the one injection site",
  );
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

// ── C · the a11y contrast contract is unchanged (HIG red line) ────────────

/** sRGB composite of `fg` (rgba) over `bg` (rgb), and the WCAG ratio. */
const over = (fg: number[], bg: number[]) =>
  fg.slice(0, 3).map((channel, i) => channel * fg[3] + bg[i] * (1 - fg[3]));
const luminance = (rgb: number[]) => {
  const [r, g, b] = rgb.map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
};
const contrast = (a: number[], b: number[]) => {
  const [l1, l2] = [luminance(a), luminance(b)];
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
};

// The recess moved from a frozen alpha to a slider-tracking band, so the
// composite under the page's body copy moved with it. What must NOT move is
// the *rule* the host asserts for the same material: a thinner recess is a
// legibility cost, but the recess must still composite denser than the sheet
// it sits on at every slider position (the R8 ladder contract), and the
// shipped default must not get *worse* than the frozen value it replaces.
test("the page's recess still composites denser than its sheet at every slider position", async () => {
  const { GLASS_CONTENT_BAND } = await import("../src/glass-material.ts");
  const band = (t: number) => Math.min(1, GLASS_CONTENT_BAND.base + GLASS_CONTENT_BAND.slope * t);
  // The page sheet, exactly as `applyPageBackground` composes it: the step's
  // haze under the frame alpha.
  const sheetAlpha = (t: number, haze: number) =>
    1 - (1 - haze * (1 - t)) * (1 - Math.min(0.98, Math.max(0, t)));
  const hazes = { frosted: 0.6, regular: 0.5, liquid: 0.2 };
  for (const [step, haze] of Object.entries(hazes)) {
    for (let t = 0.1; t <= 1.0001; t += 0.05) {
      const sheet = sheetAlpha(t, haze);
      const recess = band(t);
      const composite = 1 - (1 - sheet) * (1 - recess);
      assert.ok(
        composite > sheet,
        `${step} @ ${t.toFixed(2)}: the recess composite (${composite.toFixed(4)}) must clear the sheet (${sheet.toFixed(4)})`,
      );
      // …and stay inside the HIG regular band's 60-80% ceiling the host asserts.
      assert.ok(recess >= 0.6 && recess <= 0.81, `the band must stay in the regular range, got ${recess}`);
    }
  }
});

test("the default-transparency recess is not less legible than the frozen value it replaces", async () => {
  const { GLASS_CONTENT_BAND } = await import("../src/glass-material.ts");
  const band = Math.min(1, GLASS_CONTENT_BAND.base + GLASS_CONTENT_BAND.slope * 0.46);
  const sheet = (t: number, haze: number) =>
    1 - (1 - haze * (1 - t)) * (1 - Math.min(0.98, Math.max(0, t)));
  // Dark page: near-white copy over the recess, worst case a white desktop.
  // Light page: near-black copy, worst case a black desktop. The frozen values
  // were the dark page's 0.50 and the light page's 0.58.
  const cases = [
    { name: "dark page, 68% secondary over a white desktop", base: [17, 18, 20], text: [238, 239, 242, 0.68], backdrop: [255, 255, 255], frozen: 0.5, haze: 0.5, t: 0.46 },
    { name: "light page, 68% secondary over a black desktop", base: [243, 244, 247], text: [60, 60, 67, 0.68], backdrop: [0, 0, 0], frozen: 0.58, haze: 0.5, t: 0.46 },
  ];
  for (const c of cases) {
    const at = (recess: number) => {
      const bg = over([...c.base, sheet(c.t, c.haze)], c.backdrop);
      const sunken = over([...c.base, recess], bg);
      return contrast(over(c.text, sunken), sunken);
    };
    const now = at(band);
    const before = at(c.frozen);
    assert.ok(
      now >= before - 0.05,
      `${c.name}: the band must not lose legibility vs the frozen alpha (now ${now.toFixed(2)}, before ${before.toFixed(2)})`,
    );
  }
});

// ── D · mutation locks ────────────────────────────────────────────────────

test("mutation lock: putting a static --surface-sunken back goes red", async () => {
  const css = await pageCss();
  const mutated = css.replace(
    /--surface-sunken:\s*rgba\(17, 18, 20, var\(--glass-content-alpha\)\)/,
    "--surface-sunken: rgba(17, 18, 20, 0.5)",
  );
  assert.notEqual(mutated, css, "the mutation must land");
  const sunken = palette(mutated, ":root").get("--surface-sunken")!;
  assert.notEqual(
    sunken,
    "rgba(17, 18, 20, var(--glass-content-alpha))",
    "the 'consumes the injected band' predicate must reject the frozen alpha",
  );
  assert.ok(
    /rgba\([^)]*,\s*[\d.]+\s*\)/.test(sunken),
    "the literal-alpha predicate must reject the frozen alpha",
  );
});

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
