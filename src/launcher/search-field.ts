// R37 · the launcher's search field is one surface with one height.
//
// The user's report, verbatim: 「还有头部的输入框整体高度小了些，可以和设置页面头部
// 一样高，这样切换时一体性更好，包括其他内置插件的数据框高度应该都保持一致，他们
// 应该是在主搜索框基础上演变，应该公用一些组件，做到更强的一体性和平台化」.
//
// Three boxes were in play, and they were three *different* numbers:
//
//   · the ordinary launcher's field row — `.collapsed-card__input-row`, 42u
//     after R23's three-round trim;
//   · a plugin mode's field — R31 lifted the mode out of the query text and
//     made it explicit state, so this is the *same* DOM as the row above (the
//     scope glyph and the trailing gear are the only difference). There was
//     never a second field, only a second face on the one row; and
//   · the settings surface's top band — `.settings-card__header`, 56u.
//
// Switching between the launcher and settings therefore moved the top edge of
// the window's first band by 14u. R37 makes the launcher row the settings
// band's height, so the two surfaces' first band is one continuous line.
//
// This module is the one place the height is *written down as a number*. The
// sheets spell the same value as `calc(var(--u) * 56)` — they must, because a
// CSS length cannot import a TS constant, and because `ui-scale.test.ts` reads
// the literal off the sheet — and `tests/r37-field-height.test.ts` pins the two
// sheets and this constant to each other so the three cannot drift.
//
// What the height is *made of*, at the default interface step:
//
//   `.collapsed-card__input-row`   56u  the band (this constant)
//     └─ `.collapsed-card__input`  22u  the field's line box, flex-centred,
//                                       so 17u of dead height a side
//   `.settings-card__header`       56u  the same band, same centring
//
// The row's *content* box does not change: the field keeps its pinned 22u line
// box (R20), and the row's own `align-items: center` is what turns the extra
// 14u into 7u above and 7u below the line — no padding is added, so the caret
// stays on the row's centre and the scope glyph, the chips subline and the
// trailing gear keep the centring they always had.
//
// The budget half of the round reads {@link SEARCH_FIELD_HEIGHT_UNITS} rather
// than restating 56: the field row is the first segment of the window's height
// (see `result-budget.ts`), so a field that grows and a slab that does not is
// a card taller than its window.

/** The field band's height in `--u` units: the launcher's input row and the
 *  settings card's header, so the two surfaces' first band is the same box.
 *  R23 held the launcher row at 42u (56u before it); R37 restores the 56u to
 *  match the settings band the user pointed at. */
export const SEARCH_FIELD_HEIGHT_UNITS = 56;

/** The field's own line box inside the band, in units. Unchanged by R37: the
 *  band grows around it, the text does not move off the pinned metric. */
export const SEARCH_FIELD_LINE_UNITS = 22;

/** The dead height the band leaves above (and below) the line box. The row is
 *  flex-centred, so this is one side's share of the difference. R37: 17u a
 *  side, where R23's 42u row left 10u. */
export const searchFieldBreathUnits = (
  band: number = SEARCH_FIELD_HEIGHT_UNITS,
): number => (band - SEARCH_FIELD_LINE_UNITS) / 2;
