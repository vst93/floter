// React binding for the pin-card state in `src/terminal/pinState.ts`.
//
// Owns the reducer plus the card geometry (default → restore → persist →
// re-clamp on window resize). All terminal/IPC concerns stay in `App.tsx` and
// `PinnedTerminalCard.tsx`; this hook is deliberately dumb state plumbing.

import { useCallback, useEffect, useReducer, useState } from "react";
import {
  clampCardGeometry,
  defaultCardGeometry,
  loadPinnedGeometry,
  pinReducer,
  savePinnedGeometry,
  type CardGeometry,
  type PinEvent,
  type PinState,
} from "../terminal/pinState";

export function usePinnedTerminal(): {
  pinState: PinState;
  dispatchPinEvent: (event: PinEvent) => void;
  geometry: CardGeometry;
  /** Move/resize the card; the new geometry is clamped to the current window. */
  updateGeometry: (next: CardGeometry) => void;
} {
  const [pinState, dispatchPinEvent] = useReducer(pinReducer, { status: "idle" } as PinState);

  // Lazy initial state: a saved position wins, otherwise bottom-right of
  // whatever the window measures at mount.
  const [geometry, setGeometry] = useState<CardGeometry>(() =>
    loadPinnedGeometry(window.localStorage) ??
    defaultCardGeometry(window.innerWidth, window.innerHeight),
  );

  // Persist after every committed change; failures are swallowed upstream.
  useEffect(() => {
    savePinnedGeometry(window.localStorage, geometry);
  }, [geometry]);

  // Native launcher ↔ terminal transitions and user window resizing both end
  // up here; keep the card inside whatever bounds are current.
  useEffect(() => {
    const onResize = () => {
      setGeometry((current) =>
        clampCardGeometry(current, window.innerWidth, window.innerHeight),
      );
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  const updateGeometry = useCallback((next: CardGeometry) => {
    setGeometry(clampCardGeometry(next, window.innerWidth, window.innerHeight));
  }, []);

  return { pinState, dispatchPinEvent, geometry, updateGeometry };
}
