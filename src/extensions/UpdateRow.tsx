import { ChevronRight, LoaderCircle, Package, ShieldCheck } from "lucide-react";
import type { Translate } from "../i18n";
import type { Extension, UpdateCandidate } from "../ExtensionsPanel";

type Props = {
  extension: Extension;
  update: UpdateCandidate;
  busy: boolean;
  disabled: boolean;
  t: Translate;
  onOpen: () => void;
  onUpdate: () => void;
};

export function UpdateRow({ extension, update, busy, disabled, t, onOpen, onUpdate }: Props) {
  const channel = t(extension.channel === "beta"
    ? "settings.extensions.channelBeta"
    : "settings.extensions.channelStable");
  const signature = t(extension.signatureVerified
    ? "settings.extensions.signatureVerified"
    : "settings.extensions.signatureMissing");

  return (
    <article className="extension-update-row" data-update-kind={update.kind}>
      <span className="extension-update-row__icon">
        <Package size={16} strokeWidth={2} aria-hidden="true" />
      </span>
      <button type="button" className="extension-update-row__main" onClick={onOpen}>
        <span className="extension-update-row__heading">
          <strong>{extension.name}</strong>
          <span>{extension.packageName ?? extension.id}</span>
        </span>
        <span className="extension-update-row__detail">
          <span className="extension-update-row__versions">
            <span>v{extension.currentVersion}</span>
            <ChevronRight size={13} strokeWidth={2} aria-hidden="true" />
            <span>v{update.version}</span>
          </span>
          <span>{extension.publisherName}</span>
          <span>{channel}</span>
          <span className={extension.signatureVerified ? "extension-update-row__verified" : undefined}>
            {extension.signatureVerified && <ShieldCheck size={11} strokeWidth={2} aria-hidden="true" />}
            {signature}
          </span>
        </span>
      </button>
      <button
        type="button"
        className="extensions-action-button extensions-action-button--primary"
        aria-busy={busy}
        disabled={disabled}
        onClick={onUpdate}
      >
        {busy && <LoaderCircle className="extensions-spinner" size={13} strokeWidth={2} aria-hidden="true" />}
        {busy ? t("settings.extensions.updating") : t("settings.extensions.update")}
      </button>
    </article>
  );
}
