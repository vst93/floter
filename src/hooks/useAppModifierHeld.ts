// R60 · Whether the app modifier is held down right now.
//
// The launcher's ⌘-held terminal row (see `launcher/terminal-row.ts`) needs one
// fact the app did not track before: the modifier being *down*, as opposed to
// being part of a completed chord. The rest of the app only ever asks "was this
// key press a Cmd+N / Cmd+Enter", which a single `KeyboardEvent` answers; a row
// that exists while a key is *held* needs the key's own lifetime.
//
// The normalization is the same one `shortcuts.ts` uses for every binding: ⌘ on
// macOS, Ctrl everywhere else. One platform, one modifier — the row does not
// appear on macOS for a bare Ctrl press (Ctrl is not a launcher modifier there)
// and does not appear on Windows/Linux for a bare ⊞/Super press.
import { useEffect, useState } from "react";
import { IS_MAC } from "../shortcuts";
import { appModifierHeld } from "../launcher/terminal-row";

/**
 * Whether the app modifier is down.
 *
 * Listening is deliberately window-level and capture-phase, and it never calls
 * `preventDefault`: the row is a passive offer, and the input must keep every
 * press it already owns (the key handler still sees the same events, in the same
 * order, from its own listeners).
 *
 * The three listeners cover the three ways the modifier can stop being down:
 *
 *   * `keydown` / `keyup` — the ordinary press and release. The state is read
 *     from the event rather than toggled, so a missed release (the window losing
 *     focus mid-chord) is corrected by the very next key of any kind.
 *   * `blur` — the window went away with the key still down (a ⌘-Tab, a native
 *     reveal, the launcher hidden). Nothing will deliver that `keyup`, so the
 *     row's premise is re-derived: not held.
 */
export function useAppModifierHeld(): boolean {
  const [held, setHeld] = useState(false);
  useEffect(() => {
    const sync = (event: KeyboardEvent) => {
      const next = appModifierHeld(event, IS_MAC);
      // The functional form keeps a press/release storm from scheduling a
      // render per event: an unchanged value bails out of the update.
      setHeld((current) => (current === next ? current : next));
    };
    const release = () => setHeld(false);
    window.addEventListener("keydown", sync, true);
    window.addEventListener("keyup", sync, true);
    window.addEventListener("blur", release);
    return () => {
      window.removeEventListener("keydown", sync, true);
      window.removeEventListener("keyup", sync, true);
      window.removeEventListener("blur", release);
    };
  }, []);
  return held;
}
