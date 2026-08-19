import { ChevronRight, LoaderCircle } from "lucide-react";
import type { Translate } from "../i18n";
import type { Extension, UpdateCandidate } from "../ExtensionsPanel";

type Props = { extension: Extension; update: UpdateCandidate; busy: boolean; t: Translate; onOpen: () => void; onUpdate: () => void };

export function UpdateRow({ extension, update, busy, t, onOpen, onUpdate }: Props) {
  return <article className="extension-update-row" data-update-kind={update.kind}>
    <button type="button" className="extension-update-row__main" onClick={onOpen}><strong>{extension.name}</strong><span className="extension-update-row__versions"><span>v{extension.currentVersion}</span><ChevronRight size={13} strokeWidth={2} aria-hidden="true" /><span>v{update.version}</span></span></button>
    <button type="button" className="extensions-action-button extensions-action-button--primary" aria-busy={busy} disabled={busy} onClick={onUpdate}>{busy && <LoaderCircle className="extensions-spinner" size={13} strokeWidth={2} aria-hidden="true" />}{busy ? t("settings.extensions.updating") : t("settings.extensions.update")}</button>
  </article>;
}
