// Pure state helpers behind the app-level toast stack. Kept free of React and
// DOM so the node test suite can exercise the queueing rules directly.

export type ToastKind = "error" | "success";

export type AppToast = {
  id: number;
  kind: ToastKind;
  text: string;
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
