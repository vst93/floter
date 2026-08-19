import { AlertCircle, Trash2, Unplug } from "lucide-react";
import type { MouseEvent, RefObject } from "react";
import type { Translate } from "../i18n";
import type { Extension } from "../ExtensionsPanel";

type Props = { extension: Extension; busy: boolean; t: Translate; onCancel: () => void; onConfirm: () => void; stopPropagation: (event: MouseEvent) => void; textKey: (extension: Extension, suffix: "" | "Title" | "Description") => Parameters<Translate>[0]; dialogRef: RefObject<HTMLElement | null> };

export function RemovalDialog({ extension, busy, t, onCancel, onConfirm, stopPropagation, textKey, dialogRef }: Props) {
  const system = extension.distributionSource === "built-in" && extension.runtimeOwnership === "system";
  return <div className="extension-permission-backdrop" role="presentation" onMouseDown={() => { if (!busy) onCancel(); }}><section ref={dialogRef} className="extension-removal-dialog" role="alertdialog" aria-modal="true" aria-labelledby="extension-removal-title" tabIndex={-1} onMouseDown={stopPropagation}>
    <header><AlertCircle size={19} strokeWidth={2} aria-hidden="true" /><div><h3 id="extension-removal-title">{t(textKey(extension, "Title"), { name: extension.name })}</h3><p>{t(textKey(extension, "Description"))}</p></div></header>
    {extension.generatedCustom && <div className="extension-removal-dialog__warning">{t("settings.extensions.deleteCustomWarning")}</div>}
    <footer><button type="button" className="extensions-action-button" disabled={busy} onClick={onCancel}>{t("settings.extensions.cancel")}</button><button type="button" className="extensions-action-button extensions-action-button--danger" data-dialog-initial disabled={busy} onClick={onConfirm}>{system ? <Unplug size={14} strokeWidth={2} /> : <Trash2 size={14} strokeWidth={2} />}{busy ? t("settings.extensions.removing") : t(textKey(extension, ""))}</button></footer>
  </section></div>;
}
