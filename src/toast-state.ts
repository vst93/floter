// Pure state helpers behind the app-level toast stack. Kept free of React and
// DOM so the node test suite can exercise the queueing rules directly.

export type ToastKind = "error" | "success";

/**
 * The optional action a toast can carry.
 *
 * It exists because a plugin page's failures arrive here as data, not as
 * already-wired host code (see `BridgeNotify` in `plugin-pages.ts`): the host
 * is what knows the iframe to message, so it builds the closure as it raises
 * the toast. A toast without a `run` is a plain informational one and paints
 * no action control.
 */
export type ToastAction = {
  /** User-visible label. Already translated by the caller, which owns the
   * language the toast is painted in. */
  label: string;
  run: () => void;
};

export type AppToast = {
  id: number;
  kind: ToastKind;
  text: string;
  /** Rendered as one button beside the dismissal; absent on plain toasts. */
  action?: ToastAction;
};

/** Newest toasts win; older ones fall off the front so the stack never grows
 * past a glanceable size. */
export const MAX_TOASTS = 3;

/** How long each toast stays before auto-dismissing. Errors linger because
 * they carry information the user may want to act on. */
export const TOAST_DISMISS_MS: Record<ToastKind, number> = {
  error: 8000,
  success: 4000,
};

/** Append a toast, trimming the oldest beyond [`MAX_TOASTS`]. */
export const appendToast = (toasts: AppToast[], toast: AppToast): AppToast[] =>
  [...toasts, toast].slice(-MAX_TOASTS);

/** Drop one toast by id; unknown ids leave the list untouched. */
export const removeToast = (toasts: AppToast[], id: number): AppToast[] =>
  toasts.filter((toast) => toast.id !== id);
