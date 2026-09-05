import {
  ExternalLink,
  Link2,
  LoaderCircle,
  MoreHorizontal,
  Package,
  RefreshCw,
  Trash2,
  Unplug,
  Wrench,
} from "lucide-react";
import type { Translate } from "../i18n";
import { useEffect, useRef } from "react";
import type { Extension, ExtensionOperation } from "../ExtensionsPanel";

type Props = {
  extension: Extension;
  operation: ExtensionOperation;
  t: Translate;
  onOpen: () => void;
  onConnect?: () => void;
  onRepair: () => void;
  onReconnect: () => void;
  onToggle: () => void;
  onEdit: () => void;
  onUninstall: () => void;
};

const integrationKindKey = (extension: Extension): Parameters<Translate>[0] => {
  if (extension.generatedCustom) return "settings.extensions.integrationKind.custom";
  if (extension.distributionSource === "npm") return "settings.extensions.integrationKind.npm";
  if (!extension.connected && extension.manifestSuggestion) {
    return "settings.extensions.integrationKind.manifest";
  }
  if (!extension.connected && extension.recommended) {
    return "settings.extensions.integrationKind.recommended";
  }
  if (!extension.connected && extension.distributionSource === "local") {
    return "settings.extensions.integrationKind.tool";
  }
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
  operation,
  t,
  onOpen,
  onConnect,
  onRepair,
  onReconnect,
  onToggle,
  onEdit,
  onUninstall,
}: Props) {
  const busy = Boolean(operation);
  const menuRef = useRef<HTMLDetailsElement>(null);

  const closeMenu = () => {
    if (menuRef.current?.open) {
      menuRef.current.open = false;
      menuRef.current.querySelector("summary")?.focus();
    }
  };

  useEffect(() => {
    const handleMouseDown = (event: MouseEvent) => {
      const details = menuRef.current;
      if (details?.open && event.target instanceof Node && !details.contains(event.target)) {
        details.open = false;
      }
    };
    document.addEventListener("mousedown", handleMouseDown);
    return () => document.removeEventListener("mousedown", handleMouseDown);
  }, []);
  const rowBusy = operation?.id === extension.id;
  const rowToggleBusy = rowBusy && (operation?.kind === "enable" || operation?.kind === "disable");
  const rowInstallBusy = rowBusy && operation?.kind === "install";
  const rowRepairBusy = rowBusy && operation?.kind === "repair";
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
          {!extension.connected && extension.recommended && (
            <span className="extension-status extension-status--recommended">
              {t("settings.extensions.recommended")}
            </span>
          )}
          {!extension.connected && extension.manifestSuggestion && (
            <span className="extension-status extension-status--recommended">
              {t("settings.extensions.manifestTool")}
            </span>
          )}
          <span>{t(`settings.extensions.runtimeSource.${extension.runtimeSource}`)}</span>
          <span
            className={`extension-status extension-status--${extension.state}`}
            title={extension.state === "broken"
              ? extension.brokenReason || extension.lastErrorCode || undefined
              : undefined}
          >
            {status}
          </span>
          {!extension.runtimeAvailable && (
            <span className="extension-status extension-status--broken">
              {t("settings.extensions.runtimeUnavailable")}
            </span>
          )}
        </span>
      </span>
    </>
  );

  return (
    <article className={`extension-row${extension.connected ? "" : " extension-row--detected"}${extension.state === "broken" ? " extension-row--broken" : ""}`}>
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
        ) : null}

        {extension.connected && extension.state === "broken" && (
          <button
            type="button"
            className="extensions-icon-button extensions-icon-button--row"
            aria-label={t("settings.extensions.recheck")}
            title={t("settings.extensions.recheck")}
            aria-busy={rowRepairBusy}
            disabled={busy}
            onClick={onRepair}
          >
            {rowRepairBusy ? (
              <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" />
            ) : (
              <Wrench size={14} strokeWidth={2} aria-hidden="true" />
            )}
          </button>
        )}

        {extension.connected
          && extension.state !== "broken"
          && !extension.runtimeAvailable
          && (extension.reconnectAvailable || extension.homepage) && (
          <button
            type="button"
            className="extensions-icon-button extensions-icon-button--row"
            aria-label={t(extension.reconnectAvailable ? "settings.extensions.reconnect" : "settings.extensions.installTool")}
            title={t(extension.reconnectAvailable ? "settings.extensions.reconnect" : "settings.extensions.installTool")}
            aria-busy={rowRepairBusy}
            disabled={busy}
            onClick={extension.reconnectAvailable ? onReconnect : onRepair}
          >
            {rowRepairBusy ? (
              <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" />
            ) : extension.reconnectAvailable ? (
              <RefreshCw size={14} strokeWidth={2} aria-hidden="true" />
            ) : (
              <Wrench size={14} strokeWidth={2} aria-hidden="true" />
            )}
          </button>
        )}

        {extension.connected && (
          <details ref={menuRef} className="extension-menu" onKeyDown={(event) => {
            if (event.key !== "Escape") return;
            event.preventDefault();
            event.stopPropagation();
            closeMenu();
          }} onToggle={() => {
            if (!menuRef.current?.open) return;
            document.querySelectorAll<HTMLDetailsElement>(".extension-menu[open]").forEach((menu) => {
              if (menu !== menuRef.current) menu.open = false;
            });
          }}>
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
                <button type="button" disabled={busy} onClick={() => { closeMenu(); onEdit(); }}>
                  {t("settings.extensions.editCustom")}
                </button>
              )}
              <button type="button" className="extension-menu__danger" disabled={busy} onClick={() => { closeMenu(); onUninstall(); }}>
                {kind === "system" ? <Unplug size={14} strokeWidth={2} /> : <Trash2 size={14} strokeWidth={2} />}
                {t(kind === "custom" ? "settings.extensions.deleteCustom" : kind === "npm" ? "settings.extensions.uninstall" : kind === "system" ? "settings.extensions.disconnect" : "settings.extensions.removePackage")}
              </button>
            </div>
          </details>
        )}
      </div>

      {extension.connected && (
        <span className="extension-row__toggle-slot" onClick={(event) => event.stopPropagation()}>
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
    </article>
  );
}
