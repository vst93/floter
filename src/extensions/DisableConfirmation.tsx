import { AlertCircle, PowerOff } from "lucide-react";
import { useEffect, useRef } from "react";
import type { Translate } from "../i18n";
import type { Extension } from "../ExtensionsPanel";

/**
 * R100 · the reversible sibling of `RemovalConfirmation`.
 *
 * Disabling an integration is a *mark*, not a delete: the backend writes the
 * `enabled` flag, stops any in-flight run (R98), and touches no file. Because
 * it is reversible, the confirmation looks and reads differently from a
 * removal — a neutral notice band instead of the error band, a plain action
 * button instead of the danger fill — while keeping the same interaction
 * contract the removal bar established:
 *
 *   - the Cancel button takes focus on open and gets it back on dismiss;
 *   - Escape cancels from anywhere (capture, so a surrounding dialog's own
 *     Escape handler cannot swallow the press first);
 *   - the confirm button is disabled while a mutation is in flight;
 *   - `data-destructive-confirm` marks the affirmative button so the global
 *     Enter handler leaves the press to the button.
 *
 * It is a *sibling* rather than a `variant` of `RemovalConfirmation` because
 * R100 only verifies the removal chain: keeping that component byte-identical
 * means the removal confirmation's behavior cannot regress behind this round.
 * The shared contract is pinned by the R100 suite, not by inheritance.
 */
type Props = {
  extension: Extension;
  busy: boolean;
  t: Translate;
  onCancel: () => void;
  onConfirm: () => void;
};

export function DisableConfirmation({ extension, busy, t, onCancel, onConfirm }: Props) {
  const cancelRef = useRef<HTMLButtonElement>(null);
  const onCancelRef = useRef(onCancel);
  onCancelRef.current = onCancel;
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    // Focus without scroll to prevent layout jump when confirmation appears
    cancelRef.current?.focus({ preventScroll: true });
    const escape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.isComposing || event.keyCode === 229) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      onCancelRef.current();
    };
    document.addEventListener("keydown", escape, true);
    return () => {
      document.removeEventListener("keydown", escape, true);
      // Restore focus without scroll to prevent jump when confirmation dismisses
      if (previous?.isConnected && previous.getClientRects().length) {
        previous.focus({ preventScroll: true });
      }
    };
  }, []);
  return (
    <div className="extensions-notice extension-discard-bar" role="alert">
      <AlertCircle size={15} aria-hidden="true" />
      <span>
        {t("settings.extensions.disableTitle", { name: extension.name })}{" "}
        {t("settings.extensions.disableDescription")}
      </span>
      <button ref={cancelRef} type="button" className="extensions-action-button" onClick={onCancel}>{t("settings.extensions.cancel")}</button>
      <button type="button" data-destructive-confirm className="extensions-action-button" disabled={busy} onClick={onConfirm}>
        <PowerOff size={14} />{t("settings.extensions.disable")}
      </button>
    </div>
  );
}
