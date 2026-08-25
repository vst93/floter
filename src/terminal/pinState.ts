// Pure state logic behind the pinnable floating terminal card.
//
// Kept free of React and Tauri imports so the reducer, the geometry clamp and
// the persistence round-trip can be unit-tested directly under `node --test`
// (see `tests/pinned-terminal.test.ts`). The React side lives in
// `src/hooks/usePinnedTerminal.ts` and `src/terminal/PinnedTerminalCard.tsx`.

/**
 * Frontend id under which the pinned card's view is registered in the Rust
 * terminal manager. Frames arrive as `term://frame` events carrying this id,
 * exactly like the main view's `"main"`.
 */
export const PINNED_SESSION_ID = "pinned";

/** One pinned session: which daemon PTY it shows and which view generation
 * its frames must carry (stale generations are ignored, mirroring how the
 * main view filters its own stream). */
export type PinnedSession = {
  brokerSessionId: string;
  generation: number;
  /** Session name from the broker, when known; null until looked up. */
  label: string | null;
};

export type PinState =
  | { status: "idle" }
  | { status: "pinned"; session: PinnedSession };

export type PinEvent =
  /** Pin (or, while something is already pinned, replace) with this session. */
  | { type: "pin"; brokerSessionId: string; generation: number; label?: string | null }
  /** Card dismissed; the session returns to the normal list/view flow. */
  | { type: "unpin" }
  /** The underlying PTY exited. Only closes the card that shows it. */
  | { type: "sessionClosed"; generation: number }
  /** Broker session name resolved after the pin completed. */
  | { type: "label"; label: string };

/** Only one card exists at a time, so "pin" onto a pinned state is a replace,
 * and exit events for anything but the current generation are ignored — they
 * belong to a card that has already been replaced or unpinned. */
export function pinReducer(state: PinState, event: PinEvent): PinState {
  switch (event.type) {
    case "pin":
      return {
        status: "pinned",
        session: {
          brokerSessionId: event.brokerSessionId,
          generation: event.generation,
          label: event.label ?? null,
        },
      };
    case "unpin":
      return { status: "idle" };
    case "sessionClosed":
      return state.status === "pinned" && state.session.generation === event.generation
        ? { status: "idle" }
        : state;
    case "label":
      return state.status === "pinned"
        ? { status: "pinned", session: { ...state.session, label: event.label } }
        : state;
  }
}

// ---- geometry -------------------------------------------------------------

export interface CardGeometry {
  x: number;
  y: number;
  width: number;
  height: number;
}

export const CARD_MIN_WIDTH = 320;
export const CARD_MIN_HEIGHT = 180;
export const CARD_DEFAULT_WIDTH = 460;
export const CARD_DEFAULT_HEIGHT = 280;
export const CARD_MARGIN = 24;

const clampNumber = (value: number, min: number, max: number): number =>
  Math.min(Math.max(value, min), max);

const finiteOr = (value: unknown, fallback: number): number =>
  typeof value === "number" && Number.isFinite(value) ? value : fallback;

/**
 * Keep the card fully inside the window.
 *
 * Size first — clamped to [min, window bounds] — then position, so the card
 * never drifts out of view during drags, resizes, or the native launcher ↔
 * terminal window-size changes. When the window itself is smaller than the
 * minimum size the minimum wins (the card overflows by design rather than
 * becoming unusable); position still clamps to the top-left corner.
 */
export function clampCardGeometry(
  geometry: CardGeometry,
  boundsWidth: number,
  boundsHeight: number,
): CardGeometry {
  const maxWidth = Math.max(CARD_MIN_WIDTH, boundsWidth);
  const maxHeight = Math.max(CARD_MIN_HEIGHT, boundsHeight);
  const width = clampNumber(finiteOr(geometry.width, CARD_DEFAULT_WIDTH), CARD_MIN_WIDTH, maxWidth);
  const height = clampNumber(
    finiteOr(geometry.height, CARD_DEFAULT_HEIGHT),
    CARD_MIN_HEIGHT,
    maxHeight,
  );
  const x = clampNumber(finiteOr(geometry.x, 0), 0, Math.max(0, boundsWidth - width));
  const y = clampNumber(finiteOr(geometry.y, 0), 0, Math.max(0, boundsHeight - height));
  return { x, y, width, height };
}

/** Bottom-right placement used before the user has ever moved the card. */
export function defaultCardGeometry(boundsWidth: number, boundsHeight: number): CardGeometry {
  return clampCardGeometry(
    {
      x: boundsWidth - CARD_DEFAULT_WIDTH - CARD_MARGIN,
      y: boundsHeight - CARD_DEFAULT_HEIGHT - CARD_MARGIN,
      width: CARD_DEFAULT_WIDTH,
      height: CARD_DEFAULT_HEIGHT,
    },
    boundsWidth,
    boundsHeight,
  );
}

// ---- persistence ----------------------------------------------------------

const PINNED_GEOMETRY_STORAGE_KEY = "floter.pinned-terminal.geometry";

type GeometryStorage = Pick<Storage, "getItem" | "setItem">;

/** Read a previously saved geometry; null when absent or malformed. The result
 * is sanitized to sane sizes but NOT window-clamped — the caller does that
 * once the current window bounds are known. */
export function loadPinnedGeometry(storage: GeometryStorage | null): CardGeometry | null {
  if (!storage) return null;
  try {
    const raw = storage.getItem(PINNED_GEOMETRY_STORAGE_KEY);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null) return null;
    const record = parsed as Record<string, unknown>;
    const candidate: CardGeometry = {
      x: finiteOr(record.x, NaN),
      y: finiteOr(record.y, NaN),
      width: finiteOr(record.width, NaN),
      height: finiteOr(record.height, NaN),
    };
    if (
      !Number.isFinite(candidate.x) ||
      !Number.isFinite(candidate.y) ||
      !Number.isFinite(candidate.width) ||
      !Number.isFinite(candidate.height)
    ) {
      return null;
    }
    // Drop nonsensical saved sizes; positions are re-clamped against the live
    // window by the caller anyway.
    return clampCardGeometry({
      ...candidate,
      width: Math.max(candidate.width, 40),
      height: Math.max(candidate.height, 40),
    }, 100000, 100000);
  } catch {
    return null;
  }
}

/** Best-effort write; quota or serialization failures never break the UI. */
export function savePinnedGeometry(storage: GeometryStorage | null, geometry: CardGeometry): void {
  if (!storage) return;
  try {
    storage.setItem(PINNED_GEOMETRY_STORAGE_KEY, JSON.stringify(geometry));
  } catch {
    // Ignore: an unsaved position only costs the user their last drag.
  }
}
