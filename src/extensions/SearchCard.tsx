import { AlertTriangle, Check, Download, LoaderCircle, Package, ShieldCheck } from "lucide-react";
import type { Translate } from "../i18n";
import type { PermissionReview, SearchResult } from "../ExtensionsPanel";

type Props = {
  result: SearchResult;
  review?: PermissionReview;
  installed: boolean;
  selected: boolean;
  reviewing: boolean;
  busy: boolean;
  t: Translate;
  onPrefetch: () => void;
  onInstall: () => void;
  formatDownloads: (value: number) => string;
  deprecationDescription: (value: string) => string;
};

export function SearchCard({
  result,
  review,
  installed,
  selected,
  reviewing,
  busy,
  t,
  onPrefetch,
  onInstall,
  formatDownloads,
  deprecationDescription,
}: Props) {
  const deprecation = review?.deprecation ?? result.deprecation;

  return (
    <article className={`extension-search-card${selected ? " extension-search-card--selected" : ""}`}>
      <header className="extension-search-card__header">
        <span className="extension-search-card__icon">
          <Package size={18} strokeWidth={1.8} aria-hidden="true" />
        </span>
        <span className="extension-search-card__identity">
          <strong title={result.package}>{result.package}</strong>
          <span>v{result.version}</span>
        </span>
      </header>

      <p>{result.description || t("settings.extensions.noDescription")}</p>

      <div className="extension-search-card__badges">
        <span className={`extension-trust-badge extension-trust-badge--${result.verified ? "official" : "community"}`}>
          {result.verified
            ? <ShieldCheck size={11} strokeWidth={2} aria-hidden="true" />
            : <Package size={11} strokeWidth={2} aria-hidden="true" />}
          {t(result.verified ? "settings.extensions.trustOfficial" : "settings.extensions.trustCommunity")}
        </span>
        {deprecation !== null && (
          <span className="extension-trust-badge extension-trust-badge--deprecated" title={deprecationDescription(deprecation)}>
            <AlertTriangle size={11} strokeWidth={2} aria-hidden="true" />
            {t("settings.extensions.deprecated")}
          </span>
        )}
      </div>

      <footer className="extension-search-card__footer">
        <div className="extension-search-card__meta">
          {result.publisher && <span title={result.publisher}>{result.publisher}</span>}
          <span className="extension-search-card__downloads">
            <Download size={12} strokeWidth={2} aria-hidden="true" />
            {t("settings.extensions.downloads", { count: formatDownloads(result.downloads) })}
          </span>
          {review?.permissions.length ? (
            <span className="extension-search-card__permissions" title={review.permissions.map(({ description }) => description).join("\n")}>
              <ShieldCheck size={12} strokeWidth={2} aria-hidden="true" />
              {t("settings.extensions.permissionCount", { count: review.permissions.length })}
            </span>
          ) : null}
        </div>
        <button
          type="button"
          className="extensions-action-button extensions-action-button--primary"
          aria-busy={reviewing || busy}
          disabled={installed || busy || reviewing}
          onMouseEnter={onPrefetch}
          onFocus={onPrefetch}
          onClick={onInstall}
        >
          {installed ? (
            <><Check size={13} strokeWidth={2} aria-hidden="true" />{t("settings.extensions.installed")}</>
          ) : reviewing ? (
            <><LoaderCircle className="extensions-spinner" size={13} strokeWidth={2} aria-hidden="true" />{t("settings.extensions.reviewingPermissions")}</>
          ) : busy ? (
            <><LoaderCircle className="extensions-spinner" size={13} strokeWidth={2} aria-hidden="true" />{t("settings.extensions.installing")}</>
          ) : (
            t("settings.extensions.install")
          )}
        </button>
      </footer>
    </article>
  );
}
