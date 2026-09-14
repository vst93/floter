import { useEffect } from "react";
import { createPortal } from "react-dom";
import { AlertCircle, Check, X } from "lucide-react";
import type { Translate } from "../i18n";
import { TOAST_DISMISS_MS, type AppToast } from "../toast-state";

/** Container the toast stack renders into. App.tsx mounts it as a direct
 * child of whichever surface card is showing (`settings-card`: the floater
 * body), OUTSIDE the scrollable content area. */
export const TOAST_PORTAL_ID = "floter-app-toasts";

/**
 * The app's shared toast stack.
 *
 * Feedback about an action must be visible wherever the user happens to be.
 * Rendering the stack inside `position: fixed` while the nearest scroll
 * container is the page body makes the toasts scroll away with the content —
 * hence the portal: the host element lives on the card, whose only scrollable
 * descendant is the page content, so the stack stays pinned to the card's
 * top-right corner no matter how far down the list the user is.
 */
/**
 * Mount the toast portal target plus the stack. Place this as a direct child
 * of the surface card (the element that is the containing block for fixed
 * descendants), never inside a scroll container.
 */
export function ToastHost({
  toasts,
  t,
  onDismiss,
}: {
  toasts: AppToast[];
  t: Translate;
  onDismiss: (id: number) => void;
}) {
  return (
    <>
      <div id={TOAST_PORTAL_ID} />
      <ToastStack toasts={toasts} t={t} onDismiss={onDismiss} />
    </>
  );
}

export function ToastStack({
  toasts,
  t,
  onDismiss,
}: {
  toasts: AppToast[];
  t: Translate;
  onDismiss: (id: number) => void;
}) {
  if (!toasts.length) return null;
  const host = typeof document !== "undefined"
    ? document.getElementById(TOAST_PORTAL_ID)
    : null;
  if (!host) return null;
  return createPortal(
    <>
      {toasts.map((toast) => (
        <Toast key={toast.id} toast={toast} t={t} onDismiss={onDismiss} />
      ))}
    </>,
    host,
  );
}

function Toast({
  toast,
  t,
  onDismiss,
}: {
  toast: AppToast;
  t: Translate;
  onDismiss: (id: number) => void;
}) {
  useEffect(() => {
    const timer = window.setTimeout(
      () => onDismiss(toast.id),
      TOAST_DISMISS_MS[toast.kind],
    );
    return () => window.clearTimeout(timer);
  }, [toast.id, toast.kind, onDismiss]);

  return (
    <div
      className={`app-toast app-toast--${toast.kind}`}
      role={toast.kind === "error" ? "alert" : "status"}
    >
      {toast.kind === "error" ? (
        <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
      ) : (
        <Check size={15} strokeWidth={2} aria-hidden="true" />
      )}
      <span>{toast.text}</span>
      <button
        type="button"
        className="app-toast__dismiss"
        aria-label={t("settings.extensions.dismissNotice")}
        onClick={() => onDismiss(toast.id)}
      >
        <X size={13} strokeWidth={2} aria-hidden="true" />
      </button>
    </div>
  );
}
