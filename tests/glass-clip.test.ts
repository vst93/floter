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
  // R27 · the plugin sheets now `@import` a shared card stylesheet. An at-rule
  // carries no declarations of its own, so it is stripped before parsing —
  // otherwise it would glue itself to the selector of the rule that follows it
  // (`@import "…"; :root`) and every exact-selector lookup below would miss.
  const withoutImports = css.replace(/@import[^;]*;/g, "");
  for (const match of withoutImports.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
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

test("clipboard page.css derives its field and row cards from the injected bands in both palettes", async () => {
  const css = await pageCss();
  for (const [selector, base] of [
    [":root", "17, 18, 20"],
    ['[data-theme="light"]', "243, 244, 247"],
  ] as const) {
    const tokens = palette(css, selector);
    // GLASS-CLIP-2 split the page's one recess into two surfaces: the *field*
    // the list sits on (the page band) and the *row card* the copy sits on (the
    // same band plus the ladder rung). Both alphas come from the page's own
    // injected tokens; the colour bases stay the page's own (it is a separate
    // document with its own dark/light pair).
    const field = tokens.get("--surface-field");
    assert.ok(field, `${selector} must declare --surface-field`);
    assert.equal(
      field,
      `rgba(${base}, var(--glass-content-alpha))`,
      `${selector}'s list field must read the injected --glass-content-alpha`,
    );
    const row = tokens.get("--surface-row");
    assert.ok(row, `${selector} must declare --surface-row`);
    assert.equal(
      row,
      `rgba(${base}, var(--glass-row-alpha))`,
      `${selector}'s row card must read the injected --glass-row-alpha`,
    );
  }
});

test("the page sheet carries no static glass number: no literal recess alpha, no restated step token", async () => {
  const css = await pageCss();
  // A hardcoded alpha in a page surface is the bug. `var(--glass-…)` is the
  // only accepted form; an rgba() whose last argument is a number fails.
  for (const selector of [":root", '[data-theme="light"]']) {
    for (const token of ["--surface-field", "--surface-row"]) {
      const value = palette(css, selector).get(token)!;
      assert.ok(
        !/rgba\([^)]*,\s*[\d.]+\s*\)/.test(value),
        `${selector}'s ${token} must not carry a literal alpha, got "${value}"`,
      );
    }
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
  // The two --glass-* declarations the page may keep are the pre-JS first-paint
  // fallbacks for the two bands, and they must be *labelled* as such in the
  // source. The values are the default 46% terminal transparency evaluated
  // through the page's own table (field 0.572, row 0.652).
  const raw = await read("src/plugins/clipboard/page.css");
  assert.match(
    raw,
    /--glass-content-alpha:\s*0\.572;/,
    "the page keeps a single pre-JS fallback value for the field band",
  );
  assert.match(
    raw,
    /--glass-row-alpha:\s*0\.652;/,
    "the page keeps a single pre-JS fallback value for the row band",
  );
  assert.match(
    raw,
    /fallback[\s\S]{0,900}?--glass-content-alpha:\s*0\.572;/,
    "the fallback must be documented as a fallback, not read as a second source of truth",
  );
});

// ── B · one source of truth: the JS table mirrors base.css ────────────────

test("GLASS_CONTENT_BAND mirrors base.css's --glass-content-alpha coefficients, and the page band is separate", async () => {
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

  // GLASS-CLIP-2 · the page band is a *different table* on purpose. It must not
  // be the host's, and the host's must not have moved.
  const { GLASS_PAGE_CONTENT_BAND, glassPageContentAlpha, glassPageContentStyle, glassPageRowAlpha } =
    await import("../src/glass-material.ts");
  assert.notEqual(
    GLASS_PAGE_CONTENT_BAND.slope,
    GLASS_CONTENT_BAND.slope,
    "the page band must not reuse the host recess band's slope",
  );
  assert.equal(glassPageContentAlpha(0.46), GLASS_PAGE_CONTENT_BAND.base + GLASS_PAGE_CONTENT_BAND.slope * 0.46);
  assert.equal(glassPageRowAlpha(0.46), glassPageContentAlpha(0.46) + 0.08);
  assert.deepEqual(glassPageContentStyle(0.46), {
    "--glass-content-alpha": String(glassPageContentAlpha(0.46)),
    "--glass-row-alpha": String(glassPageRowAlpha(0.46)),
  });
  // base.css is untouched: the host formula is still exactly the coefficients
  // the first half of this test read out of the stylesheet.
  assert.match(base, /--glass-content-alpha:\s*calc\(0\.62 \+ 0\.18 \* var\(--main-opacity\)\)/);
});

test("the page resolves its own band (not the host recess band) and writes it, never a literal", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  // GLASS-CLIP-2: the page no longer evaluates the *host* recess band. That
  // band's 0.18 slope is right for a recess inside a frame and invisible for a
  // page that IS the frame's content area, which is the user's 「剪切板还是没有
  // 跟随透明度调整」. The page evaluates its own steeper band instead.
  assert.match(page, /glassPageContentAlpha/, "the page must evaluate its own band");
  assert.match(page, /glassPageRowAlpha/, "the page must evaluate the row-card rung too");
  assert.ok(
    !/glassContentAlpha\b/.test(page),
    "the page must not evaluate the host recess band — its slope is invisible at page scale",
  );
  assert.match(
    page,
    /setProperty\("--glass-content-alpha",\s*String\(glassPageContentAlpha\(/,
    "the page must write its field band onto its own root",
  );
  assert.match(
    page,
    /setProperty\("--glass-row-alpha",\s*String\(glassPageRowAlpha\(/,
    "the page must write its row band onto its own root",
  );
  // Both are written from `applyOpacity`, which both the bootstrap params and
  // the live bridge message funnel through — so a slider move reaches the list
  // material as well as the sheet.
  const apply = page.slice(page.indexOf("function applyOpacity"));
  const body = apply.slice(0, apply.indexOf("}", apply.indexOf("applyPageBackground")));
  assert.match(body, /--glass-content-alpha/, "applyOpacity must carry the field band update");
  assert.match(body, /--glass-row-alpha/, "…and the row band update");
  assert.match(body, /--page-fill|applyPageBackground/, "…alongside the sheet fill it already derived");

  // The host still injects its own (recess) bag through the same channel the
  // step uses. The page deliberately does not read it, but the host-side
  // hand-off stays asserted so the host contract cannot drift either.
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

// GLASS-CLIP-2 · the page's material is now a three-layer stack, and the
// legibility rule moved with the copy.
//
// The old assertion (recess composites denser than the sheet) described a page
// whose one content surface was the recess. The rewritten page has *two*: the
// field the list sits on (the page band, steep on purpose so the slider is
// visible) and the row card the copy sits on (the field plus the ladder rung).
// The rule that has to hold is the one `base.css` states for the host's own
// content layer — *content readability is not the user's transparency tradeoff*
// — expressed against the layer the copy is actually on, over the worst-case
// desktop, at every step and every slider position.
const sRGB = (v: number) => {
  const s = v / 255;
  return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
};
const relLuminance = (rgb: number[]) =>
  0.2126 * sRGB(rgb[0]) + 0.7152 * sRGB(rgb[1]) + 0.0722 * sRGB(rgb[2]);
const wcag = (a: number[], b: number[]) => {
  const [l1, l2] = [relLuminance(a), relLuminance(b)];
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
};

test("the page's row cards keep body copy legible over any desktop, at every step and slider position", async () => {
  const { glassPageContentAlpha, glassPageRowAlpha, GLASS_PAGE_ROW_LIFT, GLASS_STEP_TOKENS } =
    await import("../src/glass-material.ts");
  // The page sheet, exactly as `applyPageBackground` composes it: the step's
  // haze under the frame alpha. (Mirrors base.css's `--glass-tint-alpha`.)
  const sheetAlpha = (t: number, haze: number) =>
    1 - (1 - haze * (1 - t)) * (1 - Math.min(0.98, Math.max(0, t)));
  const palettes = {
    dark: { page: [17, 18, 20], copy: [238, 239, 242], backdrop: [255, 255, 255] },
    light: { page: [243, 244, 247], copy: [60, 60, 67], backdrop: [0, 0, 0] },
  } as const;
  for (const [theme, p] of Object.entries(palettes)) {
    for (const [step, { dim: haze }] of Object.entries(GLASS_STEP_TOKENS)) {
      for (let t = 0.1; t <= 1.0001; t += 0.05) {
        const field = glassPageContentAlpha(t);
        const row = glassPageRowAlpha(t);
        // The ladder is a rung, not a floor: the row is always denser.
        assert.ok(
          row > field,
          `${theme}/${step} @ ${t.toFixed(2)}: the row card (${row}) must clear the field (${field})`,
        );
        assert.equal(row, Math.min(1, field + GLASS_PAGE_ROW_LIFT), "the row must be exactly one rung above the field");
        // The composite under the glyphs: desktop → sheet → field → row card.
        const sheet = over([...p.page, sheetAlpha(t, haze)], [...p.backdrop]);
        const fieldPx = over([...p.page, field], sheet);
        const rowPx = over([...p.page, row], fieldPx);
        const ratio = wcag([...p.copy], rowPx);
        assert.ok(
          ratio >= 3,
          `${theme}/${step} @ ${t.toFixed(2)}: body copy on the row card is ${ratio.toFixed(2)}:1, under the 3:1 floor`,
        );
      }
    }
  }
});

test("the page band travels the whole slider: 10% and 95% are visibly different", async () => {
  // The user's report was not "the value did not change", it was "I cannot see
  // it change". The host recess band (0.62 + 0.18·t) moves 0.15 across the
  // slider and is invisible at page scale; the page band has to move by a
  // visible amount. This is the assertion the old suite lacked.
  const { glassPageContentAlpha, glassPageRowAlpha } = await import("../src/glass-material.ts");
  const fieldSpan = glassPageContentAlpha(0.95) - glassPageContentAlpha(0.10);
  const rowSpan = glassPageRowAlpha(0.95) - glassPageRowAlpha(0.10);
  assert.ok(
    fieldSpan >= 0.5,
    `the field band must swing at least 0.5 across the slider, got ${fieldSpan.toFixed(3)}`,
  );
  assert.ok(
    rowSpan >= 0.5,
    `the row band must swing with it, got ${rowSpan.toFixed(3)}`,
  );
  // And it really is steeper than the host's recess band, which is the whole
  // reason it is a separate table.
  const { GLASS_CONTENT_BAND } = await import("../src/glass-material.ts");
  assert.ok(
    fieldSpan > GLASS_CONTENT_BAND.slope * 0.85 + 0.1,
    "the page band's travel must clearly exceed the host recess band's",
  );
});

// ── D · mutation locks ────────────────────────────────────────────────────

test("mutation lock: flattening the page band back to the host's 0.18 slope goes red", async () => {
  const { GLASS_PAGE_CONTENT_BAND } = await import("../src/glass-material.ts");
  // The exact regression the user reported: someone "fixing" the page by
  // reusing the host recess band. The amplitude predicate must reject it.
  const mutatedSlope = 0.18;
  const span = (base: number, slope: number) => (base + slope * 0.95) - (base + slope * 0.10);
  assert.ok(
    span(GLASS_PAGE_CONTENT_BAND.base, GLASS_PAGE_CONTENT_BAND.slope) >= 0.5,
    "the shipped band must pass the amplitude predicate",
  );
  assert.ok(
    span(GLASS_PAGE_CONTENT_BAND.base, mutatedSlope) < 0.5,
    "the 0.18 slope must FAIL the amplitude predicate — otherwise the lock is vacuous",
  );
});

test("mutation lock: putting a static field alpha back goes red", async () => {
  const css = await pageCss();
  const mutated = css.replace(
    /--surface-field:\s*rgba\(17, 18, 20, var\(--glass-content-alpha\)\)/,
    "--surface-field: rgba(17, 18, 20, 0.5)",
  );
  assert.notEqual(mutated, css, "the mutation must land");
  const field = palette(mutated, ":root").get("--surface-field")!;
  assert.notEqual(
    field,
    "rgba(17, 18, 20, var(--glass-content-alpha))",
    "the 'consumes the injected band' predicate must reject the frozen alpha",
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
