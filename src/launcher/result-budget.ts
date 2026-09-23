// R10-A · How many rows the launcher shows, and how a numbered key finds one.
//
// The user's report, verbatim: "当下的搜索页面高度太低了，搜索的时候展示的项
// 太少，需要展示 9 项加一个底部的单独进入剪切板的项 一共 10 项". So the budget
// became ten rows, and R10-A-R36 spent the tenth on a fixed row at the tail that
// opened the clipboard history. The fixed row was the launcher's tenth slot —
// not a match, so the catalog itself never returned more than nine.
//
// R19 · the user re-read that same screenshot and counted the rows: "列表项也有
// 问题，除了末尾的 cmd+回车的终端执行，上方一共有 10 项了，应该最大只能有 9
// 项". Nine above the action bar is the whole list, so the budget became nine
// rows: **eight matched results plus the fixed clipboard row**, with the tail
// row carrying the last number of the `1`-`9` family.
//
// R36 · the user then read the numbered badges and asked why they stop at eight
// when the keyboard has a ninth key, and a tenth one right beside it: "列表上的
// 快捷键为什么只到 8 ，可以到 9 的，可以加上 9 后面再加上 0 ，键盘上他们挨着
// 的". So the family grows to **ten slots**: the viewport numbers `⌘1`-`⌘9`
// (nine runnable rows) and the tenth key, `⌘0`, right beside `9` on the number
// row. R36 spent that tenth key on a *fixed* clipboard row appended to every
// query state.
//
// R37 · the user read that fixed row back and rejected it: 「现在搜索页面中 剪切板
// 这项被固定放到末尾，并且总是显示，还固定为了 cmd+0，这不对，它（其他内置插件
// 也是）不应该是个特例，应该和其他项一样匹配了才显示，同时快捷键也要按顺序安排」.
// The clipboard row is not a special case: it is a **result contributor** like
// any other built-in, matched by the query and ranked with everything else. So
// the fixed tail is deleted, `FIXED_TAIL_SLOT` retires, and the ten slots are
// **ten results**: `⌘1`-`⌘9` for the first nine runnable rows in the viewport
// and `⌘0` for the tenth, assigned purely in order. Nothing is reserved.
//
// This lives in its own pure module (no React, no Tauri) for the same reason
// `file-drops.ts` does: the budget and the row-count geometry are the two things
// a review can silently delete, and a node test can only pin them if it can
// import them without dragging the app's runtime in.

// The explicit `.ts` is what lets this module be imported by the node suite:
// `../launcher` is a directory and node resolves the extension literally.
import type { LauncherItem } from "./LauncherResults";
import { SEARCH_FIELD_HEIGHT_UNITS } from "./search-field.ts";

/** Ten matched results — the whole budget. R19-R36 reserved the tenth row for
 *  the fixed clipboard row; R37 de-specializes it, so the tenth slot is a
 *  result like the other nine and the catalog may fill all ten. */
export const MAX_RESULTS = 10;

/** The last numbered slot of the `1`-`9` run: `⌘9`.
 *
 *  R34 · `1`-`9` are assigned to the runnable rows *inside the viewport*; R37
 *  keeps that and makes the tenth key the next row in the same sequence rather
 *  than a fixed item's. */
export const MAX_VIEWPORT_SLOT = MAX_RESULTS - 1;

/** The family's tenth key: `0`, right beside `9` on the number row. R36 called
 *  it `FIXED_TAIL_SLOT` and handed it to the appended clipboard row; R37 retires
 *  that concept — the key is the *tenth result's*, assigned in order like
 *  `1`-`9` (see `resultShortcutSlots`). There is no reserved slot left. */
export const LAST_RESULT_SLOT = 0;

/** Matched rows when the query also reached applications or power actions:
 *  three catalog commands, leaving seven slots for those local matches — the
 *  same 3-to-N shape the launcher showed at every earlier budget (six local
 *  slots while the tenth row was the fixed tail, R37's one extra). */
export const COMMAND_LIMIT_WITH_MATCHES = 3;

/** The two heights a result row is drawn at, in `--u` units. This pair is the
 *  single source of the numbers `styles/launcher.css` writes as
 *  `calc(var(--u) * N)` on `.launcher-result` and `.launcher-result--compact`;
 *  `tests/launcher-ten-rows.test.ts` asserts the three agree.
 *
 *  A row that prints a subtitle is two-line and takes the first; a row whose
 *  title stands alone (see `row-content.ts`) collapses to the second. */
export const ROW_HEIGHT_TWO_LINE = 42;
export const ROW_HEIGHT_COMPACT = 34;

/** The list's ceiling in `--u` units: **every** row at its tallest height.
 *
 *  R20 · the ceiling used to be computed from the *compact* height alone —
 *  `8 × ROW_HEIGHT_COMPACT + ROW_HEIGHT_TWO_LINE` = 314u — which is the height
 *  of the list in the state that happened to be photographed, not the height it
 *  can reach. A query that matches commands is exactly the state that breaks it:
 *  every command row carries a description, so all ten rows are two-line, the
 *  content is 28u taller than the ceiling it was measured against, and the list
 *  scrolls inside a cap that was sized for shorter rows. The ceiling is a
 *  *ceiling*, so it is now the worst case: `MAX_RESULTS × ROW_HEIGHT_TWO_LINE`
 *  = 420u (378u through R35, when the budget was nine rows). */
export const RESULTS_LIST_HEIGHT = MAX_RESULTS * ROW_HEIGHT_TWO_LINE;

/** What the ceiling adds on top of the rows: the fixed pixels that do not scale
 *  with the unit — the scroll-edge reservation the scroller keeps as top
 *  padding (R22: 8px in the launcher, where `styles/launcher.css` overrides
 *  `--scroll-edge` locally over base.css's 14px — R24 halves that local value
 *  again, to 4px; see `--scroll-edge` in `styles/base.css`), the nine 1px grid
 *  gaps around ten rows, and the empty-query section title (a `--text-body`
 *  line at 1.4 plus its 6/4px padding ≈ 26px). `4 + 9 + 26 = 39`; the constant
 *  is left at 50, a ceiling that is now a whole 11px clear of what it has to
 *  cover (it was `8 + 9 + 26 = 43` from R22, itself `14 + 9 + 26 = 49` before
 *  R22 with one pixel of slack). The constant only has to *clear* the chrome
 *  it stands for, so R24 leaves it where it is — R36 adds a row, and with it a
 *  tenth gap, without moving the ceiling. */
export const RESULTS_LIST_CHROME = 50;

/** Chrome the launcher draws around the result list: the query row, the action
 *  bar and the window's shadow margin. The App's inline
 *  `--launcher-results-height` cap is `screen.availHeight - this`, so the ten
 *  rows still fit a short display; on an ordinary one the CSS row budget (see
 *  `.launcher-results` in `styles/launcher.css`) is the smaller cap and is what
 *  binds.
 *
 *  R19 · re-audited against the sheet, segment by segment, because R18 changed
 *  the query block without touching this number. At the default interface step
 *  (`--ui-scale: 1`, so the unit is 1px):
 *
 *    `.collapsed-card__input-row`  56u  (min-height — 56u pinned since R10,
 *                                      tightened to 48u in R22 and to 42u in
 *                                      R23, restored to 56u in R37 to match
 *                                      the settings band)
 *    R18 breath below it            4u  (`margin-bottom` on the input row —
 *                                      8u in R18, halved in R24)
 *    `.launcher-bottom` padding     6u  (4u top + 2u bottom — R21: the tail
 *                                      pairs with the last row's own leading,
 *                                      see `styles/launcher.css`)
 *    action bar row                42u  (height — it is a row, see below)
 *    feedback row                  30u  (min-height, may appear)
 *    card margin / rounding slack   7u
 *    ─────────────────────────────────
 *                                 129u
 *
 *  R22 · the field's row loses 8u (56u → 48u, see `styles/launcher.css`) and
 *  so does this constant: 232 → 224. Nothing else in the audit moves — the
 *  row's *height* is the only number that changed, and the R18 breath, the
 *  tail, the action bar and the feedback row are all untouched — so the segment
 *  list above is 149u − 8u = 141u and the constant still clears it by 83u of
 *  slack. The scroll-edge reservation that also sits in this gap is a *list*
 *  number, not a chrome one: it is already inside `RESULTS_LIST_CHROME`.
 *
 *  R23 · the field's row loses 6u more (48u → 42u) after the user still read the
 *  space below the query as too wide (「输入框下内边距还是太宽」), and the constant
 *  follows it down by the same 6u: 224 → 216. The audit is 141u − 8u (R22) − 6u
 *  (R23) = 133u, and nothing else in the segment list moves. The slack is
 *  unchanged at 83u — both the constant and the audit shrink together — so the
 *  floor still clears the chrome it stands for: 216u ≥ 133u.
 *
 *  R24 · the third report on the same gap (「现在还是太高」), and the first
 *  round to move something *outside* the field. R23's 42u row is the floor —
 *  10u a side is the last step before the line box looks pinched — so this round
 *  takes the two segments around it: the R18 breath halves (the input row's 8u
 *  `margin-bottom` → 4u, so 12u → 8u with the panel's own 4u top padding, see
 *  `styles/launcher.css`) and the launcher's scroll-edge reservation halves
 *  (8px → 4px on `.collapsed-card`). Only the breath is in this audit — the
 *  reservation is a *list* number, already inside `RESULTS_LIST_CHROME` — so
 *  133u − 4u = 129u, and the constant follows it down by the same 4u:
 *  216 → 212. The slack is still 83u: 212u ≥ 129u.
 *
 *  R20 · the R19 audit above read the action bar as 30u (the *feedback* row's
 *  floor). `.launcher-action-bar` declares `height: calc(var(--u) * 42)`, so the
 *  chrome is 12u more than that audit claimed — the correction is in the list,
 *  not in the constant. `RESULTS_VIEWPORT_CHROME` is a **floor** for a short
 *  display, not a measurement, and it only has to be at least the chrome it
 *  stands for so that the cap it writes is never larger than the window can
 *  hold: 212u ≥ 129u, by 83u of slack (the R24 audit; it read 133u before this
 *  round). What R20 has to check is that the cap
 *  does not bind on an ordinary display, i.e. that
 *  `availHeight - 212 ≥ RESULTS_LIST_HEIGHT × 1 + RESULTS_LIST_CHROME` — the
 *  worst-case list plus its fixed chrome, `420 + 50 = 470px`. That holds for
 *  every display taller than 682px of work area (212 + 470), and it is asserted
 *  in `tests/launcher-ten-rows.test.ts`.
 *
 *  R37 · the field's row grows 42u to 56u (the settings band's height, see
 *  `search-field.ts`), and this constant follows it up by the same 14u:
 *  212 to 226. The audit is 129u + 14u = 143u and the slack is unchanged at 83u:
 *  226u against 143u. The worst-case list is still 470px, so the cap now clears
 *  on every work area taller than 696px (226 + 470) — still under the 768px of a
 *  1280x800 display. */
export const RESULTS_VIEWPORT_CHROME = 226;

/** R25 · the launcher window's height is a **constant**, derived from the same
 *  ten-row budget the list above is built from. The user's report, verbatim:
 *  「现在搜索页面输入进行过滤时页面整体有抖动的情况，不像原生应用」. The cause was
 *  structural: every keystroke changed the row count, the row count drove a
 *  measurement, and the measurement called `setSize` — the native window was
 *  resized once per key. Every native launcher (Raycast, Alfred, Spotlight,
 *  tinycast) does the opposite: the window is a fixed slab, the list scrolls
 *  inside it, and the action bar is pinned to its bottom edge. That difference
 *  is what "native" means here, so this round makes the launcher's window that
 *  slab.
 *
 *  The constant is the **worst case of the budget chain**, segment by segment
 *  (see the audit on `RESULTS_VIEWPORT_CHROME` below and `styles/launcher.css`
 *  for each box):
 *
 *    `.collapsed-card__input-row`     56u  (min-height, R37 — the settings
 *                                           band's height, `search-field.ts`)
 *    the row's `margin-bottom`         4u  (R18's breath, halved in R24)
 *    `.launcher-bottom` padding-top    4u  (with the line above: the 8u breath)
 *    `.launcher-results` max-height  420u  (ten two-line rows, R20/R36)
 *    `.launcher-action-bar` margin     3u  (R18's constant gap)
 *    `.launcher-action-bar` height    42u  (a row, not a footer)
 *    `.launcher-bottom` padding-bottom 2u  (R21's tail)
 *    ────────────────────────────────────
 *                                    531u
 *
 *  plus the pixels that do not scale with the unit: the list's own chrome
 *  (`RESULTS_LIST_CHROME`, 50px — band, gaps and the empty-query title) and the
 *  card's 1px frame top and bottom. `531u + 52px` is 583px at the default
 *  interface step, and 11px above the tallest card the sheets can actually
 *  produce (the list's real worst case is 420u + 39px, 11px under its 50px
 *  ceiling) — a window never clips, and the slack lands in the gap above the
 *  pinned action bar, where nothing can see it.
 *
 *  R36 · the budget grew from nine rows to ten (378u → 420u of list, 475u →
 *  517u of slab, 527px → 569px at the default step). The user's read of the old
 *  527px slab was that it looked as if it already held ten rows; measured
 *  against the sheet it does not — 527px is `475u + 52px`, and ten two-line
 *  rows plus their chrome need `517u + 52px`. The window therefore grows by
 *  exactly one row (42u), and the per-row resolver (R34) keeps every shorter
 *  state exactly as tall as its own rows.
 *
 *  R37 · the field's row grows 42u to 56u (see `search-field.ts`) and the slab
 *  follows: 517u to 531u, 569px to 583px at the default step. It is the first
 *  segment of the audit, so nothing else in the list moves — the per-row
 *  resolver (R34) reads the same segment through
 *  {@link LAUNCHER_ROW_CHROME_UNITS} and every shorter state stays exactly as
 *  tall as its own rows.
 *
 *  The unit split is deliberate: the budget is a layout number and has to
 *  follow the interface step, so it is written as units + fixed pixels and
 *  scaled **once**, in `launcherWindowHeight` — never by multiplying a
 *  measurement (see the note in `hooks/useLauncherHeight.ts`). */
export const LAUNCHER_WINDOW_HEIGHT_UNITS = 531;

/** The part of {@link LAUNCHER_WINDOW_HEIGHT} that does not scale: the list's
 *  `RESULTS_LIST_CHROME` and the card's 1px frame top and bottom. */
export const LAUNCHER_WINDOW_HEIGHT_CHROME = RESULTS_LIST_CHROME + 2;

/** The launcher window's height at the default interface step: `531u + 52px`
 *  = 583px. Every state of the launcher — empty query, one result, ten, a
 *  feedback row, the first-run tip — is drawn inside this slab, so nothing a
 *  keystroke does may resize the window. R37: the slab is 583px (`531u + 52px`),
 *  the field's row having grown to the settings band's 56u. */
export const LAUNCHER_WINDOW_HEIGHT =
  LAUNCHER_WINDOW_HEIGHT_UNITS + LAUNCHER_WINDOW_HEIGHT_CHROME;

/** The constant at an interface step, in logical pixels. The step multiplies
 *  the unit part only; the chrome is fixed pixels either way, exactly as
 *  `calc(var(--u) * N + Mpx)` behaves in the sheet. Rounded up, because a
 *  window one pixel short of its card is a clipped card. */
export const launcherWindowHeight = (scale: number): number =>
  Math.ceil(LAUNCHER_WINDOW_HEIGHT_UNITS * scale + LAUNCHER_WINDOW_HEIGHT_CHROME);

/**
/**
 * R34 · the launcher window is **per-row**: its height is the exact height of
 * the rows it is drawing, not the smallest of four bands that happens to hold
 * them.
 *
 * The user's report, verbatim: 「搜索页高度应该随着选项列表高度自适应，现在最大
 * 高度就行了」. R26-D's bands were `1 / 3 / 6 / 9` rows, so four, five, seven and
 * eight rows each landed in the next band up and left one or two empty rows
 * under the list — the window read as "always at its maximum height". The fix
 * keeps every property the band model was built for (a keystroke never moves the
 * window inside a row count; a boundary oscillation does not flap) and drops the
 * quantisation: a row count *is* the height.
 *
 * `resolveLauncherRows` is the sticky half. It grows immediately — a window one
 * row short would clip the row — and shrinks only once the count has fallen
 * {@link LAUNCHER_ROW_HYSTERESIS} rows *below* the held count. The one-row
 * margin is what makes the `1 ↔ 2` boundary (the empty launcher gaining its
 * first match) never oscillate: a shrink from two rows to one is absorbed, and
 * the next growth finds the window already there.
 *
 * The rows are the same worst-case rows the band table charged: every row is
 * `ROW_HEIGHT_TWO_LINE` (42u), so a row gaining a subtitle does not move the
 * window (the "same count, no resize" invariant), and the chrome segments
 * (field, breath, panel insets, the action bar, the browser filter, the
 * section title) are charged exactly as R27/R32 charged them.
 */

/** How many rows below the held count the launcher must fall before the window
 *  steps down. One is enough: a boundary oscillation is a one-row move, so
 *  requiring a row of margin on the way out kills the flap without making a
 *  genuinely shorter list wait for a second row to disappear. */
export const LAUNCHER_ROW_HYSTERESIS = 1;

/** The launcher's row count, floored at one (an empty query still draws the
 *  recents, and the launcher is never a zero-row card) and capped at the
 *  ten-row budget (the list scrolls inside the slab past that, so the window
 *  never grows). */
export const clampLauncherRows = (rows: number): number =>
  Math.max(1, Math.min(MAX_RESULTS, Math.ceil(rows)));

/**
 * Resolve the held row count from the count currently held and the raw count the
 * content needs. Growing is immediate; shrinking waits for the count to fall a
 * row below the held one (see {@link LAUNCHER_ROW_HYSTERESIS}).
 */
export const resolveLauncherRows = (current: number, rows: number): number => {
  const target = clampLauncherRows(rows);
  const held = clampLauncherRows(current);
  if (target >= held) return target;
  return target < held - LAUNCHER_ROW_HYSTERESIS ? target : held;
};

/**
 * The segments of a row-count's height that do not depend on the count, in
 * units: the field's row ({@link SEARCH_FIELD_HEIGHT_UNITS}, 56u since R37),
 * the breath below it (4u), the panel's top inset (4u) and its tail (2u).
 * `66 + rows × 42 + bar + filter` is the height.
 *
 * R37 · the field's row is read from `search-field.ts` rather than restated:
 * the band the user aligned and the first segment of the window budget are the
 * same decision, and a field that grew without the slab would be a card taller
 * than its window. */
export const LAUNCHER_ROW_CHROME_UNITS = SEARCH_FIELD_HEIGHT_UNITS + 10;

/**
 * The action bar's own segment, in units: R18's constant 3u gap plus the row
 * itself (42u). R27 · it is charged only when the bar is actually drawn — see
 * {@link launcherRowUnits}.
 */
export const LAUNCHER_ACTION_BAR_UNITS = 45;

/**
 * R32 · the browser mode's range-filter subline, in units: the chip row's own
 * 24u plus the 4u breath below it (see `.launcher-filter`). It is charged by
 * every row count while the browser scope is open — the chips row is fixed
 * chrome that is always drawn there, so it can never resize the window as the
 * list under it grows or filters. The 4u gap above it is the field row's own
 * `margin-bottom`, already inside {@link LAUNCHER_ROW_CHROME_UNITS}.
 */
export const LAUNCHER_FILTER_UNITS = 28;

/** The unit height of a row count, with or without its action bar and filter.
 *
 *  The top of the table is the R25/R36/R37 budget: `launcherRowUnits(10, true)`
 *  is `66 + 10x42 + 45 = 531u` (517u through R36). R27 · the bar is a
 *  parameter rather than a
 *  constant of every height: in the no-match state and in both plugin modes
 *  there is no bar and the height is 45u shorter, which is what stops the window
 *  from reserving a row the user never sees.
 *
 *  R32 · the browser filter is the second such parameter: a fixed subline the
 *  browser scope always draws, charged here so this module stays the one place a
 *  window height comes from. */
export const launcherRowUnits = (
  rows: number,
  actionBar = true,
  filter = false,
): number =>
  LAUNCHER_ROW_CHROME_UNITS +
  clampLauncherRows(rows) * ROW_HEIGHT_TWO_LINE +
  (actionBar ? LAUNCHER_ACTION_BAR_UNITS : 0) +
  (filter ? LAUNCHER_FILTER_UNITS : 0);

/** The list's own section heading: one `--text-body` line at 1.4 plus its 6/4px
 *  padding pair (see `.launcher-section-title`). The top of the table folds this
 *  into the R25 chrome ceiling instead of adding it, so the full slab is
 *  unchanged. */
export const LAUNCHER_SECTION_TITLE_CHROME = 26;

/**
 * The fixed pixels a row count's window adds to its unit part: the card's 1px
 * frame top and bottom, the scroller's scroll-edge reservation, the 1px grid
 * gaps between the rows, and (empty query only) the section heading.
 *
 * R27 · the ten-row top keeps the R25 ceiling — `RESULTS_LIST_CHROME + 2` —
 * because that constant is what makes the full slab 583px and it is deliberately
 * a *ceiling* with slack for the list's real worst case. Shorter heights do not
 * need that slack (their content is shorter by construction), so they use the
 * honest count and the empty launcher loses the ~30px the ceiling was holding
 * for rows it does not have. The honest count is why a four-row list is exactly
 * four rows tall instead of landing in a six-row band.
 */
export const launcherRowChrome = (rows: number, sectionTitle = false): number => {
  const count = clampLauncherRows(rows);
  if (count >= MAX_RESULTS) return LAUNCHER_WINDOW_HEIGHT_CHROME;
  return (
    2 +
    4 +
    Math.max(0, count - 1) +
    (sectionTitle ? LAUNCHER_SECTION_TITLE_CHROME : 0)
  );
};

/** A row count's window height at an interface step, never above the display
 *  cap. The unit part scales; the chrome is added once, unscaled, exactly as
 *  {@link launcherWindowHeight} does for the full slab. */
export const launcherRowHeight = (
  rows: number,
  scale: number,
  maxHeight: number,
  actionBar = true,
  sectionTitle = false,
  filter = false,
): number =>
  Math.min(
    Math.ceil(
      launcherRowUnits(rows, actionBar, filter) * scale +
        launcherRowChrome(rows, sectionTitle),
    ),
    maxHeight,
  );

/** R43 · a status note's height, in units. It is the `.launcher-status` line's
 *  `min-height` in `styles/launcher.css`, and the same 30u `.launcher-feedback`
 *  reserves — a note and a feedback line are the same object at two positions.
 *  The list's height accounting reads it instead of charging a full row. */
export const LAUNCHER_STATUS_UNITS = 30;

/**
 * R43 · the list's real content height, in units.
 *
 * The user's report, verbatim: 「搜索页现在这个列表高度看起来还是没有完全自适应，
 * 在选项和底部独立的「终端运行」这个选项之间，还是会有一些空行」. The window was
 * sized as `count × ROW_HEIGHT_TWO_LINE` — every row charged at the two-line
 * height — while a row whose subtitle was dropped (an application, a system
 * action, a plugin row with nothing to say; see `row-content.ts`) is drawn at
 * the compact height. Ten compact rows therefore sat in a window 80u taller
 * than their list, and the 34–117u of leftover landed in the gap above the
 * pinned action bar.
 *
 * This is the honest number: the sum of the rows' *own* heights, so the window
 * can be exactly the list it holds. The row count still decides the fixed
 * chrome (the scroll-edge band, the gaps, the section title) — only the unit
 * part is the real total.
 */
export const launcherListUnits = (heights: readonly number[]): number =>
  heights.reduce((total, height) => total + Math.max(0, height), 0);

/**
 * R43 · the sticky list-unit total. The mirror of {@link resolveLauncherRows}
 * for the *height* axis: growth is immediate (a window one row short would clip
 * the row), and a shrink is absorbed until the content has fallen more than one
 * worst-case row below the held total — so the 1↔2 boundary and a one-row
 * change still never flap, exactly as the count resolver guarantees for the
 * count.
 */
export const resolveLauncherUnits = (current: number, units: number): number => {
  const target = Math.max(ROW_HEIGHT_COMPACT, Math.ceil(units));
  const held = Math.max(ROW_HEIGHT_COMPACT, Math.ceil(current));
  if (target >= held) return target;
  return held - target <= ROW_HEIGHT_TWO_LINE ? held : target;
};

/**
 * R43 · a real-content list's window height at an interface step.
 *
 * `listUnits` is {@link launcherListUnits}'s sum (plus one worst-case row for
 * any chrome row the caller charges as a row); `rows` is still the rendered row
 * count, because the fixed chrome (`launcherRowChrome`) is a function of the
 * count and the section title, not of the unit total. The unit part scales and
 * the chrome is added once, exactly as {@link launcherRowHeight} does.
 */
export const launcherContentHeight = (
  listUnits: number,
  rows: number,
  scale: number,
  maxHeight: number,
  actionBar = true,
  sectionTitle = false,
  filter = false,
): number =>
  Math.min(
    Math.ceil(
      (LAUNCHER_ROW_CHROME_UNITS +
        listUnits +
        (actionBar ? LAUNCHER_ACTION_BAR_UNITS : 0) +
        (filter ? LAUNCHER_FILTER_UNITS : 0)) *
        scale +
        launcherRowChrome(rows, sectionTitle),
    ),
    maxHeight,
  );

/** Whether a row is *a* clipboard row — the `system-clipboard` entry the
 *  catalog contributes like any other built-in.
 *
 *  R37 · this used to answer "is this the *fixed* tail, or the matched command?"
 *  and the answer decided whether the App had to append a second row. The fixed
 *  row is gone (see the module header), so the predicate is only the row's own
 *  identity now — the clipboard is an ordinary result contributor and carries
 *  no rendering privilege. */
export const isClipboardResult = (item: LauncherItem): boolean =>
  item.type === "system" && item.action === "clipboard";

/**
 * R34 · the half-open index range of the rows the scroller is showing. `start`
 * is the first row that is fully visible from the top (a row clipped by the
 * scroller's top edge is skipped, so the numbers always begin on a row the user
 * can read in full); `end` is one past the last row with any part in the box.
 *
 * The launcher's numbered slots are assigned over `[start, end)` — "what you
 * see is what you select". A list that fits needs no scrolling and the range is
 * the whole list, so the ordinary case is unchanged.
 */
export type VisibleRowRange = { start: number; end: number };

/** One row's geometry, as the scroller sees it: `top` is the row's top edge in
 *  the scroller's content coordinates (i.e. `scrollTop` is already added), and
 *  `height` is its laid-out box. `null` marks an index the list did not render
 *  (a status note, which is not a row). */
export type RowSpan = { top: number; height: number };

/**
 * R34 · the visible range, from each row's geometry and the scroller's own
 * box. Pure: the component hands it the measured spans, so the mapping is a
 * function the node suite can drive without a DOM.
 *
 * The rule is the one the user asked for — the first *fully* visible row at the
 * top, then everything down to the last partially visible row at the bottom. A
 * row clipped at the top (its top is above `scrollTop`) is skipped rather than
 * numbered, because a number on a half-row is not "the list you can see".
 */
export const visibleRowRange = (
  spans: readonly (RowSpan | null)[],
  scrollTop: number,
  viewportHeight: number,
): VisibleRowRange => {
  const bottom = scrollTop + viewportHeight;
  let start = -1;
  let end = -1;
  spans.forEach((span, index) => {
    if (!span) return;
    const rowBottom = span.top + span.height;
    if (rowBottom <= scrollTop + 0.5 || span.top >= bottom - 0.5) return;
    if (start === -1) start = index;
    end = index + 1;
  });
  if (start === -1) return { start: spans.length, end: spans.length };
  // Skip a row whose top is clipped by the scroller's own edge.
  while (start < end && (spans[start]?.top ?? scrollTop) < scrollTop - 0.5) start += 1;
  if (start >= end) start = Math.max(0, end - 1);
  return { start, end };
};

/**
 * The numbered `select_result` slots for the visible rows — the single source
 * of the `⌘N` → row mapping, for the badges and the key handler alike.
 *
 * The family is `1`-`9` plus `0` — one digit behind the modifiers, see
 * `matchesResultShortcut`. R34 · the numbers follow the **scroll viewport**:
 * `1`-`9` are the first nine runnable rows inside `visible`, so scrolling
 * renumbers the list to what is on screen ("what you see is what you select").
 * Rows outside the range carry no badge, and `⌘N` cannot reach them.
 *
 * R36 · the family grew a tenth key, `0`, right beside `9` on the number row.
 *
 * R37 · the assignment is **purely in order**: the tenth runnable row in the
 * viewport takes `0` and nothing is reserved. R36 had given `0` to an appended
 * fixed clipboard row outside the viewport numbering (and `FIXED_TAIL_SLOT`
 * retired with it); the user rejected the special case — 「它不应该是个特例，
 * 应该和其他项一样匹配了才显示，同时快捷键也要按顺序安排」 — so the key is
 * just the next number in the sequence, whether the row it lands on is a plugin
 * entry, a matched command or the clipboard.
 *
 * A list longer than the viewport simply stops handing out badges past the
 * tenth visible row: `1`-`9` then `0`, and every row outside `visible` carries
 * `null`.
 *
 * Omitting `visible` numbers the whole list, which is the pre-R34 behaviour and
 * the right answer for a list that fits (no scroll, so the viewport is the
 * list).
 */
export const resultShortcutSlots = (
  rows: readonly LauncherItem[],
  runnableFlags: readonly boolean[],
  visible?: VisibleRowRange,
): Array<number | null> => {
  const start = visible ? Math.max(0, Math.floor(visible.start)) : 0;
  const end = visible ? Math.min(rows.length, Math.ceil(visible.end)) : rows.length;
  let next = 0;
  return rows.map((_, index) => {
    if (!runnableFlags[index]) return null;
    if (index < start || index >= end) return null;
    next += 1;
    if (next <= MAX_VIEWPORT_SLOT) return next;
    // The tenth runnable row in the viewport takes the family's last key, `0`.
    // Past it there is no eleventh digit to hand out.
    return next === MAX_RESULTS ? LAST_RESULT_SLOT : null;
  });
};

/**
 * The row a numbered shortcut runs: the index in the composed list whose slot
 * is `slot`, or `-1` when no visible row carries that number.
 *
 * Every `⌘N` press goes through here, so the key and the badge it matches can
 * never be derived from two different rules.
 */
export const resultIndexForSlot = (
  slots: readonly (number | null)[],
  slot: number,
): number => slots.indexOf(slot);
