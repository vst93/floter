// R7-13a · Window geometry has one source per language, and the two are pinned
// against each other.
//
// `INPUT_WINDOW_WIDTH` used to be declared four times: once in
// `useLauncherHeight.ts`, once in `App.tsx`, and twice on the Rust side
// (`src-tauri/src/lib.rs`, where the non-Windows and Windows heights also live).
// The frontend copies were a coincidence of copy-paste — nothing made the
// launcher's collapsed resize, its settings resize and the native
// `show_input` activation agree. This module is the frontend single source; the
// Rust const stays where it is because Rust cannot import TypeScript, and
// `tests/window-contract.test.ts` compares the two so a change to either side
// alone turns red.
//
// The width is the launcher's fixed column. Height is *not* declared here and
// has no frontend counterpart: the collapsed card is measured at runtime
// (`syncLauncherHeight`) and the native side only needs a height before the
// first frontend resize lands, so Rust owns it alone.

/** Width of the launcher window, in logical pixels. Mirrors
 *  `INPUT_WINDOW_WIDTH` in `src-tauri/src/lib.rs`. */
export const INPUT_WINDOW_WIDTH = 720;
