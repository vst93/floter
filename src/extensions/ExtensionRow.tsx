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
  X,
} from "lucide-react";
import type { Translate } from "../i18n";
import { useEffect, useRef } from "react";
import { freshnessDotState, freshnessOf } from "./freshness";
import type { Extension, ExtensionOperation } from "../ExtensionsPanel";

type Props = {
  extension: Extension;
  operation: ExtensionOperation;
  progress?: { stage: string; message?: string };
  t: Translate;
  /** Connected-only: opens the detail drawer. A detected row renders a <div>,
   *  never a button, so it has no open affordance. */
  onOpen?: () => void;
  /** The connect entry (detected rows only). Never reconnect. */
  onConnect?: () => void;
  /** R8-3: a `floter://register` link named this row's tool. A *mark*, not an
   *  action — the row keeps its own Connect button, and the highlight only says
   *  "this is the one the link was about". */
  highlighted?: boolean;
  /** Connected-only recheck; for a detected row this is the "install tool
   *  first" guidance (opens the publisher homepage), not a disk write. */
  onRepair?: () => void;
  /** Connected-only: inventory re-discovery + tool-lock write (R3/G3).
   *  Detected rows must not wire this. */
  onReconnect?: () => void;
  onToggle?: () => void;
  onEdit?: () => void;
  onUninstall?: () => void;
  onCancelOperation?: () => void;
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
  progress,
  t,
  onOpen,
  onConnect,
  highlighted,
  onRepair,
  onReconnect,
  onToggle,
  onEdit,
  onUninstall,
  onCancelOperation,
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
  // R7-7b · the row's freshness dot, projected by the same pure module the
  // drawer's freshness block uses. Deliberately coarser than the drawer: a row
  // never sees the health report, so a degraded integration reads `unknown`
  // here and "Succeeded with warnings" there — both true, at their own
  // resolution. The error code is passed through so a stale probe timestamp
  // cannot make the last *failed* probe read as a clean comparison.
  const dotState = freshnessDotState(freshnessOf({
    lastProbeAt: extension.lastProbeAt,
    errorCode: extension.lastErrorCode,
    commandCount: extension.commandCount,
    previousCommandCount: extension.previousCommandCount,
  }));
  const dotLabel = t("settings.extensions.freshnessDot", {
    state: t(`settings.extensions.freshnessDot.${dotState}` as Parameters<Translate>[0]),
  });

  const rowContent = (
    <>
      <span className="extension-row__icon">
        <Package size={17} strokeWidth={2} aria-hidden="true" />
      </span>
      <span className="extension-row__main">
        <span className="extension-row__title">
          {/* R8-3 · the `floter://register` mark. A status *mark*, not a
              control: it is not focusable and adds no affordance, so it never
              competes with the row's Connect button. `role="img"` + label is
              what makes it readable — an aria-hidden dot would leave "this is
              the tool the link named" with no non-visual representation. */}
          {highlighted && (
            <span
              className="extension-row__register-dot"
              role="img"
              aria-label={t("settings.extensions.registerHighlight")}
              title={t("settings.extensions.registerHighlight")}
            />
          )}
          <strong>{extension.name}</strong>
          <span>v{extension.currentVersion}</span>
          {/* A status *mark*, not a control: it is not focusable and adds no
              affordance, so it never competes with the row's existing focus
              targets. `role="img"` + label is what makes it readable — an
              aria-hidden dot would leave the state with no non-visual
              representation at all.

              Connected rows only. A detected row has no probe of any kind, so
              its dot could only ever say "unknown" — three grey dots down the
              Detected section is noise carrying zero information. `unknown`
              still has a real home on a *connected* row: a publisher-
              descriptor integration's command table is never probed, and a
              generated one is unknown until its first probe. */}
          {extension.connected && (
            <span
              className={`extension-row__dot extension-row__dot--${dotState}`}
              role="img"
              aria-label={dotLabel}
              title={dotLabel}
            />
          )}
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
              ? extension.brokenReason || (extension.lastErrorCode && t(`settings.extensions.errorCode.${extension.lastErrorCode}` as any)) || undefined
              : undefined}
          >
            {status}
          </span>
          {!extension.runtimeAvailable && (
            <span
              className="extension-status extension-status--broken"
              title={
                extension.runtimeUnavailableCode
                  ? `${t(`settings.extensions.errorCode.${extension.runtimeUnavailableCode}` as Parameters<Translate>[0])}${extension.runtimeUnavailableDetail ? ` · ${extension.runtimeUnavailableDetail}` : ""}`
                  : t("settings.extensions.runtimeUnavailable")
              }
            >
              {t("settings.extensions.runtimeUnavailable")}
              {/* The specific cause rides beside the badge only when the list
                  knows one: `runtimeUnavailableCode` comes from the single
                  availability projector, so a catalog-load failure the user
                  used to be unable to see now has a dictionary entry in the
                  row itself, without a new visual. */}
              {extension.runtimeUnavailableCode
                ? ` · ${t(`settings.extensions.errorCode.${extension.runtimeUnavailableCode}` as Parameters<Translate>[0])}`
                : ""}
            </span>
          )}
        </span>
      </span>
    </>
  );

  return (
    <article className={`extension-row${extension.connected ? "" : " extension-row--detected"}${extension.state === "broken" ? " extension-row--broken" : ""}${highlighted ? " extension-row--register" : ""}`}>
      {extension.connected ? (
        <button type="button" className="extension-row__open" onClick={onOpen}>
          {rowContent}
        </button>
      ) : (
        <div className="extension-row__open">{rowContent}</div>
      )}

      {progress && (
        <div className="extension-row__progress" title={progress.message || progress.stage}>
          <span className="extension-row__progress-text">{progress.stage}</span>
          {onCancelOperation && (
            <button
              type="button"
              className="extensions-icon-button extensions-icon-button--row"
              aria-label={t("settings.extensions.cancelOperation")}
              onClick={onCancelOperation}
            >
              <X size={14} strokeWidth={2} aria-hidden="true" />
            </button>
          )}
        </div>
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

        {/* Runtime unavailable. Reconnect whenever a candidate is known, and
            also when none is — but ONLY for a system-runtime row: an "unbound"
            system tool (no persisted binding) with zero PATH matches and no
            homepage previously rendered no action at all, leaving the row with
            a "Tool unavailable" status and no way to recover after the user
            installs the tool. Reconnect re-scans PATH, so it is the right entry
            there too. The `runtimeSource === "system"` guard keeps that
            fallback scoped to rows whose backend reconnect can actually run
            (`reconnect_system_locked` rejects non-system ownership); managed /
            bundled rows without a homepage keep the old behavior of rendering
            no action here, while a row that still advertises a homepage falls
            back to the repair/install action. */}
        {extension.connected
          && extension.state !== "broken"
          && !extension.runtimeAvailable
          && (extension.reconnectAvailable || extension.homepage || extension.runtimeSource === "system") && (
          <button
            type="button"
            className="extensions-icon-button extensions-icon-button--row"
            aria-label={t(extension.reconnectAvailable || !extension.homepage ? "settings.extensions.reconnect" : "settings.extensions.installTool")}
            title={t(
              extension.reconnectAvailable
                ? "settings.extensions.reconnect"
                : extension.homepage
                  ? "settings.extensions.installTool"
                  : "settings.extensions.reconnectUnboundHint",
            )}
            aria-busy={rowRepairBusy}
            disabled={busy}
            onClick={extension.reconnectAvailable || !extension.homepage ? onReconnect : onRepair}
          >
            {rowRepairBusy ? (
              <LoaderCircle className="extensions-spinner" size={14} strokeWidth={2} aria-hidden="true" />
            ) : extension.reconnectAvailable || !extension.homepage ? (
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
                <button type="button" disabled={busy} onClick={() => { closeMenu(); onEdit?.(); }}>
                  {t("settings.extensions.editCustom")}
                </button>
              )}
              <button type="button" className="extension-menu__danger" disabled={busy} onClick={() => { closeMenu(); onUninstall?.(); }}>
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
