import { AlertCircle, Trash2, Unplug } from "lucide-react";
import { useEffect, useRef } from "react";
import type { Translate } from "../i18n";
import type { Extension } from "../ExtensionsPanel";

type Props = {
  extension: Extension;
  busy: boolean;
  t: Translate;
  onCancel: () => void;
  onConfirm: () => void;
  textKey: (extension: Extension, suffix: "" | "Title" | "Description") => Parameters<Translate>[0];
};

export function RemovalConfirmation({ extension, busy, t, onCancel, onConfirm, textKey }: Props) {
  const cancelRef = useRef<HTMLButtonElement>(null);
  const onCancelRef = useRef(onCancel);
  onCancelRef.current = onCancel;
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    cancelRef.current?.focus({ preventScroll: true });
    cancelRef.current?.scrollIntoView({ block: "nearest" });
    const escape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.isComposing || event.keyCode === 229) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      onCancelRef.current();
    };
    document.addEventListener("keydown", escape, true);
    return () => {
      document.removeEventListener("keydown", escape, true);
      if (previous?.isConnected && previous.getClientRects().length) previous.focus({ preventScroll: true });
    };
  }, []);
  const system = extension.distributionSource === "built-in" && extension.runtimeOwnership === "system";
  return (
    <div className="extensions-notice extensions-notice--error extension-discard-bar" role="alert">
      <AlertCircle size={15} aria-hidden="true" />
      <span>{t(textKey(extension, "Title"), { name: extension.name })}{extension.generatedCustom ? ` ${t("settings.extensions.deleteCustomWarning")}` : ""}</span>
      <button ref={cancelRef} type="button" className="extensions-action-button" onClick={onCancel}>{t("settings.extensions.cancel")}</button>
      <button type="button" data-destructive-confirm className="extensions-action-button extensions-action-button--danger" disabled={busy} onClick={onConfirm}>
        {system ? <Unplug size={14} /> : <Trash2 size={14} />}{t(textKey(extension, ""))}
      </button>
    </div>
  );
}
