import {
  Download,
  ExternalLink,
  Link2,
  LoaderCircle,
  MoreHorizontal,
  Package,
  RefreshCw,
  RotateCcw,
  Trash2,
  Unplug,
  Wrench,
} from "lucide-react";
import type { Translate } from "../i18n";
import type { Extension, ExtensionOperation, UpdateCandidate } from "../ExtensionsPanel";

type Props = {
  extension: Extension;
  update?: UpdateCandidate;
  operation: ExtensionOperation;
  t: Translate;
  onOpen: () => void;
  onConnect: () => void;
  onRepair: () => void;
  onReconnect: () => void;
  onToggle: () => void;
  onUpdate: () => void;
  onRollback: () => void;
  onReinstall: () => void;
  onEdit: () => void;
  onUninstall: () => void;
};

const integrationKindKey = (extension: Extension): Parameters<Translate>[0] => {
  if (extension.generatedCustom) return "settings.extensions.integrationKind.custom";
  if (extension.distributionSource === "npm") return "settings.extensions.integrationKind.npm";
  if (extension.distributionSource === "built-in" && extension.runtimeOwnership === "system") {
    return "settings.extensions.integrationKind.system";
  }
  return "settings.extensions.integrationKind.package";
};

const removalKind = (extension: Extension) => {
  if (extension.generatedCustom) return "custom";
  if (extension.distributionSource === "npm") return "npm";
  if (extension.distributionSource === "built-in" && extension.runtimeOwnership === "system") return "system";
  return "package";
};

export function ExtensionRow({
  extension,
  update,
  operation,
  t,
  onOpen,
  onConnect,
  onRepair,
  onReconnect,
  onToggle,
  onUpdate,
  onRollback,
  onReinstall,
  onEdit,
  onUninstall,
}: Props) {
  const updateAvailable = Boolean(update);
  const busy = Boolean(operation);
  const rowBusy = operation?.id === extension.id;
  const rowToggleBusy = rowBusy && (operation?.kind === "enable" || operation?.kind === "disable");
  const rowInstallBusy = rowBusy && operation?.kind === "install";
  const rowRepairBusy = rowBusy && (operation?.kind === "repair" || operation?.kind === "reinstall");
  const rowUpdateBusy = rowBusy && operation?.kind === "update";
  const kind = removalKind(extension);
  const packageIdentity = extension.packageName ?? extension.publisherName;
  const status = extension.connected
    ? t(`settings.extensions.status.${extension.state}`)
    : t("settings.extensions.status.notConnected");

  const rowContent = (
    <>
      <span className="extension-row__icon">
        <Package size={17} strokeWidth={2} aria-hidden="true" />
      </span>
      <span className="extension-row__main">
        <span className="extension-row__title">
          <strong>{extension.name}</strong>
          <span>v{extension.currentVersion}</span>
        </span>
        <span className="extension-row__meta" title={`${packageIdentity} · ${status}`}>
          <span>{packageIdentity}</span>
          <span>{t(integrationKindKey(extension))}</span>
          <span>{t(`settings.extensions.runtimeSource.${extension.runtimeSource}`)}</span>
          <span className={`extension-status extension-status--${extension.state}`}>{status}</span>
          {!extension.runtimeAvailable && (
            <span className="extension-status extension-status--broken">
              {t("settings.extensions.runtimeUnavailable")}
            </span>
          )}
          {updateAvailable && (
            <span className="extension-status extension-status--update">
              {t("settings.extensions.status.updateAvailable")}
            </span>
          )}
        </span>
      </span>
    </>
  );

  return (
    <article className={`extension-row${extension.connected ? "" : " extension-row--detected"}`}>
      {extension.connected ? (
        <button type="button" className="extension-row__open" onClick={onOpen}>
          {rowContent}
        </button>
      ) : (
        <div className="extension-row__open">{rowContent}</div>
      )}

      <div className="extension-row__actions" onClick={(event) => event.stopPropagation()}>
        {!extension.connected ? (
          <button
            type="button"
            className="extensions-icon-button extensions-icon-button--row extensions-icon-button--primary"
            aria-label={t(extension.runtimeAvailable ? "settings.extensions.connect" : "settings.extensions.installTool")}
            title={t(extension.runtimeAvailable ? "settings.extensions.connect" : "settings.extensions.installTool")}
            aria-busy={rowInstallBusy}
            disabled={busy || (!extension.runtimeAvailable && !extension.homepage)}
            onClick={extension.runtimeAvailable ? onConnect : onRepair}
          >
            {rowInstallBusy ? (
              <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" />
            ) : extension.runtimeAvailable ? (
              <Link2 size={14} strokeWidth={2} aria-hidden="true" />
            ) : (
              <ExternalLink size={14} strokeWidth={2} aria-hidden="true" />
            )}
          </button>
        ) : extension.distributionSource === "npm" && updateAvailable ? (
          <button
            type="button"
            className="extensions-icon-button extensions-icon-button--row extensions-icon-button--primary"
            aria-label={t(rowUpdateBusy ? "settings.extensions.updating" : "settings.extensions.update")}
            title={t("settings.extensions.update")}
            aria-busy={rowUpdateBusy}
            disabled={busy}
            onClick={onUpdate}
          >
            {rowUpdateBusy ? (
              <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" />
            ) : (
              <Download size={14} strokeWidth={2} aria-hidden="true" />
            )}
          </button>
        ) : null}

        {extension.connected
          && !extension.runtimeAvailable
          && (extension.reconnectAvailable || extension.distributionSource === "npm" || extension.homepage) && (
          <button
            type="button"
            className="extensions-icon-button extensions-icon-button--row"
            aria-label={t(extension.reconnectAvailable ? "settings.extensions.reconnect" : extension.distributionSource === "npm" ? "settings.extensions.repair" : "settings.extensions.installTool")}
            title={t(extension.reconnectAvailable ? "settings.extensions.reconnect" : extension.distributionSource === "npm" ? "settings.extensions.repair" : "settings.extensions.installTool")}
            aria-busy={rowRepairBusy}
            disabled={busy}
            onClick={extension.reconnectAvailable ? onReconnect : onRepair}
          >
            {rowRepairBusy ? (
              <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" />
            ) : extension.reconnectAvailable ? (
              <RefreshCw size={14} strokeWidth={2} aria-hidden="true" />
            ) : extension.distributionSource === "npm" ? (
              <Wrench size={14} strokeWidth={2} aria-hidden="true" />
            ) : (
              <ExternalLink size={14} strokeWidth={2} aria-hidden="true" />
            )}
          </button>
        )}

        {extension.connected && (
          <span className="extension-row__toggle-slot">
            <button
              type="button"
              role="switch"
              aria-checked={extension.enabled}
              aria-label={extension.enabled ? t("settings.extensions.disable") : t("settings.extensions.enable")}
              aria-busy={rowToggleBusy}
              className={`settings-switch${extension.enabled ? " settings-switch--active" : ""}${rowToggleBusy ? " settings-switch--loading" : ""}`}
              disabled={busy || extension.state === "broken"}
              onClick={onToggle}
            >
              {rowToggleBusy ? (
                <LoaderCircle className="extensions-spinner" size={12} strokeWidth={2} aria-hidden="true" />
              ) : (
                <span className="settings-switch__thumb" />
              )}
            </button>
          </span>
        )}

        {extension.connected && (
          <details className="extension-menu">
            <summary
              className={`extensions-icon-button extensions-icon-button--row${busy ? " extensions-icon-button--disabled" : ""}`}
              aria-label={t("settings.extensions.moreActions")}
              aria-disabled={busy}
              title={t("settings.extensions.moreActions")}
              onClick={(event) => { if (busy) event.preventDefault(); }}
            >
              <MoreHorizontal size={17} strokeWidth={2} aria-hidden="true" />
            </summary>
            <div className="extension-menu__items">
              {extension.generatedCustom && (
                <button type="button" disabled={busy} onClick={onEdit}>
                  {t("settings.extensions.editCustom")}
                </button>
              )}
              <button type="button" disabled={!extension.previousVersion || busy} onClick={onRollback}>
                <RotateCcw size={14} strokeWidth={2} />
                {t("settings.extensions.rollback")}
              </button>
              <button
                type="button"
                disabled={extension.distributionSource !== "npm" || !extension.packageName || busy}
                onClick={onReinstall}
              >
                <RefreshCw size={14} strokeWidth={2} />
                {t("settings.extensions.reinstall")}
              </button>
              <button type="button" className="extension-menu__danger" disabled={busy} onClick={onUninstall}>
                {kind === "system" ? <Unplug size={14} strokeWidth={2} /> : <Trash2 size={14} strokeWidth={2} />}
                {t(kind === "custom" ? "settings.extensions.deleteCustom" : kind === "npm" ? "settings.extensions.uninstall" : kind === "system" ? "settings.extensions.disconnect" : "settings.extensions.removePackage")}
              </button>
            </div>
          </details>
        )}
      </div>
    </article>
  );
}
