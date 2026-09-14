import { useEffect } from "react";
import { createPortal } from "react-dom";
import { AlertCircle, Check, X } from "lucide-react";
import type { Translate } from "../i18n";
import { TOAST_DISMISS_MS, type AppToast } from "../toast-state";

/** Container the toast stack renders into. App.tsx mounts it as a stable
 * sibling of the mode shell (and of the plugin layer), so it survives mode
 * switches without unmounting. The element carries the current surface in
 * `data-surface`, which `#floter-app-toasts` uses to position the stack for
 * each window geometry (see extensions.css). */
export const TOAST_PORTAL_ID = "floter-app-toasts";

/**
 * The app's shared toast stack.
 *
 * Feedback about an action must be visible wherever the user happens to be.
 * The host is a sibling of the mode shell, so it never lands inside a scroll
 * container and stays put while the settings page content scrolls. Its
 * `position: fixed` resolves against the viewport: `#root` (base.css) carries
 * no `will-change` and no transform, and the shells' `will-change` alone does
 * not make its descendants' containing block either — so nothing between the
 * host and the viewport qualifies as a containing block. The viewport's height
 * differs per surface — hence the `data-surface` attribute, which the
 * stylesheet keys off to keep the stack inside the window on every mode.
 */
/**
 * Mount the toast portal target plus the stack. Place this as a stable sibling
 * of the surface shell so it is never caught inside a scroll container.
 */
export function ToastHost({
  toasts,
  t,
  dataSurface,
  onDismiss,
}: {
  toasts: AppToast[];
  t: Translate;
  dataSurface: string;
  onDismiss: (id: number) => void;
}) {
  return (
    <>
      <div id={TOAST_PORTAL_ID} data-surface={dataSurface} />
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
