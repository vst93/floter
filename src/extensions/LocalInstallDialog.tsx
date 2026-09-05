import { ShieldCheck } from "lucide-react";
import type { MouseEvent, RefObject } from "react";
import type { Translate } from "../i18n";
import type { InstallRequest, PermissionReview } from "../ExtensionsPanel";

export type PendingLocal = { review: PermissionReview; request: InstallRequest; name: string; runtime: string; platforms: string[]; source: string };

type LocalProps = { pending: PendingLocal; busy: boolean; t: Translate; onCancel: () => void; onConfirm: () => void; stopPropagation: (event: MouseEvent) => void; dialogRef: RefObject<HTMLElement | null> };

export function LocalInstallDialog({ pending, busy, t, onCancel, onConfirm, stopPropagation, dialogRef }: LocalProps) {
  return <div className="extension-permission-backdrop" role="presentation" onMouseDown={onCancel}><section ref={dialogRef} className="extension-permission-dialog" role="dialog" aria-modal="true" aria-labelledby="extension-local-title" tabIndex={-1} onMouseDown={stopPropagation}>
    <header><ShieldCheck size={18} strokeWidth={2} aria-hidden="true" /><div><h3 id="extension-local-title">{t("settings.extensions.localConfirmTitle", { name: pending.name })}</h3><p>{t("settings.extensions.localConfirmHint")}</p></div></header>
    <div className="extension-permission-list"><div><strong>{t("settings.extensions.localSource")}</strong><span>{pending.source === "local" ? t("settings.extensions.localDirectory") : pending.source}</span></div><div><strong>{t("settings.extensions.localRuntime")}</strong><span>{pending.runtime}</span></div><div><strong>{t("settings.extensions.customPlatforms")}</strong><span>{pending.platforms.join(", ") || t("settings.extensions.unavailable")}</span></div>{pending.review.permissions.map((permission) => <div key={permission.permission}><strong>{permission.title}</strong><span>{permission.description}</span></div>)}<p className="extension-permission-dialog__boundary">{t("settings.extensions.permissionReviewBoundary")}</p></div>
    <footer><button type="button" className="extensions-action-button" data-dialog-initial onClick={onCancel}>{t("settings.extensions.cancel")}</button><button type="button" className="extensions-action-button extensions-action-button--primary" disabled={busy} onClick={onConfirm}>{busy ? t("settings.extensions.installing") : t("settings.extensions.localConfirm")}</button></footer>
  </section></div>;
}
