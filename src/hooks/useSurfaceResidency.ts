// R35 · the residency clock, held in a ref so the once-registered reveal
// listener can read it without being rebuilt.
//
// The pure arithmetic lives in `src/surface-residency.ts`; this hook only owns
// *where* the clock is stored and the three operations the app needs:
//
//   * `note(surface)` — the app entered a surface (or returned to the search
//     page, `null`). Restarts the clock for a new surface, drops it for none.
//   * `refresh()` — the surface was re-confirmed (a summon landed back inside
//     it), so the window restarts. This is what keeps the residency forgiving:
//     a user who comes back is clearly still in the surface.
//   * `holds()` — whether the current clock is still inside its window. The
//     reveal handler asks this; a false answer restores the pre-R35 reset.
//
// The duration is read through a ref, so changing the setting takes effect on
// the next `note`/`refresh` without rebuilding any callback (the reveal
// listener closes over the hook's return value once, at mount).

import { useCallback, useRef } from "react";
import {
  residencyHolds,
  startResidency,
  type ResidencyClock,
  type ResidencySurface,
} from "../surface-residency";

export type SurfaceResidency = {
  note: (surface: ResidencySurface | null) => void;
  refresh: () => void;
  holds: () => boolean;
  current: () => ResidencySurface | null;
  clear: () => void;
};

export function useSurfaceResidency(seconds: number): SurfaceResidency {
  const clockRef = useRef<ResidencyClock | null>(null);
  // Read at the moment a clock starts, so a settings change lands on the next
  // entry rather than on whatever value the callbacks happened to capture.
  const secondsRef = useRef(seconds);
  secondsRef.current = seconds;

  const note = useCallback((surface: ResidencySurface | null) => {
    clockRef.current =
      surface === null ? null : startResidency(surface, Date.now(), secondsRef.current);
  }, []);

  const refresh = useCallback(() => {
    const clock = clockRef.current;
    if (!clock) return;
    clockRef.current = startResidency(clock.surface, Date.now(), secondsRef.current);
  }, []);

  const holds = useCallback(() => residencyHolds(clockRef.current, Date.now()), []);

  const current = useCallback(() => clockRef.current?.surface ?? null, []);

  const clear = useCallback(() => {
    clockRef.current = null;
  }, []);

  return { note, refresh, holds, current, clear };
}
