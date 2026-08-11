import { useEffect, useMemo, useRef, useState, type FormEvent, type MouseEvent } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  AlertCircle,
  Check,
  ChevronRight,
  Download,
  ExternalLink,
  FileDown,
  FileUp,
  MoreHorizontal,
  Package,
  RefreshCw,
  RotateCcw,
  Search,
  ShieldCheck,
  Trash2,
  X,
} from "lucide-react";
import type { Translate } from "./i18n";

type ExtensionInstallType = "managed" | "linked";
type ExtensionStateKind =
  | "enabled"
  | "disabled"
  | "broken";

type Extension = {
  id: string;
  name: string;
  publisherId: string;
  publisherName: string;
  installType: ExtensionInstallType;
  state: ExtensionStateKind;
  enabled: boolean;
  packageName: string | null;
  packageVersion: string;
  toolVersion: string | null;
  integrity: string | null;
  signatureVerified: boolean;
  currentVersion: string;
  previousVersion: string | null;
  manifestPath: string;
  executablePath: string;
  runtimeRoot: string | null;
  installedAt: number;
  updatedAt: number;
  pinned: boolean;
  channel: string;
};

type SearchResult = {
  package: string;
  version: string;
  description: string;
  publisher: string | null;
  homepage: string | null;
  verified: boolean;
  downloads: number;
};

type CommandDescriptor = {
  id: string;
  name: string;
  description: string;
  aliases: string[];
};

type ProviderDescription = {
  protocolVersion: string;
  provider: {
    id: string;
    name: string;
    version: string;
    description: string;
  };
  commands: CommandDescriptor[];
};

type ProviderResponse = {
  description: ProviderDescription;
  runtimeAvailable: boolean;
  cached: boolean;
  stderr: string | null;
};

type DiagnoseResult = {
  status: string;
  checks: Array<{ id: string; status: string; message: string }>;
};

type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };
type ConfigFieldType = "text" | "password" | "path" | "select" | "multi-select" | "boolean" | "number";

type ConfigField = {
  key: string;
  type: ConfigFieldType;
  label: string;
  description: string;
  required: boolean;
  default: JsonValue | null;
  options: JsonValue[];
  minimum: number | null;
  maximum: number | null;
  minLength: number | null;
  maxLength: number | null;
  envVar: string | null;
  argument: string | null;
};

export type ExtensionExecutionPlan = {
  program: string;
  args: string[];
  mode: "pty" | "external";
  cwd: string | null;
  environment: Record<string, string>;
  inheritEnvironment: boolean;
  planToken?: string;
  argumentOverride?: string[];
};

type ExtensionConfiguration = {
  descriptor: {
    configVersion: number;
    owner: "host" | "tool";
    openCommand: string[];
    schema: ConfigField[];
    environmentMapping: Record<string, string>;
  };
  values: Record<string, JsonValue>;
  openPlan: ExtensionExecutionPlan | null;
};

type Tab = "installed" | "discover" | "updates";
type MutationKind = "enable" | "disable" | "install" | "update" | "rollback" | "reinstall" | "uninstall" | "save";
type SyncOperation = "export" | "import";

type ExtensionsExportResult = {
  path: string;
  extensionCount: number;
};

type ExtensionsImportItem = {
  id: string;
  message: string;
};

type ExtensionsImportReport = {
  path: string;
  succeeded: ExtensionsImportItem[];
  failed: ExtensionsImportItem[];
  skipped: ExtensionsImportItem[];
};

type ExtensionsPanelProps = {
  t: Translate;
  locale: "en" | "zh";
  onOpenCommand: (plan: ExtensionExecutionPlan, label: string) => void | Promise<void>;
};

type PermissionName =
  | "filesystem-read"
  | "filesystem-write"
  | "network-fetch"
  | "process-spawn"
  | "clipboard-read"
  | "clipboard-write"
  | "environment";

type PermissionReview = {
  extensionId: string;
  extensionName: string;
  permissions: Array<{ permission: PermissionName; title: string; description: string }>;
};

type InstallRequest = {
  source: "npm";
  package: string;
  version: string;
  manifestPath: null;
  executablePath: null;
  approvedPermissions?: PermissionName[];
};

const compareVersions = (left: string, right: string): number => {
  const parse = (value: string) => {
    const [core, prerelease = ""] = value.replace(/^v/, "").split("-", 2);
    const numbers = core.split(".").map((part) => Number.parseInt(part, 10) || 0);
    return { numbers, prerelease };
  };
  const a = parse(left);
  const b = parse(right);
  for (let index = 0; index < Math.max(a.numbers.length, b.numbers.length); index += 1) {
    const difference = (a.numbers[index] ?? 0) - (b.numbers[index] ?? 0);
    if (difference !== 0) return difference;
  }
  if (a.prerelease === b.prerelease) return 0;
  if (!a.prerelease) return 1;
  if (!b.prerelease) return -1;
  return a.prerelease.localeCompare(b.prerelease, undefined, { numeric: true });
};

const formatDownloads = (value: number): string =>
  new Intl.NumberFormat(undefined, { notation: value >= 10_000 ? "compact" : "standard", maximumFractionDigits: 1 }).format(value);

const errorMessage = (error: unknown): string =>
  error instanceof Error ? error.message : String(error);

const displayJson = (value: JsonValue): string => {
  if (typeof value === "string") return value;
  if (value === null) return "";
  return JSON.stringify(value);
};

export function ExtensionsPanel({ t, locale, onOpenCommand }: ExtensionsPanelProps) {
  const [tab, setTab] = useState<Tab>("installed");
  const [extensions, setExtensions] = useState<Extension[]>([]);
  const [latestById, setLatestById] = useState<Record<string, SearchResult>>({});
  const [loading, setLoading] = useState(true);
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<{ id: string; kind: MutationKind } | null>(null);
  const [query, setQuery] = useState("");
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [hasSearched, setHasSearched] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [provider, setProvider] = useState<ProviderResponse | null>(null);
  const [diagnose, setDiagnose] = useState<DiagnoseResult | null>(null);
  const [configuration, setConfiguration] = useState<ExtensionConfiguration | null>(null);
  const [configValues, setConfigValues] = useState<Record<string, JsonValue>>({});
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [reviewingPackage, setReviewingPackage] = useState<string | null>(null);
  const [permissionReviews, setPermissionReviews] = useState<Record<string, PermissionReview>>({});
  const [pendingInstall, setPendingInstall] = useState<{ result: SearchResult; request: InstallRequest; review: PermissionReview } | null>(null);
  const [syncOperation, setSyncOperation] = useState<SyncOperation | null>(null);
  const [exportResult, setExportResult] = useState<ExtensionsExportResult | null>(null);
  const [importReport, setImportReport] = useState<ExtensionsImportReport | null>(null);
  const detailGeneration = useRef(0);

  const updates = useMemo(
    () => extensions.filter((extension) => {
      const latest = latestById[extension.id];
      return latest && compareVersions(latest.version, extension.currentVersion) > 0;
    }),
    [extensions, latestById],
  );
  const installedPackages = useMemo(
    () => new Set(extensions.map((extension) => extension.packageName).filter(Boolean)),
    [extensions],
  );
  const selected = extensions.find((extension) => extension.id === selectedId) ?? null;

  const checkForUpdates = async (entries: Extension[]) => {
    const managed = entries.filter((entry) => entry.installType === "managed" && entry.packageName && !entry.pinned);
    if (!managed.length) {
      setLatestById({});
      return;
    }
    setCheckingUpdates(true);
    const settled = await Promise.allSettled(
      managed.map(async (entry) => {
        const results = await invoke<SearchResult[]>("extensions_search", { query: entry.packageName, limit: 10 });
        return [entry.id, results.find((result) => result.package === entry.packageName) ?? null] as const;
      }),
    );
    const next: Record<string, SearchResult> = {};
    settled.forEach((result) => {
      if (result.status === "fulfilled" && result.value[1]) next[result.value[0]] = result.value[1];
    });
    setLatestById(next);
    setCheckingUpdates(false);
  };

  const refresh = async () => {
    setLoading(true);
    setError(null);
    try {
      const entries = await invoke<Extension[]>("extensions_list");
      setExtensions(entries);
      await checkForUpdates(entries);
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    if (!selectedId) {
      detailGeneration.current += 1;
      setProvider(null);
      setDiagnose(null);
      setConfiguration(null);
      setDetailError(null);
      return;
    }
    const generation = ++detailGeneration.current;
    setDetailLoading(true);
    setProvider(null);
    setDiagnose(null);
    setConfiguration(null);
    setDetailError(null);
    void Promise.allSettled([
      invoke<ProviderResponse>("extensions_describe", { id: selectedId }),
      invoke<DiagnoseResult>("extensions_diagnose", { id: selectedId }),
      invoke<ExtensionConfiguration>("extensions_config_get", { id: selectedId }),
    ]).then(([descriptionResult, diagnoseResult, configResult]) => {
      if (generation !== detailGeneration.current) return;
      if (descriptionResult.status === "fulfilled") setProvider(descriptionResult.value);
      if (diagnoseResult.status === "fulfilled") setDiagnose(diagnoseResult.value);
      if (configResult.status === "fulfilled") {
        setConfiguration(configResult.value);
        const defaults = Object.fromEntries(
          configResult.value.descriptor.schema
            .filter((field) => field.default !== null)
            .map((field) => [field.key, field.default]),
        );
        setConfigValues({ ...defaults, ...configResult.value.values });
      }
      const primaryError = descriptionResult.status === "rejected"
        ? descriptionResult.reason
        : diagnoseResult.status === "rejected" && configResult.status === "rejected"
          ? diagnoseResult.reason
          : null;
      if (primaryError) setDetailError(errorMessage(primaryError));
      setDetailLoading(false);
    });
  }, [selectedId, selected?.updatedAt]);

  useEffect(() => {
    if (!selectedId) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      setSelectedId(null);
    };
    window.addEventListener("keydown", closeOnEscape, true);
    return () => window.removeEventListener("keydown", closeOnEscape, true);
  }, [selectedId]);

  const runMutation = async (id: string, kind: MutationKind, action: () => Promise<unknown>) => {
    if (busy) return;
    setBusy({ id, kind });
    setError(null);
    try {
      await action();
      await refresh();
      if (kind === "uninstall") setSelectedId(null);
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      setBusy(null);
    }
  };

  const exportExtensions = async () => {
    if (syncOperation) return;
    setSyncOperation("export");
    setError(null);
    setExportResult(null);
    setImportReport(null);
    try {
      const result = await invoke<ExtensionsExportResult | null>("extensions_export");
      if (result) setExportResult(result);
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      setSyncOperation(null);
    }
  };

  const importExtensions = async () => {
    if (syncOperation) return;
    setSyncOperation("import");
    setError(null);
    setExportResult(null);
    setImportReport(null);
    try {
      const report = await invoke<ExtensionsImportReport | null>("extensions_import", { locale });
      if (report) {
        setImportReport(report);
        await refresh();
      }
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      setSyncOperation(null);
    }
  };

  const toggleExtension = (extension: Extension) => runMutation(
    extension.id,
    extension.enabled ? "disable" : "enable",
    () => invoke(extension.enabled ? "extensions_disable" : "extensions_enable", { id: extension.id }),
  );

  const reviewPermissions = async (request: InstallRequest): Promise<PermissionReview> =>
    invoke<PermissionReview>("extensions_permissions_summary", { request, locale });

  const prefetchPermissions = async (result: SearchResult) => {
    if (busy || reviewingPackage || permissionReviews[result.package]) return;
    const request: InstallRequest = {
      source: "npm",
      package: result.package,
      version: result.version,
      manifestPath: null,
      executablePath: null,
    };
    setReviewingPackage(result.package);
    try {
      const review = await reviewPermissions(request);
      setPermissionReviews((current) => ({ ...current, [result.package]: review }));
    } catch {
      // Installation reports preflight failures explicitly when the user clicks.
    } finally {
      setReviewingPackage(null);
    }
  };

  const updateExtension = (extension: Extension) => {
    if (!extension.packageName) return Promise.resolve();
    const version = latestById[extension.id]?.version ?? extension.channel;
    const request: InstallRequest = {
      source: "npm",
      package: extension.packageName,
      version,
      manifestPath: null,
      executablePath: null,
    };
    return runMutation(extension.id, "update", async () => {
      const review = await reviewPermissions(request);
      if (review.permissions.length && !window.confirm(t("settings.extensions.confirmPermissionsInline", {
        permissions: review.permissions.map((permission) => permission.title).join(", "),
      }))) return;
      await invoke("extensions_update", {
        id: extension.id,
        version,
        approvedPermissions: review.permissions.map(({ permission }) => permission),
      });
    });
  };

  const rollbackExtension = (extension: Extension) => {
    if (!window.confirm(t("settings.extensions.confirmRollback", { name: extension.name }))) return Promise.resolve();
    return runMutation(
      extension.id,
      "rollback",
      () => invoke("extensions_rollback", { id: extension.id }),
    );
  };

  const reinstallExtension = (extension: Extension) => {
    if (!window.confirm(t("settings.extensions.confirmReinstall", { name: extension.name }))) return Promise.resolve();
    return runMutation(
      extension.id,
      "reinstall",
      async () => {
        if (!extension.packageName) throw new Error(t("settings.extensions.reinstallUnavailable"));
        const request: InstallRequest = {
          source: "npm",
          package: extension.packageName,
          version: extension.currentVersion,
          manifestPath: null,
          executablePath: null,
        };
        const review = await reviewPermissions(request);
        if (review.permissions.length && !window.confirm(t("settings.extensions.confirmPermissionsInline", {
          permissions: review.permissions.map((permission) => permission.title).join(", "),
        }))) return;
        await invoke("extensions_uninstall", { id: extension.id, removeData: false });
        await invoke<Extension>("extensions_install", {
          request: { ...request, approvedPermissions: review.permissions.map(({ permission }) => permission) },
        });
        if (!extension.enabled) await invoke("extensions_disable", { id: extension.id });
      },
    );
  };

  const uninstallExtension = (extension: Extension) => {
    if (!window.confirm(t("settings.extensions.confirmUninstall", { name: extension.name }))) return;
    void runMutation(
      extension.id,
      "uninstall",
      () => invoke("extensions_uninstall", { id: extension.id, removeData: false }),
    );
  };

  const searchExtensions = async (event: FormEvent) => {
    event.preventDefault();
    const searchQuery = query.trim();
    if (!searchQuery || searching) return;
    setSearching(true);
    setHasSearched(true);
    setError(null);
    try {
      setSearchResults(await invoke<SearchResult[]>("extensions_search", { query: searchQuery, limit: 20 }));
    } catch (nextError) {
      setSearchResults([]);
      setError(errorMessage(nextError));
    } finally {
      setSearching(false);
    }
  };

  const performInstall = (result: SearchResult, request: InstallRequest, review: PermissionReview) => runMutation(
    result.package,
    "install",
    () => invoke("extensions_install", {
      request: { ...request, approvedPermissions: review.permissions.map(({ permission }) => permission) },
    }),
  );

  const installExtension = async (result: SearchResult) => {
    if (busy || reviewingPackage) return;
    const request: InstallRequest = {
      source: "npm",
      package: result.package,
      version: result.version,
      manifestPath: null,
      executablePath: null,
    };
    setReviewingPackage(result.package);
    setError(null);
    try {
      const review = permissionReviews[result.package] ?? await reviewPermissions(request);
      if (!permissionReviews[result.package]) {
        setPermissionReviews((current) => ({ ...current, [result.package]: review }));
      }
      if (review.permissions.length) {
        setPendingInstall({ result, request, review });
      } else {
        await performInstall(result, request, review);
      }
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      setReviewingPackage(null);
    }
  };

  const confirmPendingInstall = () => {
    if (!pendingInstall) return;
    const pending = pendingInstall;
    setPendingInstall(null);
    void performInstall(pending.result, pending.request, pending.review);
  };

  const updateAll = async () => {
    if (busy || !updates.length) return;
    setBusy({ id: "*", kind: "update" });
    setError(null);
    try {
      for (const extension of updates) {
        if (!extension.packageName) continue;
        const version = latestById[extension.id]?.version ?? extension.channel;
        const request: InstallRequest = {
          source: "npm",
          package: extension.packageName,
          version,
          manifestPath: null,
          executablePath: null,
        };
        const review = await reviewPermissions(request);
        if (review.permissions.length && !window.confirm(t("settings.extensions.confirmPermissionsInline", {
          permissions: review.permissions.map((permission) => permission.title).join(", "),
        }))) break;
        await invoke("extensions_update", {
          id: extension.id,
          version,
          approvedPermissions: review.permissions.map(({ permission }) => permission),
        });
      }
      await refresh();
    } catch (nextError) {
      setError(errorMessage(nextError));
      await refresh();
    } finally {
      setBusy(null);
    }
  };

  const saveConfiguration = async () => {
    if (busy || !selected || !configuration || configuration.descriptor.owner !== "host") return;
    setBusy({ id: selected.id, kind: "save" });
    setDetailError(null);
    try {
      const saved = await invoke<ExtensionConfiguration>("extensions_config_set", {
        id: selected.id,
        values: configValues,
      });
      setConfiguration(saved);
      setConfigValues(saved.values);
    } catch (nextError) {
      setDetailError(errorMessage(nextError));
    } finally {
      setBusy(null);
    }
  };

  const stopRowClick = (event: MouseEvent) => event.stopPropagation();

  return (
    <section className="settings-section extensions-panel" data-no-drag>
      <div className="settings-section__heading extensions-panel__heading">
        <div>
          <h2 className="settings-section__label">{t("settings.extensions.title")}</h2>
          <p className="settings-section__hint">{t("settings.extensions.hint")}</p>
        </div>
        <button
          type="button"
          className="extensions-icon-button"
          aria-label={t("settings.extensions.refresh")}
          title={t("settings.extensions.refresh")}
          disabled={loading || checkingUpdates}
          onClick={() => void refresh()}
        >
          <RefreshCw size={16} strokeWidth={2} aria-hidden="true" />
        </button>
      </div>

      <div className="extensions-tabs" role="tablist" aria-label={t("settings.extensions.title")}>
        {(["installed", "discover", "updates"] as Tab[]).map((item) => (
          <button
            key={item}
            type="button"
            role="tab"
            aria-selected={tab === item}
            className={`extensions-tabs__item${tab === item ? " extensions-tabs__item--active" : ""}`}
            onClick={() => setTab(item)}
          >
            {t(`settings.extensions.tab.${item}`)}
            {item === "installed" && extensions.length > 0 && <span>{extensions.length}</span>}
            {item === "updates" && updates.length > 0 && <span>{updates.length}</span>}
          </button>
        ))}
      </div>

      {error && (
        <div className="extensions-notice extensions-notice--error" role="alert">
          <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
          <span>{error}</span>
        </div>
      )}

      {tab === "installed" && (
        <div className="extensions-installed" role="tabpanel">
          <div className="extensions-sync-toolbar">
            <button
              type="button"
              className="extensions-action-button"
              disabled={Boolean(syncOperation) || Boolean(busy) || loading}
              onClick={() => void exportExtensions()}
            >
              <FileDown size={14} strokeWidth={2} aria-hidden="true" />
              {syncOperation === "export" ? t("settings.extensions.exporting") : t("settings.extensions.export")}
            </button>
            <button
              type="button"
              className="extensions-action-button"
              disabled={Boolean(syncOperation) || Boolean(busy) || loading}
              onClick={() => void importExtensions()}
            >
              <FileUp size={14} strokeWidth={2} aria-hidden="true" />
              {syncOperation === "import" ? t("settings.extensions.importing") : t("settings.extensions.import")}
            </button>
          </div>
          {exportResult && (
            <div className="extensions-notice extensions-notice--success" role="status" title={exportResult.path}>
              <Check size={15} strokeWidth={2} aria-hidden="true" />
              <span>{t("settings.extensions.exportComplete", { count: exportResult.extensionCount })}</span>
            </div>
          )}
          {importReport && (
            <div
              className={`extensions-notice${importReport.failed.length ? " extensions-notice--error" : " extensions-notice--success"}`}
              role="status"
              title={importReport.failed.map((item) => `${item.id}: ${item.message}`).join("\n") || importReport.path}
            >
              {importReport.failed.length
                ? <AlertCircle size={15} strokeWidth={2} aria-hidden="true" />
                : <Check size={15} strokeWidth={2} aria-hidden="true" />}
              <span>{t("settings.extensions.importSummary", {
                succeeded: importReport.succeeded.length,
                skipped: importReport.skipped.length,
                failed: importReport.failed.length,
              })}</span>
            </div>
          )}
          <div className="extensions-list">
            {loading ? (
              <EmptyState icon={<RefreshCw size={20} strokeWidth={2} />} text={t("settings.extensions.loading")} />
            ) : extensions.length === 0 ? (
              <EmptyState icon={<Package size={20} strokeWidth={2} />} text={t("settings.extensions.emptyInstalled")} />
            ) : extensions.map((extension) => (
              <ExtensionRow
                key={extension.id}
                extension={extension}
                latest={latestById[extension.id]}
                busy={Boolean(busy)}
                t={t}
                onOpen={() => setSelectedId(extension.id)}
                onToggle={() => void toggleExtension(extension)}
                onUpdate={() => void updateExtension(extension)}
                onRollback={() => void rollbackExtension(extension)}
                onReinstall={() => void reinstallExtension(extension)}
                onUninstall={() => uninstallExtension(extension)}
              />
            ))}
          </div>
        </div>
      )}

      {tab === "discover" && (
        <div className="extensions-discover" role="tabpanel">
          <form className="extensions-search" onSubmit={searchExtensions}>
            <Search size={16} strokeWidth={2} aria-hidden="true" />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t("settings.extensions.searchPlaceholder")}
              aria-label={t("settings.extensions.searchPlaceholder")}
            />
            <button type="submit" disabled={!query.trim() || searching}>
              {searching ? t("settings.extensions.searching") : t("settings.extensions.search")}
            </button>
          </form>
          <div className="extensions-source" aria-label={t("settings.extensions.source")}>
            <span className="extensions-source__active"><Check size={12} strokeWidth={2} />{t("settings.extensions.sourceNpm")}</span>
          </div>
          <div className="extensions-list">
            {searching ? (
              <EmptyState icon={<RefreshCw size={20} strokeWidth={2} />} text={t("settings.extensions.searching")} />
            ) : searchResults.length ? searchResults.map((result) => {
              const installed = installedPackages.has(result.package);
              return (
                <div className="extension-search-row" key={result.package}>
                  <div className="extension-search-row__main">
                    <div className="extension-search-row__title">
                      <strong>{result.package}</strong>
                      <span>v{result.version}</span>
                    </div>
                    <p>{result.description || t("settings.extensions.noDescription")}</p>
                    <span className="extension-search-row__downloads">
                      <Download size={12} strokeWidth={2} aria-hidden="true" />
                      {t("settings.extensions.downloads", { count: formatDownloads(result.downloads) })}
                    </span>
                  </div>
                  <div className="extension-search-row__actions">
                    {permissionReviews[result.package]?.permissions.length ? (
                      <span className="extension-search-row__permissions" title={permissionReviews[result.package].permissions.map(({ description }) => description).join("\n")}>
                        <ShieldCheck size={12} strokeWidth={2} aria-hidden="true" />
                        {t("settings.extensions.permissionCount", { count: permissionReviews[result.package].permissions.length })}
                      </span>
                    ) : null}
                    <button
                      type="button"
                      className="extensions-action-button extensions-action-button--primary"
                      disabled={installed || Boolean(busy) || Boolean(reviewingPackage)}
                      onMouseEnter={() => void prefetchPermissions(result)}
                      onFocus={() => void prefetchPermissions(result)}
                      onClick={() => void installExtension(result)}
                    >
                      {installed
                        ? t("settings.extensions.installed")
                        : reviewingPackage === result.package
                          ? t("settings.extensions.reviewingPermissions")
                          : busy?.id === result.package
                            ? t("settings.extensions.installing")
                            : t("settings.extensions.install")}
                    </button>
                  </div>
                </div>
              );
            }) : (
              <EmptyState
                icon={<Search size={20} strokeWidth={2} />}
                text={hasSearched ? t("settings.extensions.emptySearch") : t("settings.extensions.searchPrompt")}
              />
            )}
          </div>
        </div>
      )}

      {tab === "updates" && (
        <div className="extensions-updates" role="tabpanel">
          <div className="extensions-updates__toolbar">
            <span>{checkingUpdates ? t("settings.extensions.checkingUpdates") : t("settings.extensions.updateCount", { count: updates.length })}</span>
            <button
              type="button"
              className="extensions-action-button extensions-action-button--primary"
              disabled={!updates.length || Boolean(busy)}
              onClick={() => void updateAll()}
            >
              <RefreshCw size={14} strokeWidth={2} aria-hidden="true" />
              {busy?.id === "*" ? t("settings.extensions.updating") : t("settings.extensions.updateAll")}
            </button>
          </div>
          <div className="extensions-list">
            {!loading && !checkingUpdates && updates.length === 0 ? (
              <EmptyState icon={<Check size={20} strokeWidth={2} />} text={t("settings.extensions.emptyUpdates")} />
            ) : updates.map((extension) => (
              <div className="extension-update-row" key={extension.id}>
                <button type="button" className="extension-update-row__main" onClick={() => setSelectedId(extension.id)}>
                  <strong>{extension.name}</strong>
                  <span>v{extension.currentVersion} → v{latestById[extension.id].version}</span>
                </button>
                <button
                  type="button"
                  className="extensions-action-button extensions-action-button--primary"
                  disabled={Boolean(busy)}
                  onClick={() => void updateExtension(extension)}
                >
                  {busy?.id === extension.id ? t("settings.extensions.updating") : t("settings.extensions.update")}
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {pendingInstall && (
        <div className="extension-permission-backdrop" role="presentation" onMouseDown={() => setPendingInstall(null)}>
          <section
            className="extension-permission-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="extension-permission-title"
            onMouseDown={stopRowClick}
          >
            <header>
              <ShieldCheck size={18} strokeWidth={2} aria-hidden="true" />
              <div>
                <h3 id="extension-permission-title">{t("settings.extensions.permissionTitle", { name: pendingInstall.review.extensionName })}</h3>
                <p>{t("settings.extensions.permissionHint")}</p>
              </div>
            </header>
            <div className="extension-permission-list">
              {pendingInstall.review.permissions.map((permission) => (
                <div key={permission.permission}>
                  <strong>{permission.title}</strong>
                  <span>{permission.description}</span>
                </div>
              ))}
            </div>
            <footer>
              <button type="button" className="extensions-action-button" onClick={() => setPendingInstall(null)}>{t("settings.extensions.cancel")}</button>
              <button type="button" className="extensions-action-button extensions-action-button--primary" onClick={confirmPendingInstall}>{t("settings.extensions.approveInstall")}</button>
            </footer>
          </section>
        </div>
      )}

      {selected && (
        <div className="extension-drawer-backdrop" role="presentation" onMouseDown={() => setSelectedId(null)}>
          <aside
            className="extension-drawer"
            role="dialog"
            aria-modal="true"
            aria-labelledby="extension-drawer-title"
            onMouseDown={stopRowClick}
          >
            <header className="extension-drawer__header">
              <div>
                <h3 id="extension-drawer-title">{selected.name}</h3>
                <span>v{selected.currentVersion}</span>
              </div>
              <button type="button" className="extensions-icon-button" aria-label={t("settings.extensions.closeDetails")} autoFocus onClick={() => setSelectedId(null)}>
                <X size={17} strokeWidth={2} aria-hidden="true" />
              </button>
            </header>
            <div className="extension-drawer__body">
              {detailLoading && <div className="extension-drawer__loading"><RefreshCw size={17} strokeWidth={2} />{t("settings.extensions.loadingDetails")}</div>}
              {detailError && <div className="extensions-notice extensions-notice--error"><AlertCircle size={15} strokeWidth={2} /><span>{detailError}</span></div>}
              <section className="extension-detail-block">
                <h4>{t("settings.extensions.info")}</h4>
                <dl className="extension-metadata">
                  <div><dt>{t("settings.extensions.author")}</dt><dd>{selected.publisherName}</dd></div>
                  <div><dt>{t("settings.extensions.source")}</dt><dd>{t(`settings.extensions.source.${selected.installType}`)}</dd></div>
                  <div><dt>{t("settings.extensions.status")}</dt><dd>{t(`settings.extensions.status.${selected.state}`)}</dd></div>
                  <div><dt>{t("settings.extensions.signature")}</dt><dd>{t(selected.signatureVerified ? "settings.extensions.signatureVerified" : "settings.extensions.signatureMissing")}</dd></div>
                  <div><dt>{t("settings.extensions.homepage")}</dt><dd>{latestById[selected.id]?.homepage ?? t("settings.extensions.unavailable")}</dd></div>
                </dl>
                <p className="extension-detail-description">{provider?.description.provider.description || latestById[selected.id]?.description || t("settings.extensions.noDescription")}</p>
              </section>

              <section className="extension-detail-block">
                <h4>{t("settings.extensions.commands")}</h4>
                {provider?.description.commands.length ? (
                  <div className="extension-command-list">
                    {provider.description.commands.map((command) => (
                      <div key={command.id}><code>{command.name}</code><span>{command.description || t("settings.extensions.noDescription")}</span></div>
                    ))}
                  </div>
                ) : !detailLoading && <p className="extension-detail-empty">{t("settings.extensions.noCommands")}</p>}
              </section>

              <section className="extension-detail-block">
                <h4>{t("settings.extensions.diagnostics")}</h4>
                {diagnose ? (
                  <div className="extension-diagnostics">
                    <span className={`extension-diagnostics__summary extension-diagnostics__summary--${diagnose.status}`}>{diagnose.status}</span>
                    {diagnose.checks.map((check) => (
                      <div key={check.id}><span className={`extension-check extension-check--${check.status}`}>{check.status}</span><p>{check.message}</p></div>
                    ))}
                  </div>
                ) : !detailLoading && <p className="extension-detail-empty">{t("settings.extensions.diagnosticsUnavailable")}</p>}
              </section>

              {configuration && (
                <section className="extension-detail-block">
                  <h4>{t("settings.extensions.configuration")}</h4>
                  {configuration.descriptor.owner === "tool" ? (
                    <button
                      type="button"
                      className="extensions-action-button"
                      disabled={!configuration.openPlan}
                      onClick={() => configuration.openPlan && void onOpenCommand(configuration.openPlan, `${selected.name} ${t("settings.extensions.configuration")}`)}
                    >
                      <ExternalLink size={14} strokeWidth={2} aria-hidden="true" />
                      {t("settings.extensions.openConfiguration")}
                    </button>
                  ) : (
                    <form className="extension-config-form" onSubmit={(event) => { event.preventDefault(); void saveConfiguration(); }}>
                      {configuration.descriptor.schema.map((field) => (
                        <ConfigFieldControl
                          key={field.key}
                          field={field}
                          value={configValues[field.key] ?? field.default}
                          t={t}
                          onChange={(value) => setConfigValues((current) => ({ ...current, [field.key]: value }))}
                        />
                      ))}
                      <button type="submit" className="extensions-action-button extensions-action-button--primary" disabled={Boolean(busy)}>
                        {busy?.kind === "save" ? t("settings.extensions.saving") : t("settings.extensions.save")}
                      </button>
                    </form>
                  )}
                </section>
              )}
            </div>
            <footer className="extension-drawer__footer">
              <button type="button" className="extensions-action-button" disabled={!selected.previousVersion || Boolean(busy)} onClick={() => void rollbackExtension(selected)}>
                <RotateCcw size={14} strokeWidth={2} />{t("settings.extensions.rollback")}
              </button>
              <button type="button" className="extensions-action-button" disabled={selected.installType !== "managed" || !selected.packageName || Boolean(busy)} onClick={() => void reinstallExtension(selected)}>
                <RefreshCw size={14} strokeWidth={2} />{t("settings.extensions.reinstall")}
              </button>
              <button type="button" className="extensions-action-button extensions-action-button--danger" disabled={Boolean(busy)} onClick={() => uninstallExtension(selected)}>
                <Trash2 size={14} strokeWidth={2} />{t("settings.extensions.uninstall")}
              </button>
            </footer>
          </aside>
        </div>
      )}
    </section>
  );
}

type ExtensionRowProps = {
  extension: Extension;
  latest?: SearchResult;
  busy: boolean;
  t: Translate;
  onOpen: () => void;
  onToggle: () => void;
  onUpdate: () => void;
  onRollback: () => void;
  onReinstall: () => void;
  onUninstall: () => void;
};

function ExtensionRow({ extension, latest, busy, t, onOpen, onToggle, onUpdate, onRollback, onReinstall, onUninstall }: ExtensionRowProps) {
  const updateAvailable = Boolean(latest && compareVersions(latest.version, extension.currentVersion) > 0);
  return (
    <div className="extension-row" onClick={onOpen}>
      <div className="extension-row__icon"><Package size={17} strokeWidth={2} aria-hidden="true" /></div>
      <div className="extension-row__main">
        <div className="extension-row__title"><strong>{extension.name}</strong><span>v{extension.currentVersion}</span></div>
        <div className="extension-row__meta">
          <span>{t(`settings.extensions.source.${extension.installType}`)}</span>
          <span className={`extension-status extension-status--${extension.state}`}>{t(`settings.extensions.status.${extension.state}`)}</span>
          {updateAvailable && <span className="extension-status extension-status--update">{t("settings.extensions.status.updateAvailable")}</span>}
        </div>
      </div>
      <div className="extension-row__actions" onClick={(event) => event.stopPropagation()}>
        {extension.installType === "managed" && (
          <button type="button" className={`extensions-action-button${updateAvailable ? " extensions-action-button--primary" : ""}`} disabled={!updateAvailable || busy} onClick={onUpdate}>
            {t("settings.extensions.update")}
          </button>
        )}
        <button
          type="button"
          role="switch"
          aria-checked={extension.enabled}
          aria-label={extension.enabled ? t("settings.extensions.disable") : t("settings.extensions.enable")}
          className={`settings-switch${extension.enabled ? " settings-switch--active" : ""}`}
          disabled={busy || extension.state === "broken"}
          onClick={onToggle}
        ><span className="settings-switch__thumb" /></button>
        <details className="extension-menu">
          <summary className="extensions-icon-button" aria-label={t("settings.extensions.moreActions")} title={t("settings.extensions.moreActions")}>
            <MoreHorizontal size={17} strokeWidth={2} aria-hidden="true" />
          </summary>
          <div className="extension-menu__items">
            <button type="button" disabled={!extension.previousVersion || busy} onClick={onRollback}><RotateCcw size={14} strokeWidth={2} />{t("settings.extensions.rollback")}</button>
            <button type="button" disabled={extension.installType !== "managed" || !extension.packageName || busy} onClick={onReinstall}><RefreshCw size={14} strokeWidth={2} />{t("settings.extensions.reinstall")}</button>
            <button type="button" className="extension-menu__danger" disabled={busy} onClick={onUninstall}><Trash2 size={14} strokeWidth={2} />{t("settings.extensions.uninstall")}</button>
          </div>
        </details>
      </div>
      <ChevronRight className="extension-row__chevron" size={15} strokeWidth={2} aria-hidden="true" />
    </div>
  );
}

function EmptyState({ icon, text }: { icon: React.ReactNode; text: string }) {
  return <div className="extensions-empty">{icon}<span>{text}</span></div>;
}

type ConfigFieldControlProps = {
  field: ConfigField;
  value: JsonValue | undefined;
  t: Translate;
  onChange: (value: JsonValue) => void;
};

function ConfigFieldControl({ field, value, t, onChange }: ConfigFieldControlProps) {
  const id = `extension-config-${field.key}`;
  const label = field.label || field.key;
  let control: React.ReactNode;
  if (field.type === "boolean") {
    control = (
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={value === true}
        className={`settings-switch${value === true ? " settings-switch--active" : ""}`}
        onClick={() => onChange(value !== true)}
      ><span className="settings-switch__thumb" /></button>
    );
  } else if (field.type === "select") {
    control = (
      <select
        id={id}
        required={field.required}
        value={displayJson(value ?? "")}
        onChange={(event) => {
          const option = field.options.find((candidate) => displayJson(candidate) === event.target.value);
          onChange(option ?? (event.target.value === "" ? null : event.target.value));
        }}
      >
        {!field.required && <option value="">{t("settings.extensions.configNone")}</option>}
        {field.options.map((option) => <option key={displayJson(option)} value={displayJson(option)}>{displayJson(option)}</option>)}
      </select>
    );
  } else if (field.type === "multi-select") {
    const selectedValues = Array.isArray(value) ? value.map(displayJson) : [];
    control = (
      <select
        id={id}
        multiple
        required={field.required}
        value={selectedValues}
        onChange={(event) => onChange(Array.from(event.target.selectedOptions, (option) =>
          field.options.find((candidate) => displayJson(candidate) === option.value) ?? option.value,
        ))}
      >
        {field.options.map((option) => <option key={displayJson(option)} value={displayJson(option)}>{displayJson(option)}</option>)}
      </select>
    );
  } else {
    control = (
      <input
        id={id}
        type={field.type === "password" ? "password" : field.type === "number" ? "number" : "text"}
        required={field.required}
        min={field.minimum ?? undefined}
        max={field.maximum ?? undefined}
        minLength={field.type === "text" ? field.minLength ?? undefined : undefined}
        maxLength={field.type === "text" ? field.maxLength ?? undefined : undefined}
        value={displayJson(value ?? "")}
        onChange={(event) => onChange(field.type === "number" ? (event.target.value === "" ? null : event.target.valueAsNumber) : event.target.value)}
      />
    );
  }
  return (
    <label className="extension-config-field" htmlFor={id}>
      <span>{label}{field.required && <b aria-label={t("settings.extensions.required")}>*</b>}</span>
      {control}
      {field.description && <small>{field.description}</small>}
    </label>
  );
}
