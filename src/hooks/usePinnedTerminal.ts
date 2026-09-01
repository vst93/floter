// React binding for the pin-card state in `src/terminal/pinState.ts`.
//
// Owns the reducer plus the card geometry (default → restore → persist →
// re-clamp on window resize). All terminal/IPC concerns stay in `App.tsx` and
// `PinnedTerminalCard.tsx`; this hook is deliberately dumb state plumbing.

import { useCallback, useEffect, useReducer, useRef, useState } from "react";
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

/** Idle gap after the last move/resize before the geometry is written, matching
 * `TERMINAL_SIZE_SAVE_DELAY` in `useTerminalView.ts`. */
const GEOMETRY_SAVE_DELAY = 280;

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
  //
  // The saved value is clamped against the CURRENT window before it is used.
  // `loadPinnedGeometry` deliberately sanitizes sizes without knowing the
  // bounds, so a position saved on a larger display — or simply in the wider
  // terminal window, when the app restores into the collapsed launcher — would
  // otherwise place the card partly or wholly off-screen, with its header (the
  // only way to drag it back) out of reach. `defaultCardGeometry` already
  // clamps its own result.
  const [geometry, setGeometry] = useState<CardGeometry>(() => {
    const saved = loadPinnedGeometry(window.localStorage);
    return saved
      ? clampCardGeometry(saved, window.innerWidth, window.innerHeight)
      : defaultCardGeometry(window.innerWidth, window.innerHeight);
  });

  // Persist after every committed change; failures are swallowed upstream.
  //
  // Debounced, because a drag or a resize commits a new geometry on every
  // pointer event: writing synchronously there put a JSON serialization and a
  // localStorage round-trip (which blocks) between each frame of the gesture.
  // Only the geometry the gesture settles on is worth keeping, so the write is
  // deferred until the movement stops, exactly as the terminal window's own
  // size persistence does it.
  const saveTimer = useRef<number | null>(null);
  const pendingGeometry = useRef<CardGeometry | null>(null);
  useEffect(() => {
    pendingGeometry.current = geometry;
    if (saveTimer.current !== null) window.clearTimeout(saveTimer.current);
    saveTimer.current = window.setTimeout(() => {
      saveTimer.current = null;
      const pending = pendingGeometry.current;
      pendingGeometry.current = null;
      if (pending) savePinnedGeometry(window.localStorage, pending);
    }, GEOMETRY_SAVE_DELAY);
  }, [geometry]);

  // A pending write must not be lost to an unmount (app teardown mid-drag).
  useEffect(
    () => () => {
      if (saveTimer.current === null) return;
      window.clearTimeout(saveTimer.current);
      saveTimer.current = null;
      const pending = pendingGeometry.current;
      pendingGeometry.current = null;
      if (pending) savePinnedGeometry(window.localStorage, pending);
    },
    [],
  );

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
